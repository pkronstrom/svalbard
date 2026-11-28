package frontpage

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// Reading positions travel with the drive, so a book resumes where it was
// left on any computer. The dashboard's port changes per launch, which
// would otherwise reset browser storage.
const (
	readingFile       = "reading.json"
	maxReadingEntries = 500
)

type readingPosition struct {
	Chapter string `json:"chapter"`
	Block   int    `json:"block"`
	Percent int    `json:"percent"`
	At      int64  `json:"at"`
}

func (d *dashboard) hasBook(source string) bool {
	for _, g := range d.inv.Groups {
		for _, e := range g.Items {
			if e.Source == source && e.Icon == "documents" {
				return true
			}
		}
	}
	return false
}

func (d *dashboard) readingPath() string {
	return filepath.Join(d.root, ".svalbard", readingFile)
}

func (d *dashboard) loadReading() map[string]readingPosition {
	positions := map[string]readingPosition{}
	if data, err := os.ReadFile(d.readingPath()); err == nil && len(data) < 1<<20 {
		_ = json.Unmarshal(data, &positions)
	}
	return positions
}

func (d *dashboard) reading(w http.ResponseWriter, r *http.Request) {
	source := r.URL.Query().Get("source")
	if !d.hasBook(source) {
		http.NotFound(w, r)
		return
	}
	d.readingMu.Lock()
	defer d.readingMu.Unlock()
	positions := d.loadReading()
	if r.Method != http.MethodPost {
		if position, ok := positions[source]; ok {
			writeJSON(w, position)
		} else {
			writeJSON(w, nil)
		}
		return
	}
	var position readingPosition
	if !readJSON(w, r, &position) {
		return
	}
	if !fs.ValidPath(position.Chapter) || len(position.Chapter) > 512 || position.Block < 0 || position.Block > 1_000_000 || position.Percent < 0 || position.Percent > 100 {
		http.Error(w, "Invalid reading position", http.StatusBadRequest)
		return
	}
	position.At = time.Now().UnixMilli()
	positions[source] = position
	if len(positions) > maxReadingEntries {
		sources := make([]string, 0, len(positions))
		for key := range positions {
			sources = append(sources, key)
		}
		slices.SortFunc(sources, func(a, b string) int { return int(positions[a].At - positions[b].At) })
		for _, key := range sources[:len(sources)-maxReadingEntries] {
			delete(positions, key)
		}
	}
	if err := writeReading(d.readingPath(), positions); err != nil {
		http.Error(w, "This drive is read-only, so the position stays in this browser.", http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeReading replaces the file atomically so an unplugged drive keeps the
// previous positions rather than a truncated file.
func writeReading(path string, positions map[string]readingPosition) error {
	data, err := json.Marshal(positions)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".reading-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
