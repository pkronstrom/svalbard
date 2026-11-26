package openwebui

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/settings"
)

// recipeSchema is what apply generates from the open-webui recipe.
const recipeSchema = `{"version": 1, "groups": [{"owner": "open-webui", "label": "Open WebUI", "settings": [
  {"name": "open_browser", "type": "bool", "default": true, "label": "Open browser on start", "description": "Open the browser when Open WebUI is ready."}
]}]}`

func writeDriveFile(t *testing.T, driveRoot, name, content string) {
	t.Helper()
	path := filepath.Join(driveRoot, ".svalbard", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpensBrowserFollowsTheRecipeSetting(t *testing.T) {
	for name, tc := range map[string]struct {
		schema, values string
		want           bool
	}{
		"schema predates the setting": {"", "", true},
		"value saved without schema":  {"", `{"open-webui": {"open_browser": false}}`, true},
		"default":                     {recipeSchema, "", true},
		"switched off":                {recipeSchema, `{"open-webui": {"open_browser": false}}`, false},
		"switched on":                 {recipeSchema, `{"open-webui": {"open_browser": true}}`, true},
		"ignored value":               {recipeSchema, `{"open-webui": {"open_browser": "never"}}`, true},
	} {
		t.Run(name, func(t *testing.T) {
			driveRoot := t.TempDir()
			if tc.schema != "" {
				writeDriveFile(t, driveRoot, "settings.schema.json", tc.schema)
			}
			if tc.values != "" {
				writeDriveFile(t, driveRoot, "settings.json", tc.values)
			}
			prefs, err := settings.Open(driveRoot)
			if err != nil {
				t.Fatal(err)
			}
			if got := opensBrowser(prefs); got != tc.want {
				t.Errorf("opensBrowser() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRunHonoursTheDriveSettings(t *testing.T) {
	driveRoot, recordDir := newFakeDrive(t, "open-webui")
	if err := os.WriteFile(filepath.Join(driveRoot, "models", "Zeta-12B.gguf"), []byte("gguf"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDriveFile(t, driveRoot, "settings.schema.json", recipeSchema)
	writeDriveFile(t, driveRoot, "settings.json",
		`{"ai": {"context": "balanced", "default_model": "Zeta-12B"}, "open-webui": {"open_browser": false}}`)
	picked := false
	pickContext = func(int) int { picked = true; return 8192 } // newFakeDrive restores it
	t.Setenv("HOME", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdout := &syncBuffer{}
	opened := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, stdout, driveRoot, "", func(url string) error { opened <- url; return nil })
	}()

	// "Press Ctrl+C" is printed after the point where the browser would open.
	deadline := time.After(30 * time.Second)
	for !strings.Contains(stdout.String(), "Press Ctrl+C to stop.") {
		select {
		case err := <-done:
			t.Fatalf("Run() returned early: %v\n%s", err, stdout.String())
		case <-deadline:
			t.Fatalf("Open WebUI did not become ready\n%s", stdout.String())
		case <-time.After(20 * time.Millisecond):
		}
	}
	select {
	case url := <-opened:
		t.Errorf("the browser was opened on %s although open_browser is off", url)
	default:
	}
	// The URL is still printed for the user to open themselves.
	if out := stdout.String(); !strings.Contains(out, "Open WebUI: http://localhost:") {
		t.Errorf("output does not name the URL:\n%s", out)
	}

	if picked {
		t.Error("the context picker was shown although ai.context is balanced")
	}
	llama := readRecord(t, recordDir, "llama-server")
	if got := llama.Args[slices.Index(llama.Args, "--ctx-size")+1]; got != "32768" {
		t.Errorf("--ctx-size = %q, want 32768", got)
	}
	if got, want := llama.Args[slices.Index(llama.Args, "-m")+1], filepath.Join(driveRoot, "models", "Zeta-12B.gguf"); got != want {
		t.Errorf("-m = %q, want the default model %q", got, want)
	}
	webui := readRecord(t, recordDir, "open-webui")
	if !slices.Contains(webui.Env, "DEFAULT_MODELS=Zeta-12B") {
		t.Error("Open WebUI was not given the default model")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v after cancellation, want nil", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Run() did not return after cancellation")
	}
}
