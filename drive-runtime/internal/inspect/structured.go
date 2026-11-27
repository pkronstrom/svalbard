package inspect

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/ncruces/go-sqlite3/driver"
)

// SourceInfo describes a single content source on the drive.
type SourceInfo struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Size        int64    `json:"size"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	// The fields below describe a book. Path is where it is on the drive
	// (books/manuals/volvo.pdf), which is also its source name for search.
	// Title, PagesIndexed and Note come from the search index: PagesIndexed
	// is absent for a book that is not indexed and 0 for one with no text,
	// and Note says why a book is not, or not fully, searchable.
	Path         string `json:"path,omitempty"`
	Title        string `json:"title,omitempty"`
	PagesIndexed *int   `json:"pages_indexed,omitempty"`
	Note         string `json:"note,omitempty"`
	// Documents counts the PDFs and EPUBs indexed from inside an archive;
	// absent for one that holds none.
	Documents *int `json:"documents,omitempty"`
}

// DriveStats holds aggregate drive statistics.
type DriveStats struct {
	Preset  string            `json:"preset"`
	Region  string            `json:"region"`
	Created string            `json:"created"`
	Counts  map[string]int    `json:"counts"`
	Sizes   map[string]string `json:"sizes"`
}

// DatabaseInfo describes a SQLite database on the drive. For a GeoPackage
// (.gpkg, also SQLite) Tables are its layers and Note says how its shapes
// are stored.
type DatabaseInfo struct {
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Tables []string `json:"tables"`
	Note   string   `json:"note,omitempty"`
}

// geoPackageNote tells a model what it cannot read from a layer with SQL.
const geoPackageNote = "GeoPackage: each table is a map layer; its geom column holds the shapes as GeoPackage binary, which query_sql shows only by size. Query the other columns, and export shapes to GeoJSON with the geodata skill."

// DatabaseExts are the extensions of the SQLite files the drive's query
// tools open in data/.
var DatabaseExts = []string{".sqlite", ".gpkg"}

// MapInfo describes a PMTiles map file on the drive.
type MapInfo struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Category string `json:"category,omitempty"`
	Coverage string `json:"coverage,omitempty"`
}

// knownSourceDirs maps directory names to source types and their file extensions.
var knownSourceDirs = []struct {
	dir  string
	typ  string
	exts []string
}{
	{"zim", "zim", []string{".zim"}},
	{"data", "database", DatabaseExts},
	{"maps", "map", []string{".pmtiles"}},
	{"books", "book", []string{".epub", ".pdf"}},
}

// Sources scans known content directories for files with recognized extensions.
// A non-empty filter limits results to that source type.
func Sources(driveRoot, filter string) ([]SourceInfo, error) {

	var sources []SourceInfo
	for _, sd := range knownSourceDirs {
		if filter != "" && sd.typ != filter {
			continue
		}
		dirPath := filepath.Join(driveRoot, sd.dir)
		for _, ext := range sd.exts {
			files, err := ListFilesWithExtension(dirPath, ext)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			for _, f := range files {
				info, err := os.Stat(f)
				if err != nil {
					continue
				}
				name := filepath.Base(f)
				id := strings.TrimSuffix(name, filepath.Ext(name))
				source := SourceInfo{
					ID:   id,
					Type: sd.typ,
					Name: name,
					Size: info.Size(),
				}
				if sd.typ == "book" {
					// Books sit in folders, one per collection, and two
					// collections may hold a file of the same name.
					if rel, err := filepath.Rel(driveRoot, f); err == nil {
						source.Path = filepath.ToSlash(rel)
					}
				}
				sources = append(sources, source)
			}
		}
	}
	return sources, nil
}

// Stats returns aggregate drive statistics from the manifest and directory summaries.
func Stats(driveRoot string) (DriveStats, error) {
	meta, _ := ReadManifestMetadata(filepath.Join(driveRoot, "manifest.yaml"))

	counts := map[string]int{}
	sizes := map[string]string{}

	for _, dir := range []string{"zim", "maps", "models", "data", "apps", "books", "bin"} {
		full := filepath.Join(driveRoot, dir)
		count, size, err := SummarizeDirectory(full)
		if err != nil || count == 0 {
			continue
		}
		counts[dir] = count
		sizes[dir] = HumanSize(size)
	}

	return DriveStats{
		Preset:  meta["preset"],
		Region:  meta["region"],
		Created: meta["created"],
		Counts:  counts,
		Sizes:   sizes,
	}, nil
}

// Databases lists SQLite databases and GeoPackages in the data/ directory,
// including their tables. Table listing is best-effort; Tables will be nil if
// the database cannot be opened.
func Databases(driveRoot string) ([]DatabaseInfo, error) {
	dataDir := filepath.Join(driveRoot, "data")
	var files []string
	for _, ext := range DatabaseExts {
		found, err := ListFilesWithExtension(dataDir, ext)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, err
		}
		files = append(files, found...)
	}
	sort.Strings(files)

	var dbs []DatabaseInfo
	for _, f := range files {
		name := filepath.Base(f)
		rel, err := filepath.Rel(driveRoot, f)
		if err != nil {
			rel = f
		}

		dbInfo := DatabaseInfo{
			Name: name,
			Path: filepath.ToSlash(rel),
		}

		// Best-effort table listing
		if strings.EqualFold(filepath.Ext(f), ".gpkg") {
			dbInfo.Note = geoPackageNote
			if tables, err := listGeoPackageLayers(f); err == nil {
				dbInfo.Tables = tables
			}
		} else if tables, err := listSQLiteTables(f); err == nil {
			dbInfo.Tables = tables
		}

		dbs = append(dbs, dbInfo)
	}
	return dbs, nil
}

// listSQLiteTables opens a SQLite database read-only and returns table names.
func listSQLiteTables(path string) ([]string, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", path))
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// listGeoPackageLayers returns the tables a GeoPackage lists as its contents
// (features and attributes), not its own metadata or spatial index tables.
func listGeoPackageLayers(path string) ([]string, error) {
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", path))
	if err != nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query("SELECT table_name FROM gpkg_contents ORDER BY table_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

// Maps lists PMTiles map files in the maps/ directory.
func Maps(driveRoot string) ([]MapInfo, error) {
	mapsDir := filepath.Join(driveRoot, "maps")
	files, err := ListFilesWithExtension(mapsDir, ".pmtiles")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var maps []MapInfo
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		maps = append(maps, MapInfo{
			Name: filepath.Base(f),
			Size: info.Size(),
		})
	}
	return maps, nil
}
