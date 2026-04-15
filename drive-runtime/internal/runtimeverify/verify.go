package runtimeverify

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func Run(w io.Writer, driveRoot string) error {
	checksumPath := filepath.Join(driveRoot, ".svalbard", "checksums.sha256")
	file, err := os.Open(checksumPath)
	if err != nil {
		return fmt.Errorf("no checksum file found. verification not available for this drive")
	}
	defer file.Close()

	fmt.Fprintln(w, "Verifying drive integrity")
	fmt.Fprintln(w, "========================")

	var passed, failed, missing int
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid checksum line: %q", line)
		}
		expectedHash := strings.TrimSpace(parts[0])
		relPath := strings.TrimSpace(parts[1])
		fullPath := filepath.Join(driveRoot, filepath.FromSlash(relPath))

		data, err := os.ReadFile(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(w, "  MISSING  %s\n", relPath)
				missing++
				continue
			}
			return err
		}
		actualHash := fmt.Sprintf("%x", sha256.Sum256(data))
		if actualHash == expectedHash {
			fmt.Fprintf(w, "  OK       %s\n", relPath)
			passed++
		} else {
			fmt.Fprintf(w, "  FAIL     %s\n", relPath)
			failed++
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  Passed: %d  Failed: %d  Missing: %d\n", passed, failed, missing)
	return nil
}
