package frontpage

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"

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
	var j *job
	for {
		d.mu.Lock()
		if err := d.ctx.Err(); err != nil {
			d.mu.Unlock()
			return "", err
		}
		j = d.launchLocked(key, d.specs[key], "")
		stopping := j.Status == "stopping"
		d.mu.Unlock()
		if !stopping {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-j.done:
		}
	}

	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-j.ready:
	}

	d.mu.Lock()
	snapshot := j.state
	d.mu.Unlock()
	if snapshot.Status == "failed" {
		return "", fmt.Errorf("starting Kiwix: %s", snapshot.Text)
	}
	if len(snapshot.Links) == 0 {
		return "", fmt.Errorf("the archive stopped before the result could open")
	}
	viewer, err := url.Parse(snapshot.Links[0].URL)
	if err != nil {
		return "", fmt.Errorf("reading Kiwix viewer address: %w", err)
	}
	base := (&url.URL{Scheme: viewer.Scheme, Host: viewer.Host}).String()
	return search.ArchiveResultURL(base, viewer.Fragment, result), nil
}
