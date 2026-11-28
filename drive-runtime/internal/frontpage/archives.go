package frontpage

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/browse"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
)

func (d *dashboard) acquireArchiveGroup(ctx context.Context) (string, <-chan struct{}, func(), error) {
	targets, err := browse.ResolveTargets(d.root, "")
	if err != nil {
		return "", nil, nil, err
	}
	archives := make([]*browse.Archive, 0, len(targets))
	release := func() {
		for _, archive := range archives {
			archive.Close()
		}
	}
	for _, target := range targets {
		archive, err := d.kiwix.Acquire(ctx, filepath.Base(target))
		if err != nil {
			release()
			return "", nil, nil, err
		}
		archives = append(archives, archive)
	}
	first := archives[0]
	select {
	case <-first.Exited:
		release()
		return "", nil, nil, fmt.Errorf("Kiwix stopped while loading the library")
	default:
		return first.BaseURL + "/", first.Exited, release, nil
	}
}

func (d *dashboard) archiveResultURL(ctx context.Context, key string, result search.Result) (string, error) {
	viewer, err := d.readyURL(ctx, key)
	if err != nil {
		return "", err
	}
	base := (&url.URL{Scheme: viewer.Scheme, Host: viewer.Host}).String()
	return search.ArchiveResultURL(base, viewer.Fragment, result), nil
}

// readyURL waits for the shared dashboard job even when two tabs open it together.
func (d *dashboard) readyURL(ctx context.Context, key string) (*url.URL, error) {
	var j *job
	for {
		d.mu.Lock()
		if err := d.ctx.Err(); err != nil {
			d.mu.Unlock()
			return nil, err
		}
		if d.draining {
			d.mu.Unlock()
			return nil, fmt.Errorf("drive is preparing for removal")
		}
		j = d.launchLocked(key, d.specs[key], "")
		stopping := j.Status == "stopping"
		d.mu.Unlock()
		if !stopping {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-j.done:
		}
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-j.ready:
	}

	d.mu.Lock()
	snapshot := j.state
	d.mu.Unlock()
	if snapshot.Status == "failed" {
		return nil, fmt.Errorf("starting reader: %s", snapshot.Text)
	}
	if len(snapshot.Links) == 0 {
		return nil, fmt.Errorf("the service stopped before the result could open")
	}
	viewer, err := url.Parse(snapshot.Links[0].URL)
	if err != nil {
		return nil, fmt.Errorf("reading service address: %w", err)
	}
	return viewer, nil
}

func (d *dashboard) searchArchive(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	spec, ok := d.specs[key]
	if !ok || spec.name != "browse" || spec.target == "" {
		http.NotFound(w, r)
		return
	}
	var input struct{ Query string }
	if !readJSON(w, r, &input) {
		return
	}
	query := strings.TrimSpace(input.Query)
	if query == "" || len(query) > 500 {
		http.Error(w, "Enter a search of up to 500 bytes", http.StatusBadRequest)
		return
	}
	viewer, err := d.readyURL(r.Context(), key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	port, err := strconv.Atoi(viewer.Port())
	if err != nil || viewer.Fragment == "" {
		http.Error(w, "Use Open original to search this archive.", http.StatusServiceUnavailable)
		return
	}
	results, err := search.KiwixKeyword(r.Context(), port, []string{viewer.Fragment}, query, 50)
	if err != nil {
		http.Error(w, "Archive search is unavailable. Open the original to browse its index.", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, results)
}
