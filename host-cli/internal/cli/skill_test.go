package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

// skillCommands returns the svalbard command lines the skill shows.
func skillCommands() [][]string {
	var lines [][]string
	inFence := false
	for _, line := range strings.Split(skillText, "\n") {
		if strings.HasPrefix(line, "```") {
			inFence = !inFence
			continue
		}
		if fields := strings.Fields(line); inFence && len(fields) > 1 && fields[0] == "svalbard" {
			lines = append(lines, fields[1:])
		}
	}
	return lines
}

func TestSkillNamesOnlyCommandsAndFlagsThatExist(t *testing.T) {
	lines := skillCommands()
	if len(lines) < 10 {
		t.Fatalf("found %d command lines in the skill, expected the full reference", len(lines))
	}
	for _, args := range lines {
		cmd, rest, err := NewRootCommand().Find(args)
		if err != nil || cmd.Name() == "svalbard" {
			t.Errorf("svalbard %s: no such command", strings.Join(args, " "))
			continue
		}
		for _, arg := range rest {
			if !strings.HasPrefix(arg, "--") {
				continue
			}
			name := strings.TrimPrefix(arg, "--")
			if cmd.Flags().Lookup(name) == nil && cmd.InheritedFlags().Lookup(name) == nil {
				t.Errorf("svalbard %s: %s has no flag --%s", strings.Join(args, " "), cmd.Name(), name)
			}
		}
	}
}

func TestSkillNamesEverySupportedPlatform(t *testing.T) {
	for _, platform := range toolkit.SupportedPlatforms() {
		if !strings.Contains(skillText, "`"+platform+"`") {
			t.Errorf("the skill does not name platform %s", platform)
		}
	}
}

func TestSkillExamplesUseRealPresetsAndItems(t *testing.T) {
	for _, name := range []string{"default-64", "ai/harnesses"} {
		if _, _, err := run(t, "preset", "show", name); err != nil {
			t.Errorf("preset %s: %v", name, err)
		}
	}
	for _, id := range []string{"zimgit-knots", "cd3wd"} {
		if out, _, err := run(t, "catalog", id); err != nil || !strings.Contains(out, id) {
			t.Errorf("item %s is not in the catalog: %v", id, err)
		}
	}
}

func TestSkillInstallWritesOnlyWhereAsked(t *testing.T) {
	dir := t.TempDir()
	out, _, err := run(t, "skill", "install", "--dir", dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "svalbard", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil || string(data) != skillText {
		t.Fatalf("installed skill differs from the embedded one: %v", err)
	}
	if !strings.Contains(out, path) {
		t.Errorf("output does not name the installed path: %s", out)
	}

	if _, _, err := run(t, "skill", "install"); err == nil {
		t.Error("install without --agent or --dir succeeded")
	}
	if _, _, err := run(t, "skill", "install", "--agent", "nobody"); err == nil || !slices.ContainsFunc(skillAgents(), func(a string) bool { return strings.Contains(err.Error(), a) }) {
		t.Errorf("unknown agent error does not list the agents: %v", err)
	}

	printed, _, err := run(t, "skill", "print")
	if err != nil || printed != skillText {
		t.Fatalf("skill print differs from the embedded skill: %v", err)
	}
}
