package catalog

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
)

// SettingSpec is one setting a recipe declares for the drive. On the drive its
// full key is <recipe id>.<name>; the value lives in .svalbard/settings.json
// and is edited from the menu's Settings screen or with `./run settings`.
type SettingSpec struct {
	// Name is the setting's name within the recipe: [a-z][a-z0-9_]*.
	Name string `yaml:"name"`
	// Type is bool, choice, number, text or secret.
	Type string `yaml:"type"`
	// Default is required and must be of Type: true/false, one of Choices, a
	// number within Min and Max, or a string (which may be empty).
	Default     any    `yaml:"default"`
	Label       string `yaml:"label"`
	Description string `yaml:"description"`
	// Choices lists the values of a choice setting.
	Choices []string `yaml:"choices,omitempty"`
	// Min and Max optionally bound a number setting.
	Min *float64 `yaml:"min,omitempty"`
	Max *float64 `yaml:"max,omitempty"`
}

// ReservedSettingOwners are the owners of the settings built into the drive
// runtime. A recipe owns its settings under its ID, so no recipe may use one
// of these as its ID. Keep in sync with drive-runtime/internal/settings.
var ReservedSettingOwners = []string{"ai", "network", "drive", "svalbard"}

var settingNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// validateSettings rejects recipes whose settings could clash on the drive or
// could not be shown and edited there.
func (c *Catalog) validateSettings() error {
	ids := make([]string, 0, len(c.recipes))
	for id := range c.recipes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := validateRecipeSettings(id, c.recipes[id]); err != nil {
			return err
		}
	}
	return nil
}

// validateRecipeSettings is validateSettings for one recipe.
func validateRecipeSettings(id string, item Item) error {
	if slices.Contains(ReservedSettingOwners, id) {
		return fmt.Errorf("recipe %s: the id is reserved for built-in settings", id)
	}
	seen := make(map[string]bool)
	for _, setting := range item.Settings {
		if !settingNamePattern.MatchString(setting.Name) {
			return fmt.Errorf("recipe %s: setting name %q must match [a-z][a-z0-9_]*", id, setting.Name)
		}
		if seen[setting.Name] {
			return fmt.Errorf("recipe %s: setting %q is declared twice", id, setting.Name)
		}
		seen[setting.Name] = true
		if err := setting.validate(); err != nil {
			return fmt.Errorf("recipe %s: setting %q: %w", id, setting.Name, err)
		}
	}
	return nil
}

func (s SettingSpec) validate() error {
	if s.Label == "" {
		return fmt.Errorf("label is required")
	}
	if s.Description == "" {
		return fmt.Errorf("description is required")
	}
	if s.Default == nil {
		return fmt.Errorf("default is required")
	}
	if s.Type != "choice" && len(s.Choices) > 0 {
		return fmt.Errorf("choices are for type choice, not %s", s.Type)
	}
	if s.Type != "number" && (s.Min != nil || s.Max != nil) {
		return fmt.Errorf("min and max are for type number, not %s", s.Type)
	}
	switch s.Type {
	case "bool":
		if _, ok := s.Default.(bool); !ok {
			return fmt.Errorf("default %v is not true or false", s.Default)
		}
	case "choice":
		if len(s.Choices) == 0 {
			return fmt.Errorf("type choice needs choices")
		}
		for i, choice := range s.Choices {
			if choice == "" || slices.Contains(s.Choices[:i], choice) {
				return fmt.Errorf("choice %q is empty or repeated", choice)
			}
		}
		text, ok := s.Default.(string)
		if !ok || !slices.Contains(s.Choices, text) {
			return fmt.Errorf("default %v is not one of the choices %v", s.Default, s.Choices)
		}
	case "number":
		number, ok := s.NumberDefault()
		if !ok {
			return fmt.Errorf("default %v is not a number", s.Default)
		}
		if s.Min != nil && s.Max != nil && *s.Min > *s.Max {
			return fmt.Errorf("min %v is above max %v", *s.Min, *s.Max)
		}
		if s.Min != nil && number < *s.Min || s.Max != nil && number > *s.Max {
			return fmt.Errorf("default %v is outside min and max", s.Default)
		}
	case "text", "secret":
		if _, ok := s.Default.(string); !ok {
			return fmt.Errorf("default %v is not text", s.Default)
		}
	default:
		return fmt.Errorf("unknown type %q (bool, choice, number, text, secret)", s.Type)
	}
	return nil
}

// NumberDefault returns the default of a number setting; YAML reads a whole
// number as an int and any other as a float64.
func (s SettingSpec) NumberDefault() (float64, bool) {
	switch value := s.Default.(type) {
	case int:
		return float64(value), true
	case float64:
		return value, true
	default:
		return 0, false
	}
}
