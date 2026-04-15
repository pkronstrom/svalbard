package actions_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/actions"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

func TestRegistryRejectsUnknownAction(t *testing.T) {
	runner := actions.NewRunner("/tmp/drive")

	_, err := runner.Resolve(config.BuiltinAction("unknown", nil))
	if err == nil {
		t.Fatal("Resolve() error = nil, want unknown action error")
	}
}

func TestCommandUsesDriveRootAndExportsDriveRootEnv(t *testing.T) {
	runner := actions.NewRunner("/tmp/drive")

	resolved, err := runner.Resolve(config.BuiltinAction("browse", nil))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	cmd := resolved.Cmd

	if got, want := cmd.Dir, "/tmp/drive"; got != want {
		t.Fatalf("cmd.Dir = %q, want %q", got, want)
	}

	found := false
	for _, env := range cmd.Env {
		if env == "DRIVE_ROOT=/tmp/drive" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("cmd.Env = %v, want DRIVE_ROOT export", cmd.Env)
	}

	if cmd.Stdin != os.Stdin {
		t.Fatal("cmd.Stdin is not inherited from the current process")
	}
	if cmd.Stdout != os.Stdout {
		t.Fatal("cmd.Stdout is not inherited from the current process")
	}
	if cmd.Stderr != os.Stderr {
		t.Fatal("cmd.Stderr is not inherited from the current process")
	}
}

func TestInspectUsesCapturedOutputMode(t *testing.T) {
	runner := actions.NewRunner("/tmp/drive")

	resolved, err := runner.Resolve(config.BuiltinAction("inspect", nil))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got, want := resolved.Mode, actions.ModeCaptureOutput; got != want {
		t.Fatalf("resolved.Mode = %v, want %v", got, want)
	}
}

func TestNativeActionsResolveToLauncherSubcommands(t *testing.T) {
	runner := actions.NewRunner("/tmp/drive")

	verifyAction, err := runner.Resolve(config.BuiltinAction("verify", nil))
	if err != nil {
		t.Fatalf("Resolve(verify) error = %v", err)
	}
	if got, want := verifyAction.Mode, actions.ModeCaptureOutput; got != want {
		t.Fatalf("verify Mode = %v, want %v", got, want)
	}
	if len(verifyAction.Cmd.Args) < 2 || !strings.Contains(verifyAction.Cmd.Args[1], "native-verify") {
		t.Fatalf("verify args = %v, want native verify subcommand", verifyAction.Cmd.Args)
	}

	shareAction, err := runner.Resolve(config.BuiltinAction("share", nil))
	if err != nil {
		t.Fatalf("Resolve(share) error = %v", err)
	}
	if got, want := shareAction.Mode, actions.ModeExecProcess; got != want {
		t.Fatalf("share Mode = %v, want %v", got, want)
	}
	if len(shareAction.Cmd.Args) < 2 || !strings.Contains(shareAction.Cmd.Args[1], "native-share") {
		t.Fatalf("share args = %v, want native share subcommand", shareAction.Cmd.Args)
	}

	appsAction, err := runner.Resolve(config.BuiltinAction("apps", map[string]string{"app": "sqliteviz"}))
	if err != nil {
		t.Fatalf("Resolve(apps) error = %v", err)
	}
	if got, want := appsAction.Mode, actions.ModeExecProcess; got != want {
		t.Fatalf("apps Mode = %v, want %v", got, want)
	}
	if len(appsAction.Cmd.Args) < 3 || !strings.Contains(appsAction.Cmd.Args[1], "native-apps") || appsAction.Cmd.Args[2] != "sqliteviz" {
		t.Fatalf("apps args = %v, want native apps subcommand with app arg", appsAction.Cmd.Args)
	}

	mapsAction, err := runner.Resolve(config.BuiltinAction("maps", nil))
	if err != nil {
		t.Fatalf("Resolve(maps) error = %v", err)
	}
	if got, want := mapsAction.Mode, actions.ModeExecProcess; got != want {
		t.Fatalf("maps Mode = %v, want %v", got, want)
	}
	if len(mapsAction.Cmd.Args) < 2 || !strings.Contains(mapsAction.Cmd.Args[1], "native-maps") {
		t.Fatalf("maps args = %v, want native maps subcommand", mapsAction.Cmd.Args)
	}

	chatAction, err := runner.Resolve(config.BuiltinAction("chat", map[string]string{"model": "/tmp/drive/models/gemma.gguf"}))
	if err != nil {
		t.Fatalf("Resolve(chat) error = %v", err)
	}
	if got, want := chatAction.Mode, actions.ModeExecProcess; got != want {
		t.Fatalf("chat Mode = %v, want %v", got, want)
	}
	if len(chatAction.Cmd.Args) < 3 || !strings.Contains(chatAction.Cmd.Args[1], "native-chat") || chatAction.Cmd.Args[2] != "/tmp/drive/models/gemma.gguf" {
		t.Fatalf("chat args = %v, want native chat subcommand with model arg", chatAction.Cmd.Args)
	}

	agentAction, err := runner.Resolve(config.BuiltinAction("agent", map[string]string{"client": "opencode", "model": "/tmp/drive/models/gemma.gguf"}))
	if err != nil {
		t.Fatalf("Resolve(agent) error = %v", err)
	}
	if got, want := agentAction.Mode, actions.ModeExecProcess; got != want {
		t.Fatalf("agent Mode = %v, want %v", got, want)
	}
	if len(agentAction.Cmd.Args) < 4 || !strings.Contains(agentAction.Cmd.Args[1], "native-agent") || agentAction.Cmd.Args[2] != "opencode" || agentAction.Cmd.Args[3] != "/tmp/drive/models/gemma.gguf" {
		t.Fatalf("agent args = %v, want native agent subcommand with client and model args", agentAction.Cmd.Args)
	}

	serveAllAction, err := runner.Resolve(config.BuiltinAction("serve-all", nil))
	if err != nil {
		t.Fatalf("Resolve(serve-all) error = %v", err)
	}
	if got, want := serveAllAction.Mode, actions.ModeExecProcess; got != want {
		t.Fatalf("serve-all Mode = %v, want %v", got, want)
	}
	if len(serveAllAction.Cmd.Args) < 2 || !strings.Contains(serveAllAction.Cmd.Args[1], "native-serve-all") {
		t.Fatalf("serve-all args = %v, want native serve-all subcommand", serveAllAction.Cmd.Args)
	}

	searchAction, err := runner.Resolve(config.BuiltinAction("search", nil))
	if err != nil {
		t.Fatalf("Resolve(search) error = %v", err)
	}
	if got, want := searchAction.Mode, actions.ModeExecProcess; got != want {
		t.Fatalf("search Mode = %v, want %v", got, want)
	}
	if len(searchAction.Cmd.Args) < 2 || !strings.Contains(searchAction.Cmd.Args[1], "native-search") {
		t.Fatalf("search args = %v, want native search subcommand", searchAction.Cmd.Args)
	}

	embeddedAction, err := runner.Resolve(config.BuiltinAction("embedded-shell", nil))
	if err != nil {
		t.Fatalf("Resolve(embedded-shell) error = %v", err)
	}
	if got, want := embeddedAction.Mode, actions.ModeExecProcess; got != want {
		t.Fatalf("embedded-shell Mode = %v, want %v", got, want)
	}
	if len(embeddedAction.Cmd.Args) < 2 || !strings.Contains(embeddedAction.Cmd.Args[1], "native-embedded-shell") {
		t.Fatalf("embedded-shell args = %v, want native embedded-shell subcommand", embeddedAction.Cmd.Args)
	}
}

func TestBrowsePassesSelectedArchiveFilename(t *testing.T) {
	runner := actions.NewRunner("/tmp/drive")

	resolved, err := runner.Resolve(config.BuiltinAction("browse", map[string]string{"zim": "wikipedia-en-nopic.zim"}))
	if err != nil {
		t.Fatalf("Resolve(browse) error = %v", err)
	}
	if got, want := resolved.Mode, actions.ModeExecProcess; got != want {
		t.Fatalf("browse Mode = %v, want %v", got, want)
	}
	if len(resolved.Cmd.Args) < 3 || !strings.Contains(resolved.Cmd.Args[1], "native-browse") {
		t.Fatalf("browse args = %v, want native browse subcommand", resolved.Cmd.Args)
	}
	if got, want := resolved.Cmd.Args[len(resolved.Cmd.Args)-1], "wikipedia-en-nopic.zim"; got != want {
		t.Fatalf("browse argv last arg = %q, want %q", got, want)
	}
}

func TestExecActionUsesResolvedDriveBinaryAndPlaceholders(t *testing.T) {
	driveRoot := t.TempDir()
	platformName, err := platform.Detect()
	if err != nil {
		t.Fatalf("Detect() error = %v", err)
	}
	binDir := filepath.Join(driveRoot, "bin", platformName)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	binPath := filepath.Join(binDir, "sqlite3")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runner := actions.NewRunner(driveRoot)

	resolved, err := runner.Resolve(config.ExecAction(config.ExecActionConfig{
		Executable:  "sqlite3",
		ResolveFrom: "drive-bin",
		Args:        []string{"{drive_root}/data/search.db", "{platform}"},
		Cwd:         "{drive_root}",
		Mode:        "interactive",
		Env: map[string]string{
			"DATA_DIR": "{drive_root}/data",
		},
	}))
	if err != nil {
		t.Fatalf("Resolve(exec) error = %v", err)
	}

	if got, want := resolved.Mode, actions.ModeExecProcess; got != want {
		t.Fatalf("resolved.Mode = %v, want %v", got, want)
	}
	if got, want := resolved.Cmd.Path, binPath; got != want {
		t.Fatalf("cmd.Path = %q, want %q", got, want)
	}
	if got, want := resolved.Cmd.Dir, driveRoot; got != want {
		t.Fatalf("cmd.Dir = %q, want %q", got, want)
	}
	if got, want := resolved.Cmd.Args[1], filepath.Join(driveRoot, "data", "search.db"); got != want {
		t.Fatalf("cmd.Args[1] = %q, want %q", got, want)
	}
	if got, want := resolved.Cmd.Args[2], platformName; got != want {
		t.Fatalf("cmd.Args[2] = %q, want %q", got, want)
	}

	found := false
	for _, env := range resolved.Cmd.Env {
		if env == "DATA_DIR="+filepath.Join(driveRoot, "data") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("cmd.Env = %v, want expanded DATA_DIR", resolved.Cmd.Env)
	}
}

func TestExecActionCaptureModeBuffersOutput(t *testing.T) {
	runner := actions.NewRunner("/tmp/drive")

	resolved, err := runner.Resolve(config.ExecAction(config.ExecActionConfig{
		Executable:  "/usr/bin/env",
		ResolveFrom: "path",
		Mode:        "capture",
	}))
	if err != nil {
		t.Fatalf("Resolve(exec capture) error = %v", err)
	}
	if got, want := resolved.Mode, actions.ModeCaptureOutput; got != want {
		t.Fatalf("resolved.Mode = %v, want %v", got, want)
	}
	if resolved.Cmd.Stdout == os.Stdout {
		t.Fatal("capture mode stdout unexpectedly inherited")
	}
	if resolved.Cmd.Stderr == os.Stderr {
		t.Fatal("capture mode stderr unexpectedly inherited")
	}
}
