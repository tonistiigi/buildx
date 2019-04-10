package bake

import (
	"github.com/docker/cli/cli/compose/loader"
	composetypes "github.com/docker/cli/cli/compose/types"
)

func parseCompose(dt []byte) (*composetypes.Config, error) {
	parsed, err := loader.ParseYAML([]byte(dt))
	if err != nil {
		return nil, err
	}
	return loader.Load(composetypes.ConfigDetails{
		ConfigFiles: []composetypes.ConfigFile{
			{
				Config: parsed,
			},
		},
	})
}

func ParseCompose(dt []byte) (*Config, error) {
	cfg, err := parseCompose(dt)
	if err != nil {
		return nil, err
	}

	var c Config
	if len(cfg.Services) > 0 {
		c.Group = map[string]Group{}
		c.Target = map[string]Target{}

		var g Group

		for _, s := range cfg.Services {
			g.Targets = append(g.Targets, s.Name)
			t := Target{
				Context:    s.Build.Context,
				Dockerfile: s.Build.Dockerfile,
				Labels:     s.Build.Labels,
				Args:       toMap(s.Build.Args),
				CacheFrom:  s.Build.CacheFrom,
				// TODO: add platforms
			}
			if s.Build.Target != "" {
				ss := s.Build.Target // original pointer gets replaced
				t.Target = &ss
			}
			if s.Image != "" {
				t.Tags = []string{s.Image}
			}
			c.Target[s.Name] = t
		}
		c.Group["default"] = g

	}

	return &c, nil
}

func toMap(in composetypes.MappingWithEquals) map[string]string {
	m := map[string]string{}
	for k, v := range in {
		if v != nil {
			m[k] = *v
		}
	}
	return m
}
