package config

import (
	"encoding/json"
	"os"
)

type RuntimeConfig struct {
	Version int         `json:"version"`
	Preset  string      `json:"preset"`
	Groups  []MenuGroup `json:"groups"`
}

type MenuGroup struct {
	ID          string     `json:"id"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	Order       int        `json:"order"`
	Items       []MenuItem `json:"items"`
}

type MenuItem struct {
	ID          string     `json:"id"`
	Label       string     `json:"label"`
	Description string     `json:"description"`
	Subheader   string     `json:"subheader,omitempty"`
	Order       int        `json:"order"`
	Action      ActionSpec `json:"action"`
}

type ActionSpec struct {
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

type BuiltinActionConfig struct {
	Name string            `json:"name"`
	Args map[string]string `json:"args,omitempty"`
}

type ExecActionConfig struct {
	Executable  string            `json:"executable"`
	ResolveFrom string            `json:"resolve_from,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Cwd         string            `json:"cwd,omitempty"`
	Mode        string            `json:"mode,omitempty"`
}

func BuiltinAction(name string, args map[string]string) ActionSpec {
	if args == nil {
		args = map[string]string{}
	}
	return mustActionSpec("builtin", BuiltinActionConfig{
		Name: name,
		Args: args,
	})
}

func ExecAction(cfg ExecActionConfig) ActionSpec {
	if cfg.Args == nil {
		cfg.Args = []string{}
	}
	if cfg.Env == nil {
		cfg.Env = map[string]string{}
	}
	return mustActionSpec("exec", cfg)
}

func (a ActionSpec) DecodeBuiltin() (BuiltinActionConfig, error) {
	var cfg BuiltinActionConfig
	if err := json.Unmarshal(a.Config, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Args == nil {
		cfg.Args = map[string]string{}
	}
	return cfg, nil
}

func (a ActionSpec) DecodeExec() (ExecActionConfig, error) {
	var cfg ExecActionConfig
	if err := json.Unmarshal(a.Config, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Args == nil {
		cfg.Args = []string{}
	}
	if cfg.Env == nil {
		cfg.Env = map[string]string{}
	}
	return cfg, nil
}

func mustActionSpec(kind string, cfg any) ActionSpec {
	data, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	return ActionSpec{
		Type:   kind,
		Config: data,
	}
}

func Load(path string) (RuntimeConfig, error) {
	var cfg RuntimeConfig

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.Groups == nil {
		cfg.Groups = []MenuGroup{}
	}
	for i := range cfg.Groups {
		if cfg.Groups[i].Items == nil {
			cfg.Groups[i].Items = []MenuItem{}
		}
	}
	return cfg, nil
}
