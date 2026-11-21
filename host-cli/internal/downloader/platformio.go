package downloader

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// registryTransferError marks a connection/server failure, not a local write,
// cancellation, unsafe range response or rejected artifact checksum.
type registryTransferError struct{ err error }

func (e registryTransferError) Error() string { return e.err.Error() }
func (e registryTransferError) Unwrap() error { return e.err }

func isPlatformIORegistry(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "dl.registry.platformio.org" && u.User == nil && strings.HasPrefix(u.Path, "/download/")
}

// Alternate mirrors use a separate transfer file/sidecar. The preferred
// mirror's partial remains intact until a verified alternate has completed.
func downloadRegistryMirror(ctx context.Context, source, dest, expected string, progress ProgressFunc, firstErr error) (Result, error) {
	client := *httpClient
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	visited := []string{}
	latestErr := firstErr
	for attempt := 0; attempt < 4; attempt++ {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		endpoint, _ := url.Parse(source)
		q := endpoint.Query()
		if len(visited) > 0 {
			q.Set("bypass", strings.Join(visited, ","))
		}
		endpoint.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint.String(), nil)
		if err != nil {
			return Result{}, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return Result{}, fmt.Errorf("registry mirror discovery: %w", err)
		}
		resp.Body.Close()
		mirror := resp.Header.Get("X-PIO-Mirror")
		location := resp.Header.Get("Location")
		digest := strings.ToLower(resp.Header.Get("X-PIO-Content-SHA256"))
		if (resp.StatusCode != 302 && resp.StatusCode != 307) || mirror == "" || location == "" {
			break
		}
		repeated := false
		for _, v := range visited {
			if v == mirror {
				repeated = true
			}
		}
		if repeated {
			break
		}
		visited = append(visited, mirror)
		bytes, err := hex.DecodeString(digest)
		if err != nil || len(bytes) != 32 || (expected != "" && digest != strings.ToLower(expected)) {
			return Result{}, fmt.Errorf("registry checksum disagrees with pinned artifact for %s", source)
		}
		actual, err := url.Parse(location)
		if err != nil || actual.Scheme != "https" || actual.Host == "" || actual.User != nil {
			return Result{}, fmt.Errorf("invalid official registry mirror location")
		}
		alternate := dest + ".mirror"
		result, err := downloadDirect(ctx, location, alternate, digest, progress)
		if err != nil {
			latestErr = err
			var transfer registryTransferError
			if !errors.As(err, &transfer) {
				return Result{}, err
			}
			continue
		}
		info, err := os.Stat(alternate)
		if err != nil {
			return Result{}, err
		}
		state := transferState{Version: sidecarVersion, URL: source, ExpectedSHA256: expected, Size: info.Size(), SHA256: result.SHA256, Complete: true}
		result, err = promote(alternate, dest, dest+".download.json", state, progress)
		if err != nil {
			return Result{}, err
		}
		for _, suffix := range []string{".part", ".cache.part", ".mirror.part", ".mirror.download.json"} {
			_ = os.Remove(dest + suffix)
		}
		return result, nil
	}
	return Result{}, fmt.Errorf("official registry mirrors exhausted: %w", latestErr)
}
