package policy

import (
	"context"
	"encoding/json"
	"testing"

	slsa1 "github.com/in-toto/in-toto-golang/in_toto/slsa_provenance/v1"
	gwpb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/opencontainers/go-digest"
	ocispecs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
)

func TestInputGraphWithProvenanceMaterials(t *testing.T) {
	src := &gwpb.ResolveSourceMetaResponse{
		Source: &pb.SourceOp{
			Identifier: "docker-image://alpine:latest",
		},
		Image: &gwpb.ResolveSourceImageResponse{
			Digest:           "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			AttestationChain: testAttestationChainWithMaterials(t),
		},
	}
	graph, err := NewInputGraph(t.Context(), nil, src, &ocispecs.Platform{OS: "linux", Architecture: "amd64"}, nil)
	require.NoError(t, err)

	inp := graph.Input()
	require.Equal(t, 0, inp.Env.Depth)
	require.NotNil(t, inp.Image)
	require.NotNil(t, inp.Image.Provenance)
	require.Len(t, inp.Image.Provenance.Materials, 2)
	require.Equal(t, 1, inp.Image.Provenance.Materials[0].Env.Depth)
	require.Equal(t, 1, inp.Image.Provenance.Materials[1].Env.Depth)
	require.NotNil(t, inp.Image.Provenance.Materials[0].Image)
	require.NotNil(t, inp.Image.Provenance.Materials[1].Git)
	require.Equal(t, "9836771d0c5b21cbc7f0c38b81be39c42fc46b7b", inp.Image.Provenance.Materials[1].Git.Checksum)
	require.Empty(t, inp.Image.Provenance.Materials[1].Git.CommitChecksum)

	unknowns := graph.Unknowns()
	require.Contains(t, unknowns, "input.image.provenance.materials[0].image.createdTime")
	require.NotContains(t, unknowns, "input.image.provenance.materials[1].git.checksum")
	require.Contains(t, unknowns, "input.image.provenance.materials[1].git.commitChecksum")

	targetPath, req, err := graph.PlanResolveRequest([]string{"image.provenance.materials[0].image.createdTime"})
	require.NoError(t, err)
	require.Equal(t, "materials[0]", targetPath)
	require.NotNil(t, req)
	require.NotNil(t, req.Source)
	require.Equal(t, "docker-image://docker.io/library/alpine:3.23@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659", req.Source.Identifier)
	require.NotNil(t, req.Image)
	require.False(t, req.Image.NoConfig)
}

func testAttestationChainWithMaterials(t *testing.T) *gwpb.AttestationChain {
	t.Helper()

	provenanceBytes, err := json.Marshal(map[string]any{
		"buildDefinition": map[string]any{
			"buildType": "https://example.com/build-type-v1",
			"externalParameters": map[string]any{
				"configSource": map[string]any{
					"uri": "https://github.com/moby/buildkit.git#refs/heads/master",
				},
				"request": map[string]any{
					"frontend": "gateway.v0",
				},
			},
			"internalParameters": map[string]any{},
			"resolvedDependencies": []map[string]any{
				{
					"uri": "pkg:docker/alpine@3.23?platform=linux%2Famd64",
					"digest": map[string]any{
						"sha256": "25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659",
					},
				},
				{
					"uri": "https://github.com/moby/buildkit.git#refs/heads/master",
					"digest": map[string]any{
						"sha1": "9836771d0c5b21cbc7f0c38b81be39c42fc46b7b",
					},
				},
			},
		},
		"runDetails": map[string]any{
			"builder": map[string]any{
				"id": "https://example.com/builder-id-v1",
			},
		},
	})
	require.NoError(t, err)
	provenanceDigest := digest.FromBytes(provenanceBytes)

	return &gwpb.AttestationChain{
		AttestationManifest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Blobs: map[string]*gwpb.Blob{
			provenanceDigest.String(): {
				Descriptor_: &gwpb.Descriptor{
					MediaType: "application/vnd.in-toto+json",
					Digest:    provenanceDigest.String(),
					Size:      int64(len(provenanceBytes)),
					Annotations: map[string]string{
						predicateTypeAnnotation: slsa1.PredicateSLSAProvenance,
					},
				},
				Data: provenanceBytes,
			},
		},
	}
}

func TestInputGraphApplyResponseAndRebuild(t *testing.T) {
	src := &gwpb.ResolveSourceMetaResponse{
		Source: &pb.SourceOp{Identifier: "docker-image://alpine:latest"},
		Image: &gwpb.ResolveSourceImageResponse{
			Digest:           "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			AttestationChain: testAttestationChainWithMaterials(t),
		},
	}
	graph, err := NewInputGraph(context.Background(), nil, src, &ocispecs.Platform{OS: "linux", Architecture: "amd64"}, nil)
	require.NoError(t, err)

	resp := &gwpb.ResolveSourceMetaResponse{
		Source: &pb.SourceOp{Identifier: "docker-image://docker.io/library/alpine:3.23@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659"},
		Image: &gwpb.ResolveSourceImageResponse{
			Digest: "sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659",
			Config: mustMarshalImageConfig(t, ocispecs.Image{
				Config: ocispecs.ImageConfig{
					User: "root",
				},
			}),
		},
	}

	require.NoError(t, graph.ApplyResponse("materials[0]", resp))
	require.NoError(t, graph.Rebuild(context.Background()))

	inp := graph.Input()
	require.Equal(t, "root", inp.Image.Provenance.Materials[0].Image.User)
}

func TestApplyGitDigestHintSHA256(t *testing.T) {
	inp := &Input{
		Git: &Git{},
	}
	unknowns := []string{
		"input.git.checksum",
		"input.git.commitChecksum",
		"input.git.isSHA256",
		"input.git.ref",
	}
	dgst := map[string]string{
		"sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}

	got := applyGitDigestHint(inp, unknowns, dgst)
	require.Equal(t, dgst["sha256"], inp.Git.Checksum)
	require.Empty(t, inp.Git.CommitChecksum)
	require.True(t, inp.Git.IsSHA256)
	require.NotContains(t, got, "input.git.checksum")
	require.Contains(t, got, "input.git.commitChecksum")
	require.NotContains(t, got, "input.git.isSHA256")
	require.Contains(t, got, "input.git.ref")
}

func TestApplyHTTPDigestHint(t *testing.T) {
	inp := &Input{
		HTTP: &HTTP{},
	}
	unknowns := []string{
		"input.http.checksum",
	}
	dgst := map[string]string{
		"sha256": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}

	got := applyHTTPDigestHint(inp, unknowns, dgst)
	require.Equal(t, "sha256:"+dgst["sha256"], inp.HTTP.Checksum)
	require.NotContains(t, got, "input.http.checksum")
}

func TestApplyHTTPDigestHintIgnoresNonSHA256(t *testing.T) {
	inp := &Input{
		HTTP: &HTTP{},
	}
	unknowns := []string{
		"input.http.checksum",
	}
	dgst := map[string]string{
		"sha1": "cccccccccccccccccccccccccccccccccccccccc",
	}

	got := applyHTTPDigestHint(inp, unknowns, dgst)
	require.Empty(t, inp.HTTP.Checksum)
	require.Contains(t, got, "input.http.checksum")
}
