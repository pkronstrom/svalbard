package agent_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/agent"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

var updateGolden = flag.Bool("update", false, "rewrite the golden files")

// installRecipes writes the drive's record of installed recipes, which is
// what opens the gates for the skill and wiki tools.
func installRecipes(t *testing.T, driveRoot string, recipes ...[2]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(driveRoot, ".svalbard"), 0o755); err != nil {
		t.Fatal(err)
	}
	var entries []string
	for _, recipe := range recipes {
		entries = append(entries, `"`+recipe[0]+`": {"id": "`+recipe[0]+`", "type": "`+recipe[1]+`", "description": "", "tags": []}`)
	}
	if err := os.WriteFile(filepath.Join(driveRoot, ".svalbard", "recipes.json"), []byte("{"+strings.Join(entries, ",")+"}"), 0o644); err != nil {
		t.Fatal(err)
	}
}

var (
	skillsRecipe  = [2]string{"skills-core", "skills"}
	libraryRecipe = [2]string{"js-core", "js-lib"}
	wikiRecipe    = [2]string{"silverbullet", "binary"}
)

func TestDriveContextOmitsAbsentAreas(t *testing.T) {
	plain := agent.DriveContext(agent.ContextOptions{DriveRoot: "/Volumes/STICK", ToolPrefix: "mcp__svalbard__"})
	for _, absent := range []string{"skill_search", "skill_read", "wiki", "html pack", "CDN"} {
		if strings.Contains(plain, absent) {
			t.Errorf("a drive without skills, libraries or wiki is told about %q:\n%s", absent, plain)
		}
	}
	for _, want := range []string{"`/Volumes/STICK`", "no internet", "`mcp__svalbard__vault_sources`", "`mcp__svalbard__search`", "`mcp__svalbard__search_read`", "Rules:"} {
		if !strings.Contains(plain, want) {
			t.Errorf("context missing %q:\n%s", want, plain)
		}
	}

	skills := agent.DriveContext(agent.ContextOptions{DriveRoot: "/Volumes/STICK", ToolPrefix: "mcp__svalbard__", Skills: true})
	if !strings.Contains(skills, "`mcp__svalbard__skill_search`") || !strings.Contains(skills, "`mcp__svalbard__skill_read`") || strings.Contains(skills, "html pack") {
		t.Errorf("skills only:\n%s", skills)
	}
	libraries := agent.DriveContext(agent.ContextOptions{DriveRoot: "/Volumes/STICK", ToolPrefix: "mcp__svalbard__", Libraries: true})
	if !strings.Contains(libraries, "svalbard-drive html pack") || !strings.Contains(libraries, "CDN") {
		t.Errorf("libraries only:\n%s", libraries)
	}
	// The live-pages pointer belongs to the wiki's paragraph.
	if text := agent.DriveContext(agent.ContextOptions{LivePages: true}); strings.Contains(text, "wiki") {
		t.Errorf("live pages without a wiki:\n%s", text)
	}
	wiki := agent.DriveContext(agent.ContextOptions{ToolPrefix: "mcp__svalbard__", Wiki: true, LivePages: true, Skills: true})
	order := []int{strings.Index(wiki, "skill_search"), strings.Index(wiki, "The drive also has a wiki"), strings.Index(wiki, "Help/Live pages"), strings.Index(wiki, "Rules:")}
	for i := range order {
		if order[i] < 0 || i > 0 && order[i] < order[i-1] {
			t.Fatalf("paragraph order = %v, want skills, wiki, live pages, rules:\n%s", order, wiki)
		}
	}

	// Without a drive root the sentence about where the drive is goes.
	rootless := agent.DriveContext(agent.ContextOptions{})
	if !strings.Contains(rootless, "You run from a Svalbard USB drive. There is no internet.") || strings.Contains(rootless, "``") || strings.Contains(rootless, "%!") {
		t.Errorf("context without a drive root:\n%s", rootless)
	}
}

func TestDriveContextUsesHarnessPrefix(t *testing.T) {
	all := agent.ContextOptions{DriveRoot: "/Volumes/STICK", ToolPrefix: "mcp__svalbard_", Wiki: true, LivePages: true, Skills: true, Libraries: true}
	text := agent.DriveContext(all)
	for _, tool := range []string{"vault_sources", "search", "search_read", "skill_search", "skill_read", "wiki_create", "wiki_search", "wiki_read"} {
		if !strings.Contains(text, "`mcp__svalbard_"+tool+"`") {
			t.Errorf("context does not name `mcp__svalbard_%s`:\n%s", tool, text)
		}
	}
	if strings.Contains(text, "mcp__svalbard__") {
		t.Errorf("context names a tool with another harness's prefix:\n%s", text)
	}
	// Bare names, for the copy any agent may read.
	all.ToolPrefix = ""
	if bare := agent.DriveContext(all); !strings.Contains(bare, "`skill_search`") || !strings.Contains(bare, "`vault_sources`") || strings.Contains(bare, "mcp__") {
		t.Errorf("context without a prefix:\n%s", bare)
	}
}

// The host writes the same text to the drive's root, from its own copy of
// it: the modules share no code. These files are what both are held to.
func TestDriveContextGolden(t *testing.T) {
	for name, opts := range map[string]agent.ContextOptions{
		"drive_context.golden":           {Skills: true, Libraries: true},
		"drive_context_skills.golden":    {Skills: true},
		"drive_context_libraries.golden": {Libraries: true},
		"drive_context_plain.golden":     {},
	} {
		path := filepath.Join("testdata", name)
		got := agent.DriveContext(opts)
		if *updateGolden {
			if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%v (run the test with -update to write it)", err)
		}
		if got != string(want) {
			t.Errorf("%s differs from DriveContext(%+v); if the change is meant, run with -update and update host-cli/internal/toolkit/agentsmd.go to match.\ngot:\n%s", name, opts, got)
		}
	}
}

// The context is in every prompt. At the 8K tier it shares the window with
// the harness's own prompt, the tool definitions, a skill and the work.
func TestDriveContextBudget(t *testing.T) {
	longest := ""
	for _, prefix := range []string{"mcp__svalbard__", "mcp__svalbard_", agent.OpencodeToolPrefix, agent.GooseToolPrefix} {
		if len(prefix) > len(longest) {
			longest = prefix
		}
	}
	all := agent.DriveContext(agent.ContextOptions{DriveRoot: "/Volumes/SVALBARD", ToolPrefix: longest, Wiki: true, LivePages: true, Skills: true, Libraries: true})
	if words := len(strings.Fields(all)); words > 260 {
		t.Errorf("the drive context with every area is %d words, over the budget of 260:\n%s", words, all)
	}
}

func prepare(t *testing.T, driveRoot, client string) agent.LaunchConfig {
	t.Helper()
	cfg, err := agent.PrepareClientLaunchConfig(driveRoot, client, "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "qwen", 32768)
	if err != nil {
		t.Fatalf("PrepareClientLaunchConfig(%s) error = %v", client, err)
	}
	return cfg
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPrepareOpencodeWritesContext(t *testing.T) {
	driveRoot := t.TempDir()
	installRecipes(t, driveRoot, skillsRecipe, wikiRecipe)
	cfg := prepare(t, driveRoot, "opencode")
	path := filepath.Join(cfg.Env["XDG_CONFIG_HOME"], "opencode", "AGENTS.md")
	context := readFile(t, path)
	for _, want := range []string{driveRoot, "no internet", "`" + agent.OpencodeToolPrefix + "search_read`", "`" + agent.OpencodeToolPrefix + "skill_search`", "`" + agent.OpencodeToolPrefix + "wiki_create`"} {
		if !strings.Contains(context, want) {
			t.Errorf("opencode AGENTS.md missing %q:\n%s", want, context)
		}
	}
	if strings.Contains(context, "mcp__svalbard__") || strings.Contains(context, "html pack") {
		t.Errorf("opencode AGENTS.md:\n%s", context)
	}
	// A user's edit is kept, as it is for pi.
	if err := os.WriteFile(path, []byte("my own notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prepare(t, driveRoot, "opencode")
	if got := readFile(t, path); got != "my own notes\n" {
		t.Errorf("user-edited opencode AGENTS.md was overwritten:\n%s", got)
	}
}

func TestPrepareGooseWritesHints(t *testing.T) {
	driveRoot := t.TempDir()
	plain := readFile(t, filepath.Join(prepare(t, driveRoot, "goose").Env["XDG_CONFIG_HOME"], "goose", ".goosehints"))
	if !strings.Contains(plain, "`"+agent.GooseToolPrefix+"search_read`") || strings.Contains(plain, "skill_search") || strings.Contains(plain, "wiki") {
		t.Errorf("goose hints on a plain drive:\n%s", plain)
	}
	// Installed later, the skills reach the next launch.
	installRecipes(t, driveRoot, skillsRecipe, libraryRecipe)
	cfg := prepare(t, driveRoot, "goose")
	hints := readFile(t, filepath.Join(cfg.Env["XDG_CONFIG_HOME"], "goose", ".goosehints"))
	for _, want := range []string{driveRoot, "`" + agent.GooseToolPrefix + "skill_search`", "`" + agent.GooseToolPrefix + "skill_read`", "svalbard-drive html pack"} {
		if !strings.Contains(hints, want) {
			t.Errorf("goose hints missing %q:\n%s", want, hints)
		}
	}
	// The extension config beside it is still written.
	if config := readFile(t, filepath.Join(cfg.Env["XDG_CONFIG_HOME"], "goose", "config.yaml")); !strings.Contains(config, "  svalbard:") {
		t.Errorf("goose config.yaml lost the extension:\n%s", config)
	}
}

func TestPiContextNamesTheSkillToolsOnlyWhenInstalled(t *testing.T) {
	driveRoot := t.TempDir()
	path := filepath.Join(prepare(t, driveRoot, "pi").Env["PI_CODING_AGENT_DIR"], "AGENTS.md")
	if context := readFile(t, path); strings.Contains(context, "skill_search") {
		t.Errorf("pi is told about skills the drive does not have:\n%s", context)
	}
	installRecipes(t, driveRoot, skillsRecipe)
	prepare(t, driveRoot, "pi")
	if context := readFile(t, path); !strings.Contains(context, "`mcp__svalbard__skill_search`") || !strings.Contains(context, "`mcp__svalbard__skill_read`") {
		t.Errorf("pi AGENTS.md does not name the skill tools:\n%s", context)
	}
}

func TestOmpPromptMentionsSkills(t *testing.T) {
	launch := func(driveRoot string) (args, prompt string) {
		t.Helper()
		cfg := prepare(t, driveRoot, "omp")
		return strings.Join(cfg.Args, " "), readFile(t, filepath.Join(cfg.Env["PI_CODING_AGENT_DIR"], "lean-prompt.md"))
	}
	args, prompt := launch(t.TempDir())
	if strings.Contains(args, "skill_") || strings.Contains(prompt, "skill") {
		t.Errorf("omp was offered skills the drive does not have:\n%s\n%s", args, prompt)
	}

	driveRoot := t.TempDir()
	installRecipes(t, driveRoot, skillsRecipe, libraryRecipe)
	args, prompt = launch(driveRoot)
	// omp enables only the tools it is given by name, and reads no context
	// file: its prompt is all the orientation it gets.
	for _, tool := range []string{"mcp__svalbard_skill_search", "mcp__svalbard_skill_read"} {
		if !strings.Contains(args, tool) {
			t.Errorf("Args missing %s: %s", tool, args)
		}
		if !strings.Contains(prompt, "`"+tool+"`") {
			t.Errorf("lean prompt does not name %s:\n%s", tool, prompt)
		}
	}
	if !strings.Contains(prompt, "svalbard-drive html pack") || strings.Contains(prompt, "mcp__svalbard__") {
		t.Errorf("lean prompt:\n%s", prompt)
	}
}

func TestPrepareClientPutsDriveBinariesOnPath(t *testing.T) {
	driveRoot := t.TempDir()
	platformName, err := platform.Detect()
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "/usr/local/inherited:/usr/bin")
	want := strings.Join([]string{filepath.Join(driveRoot, "bin", platformName), filepath.Dir(exe), "/usr/local/inherited", "/usr/bin"}, string(os.PathListSeparator))
	for _, client := range []string{"pi", "omp", "opencode", "goose"} {
		cfg := prepare(t, driveRoot, client)
		if got := cfg.Env["PATH"]; got != want {
			t.Errorf("%s: PATH = %q, want %q", client, got, want)
		}
		// svalbard-drive finds the drive by this when an agent's shell runs it.
		if got := cfg.Env["DRIVE_ROOT"]; got != driveRoot {
			t.Errorf("%s: DRIVE_ROOT = %q, want %q", client, got, driveRoot)
		}
	}
	// An empty inherited PATH leaves no empty entry, which would mean the
	// working folder.
	t.Setenv("PATH", "")
	if got := prepare(t, driveRoot, "pi").Env["PATH"]; strings.HasSuffix(got, string(os.PathListSeparator)) || got == "" {
		t.Errorf("PATH with nothing inherited = %q", got)
	}
}
