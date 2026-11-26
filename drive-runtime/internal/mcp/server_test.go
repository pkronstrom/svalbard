package mcp

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	gomcp "github.com/mark3labs/mcp-go/mcp"
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
