package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// acquisitionHashPrefix versions the field projection below, not the recipe
// YAML syntax. Change it when the projection changes incompatibly.
const acquisitionHashPrefix = "acq-v1:sha256:"

// acquisitionFields is the part of a recipe that decides which bytes get
// installed. Descriptions, tags, menu and license text and size estimates are
// left out, so editing them does not flag an installed item for update. A new
// recipe field that affects acquisition belongs here.
type acquisitionFields struct {
	Type           string            `json:"type"`
	Strategy       string            `json:"strategy,omitempty"`
	URL            string            `json:"url,omitempty"`
	URLPattern     string            `json:"url_pattern,omitempty"`
	SHA256         string            `json:"sha256,omitempty"`
	Filename       string            `json:"filename,omitempty"`
	Platforms      map[string]string `json:"platforms,omitempty"`
	PlatformSHA256 map[string]string `json:"platform_sha256,omitempty"`
	Bundle         []BundleSource    `json:"bundle,omitempty"`
	Build          *acquisitionBuild `json:"build,omitempty"`
	Embedding      *EmbeddingSpec    `json:"embedding,omitempty"`
	Python         string            `json:"python,omitempty"`
	Venv           string            `json:"venv,omitempty"`
	Packages       []string          `json:"packages,omitempty"`
	EntryPoints    []string          `json:"entry_points,omitempty"`
}

// acquisitionBuild is BuildSpec without its size estimates.
type acquisitionBuild struct {
	Family       string            `json:"family,omitempty"`
	SourceURL    string            `json:"source_url,omitempty"`
	Output       string            `json:"output,omitempty"`
	Builder      string            `json:"builder,omitempty"`
	Assets       []BuildAsset      `json:"assets,omitempty"`
	Tables       []BuildTable      `json:"tables,omitempty"`
	Layers       []BuildLayer      `json:"layers,omitempty"`
	Requires     []string          `json:"requires,omitempty"`
	Network      bool              `json:"network,omitempty"`
	Steps        []BuildStep       `json:"steps,omitempty"`
	ArchiveRules []ArchiveRule     `json:"archive_rules,omitempty"`
	Config       map[string]string `json:"config,omitempty"`
}

// AcquisitionHash identifies how the recipe acquires its content. Two recipes
// with the same hash install the same thing; an installed item whose recorded
// hash differs from its recipe's has an update available. It does not detect
// new bytes behind an unchanged URL, and it is separate from file checksums
// and builder cache fingerprints.
func (it Item) AcquisitionHash() string {
	fields := acquisitionFields{
		Type:           it.Type,
		Strategy:       it.Strategy,
		URL:            it.URL,
		URLPattern:     it.URLPattern,
		SHA256:         it.SHA256,
		Filename:       it.Filename,
		Platforms:      it.Platforms,
		PlatformSHA256: it.PlatformSHA256,
		Bundle:         it.Bundle,
		Embedding:      it.Embedding,
		Python:         it.Python,
		Venv:           it.Venv,
		Packages:       it.Packages,
		EntryPoints:    it.EntryPoints,
	}
	if b := it.Build; b != nil {
		fields.Build = &acquisitionBuild{
			Family: b.Family, SourceURL: b.SourceURL, Output: b.Output, Builder: b.Builder,
			Assets: b.Assets, Tables: b.Tables, Layers: b.Layers, Requires: b.Requires,
			Network: b.Network, Steps: b.Steps, ArchiveRules: b.ArchiveRules, Config: b.Config,
		}
	}
	// encoding/json writes struct fields in declaration order and map keys
	// sorted, so the bytes do not depend on YAML or map iteration order.
	data, err := json.Marshal(fields)
	if err != nil {
		// Only strings, bools, ints and their slices and maps are marshalled.
		panic("catalog: marshalling acquisition fields: " + err.Error())
	}
	sum := sha256.Sum256(data)
	return acquisitionHashPrefix + hex.EncodeToString(sum[:])
}
