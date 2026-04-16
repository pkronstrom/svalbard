package actions

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

type Mode int

const (
	ModeExecProcess Mode = iota
	ModeCaptureOutput
)

const (
	NativeInspectSubcommand  = "__native-inspect"
	NativeVerifySubcommand   = "__native-verify"
	NativeShareSubcommand    = "__native-share"
	NativeBrowseSubcommand   = "__native-browse"
	NativeAppsSubcommand     = "__native-apps"
	NativeMapsSubcommand     = "__native-maps"
	NativeChatSubcommand     = "__native-chat"
	NativeAgentSubcommand    = "__native-agent"
	NativeServeAllSubcommand = "__native-serve-all"
	NativeSearchSubcommand   = "__native-search"
	NativeEmbeddedSubcommand = "__native-embedded-shell"
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

func (r Runner) Resolve(action config.ActionSpec) (ResolvedAction, error) {
	switch action.Type {
	case "", "builtin":
		builtin, err := action.DecodeBuiltin()
		if err != nil {
			return ResolvedAction{}, err
		}
		return r.resolveBuiltinAction(builtin.Name, builtin.Args)
	case "exec":
		execCfg, err := action.DecodeExec()
		if err != nil {
			return ResolvedAction{}, err
		}
		return r.resolveExecAction(execCfg)
	default:
		return ResolvedAction{}, fmt.Errorf("unknown action type: %s", action.Type)
	}
}

func nativeSubcommandForAction(actionID string) (string, bool) {
	switch actionID {
	case "inspect":
		return NativeInspectSubcommand, true
	case "verify":
		return NativeVerifySubcommand, true
	case "share":
		return NativeShareSubcommand, true
	case "browse":
		return NativeBrowseSubcommand, true
	case "apps":
		return NativeAppsSubcommand, true
	case "maps":
		return NativeMapsSubcommand, true
	case "chat":
		return NativeChatSubcommand, true
	case "agent":
		return NativeAgentSubcommand, true
	case "serve-all":
		return NativeServeAllSubcommand, true
	case "search":
		return NativeSearchSubcommand, true
	case "embedded-shell":
		return NativeEmbeddedSubcommand, true
	default:
		return "", false
	}
}

func (r Runner) resolveBuiltinAction(actionID string, args map[string]string) (ResolvedAction, error) {
	subcommand, ok := nativeSubcommandForAction(actionID)
	if !ok {
		return ResolvedAction{}, fmt.Errorf("unknown action: %s", actionID)
	}
	return r.resolveNativeAction(subcommand, actionID, args)
}

func shouldCaptureNativeAction(actionID string) bool {
	switch actionID {
	case "inspect", "verify":
		return true
	default:
		return false
	}
}

func (r Runner) resolveNativeAction(subcommand, actionID string, args map[string]string) (ResolvedAction, error) {
	bin, err := os.Executable()
	if err != nil {
		return ResolvedAction{}, err
	}

	argv := []string{subcommand}
	switch actionID {
	case "browse":
		if zim := args["zim"]; zim != "" {
			argv = append(argv, zim)
		}
	case "apps":
		if app := args["app"]; app != "" {
			argv = append(argv, app)
		}
	case "chat":
		if model := args["model"]; model != "" {
			argv = append(argv, model)
		}
	case "agent":
		if client := args["client"]; client != "" {
			argv = append(argv, client)
		}
		if model := args["model"]; model != "" {
			argv = append(argv, model)
		}
	case "search":
		if query := args["query"]; query != "" {
			argv = append(argv, query)
		}
	}

	cmd := exec.Command(bin, argv...)
	cmd.Dir = r.driveRoot
	cmd.Env = append(os.Environ(), "DRIVE_ROOT="+r.driveRoot)
	if shouldCaptureNativeAction(actionID) {
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

func (r Runner) resolveExecAction(cfg config.ExecActionConfig) (ResolvedAction, error) {
	platformName, err := platform.Detect()
	if err != nil {
		return ResolvedAction{}, err
	}

	executable := expandPlaceholders(cfg.Executable, r.driveRoot, platformName)
	cmdPath, err := resolveExecutable(executable, cfg.ResolveFrom, r.driveRoot, platformName)
	if err != nil {
		return ResolvedAction{}, err
	}

	argv := make([]string, 0, len(cfg.Args)+1)
	argv = append(argv, cmdPath)
	for _, arg := range cfg.Args {
		argv = append(argv, expandPlaceholders(arg, r.driveRoot, platformName))
	}

	cmd := exec.Command(cmdPath, argv[1:]...)
	cmd.Args = argv
	cmd.Dir = r.driveRoot
	if cfg.Cwd != "" {
		cmd.Dir = expandPlaceholders(cfg.Cwd, r.driveRoot, platformName)
	}
	cmd.Env = append(os.Environ(), "DRIVE_ROOT="+r.driveRoot)
	for key, value := range cfg.Env {
		cmd.Env = append(cmd.Env, key+"="+expandPlaceholders(value, r.driveRoot, platformName))
	}

	switch cfg.Mode {
	case "capture":
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		return ResolvedAction{
			Mode: ModeCaptureOutput,
			Cmd:  cmd,
		}, nil
	case "", "interactive", "service":
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return ResolvedAction{
			Mode: ModeExecProcess,
			Cmd:  cmd,
		}, nil
	default:
		return ResolvedAction{}, fmt.Errorf("unknown exec mode: %s", cfg.Mode)
	}
}

func resolveExecutable(executable, resolveFrom, driveRoot, platformName string) (string, error) {
	switch resolveFrom {
	case "", "path":
		return resolveFromPath(executable)
	case "drive-bin":
		return binary.Resolve(executable, driveRoot, func() (string, error) {
			return platformName, nil
		})
	case "drive-bin-or-path":
		if path, err := binary.Resolve(executable, driveRoot, func() (string, error) {
			return platformName, nil
		}); err == nil {
			return path, nil
		}
		return resolveFromPath(executable)
	default:
		return "", fmt.Errorf("unknown resolve_from: %s", resolveFrom)
	}
}

func resolveFromPath(executable string) (string, error) {
	if executable == "" {
		return "", fmt.Errorf("executable is required")
	}
	if strings.Contains(executable, string(filepath.Separator)) || filepath.IsAbs(executable) {
		return executable, nil
	}
	return exec.LookPath(executable)
}

func expandPlaceholders(value, driveRoot, platformName string) string {
	replacer := strings.NewReplacer(
		"{drive_root}", driveRoot,
		"{platform}", platformName,
	)
	return replacer.Replace(value)
}
