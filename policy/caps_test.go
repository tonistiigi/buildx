package policy

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func makeCapsPolicy(t *testing.T, env Env, module string) *Policy {
	t.Helper()
	return NewPolicy(Opt{
		Files: []File{{Filename: "test.rego", Data: []byte(module)}},
		Env:   env,
		Log: func(level logrus.Level, msg string) {
			t.Logf("[%s] %s", level, msg)
		},
	})
}

func strptr(s string) *string { return &s }

func TestCheckCapsExecProxy(t *testing.T) {
	const module = `package docker

default needs_proxy := false

needs_proxy if {
	input.env.args["ENABLE_PROXY"] == "1"
}

decision := {
	"allow": true,
	"caps": {"exec.proxy": needs_proxy},
}
`

	t.Run("enabled", func(t *testing.T) {
		p := makeCapsPolicy(t, Env{Args: map[string]*string{"ENABLE_PROXY": strptr("1")}}, module)
		caps, err := p.CheckCaps(context.Background())
		require.NoError(t, err)
		require.Equal(t, map[string]bool{"exec.proxy": true}, caps)
	})

	t.Run("disabled", func(t *testing.T) {
		p := makeCapsPolicy(t, Env{Args: map[string]*string{"ENABLE_PROXY": strptr("0")}}, module)
		caps, err := p.CheckCaps(context.Background())
		require.NoError(t, err)
		require.Equal(t, map[string]bool{"exec.proxy": false}, caps)
	})
}

// TestCheckCapsRequestMarker verifies the caps request is distinguishable from a
// regular per-source evaluation via input.env.caps_request.
func TestCheckCapsRequestMarker(t *testing.T) {
	const module = `package docker

default proxy := false

proxy if {
	input.env.caps_request
}

decision := {"caps": {"exec.proxy": proxy}}
`
	p := makeCapsPolicy(t, Env{}, module)
	caps, err := p.CheckCaps(context.Background())
	require.NoError(t, err)
	require.Equal(t, map[string]bool{"exec.proxy": true}, caps)
}

func TestCheckCapsNoCaps(t *testing.T) {
	const module = `package docker

decision := {"allow": true}
`
	p := makeCapsPolicy(t, Env{}, module)
	caps, err := p.CheckCaps(context.Background())
	require.NoError(t, err)
	require.Nil(t, caps)
}

// TestCheckCapsDefaultPolicy ensures the builtin default policy evaluates
// offline without requesting source resolution and contributes no caps.
func TestCheckCapsDefaultPolicy(t *testing.T) {
	p := makeDefaultPolicy(t, nil)
	caps, err := p.CheckCaps(context.Background())
	require.NoError(t, err)
	require.Nil(t, caps)
}

func TestCheckCapsInvalidType(t *testing.T) {
	const module = `package docker

decision := {"caps": {"exec.proxy": "yes"}}
`
	p := makeCapsPolicy(t, Env{}, module)
	_, err := p.CheckCaps(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "expecting bool")
}
