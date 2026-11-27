package frontpage

import (
	"net/http"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
)

func TestSettingsTerminalUsesDriveSettings(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".svalbard/actions.json", "{}")
	cfg := config.RuntimeConfig{Groups: []config.MenuGroup{{Label: "Settings", Items: []config.MenuItem{{
		ID: "settings", Label: "Settings", Action: config.BuiltinAction("settings", nil),
	}}}}}
	client, origin, _ := startDashboard(t, root, cfg)

	body := requestBody(t, client, origin+"/api/inventory", http.StatusOK)
	if !strings.Contains(body, `"terminal":true`) || !strings.Contains(body, `"command":"./run settings"`) {
		t.Fatalf("Settings has no prefilled terminal in inventory: %s", body)
	}
	invoke := func(body string) (string, bool) {
		var result struct {
			Output string
			Failed bool
		}
		readResponse(t, postToDashboard(t, client, origin, "/api/terminal/settings", origin, body, http.StatusOK), &result)
		return result.Output, result.Failed
	}
	if output, failed := invoke(`{"args":[]}`); failed || !strings.Contains(output, "ai.context") {
		t.Fatalf("settings list = %q, failed = %v", output, failed)
	}
	if output, failed := invoke(`{"args":["ai.context","fast"]}`); failed || output != "ai.context = fast\n" {
		t.Fatalf("settings set = %q, failed = %v", output, failed)
	}
	if output, failed := invoke(`{"args":["ai.context"]}`); failed || output != "fast\n" {
		t.Fatalf("settings get = %q, failed = %v", output, failed)
	}
	if output, failed := invoke(`{"args":["--reset","ai.context"]}`); failed || output != "ai.context = ask (default)\n" {
		t.Fatalf("settings reset = %q, failed = %v", output, failed)
	}
	if output, failed := invoke(`{"args":["missing"]}`); !failed || !strings.Contains(output, "unknown setting") {
		t.Fatalf("settings error = %q, failed = %v", output, failed)
	}
	postToDashboard(t, client, origin, "/api/terminal/settings", "https://other.invalid", `{"args":[]}`, http.StatusForbidden)
	postToDashboard(t, client, origin, "/api/terminal/settings", origin, `{"command":"./run settings"}`, http.StatusBadRequest)
}
