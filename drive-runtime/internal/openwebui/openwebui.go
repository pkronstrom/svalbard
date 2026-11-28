// Package openwebui starts Open WebUI from the drive: a llama-server backend,
// the drive's MCP server over Streamable HTTP, and Open WebUI itself, all on
// localhost with every setting supplied by environment. Unless file tools are
// off it also offers the drive's workspace folder: tools to read and write it,
// and a read-only server so a reply can link to what was written.
package openwebui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/browser"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/contextpicker"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/llamaserve"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/settings"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/workspace"
)

const (
	recipeID = "open-webui"

	// mcpServerID names the drive's tool server inside Open WebUI, which
	// prefixes every tool with it ("svalbard_search").
	mcpServerID = "svalbard"

	// filesServerID names the file tools the same way ("workspace_file_write").
	// They are a tool server of their own so a chat can switch them off and
	// keep vault search.
	filesServerID = "workspace"
	webServerID   = "web"
	webMCPPath    = "/web/mcp"

	// These two are declared by the open-webui recipe.
	settingOpenBrowser = recipeID + ".open_browser"
	settingFileTools   = recipeID + ".file_tools"
)

// pickContext is a package var so tests can skip the terminal picker.
var pickContext = contextpicker.Pick

// Launch is one start of Open WebUI: where its state lives and which local
// servers it talks to.
type Launch struct {
	RuntimeRoot string // <drive>/.svalbard/runtime/open-webui
	// BytecodeDir is where Python writes compiled modules, on this computer;
	// empty writes none.
	BytecodeDir       string
	SecretKey         string
	ModelName         string // id llama-server reports for the model
	ContextSize       int    // token limit configured on llama-server
	LlamaURL          string // OpenAI-compatible base URL, ending in /v1
	MCPURL            string // Streamable HTTP endpoint of the drive's MCP server
	Wiki              bool   // the drive's MCP server offers the wiki tools
	FilesURL          string // Streamable HTTP endpoint of the file tools; empty when they are off
	WebSearchURL      string
	WebSearchProvider string
	PDFUploadURL      string
	PDFUploadToken    string
	Port              int
}

// Environment returns everything Open WebUI needs from its environment. It is
// the complete environment apart from locale and PATH: Open WebUI reads
// several hundred variables, and one inherited from the host (a second API
// endpoint, a proxy, a database URL) would send chats off the drive.
//
// Persistent config is off, so these values win on every start and ports may
// change between runs. Chats and each user's own settings are database rows
// and still persist.
func Environment(l Launch) map[string]string {
	origin := fmt.Sprintf("http://localhost:%d;http://127.0.0.1:%d", l.Port, l.Port)
	// The wiki tools are part of the vault's server: in a chat they switch
	// on and off together with vault search.
	vault := "Search and read the content on this drive"
	if l.Wiki {
		vault += ", and read and write the pages of its wiki"
	}
	toolServers := []any{toolServer(mcpServerID, "Svalbard vault", vault, l.MCPURL)}
	toolIDs := []string{"server:mcp:" + mcpServerID}
	if l.FilesURL != "" {
		toolServers = append(toolServers, toolServer(filesServerID, "Svalbard files", "Read and save files in the drive's workspace folder", l.FilesURL))
		toolIDs = append(toolIDs, "server:mcp:"+filesServerID)
	}
	if l.WebSearchURL != "" {
		toolServers = append(toolServers, toolServer(webServerID, "Web search ("+l.WebSearchProvider+")", "Send public web search queries to "+l.WebSearchProvider, l.WebSearchURL))
		toolIDs = append(toolIDs, "server:mcp:"+webServerID)
	}
	capabilities := map[string]any{
		"file_upload": true, "vision": false,
		"web_search": false, "code_interpreter": false, "image_generation": false, "memory": false,
	}
	// Summarization is another request within the same model context. Leave
	// room for its prompt and for vault tool results added during a turn.
	compactionThreshold := strconv.Itoa(l.ContextSize * 7 / 10)
	env := map[string]string{
		// State. STATIC_DIR is rewritten on every start and would otherwise be
		// inside the installed package; the temporary directory holds uploads
		// in flight.
		"HOME":             filepath.Join(l.RuntimeRoot, "home"),
		"XDG_CONFIG_HOME":  filepath.Join(l.RuntimeRoot, "config"),
		"XDG_CACHE_HOME":   filepath.Join(l.RuntimeRoot, "cache"),
		"XDG_DATA_HOME":    filepath.Join(l.RuntimeRoot, "share"),
		"TMPDIR":           filepath.Join(l.RuntimeRoot, "tmp"),
		"DATA_DIR":         filepath.Join(l.RuntimeRoot, "data"),
		"STATIC_DIR":       filepath.Join(l.RuntimeRoot, "static"),
		"WEBUI_SECRET_KEY": l.SecretKey,

		// The slim dependency set is installed; this switch stops Open WebUI
		// importing what the full set would provide.
		"USE_SLIM_DOCKER":          "true",
		"OFFLINE_MODE":             "true",
		"WEBUI_AUTH":               "False",
		"ENABLE_PERSISTENT_CONFIG": "False",
		// Without sign-in any page in the host's browser could otherwise read
		// the chats through the default "*" origin.
		"CORS_ALLOW_ORIGIN": origin,

		"ENABLE_OLLAMA_API":   "False",
		"ENABLE_OPENAI_API":   "True",
		"OPENAI_API_BASE_URL": l.LlamaURL,
		"OPENAI_API_KEY":      "local",
		"OPENAI_API_CONFIGS": mustJSON(map[string]any{"0": map[string]any{
			"enable": true, "provider": "llama.cpp", "connection_type": "local", "auth_type": "none",
		}}),
		"DEFAULT_MODELS":                     l.ModelName,
		"ENABLE_CONTEXT_COMPACTION":          "True",
		"CONTEXT_COMPACTION_TOKEN_THRESHOLD": compactionThreshold,
		"CONTEXT_COMPACTION_TOKEN_CAP":       compactionThreshold,
		// A model still loading from a slow drive answers the list late.
		"AIOHTTP_CLIENT_TIMEOUT_MODEL_LIST": "30",

		"TOOL_SERVER_CONNECTIONS": mustJSON(toolServers),
		"DEFAULT_MODEL_METADATA": mustJSON(map[string]any{
			// Projector-enabled models override this through the model list.
			"capabilities": capabilities,
			// Tool servers listed here are switched on in a new chat.
			"toolIds":      toolIDs,
			"builtinTools": builtinTools(),
		}),

		// Each of these is a second model request queued behind the user's turn
		// on a one-slot server.
		"ENABLE_TITLE_GENERATION":           "False",
		"ENABLE_TAGS_GENERATION":            "False",
		"ENABLE_FOLLOW_UP_GENERATION":       "False",
		"ENABLE_AUTOCOMPLETE_GENERATION":    "False",
		"ENABLE_RETRIEVAL_QUERY_GENERATION": "False",
		"ENABLE_SEARCH_QUERY_GENERATION":    "False",
		"ENABLE_IMAGE_PROMPT_GENERATION":    "False",

		// Store extracted text without building a vector index.
		"BYPASS_EMBEDDING_AND_RETRIEVAL": "True",
	}
	if l.PDFUploadURL != "" {
		// Native browser chats can page through stored text without an
		// embedding model. Keep that path for both PDFs and old text files.
		env["CONTENT_EXTRACTION_ENGINE"] = "external"
		env["EXTERNAL_DOCUMENT_LOADER_URL"] = l.PDFUploadURL
		env["EXTERNAL_DOCUMENT_LOADER_API_KEY"] = l.PDFUploadToken
		env["RAG_FILE_MAX_SIZE"] = strconv.Itoa(pdfUploadLimitMiB)
		env["VIEW_FILE_DEFAULT_MAX_CHARS"] = strconv.Itoa(min(l.ContextSize/4, 10000))
		env["DEFAULT_INTERFACE_SETTINGS"] = mustJSON(map[string]string{
			"system": "For files attached to this chat, read the stored text with view_file, or use grep_chat_files for keywords. Cite the labeled PDF page when available.",
		})
	}
	// Bytecode stays off the stick, where up to 8,311 compiled modules took
	// 1.10 GB at 128 KB clusters.
	if l.BytecodeDir != "" {
		env["PYTHONPYCACHEPREFIX"] = l.BytecodeDir
	} else {
		env["PYTHONDONTWRITEBYTECODE"] = "1"
	}
	for _, feature := range []string{
		"WEB_SEARCH", "CODE_INTERPRETER", "CODE_EXECUTION", "MEMORIES", "CALENDAR", "AUTOMATIONS",
		"CHANNELS", "COMMUNITY_SHARING", "MESSAGE_RATING", "USER_STATUS",
	} {
		env["ENABLE_"+feature] = "False"
	}
	return env
}

// toolServer is one MCP connection in the shape Open WebUI stores.
func toolServer(id, name, description, url string) map[string]any {
	return map[string]any{
		"type": "mcp", "url": url, "path": "", "auth_type": "none", "key": "",
		"config": map[string]any{"enable": true},
		"info":   map[string]any{"id": id, "name": name, "description": description},
	}
}

// builtinTools keeps the two built-in tool categories that work offline with
// no back end. Every enabled category adds its tool definitions to each
// request, and unlisted categories default to on.
func builtinTools() map[string]bool {
	tools := map[string]bool{"time": true, "files": true}
	for _, category := range []string{
		"user_input", "knowledge", "chats", "users", "subagents", "memory", "web_search",
		"image_generation", "video_generation", "code_interpreter", "notes", "channels", "skills",
		"tools", "tasks", "automations", "calendar", "notifications", "visualizations",
	} {
		tools[category] = false
	}
	return tools
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

// Run starts the three servers, opens the browser once Open WebUI answers,
// and stops everything when ctx ends or any server exits.
func Run(ctx context.Context, stdout io.Writer, driveRoot, selected string, opener func(string) error) error {
	if opener == nil {
		opener = browser.Open
	}
	return run(ctx, stdout, driveRoot, selected, pickContext, func(prefs *settings.Store, url string) error {
		if opensBrowser(prefs) {
			return opener(url)
		}
		return nil
	})
}

// RunForBrowser publishes the ready URL to the dashboard and uses Auto when
// the user's context setting would otherwise prompt in the terminal.
func RunForBrowser(ctx context.Context, stdout io.Writer, driveRoot, selected string, ready func(string) error) error {
	return run(ctx, stdout, driveRoot, selected, func(auto int) int { return auto },
		func(_ *settings.Store, url string) error { return ready(url) })
}

func run(ctx context.Context, stdout io.Writer, driveRoot, selected string, pick func(int) int, ready func(*settings.Store, string) error) error {
	platformName, err := platform.Detect()
	if err != nil {
		return err
	}
	launcher, err := resolveLauncher(driveRoot, platformName)
	if err != nil {
		return err
	}
	prefs := settings.OpenForAction(driveRoot, stdout)
	model, err := prefs.ResolveModel(stdout, selected)
	if err != nil {
		return err
	}
	llamaBin, err := binary.Resolve("llama-server", driveRoot)
	if err != nil {
		return fmt.Errorf("llama-server not found")
	}

	runtimeRoot := filepath.Join(driveRoot, ".svalbard", "runtime", recipeID)
	// Uploads in flight do not survive a restart.
	if err := os.RemoveAll(filepath.Join(runtimeRoot, "tmp")); err != nil {
		return err
	}
	// Where earlier runtimes kept bytecode on the stick.
	if err := os.RemoveAll(filepath.Join(runtimeRoot, "pycache")); err != nil {
		return err
	}
	for _, dir := range []string{"home", "config", "cache", "share", "tmp", "data", "static"} {
		if err := os.MkdirAll(filepath.Join(runtimeRoot, dir), 0o755); err != nil {
			return err
		}
	}
	secret, err := loadSecretKey(filepath.Join(runtimeRoot, "secret"))
	if err != nil {
		return err
	}

	requestedContext := prefs.ContextSize(llamaserve.ContextForHost(), pick)
	ctxSize := llamaserve.ContextForModel(model, requestedContext)
	var modelPreset string
	var models []string
	if selected == "" {
		models, err = llamaserve.ChatModels(driveRoot)
		if err != nil {
			return err
		}
		if len(models) > 1 {
			var minimumContext int
			modelPreset, minimumContext, err = llamaserve.WriteRouterPresets(filepath.Join(runtimeRoot, "tmp"), filepath.Join(driveRoot, "models"), models, requestedContext)
			if err != nil {
				fmt.Fprintf(stdout, "Model switching unavailable, starting the default model: %v\n", err)
			} else {
				// WebUI's default compaction threshold and the shared tools
				// also have to work with the smallest model window.
				ctxSize = minimumContext
			}
		}
	}

	// The MCP listener is held from here on. The two child servers bind their
	// own ports later, so their searches must not land on the same free port.
	mcpListener, mcpPort, err := netutil.Listen("127.0.0.1", 8090)
	if err != nil {
		return err
	}
	defer mcpListener.Close()
	files, err := openFileTools(driveRoot, prefs)
	if err != nil {
		// The workspace is an extra; chat and vault search work without it.
		fmt.Fprintf(stdout, "File tools are off: %v\n", err)
	}
	if files != nil {
		defer files.Close()
	}
	llamaPort, err := netutil.FindAvailablePort("127.0.0.1", 8082)
	if err != nil {
		return err
	}
	port, err := netutil.FindAvailablePort("127.0.0.1", 8084)
	if err == nil && port == llamaPort {
		port, err = netutil.FindAvailablePort("127.0.0.1", llamaPort+1)
	}
	if err != nil {
		return err
	}
	modelName := strings.TrimSuffix(filepath.Base(model), filepath.Ext(model))
	llamaRoot := fmt.Sprintf("http://127.0.0.1:%d", llamaPort)
	webuiRoot := fmt.Sprintf("http://127.0.0.1:%d", port)
	visionModels := map[string]bool{}
	if modelPreset != "" {
		for _, name := range models {
			if llamaserve.VisionProjector(filepath.Join(driveRoot, "models", name)) != "" {
				visionModels[strings.TrimSuffix(name, filepath.Ext(name))] = true
			}
		}
	} else if llamaserve.VisionProjector(model) != "" {
		visionModels[modelName] = true
	}

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	defer func() {
		cancel()
		wg.Wait()
	}()
	exited := make(chan error, 5)
	var pdfURL, pdfToken string
	uploads, err := startPDFUploads(ctx, driveRoot, platformName, ctxSize)
	if err != nil {
		fmt.Fprintln(stdout, "PDF uploads are off:", err)
	} else {
		pdfURL, pdfToken = uploads.URL, uploads.Token
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-uploads.done
		}()
	}

	webuiBackend := llamaRoot
	if len(visionModels) > 0 {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("starting image-capable model connection: %w", err)
		}
		target := &url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", llamaPort)}
		proxy := &http.Server{Handler: visionProxy(target, visionModels), ReadHeaderTimeout: 5 * time.Second}
		defer listener.Close()
		defer proxy.Close()
		webuiBackend = "http://" + listener.Addr().String()
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := proxy.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
				exited <- fmt.Errorf("model connection stopped: %w", err)
			}
		}()
	}

	mcpServer := mcp.NewDriveServer(driveRoot, mcp.DriveOptions{Context: ctxSize})
	mcpServers := map[string]*mcp.Server{mcp.StreamableHTTPPath: mcpServer}
	var webURL, webProvider string
	web, err := mcp.WebSearchForSettings(prefs)
	if err != nil {
		fmt.Fprintln(stdout, "Web search is off:", err)
	} else if web != nil {
		webProvider = web.Provider()
		webURL = fmt.Sprintf("http://127.0.0.1:%d%s", mcpPort, webMCPPath)
		mcpServers[webMCPPath] = mcp.NewServer(mcp.NewWebCapability(web))
		fmt.Fprintf(stdout, "Web search is enabled. Public search queries go to %s.\n", webProvider)
	}
	filesURL := ""
	if files != nil {
		capability := mcp.NewFilesCapability(files.workspace, files.url)
		if mcp.LibrariesInstalled(driveRoot) {
			capability = capability.WithHTMLPacking(driveRoot)
		}
		mcpServers[mcp.FilesStreamableHTTPPath] = mcp.NewServer(capability)
		filesURL = fmt.Sprintf("http://127.0.0.1:%d%s", mcpPort, mcp.FilesStreamableHTTPPath)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := files.workspace.Serve(ctx, files.listener); err != nil {
				exited <- fmt.Errorf("workspace server stopped: %w", err)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer func() {
			for _, server := range mcpServers {
				_ = server.Close()
			}
		}()
		if err := mcp.ServeStreamableHTTP(ctx, mcpListener, mcpServers); err != nil {
			exited <- fmt.Errorf("MCP server stopped: %w", err)
		}
	}()

	llamaArgs := []string{"--jinja", "--host", "127.0.0.1", "--port", fmt.Sprintf("%d", llamaPort)}
	if modelPreset != "" {
		fmt.Fprintf(stdout, "Starting llama-server with %d switchable models (shared context floor %d)\n", len(models), ctxSize)
		llamaArgs = append(llamaArgs, "--models-preset", modelPreset, "--models-max", "1")
	} else {
		fmt.Fprintf(stdout, "Starting llama-server with %s (context %d)\n", modelName, ctxSize)
		// Keep the same model ID in saved chats and in the picker.
		llamaArgs = append(llamaArgs, "-m", model, "--alias", modelName)
		llamaArgs = append(llamaArgs, llamaserve.ExtraFlags(model, ctxSize)...)
	}
	llamaCmd := exec.CommandContext(ctx, llamaBin, llamaArgs...)
	if modelPreset != "" {
		// The router also scans the host's llama.cpp cache by default. Set
		// its cache to this run's directory so only vault models appear.
		llamaCmd.Env = append(os.Environ(), "LLAMA_CACHE="+filepath.Join(runtimeRoot, "tmp", "llama-cache"))
	}
	if err := start(llamaCmd, runtimeRoot, "llama-server", stdout, &wg, exited); err != nil {
		return err
	}

	fmt.Fprintln(stdout, "Starting Open WebUI")
	webuiCmd := exec.CommandContext(ctx, launcher, "serve", "--host", "127.0.0.1", "--port", fmt.Sprintf("%d", port))
	webuiCmd.Dir = runtimeRoot
	webuiCmd.Env = append(hostEnvironment(), platform.EnvList(Environment(Launch{
		RuntimeRoot:       runtimeRoot,
		BytecodeDir:       bytecodeDir(),
		SecretKey:         secret,
		ModelName:         modelName,
		ContextSize:       ctxSize,
		LlamaURL:          webuiBackend + "/v1",
		MCPURL:            fmt.Sprintf("http://127.0.0.1:%d%s", mcpPort, mcp.StreamableHTTPPath),
		Wiki:              mcp.WikiOffered(driveRoot, ctxSize),
		FilesURL:          filesURL,
		WebSearchURL:      webURL,
		WebSearchProvider: webProvider,
		PDFUploadURL:      pdfURL,
		PDFUploadToken:    pdfToken,
		Port:              port,
	}))...)
	if err := start(webuiCmd, runtimeRoot, recipeID, stdout, &wg, exited); err != nil {
		return err
	}

	// A large model read from a slow drive takes minutes, so there is no
	// deadline: the wait ends when both answer, one exits, or the user stops.
	fmt.Fprintln(stdout, "Waiting for the model and Open WebUI to be ready (Ctrl+C to stop)...")
	for _, url := range []string{llamaRoot + "/health", webuiRoot + "/health"} {
		if err := waitReady(ctx, url, exited); err != nil {
			return stopped(ctx, err)
		}
	}

	url := fmt.Sprintf("http://localhost:%d", port)
	if err := ready(prefs, url); err != nil {
		fmt.Fprintf(stdout, "  Open: %s\n", url)
	}
	fmt.Fprintf(stdout, "Open WebUI: %s\n", url)
	if files != nil {
		// Not opened: the links in a reply lead here.
		fmt.Fprintf(stdout, "Workspace:  %s/ (files the assistant saves, in %s)\n", files.url, files.workspace.Dir())
	}
	fmt.Fprintln(stdout, "Press Ctrl+C to stop.")

	select {
	case err := <-exited:
		return stopped(ctx, err)
	case <-ctx.Done():
		return nil
	}
}

// fileTools is the workspace folder and the listener its files are served on.
// The listener is a different origin from Open WebUI, so a page the model
// wrote cannot read the chats.
type fileTools struct {
	workspace *workspace.Workspace
	listener  net.Listener
	url       string // root of the served workspace, no trailing slash
}

// openFileTools returns nil when file tools are off for this launch.
func openFileTools(driveRoot string, prefs *settings.Store) (*fileTools, error) {
	if !fileToolsEnabled(prefs) {
		return nil, nil
	}
	ws, err := workspace.Open(filepath.Join(driveRoot, workspace.DirName))
	if err != nil {
		return nil, err
	}
	listener, port, err := netutil.Listen("127.0.0.1", 8086)
	if err != nil {
		ws.Close()
		return nil, err
	}
	// The link is spelled with "localhost": a 4B model copying
	// "127.0.0.1:8086" into its reply dropped digits often enough to matter.
	return &fileTools{workspace: ws, listener: listener, url: fmt.Sprintf("http://localhost:%d", port)}, nil
}

func (f *fileTools) Close() {
	f.listener.Close()
	f.workspace.Close()
}

// fileToolsEnabled reads the recipe's file_tools setting, and is the one
// place that decides whether a launch offers the workspace file tools. A
// drive whose schema file predates the setting offers them, as the default
// does.
func fileToolsEnabled(prefs *settings.Store) bool {
	_, declared := prefs.Lookup(settingFileTools)
	return !declared || prefs.Bool(settingFileTools)
}

// opensBrowser reads the recipe's open_browser setting. A drive whose schema
// file predates the setting opens the browser, as it always did.
func opensBrowser(prefs *settings.Store) bool {
	_, declared := prefs.Lookup(settingOpenBrowser)
	return !declared || prefs.Bool(settingOpenBrowser)
}

// stopped turns a server's exit into the action's result. Ctrl+C reaches the
// servers and this process together, so an exit just ahead of the
// cancellation is the user stopping, not a failure.
func stopped(ctx context.Context, err error) error {
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(500 * time.Millisecond):
		return err
	}
}

// resolveLauncher returns the wrapper the python-package build wrote for this
// platform. PATH is not searched: a host install would be a different version
// with the full dependency set.
func resolveLauncher(driveRoot, platformName string) (string, error) {
	launcher := filepath.Join(driveRoot, "bin", platformName, recipeID)
	if info, err := os.Stat(launcher); err != nil || info.IsDir() {
		return "", fmt.Errorf("Open WebUI is not installed for %s", platformName)
	}
	return launcher, nil
}

// bytecodeDir is where Python's compiled modules go on this computer: the
// same directory the drive's Python commands use (svalbard/pycache in the user
// cache dir), or none when there is no cache dir or it is macOS's /tmp.
func bytecodeDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(base, "svalbard", "pycache")
	if runtime.GOOS == "darwin" && (strings.HasPrefix(dir, "/tmp/") || strings.HasPrefix(dir, "/private/tmp/")) {
		return ""
	}
	return dir
}

// loadSecretKey returns the key that signs Open WebUI's sessions, creating it
// on first use. Open WebUI would otherwise write one into its working
// directory, and a key that changed between runs would sign the user out of
// an open tab.
func loadSecretKey(path string) (string, error) {
	if data, err := os.ReadFile(path); err == nil {
		if key := strings.TrimSpace(string(data)); key != "" {
			return key, nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	key := hex.EncodeToString(raw)
	if err := os.WriteFile(path, []byte(key+"\n"), 0o600); err != nil {
		return "", err
	}
	return key, nil
}

// hostEnvironment is what Open WebUI inherits from the host: the PATH its
// wrapper script needs, and the locale and time zone.
func hostEnvironment() []string {
	var env []string
	for _, key := range []string{"PATH", "LANG", "LC_ALL", "LC_CTYPE", "TZ"} {
		if value, ok := os.LookupEnv(key); ok {
			env = append(env, key+"="+value)
		}
	}
	return env
}

// start runs cmd with its output in <runtimeRoot>/<name>.log and reports its
// exit on exited. On cancellation the process gets SIGTERM, so llama-server
// and Open WebUI's database close cleanly, and is killed if it lingers.
func start(cmd *exec.Cmd, runtimeRoot, name string, stdout io.Writer, wg *sync.WaitGroup, exited chan<- error) error {
	logPath := filepath.Join(runtimeRoot, name+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("starting %s: %w", name, err)
	}
	fmt.Fprintf(stdout, "  log: %s\n", logPath)
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer logFile.Close()
		err := cmd.Wait()
		exited <- fmt.Errorf("%s stopped (%v); see %s", name, err, logPath)
	}()
	return nil
}

// waitReady polls url until it answers 200, a server exits, or ctx ends.
func waitReady(ctx context.Context, url string, exited <-chan error) error {
	client := &http.Client{Timeout: 2 * time.Second}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		if resp, err := client.Get(url); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case err := <-exited:
			return err
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
