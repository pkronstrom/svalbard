package netutil

import (
	"fmt"
	"net"
)

// Listen tries ports from preferred to preferred+19, returning a listener bound
// to the first that is available.  If none of those work it falls back to an
// OS-assigned ephemeral port.  The caller owns the returned listener.
func Listen(host string, preferred int) (net.Listener, int, error) {
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

// FindAvailablePort returns an available port without holding it open.  Prefer
// Listen when you intend to serve, to avoid a bind race.
func FindAvailablePort(host string, preferred int) (int, error) {
	listener, port, err := Listen(host, preferred)
	if err != nil {
		return 0, err
	}
	listener.Close()
	return port, nil
}
