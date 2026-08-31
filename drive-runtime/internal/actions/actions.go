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
	NativeActivateSubcommand = "__native-activate-shell"
	NativeMCPSubcommand      = "__native-mcp-serve"
)

// NativeInvocation is the stable built-in action ID and its named arguments.
// Its positional representation is private to this package.
type NativeInvocation struct {
	ActionID string
	Args     map[string]string
}

type nativeArgument struct {
	name         string
	required     bool
	defaultValue string
}

type nativeActionSpec struct {
	actionID   string
	subcommand string
	args       []nativeArgument
}

var nativeActionSpecs = []nativeActionSpec{
	{actionID: "inspect", subcommand: NativeInspectSubcommand},
	{actionID: "verify", subcommand: NativeVerifySubcommand},
	{actionID: "share", subcommand: NativeShareSubcommand},
	{actionID: "browse", subcommand: NativeBrowseSubcommand, args: []nativeArgument{{name: "zim"}}},
	{actionID: "apps", subcommand: NativeAppsSubcommand, args: []nativeArgument{{name: "app", required: true}}},
	{actionID: "maps", subcommand: NativeMapsSubcommand},
	{actionID: "chat", subcommand: NativeChatSubcommand, args: []nativeArgument{{name: "model"}}},
	{actionID: "agent", subcommand: NativeAgentSubcommand, args: []nativeArgument{{name: "client", required: true}, {name: "model"}}},
	{actionID: "serve-all", subcommand: NativeServeAllSubcommand, args: []nativeArgument{{name: "bind", defaultValue: "127.0.0.1"}}},
	{actionID: "search", subcommand: NativeSearchSubcommand, args: []nativeArgument{{name: "query"}}},
	{actionID: "embedded-shell", subcommand: NativeEmbeddedSubcommand},
	{actionID: "activate-shell", subcommand: NativeActivateSubcommand},
	{actionID: "mcp-serve", subcommand: NativeMCPSubcommand},
}

// EncodeNativeInvocation converts a stable action ID and named arguments into
// the hidden native subcommand argv.
func EncodeNativeInvocation(invocation NativeInvocation) ([]string, error) {
	spec, ok := nativeActionByID(invocation.ActionID)
	if !ok {
		return nil, fmt.Errorf("unknown action: %s", invocation.ActionID)
	}
	for key := range invocation.Args {
		if !spec.hasArgument(key) {
			return nil, fmt.Errorf("unknown argument %q for action %q", key, invocation.ActionID)
		}
	}

	argv := make([]string, 1, len(spec.args)+1)
	argv[0] = spec.subcommand
	for _, argument := range spec.args {
		value := invocation.Args[argument.name]
		if value == "" {
			if argument.required {
				return nil, fmt.Errorf("%s required", argument.name)
			}
			continue
		}
		argv = append(argv, value)
	}
	return argv, nil
}

// DecodeNativeInvocation parses hidden native subcommand argv. known is false
// when argv does not name a native subcommand, allowing normal action aliases
// to continue through their existing dispatch path.
func DecodeNativeInvocation(argv []string) (invocation NativeInvocation, known bool, err error) {
	if len(argv) == 0 {
		return NativeInvocation{}, false, nil
	}
	spec, known := nativeActionBySubcommand(argv[0])
	if !known {
		if strings.HasPrefix(argv[0], "__native-") {
			return NativeInvocation{}, true, fmt.Errorf("unknown native subcommand: %s", argv[0])
		}
		return NativeInvocation{}, false, nil
	}
	if len(argv)-1 > len(spec.args) {
		return NativeInvocation{}, true, fmt.Errorf("too many arguments for %s", spec.subcommand)
	}

	args := make(map[string]string, len(spec.args))
	for index, argument := range spec.args {
		if index < len(argv)-1 {
			value := argv[index+1]
			if value == "" && argument.required {
				return NativeInvocation{}, true, fmt.Errorf("%s required", argument.name)
			}
			if value != "" {
				args[argument.name] = value
			}
			continue
		}
		if argument.required {
			return NativeInvocation{}, true, fmt.Errorf("%s required", argument.name)
		}
		if argument.defaultValue != "" {
			args[argument.name] = argument.defaultValue
		}
	}
	return NativeInvocation{ActionID: spec.actionID, Args: args}, true, nil
}

func nativeActionByID(actionID string) (nativeActionSpec, bool) {
	for _, spec := range nativeActionSpecs {
		if spec.actionID == actionID {
			return spec, true
		}
	}
	return nativeActionSpec{}, false
}

func nativeActionBySubcommand(subcommand string) (nativeActionSpec, bool) {
	for _, spec := range nativeActionSpecs {
		if spec.subcommand == subcommand {
			return spec, true
		}
	}
	return nativeActionSpec{}, false
}

func (spec nativeActionSpec) hasArgument(name string) bool {
	for _, argument := range spec.args {
		if argument.name == name {
			return true
		}
	}
	return false
}

type ResolvedAction struct {
	Mode Mode
	Cmd  *exec.Cmd
}

type Runner struct {
	driveRoot string
	workDir   string
}

// NewRunner builds a Runner rooted at driveRoot. An optional workDir overrides
// the working directory for exec actions; empty or omitted defaults to driveRoot.
func NewRunner(driveRoot string, workDir ...string) Runner {
	wd := driveRoot
	if len(workDir) > 0 && workDir[0] != "" {
		wd = workDir[0]
	}
	return Runner{driveRoot: driveRoot, workDir: wd}
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

func (r Runner) resolveBuiltinAction(actionID string, args map[string]string) (ResolvedAction, error) {
	return r.resolveNativeAction(NativeInvocation{ActionID: actionID, Args: args})
}

func shouldCaptureNativeAction(actionID string) bool {
	switch actionID {
	case "inspect", "verify":
		return true
	default:
		return false
	}
}

func (r Runner) resolveNativeAction(invocation NativeInvocation) (ResolvedAction, error) {
	argv, err := EncodeNativeInvocation(invocation)
	if err != nil {
		return ResolvedAction{}, err
	}
	bin, err := os.Executable()
	if err != nil {
		return ResolvedAction{}, err
	}

	cmd := exec.Command(bin, argv...)
	cmd.Dir = r.workDir
	cmd.Env = append(os.Environ(), "DRIVE_ROOT="+r.driveRoot)
	if shouldCaptureNativeAction(invocation.ActionID) {
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
	cmd.Dir = r.workDir
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
