package netutil

import (
	"fmt"
	"net"
)

// FindAvailablePort tries ports from preferred to preferred+19, returning the
// first that is available.  If none of those work it falls back to an
// OS-assigned ephemeral port.
func FindAvailablePort(host string, preferred int) (int, error) {
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
