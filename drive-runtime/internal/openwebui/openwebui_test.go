package openwebui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	gomcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

// fakeRoleEnv turns this test binary into a stand-in for llama-server or
// Open WebUI: the drive's launchers exec it with the real arguments.
const (
	fakeRoleEnv   = "OPENWEBUI_TEST_FAKE"
	fakeRecordEnv = "OPENWEBUI_TEST_RECORD"
)

func TestMain(m *testing.M) {
	if role := os.Getenv(fakeRoleEnv); role != "" {
		runFakeServer(role)
		return
	}
	os.Exit(m.Run())
}

type fakeRecord struct {
	Args []string `json:"args"`
	Env  []string `json:"env"`
	Dir  string   `json:"dir"`
}

// runFakeServer records how it was started, answers /health on --port, and
// leaves a marker when it is asked to stop. The "crash" role exits at once.
func runFakeServer(role string) {
	recordDir := os.Getenv(fakeRecordEnv)
	dir, _ := os.Getwd()
	data, _ := json.Marshal(fakeRecord{Args: os.Args[1:], Env: os.Environ(), Dir: dir})
	_ = os.WriteFile(filepath.Join(recordDir, role+".json"), data, 0o644)
	if role == "crash" {
		fmt.Println("fake server failed to start")
		os.Exit(3)
	}

	port := os.Args[slices.Index(os.Args, "--port")+1]
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM)
	go func() {
		<-stop
		_ = os.WriteFile(filepath.Join(recordDir, role+".stopped"), nil, 0o644)
		os.Exit(0)
	}()
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, `{"status":true}`) })
	_ = http.ListenAndServe("127.0.0.1:"+port, nil)
}

// newFakeDrive lays out a drive with one model and launchers that exec this
// test binary in the given roles.
func newFakeDrive(t *testing.T, webuiRole string) (driveRoot, recordDir string) {
	t.Helper()
	driveRoot, recordDir = t.TempDir(), t.TempDir()
	platformName, err := platform.Detect()
	if err != nil {
		t.Skip(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(driveRoot, "bin", platformName)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, role := range map[string]string{"llama-server": "llama-server", "open-webui": webuiRole} {
		script := fmt.Sprintf("#!/bin/sh\n%s=%s %s='%s' exec '%s' \"$@\"\n", fakeRoleEnv, role, fakeRecordEnv, recordDir, self)
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(driveRoot, "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(driveRoot, "models", "Qwen3.5-4B-Q4_K_M.gguf"), []byte("gguf"), 0o644); err != nil {
		t.Fatal(err)
	}

	previous := pickContext
	pickContext = func(auto int) int { return 8192 }
	t.Cleanup(func() { pickContext = previous })
	return driveRoot, recordDir
}

func readRecord(t *testing.T, recordDir, role string) fakeRecord {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(recordDir, role+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var record fakeRecord
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	return record
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func TestEnvironmentConfiguresALocalSignedOutSlimInstance(t *testing.T) {
	root := filepath.Join("/drive", ".svalbard", "runtime", "open-webui")
	env := Environment(Launch{
		RuntimeRoot: root,
		SecretKey:   "key",
		ModelName:   "Qwen3.5-4B-Q4_K_M",
		LlamaURL:    "http://127.0.0.1:8082/v1",
		MCPURL:      "http://127.0.0.1:8090/mcp",
		FilesURL:    "http://127.0.0.1:8090/files/mcp",
		Port:        8084,
	})

	for key, want := range map[string]string{
		"USE_SLIM_DOCKER":          "true",
		"OFFLINE_MODE":             "true",
		"WEBUI_AUTH":               "False",
		"ENABLE_PERSISTENT_CONFIG": "False",
		"ENABLE_OLLAMA_API":        "False",
		"ENABLE_OPENAI_API":        "True",
		"OPENAI_API_BASE_URL":      "http://127.0.0.1:8082/v1",
		"OPENAI_API_KEY":           "local",
		"DEFAULT_MODELS":           "Qwen3.5-4B-Q4_K_M",
		"WEBUI_SECRET_KEY":         "key",
		"CORS_ALLOW_ORIGIN":        "http://localhost:8084;http://127.0.0.1:8084",
		// Background model calls queue behind the user's turn.
		"ENABLE_TITLE_GENERATION":           "False",
		"ENABLE_TAGS_GENERATION":            "False",
		"ENABLE_FOLLOW_UP_GENERATION":       "False",
		"ENABLE_AUTOCOMPLETE_GENERATION":    "False",
		"ENABLE_RETRIEVAL_QUERY_GENERATION": "False",
		"ENABLE_SEARCH_QUERY_GENERATION":    "False",
		"ENABLE_WEB_SEARCH":                 "False",
		"ENABLE_CODE_INTERPRETER":           "False",
	} {
		if got := env[key]; got != want {
			t.Errorf("env[%q] = %q, want %q", key, got, want)
		}
	}

	// Everything Open WebUI keeps stays below the drive's runtime directory.
	for _, key := range []string{
		"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "TMPDIR",
		"DATA_DIR", "STATIC_DIR",
	} {
		if got := env[key]; !strings.HasPrefix(got, root+string(filepath.Separator)) {
			t.Errorf("env[%q] = %q, want a path below %s", key, got, root)
		}
	}

	var connections map[string]map[string]any
	if err := json.Unmarshal([]byte(env["OPENAI_API_CONFIGS"]), &connections); err != nil {
		t.Fatalf("OPENAI_API_CONFIGS: %v", err)
	}
	if got := connections["0"]["provider"]; got != "llama.cpp" {
		t.Errorf("connection provider = %v, want llama.cpp", got)
	}

	var servers []struct {
		Type   string `json:"type"`
		URL    string `json:"url"`
		Config struct {
			Enable bool `json:"enable"`
		} `json:"config"`
		Info struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"info"`
	}
	if err := json.Unmarshal([]byte(env["TOOL_SERVER_CONNECTIONS"]), &servers); err != nil {
		t.Fatalf("TOOL_SERVER_CONNECTIONS: %v", err)
	}
	if len(servers) != 2 || servers[0].Type != "mcp" || servers[0].URL != "http://127.0.0.1:8090/mcp" || !servers[0].Config.Enable {
		t.Fatalf("tool servers = %+v, want the drive's MCP server enabled first", servers)
	}
	// The file tools are a server of their own, so a chat can switch them off.
	if servers[1].Type != "mcp" || servers[1].URL != "http://127.0.0.1:8090/files/mcp" || !servers[1].Config.Enable || servers[1].Info.Name != "Svalbard files" {
		t.Fatalf("tool servers = %+v, want the file tools enabled second", servers)
	}
	if servers[0].Info.ID == servers[1].Info.ID {
		t.Fatalf("both tool servers have the id %q", servers[0].Info.ID)
	}

	var metadata struct {
		Capabilities map[string]bool `json:"capabilities"`
		ToolIDs      []string        `json:"toolIds"`
		BuiltinTools map[string]bool `json:"builtinTools"`
	}
	if err := json.Unmarshal([]byte(env["DEFAULT_MODEL_METADATA"]), &metadata); err != nil {
		t.Fatalf("DEFAULT_MODEL_METADATA: %v", err)
	}
	// Open WebUI addresses an MCP connection as server:mcp:<info.id>, and the
	// ids listed here are on in a new chat.
	if want := []string{"server:mcp:" + servers[0].Info.ID, "server:mcp:" + servers[1].Info.ID}; !slices.Equal(metadata.ToolIDs, want) {
		t.Errorf("toolIds = %v, want %v", metadata.ToolIDs, want)
	}
	for _, category := range []string{"knowledge", "memory", "web_search", "code_interpreter", "image_generation"} {
		if enabled, listed := metadata.BuiltinTools[category]; !listed || enabled {
			t.Errorf("builtin tool category %q must be listed as off (unlisted means on)", category)
		}
	}
	if metadata.Capabilities["vision"] || metadata.Capabilities["image_generation"] {
		t.Errorf("capabilities = %v, want vision and image generation off", metadata.Capabilities)
	}
}

func TestEnvironmentDescribesTheWikiOnlyWhenItsToolsAreOffered(t *testing.T) {
	description := func(l Launch) string {
		t.Helper()
		var servers []struct {
			Info struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"info"`
		}
		if err := json.Unmarshal([]byte(Environment(l)["TOOL_SERVER_CONNECTIONS"]), &servers); err != nil || len(servers) == 0 {
			t.Fatalf("TOOL_SERVER_CONNECTIONS: %v", err)
		}
		if servers[0].Info.Name != "Svalbard vault" {
			t.Fatalf("first tool server = %q, want Svalbard vault", servers[0].Info.Name)
		}
		return servers[0].Info.Description
	}
	launch := Launch{MCPURL: "http://127.0.0.1:8090/mcp", FilesURL: "http://127.0.0.1:8090/files/mcp", Port: 8084}
	if got, want := description(launch), "Search and read the content on this drive"; got != want {
		t.Errorf("description = %q, want %q", got, want)
	}
	// The wiki tools have no switch of their own: they are the vault's.
	launch.Wiki = true
	if got, want := description(launch), "Search and read the content on this drive, and read and write the pages of its wiki"; got != want {
		t.Errorf("description with the wiki = %q, want %q", got, want)
	}
	var servers []json.RawMessage
	if err := json.Unmarshal([]byte(Environment(launch)["TOOL_SERVER_CONNECTIONS"]), &servers); err != nil || len(servers) != 2 {
		t.Errorf("tool servers with the wiki = %d (%v), want the same two", len(servers), err)
	}
}

func TestEnvironmentWithoutFileToolsOffersOnlyTheVault(t *testing.T) {
	env := Environment(Launch{MCPURL: "http://127.0.0.1:8090/mcp", Port: 8084})

	var servers []struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(env["TOOL_SERVER_CONNECTIONS"]), &servers); err != nil {
		t.Fatalf("TOOL_SERVER_CONNECTIONS: %v", err)
	}
	if len(servers) != 1 || servers[0].URL != "http://127.0.0.1:8090/mcp" {
		t.Errorf("tool servers = %+v, want only the drive's MCP server", servers)
	}
	var metadata struct {
		ToolIDs []string `json:"toolIds"`
	}
	if err := json.Unmarshal([]byte(env["DEFAULT_MODEL_METADATA"]), &metadata); err != nil {
		t.Fatalf("DEFAULT_MODEL_METADATA: %v", err)
	}
	if want := []string{"server:mcp:" + mcpServerID}; !slices.Equal(metadata.ToolIDs, want) {
		t.Errorf("toolIds = %v, want %v", metadata.ToolIDs, want)
	}
}

func TestLoadSecretKeyIsCreatedOnceAndPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret")
	first, err := loadSecretKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) < 32 {
		t.Fatalf("key %q is too short", first)
	}
	second, err := loadSecretKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("secret key changed between starts")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("secret mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestRunFailsWithoutInstalledOpenWebUI(t *testing.T) {
	driveRoot, _ := newFakeDrive(t, "open-webui")
	platformName, _ := platform.Detect()
	if err := os.Remove(filepath.Join(driveRoot, "bin", platformName, "open-webui")); err != nil {
		t.Fatal(err)
	}
	err := Run(context.Background(), &syncBuffer{}, driveRoot, "", func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "Open WebUI is not installed for "+platformName) {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestRunStartsEverythingAndStopsItOnCancel(t *testing.T) {
	driveRoot, recordDir := newFakeDrive(t, "open-webui")
	// A hosted endpoint in the host's environment must not reach Open WebUI.
	t.Setenv("OPENAI_API_BASE_URLS", "https://api.example.invalid/v1")
	t.Setenv("HOME", t.TempDir())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stdout := &syncBuffer{}
	// Bytecode an earlier runtime kept on the stick.
	stalePyc := filepath.Join(driveRoot, ".svalbard", "runtime", "open-webui", "pycache", "mod.cpython-312.pyc")
	if err := os.MkdirAll(filepath.Dir(stalePyc), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stalePyc, []byte("pyc"), 0o644); err != nil {
		t.Fatal(err)
	}
	opened := make(chan string, 2)
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, stdout, driveRoot, "", func(url string) error { opened <- url; return nil })
	}()

	var url string
	select {
	case url = <-opened:
	case err := <-done:
		t.Fatalf("Run() returned before opening the browser: %v\n%s", err, stdout.String())
	case <-time.After(30 * time.Second):
		t.Fatalf("browser was not opened\n%s", stdout.String())
	}
	if !strings.HasPrefix(url, "http://localhost:") {
		t.Fatalf("opened %q, want a localhost URL", url)
	}

	runtimeRoot := filepath.Join(driveRoot, ".svalbard", "runtime", "open-webui")
	llama := readRecord(t, recordDir, "llama-server")
	webui := readRecord(t, recordDir, "open-webui")

	llamaArgs := strings.Join(llama.Args, " ")
	for _, want := range []string{
		"-m " + filepath.Join(driveRoot, "models", "Qwen3.5-4B-Q4_K_M.gguf"),
		"--alias Qwen3.5-4B-Q4_K_M", "--jinja", "--host 127.0.0.1",
		// llamaserve.ExtraFlags: the picked context, one slot, the Qwen sampler.
		"--ctx-size 8192", "--parallel 1", "--temp 0.6",
	} {
		if !strings.Contains(llamaArgs, want) {
			t.Errorf("llama-server args %q missing %q", llamaArgs, want)
		}
	}
	llamaPort := llama.Args[slices.Index(llama.Args, "--port")+1]

	if got := strings.Join(webui.Args[:3], " "); got != "serve --host 127.0.0.1" {
		t.Errorf("open-webui args = %v, want serve bound to 127.0.0.1", webui.Args)
	}
	if webui.Args[slices.Index(webui.Args, "--port")+1] != strings.TrimPrefix(url, "http://localhost:") {
		t.Errorf("open-webui port %v does not match opened URL %s", webui.Args, url)
	}
	if resolved, _ := filepath.EvalSymlinks(runtimeRoot); webui.Dir != resolved && webui.Dir != runtimeRoot {
		t.Errorf("open-webui working directory = %q, want %q", webui.Dir, runtimeRoot)
	}
	env := map[string]string{}
	for _, entry := range webui.Env {
		key, value, _ := strings.Cut(entry, "=")
		env[key] = value
	}
	if _, inherited := env["OPENAI_API_BASE_URLS"]; inherited {
		t.Error("open-webui inherited OPENAI_API_BASE_URLS from the host")
	}
	if got, want := env["OPENAI_API_BASE_URL"], "http://127.0.0.1:"+llamaPort+"/v1"; got != want {
		t.Errorf("OPENAI_API_BASE_URL = %q, want %q", got, want)
	}
	if got, want := env["HOME"], filepath.Join(runtimeRoot, "home"); got != want {
		t.Errorf("HOME = %q, want %q", got, want)
	}
	if prefix := env["PYTHONPYCACHEPREFIX"]; strings.HasPrefix(prefix, driveRoot) || (prefix == "" && env["PYTHONDONTWRITEBYTECODE"] != "1") {
		t.Errorf("bytecode goes to %q on the drive (no-write %q)", prefix, env["PYTHONDONTWRITEBYTECODE"])
	}
	if _, err := os.Stat(filepath.Dir(stalePyc)); !os.IsNotExist(err) {
		t.Errorf("bytecode left on the drive: %v", err)
	}
	if env["DEFAULT_MODELS"] != "Qwen3.5-4B-Q4_K_M" {
		t.Errorf("DEFAULT_MODELS = %q", env["DEFAULT_MODELS"])
	}
	secret, err := os.ReadFile(filepath.Join(runtimeRoot, "secret"))
	if err != nil || strings.TrimSpace(string(secret)) != env["WEBUI_SECRET_KEY"] || env["WEBUI_SECRET_KEY"] == "" {
		t.Errorf("WEBUI_SECRET_KEY does not match %s (%v)", filepath.Join(runtimeRoot, "secret"), err)
	}

	// The MCP server Open WebUI was pointed at is the drive's own, serving now.
	var servers []struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(env["TOOL_SERVER_CONNECTIONS"]), &servers); err != nil || len(servers) != 2 {
		t.Fatalf("TOOL_SERVER_CONNECTIONS = %q (%v)", env["TOOL_SERVER_CONNECTIONS"], err)
	}
	for _, server := range servers {
		request, _ := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(
			`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("MCP initialize at %s: %v", server.URL, err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("MCP initialize at %s: status = %d", server.URL, response.StatusCode)
		}
	}

	if out := stdout.String(); !strings.Contains(out, "Open WebUI: "+url) {
		t.Errorf("output does not name the URL:\n%s", out)
	}

	// The file tools write into the drive's workspace folder, and the link
	// they return is served on an origin of its own, printed but not opened.
	files, err := mcpclient.NewStreamableHttpClient(servers[1].URL)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	if err := files.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := files.Initialize(ctx, gomcp.InitializeRequest{}); err != nil {
		t.Fatalf("file tools Initialize() error = %v", err)
	}
	call := gomcp.CallToolRequest{}
	call.Params.Name = "file_write"
	call.Params.Arguments = map[string]any{"path": "notes/page.html", "content": "<h1>Hi</h1>"}
	result, err := files.CallTool(ctx, call)
	if err != nil || result.IsError {
		t.Fatalf("file_write: result = %#v, error = %v", result, err)
	}
	var written struct {
		Link string `json:"link"`
		URL  string
	}
	if text, ok := result.Content[0].(gomcp.TextContent); !ok || json.Unmarshal([]byte(text.Text), &written) != nil {
		t.Fatalf("file_write result = %#v, want JSON", result.Content)
	}
	var found bool
	if written.URL, found = strings.CutPrefix(strings.TrimSuffix(written.Link, ")"), "[notes/page.html]("); !found {
		t.Fatalf("file_write link = %q, want a Markdown link", written.Link)
	}
	if data, err := os.ReadFile(filepath.Join(driveRoot, "workspace", "notes", "page.html")); err != nil || string(data) != "<h1>Hi</h1>" {
		t.Fatalf("workspace file = %q, %v", data, err)
	}
	workspaceRoot := strings.TrimSuffix(written.URL, "/notes/page.html")
	if !strings.HasPrefix(workspaceRoot, "http://localhost:") || workspaceRoot == url {
		t.Errorf("workspace is served at %q, want a localhost origin other than Open WebUI's %s", workspaceRoot, url)
	}
	page, err := http.Get(written.URL)
	if err != nil {
		t.Fatalf("GET %s: %v", written.URL, err)
	}
	body, _ := io.ReadAll(page.Body)
	page.Body.Close()
	if page.StatusCode != http.StatusOK || string(body) != "<h1>Hi</h1>" {
		t.Errorf("GET %s = %d %q", written.URL, page.StatusCode, body)
	}
	if policy := page.Header.Get("Content-Security-Policy"); !strings.Contains(policy, "default-src 'none'") {
		t.Errorf("Content-Security-Policy = %q", policy)
	}
	if out := stdout.String(); !strings.Contains(out, "Workspace:  "+workspaceRoot+"/") {
		t.Errorf("output does not name the workspace URL %s:\n%s", workspaceRoot, out)
	}
	select {
	case extra := <-opened:
		t.Errorf("a second page was opened: %s", extra)
	default:
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
	for _, role := range []string{"llama-server", "open-webui"} {
		if _, err := os.Stat(filepath.Join(recordDir, role+".stopped")); err != nil {
			t.Errorf("%s was not asked to stop: %v", role, err)
		}
	}
	for _, endpoint := range []string{"http://127.0.0.1:" + llamaPort + "/health", strings.Replace(url, "localhost", "127.0.0.1", 1) + "/health", servers[0].URL, servers[1].URL, written.URL} {
		if response, err := http.Get(endpoint); err == nil {
			response.Body.Close()
			t.Errorf("%s still answers after Run() returned", endpoint)
		}
	}
}

// The vault's tool server carries the wiki tools on a drive that has the
// wiki, and its description in the chat's Tools menu says so.
func TestRunTellsOpenWebUIAboutTheWikiOfADriveThatHasOne(t *testing.T) {
	driveRoot, recordDir := newFakeDrive(t, "open-webui")
	recipes := `{"silverbullet": {"id": "silverbullet", "type": "binary", "description": "SilverBullet wiki", "tags": []}}`
	if err := os.MkdirAll(filepath.Join(driveRoot, ".svalbard"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(driveRoot, ".svalbard", "recipes.json"), []byte(recipes), 0o644); err != nil {
		t.Fatal(err)
	}

	// The wiki's tools are offered from the Balanced tier up; on the Fast
	// tier they would leave a small model no room to answer.
	for ctxSize, want := range map[int]bool{8192: false, 32768: true} {
		pickContext = func(int) int { return ctxSize } // newFakeDrive restores it
		ctx, cancel := context.WithCancel(context.Background())
		stdout := &syncBuffer{}
		opened := make(chan string, 1)
		done := make(chan error, 1)
		go func() {
			done <- Run(ctx, stdout, driveRoot, "", func(url string) error { opened <- url; return nil })
		}()
		select {
		case <-opened:
		case err := <-done:
			cancel()
			t.Fatalf("Run() returned before opening the browser: %v\n%s", err, stdout.String())
		case <-time.After(30 * time.Second):
			cancel()
			t.Fatalf("browser was not opened\n%s", stdout.String())
		}

		described := false
		for _, entry := range readRecord(t, recordDir, "open-webui").Env {
			if value, ok := strings.CutPrefix(entry, "TOOL_SERVER_CONNECTIONS="); ok {
				var servers []struct {
					Info struct {
						Description string `json:"description"`
					} `json:"info"`
				}
				if err := json.Unmarshal([]byte(value), &servers); err != nil || len(servers) != 2 {
					t.Fatalf("TOOL_SERVER_CONNECTIONS = %s (%v), want the vault and the file tools", value, err)
				}
				described = strings.Contains(servers[0].Info.Description, "read and write the pages of its wiki")
			}
		}
		if described != want {
			t.Errorf("context %d: the Svalbard vault description mentions the wiki: %t, want %t", ctxSize, described, want)
		}
		// Starting Open WebUI is not using the wiki: no page folder appears.
		if _, err := os.Stat(filepath.Join(driveRoot, "wiki")); err == nil {
			t.Error("starting Open WebUI created the wiki folder")
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
}

// A stick whose workspace cannot be created (read-only, or the name is taken)
// still gets chat and vault search.
func TestRunContinuesWithoutFileToolsWhenTheWorkspaceCannotBeOpened(t *testing.T) {
	driveRoot, recordDir := newFakeDrive(t, "open-webui")
	if err := os.WriteFile(filepath.Join(driveRoot, "workspace"), []byte("not a folder"), 0o644); err != nil {
		t.Fatal(err)
	}

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

	for _, entry := range readRecord(t, recordDir, "open-webui").Env {
		if value, ok := strings.CutPrefix(entry, "TOOL_SERVER_CONNECTIONS="); ok {
			var servers []struct {
				URL string `json:"url"`
			}
			if err := json.Unmarshal([]byte(value), &servers); err != nil || len(servers) != 1 || strings.Contains(servers[0].URL, "files") {
				t.Errorf("TOOL_SERVER_CONNECTIONS = %s (%v), want only the drive's MCP server", value, err)
			}
		}
	}
	out := stdout.String()
	if !strings.Contains(out, "File tools are off: ") || strings.Contains(out, "Workspace:") {
		t.Errorf("output does not say file tools are off:\n%s", out)
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

func TestRunStopsTheModelServerWhenOpenWebUIFailsToStart(t *testing.T) {
	driveRoot, recordDir := newFakeDrive(t, "crash")

	opened := false
	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), &syncBuffer{}, driveRoot, "", func(string) error { opened = true; return nil })
	}()

	var err error
	select {
	case err = <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("Run() did not return after Open WebUI exited")
	}
	logPath := filepath.Join(driveRoot, ".svalbard", "runtime", "open-webui", "open-webui.log")
	if err == nil || !strings.Contains(err.Error(), "open-webui stopped") || !strings.Contains(err.Error(), logPath) {
		t.Fatalf("Run() error = %v, want the exit and the log path", err)
	}
	if opened {
		t.Error("browser opened although Open WebUI never became ready")
	}
	if log, _ := os.ReadFile(logPath); !strings.Contains(string(log), "fake server failed to start") {
		t.Errorf("log %s does not hold the server's output: %q", logPath, log)
	}
	if _, err := os.Stat(filepath.Join(recordDir, "llama-server.stopped")); err != nil {
		t.Errorf("llama-server was left running: %v", err)
	}
}

// Compiled modules go to this computer's cache, not the stick, where up to
// 8,311 of them took 1.10 GB at 128 KB clusters.
func TestEnvironmentKeepsBytecodeOffTheDrive(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".svalbard", "runtime", "open-webui")
	env := Environment(Launch{RuntimeRoot: root, BytecodeDir: "/home/u/.cache/svalbard/pycache", Port: 8084})
	if got := env["PYTHONPYCACHEPREFIX"]; got != "/home/u/.cache/svalbard/pycache" {
		t.Errorf("PYTHONPYCACHEPREFIX = %q", got)
	}
	if _, ok := env["PYTHONDONTWRITEBYTECODE"]; ok {
		t.Error("bytecode switched off although there is a cache dir")
	}
	env = Environment(Launch{RuntimeRoot: root, Port: 8084})
	if _, ok := env["PYTHONPYCACHEPREFIX"]; ok || env["PYTHONDONTWRITEBYTECODE"] != "1" {
		t.Errorf("without a cache dir: prefix %q, no-write %q", env["PYTHONPYCACHEPREFIX"], env["PYTHONDONTWRITEBYTECODE"])
	}

	t.Setenv("HOME", "/home/u")
	t.Setenv("XDG_CACHE_HOME", "")
	want := map[string]string{"darwin": "/home/u/Library/Caches/svalbard/pycache", "linux": "/home/u/.cache/svalbard/pycache"}[runtime.GOOS]
	if got := bytecodeDir(); want != "" && got != want {
		t.Errorf("bytecodeDir() = %q, want %q", got, want)
	}
}
