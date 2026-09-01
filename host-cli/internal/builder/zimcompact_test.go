package builder

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestZIMCompactOrchestratesPinnedTools(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source-fixture.zim")
	if err := os.WriteFile(source, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	original := runToolCommand
	t.Cleanup(func() { runToolCommand = original })
	var calls []string
	runToolCommand = func(_ context.Context, _, _ string, image, tool string, args []string) (string, error) {
		if image != BaseToolsImage {
			t.Fatalf("image = %q", image)
		}
		calls = append(calls, tool)
		switch tool {
		case "zim-compact":
			extracted := args[len(args)-1]
			if err := os.MkdirAll(extracted, 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(filepath.Join(extracted, "index"), []byte("page"), 0o644); err != nil {
				return "", err
			}
			return "main_page=index\nlanguage=eng\ntitle=Medicine", nil
		case "zimwriterfs":
			output := args[len(args)-1]
			return "", os.WriteFile(output, []byte("zim"), 0o644)
		case "zimcheck":
			return "", nil
		default:
			t.Fatalf("unexpected tool %q", tool)
			return "", nil
		}
	}
	recipe := catalog.Item{
		ID: "medicine-compact", Type: "zim", Strategy: "build",
		Build: &catalog.BuildSpec{Family: "zim-compact", SourceURL: "file://unused", Output: "medicine.zim", Config: map[string]string{"width": "160", "quality": "35"}},
	}
	// Avoid the downloader by placing the expected persistent source in staging.
	workdir := filepath.Join(root, ".staging", "build", recipe.ID)
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(source)
	if err := os.WriteFile(filepath.Join(workdir, "source.zim"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := buildZIMCompact(root, recipe, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].RelativePath != filepath.Join("zim", "medicine.zim") {
		t.Fatalf("entries = %+v", entries)
	}
	if got := strings.Join(calls, ","); got != "zim-compact,zimwriterfs,zimcheck" {
		t.Fatalf("calls = %s", got)
	}
}
