package policy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyEnvWithDepth(t *testing.T) {
	buildArg := "1"
	base := Env{
		Filename: "Dockerfile",
		Target:   "release",
		Args: map[string]*string{
			"FOO": &buildArg,
		},
		Labels: map[string]string{
			"org.example.test": "true",
		},
	}

	inp := Input{
		Image: &Image{
			Provenance: &ImageProvenance{
				Materials: []Input{
					{
						Image: &Image{
							Provenance: &ImageProvenance{
								Materials: []Input{
									{Git: &Git{}},
								},
							},
						},
					},
				},
			},
		},
	}

	applyEnvWithDepth(&inp, base, 0)

	require.Equal(t, "Dockerfile", inp.Env.Filename)
	require.Equal(t, "release", inp.Env.Target)
	require.Equal(t, 0, inp.Env.Depth)
	require.Equal(t, "true", inp.Image.Provenance.Materials[0].Env.Labels["org.example.test"])
	require.Equal(t, 1, inp.Image.Provenance.Materials[0].Env.Depth)
	require.Equal(t, 2, inp.Image.Provenance.Materials[0].Image.Provenance.Materials[0].Env.Depth)
}
