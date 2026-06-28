package apps

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/browser"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
)

func Run(ctx context.Context, stdout io.Writer, driveRoot, appName string, opener func(string) error) error {
	appDir := filepath.Join(driveRoot, "apps", appName)
	info, err := os.Stat(appDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("app not found: %s", appDir)
	}
	if opener == nil {
		opener = browser.Open
	}

	listener, port, err := netutil.Listen("127.0.0.1", 8083)
	if err != nil {
		return err
	}
	defer listener.Close()

	server := &http.Server{Handler: http.FileServer(http.Dir(driveRoot))}
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	url := fmt.Sprintf("http://localhost:%d/apps/%s/", port, appName)
	if err := opener(url); err != nil {
		fmt.Fprintf(stdout, "  Open: %s\n", url)
	}
	fmt.Fprintf(stdout, "%s: %s\n", appName, url)

	select {
	case <-ctx.Done():
		return server.Close()
	case err := <-errCh:
		return err
	}
}
