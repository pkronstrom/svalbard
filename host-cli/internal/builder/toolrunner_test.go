package builder

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A cancelled tool command stops its container, not only the docker client.
func TestExecuteToolCommandKillsTheContainerOnCancel(t *testing.T) {
	dir := t.TempDir()
	record := filepath.Join(dir, "calls")
	fake := filepath.Join(dir, "docker")
	script := "#!/bin/sh\necho \"$@\" >> '" + record + "'\nif [ \"$1\" = run ]; then exec sleep 30; fi\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	previous := dockerClient
	dockerClient = fake
	t.Cleanup(func() { dockerClient = previous })

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(300*time.Millisecond, cancel)
	start := time.Now()
	if _, err := executeToolCommand(ctx, dir, dir, "image", "python3", nil); err == nil {
		t.Fatal("cancelled command succeeded")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("cancel took %v", elapsed)
	}
	data, _ := os.ReadFile(record)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "run --rm --name svalbard-tool-") || !strings.HasPrefix(lines[1], "kill svalbard-tool-") {
		t.Fatalf("docker calls = %q, want run then kill", lines)
	}
	name := strings.Fields(lines[0])[3]
	if lines[1] != "kill "+name {
		t.Errorf("killed %q, want %s", lines[1], name)
	}
}
