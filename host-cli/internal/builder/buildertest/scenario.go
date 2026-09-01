// Package buildertest provides outcome-focused scenarios for builder tests.
package buildertest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/builder"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// Scenario is a temporary vault plus one recipe and its observable build results.
type Scenario struct {
	T       *testing.T
	Root    string
	Recipe  catalog.Item
	Catalog *catalog.Catalog
	Events  []builder.BuildEvent
	Entries []manifest.RealizedEntry
	Err     error
}

// New creates an isolated builder scenario.
func New(t *testing.T, recipe catalog.Item) *Scenario {
	t.Helper()
	return &Scenario{T: t, Root: t.TempDir(), Recipe: recipe}
}

// Source serves body over HTTP and returns its URL.
func (s *Scenario) Source(body []byte) string {
	s.T.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	s.T.Cleanup(server.Close)
	return server.URL
}

// Build dispatches and executes the recipe.
func (s *Scenario) Build() *Scenario {
	s.T.Helper()
	fn, ok := builder.Dispatch(s.Recipe)
	if !ok {
		s.Err = &UnsupportedRecipeError{Family: s.Recipe.Build.Family}
		return s
	}
	s.Entries, s.Err = fn(s.Root, s.Recipe, s.Catalog, builder.Options{
		Ctx: context.Background(),
		OnEvent: func(event builder.BuildEvent) {
			s.Events = append(s.Events, event)
		},
	})
	return s
}

// Artifact requires a drive-relative output file with exact content.
func (s *Scenario) Artifact(relative string, content []byte) *Scenario {
	s.T.Helper()
	data, err := os.ReadFile(filepath.Join(s.Root, filepath.FromSlash(relative)))
	if err != nil {
		s.T.Fatalf("artifact %s: %v", relative, err)
	}
	if string(data) != string(content) {
		s.T.Fatalf("artifact %s = %q, want %q", relative, data, content)
	}
	return s
}

// UnsupportedRecipeError is returned by the DSL before calling an absent handler.
type UnsupportedRecipeError struct{ Family string }

func (e *UnsupportedRecipeError) Error() string { return "unsupported build family: " + e.Family }
