package builder

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type toolCommandFunc func(ctx context.Context, root, workdir, image, tool string, args []string) (string, error)

var runToolCommand toolCommandFunc = executeToolCommand

// dockerClient is the docker command; tests replace it.
var dockerClient = "docker"

func executeToolCommand(ctx context.Context, root, workdir, image, tool string, args []string) (string, error) {
	translated := make([]string, len(args))
	for index, arg := range args {
		switch {
		case pathWithin(arg, workdir):
			translated[index] = translatePath(arg, workdir, "/work")
		case pathWithin(arg, root):
			translated[index] = translatePath(arg, root, "/vault")
		default:
			translated[index] = arg
		}
	}
	suffix := make([]byte, 6)
	if _, err := rand.Read(suffix); err != nil {
		return "", err
	}
	name := "svalbard-tool-" + hex.EncodeToString(suffix)
	dockerArgs := []string{"run", "--rm", "--name", name, "-v", root + ":/vault", "-v", workdir + ":/work", image, tool}
	dockerArgs = append(dockerArgs, translated...)
	cmd := exec.CommandContext(ctx, dockerClient, dockerArgs...)
	// Cancelling kills the docker client, not the container, which would go
	// on writing to the vault after the build gave up on it.
	cmd.Cancel = func() error {
		_ = exec.Command(dockerClient, "kill", name).Run()
		return cmd.Process.Kill()
	}
	cmd.WaitDelay = 10 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s: %w\n%s", tool, err, tailOf(stderr.String(), 500))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func toolArg(path string) string { return filepath.Clean(path) }
