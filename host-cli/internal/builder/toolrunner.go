package builder

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type toolCommandFunc func(ctx context.Context, root, workdir, image, tool string, args []string) (string, error)

var runToolCommand toolCommandFunc = executeToolCommand

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
	dockerArgs := []string{"run", "--rm", "-v", root + ":/vault", "-v", workdir + ":/work", image, tool}
	dockerArgs = append(dockerArgs, translated...)
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s: %w\n%s", tool, err, tailOf(stderr.String(), 500))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func toolArg(path string) string { return filepath.Clean(path) }
