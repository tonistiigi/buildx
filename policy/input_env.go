package policy

func applyEnvWithDepth(inp *Input, base Env, depth int) {
	if inp == nil {
		return
	}
	env := base
	env.Depth = depth
	inp.Env = env

	if inp.Image == nil || inp.Image.Provenance == nil || len(inp.Image.Provenance.Materials) == 0 {
		return
	}
	for i := range inp.Image.Provenance.Materials {
		applyEnvWithDepth(&inp.Image.Provenance.Materials[i], base, depth+1)
	}
}
