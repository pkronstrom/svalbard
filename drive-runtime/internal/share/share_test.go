package share_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/share"
)

func TestHandlerServesDriveContentsOverHTTP(t *testing.T) {
	driveRoot := t.TempDir()
	filePath := filepath.Join(driveRoot, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hello from svalbard"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/hello.txt", nil)
	rec := httptest.NewRecorder()

	share.Handler(driveRoot).ServeHTTP(rec, req)

	if got, want := rec.Code, http.StatusOK; got != want {
		t.Fatalf("status = %d, want %d", got, want)
	}
	if got, want := rec.Body.String(), "hello from svalbard"; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}
