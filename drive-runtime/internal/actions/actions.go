package actions

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type Mode int

const (
	ModeExecProcess Mode = iota
	ModeCaptureOutput
)

type ResolvedAction struct {
	Mode Mode
	Cmd  *exec.Cmd
}

type Runner struct {
	driveRoot string
}

func NewRunner(driveRoot string) Runner {
	return Runner{driveRoot: driveRoot}
}

func (r Runner) Resolve(actionID string, args map[string]string) (ResolvedAction, error) {
	script, argv, err := r.scriptFor(actionID, args)
	if err != nil {
		return ResolvedAction{}, err
	}

	cmd := exec.Command("bash", append([]string{script}, argv...)...)
	cmd.Dir = r.driveRoot
	cmd.Env = append(os.Environ(), "DRIVE_ROOT="+r.driveRoot)
	if actionID == "inspect" {
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		return ResolvedAction{
			Mode: ModeCaptureOutput,
			Cmd:  cmd,
		}, nil
	}

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return ResolvedAction{
		Mode: ModeExecProcess,
		Cmd:  cmd,
	}, nil
}

func (r Runner) scriptFor(actionID string, args map[string]string) (string, []string, error) {
	scripts := map[string]string{
		"browse":         ".svalbard/actions/browse.sh",
		"search":         ".svalbard/actions/search.sh",
		"maps":           ".svalbard/actions/maps.sh",
		"chat":           ".svalbard/actions/chat.sh",
		"agent":          ".svalbard/actions/agent.sh",
		"apps":           ".svalbard/actions/apps.sh",
		"share":          ".svalbard/actions/share.sh",
		"serve-all":      ".svalbard/actions/serve-all.sh",
		"verify":         ".svalbard/actions/verify.sh",
		"inspect":        ".svalbard/actions/inspect.sh",
		"embedded-shell": ".svalbard/actions/pio-setup.sh",
	}

	rel, ok := scripts[actionID]
	if !ok {
		return "", nil, fmt.Errorf("unknown action: %s", actionID)
	}

	var argv []string
	switch actionID {
	case "chat":
		if model := args["model"]; model != "" {
			argv = append(argv, model)
		}
	case "agent":
		if client := args["client"]; client != "" {
			argv = append(argv, client)
		}
	case "apps":
		if app := args["app"]; app != "" {
			argv = append(argv, app)
		}
	}

	return filepath.Join(r.driveRoot, rel), argv, nil
}
