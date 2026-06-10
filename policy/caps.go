package policy

import (
	"context"

	"github.com/open-policy-agent/opa/v1/rego"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// CapExecProxy is the capability name a policy uses to request that exec
// operations of the build run with proxied networking.
const CapExecProxy = "exec.proxy"

// Caps describes the build capabilities requested by a policy through the
// optional caps property of the decision document.
type Caps struct {
	ExecProxy bool
}

// CheckCaps evaluates the policy once before the build is issued and
// returns the capabilities requested by the policy. Only input.env is set,
// with env.capsRequest marking the request. The evaluation is a single
// offline pass: source metadata can not be resolved. The allow and
// deny_msg properties of the decision are ignored for this request.
func (p *Policy) CheckCaps(ctx context.Context) (*Caps, error) {
	baseOpts, cleanup := p.regoBaseOpts()
	defer cleanup()

	var inp Input
	applyEnvWithDepth(&inp, p.opt.Env, 0)
	inp.Env.CapsRequest = true

	st := &state{Input: inp}
	runOpts := append([]func(*rego.Rego){}, baseOpts...)
	runOpts = append(runOpts, rego.Input(inp))
	for _, f := range p.funcs {
		runOpts = append(runOpts, f.impl(st))
	}

	p.log(logrus.InfoLevel, "checking policy capabilities")

	rs, err := rego.New(runOpts...).Eval(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to evaluate policy capabilities request")
	}
	vt, err := decisionResult(rs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to evaluate policy capabilities request")
	}

	caps := &Caps{}
	v, ok := vt["caps"]
	if !ok {
		return caps, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, errors.Errorf("invalid caps property type %T, expecting object", v)
	}
	for k, val := range m {
		b, ok := val.(bool)
		if !ok {
			return nil, errors.Errorf("invalid type %T for capability %q, expecting bool", val, k)
		}
		switch k {
		case CapExecProxy:
			caps.ExecProxy = b
		default:
			return nil, errors.Errorf("unknown capability %q requested by policy", k)
		}
	}
	if caps.ExecProxy {
		p.log(logrus.InfoLevel, "policy requested capability %s", CapExecProxy)
	}
	return caps, nil
}
