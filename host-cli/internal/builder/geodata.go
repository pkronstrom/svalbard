package builder

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/downloader"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

func buildVectorStatic(root string, recipe catalog.Item, _ *catalog.Catalog, opts Options) ([]manifest.RealizedEntry, error) {
	if recipe.Build == nil || recipe.Build.SourceURL == "" {
		return nil, fmt.Errorf("vector-static %s: source_url required", recipe.ID)
	}
	if format := recipe.Build.Config["source_format"]; format != "shapefile" {
		return nil, fmt.Errorf("vector-static %s: unsupported source_format %q", recipe.ID, format)
	}
	ctx := builderContext(opts)
	workdir := filepath.Join(root, ".staging", "build", recipe.ID)
	archive := filepath.Join(workdir, "source.zip")
	extracted := filepath.Join(workdir, "source")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return nil, err
	}
	if _, err := os.Stat(archive); os.IsNotExist(err) {
		if err := stepDownload(ctx, recipe.Build.SourceURL, archive); err != nil {
			return nil, err
		}
	}
	if err := stepExtract(archive, extracted); err != nil {
		return nil, err
	}
	shape, err := firstFileWithExtension(extracted, ".shp")
	if err != nil {
		return nil, err
	}
	geojson := filepath.Join(workdir, "layer.geojson")
	ogrArgs := []string{"-f", "GeoJSON", "-t_srs", "EPSG:4326"}
	if sourceSRS := recipe.Build.Config["source_srs"]; sourceSRS != "" {
		ogrArgs = append(ogrArgs, "-s_srs", sourceSRS)
	}
	ogrArgs = append(ogrArgs, geojson, shape)
	if _, err := runToolCommand(ctx, root, workdir, BaseToolsImage, "ogr2ogr", ogrArgs); err != nil {
		return nil, err
	}
	return buildPMTilesFromGeoJSON(ctx, root, workdir, recipe, []string{geojson})
}

func buildVectorService(root string, recipe catalog.Item, _ *catalog.Catalog, opts Options) ([]manifest.RealizedEntry, error) {
	if recipe.Build == nil || recipe.Build.Config["service_type"] != "wfs" || recipe.Build.Config["service_url"] == "" {
		return nil, fmt.Errorf("vector-service %s: WFS service_url required", recipe.ID)
	}
	if len(recipe.Build.Layers) == 0 {
		return nil, fmt.Errorf("vector-service %s: layers required", recipe.ID)
	}
	ctx := builderContext(opts)
	workdir := filepath.Join(root, ".staging", "build", recipe.ID)
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		return nil, err
	}
	var inputs []string
	for index, layer := range recipe.Build.Layers {
		endpoint, err := wfsURL(recipe.Build.Config["service_url"], layer, recipe.Build.Config)
		if err != nil {
			return nil, err
		}
		destination := filepath.Join(workdir, fmt.Sprintf("layer-%02d.geojson", index+1))
		if err := stepDownload(ctx, endpoint, destination); err != nil {
			return nil, err
		}
		inputs = append(inputs, destination)
	}
	return buildPMTilesFromGeoJSON(ctx, root, workdir, recipe, inputs)
}

func wfsURL(endpoint string, layer catalog.BuildLayer, config map[string]string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set("service", "WFS")
	query.Set("version", "2.0.0")
	query.Set("request", "GetFeature")
	query.Set("typeNames", layer.Name)
	query.Set("outputFormat", config["output_format"])
	if srs := config["srs"]; srs != "" {
		query.Set("srsName", srs)
	}
	if layer.Filter != "" {
		query.Set("CQL_FILTER", layer.Filter)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func buildPMTilesFromGeoJSON(ctx context.Context, root, workdir string, recipe catalog.Item, inputs []string) ([]manifest.RealizedEntry, error) {
	output := filepath.Join(root, toolkit.TypeDirs[recipe.Type], recipe.ID+".pmtiles")
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return nil, err
	}
	layerName := recipe.Build.Config["layer_name"]
	if layerName == "" {
		layerName = recipe.ID
	}
	maxZoom := configInt(recipe.Build.Config, "max_zoom", 14)
	args := []string{"-o", output, "-l", layerName, "-z", strconv.Itoa(maxZoom), "--force"}
	args = append(args, inputs...)
	if _, err := runToolCommand(ctx, root, workdir, BaseToolsImage, "tippecanoe", args); err != nil {
		return nil, err
	}
	info, err := os.Stat(output)
	if err != nil {
		return nil, err
	}
	checksum, _ := downloader.ComputeSHA256(output)
	return []manifest.RealizedEntry{{
		ID: recipe.ID, Type: recipe.Type, Filename: filepath.Base(output),
		RelativePath: filepath.Join(toolkit.TypeDirs[recipe.Type], filepath.Base(output)),
		SizeBytes:    info.Size(), ChecksumSHA256: checksum, SourceStrategy: "build",
	}}, nil
}

func firstFileWithExtension(root, extension string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && filepath.Ext(entry.Name()) == extension {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("no %s file found in %s", extension, root)
	}
	return found, nil
}

func builderContext(opts Options) context.Context {
	if opts.Ctx != nil {
		return opts.Ctx
	}
	return context.Background()
}
