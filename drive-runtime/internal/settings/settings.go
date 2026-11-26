// Package settings is the drive's settings store. One schema drives the values
// file (<drive>/.svalbard/settings.json), the menu's Settings screen and the
// `./run settings` command. A full key is <owner>.<name>: built-in settings
// belong to a reserved owner and are compiled in, a recipe's settings belong
// to its recipe ID and arrive in settings.schema.json, written by apply for
// the recipes installed on the drive.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	svalbardDir = ".svalbard"
	valuesFile  = "settings.json"
	schemaFile  = "settings.schema.json"
)

// Type is the kind of value a setting holds.
type Type string

const (
	TypeBool   Type = "bool"
	TypeChoice Type = "choice"
	TypeNumber Type = "number"
	TypeText   Type = "text"
	TypeSecret Type = "secret"
)

// reservedOwners are the owners of built-in settings; the catalog rejects a
// recipe with one of these IDs. Keep in sync with
// host-cli/internal/catalog/settings.go.
var reservedOwners = []string{"ai", "network", "drive", "svalbard"}

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Setting is one entry of the schema. Default and every value handed out for
// it is a bool (TypeBool), a float64 (TypeNumber) or a string (the rest).
type Setting struct {
	Owner       string   `json:"-"`
	Name        string   `json:"name"`
	Type        Type     `json:"type"`
	Default     any      `json:"default"`
	Label       string   `json:"label"`
	Description string   `json:"description"`
	Choices     []string `json:"choices,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`

	// ChoiceLabels are display names for choices and ChoiceNotes a short
	// remark for the menu's picker, set for built-ins only. The command and
	// the file always use the plain choice.
	ChoiceLabels map[string]string `json:"-"`
	ChoiceNotes  map[string]string `json:"-"`
}

// Key is the setting's full key, <owner>.<name>.
func (s Setting) Key() string { return s.Owner + "." + s.Name }

// Group is one owner's settings under the heading the menu and the command
// show them with.
type Group struct {
	Owner    string    `json:"owner"`
	Label    string    `json:"label"`
	Settings []Setting `json:"settings"`
}

// schemaDocument is settings.schema.json. Keep in sync with
// host-cli/internal/toolkit/settings_schema.go.
type schemaDocument struct {
	Version int     `json:"version"`
	Groups  []Group `json:"groups"`
}

// Store is the schema together with the values saved on one drive.
type Store struct {
	driveRoot string
	groups    []Group
	values    map[string]map[string]json.RawMessage
	// foreign holds top-level entries of the values file that are not an
	// object of settings and belong to no known owner; they are written back
	// as found.
	foreign map[string]json.RawMessage
	// schemaProblem is why the generated schema file could not be used.
	schemaProblem string
	// broken is why the values file could not be read. Such a store answers
	// with defaults and refuses to write, so the file is never replaced.
	broken error
}

// Open loads the schema and the saved values of the drive at driveRoot. A
// missing or empty values file means every default. A values file that does
// not parse is returned as an error together with a store that answers with
// defaults and refuses to write.
func Open(driveRoot string) (*Store, error) {
	s := &Store{driveRoot: driveRoot}
	s.groups = builtinGroups(driveRoot)
	recipeGroups, err := loadSchema(filepath.Join(driveRoot, svalbardDir, schemaFile))
	if err != nil {
		s.schemaProblem = fmt.Sprintf("%v; recipe settings are unavailable until the next svalbard apply", err)
	}
	s.groups = append(s.groups, recipeGroups...)
	if err := s.reload(); err != nil {
		s.broken = err
		s.values, s.foreign = nil, nil
		return s, err
	}
	return s, nil
}

// OpenForAction is Open for code that only reads: an unreadable values file is
// reported on warn in one line and the action carries on with the defaults.
func OpenForAction(driveRoot string, warn io.Writer) *Store {
	s, err := Open(driveRoot)
	if err != nil {
		fmt.Fprintf(warn, "Settings ignored, using the defaults: %v\n", err)
	}
	return s
}

// loadSchema reads the recipe groups apply generated. A missing file is fine:
// the built-in settings work on their own.
func loadSchema(path string) ([]Group, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var doc schemaDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var groups []Group
	for _, group := range doc.Groups {
		// The catalog validates recipes, but the file is on a hand-editable
		// drive: an entry that would clash or cannot be shown is left out.
		if !validOwner(group.Owner) || slices.Contains(reservedOwners, group.Owner) {
			continue
		}
		kept := Group{Owner: group.Owner, Label: group.Label}
		if kept.Label == "" {
			kept.Label = group.Owner
		}
		for _, setting := range group.Settings {
			setting.Owner = group.Owner
			value, err := setting.normalize(setting.Default)
			if err != nil || !namePattern.MatchString(setting.Name) || setting.Label == "" {
				continue
			}
			setting.Default = value
			kept.Settings = append(kept.Settings, setting)
		}
		if len(kept.Settings) > 0 {
			groups = append(groups, kept)
		}
	}
	return groups, nil
}

// validOwner rejects an owner that could not be typed as part of a key. A dot
// is fine: names have none, so the last dot of a key ends the owner.
func validOwner(owner string) bool {
	return owner != "" && !strings.ContainsAny(owner, " \t\n")
}

func (s *Store) valuesPath() string {
	return filepath.Join(s.driveRoot, svalbardDir, valuesFile)
}

// reload reads the values file.
func (s *Store) reload() error {
	path := s.valuesPath()
	top := map[string]json.RawMessage{}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return err
	case len(strings.TrimSpace(string(data))) == 0:
	default:
		if err := json.Unmarshal(data, &top); err != nil {
			return fmt.Errorf("%s: %s", path, describeJSONError(data, err))
		}
	}
	values := map[string]map[string]json.RawMessage{}
	foreign := map[string]json.RawMessage{}
	for owner, raw := range top {
		var settings map[string]json.RawMessage
		if err := json.Unmarshal(raw, &settings); err == nil {
			values[owner] = settings
		} else if s.hasOwner(owner) {
			return fmt.Errorf(`%s: %q must be an object of settings, like {"<name>": <value>}`, path, owner)
		} else {
			foreign[owner] = raw
		}
	}
	s.values, s.foreign = values, foreign
	return nil
}

func (s *Store) hasOwner(owner string) bool {
	return slices.ContainsFunc(s.groups, func(g Group) bool { return g.Owner == owner })
}

// describeJSONError gives a hand-editor the line of a syntax error and names
// the shape the file has.
func describeJSONError(data []byte, err error) string {
	var syntax *json.SyntaxError
	if errors.As(err, &syntax) {
		line := 1 + strings.Count(string(data[:min(int(syntax.Offset), len(data))]), "\n")
		return fmt.Sprintf("line %d: %v", line, err)
	}
	var mismatch *json.UnmarshalTypeError
	if errors.As(err, &mismatch) {
		return `the file must be an object, like {"<owner>": {"<name>": <value>}}`
	}
	return err.Error()
}

// Groups returns the schema in display order: built-ins first, then one group
// per installed recipe that declares settings.
func (s *Store) Groups() []Group { return s.groups }

// Lookup finds a setting by its full key.
func (s *Store) Lookup(key string) (Setting, bool) {
	for _, group := range s.groups {
		for _, setting := range group.Settings {
			if setting.Key() == key {
				return setting, true
			}
		}
	}
	return Setting{}, false
}

// Keys lists every known key in display order.
func (s *Store) Keys() []string {
	var keys []string
	for _, group := range s.groups {
		for _, setting := range group.Settings {
			keys = append(keys, setting.Key())
		}
	}
	return keys
}

// stored returns the saved value of a setting, decoded but not validated.
func (s *Store) stored(setting Setting) (any, bool) {
	raw, ok := s.values[setting.Owner][setting.Name]
	if !ok {
		return nil, false
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, false
	}
	return value, true
}

// Value returns the setting's value: the saved one when it is valid, otherwise
// the default. An unknown key yields nil.
func (s *Store) Value(key string) any {
	setting, ok := s.Lookup(key)
	if !ok {
		return nil
	}
	if stored, ok := s.stored(setting); ok {
		if value, err := setting.normalize(stored); err == nil {
			return value
		}
	}
	return setting.Default
}

// Bool returns a bool setting's value.
func (s *Store) Bool(key string) bool {
	value, _ := s.Value(key).(bool)
	return value
}

// String returns the value of a choice, text or secret setting.
func (s *Store) String(key string) string {
	value, _ := s.Value(key).(string)
	return value
}

// Number returns a number setting's value.
func (s *Store) Number(key string) float64 {
	value, _ := s.Value(key).(float64)
	return value
}

// Changed reports whether the setting has a valid saved value that differs
// from its default.
func (s *Store) Changed(key string) bool {
	setting, ok := s.Lookup(key)
	return ok && s.Value(key) != setting.Default
}

// Problem describes the saved value of key when it is being ignored, and is
// empty otherwise.
func (s *Store) Problem(key string) string {
	setting, ok := s.Lookup(key)
	if !ok {
		return ""
	}
	raw, ok := s.values[setting.Owner][setting.Name]
	if !ok {
		return ""
	}
	stored, _ := s.stored(setting)
	if _, err := setting.normalize(stored); err != nil {
		return fmt.Sprintf("%s: saved value %s %v; using %s", key, strings.TrimSpace(string(raw)), err, setting.Format(setting.Default))
	}
	return ""
}

// Problems lists what is wrong without being fatal: a schema file that could
// not be used and saved values that are ignored.
func (s *Store) Problems() []string {
	var problems []string
	if s.schemaProblem != "" {
		problems = append(problems, s.schemaProblem)
	}
	for _, key := range s.Keys() {
		if problem := s.Problem(key); problem != "" {
			problems = append(problems, problem)
		}
	}
	return problems
}

// Set parses text as a value for key and saves it. A value equal to the
// default is not stored, since an absent key means the default.
func (s *Store) Set(key, text string) error {
	setting, ok := s.Lookup(key)
	if !ok {
		return s.unknownKey(key)
	}
	value, err := setting.Parse(text)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	return s.update(setting, value)
}

// Reset returns key to its default by removing its saved value.
func (s *Store) Reset(key string) error {
	setting, ok := s.Lookup(key)
	if !ok {
		return s.unknownKey(key)
	}
	return s.update(setting, setting.Default)
}

func (s *Store) unknownKey(key string) error {
	return fmt.Errorf("unknown setting %q; settings on this drive: %s", key, strings.Join(s.Keys(), ", "))
}

// update saves one value. The file is read again first, so a change made
// elsewhere since Open (the command while the menu is open, an editor) is kept.
func (s *Store) update(setting Setting, value any) error {
	if s.broken != nil {
		return s.broken
	}
	if err := s.reload(); err != nil {
		return err
	}
	owner := s.values[setting.Owner]
	if value == setting.Default {
		delete(owner, setting.Name)
		if len(owner) == 0 {
			delete(s.values, setting.Owner)
		}
	} else {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if owner == nil {
			owner = map[string]json.RawMessage{}
			s.values[setting.Owner] = owner
		}
		owner[setting.Name] = raw
	}
	if err := s.write(); err != nil {
		return err
	}
	return s.reload()
}

// write replaces the values file through a temporary file in the same
// directory, so a drive pulled mid-write keeps the old file whole.
func (s *Store) write() error {
	path := s.valuesPath()
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		return fmt.Errorf("%s is not a Svalbard drive: no %s directory", s.driveRoot, svalbardDir)
	}
	out := map[string]any{}
	for owner, raw := range s.foreign {
		out[owner] = raw
	}
	for owner, settings := range s.values {
		out[owner] = settings
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	// Best effort: exFAT and FAT drives take their modes from the mount, and
	// Linux refuses a chmod there.
	_ = tmp.Chmod(0o644)
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// normalize checks a decoded JSON value against the setting and returns it in
// the setting's Go type.
func (s Setting) normalize(value any) (any, error) {
	switch s.Type {
	case TypeBool:
		if b, ok := value.(bool); ok {
			return b, nil
		}
		return nil, errors.New("is not true or false")
	case TypeNumber:
		n, ok := value.(float64)
		if !ok {
			return nil, errors.New("is not a number")
		}
		if s.Min != nil && n < *s.Min || s.Max != nil && n > *s.Max {
			return nil, fmt.Errorf("is outside %s", s.rangeText())
		}
		return n, nil
	case TypeChoice:
		text, ok := value.(string)
		if !ok || !slices.Contains(s.Choices, text) {
			return nil, fmt.Errorf("is not one of %s", strings.Join(s.Choices, ", "))
		}
		return text, nil
	case TypeText, TypeSecret:
		text, ok := value.(string)
		if !ok {
			return nil, errors.New("is not text")
		}
		if strings.ContainsAny(text, "\r\n") {
			return nil, errors.New("has more than one line")
		}
		return text, nil
	default:
		return nil, fmt.Errorf("has unknown type %q", s.Type)
	}
}

// Parse reads a value typed by a person: on the command line or in the menu's
// edit line.
func (s Setting) Parse(text string) (any, error) {
	switch s.Type {
	case TypeBool:
		switch strings.ToLower(strings.TrimSpace(text)) {
		case "on", "true", "yes", "1":
			return true, nil
		case "off", "false", "no", "0":
			return false, nil
		}
		return nil, fmt.Errorf("%q is not on or off", text)
	case TypeNumber:
		n, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number%s", text, s.rangeHint())
		}
		if _, err := s.normalize(n); err != nil {
			return nil, fmt.Errorf("%s %v", formatNumber(n), err)
		}
		return n, nil
	default:
		if _, err := s.normalize(text); err != nil {
			return nil, fmt.Errorf("%q %v", text, err)
		}
		return text, nil
	}
}

// Format renders a value the way the command prints and accepts it.
func (s Setting) Format(value any) string {
	switch value := value.(type) {
	case bool:
		if value {
			return "on"
		}
		return "off"
	case float64:
		return formatNumber(value)
	case string:
		return value
	default:
		return ""
	}
}

// Display renders a value for the menu: Format, with a built-in choice under
// its display name.
func (s Setting) Display(value any) string {
	if text, ok := value.(string); ok {
		if label, ok := s.ChoiceLabels[text]; ok {
			return label
		}
	}
	return s.Format(value)
}

// rangeText names the bounds of a number setting: "1 to 10", "1 or more".
func (s Setting) rangeText() string {
	switch {
	case s.Min != nil && s.Max != nil:
		return formatNumber(*s.Min) + " to " + formatNumber(*s.Max)
	case s.Min != nil:
		return formatNumber(*s.Min) + " or more"
	case s.Max != nil:
		return formatNumber(*s.Max) + " or less"
	default:
		return ""
	}
}

func (s Setting) rangeHint() string {
	if text := s.rangeText(); text != "" {
		return " (" + text + ")"
	}
	return ""
}

func formatNumber(n float64) string {
	return strconv.FormatFloat(n, 'f', -1, 64)
}
