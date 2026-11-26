package mcp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	gomcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/workspace"
)

// stubCap is a minimal Capability for testing.
type stubCap struct{}

func (s *stubCap) Tool() string { return "test_tool" }
func (s *stubCap) Close() error { return nil }
func (s *stubCap) Actions() []ActionDef {
	return []ActionDef{
		{Name: "ping", Desc: "Returns pong", Params: nil},
		{Name: "echo", Desc: "Echoes input", Params: []ParamDef{
			{Name: "text", Type: "string", Required: true, Desc: "Text to echo"},
		}},
	}
}

func (s *stubCap) Handle(_ context.Context, action string, params map[string]any) (ActionResult, error) {
	switch action {
	case "ping":
		return ActionResult{Text: "pong"}, nil
	case "echo":
		text, _ := params["text"].(string)
		return ActionResult{Text: text}, nil
	default:
		return ActionResult{}, fmt.Errorf("unknown action: %s", action)
	}
}

func TestRegisteredTools(t *testing.T) {
	srv := NewServer(&stubCap{})
	defer srv.Close()

	tools := srv.inner.ListTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	ping, ok := tools["test_tool_ping"]
	if !ok {
		t.Fatal("expected explicit ping tool to be registered")
	}
	echo, ok := tools["test_tool_echo"]
	if !ok {
		t.Fatal("expected explicit echo tool to be registered")
	}
	if ping.Tool.Description != "Returns pong" {
		t.Errorf("expected ping description 'Returns pong', got %q", ping.Tool.Description)
	}
	if echo.Tool.Description != "Echoes input" {
		t.Errorf("expected echo description 'Echoes input', got %q", echo.Tool.Description)
	}
	if got := echo.Tool.InputSchema.Required; len(got) != 1 || got[0] != "text" {
		t.Fatalf("echo required fields = %v, want [text]", got)
	}
	textProp, ok := echo.Tool.InputSchema.Properties["text"].(map[string]any)
	if !ok {
		t.Fatalf("echo text property missing or wrong type: %#v", echo.Tool.InputSchema.Properties["text"])
	}
	if textProp["type"] != "string" {
		t.Fatalf("echo text property type = %#v, want string", textProp["type"])
	}
	if echo.Tool.Annotations.ReadOnlyHint == nil || !*echo.Tool.Annotations.ReadOnlyHint {
		t.Fatal("expected readOnlyHint=true")
	}
}

func TestMultipleCapabilities(t *testing.T) {
	srv := NewServer(&stubCap{}, &stubCap{})
	defer srv.Close()

	// Both register under the same name, so mcp-go may deduplicate.
	// The important thing is it doesn't panic.
	if len(srv.inner.ListTools()) == 0 {
		t.Fatal("expected at least 1 tool")
	}
}

func TestCloseCallsCapabilities(t *testing.T) {
	cap := &stubCap{}
	srv := NewServer(cap)
	if err := srv.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAllCapabilitiesRegisterExpectedTools(t *testing.T) {
	root := t.TempDir()
	srv := NewServer(
		NewVaultCapability(root, DriveMetadata{}),
		NewQueryCapability(root, DriveMetadata{}),
		NewSearchCapability(root, DriveMetadata{}),
	)
	defer srv.Close()

	tools := srv.inner.ListTools()
	for _, name := range []string{
		"vault_sources",
		"vault_databases",
		"vault_maps",
		"vault_stats",
		"query_describe",
		"query_sql",
		"search",
		"search_read",
	} {
		if _, ok := tools[name]; !ok {
			t.Errorf("missing tool %q", name)
		}
	}
	if len(tools) != 8 {
		t.Fatalf("len(tools) = %d, want 8", len(tools))
	}
	searchTool := tools["search"].Tool
	if got := searchTool.InputSchema.Required; len(got) != 1 || got[0] != "query" {
		t.Fatalf("search required fields = %v, want [query]", got)
	}
	if _, ok := searchTool.InputSchema.Properties["source"]; !ok {
		t.Fatalf("search tool missing optional source filter in schema: %#v", searchTool.InputSchema.Properties)
	}
}

// The terminal harnesses use this server over stdio and bring their own file
// tools, so the workspace tools must not appear in it.
func TestDriveServerHasTheVaultToolsAndNoFileTools(t *testing.T) {
	root := t.TempDir()
	srv := NewServer(DriveCapabilities(root, DriveOptions{})...)
	defer srv.Close()

	var names []string
	for name := range srv.inner.ListTools() {
		names = append(names, name)
	}
	slices.Sort(names)
	want := []string{"query_describe", "query_sql", "search", "search_read", "vault_databases", "vault_maps", "vault_sources", "vault_stats"}
	if !slices.Equal(names, want) {
		t.Errorf("drive tools = %v, want %v", names, want)
	}
	listed := DriveToolNames(root, 0)
	slices.Sort(listed)
	if !slices.Equal(listed, want) {
		t.Errorf("DriveToolNames() = %v, want %v", listed, want)
	}
	if _, err := os.Stat(filepath.Join(root, workspace.DirName)); err == nil {
		t.Error("the drive server created a workspace folder")
	}
}

// Below the Balanced tier the wiki tools and a full read leave a small model
// no room to answer after a lookup (pi on the Fast tier): the server offers
// the wiki from the Balanced tier up and reads shorter pieces below it. A
// client that does not say its context gets everything.
func TestSmallContextOffersNoWikiAndShorterReads(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".svalbard"), 0o755); err != nil {
		t.Fatal(err)
	}
	recipes := `{"silverbullet": {"id": "silverbullet", "type": "binary"}, "skills-core": {"id": "skills-core", "type": "skills"}}`
	if err := os.WriteFile(filepath.Join(root, ".svalbard", "recipes.json"), []byte(recipes), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		context int
		wiki    bool
		read    ReadLimits
	}{
		{0, true, ReadLimits{readChars, readLinks, fullDetailChars}},
		{8192, false, ReadLimits{smallReadChars, smallReadLinks, smallFullDetailChars}},
		{32768, true, ReadLimits{readChars, readLinks, fullDetailChars}},
		{131072, true, ReadLimits{readChars, readLinks, fullDetailChars}},
	} {
		srv := NewDriveServer(root, DriveOptions{Context: tc.context})
		var names []string
		for _, tool := range srv.Tools() {
			names = append(names, tool.Name)
		}
		if got := slices.Contains(names, "wiki_create"); got != tc.wiki || WikiOffered(root, tc.context) != tc.wiki {
			t.Errorf("context %d: wiki tools %t, WikiOffered %t, want %t", tc.context, got, WikiOffered(root, tc.context), tc.wiki)
		}
		if !slices.Contains(names, "skill_search") || !slices.Contains(names, "search_read") {
			t.Errorf("context %d: tools = %v, want the search and skill tools", tc.context, names)
		}
		// Launchers that enable tools by name get the same list.
		listed := DriveToolNames(root, tc.context)
		slices.Sort(listed)
		if !slices.Equal(listed, names) {
			t.Errorf("context %d: DriveToolNames() = %v, want %v", tc.context, listed, names)
		}
		for _, cap := range srv.caps {
			switch cap := cap.(type) {
			case *SearchCapability:
				if cap.read != tc.read {
					t.Errorf("context %d: search_read size %+v, want %+v", tc.context, cap.read, tc.read)
				}
			case *SkillCapability:
				if cap.read != tc.read {
					t.Errorf("context %d: skill_read size %+v, want %+v", tc.context, cap.read, tc.read)
				}
			}
		}
		_ = srv.Close()
	}
}

func TestFileToolsAreMarkedAsWritingWhereTheyWrite(t *testing.T) {
	ws, err := workspace.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	srv := NewServer(NewFilesCapability(ws, ""))
	defer srv.Close()

	tools := srv.inner.ListTools()
	if len(tools) != 4 {
		t.Fatalf("len(tools) = %d, want 4", len(tools))
	}
	for name, writes := range map[string]bool{"file_list": false, "file_read": false, "file_write": true, "file_edit": true} {
		tool, ok := tools[name]
		if !ok {
			t.Errorf("missing tool %q", name)
			continue
		}
		hints := tool.Tool.Annotations
		if *hints.ReadOnlyHint == writes || *hints.DestructiveHint != writes {
			t.Errorf("%s: readOnlyHint=%v destructiveHint=%v, want writes=%v", name, *hints.ReadOnlyHint, *hints.DestructiveHint, writes)
		}
	}
	if got := tools["file_edit"].Tool.InputSchema.Required; !slices.Equal(got, []string{"path", "old_text", "new_text"}) {
		t.Errorf("file_edit required fields = %v", got)
	}
}

// Open WebUI shows each path as a tool server of its own, which is what lets a
// chat switch the file tools off and keep vault search.
func TestServeStreamableHTTPServesEachServerAtItsPath(t *testing.T) {
	ws, err := workspace.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	drive := NewServer(&stubCap{})
	defer drive.Close()
	files := NewServer(NewFilesCapability(ws, "http://127.0.0.1:8086"))
	defer files.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	go func() {
		served <- ServeStreamableHTTP(ctx, listener, map[string]*Server{StreamableHTTPPath: drive, FilesStreamableHTTPPath: files})
	}()

	connect := func(path string) *mcpclient.Client {
		t.Helper()
		client, err := mcpclient.NewStreamableHttpClient("http://" + listener.Addr().String() + path)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.Start(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := client.Initialize(ctx, gomcp.InitializeRequest{}); err != nil {
			t.Fatalf("Initialize(%s) error = %v", path, err)
		}
		return client
	}
	toolNames := func(client *mcpclient.Client) []string {
		t.Helper()
		tools, err := client.ListTools(ctx, gomcp.ListToolsRequest{})
		if err != nil {
			t.Fatalf("ListTools() error = %v", err)
		}
		var names []string
		for _, tool := range tools.Tools {
			names = append(names, tool.Name)
		}
		slices.Sort(names)
		return names
	}

	driveClient := connect(StreamableHTTPPath)
	defer driveClient.Close()
	if got := toolNames(driveClient); !slices.Equal(got, []string{"test_tool_echo", "test_tool_ping"}) {
		t.Errorf("tools at %s = %v", StreamableHTTPPath, got)
	}
	filesClient := connect(FilesStreamableHTTPPath)
	defer filesClient.Close()
	if got := toolNames(filesClient); !slices.Equal(got, []string{"file_edit", "file_list", "file_read", "file_write"}) {
		t.Errorf("tools at %s = %v", FilesStreamableHTTPPath, got)
	}

	call := gomcp.CallToolRequest{}
	call.Params.Name = "file_write"
	call.Params.Arguments = map[string]any{"path": "notes/page.html", "content": "<h1>Hi</h1>"}
	result, err := filesClient.CallTool(ctx, call)
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if text, ok := result.Content[0].(gomcp.TextContent); !ok || text.Text != `{"path":"notes/page.html","bytes":11,"link":"[notes/page.html](http://127.0.0.1:8086/notes/page.html)"}` {
		t.Fatalf("file_write result = %#v", result.Content)
	}
	if data, err := os.ReadFile(filepath.Join(ws.Dir(), "notes", "page.html")); err != nil || string(data) != "<h1>Hi</h1>" {
		t.Fatalf("written file = %q, %v", data, err)
	}
	// A refused path is a tool error the model can read, not a protocol error.
	call.Params.Arguments = map[string]any{"path": "../page.html", "content": "x"}
	result, err = filesClient.CallTool(ctx, call)
	if err != nil || !result.IsError {
		t.Fatalf("file_write outside the workspace: result = %#v, error = %v; want a tool error", result, err)
	}
	if text, ok := result.Content[0].(gomcp.TextContent); !ok || !strings.Contains(text.Text, "notes/page.html") {
		t.Errorf("tool error = %#v, want it to say what to send instead", result.Content)
	}

	// A page on some other site must not be able to write files. A browser
	// sends a cross-site POST without asking only when its content type is a
	// plain form type, and a name rebound to this address keeps its own Host.
	post := func(body string, change func(*http.Request)) *http.Response {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, "http://"+listener.Addr().String()+FilesStreamableHTTPPath, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		change(request)
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response
	}
	session := post(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`,
		func(*http.Request) {}).Header.Get("Mcp-Session-Id")
	body := `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"file_write","arguments":{"path":"planted.html","content":"x"}}}`
	planted := filepath.Join(ws.Dir(), "planted.html")
	for name, change := range map[string]func(*http.Request){
		"form content type": func(r *http.Request) { r.Header.Set("Content-Type", "text/plain") },
		"foreign Host":      func(r *http.Request) { r.Host = "evil.example" },
	} {
		response := post(body, func(r *http.Request) {
			r.Header.Set("Mcp-Session-Id", session)
			change(r)
		})
		if response.StatusCode != http.StatusBadRequest && response.StatusCode != http.StatusForbidden {
			t.Errorf("POST with %s: status = %d, want it refused", name, response.StatusCode)
		}
		if response.Header.Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("POST with %s: the server allows cross-origin reads", name)
		}
		if _, err := os.Stat(planted); err == nil {
			t.Fatalf("POST with %s wrote a file", name)
		}
	}
	// The same request sent as a local client sends it does write, so each
	// refusal above is about the one header it changed.
	response := post(body, func(r *http.Request) { r.Header.Set("Mcp-Session-Id", session) })
	if _, err := os.Stat(planted); response.StatusCode != http.StatusOK || err != nil {
		t.Errorf("POST as a local client: status = %d, file: %v; want it written", response.StatusCode, err)
	}

	cancel()
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("ServeStreamableHTTP() error = %v, want nil after cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ServeStreamableHTTP() did not return after cancellation")
	}
	if _, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second); err == nil {
		t.Fatal("listener still accepts connections after cancellation")
	}
}

func TestServeStreamableHTTPListsAndCallsTools(t *testing.T) {
	srv := NewServer(&stubCap{})
	defer srv.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- srv.ServeStreamableHTTP(ctx, listener) }()

	client, err := mcpclient.NewStreamableHttpClient("http://" + listener.Addr().String() + StreamableHTTPPath)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Initialize(ctx, gomcp.InitializeRequest{}); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}
	tools, err := client.ListTools(ctx, gomcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(tools.Tools) != 2 {
		t.Fatalf("tools = %d, want 2", len(tools.Tools))
	}
	call := gomcp.CallToolRequest{}
	call.Params.Name = "test_tool_echo"
	call.Params.Arguments = map[string]any{"text": "hello"}
	result, err := client.CallTool(ctx, call)
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if text, ok := result.Content[0].(gomcp.TextContent); !ok || text.Text != "hello" {
		t.Fatalf("echo result = %#v, want hello", result.Content)
	}

	cancel()
	select {
	case err := <-served:
		if err != nil {
			t.Fatalf("ServeStreamableHTTP() error = %v, want nil after cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ServeStreamableHTTP() did not return after cancellation")
	}
	if _, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second); err == nil {
		t.Fatal("listener still accepts connections after cancellation")
	}
}
