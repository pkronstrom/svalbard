package main

import (
	"context"
	"database/sql"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/actions"

	_ "github.com/ncruces/go-sqlite3/driver"
)

func TestRunResolvedActionContextReapsChildProcessGroup(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")
	script := filepath.Join(dir, "runner")
	body := "#!/bin/sh\n(trap '' TERM HUP; while :; do sleep 1; done) &\necho $! > " + shellQuote(pidFile) + "\ntrap 'exit 0' TERM\nwhile :; do sleep 1; done\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	finished := false
	t.Cleanup(func() {
		cancel()
		if !finished {
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Error("cancellation fixture did not reap its process group")
			}
		}
	})
	go func() {
		done <- runResolvedActionContext(ctx, actions.ResolvedAction{Mode: actions.ModeExecProcess, Cmd: exec.Command(script)})
	}()

	var data []byte
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var err error
		data, err = os.ReadFile(pidFile)
		if err == nil && strings.TrimSpace(string(data)) != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		t.Fatal("action child did not report its PID before deadline")
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		finished = true
		if err != context.Canceled {
			t.Fatalf("runResolvedActionContext() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("action runner did not reap child process group")
	}
	deadline = time.Now().Add(time.Second)
	for syscall.Kill(pid, 0) == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if err := syscall.Kill(pid, 0); err != syscall.ESRCH {
		t.Fatalf("surviving child %d remains: %v", pid, err)
	}
}

// TestInteractiveActionReadsTerminal gives the runner a real controlling
// terminal: an interactive action must be able to read what is typed there,
// survive Ctrl+Z, leave nothing running, and hand the terminal back.
func TestInteractiveActionReadsTerminal(t *testing.T) {
	helper := exec.Command(os.Args[0], "-test.run=^TestInteractiveActionHelper$")
	// TERM=dumb keeps the terminal libraries from querying a terminal nobody answers.
	helper.Env = append(os.Environ(), "SVALBARD_TTY_HELPER=1", "TERM=dumb")
	terminal, err := pty.Start(helper)
	if err != nil {
		t.Fatal(err)
	}
	// A stopped action never closes the terminal, so the deadline ends the read.
	deadline := time.AfterFunc(10*time.Second, func() {
		_ = helper.Process.Kill()
		_ = terminal.Close()
	})
	t.Cleanup(func() {
		deadline.Stop()
		_ = helper.Process.Kill()
		_ = terminal.Close()
	})

	var seen strings.Builder
	typed := false
	buf := make([]byte, 256)
	for {
		n, err := terminal.Read(buf)
		seen.Write(buf[:n])
		if !typed && strings.Contains(seen.String(), "ready") {
			typed = true
			// Ctrl+Z first: the action must still answer the line after it.
			if _, err := terminal.WriteString("\x1ahello\n"); err != nil {
				t.Fatal(err)
			}
		}
		if err != nil {
			break
		}
	}
	if err := helper.Wait(); err != nil {
		t.Fatalf("helper: %v; terminal output %q", err, seen.String())
	}
	for _, want := range []string{"got:hello", "leftover-reaped:true", "foreground-restored:true"} {
		if !strings.Contains(seen.String(), want) {
			t.Fatalf("terminal output %q lacks %q", seen.String(), want)
		}
	}
}

// TestInteractiveActionHelper is the process TestInteractiveActionReadsTerminal
// runs under a pty; on its own it does nothing.
func TestInteractiveActionHelper(t *testing.T) {
	if os.Getenv("SVALBARD_TTY_HELPER") != "1" {
		return
	}
	pidFile := filepath.Join(os.TempDir(), "svalbard-tty-helper-"+strconv.Itoa(os.Getpid()))
	cmd := exec.Command("sh", "-c", `sleep 300 & echo $! > "$0"; echo ready; read line; echo "got:$line"`, pidFile)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	err := runResolvedActionContext(context.Background(), actions.ResolvedAction{Mode: actions.ModeExecProcess, Cmd: cmd})
	data, _ := os.ReadFile(pidFile)
	leftover, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	// The swept process lingers as a zombie until its new parent collects it.
	for deadline := time.Now().Add(time.Second); syscall.Kill(leftover, 0) == nil && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	println("leftover-reaped:" + strconv.FormatBool(leftover > 0 && syscall.Kill(leftover, 0) == syscall.ESRCH))
	foreground, _ := unix.IoctlGetInt(int(os.Stdin.Fd()), unix.TIOCGPGRP)
	println("foreground-restored:" + strconv.FormatBool(foreground == syscall.Getpgrp()))
	_ = os.Remove(pidFile)
	if err != nil {
		println("action error: " + err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}

func TestNativeSearchSubcommand(t *testing.T) {
	root := newNativeSearchDrive(t)
	t.Setenv("DRIVE_ROOT", root)

	stdin, writeStdin, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writeStdin.WriteString("q\n"); err != nil {
		t.Fatal(err)
	}
	if err := writeStdin.Close(); err != nil {
		t.Fatal(err)
	}
	stdout, readStdout, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	oldArgs, oldStdin, oldStdout := os.Args, os.Stdin, os.Stdout
	os.Args = []string{"svalbard-drive", "__native-search", "water"}
	os.Stdin = stdin
	os.Stdout = readStdout
	t.Cleanup(func() {
		os.Args = oldArgs
		os.Stdin = oldStdin
		os.Stdout = oldStdout
		_ = stdin.Close()
		_ = readStdout.Close()
	})

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if err := readStdout.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(output), "Searching (keyword): water") || !strings.Contains(string(output), "Water purification") {
		t.Fatalf("native search output = %q", output)
	}
}

func newNativeSearchDrive(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".svalbard"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".svalbard", "actions.json"), []byte(`{"version":1,"preset":"test","groups":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite3", filepath.Join(root, "data", "search.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	const schema = `
		CREATE TABLE sources (id INTEGER PRIMARY KEY, filename TEXT NOT NULL);
		CREATE TABLE articles (id INTEGER PRIMARY KEY, source_id INTEGER NOT NULL, path TEXT NOT NULL, title TEXT NOT NULL, body TEXT NOT NULL DEFAULT '');
		CREATE VIRTUAL TABLE articles_fts USING fts5(title, body, content='articles', content_rowid='id');
		CREATE TABLE meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
	`
	if _, err := db.Exec(schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	source, err := db.Exec("INSERT INTO sources (filename) VALUES ('wiki.zim')")
	if err != nil {
		t.Fatal(err)
	}
	sourceID, err := source.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	article, err := db.Exec("INSERT INTO articles (source_id, path, title, body) VALUES (?, '/Water_purification', 'Water purification', 'Water is essential.')", sourceID)
	if err != nil {
		t.Fatal(err)
	}
	articleID, err := article.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO articles_fts (rowid, title, body) VALUES (?, 'Water purification', 'Water is essential.')", articleID); err != nil {
		t.Fatal(err)
	}
	return root
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// runSubcommand runs run() with the given arguments and returns what it wrote
// to stdout, which is a pipe here as it is under a script.
func runSubcommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return runAs(t, "svalbard-drive", args...)
}

// runAs is runSubcommand with the program run as argv0.
func runAs(t *testing.T, argv0 string, args ...string) (string, error) {
	t.Helper()
	stdout, writeStdout, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldArgs, oldStdout := os.Args, os.Stdout
	os.Args = append([]string{argv0}, args...)
	os.Stdout = writeStdout
	runErr := run()
	os.Args, os.Stdout = oldArgs, oldStdout
	if err := writeStdout.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	_ = stdout.Close()
	return string(output), runErr
}

func TestSettingsSubcommandNeedsNoActionsFile(t *testing.T) {
	// A drive with nothing generated on it: no actions.json, no schema file.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".svalbard"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DRIVE_ROOT", root)

	if out, err := runSubcommand(t, "settings", "ai.context", "balanced"); err != nil || out != "ai.context = balanced\n" {
		t.Fatalf("set: %q, %v", out, err)
	}
	if out, err := runSubcommand(t, "settings", "ai.context"); err != nil || out != "balanced\n" {
		t.Fatalf("get: %q, %v", out, err)
	}
	out, err := runSubcommand(t, "settings")
	if err != nil || !strings.Contains(out, "Local AI\n  ai.default_model    ask\n  ai.context        * balanced\n") {
		t.Fatalf("list: %q, %v", out, err)
	}
	if strings.ContainsAny(out, "\x1b\r") {
		t.Errorf("list holds control sequences: %q", out)
	}
	saved, err := os.ReadFile(filepath.Join(root, ".svalbard", "settings.json"))
	if err != nil || !strings.Contains(string(saved), `"context": "balanced"`) {
		t.Fatalf("settings.json = %q, %v", saved, err)
	}

	if out, err := runSubcommand(t, "settings", "ai.context", "huge"); err == nil || out != "" ||
		!strings.Contains(err.Error(), "is not one of ask, auto, fast, 16k, balanced, 64k, max, 256k, 512k, model-max") {
		t.Fatalf("bad value: %q, %v", out, err)
	}
	// Everything else still needs the generated menu.
	if _, err := runSubcommand(t, "inspect"); err == nil {
		t.Fatal("a command other than settings ran without actions.json")
	}
}

// The launchers pass the model's window to the MCP server; a value that is not
// a number stops it before it serves anything.
func TestMCPSubcommandRefusesAContextThatIsNotANumber(t *testing.T) {
	if _, err := runSubcommand(t, "mcp", "--drive", t.TempDir(), "--context", "fast"); err == nil || !strings.Contains(err.Error(), "--context needs a number") {
		t.Fatalf("err = %v, want the bad --context named", err)
	}
}
