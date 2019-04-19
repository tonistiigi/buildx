package build

import (
	"bufio"
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/urlutil"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/upload/uploadprovider"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/tonistiigi/buildx/driver"
	"github.com/tonistiigi/buildx/util/progress"
	"golang.org/x/sync/errgroup"
)

var (
	errStdinConflict      = errors.New("invalid argument: can't use stdin for both build context and dockerfile")
	errDockerfileConflict = errors.New("ambiguous Dockerfile source: both stdin and flag correspond to Dockerfiles")
)

type Options struct {
	Inputs      Inputs
	Tags        []string
	Labels      map[string]string
	BuildArgs   map[string]string
	Pull        bool
	ImageIDFile string
	ExtraHosts  []string
	NetworkMode string

	NoCache   bool
	Target    string
	Platforms []specs.Platform
	Exports   []client.ExportEntry
	Session   []session.Attachable

	CacheFrom []client.CacheOptionsEntry
	CacheTo   []client.CacheOptionsEntry

	// DockerTarget
}

type Inputs struct {
	ContextPath    string
	DockerfilePath string
	InStream       io.Reader
}

type DriverInfo struct {
	Driver   driver.Driver
	Name     string
	Platform []specs.Platform
	Err      error
}

type DockerAPI interface {
	DockerAPI(name string) (dockerclient.APIClient, error)
}

func getFirstDriver(drivers []DriverInfo) (driver.Driver, error) {
	err := errors.Errorf("no drivers found")
	for _, di := range drivers {
		if di.Driver != nil {
			return di.Driver, nil
		}
		if di.Err != nil {
			err = di.Err
		}
	}
	return nil, err
}

func filterAvailableDrivers(drivers []DriverInfo) ([]DriverInfo, error) {
	out := make([]DriverInfo, 0, len(drivers))
	err := errors.Errorf("no drivers found")
	for _, di := range drivers {
		if di.Err == nil && di.Driver != nil {
			out = append(out, di)
		}
		if di.Err != nil {
			err = di.Err
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	return nil, err
}

type driverPair struct {
	driverIndex int
	platforms   []specs.Platform
}

func driverIndexes(m map[string][]driverPair) []int {
	out := make([]int, 0, len(m))
	visited := map[int]struct{}{}
	for _, dp := range m {
		for _, d := range dp {
			if _, ok := visited[d.driverIndex]; ok {
				continue
			}
			visited[d.driverIndex] = struct{}{}
			out = append(out, d.driverIndex)
		}
	}
	return out
}

func allIndexes(l int) []int {
	out := make([]int, 0, l)
	for i := 0; i < l; i++ {
		out = append(out, i)
	}
	return out
}

func ensureBooted(ctx context.Context, drivers []DriverInfo, idxs []int, pw progress.Writer) ([]*client.Client, error) {
	clients := make([]*client.Client, len(drivers))

	eg, ctx := errgroup.WithContext(ctx)

	for _, i := range idxs {
		func(i int) {
			eg.Go(func() error {
				c, err := driver.Boot(ctx, drivers[i].Driver, pw)
				if err != nil {
					return err
				}
				clients[i] = c
				return nil
			})
		}(i)
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return clients, nil
}

func splitToDriverPairs(availablePlatforms map[string]int, opt map[string]Options) map[string][]driverPair {
	m := map[string][]driverPair{}
	for k, opt := range opt {
		mm := map[int][]specs.Platform{}
		for _, p := range opt.Platforms {
			k := platforms.Format(p)
			idx := availablePlatforms[k] // default 0
			pp := mm[idx]
			pp = append(pp, p)
			mm[idx] = pp
		}
		dps := make([]driverPair, 0, 2)
		for idx, pp := range mm {
			dps = append(dps, driverPair{driverIndex: idx, platforms: pp})
		}
		m[k] = dps
	}
	return m
}

func resolveDrivers(ctx context.Context, drivers []DriverInfo, opt map[string]Options, pw progress.Writer) (map[string][]driverPair, []*client.Client, error) {

	availablePlatforms := map[string]int{}
	for i, d := range drivers {
		for _, p := range d.Platform {
			availablePlatforms[platforms.Format(p)] = i
		}
	}

	undetectedPlatform := false
	allPlatforms := map[string]int{}
	for _, opt := range opt {
		for _, p := range opt.Platforms {
			k := platforms.Format(p)
			allPlatforms[k] = -1
			if _, ok := availablePlatforms[k]; !ok {
				undetectedPlatform = true
			}
		}
	}

	// fast path
	if len(drivers) == 1 || len(allPlatforms) == 0 {
		m := map[string][]driverPair{}
		for k, opt := range opt {
			m[k] = []driverPair{{driverIndex: 0, platforms: opt.Platforms}}
		}
		clients, err := ensureBooted(ctx, drivers, driverIndexes(m), pw)
		if err != nil {
			return nil, nil, err
		}
		return m, clients, nil
	}

	// map based on existing platforms
	if !undetectedPlatform {
		// for k := range opt {
		// 	mm := map[int][]specs.Platform{}
		// 	for _, p := range opt.Platforms {
		// 		k := platforms.Format(p)
		// 		idx := availablePlatforms[k]
		// 		pp := mm[idx]
		// 		pp = append(pp, p)
		// 		mm[idx] = pp
		// 	}
		// 	dps := make([]driverPair, 0, 2)
		// 	for idx, pp := range mm {
		// 		dps = append(dps, driverPair{driverIndex: idx, platforms: pp})
		// 	}
		// 	m[k] = dps
		// }
		m := splitToDriverPairs(availablePlatforms, opt)
		clients, err := ensureBooted(ctx, drivers, driverIndexes(m), pw)
		if err != nil {
			return nil, nil, err
		}
		return m, clients, nil
	}

	// boot all drivers in k
	clients, err := ensureBooted(ctx, drivers, allIndexes(len(drivers)), pw)
	if err != nil {
		return nil, nil, err
	}

	eg, ctx := errgroup.WithContext(ctx)
	workers := make([][]*client.WorkerInfo, len(clients))

	for i, c := range clients {
		if c == nil {
			continue
		}
		func(i int) {
			eg.Go(func() error {
				ww, err := clients[i].ListWorkers(ctx)
				if err != nil {
					return errors.Wrap(err, "listing workers")
				}
				workers[i] = ww
				return nil
			})
		}(i)
	}

	if err := eg.Wait(); err != nil {
		return nil, nil, err
	}

	for i, ww := range workers {
		for _, w := range ww {
			for _, p := range w.Platforms {
				p = platforms.Normalize(p)
				ps := platforms.Format(p)

				if _, ok := availablePlatforms[ps]; !ok {
					availablePlatforms[ps] = i
				}
			}
		}
	}

	return splitToDriverPairs(availablePlatforms, opt), clients, nil
}

func numDrivers(m map[string][]driverPair) int {
	mm := map[int]struct{}{}
	for _, v := range m {
		for _, dp := range v {
			if _, ok := mm[dp.driverIndex]; !ok {
				mm[dp.driverIndex] = struct{}{}
			}
		}
	}
	return len(mm)
}

func Build(ctx context.Context, drivers []DriverInfo, opt map[string]Options, docker DockerAPI, pw progress.Writer) (map[string]*client.SolveResponse, error) {
	if len(drivers) == 0 {
		return nil, errors.Errorf("driver required for build")
	}

	drivers, err := filterAvailableDrivers(drivers)
	if err != nil {
		return nil, errors.Wrapf(err, "no valid drivers found")
	}

	m, clients, err := resolveDrivers(ctx, drivers, opt, pw)
	if err != nil {
		close(pw.Status())
		<-pw.Done()
		return nil, err
	}

	exportsWarned := false
	multiDriver := numDrivers(m) > 1
	multiTarget := len(opt) > 1
	withPrefix := multiTarget || multiDriver

	mw := progress.NewMultiWriter(pw)

	eg, ctx := errgroup.WithContext(ctx)

	resp := map[string]*client.SolveResponse{}
	var mu sync.Mutex

	var defers []func()
	defer func() {
		for _, f := range defers {
			f()
		}
	}()

	for k, opt := range opt {
		for _, dp := range m[k] {
			err := func() error {
				di := drivers[dp.driverIndex]
				d := di.Driver
				c := clients[dp.driverIndex]
				_, isDefaultMobyDriver := d.(interface {
					IsDefaultMobyDriver()
				})

				if !isDefaultMobyDriver && len(opt.Exports) == 0 && !exportsWarned {
					// TODO
					// logrus.Warnf("No output specified for %s driver. Build result will only remain in the build cache. To push result image into registry use --push or to load image into docker use --load", d.Factory().Name())
					exportsWarned = true
				}

				kk := k
				if multiDriver {
					if multiTarget {
						kk += di.Name + " "
					} else {
						kk = di.Name
					}
				}

				pw := mw.WithPrefix(kk, withPrefix)

				pl := dp.platforms

				if opt.ImageIDFile != "" {
					if len(pl) != 0 {
						return errors.Errorf("image ID file cannot be specified when building for multiple platforms")
					}
					// Avoid leaving a stale file if we eventually fail
					if err := os.Remove(opt.ImageIDFile); err != nil && !os.IsNotExist(err) {
						return errors.Wrap(err, "removing image ID file")
					}
				}

				if v, ok := opt.BuildArgs["BUILDKIT_INLINE_CACHE"]; ok {
					if v, _ := strconv.ParseBool(v); v {
						opt.CacheTo = append(opt.CacheTo, client.CacheOptionsEntry{
							Type:  "inline",
							Attrs: map[string]string{},
						})
					}
				}

				for _, e := range opt.CacheTo {
					if e.Type != "inline" && !d.Features()[driver.CacheExport] {
						return notSupported(d, driver.CacheExport)
					}
				}

				so := client.SolveOpt{
					Frontend:      "dockerfile.v0",
					FrontendAttrs: map[string]string{},
					LocalDirs:     map[string]string{},
					CacheExports:  opt.CacheTo,
					CacheImports:  opt.CacheFrom,
				}

				switch len(opt.Exports) {
				case 1:
					// valid
				case 0:
					if isDefaultMobyDriver {
						// backwards compat for docker driver only:
						// this ensures the build results in a docker image.
						opt.Exports = []client.ExportEntry{{Type: "image", Attrs: map[string]string{}}}
					}
				default:
					return errors.Errorf("multiple outputs currently unsupported")
				}

				if len(opt.Tags) > 0 {
					tags := make([]string, len(opt.Tags))
					for i, tag := range opt.Tags {
						ref, err := reference.Parse(tag)
						if err != nil {
							return errors.Wrapf(err, "invalid tag %q", tag)
						}
						tags[i] = ref.String()
					}
					for i, e := range opt.Exports {
						switch e.Type {
						case "image", "oci", "docker":
							opt.Exports[i].Attrs["name"] = strings.Join(tags, ",")
						}
					}
				} else {
					for _, e := range opt.Exports {
						if e.Type == "image" && e.Attrs["name"] == "" && e.Attrs["push"] != "" {
							if ok, _ := strconv.ParseBool(e.Attrs["push"]); ok {
								return errors.Errorf("tag is needed when pushing to registry")
							}
						}
					}
				}

				for i, e := range opt.Exports {
					if (e.Type == "local" || e.Type == "tar") && opt.ImageIDFile != "" {
						return errors.Errorf("local and tar exporters are incompatible with image ID file")
					}
					if e.Type == "oci" && !d.Features()[driver.OCIExporter] {
						return notSupported(d, driver.OCIExporter)
					}
					if e.Type == "docker" {
						if e.Output == nil {
							if isDefaultMobyDriver {
								e.Type = "image"
							} else {
								w, cancel, err := newDockerLoader(ctx, docker, e.Attrs["context"], mw)
								if err != nil {
									return err
								}
								defers = append(defers, cancel)
								opt.Exports[i].Output = w
							}
						} else if !d.Features()[driver.DockerExporter] {
							return notSupported(d, driver.DockerExporter)
						}
					}
					if e.Type == "image" && isDefaultMobyDriver {
						opt.Exports[i].Type = "moby"
						if e.Attrs["push"] != "" {
							if ok, _ := strconv.ParseBool(e.Attrs["push"]); ok {
								return errors.Errorf("auto-push is currently not implemented for docker driver")
							}
						}
					}
				}

				// TODO: handle loading to docker daemon

				so.Exports = opt.Exports
				so.Session = opt.Session

				release, err := LoadInputs(opt.Inputs, &so)
				if err != nil {
					return err
				}
				defers = append(defers, release)

				if opt.Pull {
					so.FrontendAttrs["image-resolve-mode"] = "pull"
				}
				if opt.Target != "" {
					so.FrontendAttrs["target"] = opt.Target
				}
				if opt.NoCache {
					so.FrontendAttrs["no-cache"] = ""
				}
				for k, v := range opt.BuildArgs {
					so.FrontendAttrs["build-arg:"+k] = v
				}
				for k, v := range opt.Labels {
					so.FrontendAttrs["label:"+k] = v
				}

				if len(pl) != 0 {
					pp := make([]string, len(pl))
					for i, p := range pl {
						pp[i] = platforms.Format(p)
					}
					if len(pp) > 1 && !d.Features()[driver.MultiPlatform] {
						return notSupported(d, driver.MultiPlatform)
					}
					so.FrontendAttrs["platform"] = strings.Join(pp, ",")
				}

				switch opt.NetworkMode {
				case "host", "none":
					so.FrontendAttrs["force-network-mode"] = opt.NetworkMode
				case "", "default":
				default:
					return errors.Errorf("network mode %q not supported by buildkit", opt.NetworkMode)
				}

				extraHosts, err := toBuildkitExtraHosts(opt.ExtraHosts)
				if err != nil {
					return err
				}
				so.FrontendAttrs["add-hosts"] = extraHosts

				var statusCh chan *client.SolveStatus
				if pw != nil {
					pw = progress.ResetTime(pw)
					statusCh = pw.Status()
					eg.Go(func() error {
						<-pw.Done()
						return pw.Err()
					})
				}

				iidfile := opt.ImageIDFile
				eg.Go(func() error {
					rr, err := c.Solve(ctx, nil, so, statusCh)
					if err != nil {
						return err
					}
					mu.Lock()
					resp[k] = rr
					mu.Unlock()
					if iidfile != "" {
						return ioutil.WriteFile(iidfile, []byte(rr.ExporterResponse["containerimage.digest"]), 0644)
					}
					return nil
				})
				return nil
			}()
			if err != nil {
				return nil, err
			}
		}
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return resp, nil
}

func createTempDockerfile(r io.Reader) (string, error) {
	dir, err := ioutil.TempDir("", "dockerfile")
	if err != nil {
		return "", err
	}
	f, err := os.Create(filepath.Join(dir, "Dockerfile"))
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	return dir, err
}

func LoadInputs(inp Inputs, target *client.SolveOpt) (func(), error) {
	if inp.ContextPath == "" {
		return nil, errors.New("please specify build context (e.g. \".\" for the current directory)")
	}

	// TODO: handle stdin, symlinks, remote contexts, check files exist

	var (
		err              error
		dockerfileReader io.Reader
		dockerfileDir    string
		dockerfileName   = inp.DockerfilePath
		toRemove         []string
	)

	switch {
	case inp.ContextPath == "-":
		if inp.DockerfilePath == "-" {
			return nil, errStdinConflict
		}

		buf := bufio.NewReader(os.Stdin)
		magic, err := buf.Peek(archiveHeaderSize * 2)
		if err != nil && err != io.EOF {
			return nil, errors.Wrap(err, "failed to peek context header from STDIN")
		}

		if isArchive(magic) {
			// stdin is context
			up := uploadprovider.New()
			target.FrontendAttrs["context"] = up.Add(buf)
			target.Session = append(target.Session, up)
		} else {
			if inp.DockerfilePath != "" {
				return nil, errDockerfileConflict
			}
			// stdin is dockerfile
			dockerfileReader = buf
			inp.ContextPath, _ = ioutil.TempDir("", "empty-dir")
			toRemove = append(toRemove, inp.ContextPath)
			target.LocalDirs["context"] = inp.ContextPath
		}

	case isLocalDir(inp.ContextPath):
		target.LocalDirs["context"] = inp.ContextPath
		switch inp.DockerfilePath {
		case "-":
			dockerfileReader = os.Stdin
		case "":
			dockerfileDir = inp.ContextPath
		default:
			dockerfileDir = filepath.Dir(inp.DockerfilePath)
			dockerfileName = filepath.Base(inp.DockerfilePath)
		}

	case urlutil.IsGitURL(inp.ContextPath), urlutil.IsURL(inp.ContextPath):
		if inp.DockerfilePath == "-" {
			return nil, errors.Errorf("Dockerfile from stdin is not supported with remote contexts")
		}
		target.FrontendAttrs["context"] = inp.ContextPath
	default:
		return nil, errors.Errorf("unable to prepare context: path %q not found", inp.ContextPath)
	}

	if dockerfileReader != nil {
		dockerfileDir, err = createTempDockerfile(dockerfileReader)
		if err != nil {
			return nil, err
		}
		toRemove = append(toRemove, dockerfileDir)
	}

	if dockerfileName == "" {
		dockerfileName = "Dockerfile"
	}
	target.FrontendAttrs["filename"] = dockerfileName

	if dockerfileDir != "" {
		target.LocalDirs["dockerfile"] = dockerfileDir
	}

	release := func() {
		for _, dir := range toRemove {
			os.RemoveAll(dir)
		}
	}
	return release, nil
}

func notSupported(d driver.Driver, f driver.Feature) error {
	return errors.Errorf("%s feature is currently not supported for %s driver. Please switch to a different driver (eg. \"docker buildx create\")", f, d.Factory().Name())
}

func newDockerLoader(ctx context.Context, d DockerAPI, name string, mw *progress.MultiWriter) (io.WriteCloser, func(), error) {
	c, err := d.DockerAPI(name)
	if err != nil {
		return nil, nil, err
	}

	pr, pw := io.Pipe()
	started := make(chan struct{})
	w := &waitingWriter{
		PipeWriter: pw,
		f: func() {
			resp, err := c.ImageLoad(ctx, pr, false)
			if err != nil {
				pr.CloseWithError(err)
				return
			}
			prog := mw.WithPrefix("", false)
			close(started)
			progress.FromReader(prog, "importing to docker", resp.Body)
		},
		started: started,
	}
	return w, func() {
		pr.Close()
	}, nil
}

type waitingWriter struct {
	*io.PipeWriter
	f       func()
	once    sync.Once
	mu      sync.Mutex
	err     error
	started chan struct{}
}

func (w *waitingWriter) Write(dt []byte) (int, error) {
	w.once.Do(func() {
		go w.f()
	})
	return w.PipeWriter.Write(dt)
}

func (w *waitingWriter) Close() error {
	err := w.PipeWriter.Close()
	<-w.started
	return err
}
