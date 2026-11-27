package menu

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
)

const openWebUISchema = `{"version": 1, "groups": [
  {"owner": "open-webui", "label": "Open WebUI", "settings": [
    {"name": "open_browser", "type": "bool", "default": true, "label": "Open browser on start", "description": "Open the browser when Open WebUI is ready."}
  ]}
]}`

const radioSchema = `{"version": 1, "groups": [
  {"owner": "radio-tool", "label": "Radio tool", "settings": [
    {"name": "gain", "type": "number", "default": 5, "label": "Gain", "description": "Receiver gain.", "min": 1, "max": 10},
    {"name": "station", "type": "text", "default": "", "label": "Station", "description": "Station name."},
    {"name": "token", "type": "secret", "default": "", "label": "Token", "description": "Access token."}
  ]}
]}`

// settingsConfig is a menu as apply generates it: Settings last, opened at once.
func settingsConfig() config.RuntimeConfig {
	cfg := sampleGroupedConfig()
	cfg.Groups = append(cfg.Groups, config.MenuGroup{
		ID: "settings", Label: "Settings", Description: "Choices saved on this drive.", AutoActivate: true,
		Items: []config.MenuItem{
			{ID: "settings", Label: "Settings", Description: "Change the choices saved on this drive.", Action: config.BuiltinAction("settings", nil)},
		},
	})
	return cfg
}

// newSettingsDrive lays out a drive with two chat models and the given
// generated schema and saved values; either may be empty for "no file".
func newSettingsDrive(t *testing.T, schema, values string) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{".svalbard", "models"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, model := range []string{"Qwen3.8-27B-UD-IQ4_XS.gguf", "gemma-4-12B-it-qat.gguf"} {
		if err := os.WriteFile(filepath.Join(root, "models", model), []byte("gguf"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range map[string]string{"settings.schema.json": schema, "settings.json": values} {
		if content == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(root, ".svalbard", name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// openSettingsScreen selects the last top-level entry and presses Enter in a
// terminal of the given size.
func openSettingsScreen(t *testing.T, root string, width, height int) Model {
	t.Helper()
	m := NewModel(settingsConfig(), root)
	m = send(m, tea.WindowSizeMsg{Width: width, Height: height})
	m.SetSelected(len(m.VisibleGroups()) - 1)
	m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.settingsActive {
		t.Fatal("Enter on Settings did not open the settings screen")
	}
	return m
}

func send(m Model, msgs ...tea.Msg) Model {
	for _, msg := range msgs {
		updated, _ := m.Update(msg)
		m = updated.(Model)
	}
	return m
}

func press(m Model, keys ...string) Model {
	for _, key := range keys {
		switch key {
		case "enter":
			m = send(m, tea.KeyMsg{Type: tea.KeyEnter})
		case "esc":
			m = send(m, tea.KeyMsg{Type: tea.KeyEsc})
		case "up":
			m = send(m, tea.KeyMsg{Type: tea.KeyUp})
		case "down":
			m = send(m, tea.KeyMsg{Type: tea.KeyDown})
		case "backspace":
			m = send(m, tea.KeyMsg{Type: tea.KeyBackspace})
		default:
			m = send(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		}
	}
	return m
}

func savedSettings(t *testing.T, root string) map[string]map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".svalbard", "settings.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("settings.json: %v\n%s", err, data)
	}
	return values
}

func TestSettingsIsTheLastTopLevelEntry(t *testing.T) {
	m := NewModel(settingsConfig(), newSettingsDrive(t, "", ""))
	groups := m.VisibleGroups()
	if last := groups[len(groups)-1]; last.ID != "settings" {
		t.Fatalf("last top-level entry = %q, want settings", last.ID)
	}
	view := withWindowSize(m).View()
	if strings.LastIndex(view, "Settings") < strings.LastIndex(view, "Tools") {
		t.Fatalf("Settings is not listed after Tools:\n%s", view)
	}
}

func TestSettingsScreenMatchesTheWireframe(t *testing.T) {
	root := newSettingsDrive(t, openWebUISchema, `{"ai": {"context": "auto"}, "open-webui": {"open_browser": false}}`)
	m := openSettingsScreen(t, root, 80, 24)

	want := `Svalbard / Settings
Choices saved on this drive (.svalbard/settings.json).

Local AI
  > Default model            ask every time
    Context size           * Auto
Open WebUI
    Open browser on start  * off

Selected
Default model: what Chat and the AI clients start with.
Choices: ask every time, Qwen3.8-27B-UD-IQ4_XS, gemma-4-12B-it-qat

enter change · r reset to default · esc back
* changed from the default`
	if got := m.View(); got != want {
		t.Errorf("View() =\n%s\nwant\n%s", got, want)
	}
	for i, line := range strings.Split(m.View(), "\n") {
		if len([]rune(line)) > 80 {
			t.Errorf("line %d is wider than 80 columns: %q", i+1, line)
		}
	}

	// The selected block follows the cursor.
	view := press(m, "down", "down").View()
	for _, want := range []string{
		"  > Open browser on start  * off",
		"    Default model            ask every time",
		"Open browser on start: Open the browser when Open WebUI is ready.",
		"Choices: on, off",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("after moving to the last row the view lacks %q:\n%s", want, view)
		}
	}
}

func TestSettingsScreenShowsRecipeGroupsOnlyWhenTheirSchemaIsThere(t *testing.T) {
	// Built-ins only: no generated schema on the drive.
	without := openSettingsScreen(t, newSettingsDrive(t, "", `{"open-webui": {"open_browser": false}}`), 80, 24).View()
	if strings.Contains(without, "Open WebUI") || strings.Contains(without, "Open browser on start") {
		t.Errorf("recipe settings shown without the recipe installed:\n%s", without)
	}
	if !strings.Contains(without, "Local AI") || !strings.Contains(without, "Context size") {
		t.Errorf("built-in settings missing:\n%s", without)
	}

	with := openSettingsScreen(t, newSettingsDrive(t, openWebUISchema, ""), 80, 24).View()
	if !strings.Contains(with, "Open WebUI\n    Open browser on start    on\n\nSelected") {
		t.Errorf("recipe group missing with its schema installed:\n%s", with)
	}
	if strings.Index(with, "Local AI") > strings.Index(with, "Open WebUI") {
		t.Errorf("built-ins are not listed first:\n%s", with)
	}
}

func TestSettingsEnterTogglesABoolAtOnce(t *testing.T) {
	root := newSettingsDrive(t, openWebUISchema, "")
	m := press(openSettingsScreen(t, root, 80, 24), "down", "down", "enter")

	if want := map[string]map[string]any{"open-webui": {"open_browser": false}}; !reflect.DeepEqual(savedSettings(t, root), want) {
		t.Fatalf("settings.json = %v, want %v", savedSettings(t, root), want)
	}
	if view := m.View(); !strings.Contains(view, "  > Open browser on start  * off") {
		t.Errorf("row does not show the new value with the changed mark:\n%s", view)
	}

	// Toggling back to the default removes the saved value and the mark.
	m = press(m, "enter")
	if saved := savedSettings(t, root); len(saved) != 0 {
		t.Fatalf("settings.json = %v, want nothing stored for a default", saved)
	}
	if view := m.View(); !strings.Contains(view, "  > Open browser on start    on") {
		t.Errorf("row does not show the default again:\n%s", view)
	}
}

func TestSettingsEnterOpensAPickerForAChoice(t *testing.T) {
	root := newSettingsDrive(t, openWebUISchema, `{"ai": {"context": "fast"}}`)
	m := press(openSettingsScreen(t, root, 80, 24), "down", "enter")

	want := `Svalbard / Settings
Choices saved on this drive (.svalbard/settings.json).

Context size: how much of the conversation the model keeps in view. Larger is
slower and needs more memory.

  ask every time
  Auto — sized for this computer's memory
▸ Fast — 8K, snappier, less memory
  16K
  Balanced — 32K
  64K
  128K
  256K — high memory
  512K — high memory
  Max — model limit, highest memory

↑/↓ select · enter confirm · esc keeps Fast`
	if got := m.View(); got != want {
		t.Errorf("picker View() =\n%s\nwant\n%s", got, want)
	}

	// Esc keeps the value; nothing is written.
	if kept := press(m, "down", "esc"); kept.settings.mode != settingsBrowse || savedSettings(t, root)["ai"]["context"] != "fast" {
		t.Fatalf("esc changed the value: %v", savedSettings(t, root))
	}

	m = press(m, "down", "down", "enter")
	if got := savedSettings(t, root)["ai"]["context"]; got != "balanced" {
		t.Fatalf("saved ai.context = %v, want balanced", got)
	}
	if view := m.View(); !strings.Contains(view, "  > Context size           * Balanced") {
		t.Errorf("list does not show the picked value:\n%s", view)
	}

	// The model picker offers what is on the drive.
	m = press(m, "up", "enter")
	if view := m.View(); !strings.Contains(view, "▸ ask every time\n  Qwen3.8-27B-UD-IQ4_XS\n  gemma-4-12B-it-qat\n") {
		t.Errorf("model picker:\n%s", view)
	}
	m = press(m, "down", "down", "enter")
	if got := savedSettings(t, root)["ai"]["default_model"]; got != "gemma-4-12B-it-qat" {
		t.Fatalf("saved ai.default_model = %v, want gemma-4-12B-it-qat", got)
	}
}

func TestSettingsEnterOpensAOneLineEdit(t *testing.T) {
	root := newSettingsDrive(t, radioSchema, "")
	m := openSettingsScreen(t, root, 80, 24)

	// Number: the edit line starts on the current value.
	m = press(m, "down", "down", "enter")
	if view := m.View(); !strings.Contains(view, "> 5") || !strings.Contains(view, "enter save · esc cancel") {
		t.Fatalf("number edit line missing:\n%s", view)
	}
	// A value outside the range is refused and the edit stays open on it.
	m = press(m, "0", "enter")
	if view := m.View(); m.settings.mode != settingsEdit || !strings.Contains(view, "Not saved: radio-tool.gain: 50 is outside 1 to 10") {
		t.Fatalf("out-of-range number was not refused in place:\n%s", view)
	}
	if saved := savedSettings(t, root); saved != nil {
		t.Fatalf("a refused value was written: %v", saved)
	}
	m = press(m, "backspace", "backspace", "7", "enter")
	if got := savedSettings(t, root)["radio-tool"]["gain"]; got != 7.0 {
		t.Fatalf("saved gain = %v, want 7", got)
	}
	if view := m.View(); m.settings.mode != settingsBrowse || !strings.Contains(view, "  > Gain           * 7") {
		t.Errorf("list does not show the saved number:\n%s", view)
	}

	// Text: letters that are commands in the list (r, q, j, k) are typed here.
	if view := press(m, "down").View(); !strings.Contains(view, "  > Station          not set\n") {
		t.Errorf("empty text row:\n%s", view)
	}
	m = press(m, "down", "enter", "r", "q", " ", "j", "k", "enter")
	if got := savedSettings(t, root)["radio-tool"]["station"]; got != "rq jk" {
		t.Fatalf("saved station = %q, want %q", got, "rq jk")
	}
	// Esc leaves the edit without saving.
	m = press(m, "enter", "x", "esc")
	if got := savedSettings(t, root)["radio-tool"]["station"]; got != "rq jk" || m.settings.mode != settingsBrowse {
		t.Fatalf("esc saved the edit: %q", got)
	}

	// Secret: dots while typing and in the list, the value only in the file.
	m = press(m, "down")
	if view := m.View(); !strings.Contains(view, "  > Token            not set") {
		t.Errorf("unset secret row:\n%s", view)
	}
	m = press(m, "enter", "h", "u", "n", "t", "e", "r", "2")
	if view := m.View(); strings.Contains(view, "hunter2") || !strings.Contains(view, "> •••••••") {
		t.Errorf("secret is not shown as dots while typing:\n%s", view)
	}
	m = press(m, "enter")
	if got := savedSettings(t, root)["radio-tool"]["token"]; got != "hunter2" {
		t.Fatalf("saved token = %v, want hunter2", got)
	}
	if view := m.View(); strings.Contains(view, "hunter2") || !strings.Contains(view, "  > Token          * ••••••••") {
		t.Errorf("saved secret is not shown as dots:\n%s", view)
	}
}

func TestSettingsRResetsToTheDefault(t *testing.T) {
	root := newSettingsDrive(t, openWebUISchema, `{"ai": {"context": "max", "default_model": "gemma-4-12B-it-qat"}, "open-webui": {"open_browser": false}}`)
	m := press(openSettingsScreen(t, root, 80, 24), "down", "r")

	want := map[string]map[string]any{"ai": {"default_model": "gemma-4-12B-it-qat"}, "open-webui": {"open_browser": false}}
	if got := savedSettings(t, root); !reflect.DeepEqual(got, want) {
		t.Fatalf("settings.json = %v, want only ai.context removed: %v", got, want)
	}
	if view := m.View(); !strings.Contains(view, "  > Context size             ask every time") ||
		!strings.Contains(view, "    Default model          * gemma-4-12B-it-qat") {
		t.Errorf("view after reset:\n%s", view)
	}
}

func TestSettingsEscReturnsToTheMenu(t *testing.T) {
	m := press(openSettingsScreen(t, newSettingsDrive(t, "", ""), 100, 40), "esc")
	if m.settingsActive || m.inGroup {
		t.Fatal("esc did not return to the top-level menu")
	}
	if view := m.View(); !strings.Contains(view, "Library") || strings.Contains(view, "Svalbard / Settings") {
		t.Errorf("top-level menu not shown after esc:\n%s", view)
	}
	// q in the list quits nothing: it goes back like esc.
	m = press(openSettingsScreen(t, newSettingsDrive(t, "", ""), 100, 40), "q")
	if m.settingsActive {
		t.Fatal("q did not leave the settings screen")
	}
}

func TestSettingsListScrollsInAShortTerminal(t *testing.T) {
	const count = 40
	var entries []string
	for i := 1; i <= count; i++ {
		entries = append(entries, fmt.Sprintf(`{"name": "option_%02d", "type": "bool", "default": false, "label": "Option %02d", "description": "Description of option %02d."}`, i, i, i))
	}
	schema := `{"version": 1, "groups": [{"owner": "many", "label": "Many", "settings": [` + strings.Join(entries, ",") + `]}]}`
	root := newSettingsDrive(t, schema, "")
	const width, height = 80, 24
	m := openSettingsScreen(t, root, width, height)

	check := func(m Model, selected string) string {
		t.Helper()
		view := m.View()
		// Every line counts, a trailing empty one too: one line more than the
		// terminal has scrolls the title away.
		lines := strings.Split(view, "\n")
		if len(lines) > height {
			t.Fatalf("%s selected: view has %d lines in a %d-line terminal:\n%s", selected, len(lines), height, view)
		}
		if lines[0] != "Svalbard / Settings" || lines[len(lines)-1] != "* changed from the default" {
			t.Errorf("%s selected: the title or the footer scrolled away:\n%s", selected, view)
		}
		if !strings.Contains(view, "  > "+selected+" ") {
			t.Errorf("%s selected but its row is not on screen:\n%s", selected, view)
		}
		if !strings.Contains(view, selected+": Description of option") && !strings.Contains(view, selected+": what Chat") {
			t.Errorf("%s selected but the Selected block shows something else:\n%s", selected, view)
		}
		return view
	}

	top := check(m, "Default model")
	if strings.Contains(top, "↑") || !strings.Contains(top, "↓ ") || strings.Contains(top, "Option 40") {
		t.Errorf("top of the list should show only a mark for more below:\n%s", top)
	}

	// Walk to the bottom one row at a time; the cursor row is always visible.
	for i := 0; i < 2+count-1; i++ {
		m = press(m, "down")
		if i >= 1 {
			check(m, fmt.Sprintf("Option %02d", i))
		}
	}
	bottom := check(m, "Option 40")
	if !strings.Contains(bottom, "↑ ") || strings.Contains(bottom, "↓") || strings.Contains(bottom, "Local AI") {
		t.Errorf("bottom of the list should show only a mark for more above:\n%s", bottom)
	}
	// One more down stays on the last row.
	if press(m, "down").settings.cursor != m.settings.cursor {
		t.Error("cursor moved past the last row")
	}

	// A change made while scrolled is saved for the row under the cursor.
	m = press(m, "enter")
	if got := savedSettings(t, root)["many"]["option_40"]; got != true {
		t.Fatalf("saved many.option_40 = %v, want true", got)
	}

	// Back up: the group heading returns with its first row.
	for i := 0; i < count-1; i++ {
		m = press(m, "up")
	}
	if view := check(m, "Option 01"); !strings.Contains(view, "Many\n  > Option 01") {
		t.Errorf("the group heading is not shown above its first row:\n%s", view)
	}
	for i := 0; i < 2; i++ {
		m = press(m, "up")
	}
	if view := check(m, "Default model"); !strings.Contains(view, "Local AI\n  > Default model") || strings.Contains(view, "↑") {
		t.Errorf("back at the top:\n%s", view)
	}

	// Growing the terminal shows everything without marks.
	m = send(m, tea.WindowSizeMsg{Width: width, Height: 80})
	if view := m.View(); strings.Contains(view, "more") || !strings.Contains(view, "Option 40") {
		t.Errorf("tall terminal still scrolls:\n%s", view)
	}
}

func TestSettingsPickerScrollsWithManyChoices(t *testing.T) {
	root := newSettingsDrive(t, "", "")
	for i := 0; i < 30; i++ {
		name := fmt.Sprintf("model-%02d.gguf", i)
		if err := os.WriteFile(filepath.Join(root, "models", name), []byte("gguf"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := press(openSettingsScreen(t, root, 80, 24), "enter")
	for i := 0; i < 32; i++ {
		m = press(m, "down")
		view := m.View()
		if lines := strings.Count(view, "\n") + 1; lines > 24 {
			t.Fatalf("picker has %d lines in a 24-line terminal:\n%s", lines, view)
		}
		if !strings.Contains(view, "▸ ") {
			t.Fatalf("picker cursor scrolled off screen after %d moves:\n%s", i+1, view)
		}
	}
	m = press(m, "enter")
	if got := savedSettings(t, root)["ai"]["default_model"]; got != "model-29" {
		t.Fatalf("saved ai.default_model = %v, want the last choice model-29", got)
	}
}

func TestSettingsScreenRefusesChangesToAnUnparsableFile(t *testing.T) {
	broken := `{"ai": {"context": "max"`
	root := newSettingsDrive(t, openWebUISchema, broken)
	m := openSettingsScreen(t, root, 80, 24)

	view := m.View()
	if !strings.Contains(view, "Nothing can be changed until the file is fixed or deleted") || !strings.Contains(view, "settings.json") {
		t.Errorf("the screen does not say why nothing can be changed:\n%s", view)
	}
	if !strings.Contains(view, "Context size             ask every time") {
		t.Errorf("defaults are not shown for an unreadable file:\n%s", view)
	}
	m = press(m, "down", "down", "enter", "r")
	if view := m.View(); !strings.Contains(view, "Not saved:") {
		t.Errorf("a refused change is not reported:\n%s", view)
	}
	if got, _ := os.ReadFile(filepath.Join(root, ".svalbard", "settings.json")); string(got) != broken {
		t.Errorf("the unparsable file was replaced: %q", got)
	}
}

func TestSettingsScreenFlagsAnIgnoredValue(t *testing.T) {
	root := newSettingsDrive(t, "", `{"ai": {"default_model": "gone-model"}}`)
	view := openSettingsScreen(t, root, 80, 24).View()
	if !strings.Contains(view, "  > Default model    ask every time") {
		t.Errorf("row should show the default without a changed mark:\n%s", view)
	}
	if !strings.Contains(view, `Ignored: ai.default_model: saved value "gone-model" is not one of`) {
		t.Errorf("the ignored value is not explained:\n%s", view)
	}
}
