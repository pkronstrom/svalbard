package frontpage

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
)

func TestCatalogExposesPreviewsForInstalledItems(t *testing.T) {
	root := t.TempDir()
	cover := strings.Repeat("a", 64) + ".jpg"
	other := strings.Repeat("b", 64) + ".jpg"
	writeFile(t, root, ".svalbard/recipes.json", `{
		"wikiciv":{"id":"wikiciv","type":"zim","description":"Tech reference","tags":["field guides"],
			"preview":{"summary":"Build practical technology.",
				"image":{"file":"`+cover+`","alt":"A pencil sketch of a water wheel"}}},
		"unlisted":{"id":"unlisted","type":"zim","preview":{"image":{"file":"`+other+`"}}}
	}`)
	const coverBytes = "\xff\xd8\xff\xd9"
	writeFile(t, root, ".svalbard/previews/"+cover, coverBytes)
	writeFile(t, root, ".svalbard/previews/"+other, coverBytes)
	cfg := config.RuntimeConfig{Groups: []config.MenuGroup{{Label: "Library", Items: []config.MenuItem{{
		ID: "wikiciv", Label: "Wikiciv", Action: config.BuiltinAction("browse", map[string]string{"zim": "wiki.zim"}),
	}}}}}
	client, origin, _ := startDashboard(t, root, cfg)
	const base = "/api/previews/"
	var inv struct {
		Groups []struct {
			Items []struct {
				Summary, RecipeID string
				Tags              []string
				Image             struct{ URL, Alt string }
			}
		}
	}
	readResponse(t, requestBody(t, client, origin+"/api/inventory", http.StatusOK), &inv)
	item := inv.Groups[0].Items[0]
	if item.Summary != "Build practical technology." || item.RecipeID != "wikiciv" ||
		len(item.Tags) != 1 || item.Tags[0] != "field guides" ||
		item.Image.URL != base+cover || item.Image.Alt != "A pencil sketch of a water wheel" {
		t.Fatalf("inventory metadata = %+v", item)
	}
	resp, err := client.Get(origin + item.Image.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/jpeg" || string(body) != coverBytes {
		t.Fatalf("thumbnail response = %d, %q, %q", resp.StatusCode, resp.Header.Get("Content-Type"), body)
	}
	requestBody(t, client, origin+base+other, http.StatusNotFound)
}
