package openwebui

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/settings"
)

// recipeSchema is what apply generates from the open-webui recipe.
const recipeSchema = `{"version": 1, "groups": [{"owner": "open-webui", "label": "Open WebUI", "settings": [
  {"name": "open_browser", "type": "bool", "default": true, "label": "Open browser on start", "description": "Open the browser when Open WebUI is ready."},
  {"name": "file_tools", "type": "bool", "default": true, "label": "File tools", "description": "Let the assistant save and edit files in the drive's workspace folder."}
]}]}`

// schemaBeforeFileTools is the schema of a drive applied before the recipe
// declared file_tools.
const schemaBeforeFileTools = `{"version": 1, "groups": [{"owner": "open-webui", "label": "Open WebUI", "settings": [
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

func TestFileToolsFollowTheRecipeSetting(t *testing.T) {
	for name, tc := range map[string]struct {
		schema, values string
		want           bool
	}{
		"no schema":                   {"", "", true},
		"schema predates the setting": {schemaBeforeFileTools, `{"open-webui": {"file_tools": false}}`, true},
		"default":                     {recipeSchema, "", true},
		"switched off":                {recipeSchema, `{"open-webui": {"file_tools": false}}`, false},
		"switched on":                 {recipeSchema, `{"open-webui": {"file_tools": true}}`, true},
		"ignored value":               {recipeSchema, `{"open-webui": {"file_tools": "sometimes"}}`, true},
		// The two switches are independent.
		"only the browser is off": {recipeSchema, `{"open-webui": {"open_browser": false}}`, true},
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
			if got := fileToolsEnabled(prefs); got != tc.want {
				t.Errorf("fileToolsEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

// The workspace folder and its listener exist only when the setting is on.
func TestOpenFileToolsOpensNothingWhenTheSettingIsOff(t *testing.T) {
	driveRoot := t.TempDir()
	writeDriveFile(t, driveRoot, "settings.schema.json", recipeSchema)
	writeDriveFile(t, driveRoot, "settings.json", `{"open-webui": {"file_tools": false}}`)
	prefs, err := settings.Open(driveRoot)
	if err != nil {
		t.Fatal(err)
	}
	files, err := openFileTools(driveRoot, prefs)
	if files != nil || err != nil {
		t.Fatalf("openFileTools() = %+v, %v; want nothing", files, err)
	}
	if _, err := os.Stat(filepath.Join(driveRoot, "workspace")); err == nil {
		t.Error("the workspace folder was created although file tools are off")
	}

	if err := prefs.Set("open-webui.file_tools", "true"); err != nil {
		t.Fatal(err)
	}
	files, err = openFileTools(driveRoot, prefs)
	if err != nil || files == nil {
		t.Fatalf("openFileTools() = %+v, %v; want the workspace", files, err)
	}
	defer files.Close()
	if files.workspace.Dir() == "" || !strings.HasPrefix(files.url, "http://localhost:") {
		t.Errorf("file tools = %+v", files)
	}
	connection, err := net.DialTimeout("tcp", files.listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatalf("the workspace listener is not held: %v", err)
	}
	connection.Close()
}

// With file_tools off the launch is the one from before the file tools: one
// tool server and no workspace.
func TestRunOffersNoFileToolsWhenTheSettingIsOff(t *testing.T) {
	driveRoot, recordDir := newFakeDrive(t, "open-webui")
	writeDriveFile(t, driveRoot, "settings.schema.json", recipeSchema)
	writeDriveFile(t, driveRoot, "settings.json", `{"open-webui": {"file_tools": false}}`)
	t.Setenv("HOME", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdout := &syncBuffer{}
	opened := make(chan string, 1)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, stdout, driveRoot, "", func(url string) error { opened <- url; return nil })
	}()
	select {
	case <-opened:
	case err := <-done:
		t.Fatalf("Run() returned before opening the browser: %v\n%s", err, stdout.String())
	case <-time.After(30 * time.Second):
		t.Fatalf("browser was not opened\n%s", stdout.String())
	}

	var servers []struct {
		URL  string `json:"url"`
		Info struct {
			ID string `json:"id"`
		} `json:"info"`
	}
	var metadata struct {
		ToolIDs []string `json:"toolIds"`
	}
	for _, entry := range readRecord(t, recordDir, "open-webui").Env {
		if value, ok := strings.CutPrefix(entry, "TOOL_SERVER_CONNECTIONS="); ok {
			if err := json.Unmarshal([]byte(value), &servers); err != nil {
				t.Fatalf("TOOL_SERVER_CONNECTIONS = %s: %v", value, err)
			}
		}
		if value, ok := strings.CutPrefix(entry, "DEFAULT_MODEL_METADATA="); ok {
			if err := json.Unmarshal([]byte(value), &metadata); err != nil {
				t.Fatalf("DEFAULT_MODEL_METADATA = %s: %v", value, err)
			}
		}
	}
	if len(servers) != 1 || servers[0].Info.ID != mcpServerID {
		t.Fatalf("tool servers = %+v, want only the drive's", servers)
	}
	if want := []string{"server:mcp:" + mcpServerID}; !slices.Equal(metadata.ToolIDs, want) {
		t.Errorf("toolIds = %v, want %v", metadata.ToolIDs, want)
	}
	// The file tools are not mounted beside the drive's server either.
	filesEndpoint := strings.TrimSuffix(servers[0].URL, mcp.StreamableHTTPPath) + mcp.FilesStreamableHTTPPath
	response, err := http.Post(filesEndpoint, "application/json", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNotFound {
		t.Errorf("POST %s = %d, want 404", filesEndpoint, response.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(driveRoot, "workspace")); err == nil {
		t.Error("the workspace folder was created although file tools are off")
	}
	if out := stdout.String(); strings.Contains(out, "Workspace:") || strings.Contains(out, "File tools are off") {
		t.Errorf("output mentions the workspace although the setting is off:\n%s", out)
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
	preset := slices.Index(llama.Args, "--models-preset")
	if preset < 0 {
		t.Fatalf("missing router preset: %v", llama.Args)
	}
	config, err := os.ReadFile(llama.Args[preset+1])
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(config), "ctx-size = 32768\n"); got != 2 {
		t.Errorf("found context size 32768 in %d presets, want 2", got)
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
