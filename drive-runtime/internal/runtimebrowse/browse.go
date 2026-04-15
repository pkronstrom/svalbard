package runtimebrowse

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimebinary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimebrowser"
)

func ResolveTargets(driveRoot, selected string) ([]string, error) {
	if selected != "" {
		path := filepath.Join(driveRoot, "zim", selected)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			return nil, fmt.Errorf("selected zim not found: %s", selected)
		}
		return []string{path}, nil
	}

	pattern := filepath.Join(driveRoot, "zim", "*.zim")
	paths, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	filtered := paths[:0]
	for _, path := range paths {
		if strings.HasPrefix(filepath.Base(path), "._") {
			continue
		}
		filtered = append(filtered, path)
	}
	sort.Strings(filtered)
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no ZIM files found in zim/")
	}
	return filtered, nil
}

func Run(ctx context.Context, stdout io.Writer, driveRoot, selected string, opener func(string) error) error {
	if opener == nil {
		opener = runtimebrowser.Open
	}
	targets, err := ResolveTargets(driveRoot, selected)
	if err != nil {
		return err
	}
	kiwixBin, err := runtimebinary.Resolve("kiwix-serve", driveRoot, platform.Detect)
	if err != nil {
		return fmt.Errorf("kiwix-serve not found")
	}
	port, err := findAvailablePort("127.0.0.1", 8080)
	if err != nil {
		return err
	}

	args := []string{"--port", fmt.Sprintf("%d", port)}
	args = append(args, targets...)
	cmd := exec.CommandContext(ctx, kiwixBin, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stdout
	if err := cmd.Start(); err != nil {
		return err
	}

	url := fmt.Sprintf("http://localhost:%d", port)
	if err := opener(url); err != nil {
		fmt.Fprintf(stdout, "  Open: %s\n", url)
	}
	fmt.Fprintf(stdout, "Kiwix: %s\n", url)

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		return <-waitCh
	case err := <-waitCh:
		return err
	}
}

func findAvailablePort(host string, preferred int) (int, error) {
	for port := preferred; port < preferred+20; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
