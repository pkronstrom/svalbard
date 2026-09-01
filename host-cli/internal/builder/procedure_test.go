package builder

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestCompileProceduresPreservesLinearActions(t *testing.T) {
	vars := map[string]string{"workdir": "/tmp/work", "output": "/vault/out.zim"}
	got, err := CompileProcedures([]catalog.BuildStep{
		{Download: "https://example.test/source", Dest: "{workdir}/source"},
		{Tool: "zimwriterfs", Args: []string{"{workdir}/site", "{output}"}, Inputs: []string{"{workdir}/site"}, Outputs: []string{"{workdir}/packed"}},
		{Verify: "{output}", MinSize: 1},
	}, vars)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != "001-download" || got[1].ID != "002-tool" || got[2].ID != "003-verify" {
		t.Fatalf("procedure IDs = %q, %q, %q", got[0].ID, got[1].ID, got[2].ID)
	}
	if got[0].Destination != "/tmp/work/source" {
		t.Fatalf("download destination = %q", got[0].Destination)
	}
	if want := []string{"/tmp/work/site", "/vault/out.zim"}; !reflect.DeepEqual(got[1].Args, want) {
		t.Fatalf("tool args = %v, want %v", got[1].Args, want)
	}
	if got[1].Inputs[0] != "/tmp/work/site" || got[1].Outputs[0] != "/tmp/work/packed" {
		t.Fatalf("tool edges = inputs:%v outputs:%v", got[1].Inputs, got[1].Outputs)
	}
	if got[0].Fingerprint == "" || got[1].Fingerprint == "" {
		t.Fatal("procedure fingerprint is empty")
	}
}

func TestCompileProceduresRejectsAmbiguousStep(t *testing.T) {
	_, err := CompileProcedures([]catalog.BuildStep{{Download: "https://example.test", Verify: "out"}}, nil)
	if err == nil {
		t.Fatal("expected ambiguous step error")
	}
}

func TestRunLinearReusesVerifiedProcedure(t *testing.T) {
	root := t.TempDir()
	workdir := filepath.Join(root, ".staging", "build", "test")
	output := filepath.Join(root, "output.zim")
	if err := os.WriteFile(output, []byte("zim"), 0o644); err != nil {
		t.Fatal(err)
	}
	procedures, err := CompileProcedures([]catalog.BuildStep{{Verify: output, MinSize: 1}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var first, second []EventState
	if err := RunLinear(context.Background(), root, workdir, "test", procedures, Options{OnEvent: func(event BuildEvent) {
		first = append(first, event.State)
	}}); err != nil {
		t.Fatal(err)
	}
	if err := RunLinear(context.Background(), root, workdir, "test", procedures, Options{OnEvent: func(event BuildEvent) {
		second = append(second, event.State)
	}}); err != nil {
		t.Fatal(err)
	}
	if want := []EventState{EventQueued, EventStarted, EventCompleted}; !reflect.DeepEqual(first, want) {
		t.Fatalf("first states = %v, want %v", first, want)
	}
	if want := []EventState{EventQueued, EventSkipped}; !reflect.DeepEqual(second, want) {
		t.Fatalf("second states = %v, want %v", second, want)
	}
}

func TestToolsImageSelectsPinnedTargets(t *testing.T) {
	if got, err := toolsImage("zimwriterfs", ""); err != nil || got != BaseToolsImage {
		t.Fatalf("base image = %q, err = %v", got, err)
	}
	if got, err := toolsImage("zimit", ""); err != nil || got != BrowserToolsImage {
		t.Fatalf("browser image = %q, err = %v", got, err)
	}
	if _, err := toolsImage("tool", "ghcr.io/example/arbitrary:latest"); err == nil {
		t.Fatal("arbitrary image was accepted")
	}
}
