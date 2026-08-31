package mcp

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

func TestMain(m *testing.M) {
	if os.Getenv("SVALBARD_TEST_MCP_KIWIX") != "1" {
		os.Exit(m.Run())
	}
	for i, arg := range os.Args {
		if arg == "--port" && i+1 < len(os.Args) {
			if err := http.ListenAndServe("127.0.0.1:"+os.Args[i+1], http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})); err != nil {
				os.Exit(1)
			}
		}
	}
	os.Exit(1)
}

func TestSearchCapabilityReusesAndClosesKiwix(t *testing.T) {
	root := t.TempDir()
	platformName, err := platform.Detect()
	if err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(root, "bin", platformName)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(os.Args[0], filepath.Join(binDir, "kiwix-serve")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "zim"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "zim", "wiki.zim"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SVALBARD_TEST_MCP_KIWIX", "1")

	capability := NewSearchCapability(root, DriveMetadata{})
	if err := capability.ensureKiwix(); err != nil {
		t.Fatalf("first ensureKiwix() error = %v", err)
	}
	first := capability.kiwixCmd
	if err := capability.ensureKiwix(); err != nil {
		t.Fatalf("second ensureKiwix() error = %v", err)
	}
	if capability.kiwixCmd != first {
		t.Fatal("ensureKiwix() did not reuse the cached process")
	}
	if err := capability.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if first.ProcessState == nil {
		t.Fatal("Close() did not wait for the cached Kiwix process")
	}
}
