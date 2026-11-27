package frontpage

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

func TestWikiAndServeEverythingShareTheEditor(t *testing.T) {
	root := t.TempDir()
	target, err := platform.Detect()
	if err != nil {
		t.Skip(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join("bin", target, "silverbullet")
	writeFile(t, root, binary, fmt.Sprintf("#!/bin/sh\nexec '%s' -test.run '^TestSilverBulletFixture$'\n", self))
	if err := os.Chmod(filepath.Join(root, binary), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.RuntimeConfig{Groups: []config.MenuGroup{{Label: "Tools", Items: []config.MenuItem{
		{Label: "Wiki", Action: config.BuiltinAction("wiki", nil)},
		{Label: "Serve Everything", Action: config.BuiltinAction("serve-all", nil)},
	}}}}
	client, origin, _ := startDashboard(t, root, cfg)
	var inv inventory
	readResponse(t, requestBody(t, client, origin+"/api/inventory", http.StatusOK), &inv)
	wikiKey, allKey := inv.Groups[0].Items[0].Service, inv.Groups[0].Items[1].Service
	postToDashboard(t, client, origin, "/api/services/"+wikiKey, origin, "{}", http.StatusOK)
	address := waitForState(t, client, origin, wikiKey, "running").Links[0].URL
	requestBody(t, client, address+"/.ping", http.StatusOK)

	postToDashboard(t, client, origin, "/api/services/"+allKey, origin, "{}", http.StatusOK)
	var shared string
	var all map[string]state
	for i := 0; i < 75 && shared == ""; i++ {
		readResponse(t, requestBody(t, client, origin+"/api/services", http.StatusOK), &all)
		for _, link := range all[allKey].Links {
			if link.Label == "Wiki (local only)" {
				shared = link.URL
				break
			}
		}
		if shared == "" {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if shared != address {
		t.Fatalf("Serve Everything used wiki URL %q, want %q", shared, address)
	}
	postToDashboard(t, client, origin, "/api/services/"+wikiKey+"/stop", origin, "{}", http.StatusOK)
	waitForState(t, client, origin, wikiKey, "stopped")
	requestBody(t, client, address+"/.ping", http.StatusOK)

	postToDashboard(t, client, origin, "/api/services/"+allKey+"/stop", origin, "{}", http.StatusOK)
	waitForState(t, client, origin, allKey, "stopped")
	requireClosed(t, client, address)
}

func TestSilverBulletFixture(t *testing.T) {
	socket := os.Getenv("SB_UNIX_SOCKET")
	if socket == "" {
		return
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	t.Fatal(http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "OK")
	})))
}
