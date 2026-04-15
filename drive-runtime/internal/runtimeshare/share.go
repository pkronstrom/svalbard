package runtimeshare

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
)

func Run(ctx context.Context, stdout io.Writer, driveRoot string) error {
	listener, port, err := listenOnAvailablePort("0.0.0.0", 8080)
	if err != nil {
		return err
	}
	defer listener.Close()

	server := &http.Server{
		Handler: Handler(driveRoot),
	}
	defer server.Close()

	errCh := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	ip := lanIP()
	fmt.Fprintln(stdout, "Sharing drive on local network")
	fmt.Fprintln(stdout, "==============================")
	fmt.Fprintln(stdout)
	fmt.Fprintf(stdout, "  http://%s:%d\n", ip, port)
	fmt.Fprintln(stdout)
	fmt.Fprintln(stdout, "  Tell others to open this address in their browser.")

	select {
	case <-ctx.Done():
		return server.Close()
	case err := <-errCh:
		return err
	}
}

func Handler(driveRoot string) http.Handler {
	return http.FileServer(http.Dir(driveRoot))
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

func lanIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "0.0.0.0"
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		if ip := ipNet.IP.To4(); ip != nil {
			return ip.String()
		}
	}
	return "0.0.0.0"
}
