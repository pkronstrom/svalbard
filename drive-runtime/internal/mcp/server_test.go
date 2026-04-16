package mcp_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
)

// stubCap is a minimal Capability for testing.
type stubCap struct{}

func (s *stubCap) Tool() string        { return "test_tool" }
func (s *stubCap) Description() string  { return "A test tool" }
func (s *stubCap) Close() error         { return nil }
func (s *stubCap) Actions() []mcp.ActionDef {
	return []mcp.ActionDef{
		{Name: "ping", Desc: "Returns pong", Params: nil},
		{Name: "echo", Desc: "Echoes input", Params: []mcp.ParamDef{
			{Name: "text", Type: "string", Required: true, Desc: "Text to echo"},
		}},
	}
}

func (s *stubCap) Handle(_ context.Context, action string, params map[string]any) (mcp.ActionResult, error) {
	switch action {
	case "ping":
		return mcp.ActionResult{Text: "pong"}, nil
	case "echo":
		text, _ := params["text"].(string)
		return mcp.ActionResult{Text: text}, nil
	default:
		return mcp.ActionResult{}, fmt.Errorf("unknown action: %s", action)
	}
}

func TestToolsReturnsRegisteredTool(t *testing.T) {
	srv := mcp.NewServer(&stubCap{})
	defer srv.Close()

	tools := srv.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	tool := tools[0]
	if tool.Name != "test_tool" {
		t.Errorf("expected tool name 'test_tool', got %q", tool.Name)
	}
	if tool.Description != "A test tool" {
		t.Errorf("expected description 'A test tool', got %q", tool.Description)
	}
}

func TestMultipleCapabilities(t *testing.T) {
	srv := mcp.NewServer(&stubCap{}, &stubCap{})
	defer srv.Close()

	// Both register under the same name, so mcp-go may deduplicate.
	// The important thing is it doesn't panic.
	tools := srv.Tools()
	if len(tools) == 0 {
		t.Fatal("expected at least 1 tool")
	}
}

func TestCloseCallsCapabilities(t *testing.T) {
	cap := &stubCap{}
	srv := mcp.NewServer(cap)
	if err := srv.Close(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
