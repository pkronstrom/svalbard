package manifest

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Manifest represents a v2 vault manifest.
type Manifest struct {
	Version  int           `yaml:"version"`
	Vault    VaultMeta     `yaml:"vault"`
	Desired  DesiredState  `yaml:"desired"`
	Realized RealizedState `yaml:"realized"`
}

// VaultMeta holds vault-level metadata.
type VaultMeta struct {
	Name      string `yaml:"name"`
	CreatedAt string `yaml:"created_at"`
}

// DesiredState describes what the vault should contain.
type DesiredState struct {
	Presets []string       `yaml:"presets"`
	Items   []string       `yaml:"items"`
	Options DesiredOptions `yaml:"options"`
}

// DesiredOptions holds configuration knobs for the desired state.
type DesiredOptions struct {
	Region        string   `yaml:"region"`
	HostPlatforms []string `yaml:"host_platforms"`
	IndexStrategy string   `yaml:"index_strategy"`
}

// RealizedState records what was actually applied to the vault.
type RealizedState struct {
	AppliedAt string          `yaml:"applied_at"`
	Entries   []RealizedEntry `yaml:"entries"`
}

// RealizedEntry describes a single file materialized in the vault.
type RealizedEntry struct {
	ID             string `yaml:"id"`
	Type           string `yaml:"type"`
	Filename       string `yaml:"filename"`
	RelativePath   string `yaml:"relative_path"`
	SizeBytes      int64  `yaml:"size_bytes"`
	ChecksumSHA256 string `yaml:"checksum_sha256"`
	SourceStrategy string `yaml:"source_strategy"`
}

// New creates a v2 manifest with initialized slices and CreatedAt set to now.
func New(name string) Manifest {
	return Manifest{
		Version: 2,
		Vault: VaultMeta{
			Name:      name,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		},
		Desired: DesiredState{
			Presets: []string{},
			Items:   []string{},
			Options: DesiredOptions{
				HostPlatforms: []string{},
			},
		},
		Realized: RealizedState{
			Entries: []RealizedEntry{},
		},
	}
}

// Load reads a YAML manifest from the given path.
func Load(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Save marshals the manifest to YAML and writes it to the given path.
func Save(path string, m Manifest) error {
	data, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
