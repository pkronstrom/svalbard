package netutil

import (
	"fmt"
	"net"
	"sync"
)

// Listen tries ports from preferred to preferred+19, returning a listener bound
// to the first that is available.  If none of those work it falls back to an
// OS-assigned ephemeral port.  The caller owns the returned listener.
func Listen(host string, preferred int) (net.Listener, int, error) {
	for port := preferred; preferred > 0 && port < preferred+20; port++ {
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

// ReserveEphemeralPort gives a child process a port to bind itself. The OS
// socket is released before returning. The reservation prevents competing
// launchers in this process from choosing that same port before the child
// binds it. Callers release it when the child exits.
func ReserveEphemeralPort(host string) (port int, release func(), err error) {
	ephemeral.Lock()
	defer ephemeral.Unlock()
	for {
		listener, port, err := Listen(host, 0)
		if err != nil {
			return 0, nil, err
		}
		_ = listener.Close()
		if ephemeral.ports[port] {
			continue
		}
		ephemeral.ports[port] = true
		return port, func() {
			ephemeral.Lock()
			delete(ephemeral.ports, port)
			ephemeral.Unlock()
		}, nil
	}
}

var ephemeral = struct {
	sync.Mutex
	ports map[int]bool
}{ports: make(map[int]bool)}
