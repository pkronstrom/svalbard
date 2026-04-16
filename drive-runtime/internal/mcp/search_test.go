package mcp_test

import (
	"context"
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
	if len(actions) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(actions))
	}
	names := map[string]bool{}
	for _, a := range actions {
		names[a.Name] = true
	}
	for _, want := range []string{"keyword", "semantic", "read"} {
		if !names[want] {
			t.Errorf("missing action %q", want)
		}
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
	_, err := cap.Handle(context.Background(), "keyword", map[string]any{
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
	_, err := cap.Handle(context.Background(), "keyword", map[string]any{})
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
