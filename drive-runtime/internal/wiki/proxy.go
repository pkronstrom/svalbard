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
			// SilverBullet protects only its dashboard from being framed.
			// The wiki in a frame on another site writes as the user clicks,
			// and its writes are same-origin. It frames its own widgets.
			response.Header.Set("X-Frame-Options", "SAMEORIGIN")
			response.Header.Add("Content-Security-Policy", contentPolicy)
			// No other site may load a page's text as a script or an image.
			response.Header.Set("Cross-Origin-Resource-Policy", "same-origin")
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "The wiki is not answering.", http.StatusBadGateway)
		},
	}
	return guard(port, upstream)
}

// contentPolicy limits what a script running on the wiki's address can
// reach. A page is a program to SilverBullet (Lua in a page has the
// browser's JavaScript), so text that got into a page must not be able to
// send the notes anywhere or pull code in: requests, images, media, fonts,
// frames and scripts all stay on the wiki's own address. An offline wiki
// has no use for any other.
const contentPolicy = "default-src 'self'; " +
	"script-src 'self' 'unsafe-inline' 'unsafe-eval' blob:; " +
	"style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:; media-src 'self' data: blob:; font-src 'self' data:; " +
	"connect-src 'self' data: blob:; worker-src 'self' blob:; frame-src 'self' data: blob:; " +
	"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'"

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
		case r.Method != http.MethodGet && r.Method != http.MethodHead && !fromOwnPage(r):
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

// fromOwnPage reports whether a browser says the request comes from the
// wiki's own page. Browsers set Sec-Fetch-Site themselves; "same-site" is
// another port on localhost, such as a page a model wrote into the workspace,
// and is refused like any other site. A browser too old for the header still
// sends Origin with a write. A write with neither is refused as well: only a
// very old browser sends one from a page, and nothing of the drive's writes
// through the proxy, since the tools write the files.
func fromOwnPage(r *http.Request) bool {
	switch strings.ToLower(strings.TrimSpace(r.Header.Get("Sec-Fetch-Site"))) {
	case "same-origin", "none":
		return true
	case "":
		return strings.EqualFold(r.Header.Get("Origin"), "http://"+r.Host)
	}
	return false
}
