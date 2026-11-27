package frontpage

import (
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/query"
)

func datasetGroup(folder string, metadata mcp.DriveMetadata) group {
	g := group{ID: "datasets", Label: "Datasets"}
	root, err := os.OpenRoot(folder)
	if err != nil {
		return g
	}
	defer root.Close()
	for file, id := range metadata.FileRecipes {
		if !strings.HasPrefix(file, "data/") || path.Base(file) == "search.db" {
			continue
		}
		f, err := openSourceFile(root, file)
		if err != nil {
			continue
		}
		info, err := f.Stat()
		f.Close()
		if err != nil {
			continue
		}
		ext := strings.ToUpper(strings.TrimPrefix(path.Ext(file), "."))
		e := entry{ID: file, Label: path.Base(file), RecipeID: id, Format: ext, Bytes: info.Size(), Download: file, Kind: "dataset", Icon: "dataset"}
		addRecipePreview(&e, id, metadata.Recipes)
		g.Items = append(g.Items, e)
	}
	slices.SortFunc(g.Items, func(a, b entry) int { return strings.Compare(a.ID, b.ID) })
	return g
}

func (d *dashboard) dataset(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	for _, g := range d.inv.Groups {
		for _, e := range g.Items {
			if e.ID != id || e.Kind != "dataset" {
				continue
			}
			root, err := os.OpenRoot(d.root)
			if err != nil {
				break
			}
			defer root.Close()
			f, err := openSourceFile(root, e.Download)
			if err != nil {
				break
			}
			defer f.Close()
			if e.Format == "CSV" || e.Format == "TSV" {
				reader := csv.NewReader(io.LimitReader(f, 256<<10))
				if e.Format == "TSV" {
					reader.Comma = '\t'
				}
				columns, err := reader.Read()
				if err != nil {
					break
				}
				rows := [][]string{}
				for len(rows) < 20 {
					row, err := reader.Read()
					if errors.Is(err, io.EOF) {
						break
					}
					if err != nil {
						break
					}
					rows = append(rows, row)
				}
				writeJSON(w, struct {
					Columns []string   `json:"columns"`
					Rows    [][]string `json:"rows"`
				}{columns, rows})
				return
			}
			if (e.Format == "GPKG" || e.Format == "SQLITE" || e.Format == "DB") && path.Dir(e.Download) == "data" && !strings.ContainsAny(e.Download, "?#%") {
				schema, err := query.Describe(d.root, path.Base(e.Download), r.URL.Query().Get("table"))
				if err != nil {
					http.Error(w, "Table unavailable.", 422)
					return
				}
				// Keep documents and long text cells out of a bounded preview.
				if len(schema.Tables) > 250 {
					schema.Tables = schema.Tables[:250]
					schema.Note = "Showing the first 250 tables."
				}
				for _, t := range schema.Tables {
					for _, row := range t.Samples {
						for key, v := range row {
							if s, ok := v.(string); ok && len(s) > 1024 {
								row[key] = s[:1024] + "…"
							}
						}
					}
				}
				writeJSON(w, schema)
				return
			}
			http.Error(w, "Open the original file to inspect this format.", 422)
			return
		}
	}
	http.NotFound(w, r)
}
