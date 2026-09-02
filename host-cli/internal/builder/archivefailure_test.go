package builder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestBuildContentArchiveRecordsUnavailablePDFProject(t *testing.T) {
	var sourcePDF []byte
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/source.pdf":
			writer.Header().Set("Content-Type", "application/pdf")
			_, _ = writer.Write(sourcePDF)
		case "/blocked":
			http.Error(writer, "blocked", http.StatusForbidden)
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()
	sourcePDF = minimalTextPDF(server.URL + "/blocked")

	previous := runToolCommand
	runToolCommand = func(_ context.Context, _ string, _ string, _ string, tool string, args []string) (string, error) {
		if tool == "zimwriterfs" {
			return "", os.WriteFile(args[len(args)-1], []byte("zim"), 0o644)
		}
		return "", nil
	}
	defer func() { runToolCommand = previous }()

	root := t.TempDir()
	recipe := catalog.Item{ID: "unavailable", Type: "zim", Build: &catalog.BuildSpec{
		Family: "content-archive", SourceURL: server.URL + "/source.pdf", Config: map[string]string{"source_format": "pdf-links"},
	}}
	if _, err := buildContentArchive(root, recipe, nil, Options{}); err != nil {
		t.Fatal(err)
	}
	source, _ := archiveURL(server.URL + "/blocked")
	project := filepath.Join(root, ".staging", "build", recipe.ID, "site", "projects", newArchiveSource(source, 1).ID)
	record, err := os.ReadFile(filepath.Join(project, "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(record), `"status": "unavailable"`) || !strings.Contains(string(record), "403 Forbidden") {
		t.Fatalf("project record = %s", record)
	}
	page, err := os.ReadFile(filepath.Join(project, "blocked.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), "Unavailable") {
		t.Fatalf("failure page = %s", page)
	}
}
