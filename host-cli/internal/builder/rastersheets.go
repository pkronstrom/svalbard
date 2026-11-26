package builder

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/downloader"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

// buildRasterSheets renders georeferenced map sheets into a raster PMTiles.
// Each band names a directory tree of PNG sheets with .pgw world files in
// EPSG:3067 (an nginx-style autoindex, as kartat.kapsi.fi serves MML's map
// series) and the zoom range it covers. Sheets that intersect the recipe's
// bbox are downloaded, mosaicked and tiled to WebP with GDAL in the tools
// image, then all bands are packed into one MBTiles and converted to PMTiles.
func buildRasterSheets(root string, recipe catalog.Item, _ *catalog.Catalog, opts Options) ([]manifest.RealizedEntry, error) {
	if recipe.Build == nil || len(recipe.Build.Bands) == 0 {
		return nil, fmt.Errorf("raster-sheets %s: bands required", recipe.ID)
	}
	if srs := recipe.Build.Config["source_srs"]; srs != "" && srs != "EPSG:3067" {
		return nil, fmt.Errorf("raster-sheets %s: unsupported source_srs %q (only EPSG:3067)", recipe.ID, srs)
	}
	bbox, err := parseBBox(recipe.Build.Config["bbox"])
	if err != nil {
		return nil, fmt.Errorf("raster-sheets %s: %w", recipe.ID, err)
	}
	extent := tm35finExtent(bbox)
	quality := configInt(recipe.Build.Config, "quality", 80)
	ctx := builderContext(opts)
	workdir := filepath.Join(root, ".staging", "build", recipe.ID)
	tiles := filepath.Join(workdir, "tiles")
	if err := os.RemoveAll(tiles); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(tiles, 0o755); err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	minZoom, maxZoom := recipe.Build.Bands[0].MinZoom, recipe.Build.Bands[0].MaxZoom
	for index, band := range recipe.Build.Bands {
		if band.Source == "" || band.MinZoom < 0 || band.MaxZoom < band.MinZoom {
			return nil, fmt.Errorf("raster-sheets %s: band %d needs a source and min_zoom <= max_zoom", recipe.ID, index+1)
		}
		minZoom, maxZoom = min(minZoom, band.MinZoom), max(maxZoom, band.MaxZoom)
		emit(opts, BuildEvent{RecipeID: recipe.ID, Procedure: "list-sheets", State: EventStarted,
			Message: fmt.Sprintf("band %d: listing sheets for zooms %d-%d", index+1, band.MinZoom, band.MaxZoom)})
		sheets, err := listSheets(ctx, client, band.Source, extent)
		if err != nil {
			return nil, fmt.Errorf("raster-sheets %s: band %d: %w", recipe.ID, index+1, err)
		}
		if len(sheets) == 0 {
			return nil, fmt.Errorf("raster-sheets %s: band %d: no sheets under %s intersect bbox %s", recipe.ID, index+1, band.Source, recipe.Build.Config["bbox"])
		}
		bandDir := filepath.Join(workdir, fmt.Sprintf("band-%02d", index+1))
		for _, sheet := range sheets {
			for _, file := range []string{sheet.png, sheet.pgw} {
				dest := filepath.Join(bandDir, "sheets", path.Base(file))
				if err := stepDownload(ctx, file, dest); err != nil {
					return nil, err
				}
			}
		}
		script := filepath.Join(bandDir, "render.sh")
		if err := os.WriteFile(script, []byte(renderScript(band, extent, quality)), 0o644); err != nil {
			return nil, err
		}
		emit(opts, BuildEvent{RecipeID: recipe.ID, Procedure: "render", State: EventStarted,
			Message: fmt.Sprintf("band %d: rendering %d sheets", index+1, len(sheets))})
		if _, err := runToolCommand(ctx, root, workdir, BaseToolsImage, "sh", []string{script}); err != nil {
			return nil, err
		}
	}

	mbtiles := filepath.Join(workdir, recipe.ID+".mbtiles")
	attribution := ""
	if recipe.License != nil {
		attribution = recipe.License.Attribution
	}
	if err := packMBTiles(tiles, mbtiles, map[string]string{
		"name": recipe.ID, "format": "webp", "type": "baselayer",
		"bounds":  recipe.Build.Config["bbox"],
		"minzoom": strconv.Itoa(minZoom), "maxzoom": strconv.Itoa(maxZoom),
		"attribution": attribution,
	}); err != nil {
		return nil, fmt.Errorf("raster-sheets %s: %w", recipe.ID, err)
	}
	output := filepath.Join(root, toolkit.TypeDirs[recipe.Type], recipe.ID+".pmtiles")
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return nil, err
	}
	if _, err := runToolCommand(ctx, root, workdir, BaseToolsImage, "pmtiles", []string{"convert", mbtiles, output}); err != nil {
		return nil, err
	}
	info, err := os.Stat(output)
	if err != nil {
		return nil, err
	}
	checksum, _ := downloader.ComputeSHA256(output)
	// The sheets, tiles and MBTiles are several times the map's size and
	// would otherwise stay on the vault. Files the tools container created
	// may belong to root on Linux, so a failed removal only warns.
	if err := os.RemoveAll(workdir); err != nil {
		slog.Warn("raster-sheets: could not remove build files", "dir", workdir, "err", err)
	}
	return []manifest.RealizedEntry{{
		ID: recipe.ID, Type: recipe.Type, Filename: filepath.Base(output),
		RelativePath: filepath.Join(toolkit.TypeDirs[recipe.Type], filepath.Base(output)),
		SizeBytes:    info.Size(), ChecksumSHA256: checksum, SourceStrategy: "build",
	}}, nil
}

// renderScript tiles one band's sheets into the shared ../tiles XYZ tree.
// Paletted sheets are expanded to RGB first so neighbouring palettes mosaic
// and average-resample correctly; -addalpha keeps the area outside the
// sheets transparent.
func renderScript(band catalog.RasterBand, extent [4]float64, quality int) string {
	return fmt.Sprintf(`set -eu
cd "$(dirname "$0")"
for png in sheets/*.png; do
  expand=""
  if gdalinfo "$png" | grep -q "Color Table"; then expand="-expand rgb"; fi
  gdal_translate -q -of VRT -a_srs EPSG:3067 $expand "$png" "${png%%.png}.vrt"
done
gdalbuildvrt -q -addalpha -te %.0f %.0f %.0f %.0f band.vrt sheets/*.vrt
gdal2tiles --xyz -q -s EPSG:3067 -z %d-%d -r average --tiledriver=WEBP --webp-quality=%d --webviewer=none --processes=%d band.vrt ../tiles
`, extent[0], extent[1], extent[2], extent[3], band.MinZoom, band.MaxZoom, quality, runtime.NumCPU())
}

type mapSheet struct {
	png, pgw string
}

var autoindexHref = regexp.MustCompile(`href="([^"]+)"`)

// listSheets walks an autoindex tree from dir and returns the PNG sheets
// whose world-file extent intersects extent (EPSG:3067 xmin, ymin, xmax, ymax).
func listSheets(ctx context.Context, client *http.Client, dir string, extent [4]float64) ([]mapSheet, error) {
	base, err := url.Parse(dir)
	if err != nil {
		return nil, err
	}
	body, err := fetchBytes(ctx, client, dir, 0)
	if err != nil {
		return nil, err
	}
	files := map[string]bool{}
	var subdirs, pngs []string
	for _, match := range autoindexHref.FindAllSubmatch(body, -1) {
		name := string(match[1])
		if strings.HasPrefix(name, "?") || strings.HasPrefix(name, "/") || strings.HasPrefix(name, ".") || strings.Contains(name, "://") {
			continue
		}
		switch {
		case strings.HasSuffix(name, "/"):
			subdirs = append(subdirs, name)
		case strings.HasSuffix(name, ".png"):
			pngs = append(pngs, name)
		}
		files[name] = true
	}

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		sheets  []mapSheet
		failure error
		slots   = make(chan struct{}, 8)
	)
	for _, name := range pngs {
		pgw := strings.TrimSuffix(name, ".png") + ".pgw"
		if !files[pgw] {
			continue
		}
		sheet := mapSheet{png: base.ResolveReference(&url.URL{Path: name}).String(), pgw: base.ResolveReference(&url.URL{Path: pgw}).String()}
		wg.Add(1)
		slots <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			bounds, err := sheetExtent(ctx, client, sheet)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err != nil && failure == nil:
				failure = err
			case err == nil && intersects(bounds, extent):
				sheets = append(sheets, sheet)
			}
		}()
	}
	wg.Wait()
	if failure != nil {
		return nil, failure
	}
	for _, sub := range subdirs {
		found, err := listSheets(ctx, client, base.ResolveReference(&url.URL{Path: sub}).String(), extent)
		if err != nil {
			return nil, err
		}
		sheets = append(sheets, found...)
	}
	return sheets, nil
}

// sheetExtent reads a sheet's world file and its PNG header (width and
// height) and returns the sheet's EPSG:3067 extent.
func sheetExtent(ctx context.Context, client *http.Client, sheet mapSheet) ([4]float64, error) {
	world, err := fetchBytes(ctx, client, sheet.pgw, 0)
	if err != nil {
		return [4]float64{}, err
	}
	var params []float64
	scanner := bufio.NewScanner(bytes.NewReader(world))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			value, err := strconv.ParseFloat(line, 64)
			if err != nil {
				return [4]float64{}, fmt.Errorf("%s: %w", sheet.pgw, err)
			}
			params = append(params, value)
		}
	}
	if len(params) != 6 || params[1] != 0 || params[2] != 0 {
		return [4]float64{}, fmt.Errorf("%s: want a north-up six-line world file", sheet.pgw)
	}
	header, err := fetchBytes(ctx, client, sheet.png, 24)
	if err != nil {
		return [4]float64{}, err
	}
	if len(header) < 24 || !bytes.HasPrefix(header, []byte("\x89PNG")) || string(header[12:16]) != "IHDR" {
		return [4]float64{}, fmt.Errorf("%s: not a PNG", sheet.png)
	}
	width := float64(binary.BigEndian.Uint32(header[16:20]))
	height := float64(binary.BigEndian.Uint32(header[20:24]))
	// The world file gives the centre of the upper-left pixel.
	xmin, ymax := params[4]-params[0]/2, params[5]-params[3]/2
	return [4]float64{xmin, ymax + params[3]*height, xmin + params[0]*width, ymax}, nil
}

// fetchBytes GETs target and returns its body, or only its first limit bytes
// (requested with a Range header) when limit > 0.
func fetchBytes(ctx context.Context, client *http.Client, target string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	if limit > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=0-%d", limit-1))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("GET %s: %s", target, resp.Status)
	}
	reader := io.Reader(resp.Body)
	if limit > 0 {
		reader = io.LimitReader(resp.Body, limit)
	}
	return io.ReadAll(reader)
}

func intersects(a, b [4]float64) bool {
	return a[0] < b[2] && b[0] < a[2] && a[1] < b[3] && b[1] < a[3]
}

// parseBBox reads "west,south,east,north" in degrees.
func parseBBox(value string) ([4]float64, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 4 {
		return [4]float64{}, fmt.Errorf("bbox %q: want west,south,east,north", value)
	}
	var bbox [4]float64
	for index, part := range parts {
		number, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil {
			return [4]float64{}, fmt.Errorf("bbox %q: %w", value, err)
		}
		bbox[index] = number
	}
	if bbox[0] >= bbox[2] || bbox[1] >= bbox[3] {
		return [4]float64{}, fmt.Errorf("bbox %q: west/south must be below east/north", value)
	}
	return bbox, nil
}

// tm35finExtent projects a degree bbox to EPSG:3067, sampling its edges
// because they curve in the projection.
func tm35finExtent(bbox [4]float64) [4]float64 {
	extent := [4]float64{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	const steps = 32
	for i := 0; i <= steps; i++ {
		f := float64(i) / steps
		lon := bbox[0] + f*(bbox[2]-bbox[0])
		lat := bbox[1] + f*(bbox[3]-bbox[1])
		for _, point := range [][2]float64{{lon, bbox[1]}, {lon, bbox[3]}, {bbox[0], lat}, {bbox[2], lat}} {
			x, y := tm35fin(point[0], point[1])
			extent = [4]float64{min(extent[0], x), min(extent[1], y), max(extent[2], x), max(extent[3], y)}
		}
	}
	return extent
}

// tm35fin projects longitude and latitude (degrees, ETRS89) to ETRS-TM35FIN
// (EPSG:3067): transverse Mercator on GRS80, central meridian 27°E, scale
// 0.9996, false easting 500 km. It uses Krüger's series, which is accurate
// to well under a millimetre across Finland.
func tm35fin(lon, lat float64) (float64, float64) {
	const (
		a  = 6378137.0
		f  = 1 / 298.257222101
		k0 = 0.9996
		e0 = 500000.0
	)
	n := f / (2 - f)
	A := a / (1 + n) * (1 + n*n/4 + n*n*n*n/64)
	alpha := [3]float64{
		n/2 - 2*n*n/3 + 5*n*n*n/16,
		13*n*n/48 - 3*n*n*n/5,
		61 * n * n * n / 240,
	}
	phi := lat * math.Pi / 180
	lambda := (lon - 27) * math.Pi / 180
	c := 2 * math.Sqrt(n) / (1 + n)
	t := math.Sinh(math.Atanh(math.Sin(phi)) - c*math.Atanh(c*math.Sin(phi)))
	xi := math.Atan2(t, math.Cos(lambda))
	eta := math.Atanh(math.Sin(lambda) / math.Sqrt(1+t*t))
	x, y := eta, xi
	for j, aj := range alpha {
		k := float64(2 * (j + 1))
		x += aj * math.Cos(k*xi) * math.Sinh(k*eta)
		y += aj * math.Sin(k*xi) * math.Cosh(k*eta)
	}
	return e0 + k0*A*x, k0 * A * y
}

// packMBTiles writes the XYZ tree under tilesDir ({z}/{x}/{y}.webp) into a
// new MBTiles file, flipping rows to the TMS scheme MBTiles uses.
func packMBTiles(tilesDir, path string, metadata map[string]string) error {
	_ = os.Remove(path)
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.Exec(`
		CREATE TABLE metadata (name TEXT, value TEXT);
		CREATE TABLE tiles (zoom_level INTEGER, tile_column INTEGER, tile_row INTEGER, tile_data BLOB);
		CREATE UNIQUE INDEX tile_index ON tiles (zoom_level, tile_column, tile_row);`); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	insert, err := tx.Prepare(`INSERT INTO tiles VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insert.Close()
	count := 0
	err = filepath.WalkDir(tilesDir, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || filepath.Ext(file) != ".webp" {
			return walkErr
		}
		rel, err := filepath.Rel(tilesDir, file)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(strings.TrimSuffix(rel, ".webp")), "/")
		if len(parts) != 3 {
			return nil
		}
		var zxy [3]int
		for index, part := range parts {
			if zxy[index], err = strconv.Atoi(part); err != nil {
				return nil
			}
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		count++
		_, err = insert.Exec(zxy[0], zxy[1], (1<<zxy[0])-1-zxy[2], data)
		return err
	})
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("no tiles rendered under %s", tilesDir)
	}
	for name, value := range metadata {
		if _, err := tx.Exec(`INSERT INTO metadata VALUES (?, ?)`, name, value); err != nil {
			return err
		}
	}
	return tx.Commit()
}
