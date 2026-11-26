package wiki

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
)

// newProxy returns what the launcher serves on 127.0.0.1:port: SilverBullet,
// reached through its Unix socket, behind the checks SilverBullet does not
// make for itself when it has no accounts. Without a session cookie it skips
// its cross-origin check, so any page open in the user's browser could
// otherwise write the wiki. The socket is the only way in, so nothing goes
// around these checks.
func newProxy(socket string, port int, folder string) http.Handler {
	upstream := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			request.SetURL(&url.URL{Scheme: "http", Host: "silverbullet"})
			// SilverBullet works out its own origin from the Host header.
			request.Out.Host = request.In.Host
		},
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socket)
			},
			// Bodies pass through as SilverBullet encoded them.
			DisableCompression: true,
		},
		// The page holds one event stream open (/.events), and a change on
		// disk must reach it at once.
		FlushInterval: -1,
		ModifyResponse: func(response *http.Response) error {
			if response.Request.URL.Path == pingPath {
				response.Header.Set(folderHeader, folder)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "The wiki is not answering.", http.StatusBadGateway)
		},
	}
	return guard(port, upstream)
}

// guard refuses the requests that must not reach SilverBullet.
func guard(port int, next http.Handler) http.Handler {
	hosts := []string{fmt.Sprintf("localhost:%d", port), fmt.Sprintf("127.0.0.1:%d", port)}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case !strings.EqualFold(r.Host, hosts[0]) && r.Host != hosts[1]:
			// DNS rebinding: another site's name pointed at this address.
			refuse(w, fmt.Sprintf("This wiki answers only at http://%s", hosts[0]))
		case r.Header.Get("Upgrade") != "":
			// SilverBullet has no WebSocket endpoint, and a handshake is a
			// GET that browsers send across origins.
			refuse(w, "This wiki does not accept protocol upgrades.")
		case !cleanPath(r.URL.Path):
			// One spelling per path, so the check below sees what
			// SilverBullet will route.
			http.Error(w, "The path is not in its plain form.", http.StatusBadRequest)
		case fetchProxyPath(r.URL.Path):
			// SilverBullet's own fetch proxy returns a remote page with its
			// content type, which would then run as part of the wiki and
			// pass every check here. An offline wiki never needs it.
			refuse(w, "The wiki's fetch proxy is switched off.")
		case r.Method != http.MethodGet && r.Method != http.MethodHead && crossSite(r):
			refuse(w, "The wiki accepts changes only from its own page.")
		default:
			next.ServeHTTP(w, r)
		}
	})
}

func refuse(w http.ResponseWriter, message string) {
	http.Error(w, message, http.StatusForbidden)
}

// cleanPath reports whether p, already percent-decoded, has no empty, "." or
// ".." part.
func cleanPath(p string) bool {
	if !strings.HasPrefix(p, "/") {
		return false
	}
	cleaned := path.Clean(p)
	return p == cleaned || p == cleaned+"/"
}

// fetchProxyPath reports whether p has ".proxy" as one of its parts, at any
// depth: a space can be mounted below a prefix.
func fetchProxyPath(p string) bool {
	for _, segment := range strings.Split(p, "/") {
		if strings.EqualFold(segment, ".proxy") {
			return true
		}
	}
	return false
}

// crossSite reports whether a browser says the request comes from another
// page than the wiki's own. Browsers set Sec-Fetch-Site themselves, and one
// too old for it still sends Origin with a write. A request with neither is
// not from a page: it is a local program, which could write the files
// directly.
func crossSite(r *http.Request) bool {
	switch strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))) {
	case "cross-site", "same-site":
		// Same-site is another port on localhost: a page a model wrote into
		// the workspace, or Open WebUI.
		return true
	case "":
		origin := r.Header.Get("Origin")
		return origin != "" && !strings.EqualFold(origin, "http://"+r.Host)
	}
	return false
}
