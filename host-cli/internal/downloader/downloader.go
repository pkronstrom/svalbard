// Package downloader provides HTTP file downloading with safe resume support.
package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	chunkSize      = 64 * 1024
	hashChunkSz    = 1024 * 1024
	sidecarVersion = 1
)

type Result struct {
	Path, SHA256 string
	Cached       bool
}
type ProgressFunc func(downloaded, total int64)

var httpClient = &http.Client{}

type transferState struct {
	Version        int    `json:"version"`
	URL            string `json:"url"`
	ExpectedSHA256 string `json:"expected_sha256"`
	ETag           string `json:"etag,omitempty"`
	LastModified   string `json:"last_modified,omitempty"`
	Size           int64  `json:"size,omitempty"`
	SHA256         string `json:"sha256,omitempty"`
	Complete       bool   `json:"complete"`
}

func Download(ctx context.Context, url, destPath, expectedSHA256 string, onProgress ...ProgressFunc) (Result, error) {
	result, err := downloadDirect(ctx, url, destPath, expectedSHA256, onProgress...)
	if err == nil || !isPlatformIORegistry(url) {
		return result, err
	}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	var transfer registryTransferError
	if !errors.As(err, &transfer) {
		return Result{}, err
	}
	var progress ProgressFunc
	if len(onProgress) > 0 {
		progress = onProgress[0]
	}
	return downloadRegistryMirror(ctx, url, destPath, expectedSHA256, progress, err)
}

func downloadDirect(ctx context.Context, url, destPath, expectedSHA256 string, onProgress ...ProgressFunc) (Result, error) {
	var progress ProgressFunc
	if len(onProgress) > 0 {
		progress = onProgress[0]
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return Result{}, fmt.Errorf("create parent dirs: %w", err)
	}
	if result, ok := reusableFinal(ctx, url, destPath, expectedSHA256, progress); ok {
		_ = os.Remove(destPath + ".part")
		_ = os.Remove(destPath + ".cache.part")
		return result, nil
	}
	part, statePath := destPath+".part", destPath+".download.json"
	state, validState := loadState(statePath)
	if !validState || state.URL != url || state.ExpectedSHA256 != expectedSHA256 {
		state = transferState{Version: sidecarVersion, URL: url, ExpectedSHA256: expectedSHA256}
		_ = os.Remove(part)
		_ = os.Remove(statePath)
	}
	if state.Complete {
		// Completed metadata may precede either HTTP or cache promotion.
		for _, candidate := range []string{part, destPath + ".cache.part"} {
			result, ok, err := reusablePart(ctx, candidate, destPath, state, progress)
			if err != nil {
				return Result{}, err
			}
			if ok {
				_ = os.Remove(part)
				_ = os.Remove(destPath + ".cache.part")
				return result, nil
			}
		}
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		_ = os.Remove(part)
		_ = os.Remove(destPath + ".cache.part")
		_ = os.Remove(statePath)
		state = transferState{Version: sidecarVersion, URL: url, ExpectedSHA256: expectedSHA256}
	}
	return download(ctx, url, destPath, part, statePath, state, progress, false)
}

func reusableFinal(ctx context.Context, url, dest, expected string, progress ProgressFunc) (Result, bool) {
	info, err := os.Stat(dest)
	if err != nil || info.Size() <= 0 {
		return Result{}, false
	}
	hash, err := computeSHA256Context(ctx, dest)
	if err != nil {
		return Result{}, false
	}
	if expected != "" && hash == expected {
		if progress != nil {
			progress(info.Size(), info.Size())
		}
		return Result{dest, hash, true}, true
	}
	state, ok := loadState(dest + ".download.json")
	if ok && state.Complete && state.URL == url && state.ExpectedSHA256 == expected && state.Size == info.Size() && state.SHA256 == hash {
		if progress != nil {
			progress(info.Size(), info.Size())
		}
		return Result{dest, hash, true}, true
	}
	return Result{}, false
}

func reusablePart(ctx context.Context, part, dest string, state transferState, progress ProgressFunc) (Result, bool, error) {
	info, err := os.Stat(part)
	if os.IsNotExist(err) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, fmt.Errorf("stat completed partial: %w", err)
	}
	if info.Size() <= 0 || (state.Size > 0 && info.Size() != state.Size) {
		return Result{}, false, nil
	}
	hash, err := computeSHA256Context(ctx, part)
	if err != nil {
		return Result{}, false, fmt.Errorf("hash completed partial: %w", err)
	}
	if hash != state.SHA256 || (state.ExpectedSHA256 != "" && hash != state.ExpectedSHA256) {
		return Result{}, false, nil
	}
	if err := os.Rename(part, dest); err != nil {
		return Result{}, false, fmt.Errorf("promote completed partial: %w", err)
	}
	if progress != nil {
		progress(info.Size(), info.Size())
	}
	return Result{dest, hash, true}, true, nil
}

func download(ctx context.Context, url, dest, part, statePath string, state transferState, progress ProgressFunc, retried416 bool) (Result, error) {
	info, _ := os.Stat(part)
	existing := int64(0)
	if info != nil {
		existing = info.Size()
	}
	canResume := existing > 0 && (state.ETag != "" || state.LastModified != "" || state.ExpectedSHA256 != "")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept-Encoding", "identity")
	if canResume {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existing))
		if state.ETag != "" {
			req.Header.Set("If-Range", state.ETag)
		} else if state.LastModified != "" {
			req.Header.Set("If-Range", state.LastModified)
		}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return Result{}, registryTransferError{fmt.Errorf("http request: %w", err)}
	}
	defer resp.Body.Close()
	responseState := updateValidators(state, resp)
	switch resp.StatusCode {
	case http.StatusOK:
		state = updateValidators(transferState{Version: sidecarVersion, URL: url, ExpectedSHA256: state.ExpectedSHA256}, resp)
		existing = 0
		state.Size = resp.ContentLength
		// Reset old bytes before publishing the new identity; persist that
		// identity before incoming bytes so an interrupted first transfer resumes.
		if err := writeResponse(ctx, part, strings.NewReader(""), 0, 0, true, nil); err != nil {
			return Result{}, err
		}
		if err := saveState(statePath, state); err != nil {
			return Result{}, err
		}
		if err := writeResponse(ctx, part, resp.Body, 0, resp.ContentLength, false, progress); err != nil {
			return Result{}, err
		}
	case http.StatusPartialContent:
		start, end, total, ok := contentRange(resp.Header.Get("Content-Range"))
		validatorRecovery := state.ETag != "" || state.LastModified != ""
		if !canResume || !ok || start != existing || end < start || (resp.ContentLength >= 0 && resp.ContentLength != end-start+1) || total <= end || (validatorRecovery && !sameIdentity(state, resp)) {
			return Result{}, fmt.Errorf("invalid partial content response")
		}
		state = responseState
		state.Size = total
		if err := saveState(statePath, state); err != nil {
			return Result{}, err
		}
		writeErr := writeResponse(ctx, part, resp.Body, existing, total, false, progress)
		written, statErr := os.Stat(part)
		// Even an interrupted response must not leave an overlong range
		// available for a later 416 to accept as complete.
		if statErr != nil {
			return Result{}, fmt.Errorf("stat range partial: %w", statErr)
		}
		if written.Size() > end+1 {
			_ = os.Remove(part)
			_ = os.Remove(statePath)
			return Result{}, fmt.Errorf("invalid partial content length")
		}
		if writeErr != nil {
			return Result{}, writeErr
		}
		if written.Size()-existing != end-start+1 {
			_ = os.Remove(part)
			_ = os.Remove(statePath)
			return Result{}, fmt.Errorf("invalid partial content length")
		}
	case http.StatusRequestedRangeNotSatisfiable:
		total, ok := unsatisfiedRange(resp.Header.Get("Content-Range"))
		valid := canResume && ok && total == existing && (sameIdentity(state, resp) || state.ExpectedSHA256 != "")
		if valid {
			hash, hashErr := computeSHA256Context(ctx, part)
			if hashErr != nil {
				return Result{}, fmt.Errorf("validate completed range: %w", hashErr)
			}
			if state.ExpectedSHA256 == "" || hash == state.ExpectedSHA256 {
				state.Size, state.SHA256, state.Complete = existing, hash, true
				return promote(part, dest, statePath, state, progress)
			}
		}
		if retried416 {
			return Result{}, fmt.Errorf("unsafe range not satisfiable response")
		}
		_ = os.Remove(part)
		_ = os.Remove(statePath)
		return download(ctx, url, dest, part, statePath, transferState{Version: sidecarVersion, URL: url, ExpectedSHA256: state.ExpectedSHA256}, progress, true)
	default:
		err := fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
		if resp.StatusCode >= 500 {
			return Result{}, registryTransferError{err}
		}
		return Result{}, err
	}
	info, err = os.Stat(part)
	if err != nil {
		return Result{}, fmt.Errorf("stat partial: %w", err)
	}
	if state.Size > 0 && info.Size() != state.Size {
		return Result{}, fmt.Errorf("incomplete response: got %d bytes, want %d", info.Size(), state.Size)
	}
	hash, err := computeSHA256Context(ctx, part)
	if err != nil {
		return Result{}, fmt.Errorf("compute sha256: %w", err)
	}
	if state.ExpectedSHA256 != "" && hash != state.ExpectedSHA256 {
		return Result{}, fmt.Errorf("sha256 mismatch: got %s, want %s", hash, state.ExpectedSHA256)
	}
	state.Size, state.SHA256, state.Complete = info.Size(), hash, true
	return promote(part, dest, statePath, state, progress)
}

func promote(part, dest, statePath string, state transferState, progress ProgressFunc) (Result, error) {
	if err := saveState(statePath, state); err != nil {
		return Result{}, err
	}
	if err := os.Rename(part, dest); err != nil {
		return Result{}, fmt.Errorf("promote partial: %w", err)
	}
	if progress != nil {
		progress(state.Size, state.Size)
	}
	slog.Debug("download complete", "path", dest, "sha256", state.SHA256)
	return Result{Path: dest, SHA256: state.SHA256}, nil
}
func updateValidators(state transferState, resp *http.Response) transferState {
	if etag := strings.TrimSpace(resp.Header.Get("ETag")); strongETag(etag) {
		state.ETag, state.LastModified = etag, ""
		return state
	}
	if modified := resp.Header.Get("Last-Modified"); modified != "" {
		if changed, err := http.ParseTime(modified); err == nil {
			if date, dateErr := http.ParseTime(resp.Header.Get("Date")); dateErr == nil && date.Sub(changed) >= time.Minute {
				state.LastModified = modified
			}
		}
	}
	return state
}
func strongETag(etag string) bool {
	if len(etag) < 2 || etag[0] != '"' || etag[len(etag)-1] != '"' {
		return false
	}
	for i := 1; i < len(etag)-1; i++ {
		if etag[i] < 0x21 || etag[i] == '"' || etag[i] == 0x7f {
			return false
		}
	}
	return true
}
func sameIdentity(state transferState, resp *http.Response) bool {
	if state.ETag != "" {
		return resp.Header.Get("ETag") == state.ETag
	}
	return state.LastModified != "" && resp.Header.Get("Last-Modified") == state.LastModified
}
func contentRange(value string) (int64, int64, int64, bool) {
	var s, e, t int64
	if _, err := fmt.Sscanf(value, "bytes %d-%d/%d", &s, &e, &t); err != nil {
		return 0, 0, 0, false
	}
	if value != fmt.Sprintf("bytes %d-%d/%d", s, e, t) {
		return 0, 0, 0, false
	}
	return s, e, t, true
}
func unsatisfiedRange(value string) (int64, bool) {
	const prefix = "bytes */"
	if !strings.HasPrefix(value, prefix) {
		return 0, false
	}
	n, err := strconv.ParseInt(strings.TrimPrefix(value, prefix), 10, 64)
	return n, err == nil && n >= 0
}
func loadState(path string) (transferState, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return transferState{}, false
	}
	var state transferState
	if json.Unmarshal(b, &state) != nil || state.Version != sidecarVersion || state.URL == "" {
		return transferState{}, false
	}
	return state, true
}
func saveState(path string, state transferState) error {
	b, err := json.Marshal(state)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".download-*")
	if err != nil {
		return fmt.Errorf("create sidecar: %w", err)
	}
	name := f.Name()
	defer os.Remove(name)
	defer f.Close()
	if _, err = f.Write(b); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func ComputeSHA256(path string) (string, error) {
	return computeSHA256Context(context.Background(), path)
}
func computeSHA256Context(ctx context.Context, path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	buf := make([]byte, hashChunkSz)
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, readErr := f.Read(buf)
		if n > 0 {
			if _, err := h.Write(buf[:n]); err != nil {
				return "", err
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
func writeResponse(ctx context.Context, path string, r io.Reader, base, total int64, truncate bool, progress ProgressFunc) error {
	flags := os.O_CREATE | os.O_WRONLY
	if truncate {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_APPEND
	}
	f, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		return fmt.Errorf("open partial: %w", err)
	}
	if err := streamToFile(ctx, f, r, base, total, progress); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}
func streamToFile(ctx context.Context, f *os.File, r io.Reader, base, total int64, progress ProgressFunc) error {
	buf := make([]byte, chunkSize)
	downloaded := base
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, err := r.Read(buf)
		if n > 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			if _, werr := f.Write(buf[:n]); werr != nil {
				return fmt.Errorf("stream to file: %w", werr)
			}
			downloaded += int64(n)
			if progress != nil {
				progress(downloaded, total)
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("stream to file: %w", err)
		}
	}
}
