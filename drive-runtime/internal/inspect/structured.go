package inspect

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// SourceInfo describes a single content source on the drive.
type SourceInfo struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Name        string   `json:"name"`
	Size        int64    `json:"size"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// DriveStats holds aggregate drive statistics.
type DriveStats struct {
	Preset  string            `json:"preset"`
	Region  string            `json:"region"`
	Created string            `json:"created"`
	Counts  map[string]int    `json:"counts"`
	Sizes   map[string]string `json:"sizes"`
}

// DatabaseInfo describes a SQLite database on the drive.
type DatabaseInfo struct {
	Name   string   `json:"name"`
	Path   string   `json:"path"`
	Tables []string `json:"tables"`
}

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
	{"data", "database", []string{".sqlite"}},
	{"maps", "map", []string{".pmtiles"}},
	{"books", "book", []string{".epub", ".pdf"}},
}

// Sources scans known content directories for files with recognized extensions.
// If filterType is provided, only sources matching that type are returned.
func Sources(driveRoot string, filterType ...string) ([]SourceInfo, error) {
	var filter string
	if len(filterType) > 0 {
		filter = filterType[0]
	}

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
				sources = append(sources, SourceInfo{
					ID:   id,
					Type: sd.typ,
					Name: name,
					Size: info.Size(),
				})
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

// Databases lists SQLite databases in the data/ directory, including their tables.
// Table listing is best-effort; Tables will be nil if the database cannot be opened.
func Databases(driveRoot string) ([]DatabaseInfo, error) {
	dataDir := filepath.Join(driveRoot, "data")
	files, err := ListFilesWithExtension(dataDir, ".sqlite")
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

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
		tables, err := listSQLiteTables(f)
		if err == nil {
			dbInfo.Tables = tables
		}

		dbs = append(dbs, dbInfo)
	}
	return dbs, nil
}

// listSQLiteTables opens a SQLite database read-only and returns table names.
func listSQLiteTables(path string) ([]string, error) {
	db, err := sql.Open("sqlite", path+"?mode=ro")
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
