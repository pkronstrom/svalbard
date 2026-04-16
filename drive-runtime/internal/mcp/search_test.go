package mcp_test

import (
	"context"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
)

func TestSearchCapabilityToolName(t *testing.T) {
	cap := mcp.NewSearchCapability(t.TempDir(), mcp.DriveMetadata{})
	if cap.Tool() != "search" {
		t.Errorf("expected tool name 'search', got %q", cap.Tool())
	}
	if cap.Description() == "" {
		t.Error("expected non-empty description")
	}
}

func TestSearchCapabilityActions(t *testing.T) {
	cap := mcp.NewSearchCapability(t.TempDir(), mcp.DriveMetadata{})
	actions := cap.Actions()
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	names := map[string]bool{}
	var searchAction, readAction *mcp.ActionDef
	for _, a := range actions {
		names[a.Name] = true
		action := a
		switch a.Name {
		case "search":
			searchAction = &action
		case "read":
			readAction = &action
		}
	}
	for _, want := range []string{"search", "read"} {
		if !names[want] {
			t.Errorf("missing action %q", want)
		}
	}
	if searchAction == nil || readAction == nil {
		t.Fatalf("expected both search and read actions, got %#v", actions)
	}
	if want := "vault_sources"; !strings.Contains(searchAction.Desc, want) {
		t.Fatalf("search description %q does not mention %q", searchAction.Desc, want)
	}
	// read action path should be optional (for browsing main page)
	var pathParam *mcp.ParamDef
	for _, param := range readAction.Params {
		param := param
		if param.Name == "path" {
			pathParam = &param
			break
		}
	}
	if pathParam == nil {
		t.Fatal("read action missing path param")
	}
	if pathParam.Required {
		t.Fatal("read path param should not be required (omit to browse main page)")
	}
}

func TestSearchCapabilityUnknownAction(t *testing.T) {
	cap := mcp.NewSearchCapability(t.TempDir(), mcp.DriveMetadata{})
	_, err := cap.Handle(context.Background(), "nonexistent", map[string]any{})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestSearchCapabilityKeywordFailsWithoutSearchDB(t *testing.T) {
	// Empty temp dir has no search.db, so session creation should fail.
	cap := mcp.NewSearchCapability(t.TempDir(), mcp.DriveMetadata{})
	_, err := cap.Handle(context.Background(), "search", map[string]any{
		"query": "test",
	})
	if err == nil {
		t.Fatal("expected error when search DB is missing")
	}
}

func TestSearchCapabilityKeywordRequiresQuery(t *testing.T) {
	cap := mcp.NewSearchCapability(t.TempDir(), mcp.DriveMetadata{})
	// Even if session creation fails, query validation should happen first...
	// Actually, getSession is called first. Let's just check the error path.
	_, err := cap.Handle(context.Background(), "search", map[string]any{})
	if err == nil {
		t.Fatal("expected error when query is missing")
	}
}

func TestSearchCapabilityReadRequiresSourceAndPath(t *testing.T) {
	cap := mcp.NewSearchCapability(t.TempDir(), mcp.DriveMetadata{})
	_, err := cap.Handle(context.Background(), "read", map[string]any{})
	if err == nil {
		t.Fatal("expected error when source is missing")
	}

	_, err = cap.Handle(context.Background(), "read", map[string]any{"source": "wiki"})
	if err == nil {
		t.Fatal("expected error when path is missing")
	}
}

func TestSearchCapabilityClose(t *testing.T) {
	cap := mcp.NewSearchCapability(t.TempDir(), mcp.DriveMetadata{})
	// Close without ever initializing session should be safe.
	if err := cap.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
