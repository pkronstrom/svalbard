package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebSearchFollowsDriveOptInAcrossHarnesses(t *testing.T) {
	paths := map[string][2]string{
		"opencode": {"config/opencode.json", "config/opencode/AGENTS.md"},
		"goose":    {"config/goose/config.yaml", "config/goose/.goosehints"},
		"pi":       {"config/mcp.json", "config/AGENTS.md"},
		"omp":      {"config/mcp.json", "config/lean-prompt.md"},
	}
	for client, files := range paths {
		t.Run(client, func(t *testing.T) {
			root := t.TempDir()
			state := filepath.Join(root, ".svalbard")
			if err := os.MkdirAll(state, 0o755); err != nil {
				t.Fatal(err)
			}
			for _, test := range []struct {
				value string
				on    bool
			}{{`{"ai":{"web_search":true}}`, true}, {`{"ai":{"web_search":false}}`, false}, {`{"ai":{"web_search":"true"}}`, false}} {
				if err := os.WriteFile(filepath.Join(state, "settings.json"), []byte(test.value), 0o600); err != nil {
					t.Fatal(err)
				}
				cfg, err := PrepareClientLaunchConfig(root, client, "http://localhost", "http://localhost/v1", "local", 32768)
				if err != nil {
					t.Fatal(err)
				}
				for i, marker := range []string{"--web-search", "web_search"} {
					data, err := os.ReadFile(filepath.Join(state, "runtime", client, files[i]))
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(string(data), marker) != test.on {
						t.Errorf("%s in %s, want enabled=%v (settings %s)", marker, files[i], test.on, test.value)
					}
				}
				if client == "omp" && strings.Contains(strings.Join(cfg.Args, " "), OmpToolPrefix+"web_search") != test.on {
					t.Errorf("wrong oh-my-pi allowlist for %s", test.value)
				}
			}
		})
	}
}
