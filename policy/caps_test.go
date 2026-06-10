package policy

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"

	gwpb "github.com/moby/buildkit/frontend/gateway/pb"
	"github.com/moby/buildkit/solver/pb"
	moby_buildkit_v1_sourcepolicy "github.com/moby/buildkit/sourcepolicy/pb"
	"github.com/moby/buildkit/sourcepolicy/policysession"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func makeCapsTestPolicy(t *testing.T, policySrc string, env Env, fsys fs.StatFS) *Policy {
	t.Helper()
	opt := Opt{
		Files: []File{{Filename: "test.rego", Data: []byte(policySrc)}},
		Env:   env,
		Log: func(level logrus.Level, msg string) {
			t.Logf("[%s] %s", level, msg)
		},
	}
	if fsys != nil {
		opt.FS = func() (fs.StatFS, func() error, error) {
			return fsys, nil, nil
		}
	}
	return NewPolicy(opt)
}

func TestCheckCapsExecProxy(t *testing.T) {
	policySrc := `
package docker

default allow := true

default caps := {}

caps := {"exec.proxy": true} if input.env.capsRequest

decision := {"allow": allow, "caps": caps}
`
	p := makeCapsTestPolicy(t, policySrc, Env{}, nil)

	caps, err := p.CheckCaps(context.Background())
	require.NoError(t, err)
	require.True(t, caps.ExecProxy)

	// caps in the decision of a regular source request is ignored
	resp, meta, err := p.CheckPolicy(context.Background(), &policysession.CheckPolicyRequest{
		Source: &gwpb.ResolveSourceMetaResponse{
			Source: &pb.SourceOp{Identifier: "https://example.com/foo.tar.gz"},
			HTTP: &gwpb.ResolveSourceHTTPResponse{
				Checksum: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			},
		},
	})
	require.NoError(t, err)
	require.Nil(t, meta)
	require.Equal(t, moby_buildkit_v1_sourcepolicy.PolicyAction_ALLOW, resp.Action)
}

func TestCheckCapsNotRequested(t *testing.T) {
	policySrc := `
package docker

decision := {"allow": true}
`
	p := makeCapsTestPolicy(t, policySrc, Env{}, nil)

	caps, err := p.CheckCaps(context.Background())
	require.NoError(t, err)
	require.False(t, caps.ExecProxy)
}

func TestCheckCapsExecProxyFalse(t *testing.T) {
	policySrc := `
package docker

decision := {"allow": true, "caps": {"exec.proxy": false}}
`
	p := makeCapsTestPolicy(t, policySrc, Env{}, nil)

	caps, err := p.CheckCaps(context.Background())
	require.NoError(t, err)
	require.False(t, caps.ExecProxy)
}

func TestCheckCapsEnvInput(t *testing.T) {
	policySrc := `
package docker

default caps := {}

caps := {"exec.proxy": true} if {
	input.env.capsRequest
	input.env.target == "prod"
}

decision := {"allow": true, "caps": caps}
`
	p := makeCapsTestPolicy(t, policySrc, Env{Target: "prod"}, nil)
	caps, err := p.CheckCaps(context.Background())
	require.NoError(t, err)
	require.True(t, caps.ExecProxy)

	p = makeCapsTestPolicy(t, policySrc, Env{Target: "dev"}, nil)
	caps, err = p.CheckCaps(context.Background())
	require.NoError(t, err)
	require.False(t, caps.ExecProxy)
}

func TestCheckCapsUnknownCap(t *testing.T) {
	policySrc := `
package docker

default caps := {}

caps := {"exec.future": true} if input.env.capsRequest

decision := {"allow": true, "caps": caps}
`
	p := makeCapsTestPolicy(t, policySrc, Env{}, nil)

	_, err := p.CheckCaps(context.Background())
	require.ErrorContains(t, err, `unknown capability "exec.future"`)
}

func TestCheckCapsInvalidValueType(t *testing.T) {
	policySrc := `
package docker

default caps := {}

caps := {"exec.proxy": "yes"} if input.env.capsRequest

decision := {"allow": true, "caps": caps}
`
	p := makeCapsTestPolicy(t, policySrc, Env{}, nil)

	_, err := p.CheckCaps(context.Background())
	require.ErrorContains(t, err, "expecting bool")
}

func TestCheckCapsLoadJSON(t *testing.T) {
	policySrc := `
package docker

default caps := {}

caps := {"exec.proxy": load_json("caps.json").proxy} if input.env.capsRequest

decision := {"allow": true, "caps": caps}
`
	fsys := fstest.MapFS{
		"caps.json": &fstest.MapFile{Data: []byte(`{"proxy": true}`)},
	}
	p := makeCapsTestPolicy(t, policySrc, Env{}, fsys)

	caps, err := p.CheckCaps(context.Background())
	require.NoError(t, err)
	require.True(t, caps.ExecProxy)
}

func TestCheckCapsUndefinedDecision(t *testing.T) {
	// a decision that is only defined when source input is present fails
	// the offline capabilities request
	policySrc := `
package docker

decision := {"allow": true} if input.image
`
	p := makeCapsTestPolicy(t, policySrc, Env{}, nil)

	_, err := p.CheckCaps(context.Background())
	require.ErrorContains(t, err, "failed to evaluate policy capabilities request")
}

func TestCheckCapsDefaultPolicy(t *testing.T) {
	// the builtin default policy must evaluate cleanly offline and request
	// no capabilities
	p := makeDefaultPolicy(t, nil)

	caps, err := p.CheckCaps(context.Background())
	require.NoError(t, err)
	require.False(t, caps.ExecProxy)
}
