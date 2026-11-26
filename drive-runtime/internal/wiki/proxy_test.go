package wiki

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// proxied is the launcher's proxy in front of a stand-in SilverBullet.
type proxied struct {
	addr      string // 127.0.0.1:<port>
	port      int
	pages     string
	recordDir string
}

func newProxied(t *testing.T) proxied {
	t.Helper()
	pages, recordDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(pages, "index.md"), []byte("start\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A socket path has to be short; the test's own folder is not.
	socketDir, err := os.MkdirTemp("/tmp", "sbwt-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "s")
	upstream, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	silverbullet := &http.Server{Handler: fakeSilverBullet(pages, recordDir)}
	go silverbullet.Serve(upstream)
	t.Cleanup(func() { silverbullet.Close() })

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	proxy := &http.Server{Handler: newProxy(socket, port, pages)}
	go proxy.Serve(listener)
	t.Cleanup(func() { proxy.Close() })
	return proxied{addr: listener.Addr().String(), port: port, pages: pages, recordDir: recordDir}
}

// send writes one request as given, so a path or header no client library
// would produce reaches the proxy, and returns the status and the body.
func (p proxied) send(t *testing.T, method, target string, headers map[string]string, body string) (int, string) {
	t.Helper()
	conn, err := net.Dial("tcp", p.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	request := fmt.Sprintf("%s %s HTTP/1.1\r\n", method, target)
	if _, set := headers["Host"]; !set {
		request += fmt.Sprintf("Host: localhost:%d\r\n", p.port)
	}
	for key, value := range headers {
		request += key + ": " + value + "\r\n"
	}
	request += fmt.Sprintf("Content-Length: %d\r\nConnection: close\r\n\r\n%s", len(body), body)
	if _, err := io.WriteString(conn, request); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatalf("%s %s: %v", method, target, err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(data)
}

// unchanged fails the test if anything in the page folder differs from the
// one page it started with, or the fetch proxy was reached.
func (p proxied) unchanged(t *testing.T, what string) {
	t.Helper()
	entries, _ := os.ReadDir(p.pages)
	if len(entries) != 1 {
		t.Errorf("%s: the page folder now holds %v", what, entries)
	}
	if data, _ := os.ReadFile(filepath.Join(p.pages, "index.md")); string(data) != "start\n" {
		t.Errorf("%s: the start page is now %q", what, data)
	}
	if path, err := os.ReadFile(filepath.Join(p.recordDir, "proxied")); err == nil {
		t.Errorf("%s: SilverBullet's fetch proxy was reached with %q", what, path)
	}
}

func TestProxyPassesTheWikisOwnRequests(t *testing.T) {
	p := newProxied(t)
	own := fmt.Sprintf("http://localhost:%d", p.port)

	// The page's reads, under either name of this computer.
	for _, host := range []string{fmt.Sprintf("localhost:%d", p.port), fmt.Sprintf("127.0.0.1:%d", p.port), fmt.Sprintf("LOCALHOST:%d", p.port)} {
		status, body := p.send(t, "GET", "/Projects/Garden%20plan", map[string]string{"Host": host}, "")
		// SilverBullet sees the Host the browser sent.
		if want := "path=/Projects/Garden plan host=" + host + " forwarded="; status != 200 || body != want {
			t.Errorf("GET with Host %s = %d %q, want 200 %q", host, status, body, want)
		}
	}
	// The page's writes: a current browser, and one too old to say where a
	// request comes from other than by Origin.
	for name, headers := range map[string]map[string]string{
		"same-origin fetch":        {"Sec-Fetch-Site": "same-origin", "Origin": own},
		"browser without metadata": {"Origin": own},
		"the other loopback name":  {"Host": fmt.Sprintf("127.0.0.1:%d", p.port), "Origin": fmt.Sprintf("http://127.0.0.1:%d", p.port)},
		"a typed address":          {"Sec-Fetch-Site": "none"},
	} {
		file := strings.ReplaceAll(name, " ", "-") + ".md"
		if status, body := p.send(t, "PUT", "/.fs/"+file, headers, "written"); status != 200 {
			t.Errorf("%s: PUT = %d %q, want 200", name, status, body)
		}
		if data, _ := os.ReadFile(filepath.Join(p.pages, file)); string(data) != "written" {
			t.Errorf("%s: the page was not written (%q)", name, data)
		}
	}
	if status, _ := p.send(t, "DELETE", "/.fs/a-typed-address.md", map[string]string{"Sec-Fetch-Site": "same-origin"}, ""); status != 200 {
		t.Errorf("same-origin DELETE = %d, want 200", status)
	}
	// A cross-site read is the browser's to keep from the other page; the
	// proxy does not answer it differently.
	if status, _ := p.send(t, "GET", "/.fs/index.md", map[string]string{"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"}, ""); status != 200 {
		t.Errorf("cross-site GET = %d, want 200", status)
	}
}

func TestProxyRefusesAForeignHost(t *testing.T) {
	p := newProxied(t)
	// DNS rebinding: evil.example now resolves to 127.0.0.1, and its page is
	// same-origin with the name it was loaded from.
	for _, host := range []string{
		fmt.Sprintf("evil.example:%d", p.port), "evil.example", "localhost", fmt.Sprintf("localhost:%d", p.port+1),
		fmt.Sprintf("localhost.evil.example:%d", p.port), fmt.Sprintf("127.0.0.1.nip.io:%d", p.port), fmt.Sprintf("[::1]:%d", p.port), "",
	} {
		for _, method := range []string{"GET", "PUT"} {
			headers := map[string]string{"Host": host, "Sec-Fetch-Site": "same-origin", "Origin": "http://" + host}
			if status, body := p.send(t, method, "/.fs/rebound.md", headers, "owned"); status != 403 && status != 400 {
				t.Errorf("%s with Host %q = %d %q, want a refusal", method, host, status, body)
			}
		}
	}
	p.unchanged(t, "foreign Host")
}

func TestProxyRefusesCrossSiteWrites(t *testing.T) {
	p := newProxied(t)
	for name, headers := range map[string]map[string]string{
		// A form on another site: no preflight, and a body the server would take.
		"foreign Origin, text/plain": {"Origin": "https://evil.example", "Content-Type": "text/plain"},
		"cross-site fetch":           {"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"},
		"cross-site without Origin":  {"Sec-Fetch-Site": "cross-site"},
		// Another port on localhost: a page a model wrote, or Open WebUI.
		"same-site fetch":             {"Sec-Fetch-Site": "same-site", "Origin": "http://localhost:8086"},
		"same-site claiming our name": {"Sec-Fetch-Site": "same-site", "Origin": fmt.Sprintf("http://localhost:%d", p.port)},
		"other port, old browser":     {"Origin": "http://localhost:8086"},
		"https spelling of our name":  {"Origin": fmt.Sprintf("https://localhost:%d", p.port)},
		"sandboxed page":              {"Origin": "null"},
		// Neither header: a browser from before 2020 posting a form. Nothing
		// of the drive's writes through the proxy, so nothing is lost.
		"no word on where it is from": {},
		"a value not known today":     {"Sec-Fetch-Site": "same-party"},
	} {
		for _, method := range []string{"POST", "PUT", "DELETE", "PATCH", "OPTIONS"} {
			target := "/.fs/owned.md"
			if method == "DELETE" {
				target = "/.fs/index.md"
			}
			status, body := p.send(t, method, target, headers, "owned")
			if status != 403 || !strings.Contains(body, "only from its own page") {
				t.Errorf("%s, %s = %d %q, want 403", name, method, status, body)
			}
		}
		p.unchanged(t, name)
	}
}

func TestProxyRefusesTheFetchProxyInEverySpelling(t *testing.T) {
	p := newProxied(t)
	for _, target := range []string{
		"/.proxy/evil.example/page.html",
		"/.proxy/https://evil.example/",
		"/.proxy",
		"/.proxy/",
		"/%2Eproxy/evil.example/",
		"/%2eproxy/evil.example/",
		"/.proxy%2Fevil.example/",
		"/.PROXY/evil.example/",
		"//.proxy/evil.example/",
		"/./.proxy/evil.example/",
		"/.fs/../.proxy/evil.example/",
		"/.fs/%2E%2E/.proxy/evil.example/",
		"/space/.proxy/evil.example/",
		"/.proxy/evil.example/?x=1",
	} {
		for _, method := range []string{"GET", "POST", "HEAD"} {
			// From the wiki's own page, which passes every other check.
			status, body := p.send(t, method, target, map[string]string{"Sec-Fetch-Site": "same-origin"}, "")
			if status != 403 && status != 400 {
				t.Errorf("%s %s = %d %q, want a refusal", method, target, status, body)
			}
			if strings.Contains(body, "remote()") {
				t.Errorf("%s %s returned the remote page", method, target)
			}
		}
	}
	p.unchanged(t, "fetch proxy")

	// A path that only resembles it is a page like any other.
	for _, target := range []string{"/.proxyish", "/Notes/proxy", "/x.proxy/y"} {
		if status, _ := p.send(t, "GET", target, nil, ""); status != 200 {
			t.Errorf("GET %s = %d, want 200", target, status)
		}
	}
}

func TestProxyRefusesPathsThatAreNotInPlainForm(t *testing.T) {
	p := newProxied(t)
	for _, target := range []string{"//x", "/a//b", "/a/./b", "/a/../b", "/..", "/%2e%2e/etc", "/a/%2E/b"} {
		if status, body := p.send(t, "GET", target, nil, ""); status != 400 {
			t.Errorf("GET %s = %d %q, want 400", target, status, body)
		}
	}
	// A folder's address ends in a slash.
	for _, target := range []string{"/", "/.fs/", "/Projects/"} {
		if status, _ := p.send(t, "GET", target, nil, ""); status != 200 && status != 404 {
			t.Errorf("GET %s = %d, want it passed on", target, status)
		}
	}
}

func TestProxyRefusesProtocolUpgrades(t *testing.T) {
	p := newProxied(t)
	// A WebSocket handshake is a GET, and browsers send it across origins.
	status, body := p.send(t, "GET", "/.events", map[string]string{
		"Upgrade": "websocket", "Connection": "Upgrade", "Sec-WebSocket-Key": "dGhlIHNhbXBsZSBub25jZQ==", "Sec-WebSocket-Version": "13",
		"Origin": "https://evil.example",
	}, "")
	if status != 403 || !strings.Contains(body, "protocol upgrades") {
		t.Errorf("upgrade = %d %q, want 403", status, body)
	}
}

func TestProxyDoesNotPassOnForwardingHeaders(t *testing.T) {
	p := newProxied(t)
	// SilverBullet trusts X-Forwarded-Proto to decide what its origin is.
	_, body := p.send(t, "GET", "/page", map[string]string{"X-Forwarded-For": "203.0.113.9", "X-Forwarded-Proto": "https"}, "")
	if !strings.HasSuffix(body, "forwarded=") {
		t.Errorf("SilverBullet saw %q, want no forwarding headers", body)
	}
}

func TestProxyStreamsEventsAsTheyHappen(t *testing.T) {
	p := newProxied(t)
	conn, err := net.Dial("tcp", p.addr)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	fmt.Fprintf(conn, "GET /.events HTTP/1.1\r\nHost: localhost:%d\r\nAccept: text/event-stream\r\n\r\n", p.port)
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	// The body is not closed: that would wait for the end of a stream that
	// has none. Closing the connection ends it.
	if got := response.Header.Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("content type = %q, want text/event-stream", got)
	}
	// The stream never ends; the event must arrive while it is open.
	reader := bufio.NewReader(response.Body)
	var received string
	for !strings.Contains(received, "data: changed\n") {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("the event did not arrive while the stream was open: %v (got %q)", err, received)
		}
		received += line
	}
}

func TestProxyKeepsTheWikiOutOfOtherSitesFrames(t *testing.T) {
	p := newProxied(t)
	// SilverBullet sends these on its dashboard only. A wiki framed by
	// another site writes as the user clicks, and its writes are same-origin.
	for _, target := range []string{"/", "/index", "/.fs/index.md", pingPath} {
		response, err := http.Get("http://" + p.addr + target)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if got := response.Header.Get("X-Frame-Options"); got != "SAMEORIGIN" {
			t.Errorf("GET %s X-Frame-Options = %q, want SAMEORIGIN", target, got)
		}
		policy := response.Header.Get("Content-Security-Policy")
		// A script on the wiki's address reaches nothing but the wiki.
		for _, want := range []string{
			"frame-ancestors 'self'", "form-action 'self'", "default-src 'self'", "connect-src 'self' data: blob:",
			"img-src 'self' data: blob:", "object-src 'none'", "base-uri 'self'",
		} {
			if !strings.Contains(policy, want) {
				t.Errorf("GET %s Content-Security-Policy = %q, want it to hold %q", target, policy, want)
			}
		}
		for _, directive := range strings.Split(policy, ";") {
			if strings.Contains(directive, "http:") || strings.Contains(directive, "https:") || strings.Contains(directive, "*") {
				t.Errorf("GET %s policy lets another address in: %q", target, directive)
			}
		}
		// A page's text must not load as another site's script or image.
		if got := response.Header.Get("Cross-Origin-Resource-Policy"); got != "same-origin" {
			t.Errorf("GET %s Cross-Origin-Resource-Policy = %q, want same-origin", target, got)
		}
	}
}

func TestProxyNamesItsFolderOnPingOnly(t *testing.T) {
	p := newProxied(t)
	for target, want := range map[string]string{pingPath: p.pages, "/index": "", "/.fs/index.md": ""} {
		response, err := http.Get("http://" + p.addr + target)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if got := response.Header.Get(folderHeader); got != want {
			t.Errorf("GET %s folder header = %q, want %q", target, got, want)
		}
	}
}

func TestProxyAnswersWhenSilverBulletIsGone(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	proxy := &http.Server{Handler: newProxy(filepath.Join(t.TempDir(), "gone"), port, "/pages")}
	go proxy.Serve(listener)
	defer proxy.Close()
	response, err := http.Get(fmt.Sprintf("http://localhost:%d/index", port))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusBadGateway || !strings.Contains(string(body), "The wiki is not answering") {
		t.Errorf("answer = %d %q, want 502 and a plain sentence", response.StatusCode, body)
	}
}
