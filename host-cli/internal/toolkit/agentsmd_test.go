package toolkit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// driveContextGolden reads what the drive runtime writes for its harnesses.
// The two modules share no code, so the text exists twice; these files hold
// the copies together.
func driveContextGolden(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "drive-runtime", "internal", "agent", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestAgentsMDTextMatchesTheDriveContext(t *testing.T) {
	for name, areas := range map[string][2]bool{
		"drive_context.golden":           {true, true},
		"drive_context_skills.golden":    {true, false},
		"drive_context_libraries.golden": {false, true},
		"drive_context_plain.golden":     {false, false},
	} {
		want := driveContextGolden(t, name) + agentsMDPrefixNote
		if got := agentsMDText(areas[0], areas[1]); got != want {
			t.Errorf("agentsMDText(skills %v, libraries %v) differs from %s plus the prefix sentence; change agentsmd.go and drive-runtime/internal/agent/context.go together.\ngot:\n%s\nwant:\n%s", areas[0], areas[1], name, got, want)
		}
	}
	if !strings.Contains(agentsMDPrefixNote, "`svalbard`") {
		t.Errorf("prefix note = %q", agentsMDPrefixNote)
	}
}

func TestGenerateWritesRootAgentsMD(t *testing.T) {
	stubRuntimeBinarySources(t)
	skills := manifest.RealizedEntry{ID: "skills-core", Type: "skills", RelativePath: "skills/skills-core"}
	library := manifest.RealizedEntry{ID: "js-core", Type: "js-lib", RelativePath: "lib/js/js-core"}
	pi := manifest.RealizedEntry{ID: "pi", Type: "binary", Filename: "pi", RelativePath: "bin/macos-arm64/pi"}
	kiwix := manifest.RealizedEntry{ID: "kiwix-serve", Type: "binary", Filename: "kiwix-serve", RelativePath: "bin/macos-arm64/kiwix-serve"}
	wiki := manifest.RealizedEntry{ID: "wikem", Type: "zim", Filename: "wikem.zim", RelativePath: "zim/wikem.zim"}
	menus := map[string]catalog.MenuSpec{"pi": {Group: "local-ai", Subheader: "AI Clients", Label: "pi"}}
	generate := func(root string, entries ...manifest.RealizedEntry) {
		t.Helper()
		if err := Generate(root, entries, "test", GenerateOpts{Menus: menus}); err != nil {
			t.Fatal(err)
		}
	}
	read := func(root string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}

	root := t.TempDir()
	generate(root, wiki, skills)
	if got, want := read(root), driveContextGolden(t, "drive_context_skills.golden")+agentsMDPrefixNote; got != want {
		t.Fatalf("root AGENTS.md with a skills recipe:\n%s\nwant:\n%s", got, want)
	}
	// It names no place and no harness's tool prefix: any agent may read it.
	if text := read(root); strings.Contains(text, root) || strings.Contains(text, "mcp__") || !strings.Contains(text, "`skill_search`") {
		t.Errorf("root AGENTS.md:\n%s", text)
	}
	// The drive's root holds the file and nothing that tracks it.
	names, _ := filepath.Glob(filepath.Join(root, "AGENTS.md*"))
	if len(names) != 1 {
		t.Errorf("drive root holds %v, want AGENTS.md alone", names)
	}

	// It follows what is installed.
	generate(root, wiki, skills, library)
	if got := read(root); got != driveContextGolden(t, "drive_context.golden")+agentsMDPrefixNote {
		t.Errorf("root AGENTS.md after adding libraries:\n%s", got)
	}
	generate(root, wiki, pi)
	if got := read(root); got != driveContextGolden(t, "drive_context_plain.golden")+agentsMDPrefixNote {
		t.Errorf("root AGENTS.md with a harness and no skills:\n%s", got)
	}
	// With no harness and no skills there is nobody to orient.
	generate(root, wiki, kiwix)
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Errorf("root AGENTS.md was left behind: %v", err)
	}
	leftovers, _ := filepath.Glob(filepath.Join(root, ".svalbard", "AGENTS.md*"))
	if len(leftovers) != 0 {
		t.Errorf("marker left behind: %v", leftovers)
	}

	// A file the user wrote or edited is theirs: never replaced, never removed.
	for name, prepare := range map[string]func(root string){
		"written by the user": func(root string) {},
		"edited by the user":  func(root string) { generate(root, skills) },
	} {
		root := t.TempDir()
		prepare(root)
		if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("my own notes\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		generate(root, skills, pi)
		if got := read(root); got != "my own notes\n" {
			t.Errorf("%s: AGENTS.md was overwritten:\n%s", name, got)
		}
		generate(root, wiki)
		if got := read(root); got != "my own notes\n" {
			t.Errorf("%s: AGENTS.md was removed or changed: %q", name, got)
		}
	}
}
