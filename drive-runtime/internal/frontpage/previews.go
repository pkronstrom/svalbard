package frontpage

import (
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
)

const previewURLPrefix = "/api/previews/"

type previewImage struct {
	URL string `json:"url"`
	Alt string `json:"alt,omitempty"`
}

func addRecipePreview(view *entry, id string, recipes map[string]mcp.RecipeMeta) {
	recipe, ok := recipes[id]
	if !ok {
		return
	}
	view.RecipeID = id
	view.Tags = recipe.Tags
	if view.Description == "" {
		view.Description = recipe.Description
	}
	if recipe.Preview == nil {
		return
	}
	view.Summary = recipe.Preview.Summary
	if cover := recipe.Preview.Image; cover != nil && isPreviewFile(cover.File) {
		view.Image = &previewImage{URL: previewURLPrefix + cover.File, Alt: cover.Alt}
	}
}

func isPreviewFile(name string) bool {
	hash, ok := strings.CutSuffix(name, ".jpg")
	if !ok || len(hash) != 64 || hash != strings.ToLower(hash) {
		return false
	}
	_, err := hex.DecodeString(hash)
	return err == nil
}

func installedPreviewFiles(inv inventory) map[string]bool {
	files := make(map[string]bool)
	for _, group := range inv.Groups {
		for _, item := range group.Items {
			if item.Image != nil {
				files[strings.TrimPrefix(item.Image.URL, previewURLPrefix)] = true
			}
		}
	}
	return files
}

func (d *dashboard) servePreview(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("file")
	if !d.previews[name] {
		http.NotFound(w, r)
		return
	}
	drive, err := os.OpenRoot(d.root)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer drive.Close()
	thumbnail := filepath.Join(".svalbard", "previews", name)
	info, err := drive.Stat(thumbnail)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 128<<10 {
		http.NotFound(w, r)
		return
	}
	image, err := drive.Open(thumbnail)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer image.Close()
	w.Header().Set("Content-Type", "image/jpeg")
	http.ServeContent(w, r, name, info.ModTime(), image)
}
