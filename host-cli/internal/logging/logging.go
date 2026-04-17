// Package logging provides structured logging setup using log/slog.
package logging

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

// Options configures the logging setup.
type Options struct {
	LogFile string // Path to log file (required).
	Stderr  bool   // Also log to stderr (CLI mode).
	Debug   bool   // Enable debug-level logging.
}

// Init sets up the default slog logger based on the given options.
// It creates parent directories for LogFile, opens the file for append,
// and configures a handler. Returns a cleanup function that closes the
// log file. The caller must invoke cleanup when done (typically deferred).
func Init(opts Options) (cleanup func(), err error) {
	if opts.LogFile == "" {
		return nil, errors.New("logging: LogFile is required")
	}

	if err := os.MkdirAll(filepath.Dir(opts.LogFile), 0o755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(opts.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	level := slog.LevelInfo
	if opts.Debug {
		level = slog.LevelDebug
	}

	handlerOpts := &slog.HandlerOptions{Level: level}

	var handler slog.Handler
	if opts.Stderr {
		w := io.MultiWriter(f, os.Stderr)
		handler = slog.NewTextHandler(w, handlerOpts)
	} else {
		handler = slog.NewJSONHandler(f, handlerOpts)
	}

	slog.SetDefault(slog.New(handler))

	return func() { f.Close() }, nil
}

// DefaultLogPath returns the default log file path. If vaultRoot is
// non-empty it returns <vaultRoot>/.svalbard/svalbard.log, otherwise
// it falls back to os.TempDir()/svalbard.log.
func DefaultLogPath(vaultRoot string) string {
	if vaultRoot != "" {
		return filepath.Join(vaultRoot, ".svalbard", "svalbard.log")
	}
	return filepath.Join(os.TempDir(), "svalbard.log")
}
