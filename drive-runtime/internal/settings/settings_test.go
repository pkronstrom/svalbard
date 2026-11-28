package settings

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/llamaserve"
)

const testSchema = `{
  "version": 1,
  "groups": [
    {"owner": "open-webui", "label": "Open WebUI", "settings": [
      {"name": "open_browser", "type": "bool", "default": true, "label": "Open browser on start", "description": "Open the browser when Open WebUI is ready."}
    ]},
    {"owner": "radio-tool", "label": "Radio tool", "settings": [
      {"name": "band", "type": "choice", "default": "vhf", "label": "Band", "description": "Band to scan.", "choices": ["vhf", "uhf"]},
      {"name": "gain", "type": "number", "default": 5, "label": "Gain", "description": "Receiver gain.", "min": 1, "max": 10},
      {"name": "station", "type": "text", "default": "", "label": "Station", "description": "Station name."},
      {"name": "token", "type": "secret", "default": "", "label": "Token", "description": "Access token."}
    ]}
  ]
}`

// newDrive lays out a drive with two chat models and, when schema is not
// empty, the schema file apply would have generated.
func newDrive(t *testing.T, schema string) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{".svalbard", "models/embed"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, model := range []string{"gemma-4-12B-it-qat.gguf", "Qwen3.8-27B-UD-IQ4_XS.gguf", "._gemma-4-12B-it-qat.gguf", "embed/nomic-embed.gguf"} {
		if err := os.WriteFile(filepath.Join(root, "models", model), []byte("gguf"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if schema != "" {
		writeFile(t, filepath.Join(root, ".svalbard", schemaFile), schema)
	}
	return root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeValues(t *testing.T, root, content string) {
	t.Helper()
	writeFile(t, filepath.Join(root, ".svalbard", valuesFile), content)
}

func readValues(t *testing.T, root string) map[string]map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".svalbard", valuesFile))
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]map[string]any
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("%s is not owner → name → value: %v\n%s", valuesFile, err, data)
	}
	return values
}

func mustOpen(t *testing.T, root string) *Store {
	t.Helper()
	store, err := Open(root)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	return store
}

func TestBuiltinsWorkWithoutSchemaOrValuesFile(t *testing.T) {
	root := newDrive(t, "")
	store := mustOpen(t, root)

	if got, want := store.Keys(), []string{"ai.default_model", "ai.context", "ai.web_search", "ai.web_search_url"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Keys() = %v, want %v", got, want)
	}
	if group := store.Groups()[0]; group.Owner != "ai" || group.Label != "Local AI" {
		t.Errorf("group = %q %q, want ai / Local AI", group.Owner, group.Label)
	}
	for _, key := range []string{KeyDefaultModel, KeyContext} {
		if got := store.String(key); got != Ask {
			t.Errorf("%s = %q, want the default %q", key, got, Ask)
		}
	}
	if store.Bool(KeyWebSearch) || store.String(KeyWebSearchURL) != "" {
		t.Fatal("Web search must be off without explicit settings")
	}
	for _, key := range store.Keys() {
		if store.Changed(key) {
			t.Errorf("%s reported as changed with no values file", key)
		}
	}
	// The model choices are the chat models on the drive: no embedding
	// model, no macOS resource fork.
	model, _ := store.Lookup(KeyDefaultModel)
	if want := []string{Ask, "Qwen3.8-27B-UD-IQ4_XS", "gemma-4-12B-it-qat"}; !reflect.DeepEqual(model.Choices, want) {
		t.Errorf("default_model choices = %v, want %v", model.Choices, want)
	}
	if got := model.Display(Ask); got != "ask every time" {
		t.Errorf("ask displays as %q, want %q", got, "ask every time")
	}
	if len(store.Problems()) != 0 {
		t.Errorf("Problems() = %v, want none", store.Problems())
	}
	if _, err := os.Stat(filepath.Join(root, ".svalbard", valuesFile)); !os.IsNotExist(err) {
		t.Errorf("reading created %s: %v", valuesFile, err)
	}
}

func TestEmptyValuesFileMeansDefaults(t *testing.T) {
	for _, content := range []string{"", "  \n", "{}"} {
		root := newDrive(t, testSchema)
		writeValues(t, root, content)
		store := mustOpen(t, root)
		if !store.Bool("open-webui.open_browser") || store.String("radio-tool.band") != "vhf" || store.Number("radio-tool.gain") != 5 {
			t.Errorf("content %q: values are not the defaults", content)
		}
	}
}

func TestRecipeSettingsComeFromTheSchemaFileAfterBuiltins(t *testing.T) {
	store := mustOpen(t, newDrive(t, testSchema))

	var groups []string
	for _, group := range store.Groups() {
		groups = append(groups, group.Owner+"="+group.Label)
	}
	if want := []string{"ai=Local AI", "open-webui=Open WebUI", "radio-tool=Radio tool"}; !reflect.DeepEqual(groups, want) {
		t.Fatalf("groups = %v, want %v", groups, want)
	}
	gain, ok := store.Lookup("radio-tool.gain")
	if !ok || gain.Type != TypeNumber || gain.Default != float64(5) || *gain.Min != 1 || *gain.Max != 10 {
		t.Errorf("radio-tool.gain = %+v (%v)", gain, ok)
	}
	if _, ok := store.Lookup("radio-tool.missing"); ok {
		t.Error("Lookup found a setting the schema does not declare")
	}
}

func TestSchemaFileCannotTakeOverABuiltinOwner(t *testing.T) {
	root := newDrive(t, `{"version":1,"groups":[
		{"owner":"ai","label":"Hijack","settings":[{"name":"context","type":"bool","default":true,"label":"L","description":"D"}]},
		{"owner":"tool","label":"Tool","settings":[
			{"name":"ok","type":"bool","default":false,"label":"OK","description":"D"},
			{"name":"Bad-Name","type":"bool","default":false,"label":"L","description":"D"},
			{"name":"bad_default","type":"choice","default":"z","choices":["a"],"label":"L","description":"D"}]}]}`)
	store := mustOpen(t, root)
	if got, want := store.Keys(), []string{"ai.default_model", "ai.context", "ai.web_search", "ai.web_search_url", "tool.ok"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Keys() = %v, want %v", got, want)
	}
	if context, _ := store.Lookup(KeyContext); context.Type != TypeChoice {
		t.Errorf("ai.context was replaced by the schema file: %+v", context)
	}
}

func TestUnreadableSchemaFileLeavesBuiltinsWorking(t *testing.T) {
	root := newDrive(t, `{"version": 1, "groups": [`)
	store := mustOpen(t, root)
	if got := len(store.Keys()); got != 4 {
		t.Fatalf("Keys() = %v, want the four built-ins", store.Keys())
	}
	problems := store.Problems()
	if len(problems) != 1 || !strings.Contains(problems[0], schemaFile) || !strings.Contains(problems[0], "svalbard apply") {
		t.Errorf("Problems() = %v, want the schema file named with the way to regenerate it", problems)
	}
	if err := store.Set(KeyContext, "fast"); err != nil {
		t.Errorf("Set() on a built-in failed: %v", err)
	}
}

func TestSetStoresOnlyChangedValuesAndResetRemovesThem(t *testing.T) {
	root := newDrive(t, testSchema)
	store := mustOpen(t, root)

	for key, value := range map[string]string{
		KeyContext:                "balanced",
		"open-webui.open_browser": "off",
		"radio-tool.gain":         "7.5",
		"radio-tool.station":      "OH2 base",
		"radio-tool.token":        "s3cret",
	} {
		if err := store.Set(key, value); err != nil {
			t.Fatalf("Set(%s, %s) error = %v", key, value, err)
		}
	}
	want := map[string]map[string]any{
		"ai":         {"context": "balanced"},
		"open-webui": {"open_browser": false},
		"radio-tool": {"gain": 7.5, "station": "OH2 base", "token": "s3cret"},
	}
	if got := readValues(t, root); !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %v, want %v", valuesFile, got, want)
	}
	if !store.Changed(KeyContext) || store.Changed(KeyDefaultModel) {
		t.Error("Changed() does not follow the saved values")
	}

	// A second store sees what the first saved.
	reopened := mustOpen(t, root)
	if reopened.String(KeyContext) != "balanced" || reopened.Bool("open-webui.open_browser") || reopened.Number("radio-tool.gain") != 7.5 {
		t.Error("saved values were not read back")
	}

	// Setting the default value and resetting both remove the key, and an
	// owner left without keys goes too.
	if err := store.Set("open-webui.open_browser", "on"); err != nil {
		t.Fatal(err)
	}
	if err := store.Reset(KeyContext); err != nil {
		t.Fatal(err)
	}
	if err := store.Reset("radio-tool.gain"); err != nil {
		t.Fatal(err)
	}
	want = map[string]map[string]any{"radio-tool": {"station": "OH2 base", "token": "s3cret"}}
	if got := readValues(t, root); !reflect.DeepEqual(got, want) {
		t.Fatalf("%s after reset = %v, want %v", valuesFile, got, want)
	}
	if store.String(KeyContext) != Ask || store.Changed(KeyContext) {
		t.Error("reset did not return ai.context to its default")
	}
}

func TestBoolAcceptsTheUsualSpellings(t *testing.T) {
	store := mustOpen(t, newDrive(t, testSchema))
	const key = "open-webui.open_browser"
	for text, want := range map[string]bool{
		"on": true, "true": true, "yes": true, "1": true, "ON": true,
		"off": false, "false": false, "no": false, "0": false, "Off": false,
	} {
		if err := store.Set(key, text); err != nil {
			t.Errorf("Set(%q) error = %v", text, err)
		} else if store.Bool(key) != want {
			t.Errorf("Set(%q) gave %v, want %v", text, store.Bool(key), want)
		}
	}
	setting, _ := store.Lookup(key)
	if setting.Format(true) != "on" || setting.Format(false) != "off" {
		t.Errorf("bools print as %q/%q, want on/off", setting.Format(true), setting.Format(false))
	}
}

func TestSetRejectsBadValuesNamingWhatIsValid(t *testing.T) {
	root := newDrive(t, testSchema)
	store := mustOpen(t, root)
	for _, tc := range []struct{ key, value, want string }{
		{KeyContext, "huge", `ai.context: "huge" is not one of ask, auto, fast, 16k, balanced, 64k, max, 256k, 512k, model-max`},
		{KeyDefaultModel, "llama-99", "is not one of ask, Qwen3.8-27B-UD-IQ4_XS, gemma-4-12B-it-qat"},
		{"open-webui.open_browser", "maybe", `"maybe" is not on or off`},
		{"radio-tool.band", "hf", "is not one of vhf, uhf"},
		{"radio-tool.gain", "loud", `"loud" is not a number (1 to 10)`},
		{"radio-tool.gain", "11", "11 is outside 1 to 10"},
		{"radio-tool.station", "two\nlines", "more than one line"},
		{"radio-tool.volume", "3", `unknown setting "radio-tool.volume"; settings on this drive: ai.default_model, ai.context, ai.web_search, ai.web_search_url, open-webui.open_browser`},
	} {
		err := store.Set(tc.key, tc.value)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("Set(%s, %q) error = %v, want it to contain %q", tc.key, tc.value, err, tc.want)
		}
		if err != nil && strings.Contains(err.Error(), "\n") {
			t.Errorf("Set(%s, %q) error is not one line: %q", tc.key, tc.value, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".svalbard", valuesFile)); !os.IsNotExist(err) {
		t.Errorf("a rejected value created %s: %v", valuesFile, err)
	}
}

func TestBadSavedValuesAreIgnoredAndReported(t *testing.T) {
	root := newDrive(t, testSchema)
	writeValues(t, root, `{
  "ai": {"context": "huge", "default_model": "gemma-4-12B-it-qat"},
  "open-webui": {"open_browser": "no"},
  "radio-tool": {"gain": 99, "band": 3, "station": "ok"}
}`)
	store := mustOpen(t, root)

	if got := store.String(KeyContext); got != Ask {
		t.Errorf("ai.context = %q, want the default for a value outside the choices", got)
	}
	if !store.Bool("open-webui.open_browser") {
		t.Error("open_browser took a string; want the default true")
	}
	if got := store.Number("radio-tool.gain"); got != 5 {
		t.Errorf("gain = %v, want the default 5 for a value above max", got)
	}
	if got := store.String("radio-tool.band"); got != "vhf" {
		t.Errorf("band = %q, want the default for a number", got)
	}
	// Valid neighbours are still honoured.
	if store.String(KeyDefaultModel) != "gemma-4-12B-it-qat" || store.String("radio-tool.station") != "ok" {
		t.Error("valid values beside the bad ones were dropped")
	}
	// An ignored value is not a change from the default.
	if store.Changed(KeyContext) {
		t.Error("an ignored value is marked as changed")
	}

	problems := strings.Join(store.Problems(), "\n")
	for _, want := range []string{
		`ai.context: saved value "huge" is not one of ask, auto, fast, 16k, balanced, 64k, max, 256k, 512k, model-max; using ask`,
		`open-webui.open_browser: saved value "no" is not true or false; using on`,
		`radio-tool.gain: saved value 99 is outside 1 to 10; using 5`,
		`radio-tool.band: saved value 3 is not one of vhf, uhf; using vhf`,
	} {
		if !strings.Contains(problems, want) {
			t.Errorf("Problems() lacks %q:\n%s", want, problems)
		}
	}
	if len(store.Problems()) != 4 {
		t.Errorf("Problems() has %d entries, want 4:\n%s", len(store.Problems()), problems)
	}
}

func TestUnknownOwnersAndKeysSurviveARewrite(t *testing.T) {
	root := newDrive(t, testSchema)
	writeValues(t, root, `{
  "removed-recipe": {"speed": 3, "nested": {"a": [1, 2]}},
  "notes": "mine",
  "ai": {"context": "fast", "future_switch": true},
  "radio-tool": {"band": "uhf", "retired": "x"}
}`)
	store := mustOpen(t, root)
	if err := store.Set(KeyContext, "max"); err != nil {
		t.Fatal(err)
	}
	if err := store.Reset("radio-tool.band"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".svalbard", valuesFile))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("rewritten file does not parse: %v\n%s", err, data)
	}
	want := map[string]any{
		"removed-recipe": map[string]any{"speed": 3.0, "nested": map[string]any{"a": []any{1.0, 2.0}}},
		"notes":          "mine",
		"ai":             map[string]any{"context": "max", "future_switch": true},
		"radio-tool":     map[string]any{"retired": "x"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("rewritten file = %v, want %v", got, want)
	}
}

func TestUnparsableValuesFileIsAnErrorAndIsNeverReplaced(t *testing.T) {
	for name, tc := range map[string]struct{ content, want string }{
		"syntax error":           {"{\n  \"ai\": {\"context\": \"fast\",}\n}\n", "line 2"},
		"cut short":              {`{"ai": {"context": "fa`, "unexpected end"},
		"not an object":          {`["ai.context", "fast"]`, "must be an object"},
		"known owner not a map":  {`{"ai": "fast"}`, `"ai" must be an object of settings`},
		"toml written by a hand": {"[ai]\ncontext = \"fast\"\n", "line 1"},
	} {
		t.Run(name, func(t *testing.T) {
			root := newDrive(t, testSchema)
			writeValues(t, root, tc.content)
			path := filepath.Join(root, ".svalbard", valuesFile)

			store, err := Open(root)
			if err == nil {
				t.Fatal("Open() error = nil for a file that does not parse")
			}
			if !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want the path %s and the reason %q", err, path, tc.want)
			}
			// The store still answers, with defaults, so an action can go on.
			if store == nil || store.String(KeyContext) != Ask || !store.Bool("open-webui.open_browser") {
				t.Fatal("the store of an unreadable file does not answer with defaults")
			}
			// It never writes: the user's file stays for them to fix.
			if err := store.Set(KeyContext, "max"); err == nil || !strings.Contains(err.Error(), path) {
				t.Errorf("Set() error = %v, want the parse error", err)
			}
			if err := store.Reset(KeyContext); err == nil {
				t.Error("Reset() succeeded on an unreadable file")
			}
			if got, _ := os.ReadFile(path); string(got) != tc.content {
				t.Errorf("the file was changed:\n%s", got)
			}
		})
	}
}

func TestOpenForActionWarnsInOneLineAndUsesDefaults(t *testing.T) {
	root := newDrive(t, testSchema)
	writeValues(t, root, `{"ai": {"context": "max"`)
	var warn bytes.Buffer
	store := OpenForAction(root, &warn)
	if store.String(KeyContext) != Ask {
		t.Errorf("ai.context = %q, want the default", store.String(KeyContext))
	}
	if out := warn.String(); !strings.Contains(out, valuesFile) || strings.Count(out, "\n") != 1 {
		t.Errorf("warning = %q, want one line naming %s", out, valuesFile)
	}

	warn.Reset()
	writeValues(t, root, `{"ai": {"context": "max"}}`)
	if store := OpenForAction(root, &warn); store.String(KeyContext) != "max" || warn.Len() != 0 {
		t.Errorf("readable file: context %q, warning %q", store.String(KeyContext), warn.String())
	}
}

func TestWriteIsAtomic(t *testing.T) {
	root := newDrive(t, testSchema)
	dir := filepath.Join(root, ".svalbard")
	path := filepath.Join(dir, valuesFile)
	store := mustOpen(t, root)
	if err := store.Set(KeyContext, "fast"); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(before, []byte("}\n")) {
		t.Errorf("file does not end with a newline: %q", before)
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644", info.Mode().Perm())
	}

	// The new content is written beside the file and renamed over it. With the
	// directory read-only neither step can happen, and the old file is whole.
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	if os.Geteuid() != 0 {
		if err := store.Set(KeyContext, "max"); err == nil {
			t.Fatal("Set() succeeded in a read-only directory")
		}
		if after, _ := os.ReadFile(path); !bytes.Equal(after, before) {
			t.Errorf("a failed write changed the file:\n%s", after)
		}
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := store.Set(KeyContext, "max"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != valuesFile && entry.Name() != schemaFile {
			t.Errorf("write left %s behind", entry.Name())
		}
	}
}

func TestSetKeepsAChangeMadeSinceOpen(t *testing.T) {
	root := newDrive(t, testSchema)
	menu := mustOpen(t, root)
	command := mustOpen(t, root)
	if err := command.Set(KeyContext, "fast"); err != nil {
		t.Fatal(err)
	}
	// The menu's store was opened before the command wrote.
	if err := menu.Set("open-webui.open_browser", "off"); err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]any{"ai": {"context": "fast"}, "open-webui": {"open_browser": false}}
	if got := readValues(t, root); !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %v, want both changes: %v", valuesFile, got, want)
	}
	if menu.String(KeyContext) != "fast" {
		t.Error("the store did not pick up the other writer's value")
	}
}

func TestSetOutsideADriveFailsWithoutCreatingOne(t *testing.T) {
	root := t.TempDir()
	store := mustOpen(t, root)
	err := store.Set(KeyContext, "fast")
	if err == nil || !strings.Contains(err.Error(), "not a Svalbard drive") {
		t.Fatalf("Set() error = %v, want it to say this is not a drive", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".svalbard")); !os.IsNotExist(err) {
		t.Error("Set() created .svalbard in a directory that is not a drive")
	}
}

func TestContextSizeFollowsTheSetting(t *testing.T) {
	root := newDrive(t, "")
	const auto = 16384
	for value, want := range map[string]int{
		"auto": auto, "fast": llamaserve.CtxFast, "16k": llamaserve.Ctx16K, "balanced": llamaserve.CtxBalanced,
		"64k": llamaserve.Ctx64K, "max": llamaserve.CtxMax, "256k": llamaserve.Ctx256K,
		"512k": llamaserve.Ctx512K, "model-max": llamaserve.CtxModelMax,
	} {
		writeValues(t, root, `{"ai": {"context": "`+value+`"}}`)
		asked := false
		got := mustOpen(t, root).ContextSize(auto, func(int) int { asked = true; return 1 })
		if got != want || asked {
			t.Errorf("ai.context=%s: size %d (asked: %v), want %d without asking", value, got, asked, want)
		}
	}

	// ask, the default, and an ignored value all leave it to the picker,
	// which is handed the host-adaptive size.
	for _, content := range []string{``, `{"ai": {"context": "ask"}}`, `{"ai": {"context": "huge"}}`} {
		writeValues(t, root, content)
		var offered int
		got := mustOpen(t, root).ContextSize(auto, func(autoCtx int) int { offered = autoCtx; return 4096 })
		if got != 4096 || offered != auto {
			t.Errorf("values %q: size %d, picker offered %d; want the picker's 4096 with %d offered", content, got, offered, auto)
		}
	}
}

func TestResolveModelUsesTheDefaultModelOnlyWhenNoneIsNamed(t *testing.T) {
	root := newDrive(t, "")
	first := filepath.Join(root, "models", "Qwen3.8-27B-UD-IQ4_XS.gguf")
	gemma := filepath.Join(root, "models", "gemma-4-12B-it-qat.gguf")

	var out bytes.Buffer
	// ask: exactly llamaserve.ResolveModel, which takes the first file.
	if got, err := mustOpen(t, root).ResolveModel(&out, ""); err != nil || got != first {
		t.Fatalf("ask: ResolveModel() = %q, %v; want %q", got, err, first)
	}

	writeValues(t, root, `{"ai": {"default_model": "gemma-4-12B-it-qat"}}`)
	store := mustOpen(t, root)
	if got, err := store.ResolveModel(&out, ""); err != nil || got != gemma {
		t.Fatalf("default model: ResolveModel() = %q, %v; want %q", got, err, gemma)
	}
	// A model the action names wins over the setting.
	if got, err := store.ResolveModel(&out, "Qwen3.8-27B-UD-IQ4_XS.gguf"); err != nil || got != first {
		t.Fatalf("named model: ResolveModel() = %q, %v; want %q", got, err, first)
	}
	if out.Len() != 0 {
		t.Errorf("unexpected output: %q", out.String())
	}
}

func TestResolveModelFallsBackWhenTheDefaultModelIsGone(t *testing.T) {
	root := newDrive(t, "")
	writeValues(t, root, `{"ai": {"default_model": "gemma-4-12B-it-qat"}}`)
	if err := os.Remove(filepath.Join(root, "models", "gemma-4-12B-it-qat.gguf")); err != nil {
		t.Fatal(err)
	}
	store := mustOpen(t, root)

	var out bytes.Buffer
	got, err := store.ResolveModel(&out, "")
	if want := filepath.Join(root, "models", "Qwen3.8-27B-UD-IQ4_XS.gguf"); err != nil || got != want {
		t.Fatalf("ResolveModel() = %q, %v; want today's pick %q", got, err, want)
	}
	if text := out.String(); !strings.Contains(text, "gemma-4-12B-it-qat is no longer on the drive") || strings.Count(text, "\n") != 1 {
		t.Errorf("output = %q, want one line naming the missing model", text)
	}
	// The command reports the same stale value.
	if problems := store.Problems(); len(problems) != 1 || !strings.Contains(problems[0], "ai.default_model") {
		t.Errorf("Problems() = %v, want the stale default model", problems)
	}

	// A value that is not a model name at all is ignored without a path lookup.
	writeValues(t, root, `{"ai": {"default_model": "../../outside"}}`)
	out.Reset()
	if _, err := mustOpen(t, root).ResolveModel(&out, ""); err != nil || out.Len() != 0 {
		t.Errorf("path-like value: error %v, output %q", err, out.String())
	}
}
