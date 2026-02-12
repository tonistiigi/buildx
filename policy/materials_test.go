package policy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaterialSourceFromURI(t *testing.T) {
	t.Run("docker-purl", func(t *testing.T) {
		src, err := materialSourceFromURI("pkg:docker/alpine@3.23?platform=linux%2Famd64", map[string]string{
			"sha256": "25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659",
		})
		require.NoError(t, err)
		require.Equal(t, "docker-image://docker.io/library/alpine:3.23@sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659", src.Source.Identifier)
		require.NotNil(t, src.Platform)
		require.Equal(t, "linux", src.Platform.OS)
		require.Equal(t, "amd64", src.Platform.Architecture)
	})

	t.Run("docker-purl-canonical-mismatch", func(t *testing.T) {
		_, err := materialSourceFromURI("pkg:docker/alpine@3.23?digest=sha256:25109184c71bdad752c8312a8623239686a9a2071e8825f20acb8f2198c3f659", map[string]string{
			"sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "material digest mismatch")
	})

	t.Run("git-url", func(t *testing.T) {
		src, err := materialSourceFromURI("https://github.com/moby/buildkit.git#refs/heads/master", map[string]string{
			"sha1": "9836771d0c5b21cbc7f0c38b81be39c42fc46b7b",
		})
		require.NoError(t, err)
		require.Equal(t, "git://github.com/moby/buildkit.git#refs/heads/master", src.Source.Identifier)
		require.Equal(t, "https://github.com/moby/buildkit.git#refs/heads/master", src.Source.Attrs["git.fullurl"])
	})

	t.Run("http-url", func(t *testing.T) {
		src, err := materialSourceFromURI("https://example.com/archive.tar.gz", map[string]string{
			"sha256": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		})
		require.NoError(t, err)
		require.Equal(t, "https://example.com/archive.tar.gz", src.Source.Identifier)
		require.Equal(t, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", src.Digest["sha256"])
	})

	t.Run("unsupported", func(t *testing.T) {
		_, err := materialSourceFromURI("pkg:npm/example@1.0.0", nil)
		require.Error(t, err)
	})
}
