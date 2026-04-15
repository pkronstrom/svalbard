package config

import (
	"encoding/json"
	"os"
)

type RuntimeConfig struct {
	Version int          `json:"version"`
	Preset  string       `json:"preset"`
	Actions []MenuAction `json:"actions"`
}

type MenuAction struct {
	Section string            `json:"section"`
	Label   string            `json:"label"`
	Action  string            `json:"action"`
	Args    map[string]string `json:"args"`
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
	if cfg.Actions == nil {
		cfg.Actions = []MenuAction{}
	}
	return cfg, nil
}
