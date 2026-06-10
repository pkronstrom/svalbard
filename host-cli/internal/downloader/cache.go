package downloader

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// CacheEnvVar is the environment variable holding colon-separated directories
// to search (recursively) for already-downloaded files before fetching them.
const CacheEnvVar = "SVALBARD_CACHE_DIRS"

// CacheDirs returns the configured local cache directories, expanding a leading
// "~/". Returns nil when SVALBARD_CACHE_DIRS is unset or empty.
func CacheDirs() []string {
	raw := os.Getenv(CacheEnvVar)
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var dirs []string
	for _, p := range strings.Split(raw, ":") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if p == "~" || strings.HasPrefix(p, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				p = filepath.Join(home, strings.TrimPrefix(p[1:], "/"))
			}
		}
		dirs = append(dirs, p)
	}
	return dirs
}

// FindInCache searches dirs recursively for a non-empty file whose basename
// equals filename, returning the first match.
func FindInCache(dirs []string, filename string) (string, bool) {
	for _, dir := range dirs {
		var found string
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // unreadable entry — skip, keep walking
			}
			if !d.IsDir() && d.Name() == filename {
				if info, statErr := d.Info(); statErr == nil && info.Size() > 0 {
					found = path
					return filepath.SkipAll
				}
			}
			return nil
		})
		if found != "" {
			return found, true
		}
	}
	return "", false
}

// ReuseFromCache copies cachedPath to destPath when it plausibly matches what
// url would serve: it accepts the file if the remote Content-Length matches the
// cached size, or if the remote size can't be determined (offline). It returns
// used=false (without error) when the cached file is unsuitable so the caller
// falls back to a normal download.
func ReuseFromCache(ctx context.Context, cachedPath, url, destPath string, onProgress ...ProgressFunc) (Result, bool, error) {
	info, err := os.Stat(cachedPath)
	if err != nil || info.Size() == 0 {
		return Result{}, false, nil
	}
	if remote, ok := remoteSize(ctx, url); ok && remote != info.Size() {
		return Result{}, false, nil // different version/quant — download instead
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return Result{}, false, fmt.Errorf("create parent dirs: %w", err)
	}
	var progress ProgressFunc
	if len(onProgress) > 0 {
		progress = onProgress[0]
	}
	if err := copyFile(cachedPath, destPath, info.Size(), progress); err != nil {
		return Result{}, false, fmt.Errorf("copy from cache: %w", err)
	}
	sha, err := ComputeSHA256(destPath)
	if err != nil {
		return Result{}, false, fmt.Errorf("hash cached copy: %w", err)
	}
	return Result{Path: destPath, SHA256: sha, Cached: true}, true, nil
}

// remoteSize reports the URL's content length via a HEAD request. ok is false
// when the size can't be determined (network error, or no Content-Length).
func remoteSize(ctx context.Context, url string) (int64, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, false
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.ContentLength <= 0 {
		return 0, false
	}
	return resp.ContentLength, true
}

func copyFile(src, dst string, total int64, progress ProgressFunc) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	if err := streamToFile(out, in, 0, total, progress); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
