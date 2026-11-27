package toolkit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"gopkg.in/yaml.v3"
)

func TestGeneratePublishesOptionalRecipePreviews(t *testing.T) {
	stubRuntimeBinarySources(t)

	var requests atomic.Int32
	var src bytes.Buffer
	picture := image.NewRGBA(image.Rect(0, 0, 960, 720))
	picture.SetRGBA(0, 0, color.RGBA{R: 200, A: 255})
	if err := png.Encode(&src, picture); err != nil {
		t.Fatal(err)
	}
	fixture := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Path != "/cover.png" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(src.Bytes())
	}))
	t.Cleanup(fixture.Close)

	root := t.TempDir()
	entries := []manifest.RealizedEntry{{ID: "manuals", Type: "pdf"}}
	var notices []string
	publish := func(summary, imagePath, alt string) struct {
		Summary string
		Image   *struct{ File, Alt string }
	} {
		t.Helper()
		var recipe catalog.Item
		definition := fmt.Sprintf("id: manuals\ntype: pdf\npreview:\n  summary: %q\n  image:\n    url: %q\n    alt: %q\n", summary, fixture.URL+imagePath, alt)
		if err := yaml.Unmarshal([]byte(definition), &recipe); err != nil {
			t.Fatal(err)
		}
		if err := Generate(root, entries, "", GenerateOpts{
			Recipes: map[string]catalog.Item{"manuals": recipe},
			Notice:  func(message string) { notices = append(notices, message) },
		}); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(root, ".svalbard", "recipes.json"))
		if err != nil {
			t.Fatal(err)
		}
		var metadata map[string]struct {
			Preview struct {
				Summary string
				Image   *struct{ File, Alt string }
			}
		}
		if err := json.Unmarshal(data, &metadata); err != nil {
			t.Fatal(err)
		}
		return metadata["manuals"].Preview
	}

	missing := publish("Guides for field repairs.", "/missing.png", "Old missing cover")
	if missing.Summary != "Guides for field repairs." || missing.Image != nil {
		t.Fatalf("missing image yielded %+v; want summary only", missing)
	}

	first := publish("Guides for field repairs.", "/cover.png", "Repair manuals")
	if first.Image == nil || first.Image.Alt != "Repair manuals" || !strings.HasSuffix(first.Image.File, ".jpg") {
		t.Fatalf("preview = %+v; want a normalized JPEG with alt text", first)
	}
	if first.Image.File != filepath.Base(first.Image.File) {
		t.Fatalf("metadata must record only the image basename: %q", first.Image.File)
	}
	installed, err := os.ReadFile(filepath.Join(root, ".svalbard", "previews", first.Image.File))
	if err != nil {
		t.Fatal(err)
	}
	config, err := jpeg.DecodeConfig(bytes.NewReader(installed))
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != 400 || config.Height != 300 || len(installed) > 80<<10 {
		t.Fatalf("thumbnail = %dx%d, %d bytes; want 400x300 at most 80 KiB", config.Width, config.Height, len(installed))
	}

	cached := publish("Updated summary.", "/cover.png", "Updated alt")
	if cached.Summary != "Updated summary." || cached.Image == nil ||
		cached.Image.File != first.Image.File || cached.Image.Alt != "Updated alt" || requests.Load() != 2 {
		t.Fatalf("metadata-only changes should reuse thumbnail: %+v, %d requests", cached, requests.Load())
	}

	fallback := publish("Updated summary.", "/unavailable.png", "Unavailable alt")
	if fallback.Image == nil || *fallback.Image != *cached.Image || requests.Load() != 3 {
		t.Fatalf("a failed replacement should preserve the last good image: %+v, %d requests", fallback, requests.Load())
	}
	if len(notices) != 2 {
		t.Fatalf("notices = %v; want one per failed fetch", notices)
	}
}

func TestRecipePreviewChangesDoNotAffectAcquisitionHash(t *testing.T) {
	var original, edited catalog.Item
	for i, recipe := range []*catalog.Item{&original, &edited} {
		definition := "id: manuals\ntype: pdf\nurl: https://example.com/manual.pdf\n"
		if i != 0 {
			definition += "preview:\n  summary: A desktop reference.\n  image:\n    url: https://example.com/cover.png\n"
		}
		if err := yaml.Unmarshal([]byte(definition), recipe); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := edited.AcquisitionHash(), original.AcquisitionHash(); got != want {
		t.Fatalf("preview metadata changed the acquisition hash: %q vs %q", got, want)
	}
}
