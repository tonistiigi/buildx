package policy

import (
	"testing"

	policylib "github.com/docker/buildx/policy"
	gwpb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/solver/pb"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

func TestSourceResolverOptIncludesResolveAttestations(t *testing.T) {
	req := &gwpb.ResolveSourceMetaRequest{
		ResolveMode: "default",
		Image: &gwpb.ResolveSourceImageRequest{
			NoConfig:            true,
			ResolveAttestations: []string{"https://slsa.dev/provenance/v0.2"},
		},
	}
	platform := &ocispecs.Platform{OS: "linux", Architecture: "amd64"}

	opt := sourceResolverOpt(req, platform)
	require.NotNil(t, opt.ImageOpt)
	require.True(t, opt.ImageOpt.NoConfig)
	require.Equal(t, []string{"https://slsa.dev/provenance/v0.2"}, opt.ImageOpt.ResolveAttestations)
	require.Equal(t, "default", opt.ImageOpt.ResolveMode)
	require.Equal(t, platform, opt.ImageOpt.Platform)
}

func TestSourceResolverOptUsesRequestPlatformOverride(t *testing.T) {
	req := &gwpb.ResolveSourceMetaRequest{
		Image: &gwpb.ResolveSourceImageRequest{
			NoConfig: true,
		},
		Platform: &pb.Platform{
			OS:           "linux",
			Architecture: "arm64",
		},
	}
	defaultPlatform := &ocispecs.Platform{OS: "linux", Architecture: "amd64"}

	opt := sourceResolverOpt(req, defaultPlatform)
	require.NotNil(t, opt.ImageOpt)
	require.Equal(t, "arm64", opt.ImageOpt.Platform.Architecture)
}

func TestSelectReloadFieldsUsesUnknownAncestor(t *testing.T) {
	reload, invalid := selectReloadFields(
		[]string{"image.provenance.materials[0].image.createdTime"},
		[]string{"image.provenance"},
	)
	require.Equal(t, []string{"image.provenance"}, reload)
	require.Empty(t, invalid)
}

func TestSelectReloadFieldsMarksInvalidAfterExpansion(t *testing.T) {
	reload, invalid := selectReloadFields(
		[]string{"image.provenance.materials[0].creationTime"},
		[]string{"image.provenance.materials[0].image.createdTime"},
	)
	require.Empty(t, reload)
	require.Equal(t, []string{"image.provenance.materials[0].creationTime"}, invalid)
}

func TestFilterInvalidFields(t *testing.T) {
	in := policylib.Input{
		Image: &policylib.Image{
			Provenance: &policylib.ImageProvenance{
				Materials: []policylib.Input{
					{Image: &policylib.Image{CreatedTime: "2026-01-01T00:00:00Z"}},
				},
			},
		},
	}
	got := filterInvalidFields(in, []string{
		"image.provenance.materials[0].image.createdTime",
		"image.provenance.materials[0].creationTime",
	})
	require.Equal(t, []string{"image.provenance.materials[0].creationTime"}, got)
}

func TestSummarizeEvalUnknownsWithRequestedFields(t *testing.T) {
	in := policylib.Input{
		Image: &policylib.Image{
			Provenance: &policylib.ImageProvenance{
				Materials: []policylib.Input{
					{Image: &policylib.Image{CreatedTime: "2026-01-01T00:00:00Z"}},
				},
			},
		},
	}
	unknowns := []string{
		"image.provenance.materials[0].image.signatures",
		"image.provenance.materials[0].image.provenance.materials[4].git.commit",
		"image.provenance.materials[0].image.labels",
	}
	got := summarizeEvalUnknowns(in, unknowns, []string{
		"image.provenance.materials[0].image.signatures",
		"image.provenance.materials[0].image.provenance.materials[4].git.commit",
	})
	require.Equal(t, []string{
		"image.provenance.materials[0].image.signatures",
		"image.provenance.materials[0].image.provenance.materials[4].git.commit",
	}, got)
}

func TestSummarizeEvalUnknownsWithoutRequestedFields(t *testing.T) {
	got := summarizeEvalUnknowns(policylib.Input{}, []string{
		"image.provenance.materials[0].image.signatures",
		"image.provenance.materials[9].git.commit",
		"image.labels",
		"http.checksum",
	}, nil)
	require.Equal(t, []string{
		"image.provenance.materials",
		"image.labels",
		"http.checksum",
	}, got)
}
