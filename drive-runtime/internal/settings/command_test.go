package settings

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// runCommand runs `./run settings <args>` against root. Every byte either
// stream receives is checked for escape sequences: the writers are plain
// buffers, as a pipe or a file would be.
func runCommand(t *testing.T, root string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer
	err = Run(&out, &errOut, root, args)
	for name, text := range map[string]string{"stdout": out.String(), "stderr": errOut.String()} {
		if strings.ContainsAny(text, "\x1b\r\x07") {
			t.Errorf("%s holds control sequences: %q", name, text)
		}
	}
	return out.String(), errOut.String(), err
}

func TestCommandListsEverySettingGrouped(t *testing.T) {
	root := newDrive(t, testSchema)
	writeValues(t, root, `{"ai": {"context": "balanced"}, "open-webui": {"open_browser": false}, "radio-tool": {"token": "s3cret"}}`)

	stdout, stderr, err := runCommand(t, root)
	if err != nil {
		t.Fatal(err)
	}
	want := `Local AI
  ai.default_model           ask
  ai.context               * balanced

Open WebUI
  open-webui.open_browser  * off

Radio tool
  radio-tool.band            vhf
  radio-tool.gain            5
  radio-tool.station
  radio-tool.token         * set

* changed from the default
Change one with ./run settings <key> <value>, undo with ./run settings --reset <key>.
`
	if stdout != want {
		t.Errorf("list output:\n%s\nwant:\n%s", stdout, want)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want nothing", stderr)
	}
	if strings.Contains(stdout, "s3cret") {
		t.Error("the list spelled out a secret")
	}
}

func TestCommandListWithNothingChangedHasNoLegend(t *testing.T) {
	stdout, _, err := runCommand(t, newDrive(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(stdout, "*") {
		t.Errorf("list marks a change on a fresh drive:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Local AI\n  ai.default_model  ") {
		t.Errorf("list without a schema file lacks the built-ins:\n%s", stdout)
	}
}

func TestCommandGetPrintsTheBareValue(t *testing.T) {
	root := newDrive(t, testSchema)
	writeValues(t, root, `{"ai": {"context": "balanced"}, "open-webui": {"open_browser": false}, "radio-tool": {"token": "s3cret", "gain": 2.5}}`)
	for key, want := range map[string]string{
		KeyContext:                "balanced\n",
		KeyDefaultModel:           "ask\n",
		"open-webui.open_browser": "off\n",
		"radio-tool.gain":         "2.5\n",
		"radio-tool.station":      "\n",
		// The explicit single-key form is where a secret can be read back.
		"radio-tool.token": "s3cret\n",
	} {
		stdout, stderr, err := runCommand(t, root, key)
		if err != nil || stdout != want || stderr != "" {
			t.Errorf("settings %s = %q, stderr %q, error %v; want %q", key, stdout, stderr, err, want)
		}
	}
}

func TestCommandSetAndReset(t *testing.T) {
	root := newDrive(t, testSchema)

	stdout, _, err := runCommand(t, root, KeyContext, "balanced")
	if err != nil || stdout != "ai.context = balanced\n" {
		t.Fatalf("set: %q, %v", stdout, err)
	}
	stdout, _, err = runCommand(t, root, "open-webui.open_browser", "no")
	if err != nil || stdout != "open-webui.open_browser = off\n" {
		t.Fatalf("set bool: %q, %v", stdout, err)
	}
	stdout, _, err = runCommand(t, root, "radio-tool.token", "s3cret")
	if err != nil || stdout != "radio-tool.token = set\n" {
		t.Fatalf("set secret: %q, %v", stdout, err)
	}
	// A negative number is a value, not a flag.
	if _, _, err := runCommand(t, root, "radio-tool.gain", "-3"); err == nil || !strings.Contains(err.Error(), "-3 is outside 1 to 10") {
		t.Fatalf("set negative number: %v", err)
	}
	want := map[string]map[string]any{
		"ai": {"context": "balanced"}, "open-webui": {"open_browser": false}, "radio-tool": {"token": "s3cret"},
	}
	if got := readValues(t, root); !reflect.DeepEqual(got, want) {
		t.Fatalf("file = %v, want %v", got, want)
	}

	stdout, _, err = runCommand(t, root, "--reset", KeyContext)
	if err != nil || stdout != "ai.context = ask (default)\n" {
		t.Fatalf("reset: %q, %v", stdout, err)
	}
	if stdout, _, _ := runCommand(t, root, KeyContext); stdout != "ask\n" {
		t.Fatalf("after reset: %q", stdout)
	}
	if _, ok := readValues(t, root)["ai"]; ok {
		t.Error("reset left the owner in the file")
	}
}

func TestCommandErrorsAreOneLineAndNameWhatIsValid(t *testing.T) {
	root := newDrive(t, testSchema)
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"ai.contxt"}, `unknown setting "ai.contxt"; settings on this drive: ai.default_model, ai.context,`},
		{[]string{"ai.contxt", "fast"}, `unknown setting "ai.contxt"`},
		{[]string{"--reset", "nope.nope"}, `unknown setting "nope.nope"`},
		{[]string{"--reset"}, "--reset takes one key"},
		{[]string{KeyContext, "huge"}, `ai.context: "huge" is not one of ask, auto, fast, balanced, max`},
		{[]string{"open-webui.open_browser", "sometimes"}, "is not on or off"},
	} {
		stdout, _, err := runCommand(t, root, tc.args...)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("settings %v: error = %v, want it to contain %q", tc.args, err, tc.want)
			continue
		}
		if strings.Contains(err.Error(), "\n") {
			t.Errorf("settings %v: error is not one line: %q", tc.args, err)
		}
		if stdout != "" {
			t.Errorf("settings %v printed %q with an error", tc.args, stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".svalbard", valuesFile)); !os.IsNotExist(err) {
		t.Errorf("a failed command wrote %s", valuesFile)
	}
}

func TestCommandReportsIgnoredValuesAndRefusesAnUnparsableFile(t *testing.T) {
	root := newDrive(t, testSchema)
	writeValues(t, root, `{"ai": {"context": "huge"}}`)

	stdout, stderr, err := runCommand(t, root)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "ai.context                 ask\n") {
		t.Errorf("list does not show the default for the ignored value:\n%s", stdout)
	}
	if want := "warning: ai.context: saved value \"huge\" is not one of ask, auto, fast, balanced, max; using ask\n"; stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
	stdout, stderr, err = runCommand(t, root, KeyContext)
	if err != nil || stdout != "ask\n" || !strings.Contains(stderr, "warning: ai.context") {
		t.Errorf("get: %q, stderr %q, %v", stdout, stderr, err)
	}

	broken := `{"ai": {"context": "fast"`
	writeValues(t, root, broken)
	path := filepath.Join(root, ".svalbard", valuesFile)
	for _, args := range [][]string{nil, {KeyContext}, {KeyContext, "max"}, {"--reset", KeyContext}} {
		stdout, _, err := runCommand(t, root, args...)
		if err == nil || !strings.Contains(err.Error(), path) || stdout != "" {
			t.Errorf("settings %v on an unparsable file: output %q, error %v; want the path and nothing printed", args, stdout, err)
		}
	}
	if got, _ := os.ReadFile(path); string(got) != broken {
		t.Errorf("the unparsable file was replaced: %q", got)
	}
}

func TestCommandHelp(t *testing.T) {
	stdout, _, err := runCommand(t, newDrive(t, ""), "--help")
	if err != nil || !strings.Contains(stdout, "./run settings --reset <key>") {
		t.Errorf("help: %q, %v", stdout, err)
	}
}
