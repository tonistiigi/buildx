package imagetools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/platforms"
	"github.com/docker/distribution/reference"
	"github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
)

const defaultPfx = "  "

type Printer struct {
	ctx      context.Context
	resolver *Resolver

	name   string
	format string

	res      *result
	raw      []byte
	ref      reference.Named
	manifest ocispecs.Descriptor
	index    ocispecs.Index
}

func NewPrinter(ctx context.Context, opt Opt, name string, format string) (*Printer, error) {
	resolver := New(opt)

	res, err := newLoader(resolver.resolver()).Load(ctx, name)
	if err != nil {
		return nil, err
	}

	ref, err := parseRef(name)
	if err != nil {
		return nil, err
	}

	dt, mfst, err := resolver.Get(ctx, ref.String())
	if err != nil {
		return nil, err
	}

	var idx ocispecs.Index
	if err = json.Unmarshal(dt, &idx); err != nil {
		return nil, err
	}

	return &Printer{
		ctx:      ctx,
		resolver: resolver,
		name:     name,
		format:   format,
		res:      res,
		raw:      dt,
		ref:      ref,
		manifest: mfst,
		index:    idx,
	}, nil
}

func (p *Printer) Print(raw bool, out io.Writer) error {
	if raw {
		_, err := fmt.Fprintf(out, "%s", p.raw) // avoid newline to keep digest
		return err
	}

	if p.format == "" {
		w := tabwriter.NewWriter(out, 0, 0, 1, ' ', 0)
		_, _ = fmt.Fprintf(w, "Name:\t%s\n", p.ref.String())
		_, _ = fmt.Fprintf(w, "MediaType:\t%s\n", p.manifest.MediaType)
		_, _ = fmt.Fprintf(w, "Digest:\t%s\n", p.manifest.Digest)
		_ = w.Flush()
		switch p.manifest.MediaType {
		case images.MediaTypeDockerSchema2ManifestList, ocispecs.MediaTypeImageIndex:
			if err := p.printManifestList(out); err != nil {
				return err
			}
		}
		return nil
	}

	tpl, err := template.New("").Funcs(template.FuncMap{
		"json": func(v interface{}) string {
			b, _ := json.MarshalIndent(v, "", "  ")
			return string(b)
		},
	}).Parse(p.format)
	if err != nil {
		return err
	}

	imageconfigs := p.res.Configs()
	provenances := p.res.Provenances()
	sboms := p.res.SBOMs()
	format := tpl.Root.String()

	var mfst interface{}
	switch p.manifest.MediaType {
	case images.MediaTypeDockerSchema2Manifest, ocispecs.MediaTypeImageManifest:
		mfst = p.manifest
	case images.MediaTypeDockerSchema2ManifestList, ocispecs.MediaTypeImageIndex:
		mfst = struct {
			SchemaVersion int                   `json:"schemaVersion"`
			MediaType     string                `json:"mediaType,omitempty"`
			Digest        digest.Digest         `json:"digest"`
			Size          int64                 `json:"size"`
			Manifests     []ocispecs.Descriptor `json:"manifests"`
			Annotations   map[string]string     `json:"annotations,omitempty"`
		}{
			SchemaVersion: p.index.Versioned.SchemaVersion,
			MediaType:     p.index.MediaType,
			Digest:        p.manifest.Digest,
			Size:          p.manifest.Size,
			Manifests:     p.index.Manifests,
			Annotations:   p.index.Annotations,
		}
	}

	switch {
	// TODO: print formatted config
	case strings.HasPrefix(format, "{{.Manifest"), strings.HasPrefix(format, "{{.BuildInfo"), strings.HasPrefix(format, "{{.Provenance"):
		w := tabwriter.NewWriter(out, 0, 0, 1, ' ', 0)
		_, _ = fmt.Fprintf(w, "Name:\t%s\n", p.ref.String())
		switch {
		case strings.HasPrefix(format, "{{.Manifest"):
			_, _ = fmt.Fprintf(w, "MediaType:\t%s\n", p.manifest.MediaType)
			_, _ = fmt.Fprintf(w, "Digest:\t%s\n", p.manifest.Digest)
			_ = w.Flush()
			switch p.manifest.MediaType {
			case images.MediaTypeDockerSchema2ManifestList, ocispecs.MediaTypeImageIndex:
				_ = p.printManifestList(out)
			}
		case strings.HasPrefix(format, "{{.BuildInfo"), strings.HasPrefix(format, "{{.Provenance"):
			_ = w.Flush()
			_ = p.printProvenances(provenances, out)
		}
	default:
		if len(p.res.platforms) > 1 {
			return tpl.Execute(out, struct {
				Name       string                     `json:"name,omitempty"`
				Manifest   interface{}                `json:"manifest,omitempty"`
				Image      map[string]*ocispecs.Image `json:"image,omitempty"`
				Provenance map[string]json.RawMessage `json:"SLSA,omitempty"`
				SBOM       map[string]json.RawMessage `json:"SPDX,omitempty"`
			}{
				Name:       p.name,
				Manifest:   mfst,
				Image:      imageconfigs,
				Provenance: provenances,
				SBOM:       sboms,
			})
		}
		var ic *ocispecs.Image
		for _, v := range imageconfigs {
			ic = v
		}
		var prv json.RawMessage
		for _, v := range provenances {
			prv = v
		}
		var sbom json.RawMessage
		for _, v := range sboms {
			sbom = v
		}
		return tpl.Execute(out, struct {
			Name       string          `json:"name,omitempty"`
			Manifest   interface{}     `json:"manifest,omitempty"`
			Image      *ocispecs.Image `json:"image,omitempty"`
			Provenance json.RawMessage `json:"provenance,omitempty"`
			SBOM       json.RawMessage `json:"sbom,omitempty"`
		}{
			Name:       p.name,
			Manifest:   mfst,
			Image:      ic,
			Provenance: prv,
			SBOM:       sbom,
		})
	}

	return nil
}

func (p *Printer) printManifestList(out io.Writer) error {
	w := tabwriter.NewWriter(out, 0, 0, 1, ' ', 0)
	_, _ = fmt.Fprintf(w, "\t\n")
	_, _ = fmt.Fprintf(w, "Manifests:\t\n")
	_ = w.Flush()

	w = tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
	for i, m := range p.index.Manifests {
		if i != 0 {
			_, _ = fmt.Fprintf(w, "\t\n")
		}
		cr, err := reference.WithDigest(p.ref, m.Digest)
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintf(w, "%sName:\t%s\n", defaultPfx, cr.String())
		_, _ = fmt.Fprintf(w, "%sMediaType:\t%s\n", defaultPfx, m.MediaType)
		if p := m.Platform; p != nil {
			_, _ = fmt.Fprintf(w, "%sPlatform:\t%s\n", defaultPfx, platforms.Format(*p))
			if p.OSVersion != "" {
				_, _ = fmt.Fprintf(w, "%sOSVersion:\t%s\n", defaultPfx, p.OSVersion)
			}
			if len(p.OSFeatures) > 0 {
				_, _ = fmt.Fprintf(w, "%sOSFeatures:\t%s\n", defaultPfx, strings.Join(p.OSFeatures, ", "))
			}
			if len(m.URLs) > 0 {
				_, _ = fmt.Fprintf(w, "%sURLs:\t%s\n", defaultPfx, strings.Join(m.URLs, ", "))
			}
			if len(m.Annotations) > 0 {
				_, _ = fmt.Fprintf(w, "%sAnnotations:\t\n", defaultPfx)
				_ = w.Flush()
				w2 := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
				for k, v := range m.Annotations {
					_, _ = fmt.Fprintf(w2, "%s%s:\t%s\n", defaultPfx+defaultPfx, k, v)
				}
				_ = w2.Flush()
			}
		}
	}
	return w.Flush()
}

func (p *Printer) printProvenances(provenances map[string]json.RawMessage, out io.Writer) error {
	if len(provenances) == 0 {
		return nil
	} else if len(provenances) == 1 {
		for _, pr := range provenances {
			return p.printProvenance(pr, "", out)
		}
	}
	pkeys := append([]string{}, p.res.platforms...)

	sort.Strings(pkeys)
	for _, platform := range pkeys {
		if pr, ok := provenances[platform]; ok {
			w := tabwriter.NewWriter(out, 0, 0, 1, ' ', 0)
			_, _ = fmt.Fprintf(w, "\t\nPlatform:\t%s\t\n", platform)
			_ = w.Flush()
			if err := p.printProvenance(pr, "", out); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Printer) printProvenance(pr json.RawMessage, pfx string, out io.Writer) error {
	_, err := out.Write(pr)
	return err
}
