package maps

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/browser"
)

func Run(ctx context.Context, stdout io.Writer, driveRoot string, opener func(string) error) error {
	if opener == nil {
		opener = browser.Open
	}

	listener, port, err := listenOnAvailablePort("127.0.0.1", 8081)
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

	filesURL := fmt.Sprintf("http://localhost:%d", port)
	viewerPath := filepath.Join(driveRoot, "apps", "map", "index.html")
	if _, err := os.Stat(viewerPath); err == nil {
		viewerURL := filesURL + "/apps/map/"
		if err := opener(viewerURL); err != nil {
			fmt.Fprintf(stdout, "  Open: %s\n", viewerURL)
		}
		fmt.Fprintf(stdout, "Map viewer: %s\n", viewerURL)
	}
	fmt.Fprintf(stdout, "Files: %s\n", filesURL)

	select {
	case <-ctx.Done():
		return server.Close()
	case err := <-errCh:
		return err
	}
}

func listenOnAvailablePort(host string, preferred int) (net.Listener, int, error) {
	for port := preferred; port < preferred+20; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
		if err == nil {
			return listener, port, nil
		}
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return nil, 0, err
	}
	return listener, listener.Addr().(*net.TCPAddr).Port, nil
}
