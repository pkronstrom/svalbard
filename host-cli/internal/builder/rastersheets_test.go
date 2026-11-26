package builder

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestTM35FINMatchesGDAL(t *testing.T) {
	// Expected values from gdaltransform -s_srs OGC:CRS84 -t_srs EPSG:3067.
	for _, tc := range []struct{ lon, lat, x, y float64 }{
		{27, 60, 500000, 6651411.1902397},
		{24.9384, 60.1699, 385611.316684942, 6672118.38020209},
		{23.8, 59.8, 320481.304036303, 6633472.3887684},
	} {
		x, y := tm35fin(tc.lon, tc.lat)
		if math.Abs(x-tc.x) > 0.01 || math.Abs(y-tc.y) > 0.01 {
			t.Errorf("tm35fin(%v, %v) = (%.3f, %.3f), want (%.3f, %.3f)", tc.lon, tc.lat, x, y, tc.x, tc.y)
		}
	}
}

// pngHeader is the start of a PNG file: signature and IHDR width and height.
func pngHeader(width, height uint32) []byte {
	header := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR")
	header = binary.BigEndian.AppendUint32(header, width)
	header = binary.BigEndian.AppendUint32(header, height)
	return append(header, 8, 3, 0, 0, 0)
}

// sheetServer serves an nginx-style autoindex of map sheets: one sheet near
// (27°E, 60°N) in a subdirectory and one far west at the top level.
func sheetServer(t *testing.T) *httptest.Server {
	t.Helper()
	pages := map[string]string{
		"/png/":    `<a href="../">../</a><a href="L5/">L5/</a><a href="far.pgw">far.pgw</a><a href="far.png">far.png</a>`,
		"/png/L5/": `<a href="../">../</a><a href="near.pgw">near.pgw</a><a href="near.png">near.png</a>`,
		// 4 m pixels, upper-left pixel centre at (496002, 6655998): a 12x6 km sheet.
		"/png/L5/near.pgw": "4.0\n0.0\n0.0\n-4.0\n496002.00\n6655998.00\n",
		"/png/far.pgw":     "4.0\n0.0\n0.0\n-4.0\n100002.00\n7000000.00\n",
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".png") {
			_, _ = w.Write(pngHeader(3000, 1500))
			return
		}
		body, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(body))
	}))
}

func TestRasterSheetsRendersIntersectingSheetsToPMTiles(t *testing.T) {
	server := sheetServer(t)
	defer server.Close()
	root := t.TempDir()
	original := runToolCommand
	t.Cleanup(func() { runToolCommand = original })
	var scripts []string
	runToolCommand = func(_ context.Context, _, workdir, image, tool string, args []string) (string, error) {
		if image != BaseToolsImage {
			t.Fatalf("image = %q", image)
		}
		switch tool {
		case "sh":
			script, err := os.ReadFile(args[0])
			if err != nil {
				return "", err
			}
			scripts = append(scripts, string(script))
			sheets, _ := filepath.Glob(filepath.Join(filepath.Dir(args[0]), "sheets", "*.png"))
			if len(sheets) != 1 || filepath.Base(sheets[0]) != "near.png" {
				t.Fatalf("band sheets = %v, want only near.png", sheets)
			}
			// Stand in for gdal2tiles: one tile per zoom of the band.
			for z := 13; z <= 14; z++ {
				tile := filepath.Join(workdir, "tiles", fmt.Sprint(z), "4", "7.webp")
				if err := os.MkdirAll(filepath.Dir(tile), 0o755); err != nil {
					return "", err
				}
				if err := os.WriteFile(tile, []byte(fmt.Sprintf("tile-%d", z)), 0o644); err != nil {
					return "", err
				}
			}
		case "pmtiles":
			if len(args) != 3 || args[0] != "convert" {
				t.Fatalf("pmtiles args = %v", args)
			}
			checkMBTiles(t, args[1])
			return "", os.WriteFile(args[2], []byte("pmtiles"), 0o644)
		default:
			t.Fatalf("unexpected tool %s", tool)
		}
		return "", nil
	}
	recipe := catalog.Item{
		ID: "topo", Type: "pmtiles", Strategy: "build",
		License: &catalog.LicenseSpec{Attribution: "National Land Survey of Finland"},
		Build: &catalog.BuildSpec{
			Family: "raster-sheets",
			Bands:  []catalog.RasterBand{{MinZoom: 13, MaxZoom: 14, Source: server.URL + "/png/"}},
			Config: map[string]string{"bbox": "26.95,59.97,27.05,60.03", "quality": "75"},
		},
	}
	entries, err := buildRasterSheets(root, recipe, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].RelativePath != filepath.Join("maps", "topo.pmtiles") {
		t.Fatalf("entries = %+v", entries)
	}
	if _, err := os.Stat(filepath.Join(root, ".staging", "build", "topo")); !os.IsNotExist(err) {
		t.Errorf("build files left on the vault: %v", err)
	}
	if len(scripts) != 1 {
		t.Fatalf("render scripts = %d, want 1", len(scripts))
	}
	for _, want := range []string{"gdal2tiles", "-z 13-14", "--tiledriver=WEBP", "--webp-quality=75", "-a_srs EPSG:3067"} {
		if !strings.Contains(scripts[0], want) {
			t.Errorf("render script lacks %q:\n%s", want, scripts[0])
		}
	}
}

func checkMBTiles(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var row int
	var data string
	// XYZ row 7 at zoom 13 is TMS row 2^13-1-7.
	if err := db.QueryRow(`SELECT tile_row, tile_data FROM tiles WHERE zoom_level = 13 AND tile_column = 4`).Scan(&row, &data); err != nil {
		t.Fatal(err)
	}
	if row != (1<<13)-1-7 || data != "tile-13" {
		t.Errorf("zoom 13 tile row %d data %q", row, data)
	}
	meta := map[string]string{}
	rows, err := db.Query(`SELECT name, value FROM metadata`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			t.Fatal(err)
		}
		meta[name] = value
	}
	for name, want := range map[string]string{
		"format": "webp", "minzoom": "13", "maxzoom": "14",
		"bounds": "26.95,59.97,27.05,60.03", "attribution": "National Land Survey of Finland",
	} {
		if meta[name] != want {
			t.Errorf("metadata %s = %q, want %q", name, meta[name], want)
		}
	}
}

func TestRasterSheetsRejectsBandWithoutSheets(t *testing.T) {
	server := sheetServer(t)
	defer server.Close()
	original := runToolCommand
	t.Cleanup(func() { runToolCommand = original })
	runToolCommand = func(context.Context, string, string, string, string, []string) (string, error) {
		t.Fatal("no tool may run when a band has no sheets")
		return "", nil
	}
	recipe := catalog.Item{
		ID: "topo", Type: "pmtiles", Strategy: "build",
		Build: &catalog.BuildSpec{
			Family: "raster-sheets",
			Bands:  []catalog.RasterBand{{MinZoom: 0, MaxZoom: 8, Source: server.URL + "/png/"}},
			Config: map[string]string{"bbox": "21.0,69.0,21.1,69.1"},
		},
	}
	if _, err := buildRasterSheets(t.TempDir(), recipe, nil, Options{}); err == nil || !strings.Contains(err.Error(), "no sheets") {
		t.Fatalf("err = %v, want a no-sheets error", err)
	}
}
