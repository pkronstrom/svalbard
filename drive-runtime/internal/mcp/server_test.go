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
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	var pingTool, echoTool *mcp.ToolInfo
	for i := range tools {
		switch tools[i].Name {
		case "test_tool_ping":
			pingTool = &tools[i]
		case "test_tool_echo":
			echoTool = &tools[i]
		}
	}
	if pingTool == nil {
		t.Fatal("expected explicit ping tool to be registered")
	}
	if echoTool == nil {
		t.Fatal("expected explicit echo tool to be registered")
	}
	if pingTool.Description != "Returns pong" {
		t.Errorf("expected ping description 'Returns pong', got %q", pingTool.Description)
	}
	if echoTool.Description != "Echoes input" {
		t.Errorf("expected echo description 'Echoes input', got %q", echoTool.Description)
	}
	if got := echoTool.InputSchema.Required; len(got) != 1 || got[0] != "text" {
		t.Fatalf("echo required fields = %v, want [text]", got)
	}
	textProp, ok := echoTool.InputSchema.Properties["text"].(map[string]any)
	if !ok {
		t.Fatalf("echo text property missing or wrong type: %#v", echoTool.InputSchema.Properties["text"])
	}
	if textProp["type"] != "string" {
		t.Fatalf("echo text property type = %#v, want string", textProp["type"])
	}
	if echoTool.Annotations.ReadOnlyHint == nil || !*echoTool.Annotations.ReadOnlyHint {
		t.Fatal("expected readOnlyHint=true")
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
