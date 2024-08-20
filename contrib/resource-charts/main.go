package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"slices"
	"text/tabwriter"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/content/proxy"
	"github.com/docker/cli/cli/connhelper/commandconn"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	controlapi "github.com/moby/buildkit/api/services/control"
	"github.com/moby/buildkit/client"
	provenancetypes "github.com/moby/buildkit/solver/llbsolver/provenance/types"
	"github.com/moby/buildkit/util/appcontext"
	"github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

func main() {
	if err := run(appcontext.Context()); err != nil {
		log.Printf("error: %+v", err)
		os.Exit(1)
	}
}

type flags struct {
	output string
}

func run(ctx context.Context) error {
	var opts flags
	flag.StringVar(&opts.output, "output", "charts.html", "output file")

	flag.Parse()

	args := flag.Args()

	if len(args) == 1 && args[0] == "list" {
		return list(ctx)
	}

	if len(args) == 2 && args[0] == "chart" {
		return chart(ctx, args[1], opts)
	}

	return errors.Errorf("invalid arguments: %v", args)
}

func chart(ctx context.Context, ref string, flags flags) error {
	var dt []byte

	if ref == "-" {
		var err error
		dt, err = io.ReadAll(os.Stdin)
		if err != nil {
			return errors.Wrap(err, "failed to read stdin")
		}
	} else {
		c, err := newClient(ctx)
		if err != nil {
			return err
		}

		rec, err := records(ctx, c, ref)
		if err != nil {
			return err
		}

		if len(rec) == 0 {
			return errors.Errorf("no records found for ref %s", ref)
		}

		dgst, _, err := findProvenance(rec[0])
		if err != nil {
			return err
		}

		store := proxy.NewContentStore(c.ContentClient())
		dt, err = content.ReadBlob(ctx, store, ocispecs.Descriptor{Digest: dgst})
		if err != nil {
			return errors.Wrap(err, "failed to read provenance")
		}
	}

	var pred provenancetypes.ProvenancePredicate
	if err := json.Unmarshal(dt, &pred); err != nil {
		return errors.Wrap(err, "failed to unmarshal provenance")
	}

	if pred.Metadata == nil || pred.Metadata.BuildKitMetadata.SysUsage == nil {
		return errors.New("no sysusage found")
	}

	sysUsage := pred.Metadata.BuildKitMetadata.SysUsage

	type psiData struct {
		cpuSome    []opts.LineData
		cpuFull    []opts.LineData
		memorySome []opts.LineData
		memoryFull []opts.LineData
		ioSome     []opts.LineData
		ioFull     []opts.LineData
	}

	var avg10Data psiData
	var avg60Data psiData

	for _, s := range sysUsage {
		if s.CPUPressure != nil {
			avg10Data.cpuSome = append(avg10Data.cpuSome, opts.LineData{Value: s.CPUPressure.Some.Avg10})
			avg60Data.cpuSome = append(avg60Data.cpuSome, opts.LineData{Value: s.CPUPressure.Some.Avg60})
			avg10Data.cpuFull = append(avg10Data.cpuFull, opts.LineData{Value: s.CPUPressure.Full.Avg10})
			avg60Data.cpuFull = append(avg60Data.cpuFull, opts.LineData{Value: s.CPUPressure.Full.Avg60})
		}
		if s.MemoryPressure != nil {
			avg10Data.memorySome = append(avg10Data.memorySome, opts.LineData{Value: s.MemoryPressure.Some.Avg10})
			avg60Data.memorySome = append(avg60Data.memorySome, opts.LineData{Value: s.MemoryPressure.Some.Avg60})
			avg10Data.memoryFull = append(avg10Data.memoryFull, opts.LineData{Value: s.MemoryPressure.Full.Avg10})
			avg60Data.memoryFull = append(avg60Data.memoryFull, opts.LineData{Value: s.MemoryPressure.Full.Avg60})
		}
		if s.IOPressure != nil {
			avg10Data.ioSome = append(avg10Data.ioSome, opts.LineData{Value: s.IOPressure.Some.Avg10})
			avg60Data.ioSome = append(avg60Data.ioSome, opts.LineData{Value: s.IOPressure.Some.Avg60})
			avg10Data.ioFull = append(avg10Data.ioFull, opts.LineData{Value: s.IOPressure.Full.Avg10})
			avg60Data.ioFull = append(avg60Data.ioFull, opts.LineData{Value: s.IOPressure.Full.Avg60})
		}
	}

	type cpuData struct {
		user   []opts.LineData
		nice   []opts.LineData
		system []opts.LineData
		idle   []opts.LineData
		iowait []opts.LineData
	}

	var cpu cpuData

	for _, s := range sysUsage {
		if s.CPUStat != nil {
			cpu.user = append(cpu.user, opts.LineData{Value: s.CPUStat.User})
			cpu.nice = append(cpu.nice, opts.LineData{Value: s.CPUStat.Nice})
			cpu.system = append(cpu.system, opts.LineData{Value: s.CPUStat.System})
			cpu.idle = append(cpu.idle, opts.LineData{Value: s.CPUStat.Idle})
			cpu.iowait = append(cpu.iowait, opts.LineData{Value: s.CPUStat.Iowait})
		}
	}

	type procData struct {
		contextSwitches  []opts.LineData
		processCreated   []opts.LineData
		processesRunning []opts.LineData
	}

	var proc procData

	for idx, s := range sysUsage {
		if s.ProcStat != nil {
			if idx > 0 {
				proc.contextSwitches = append(proc.contextSwitches, opts.LineData{Value: float64(s.ProcStat.ContextSwitches - sysUsage[idx-1].ProcStat.ContextSwitches)})
				proc.processCreated = append(proc.processCreated, opts.LineData{Value: float64(s.ProcStat.ProcessCreated - sysUsage[idx-1].ProcStat.ProcessCreated)})
			} else {
				proc.contextSwitches = append(proc.contextSwitches, opts.LineData{})
				proc.processCreated = append(proc.processCreated, opts.LineData{})
			}
			proc.processesRunning = append(proc.processesRunning, opts.LineData{Value: float64(s.ProcStat.ProcessesRunning)})
		}
	}

	type memoryData struct {
		total     []opts.LineData
		free      []opts.LineData
		available []opts.LineData
		buffers   []opts.LineData
		cached    []opts.LineData
		active    []opts.LineData
		inactive  []opts.LineData
		swap      []opts.LineData
		dirty     []opts.LineData
		writeback []opts.LineData
		slab      []opts.LineData
	}

	var memory memoryData

	for _, s := range sysUsage {
		if s.MemoryStat != nil {
			memory.total = append(memory.total, opts.LineData{Value: float64(*s.MemoryStat.Total)})
			memory.free = append(memory.free, opts.LineData{Value: float64(*s.MemoryStat.Free)})
			memory.available = append(memory.available, opts.LineData{Value: float64(*s.MemoryStat.Available)})
			memory.buffers = append(memory.buffers, opts.LineData{Value: float64(*s.MemoryStat.Buffers)})
			memory.cached = append(memory.cached, opts.LineData{Value: float64(*s.MemoryStat.Cached)})
			memory.active = append(memory.active, opts.LineData{Value: float64(*s.MemoryStat.Active)})
			memory.inactive = append(memory.inactive, opts.LineData{Value: float64(*s.MemoryStat.Inactive)})
			memory.swap = append(memory.swap, opts.LineData{Value: float64(*s.MemoryStat.Swap)})
			memory.dirty = append(memory.dirty, opts.LineData{Value: float64(*s.MemoryStat.Dirty)})
			memory.writeback = append(memory.writeback, opts.LineData{Value: float64(*s.MemoryStat.Writeback)})
			memory.slab = append(memory.slab, opts.LineData{Value: float64(*s.MemoryStat.Slab)})
		}
	}

	xAxis := []string{}
	for _, s := range sysUsage {
		xAxis = append(xAxis, s.Timestamp_.Format("15:04:05"))
	}

	avg10 := charts.NewLine()
	avg10.SetGlobalOptions(charts.WithTitleOpts(opts.Title{Title: "PSI Avg10"}))
	avg10.SetXAxis(xAxis).
		AddSeries("CPU Some", avg10Data.cpuSome).
		AddSeries("CPU Full", avg10Data.cpuFull).
		AddSeries("Memory Some", avg10Data.memorySome).
		AddSeries("Memory Full", avg10Data.memoryFull).
		AddSeries("IO Some", avg10Data.ioSome).
		AddSeries("IO Full", avg10Data.ioFull)

	avg60 := charts.NewLine()
	avg60.SetGlobalOptions(charts.WithTitleOpts(opts.Title{Title: "PSI Avg60"}))
	avg60.SetXAxis(xAxis).
		AddSeries("CPU Some", avg60Data.cpuSome).
		AddSeries("CPU Full", avg60Data.cpuFull).
		AddSeries("Memory Some", avg60Data.memorySome).
		AddSeries("Memory Full", avg60Data.memoryFull).
		AddSeries("IO Some", avg60Data.ioSome).
		AddSeries("IO Full", avg60Data.ioFull)

	cpuChart := charts.NewLine()
	cpuChart.SetGlobalOptions(charts.WithTitleOpts(opts.Title{Title: "CPU"}))
	cpuChart.SetXAxis(xAxis).
		AddSeries("User", cpu.user).
		AddSeries("Nice", cpu.nice).
		AddSeries("System", cpu.system).
		AddSeries("Idle", cpu.idle).
		AddSeries("Iowait", cpu.iowait)
	cpuChart.SetGlobalOptions(charts.WithLegendOpts(opts.Legend{Selected: map[string]bool{"Idle": false}}))

	procChart := charts.NewLine()
	procChart.SetGlobalOptions(charts.WithTitleOpts(opts.Title{Title: "Processes"}))
	procChart.SetXAxis(xAxis).
		AddSeries("Context Switches", proc.contextSwitches).
		AddSeries("Process Created", proc.processCreated).
		AddSeries("Processes Running", proc.processesRunning)

	memoryChart := charts.NewLine()
	memoryChart.SetGlobalOptions(charts.WithTitleOpts(opts.Title{Title: "Memory"}))
	memoryChart.SetXAxis(xAxis).
		AddSeries("Total", memory.total).
		AddSeries("Free", memory.free).
		AddSeries("Available", memory.available).
		AddSeries("Buffers", memory.buffers).
		AddSeries("Cached", memory.cached).
		AddSeries("Active", memory.active).
		AddSeries("Inactive", memory.inactive).
		AddSeries("Swap", memory.swap).
		AddSeries("Dirty", memory.dirty).
		AddSeries("Writeback", memory.writeback).
		AddSeries("Slab", memory.slab)
	memoryChart.SetGlobalOptions(charts.WithLegendOpts(opts.Legend{Selected: map[string]bool{
		"Total":     false,
		"Dirty":     false,
		"Writeback": false,
		"Slab":      false,
	}}))

	f, err := os.Create(flags.output)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	page := components.NewPage()
	page.AddCharts(avg10, avg60, cpuChart, procChart, memoryChart)
	page.Render(f)

	return nil
}

func findProvenance(rec *controlapi.BuildHistoryRecord) (digest.Digest, bool, error) {
	if rec.Result != nil {
		for _, att := range rec.Result.Attestations {
			if att.Annotations["in-toto.io/predicate-type"] == "https://slsa.dev/provenance/v0.2" {
				return att.Digest, false, nil
			}
		}
	}
	for _, res := range rec.Results {
		for _, att := range res.Attestations {
			if att.Annotations["in-toto.io/predicate-type"] == "https://slsa.dev/provenance/v0.2" {
				return att.Digest, len(rec.Results) > 1, nil
			}
		}
	}
	return "", false, errors.New("no provenance found")
}

func newClient(ctx context.Context) (*client.Client, error) {
	c, err := client.New(ctx, "", client.WithContextDialer(stdioDialer))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client")
	}
	return c, nil
}

func records(ctx context.Context, c *client.Client, ref string) ([]*controlapi.BuildHistoryRecord, error) {

	cl, err := c.ControlClient().ListenBuildHistory(ctx, &controlapi.BuildHistoryRequest{
		EarlyExit: true,
		Ref:       ref,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to listen build history")
	}

	var records []*controlapi.BuildHistoryRecord

loop0:
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			ev, err := cl.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break loop0
				}
				return nil, errors.Wrap(err, "failed to receive event")
			}

			if ev.Record != nil {
				records = append(records, ev.Record)
			}
		}
	}

	return records, nil
}

func list(ctx context.Context) error {
	c, err := newClient(ctx)
	if err != nil {
		return err
	}

	records, err := records(ctx, c, "")
	if err != nil {
		return err
	}

	slices.SortFunc(records, func(a, b *controlapi.BuildHistoryRecord) int {
		return b.CreatedAt.Compare(*a.CreatedAt)
	})

	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	fmt.Fprintln(tw, "REF\tCREATED AT\tSUCCESS")
	for _, r := range records {
		fmt.Fprintf(tw, "%s\t%s\t%v\n", r.Ref, r.CreatedAt, r.Error == nil)
	}
	tw.Flush()

	return nil
}

func stdioDialer(ctx context.Context, address string) (net.Conn, error) {
	return commandconn.New(ctx, "docker", "buildx", "dial-stdio")
}
