package frontpage

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
)

func TestDesktopReadyHandoffAndGracefulClose(t *testing.T) {
	root := t.TempDir()
	installTestAtlas(t, root)
	p, err := pathsFor(root)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(p.log)
	defer os.Remove(p.lock)
	lock, free, err := tryLock(p)
	if err != nil || !free {
		t.Fatal("reserve the vault", err)
	}
	defer lock.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := &desktopSession{paths: p}
	done := make(chan error, 1)
	go func() { done <- runDashboard(ctx, nil, root, config.RuntimeConfig{}, nil, session) }()
	deadline := time.Now().Add(5 * time.Second)
	var reply launchReply
	for {
		reply, err = p.request("open")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	parsed, err := url.Parse(reply.URL)
	if err != nil || parsed.Hostname() != "127.0.0.1" || parsed.Fragment != "" || parsed.RawQuery != "" {
		t.Fatalf("non-local or credentialed URL %q", reply.URL)
	}
	info, err := os.Stat(reply.File)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("handoff permissions: %v", err)
	}
	body, err := os.ReadFile(reply.File)
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile(`name="grant" value="([^"]+)"`).FindSubmatch(body)
	if len(match) != 2 {
		t.Fatal("missing handoff form")
	}
	grant := string(match[1])
	client := &http.Client{Timeout: 2 * time.Second}
	post := func(origin string) int {
		req, _ := http.NewRequest("POST", reply.URL+"browser-launch", strings.NewReader(url.Values{"grant": {grant}}.Encode()))
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		return res.StatusCode
	}
	if post("https://example.test") != 403 || post("null") != 200 || post("null") != 403 {
		t.Fatal("grant must be origin-checked and single use")
	}
	if _, err := os.Stat(reply.File); !os.IsNotExist(err) {
		t.Fatalf("redeemed file remains: %v", err)
	}
	err = Launch(root, nil, io.Discard, func(string) error { return errors.New("no default browser") })
	if err == nil || !strings.Contains(err.Error(), reply.URL) || !strings.Contains(err.Error(), p.log) {
		t.Fatalf("missing usable fallback URL/log: %v", err)
	}
	origin := strings.TrimSuffix(reply.URL, "/")
	var reg struct{ Version string }
	if err := json.Unmarshal([]byte(requestBody(t, client, origin+"/api/registry", 200)), &reg); err != nil {
		t.Fatal(err)
	}
	postToDashboard(t, client, origin, "/api/lifecycle", origin, `{"action":"prepare","version":"`+reg.Version+`"}`, 200)
	if err := json.Unmarshal([]byte(requestBody(t, client, origin+"/api/registry", 200)), &reg); err != nil {
		t.Fatal(err)
	}
	postToDashboard(t, client, origin, "/api/lifecycle", origin, `{"action":"close","version":"`+reg.Version+`"}`, 200)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("dashboard failed to stop")
	}
	requireClosed(t, client, origin)
	if filepath.Dir(reply.File) != p.dir {
		t.Fatal("browser ticket must be host-local")
	}
}

func TestSessionFollowsDirectoryIdentity(t *testing.T) {
	root := t.TempDir()
	composed := filepath.Join(root, "Åland")
	if err := os.Mkdir(composed, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(composed, alias); err != nil {
		t.Fatal(err)
	}
	actual, err := pathsFor(composed)
	if err != nil {
		t.Fatal(err)
	}
	variants := []string{alias}
	decomposed := filepath.Join(root, "A\u030aland")
	if _, err := os.Stat(decomposed); err == nil {
		variants = append(variants, decomposed)
	}
	for _, variant := range variants {
		got, err := pathsFor(variant)
		if err != nil || got.socket != actual.socket {
			t.Fatalf("alias did not share the original socket: %s, %v", variant, err)
		}
	}
}

func TestLaunchWaitsForTheRightVaultAndTimesOutWithoutOpening(t *testing.T) {
	root := t.TempDir()
	installTestAtlas(t, root)
	p, err := pathsFor(root)
	if err != nil {
		t.Fatal(err)
	}
	lock, free, err := tryLock(p)
	if err != nil || !free {
		t.Fatal("could not reserve test vault lock", err)
	}
	defer lock.Close()
	defer os.Remove(p.lock)
	// A peer holds the real vault lock, but has never published its verified
	// readiness socket. There must be no browser call or signal to the peer.
	called := false
	err = Launch(root, nil, io.Discard, func(string) error { called = true; return nil })
	if called || err == nil || !strings.Contains(err.Error(), "12 seconds") || !strings.Contains(err.Error(), p.log) {
		t.Fatalf("unready launch: opened %t, error %v", called, err)
	}
}

func TestMissingRecipeExplainsRepair(t *testing.T) {
	root := t.TempDir()
	opened := false
	err := Launch(root, nil, io.Discard, func(string) error { opened = true; return nil })
	if opened || err == nil || !strings.Contains(err.Error(), "Add web-launcher") {
		t.Fatalf("missing recipe: %v", err)
	}
}
