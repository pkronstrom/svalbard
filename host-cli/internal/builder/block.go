package builder

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func stepExecBlock(ctx context.Context, root, recipeWorkdir string, procedure Procedure) error {
	if len(procedure.Outputs) != 1 {
		return fmt.Errorf("tool block requires one output directory")
	}
	output := procedure.Outputs[0]
	blockWork := filepath.Join(recipeWorkdir, "blocks", procedure.ID)
	if err := os.MkdirAll(blockWork, 0o755); err != nil {
		return err
	}
	if _, err := os.Stat(output); err == nil {
		previous := filepath.Join(blockWork, "previous-output")
		_ = os.RemoveAll(previous)
		if err := os.Rename(output, previous); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		return err
	}

	image, err := toolsImage(procedure.Tool, procedure.Image)
	if err != nil {
		return err
	}
	dockerArgs, err := blockDockerArgs(root, blockWork, output, procedure.Inputs, procedure.Args)
	if err != nil {
		return err
	}
	dockerArgs = append(dockerArgs, image, procedure.Tool)
	dockerArgs = append(dockerArgs, translateBlockArgs(blockWork, output, procedure.Inputs, procedure.Args)...)
	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	var buffer bytes.Buffer
	cmd.Stdout = &buffer
	cmd.Stderr = &buffer
	if err := cmd.Run(); err != nil {
		slog.Warn("tool block failed", "tool", procedure.Tool, "image", image, "output", tailOf(buffer.String(), 2000))
		return fmt.Errorf("%w\n%s", err, tailOf(buffer.String(), 500))
	}
	return nil
}

func blockDockerArgs(root, workdir, output string, inputs, args []string) ([]string, error) {
	for _, arg := range args {
		if filepath.IsAbs(arg) && pathWithin(arg, root) && !pathWithin(arg, workdir) && !pathWithin(arg, output) && !withinAny(arg, inputs) {
			return nil, fmt.Errorf("tool block argument accesses undeclared vault path: %s", arg)
		}
	}
	dockerArgs := []string{"run", "--rm", "-v", workdir + ":/work", "-v", output + ":/output"}
	for index, input := range inputs {
		if _, err := os.Stat(input); err != nil {
			return nil, fmt.Errorf("block input %s: %w", input, err)
		}
		dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s:/input/%d:ro", input, index))
	}
	return dockerArgs, nil
}

func translateBlockArgs(workdir, output string, inputs, args []string) []string {
	translated := make([]string, len(args))
	for index, arg := range args {
		switch {
		case pathWithin(arg, workdir):
			translated[index] = translatePath(arg, workdir, "/work")
		case pathWithin(arg, output):
			translated[index] = translatePath(arg, output, "/output")
		default:
			translated[index] = arg
			for inputIndex, input := range inputs {
				if pathWithin(arg, input) {
					translated[index] = translatePath(arg, input, fmt.Sprintf("/input/%d", inputIndex))
					break
				}
			}
		}
	}
	return translated
}

func translatePath(path, root, containerRoot string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." {
		return containerRoot
	}
	return containerRoot + "/" + filepath.ToSlash(relative)
}

func pathWithin(path, root string) bool {
	if path == "" || root == "" || !filepath.IsAbs(path) {
		return false
	}
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func withinAny(path string, roots []string) bool {
	for _, root := range roots {
		if pathWithin(path, root) {
			return true
		}
	}
	return false
}
