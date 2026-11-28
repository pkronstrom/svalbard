package frontpage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"golang.org/x/net/html"
)

func TestArticleAdapterLimitsMarkupAndArchivePaths(t *testing.T) {
	root, _ := url.Parse("http://localhost:4100/content/book/")
	page := root.ResolveReference(&url.URL{Path: "guide"})
	input := `<html dir="rtl" lang="ar"><div class="mw-parser-output"><script>unsafeMarker()</script><h2 id="section" onclick="unsafeMarker()">Section</h2><a href="second#part">Next</a><img src="/content/another/private.png"><img src="fig.png"><a href="javascript:unsafeMarker()">link</a><table><tr><td>x</td></tr></table></div></html>`
	doc, _ := html.Parse(strings.NewReader(input))
	article := adaptArticle(doc, page, root)
	data, _ := json.Marshal(article)
	s := string(data)
	if strings.Contains(s, "unsafeMarker") || strings.Contains(s, "private.png") || article.Dir != "rtl" || article.Lang != "ar" || !strings.Contains(s, `"path":"fig.png"`) || !strings.Contains(s, `"anchor":"part"`) || !strings.Contains(s, `"tag":"table"`) {
		t.Fatalf("unexpected article: %s", s)
	}
	for _, extra := range []string{"<main>Interactive application</main>", `<div class="mw-parser-output"><form></form></div>`} {
		doc, _ = html.Parse(strings.NewReader(extra))
		if len(adaptArticle(doc, page, root).Nodes) != 0 {
			t.Fatal("unsupported content escaped original-mode fallback")
		}
	}
}
func TestServiceRegistryRestartDrainAndOwnership(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "apps/field/index.html", "field application")
	cfg := config.RuntimeConfig{Groups: []config.MenuGroup{{Label: "Apps", Items: []config.MenuItem{
		{ID: "field", Label: "Field", Action: config.BuiltinAction("apps", map[string]string{"app": "field"})},
		{ID: "missing", Label: "Missing", Action: config.BuiltinAction("apps", map[string]string{"app": "missing"})},
	}}}}
	client, base, shutdown := startDashboard(t, root, cfg)
	defer shutdown()
	var inv inventory
	readResponse(t, requestBody(t, client, base+"/api/inventory", 200), &inv)
	key := inv.Groups[0].Items[0].Service
	missing := inv.Groups[0].Items[1].Service
	reg := func() registrySnapshot {
		var snap registrySnapshot
		readResponse(t, requestBody(t, client, base+"/api/registry", 200), &snap)
		return snap
	}
	action := func(name, id, v string, code int) {
		payload, _ := json.Marshal(map[string]string{"action": name, "id": id, "version": v})
		postToDashboard(t, client, base, "/api/lifecycle", base, string(payload), code)
	}
	postToDashboard(t, client, base, "/api/services/"+key, base, "{}", 200)
	waitForState(t, client, base, key, "running")
	snap := reg()
	if len(snap.Records) != 1 || snap.Records[0].PID == 0 || len(snap.Records[0].Events) < 2 {
		t.Fatal(snap)
	}
	action("restart", snap.Records[0].ID, "stale", 409)
	action("restart", snap.Records[0].ID, snap.Version, 200)
	waitForState(t, client, base, key, "running")
	postToDashboard(t, client, base, "/api/services/"+missing, base, "{}", 200)
	waitForState(t, client, base, missing, "failed")
	action("stop-all", "", reg().Version, 200)
	waitForState(t, client, base, key, "stopped")
	action("prepare", "", reg().Version, 200)
	if !reg().Draining {
		t.Fatal("new launches still allowed")
	}
	postToDashboard(t, client, base, "/api/services/"+key, base, "{}", http.StatusServiceUnavailable)
	action("resume", "", "", 200)
	if reg().Draining {
		t.Fatal("still draining")
	}
}
func TestLiveKiwixRecordsCountOneListener(t *testing.T) {
	d := dashboard{ctx: context.Background(), jobs: map[string]*job{}, specs: map[string]serviceSpec{}, resourceLabels: map[string][]string{}}
	for _, name := range []string{"first", "second"} {
		d.jobs[name] = &job{state: state{Status: "running", Links: []link{{Label: "Open in Kiwix", URL: "http://localhost:44000/viewer#" + name}}}, pid: 27}
		d.specs[name] = serviceSpec{name: "browse", target: name + ".zim"}
		d.resourceLabels[name] = []string{name}
	}
	v := d.registryLocked()
	if len(v.Records) != 1 || len(v.Records[0].Keys) != 2 || len(v.Records[0].Resources) != 2 || v.Records[0].PID != 27 {
		t.Fatal(v)
	}
}

func TestRemovalWaitsForTerminalAndAccountsForIncompleteStops(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	j := &job{state: state{Status: "running"}, done: make(chan struct{}), cancel: func() {}}
	d := dashboard{ctx: ctx, terminals: 1, jobs: map[string]*job{"example": j}}
	call := func(ctx context.Context, action string) *httptest.ResponseRecorder {
		v := d.registryLocked()
		payload, _ := json.Marshal(map[string]string{"action": action, "version": v.Version})
		req := httptest.NewRequest(http.MethodPost, "/api/lifecycle", strings.NewReader(string(payload))).WithContext(ctx)
		w := httptest.NewRecorder()
		d.lifecycle(w, req)
		return w
	}
	if w := call(ctx, "prepare"); w.Code != 409 || d.draining || j.Status != "running" {
		t.Fatal(w.Code, w.Body)
	}
	d.terminals = 0
	expired, stop := context.WithCancel(ctx)
	stop()
	if w := call(expired, "prepare"); w.Code != 503 || j.Status != "stopping" {
		t.Fatal(w.Code, w.Body)
	}
}

func TestRemovalDetectsUnownedOpenFileAndLeavesItAlone(t *testing.T) {
	if _, err := exec.LookPath("lsof"); err != nil {
		t.Skip("lsof unavailable")
	}
	root := t.TempDir()
	writeFile(t, root, "data", "small fixture")
	cmd := exec.Command("sh", "-c", `exec 3<"$1/data"; echo ready; exec sleep 12`, "reader", root)
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { cmd.Process.Kill(); cmd.Wait() }()
	var ready [6]byte
	if _, err = out.Read(ready[:]); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	result := removalStatus(ctx, root)
	if !strings.Contains(result, "Removal blocked") || !strings.Contains(result, strconv.Itoa(cmd.Process.Pid)) {
		t.Fatal(result)
	}
	if err = cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatal(err)
	}
}

func TestRemovalWaitsForSettingsWriter(t *testing.T) {
	d := dashboard{ctx: t.Context(), root: t.TempDir()}
	payload, err := json.Marshal(map[string]string{"action": "prepare", "version": d.registryLocked().Version})
	if err != nil {
		t.Fatal(err)
	}
	d.settingsMu.Lock()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		d.lifecycle(w, httptest.NewRequest(http.MethodPost, "/api/lifecycle", strings.NewReader(string(payload))))
		done <- w
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		d.mu.Lock()
		draining := d.draining
		d.mu.Unlock()
		if draining {
			break
		}
		if time.Now().After(deadline) {
			d.settingsMu.Unlock()
			t.Fatal("preparation did not start")
		}
		time.Sleep(time.Millisecond)
	}
	select {
	case <-done:
		d.settingsMu.Unlock()
		t.Fatal("removal completed during a settings write")
	default:
	}
	d.settingsMu.Unlock()
	select {
	case w := <-done:
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
	case <-time.After(7 * time.Second):
		t.Fatal("removal did not resume")
	}
}
