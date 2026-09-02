package catalog

import "gopkg.in/yaml.v3"

// BuildSpec describes how to build a recipe from source data.
// Common structured values are typed; remaining scalar family configuration is
// available through Config for family-specific template expansion.
type BuildSpec struct {
	Family              string            `yaml:"family,omitempty"`
	SourceURL           string            `yaml:"source_url,omitempty"`
	Output              string            `yaml:"output,omitempty"`
	Builder             string            `yaml:"builder,omitempty"`
	Assets              []BuildAsset      `yaml:"assets,omitempty"`
	Tables              []BuildTable      `yaml:"tables,omitempty"`
	Layers              []BuildLayer      `yaml:"layers,omitempty"`
	Requires            []string          `yaml:"requires,omitempty"`
	Network             bool              `yaml:"network,omitempty"`
	EstimatedDownloadGB float64           `yaml:"estimated_download_gb,omitempty"`
	EstimatedWorkGB     float64           `yaml:"estimated_work_gb,omitempty"`
	Steps               []BuildStep       `yaml:"steps,omitempty"`
	ArchiveRules        []ArchiveRule     `yaml:"archive_rules,omitempty"`
	Config              map[string]string `yaml:"-"`
}

// BuildAsset is one URL copied into an app or data bundle.
type BuildAsset struct {
	URL  string `yaml:"url"`
	Dest string `yaml:"dest"`
}

// BuildTable describes a table produced by a structured reference builder.
type BuildTable struct {
	Name       string   `yaml:"name"`
	FTS        bool     `yaml:"fts,omitempty"`
	FTSColumns []string `yaml:"fts_columns,omitempty"`
}

// BuildLayer describes one layer fetched by a vector-service builder.
type BuildLayer struct {
	Name   string `yaml:"name"`
	Label  string `yaml:"label,omitempty"`
	Filter string `yaml:"filter,omitempty"`
}

// ArchiveRule selects and cleans pages from one domain. Rules stay data;
// API, authentication, or browser work remains an explicit builder seam.
type ArchiveRule struct {
	Domain  string   `yaml:"domain"`
	Content string   `yaml:"content,omitempty"`
	Remove  []string `yaml:"remove,omitempty"`
}

// UnmarshalYAML captures unknown scalar fields in Config while preserving all
// structured fields used by current recipe families.
func (b *BuildSpec) UnmarshalYAML(value *yaml.Node) error {
	type plain BuildSpec
	if err := value.Decode((*plain)(b)); err != nil {
		return err
	}

	var raw map[string]yaml.Node
	if err := value.Decode(&raw); err != nil {
		return nil
	}
	known := map[string]bool{
		"family": true, "source_url": true, "output": true, "builder": true,
		"assets": true, "tables": true, "layers": true, "requires": true,
		"network": true, "estimated_download_gb": true, "estimated_work_gb": true,
		"steps": true, "archive_rules": true,
	}
	for key, node := range raw {
		if known[key] {
			continue
		}
		var value string
		if err := node.Decode(&value); err != nil {
			continue
		}
		if b.Config == nil {
			b.Config = make(map[string]string)
		}
		b.Config[key] = value
	}
	return nil
}

type BuildStep struct {
	Download    string   `yaml:"download,omitempty"`
	Extract     string   `yaml:"extract,omitempty"`
	Exec        string   `yaml:"exec,omitempty"`
	Tool        string   `yaml:"tool,omitempty"`
	Verify      string   `yaml:"verify,omitempty"`
	Args        []string `yaml:"args,omitempty"`
	Inputs      []string `yaml:"inputs,omitempty"`
	Outputs     []string `yaml:"outputs,omitempty"`
	Dest        string   `yaml:"dest,omitempty"`
	NotEmpty    bool     `yaml:"not_empty,omitempty"`
	MinSize     int64    `yaml:"min_size,omitempty"`
	DockerImage string   `yaml:"docker_image,omitempty"`
}
