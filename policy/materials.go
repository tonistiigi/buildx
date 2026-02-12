package policy

import (
	"maps"
	"path"
	"strings"

	"github.com/distribution/reference"
	"github.com/docker/buildx/util/urlutil"
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/gitutil"
	"github.com/moby/buildkit/util/purl"
	"github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	packageurl "github.com/package-url/packageurl-go"
	"github.com/pkg/errors"
)

type materialSource struct {
	URI      string
	Source   *pb.SourceOp
	Platform *ocispecs.Platform
	Digest   map[string]string
}

func materialSourceFromURI(uri string, dgst map[string]string) (*materialSource, error) {
	if p, err := packageurl.FromString(uri); err == nil {
		if p.Type == "docker" {
			return dockerMaterialSource(uri, dgst)
		}
		return nil, errors.Errorf("unsupported material URI %q", uri)
	}

	if gu, err := gitutil.ParseURL(uri); err == nil {
		if strings.HasSuffix(strings.ToLower(gu.Path), ".git") || gu.Scheme != gitutil.HTTPProtocol && gu.Scheme != gitutil.HTTPSProtocol {
			return gitMaterialSource(uri, dgst)
		}
	}

	if urlutil.IsHTTPURL(uri) {
		return &materialSource{
			URI:    uri,
			Source: &pb.SourceOp{Identifier: uri},
			Digest: maps.Clone(dgst),
		}, nil
	}

	return nil, errors.Errorf("unsupported material URI %q", uri)
}

func dockerMaterialSource(uri string, dgst map[string]string) (*materialSource, error) {
	ref, platform, err := purl.PURLToRef(uri)
	if err != nil {
		return nil, err
	}

	named, err := reference.ParseNormalizedNamed(ref)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid docker reference %q from %q", ref, uri)
	}
	if checksum := strings.TrimSpace(dgst["sha256"]); checksum != "" {
		dgstRef, err := digest.Parse(checksum)
		if err != nil {
			dgstRef, err = digest.Parse("sha256:" + checksum)
		}
		if err != nil {
			return nil, errors.Wrapf(err, "invalid material digest %q for %q", checksum, uri)
		}
		if canonical, ok := named.(reference.Canonical); ok {
			if canonical.Digest() != dgstRef {
				return nil, errors.Errorf("material digest mismatch for %q: ref has %s but provenance has %s", uri, canonical.Digest(), dgstRef)
			}
		} else {
			named, err = reference.WithDigest(named, dgstRef)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to add digest %q to %q", dgstRef, ref)
			}
		}
	}

	return &materialSource{
		URI:      uri,
		Platform: platform,
		Digest:   maps.Clone(dgst),
		Source: &pb.SourceOp{
			Identifier: "docker-image://" + named.String(),
		},
	}, nil
}

func gitMaterialSource(uri string, dgst map[string]string) (*materialSource, error) {
	gu, err := gitutil.ParseURL(uri)
	if err != nil {
		return nil, err
	}

	switch gu.Scheme {
	case gitutil.HTTPSProtocol, gitutil.HTTPProtocol, gitutil.SSHProtocol, gitutil.GitProtocol:
	default:
		return nil, errors.Errorf("unsupported git material URI %q", uri)
	}

	// Match BuildKit git source ID normalization so different transports
	// resolve to the same identity while preserving original URL in attrs.
	id := gu.Host + path.Join("/", gu.Path)
	if gu.Opts != nil && (gu.Opts.Ref != "" || gu.Opts.Subdir != "") {
		id += "#" + gu.Opts.Ref
		if gu.Opts.Subdir != "" {
			id += ":" + gu.Opts.Subdir
		}
	}

	return &materialSource{
		URI:    uri,
		Source: &pb.SourceOp{
			Identifier: "git://" + id,
			Attrs: map[string]string{
				pb.AttrFullRemoteURL: uri,
			},
		},
		Digest: maps.Clone(dgst),
	}, nil
}
