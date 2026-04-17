// Package downloader provides HTTP file downloading with resume support
// and SHA256 verification.
package downloader

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
)

const (
	chunkSize    = 64 * 1024   // 64KB for download streaming
	hashChunkSz  = 1024 * 1024 // 1MB for SHA256 computation
)

// Result describes the outcome of a Download call.
type Result struct {
	Path   string
	SHA256 string
	Cached bool
}

// ProgressFunc reports download progress. Called approximately every 64KB.
// downloaded is bytes written so far (including resumed bytes); total is the
// full file size (-1 if unknown).
type ProgressFunc func(downloaded, total int64)

// httpClient has no overall timeout — cancellation is handled via context.
var httpClient = &http.Client{}

// Download fetches url into destPath. If expectedSHA256 is non-empty and the
// file already exists with a matching hash, the download is skipped (cached).
// Partial files are resumed using the HTTP Range header. After a successful
// download the file's SHA256 is verified against expectedSHA256 (if set).
// The provided context controls cancellation of the HTTP request.
func Download(ctx context.Context, url, destPath, expectedSHA256 string, onProgress ...ProgressFunc) (Result, error) {
	var progress ProgressFunc
	if len(onProgress) > 0 {
		progress = onProgress[0]
	}

	// 1. Cache check
	if expectedSHA256 != "" {
		if info, err := os.Stat(destPath); err == nil && info.Size() > 0 {
			hash, err := ComputeSHA256(destPath)
			if err == nil && hash == expectedSHA256 {
				slog.Debug("cache hit", "path", destPath, "sha256", expectedSHA256)
				if progress != nil {
					progress(info.Size(), info.Size())
				}
				return Result{Path: destPath, SHA256: expectedSHA256, Cached: true}, nil
			}
			// Hash mismatch — remove and redownload.
			os.Remove(destPath)
		}
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return Result{}, fmt.Errorf("create parent dirs: %w", err)
	}

	// 2. Resume check — determine existing file size.
	var existingSize int64
	if info, err := os.Stat(destPath); err == nil {
		existingSize = info.Size()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{}, fmt.Errorf("create request: %w", err)
	}
	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
	}
	slog.Debug("downloading", "url", url, "resume_from", existingSize)

	resp, err := httpClient.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()
	slog.Debug("download response", "url", url, "status", resp.StatusCode)

	switch resp.StatusCode {
	case http.StatusOK:
		// Server ignores Range or sends full content — start fresh.
		total := resp.ContentLength // -1 if unknown
		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return Result{}, fmt.Errorf("open file (fresh): %w", err)
		}
		if err := streamToFile(f, resp.Body, 0, total, progress); err != nil {
			f.Close()
			return Result{}, err
		}
		f.Close()

	case http.StatusPartialContent:
		// Append remaining bytes.
		total := int64(-1)
		if resp.ContentLength > 0 {
			total = existingSize + resp.ContentLength
		}
		f, err := os.OpenFile(destPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return Result{}, fmt.Errorf("open file (append): %w", err)
		}
		if err := streamToFile(f, resp.Body, existingSize, total, progress); err != nil {
			f.Close()
			return Result{}, err
		}
		f.Close()

	case http.StatusRequestedRangeNotSatisfiable:
		// File is already complete — nothing to do.
		if progress != nil {
			progress(existingSize, existingSize)
		}

	default:
		return Result{}, fmt.Errorf("unexpected HTTP status %d", resp.StatusCode)
	}

	// 4. Verify SHA256.
	hash, err := ComputeSHA256(destPath)
	if err != nil {
		return Result{}, fmt.Errorf("compute sha256: %w", err)
	}
	slog.Debug("download complete", "path", destPath, "sha256", hash)

	if expectedSHA256 != "" && hash != expectedSHA256 {
		os.Remove(destPath)
		return Result{}, fmt.Errorf("sha256 mismatch: got %s, want %s", hash, expectedSHA256)
	}

	return Result{Path: destPath, SHA256: hash, Cached: false}, nil
}

// ComputeSHA256 returns the hex-encoded SHA256 digest of the file at path,
// reading in 1MB chunks.
func ComputeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, hashChunkSz)
	if _, err := io.CopyBuffer(h, f, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// streamToFile copies from r to f in 64KB chunks, reporting progress.
func streamToFile(f *os.File, r io.Reader, baseBytes, totalBytes int64, progress ProgressFunc) error {
	buf := make([]byte, chunkSize)
	downloaded := baseBytes
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if _, wErr := f.Write(buf[:n]); wErr != nil {
				return fmt.Errorf("stream to file: %w", wErr)
			}
			downloaded += int64(n)
			if progress != nil {
				progress(downloaded, totalBytes)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("stream to file: %w", err)
		}
	}
	return nil
}
