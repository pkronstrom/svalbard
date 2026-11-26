package agent_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/agent"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

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
	// A drive that was applied has a search index unless its strategy is
	// "none", and its skills and quickstarts are in it; removeIndex takes it
	// away again.
	path := filepath.Join(driveRoot, "data", "search.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(path)
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE sources (id INTEGER PRIMARY KEY, filename TEXT NOT NULL, title TEXT NOT NULL DEFAULT '')`); err != nil {
		t.Fatal(err)
	}
	for _, recipe := range recipes {
		filename := map[string]string{"skills": "skills/" + recipe[0] + "/data-dashboard/SKILL.md", "js-lib": "lib/js/" + recipe[0] + "/docs/chartjs.md"}[recipe[1]]
		if filename == "" {
			continue
		}
		if _, err := db.Exec(`INSERT INTO sources (filename, title) VALUES (?, ?)`, filename, recipe[0]); err != nil {
			t.Fatal(err)
		}
	}
}

func removeIndex(t *testing.T, driveRoot string) {
	t.Helper()
	if err := os.Remove(filepath.Join(driveRoot, "data", "search.db")); err != nil {
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
	for _, absent := range []string{"skill_search", "skill_read", "wiki", "html pack"} {
		if strings.Contains(plain, absent) {
			t.Errorf("a drive without skills, libraries or wiki is told about %q:\n%s", absent, plain)
		}
	}
	for _, want := range []string{"`/Volumes/STICK`", "no internet", "`mcp__svalbard__vault_sources`", "`mcp__svalbard__search`", "`mcp__svalbard__search_read`", "Rules:", "Work only in your starting folder and the drive."} {
		if !strings.Contains(plain, want) {
			t.Errorf("context missing %q:\n%s", want, plain)
		}
	}

	skills := agent.DriveContext(agent.ContextOptions{DriveRoot: "/Volumes/STICK", ToolPrefix: "mcp__svalbard__", Skills: true})
	if !strings.Contains(skills, "`mcp__svalbard__skill_search`") || !strings.Contains(skills, "`mcp__svalbard__skill_read`") || strings.Contains(skills, "html pack") {
		t.Errorf("skills only:\n%s", skills)
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
	all := agent.ContextOptions{DriveRoot: "/Volumes/STICK", ToolPrefix: "mcp__svalbard_", Wiki: true, LivePages: true, Skills: true}
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

// The context is in every prompt. At the 8K tier it shares the window with
// the harness's own prompt, the tool definitions, a skill and the work.
func TestDriveContextBudget(t *testing.T) {
	longest := ""
	for _, prefix := range []string{"mcp__svalbard__", "mcp__svalbard_", agent.OpencodeToolPrefix, agent.GooseToolPrefix} {
		if len(prefix) > len(longest) {
			longest = prefix
		}
	}
	for _, noIndex := range []bool{false, true} {
		all := agent.DriveContext(agent.ContextOptions{DriveRoot: "/Volumes/SVALBARD", ToolPrefix: longest, Wiki: true, LivePages: true, Skills: true, NoIndex: noIndex, Libraries: true})
		t.Logf("every area (no index: %v): %d words", noIndex, len(strings.Fields(all)))
		if words := len(strings.Fields(all)); words > 260 {
			t.Errorf("the drive context with every area (no index: %v) is %d words, over the budget of 260:\n%s", noIndex, words, all)
		}
	}
}

// A drive applied with the index strategy "none" has skills and no way to
// search them: the context sends the model to their folder, not to a tool
// that can only answer that there is no index.
func TestDriveContextSaysWhenThereIsNoIndex(t *testing.T) {
	text := agent.DriveContext(agent.ContextOptions{DriveRoot: "/Volumes/STICK", ToolPrefix: "mcp__svalbard__", Skills: true, NoIndex: true})
	if strings.Contains(text, "skill_search") || strings.Contains(text, "skill_read") {
		t.Errorf("a drive without an index is told to search its skills:\n%s", text)
	}
	for _, want := range []string{"no search index", "`/Volumes/STICK/skills/`", "SKILL.md"} {
		if !strings.Contains(text, want) {
			t.Errorf("context missing %q:\n%s", want, text)
		}
	}
	// NoIndex alone says nothing: a drive without skills has nothing to find.
	if plain := agent.DriveContext(agent.ContextOptions{NoIndex: true}); strings.Contains(plain, "index") {
		t.Errorf("a drive without skills:\n%s", plain)
	}

	// It follows the drive: pi and omp are told when the index is gone and
	// when it is back.
	driveRoot := t.TempDir()
	installRecipes(t, driveRoot, skillsRecipe)
	removeIndex(t, driveRoot)
	pi := readFile(t, filepath.Join(prepare(t, driveRoot, "pi").Env["PI_CODING_AGENT_DIR"], "AGENTS.md"))
	omp := readFile(t, filepath.Join(prepare(t, driveRoot, "omp").Env["PI_CODING_AGENT_DIR"], "lean-prompt.md"))
	for name, text := range map[string]string{"pi": pi, "omp": omp} {
		if strings.Contains(text, "skill_search") || !strings.Contains(text, "no search index") || !strings.Contains(text, filepath.ToSlash(filepath.Join(driveRoot, "skills"))+"/") {
			t.Errorf("%s without an index:\n%s", name, text)
		}
	}
	installRecipes(t, driveRoot, skillsRecipe)
	if pi = readFile(t, filepath.Join(prepare(t, driveRoot, "pi").Env["PI_CODING_AGENT_DIR"], "AGENTS.md")); !strings.Contains(pi, "`mcp__svalbard__skill_search`") {
		t.Errorf("pi after the index came back:\n%s", pi)
	}

	// The file search.db does not decide it. A vault switched to the
	// strategy "none" keeps its old index, and an index built before the
	// skills were installed holds none of them.
	noIndex := func(driveRoot string) bool {
		text := readFile(t, filepath.Join(prepare(t, driveRoot, "pi").Env["PI_CODING_AGENT_DIR"], "AGENTS.md"))
		return strings.Contains(text, "no search index") && !strings.Contains(text, "skill_search")
	}
	switched := t.TempDir()
	installRecipes(t, switched, skillsRecipe)
	manifest := "version: 2\ndesired:\n    options:\n        index_strategy: none\n"
	if err := os.WriteFile(filepath.Join(switched, "manifest.yaml"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if !noIndex(switched) {
		t.Error("a vault switched to the strategy none is sent to skill_search")
	}
	before := t.TempDir()
	installRecipes(t, before, wikiRecipe)
	installRecipesKeepingIndex(t, before, skillsRecipe, wikiRecipe)
	if !noIndex(before) {
		t.Error("skills that are not in the index are sent to skill_search")
	}
}

// installRecipesKeepingIndex changes the installed recipes and leaves the
// index as it is, as when an apply did not index.
func installRecipesKeepingIndex(t *testing.T, driveRoot string, recipes ...[2]string) {
	t.Helper()
	index := filepath.Join(driveRoot, "data", "search.db")
	kept, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	installRecipes(t, driveRoot, recipes...)
	if err := os.WriteFile(index, kept, 0o644); err != nil {
		t.Fatal(err)
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
	for _, want := range []string{driveRoot, "`" + agent.GooseToolPrefix + "skill_search`", "`" + agent.GooseToolPrefix + "skill_read`"} {
		if !strings.Contains(hints, want) {
			t.Errorf("goose hints missing %q:\n%s", want, hints)
		}
	}
	// The extension config beside it is still written.
	if config := readFile(t, filepath.Join(cfg.Env["XDG_CONFIG_HOME"], "goose", "config.yaml")); !strings.Contains(config, "  svalbard:") {
		t.Errorf("goose config.yaml lost the extension:\n%s", config)
	}
}

// On the Fast tier the drive's server offers no wiki tools, so no harness is
// told about them, and each starts the server with its model's window.
func TestFastTierHarnessesAreNotToldAboutTheWiki(t *testing.T) {
	driveRoot := t.TempDir()
	installRecipes(t, driveRoot, skillsRecipe, wikiRecipe)
	contextFiles := map[string][2]string{
		"pi":       {"PI_CODING_AGENT_DIR", "AGENTS.md"},
		"opencode": {"XDG_CONFIG_HOME", "opencode/AGENTS.md"},
		"goose":    {"XDG_CONFIG_HOME", "goose/.goosehints"},
	}
	serverConfigs := map[string][2]string{
		"pi":       {"PI_CODING_AGENT_DIR", "mcp.json"},
		"opencode": {"XDG_CONFIG_HOME", "opencode.json"},
		"goose":    {"XDG_CONFIG_HOME", "goose/config.yaml"},
	}
	for client, file := range contextFiles {
		for _, ctxSize := range []int{8192, 32768} {
			cfg, err := agent.PrepareClientLaunchConfig(driveRoot, client, "http://127.0.0.1:8082", "http://127.0.0.1:8082/v1", "qwen", ctxSize)
			if err != nil {
				t.Fatal(err)
			}
			text := readFile(t, filepath.Join(cfg.Env[file[0]], filepath.FromSlash(file[1])))
			if hasWiki := strings.Contains(text, "wiki_create"); hasWiki != (ctxSize >= 32768) {
				t.Errorf("%s at %d: wiki in the context %t:\n%s", client, ctxSize, hasWiki, text)
			}
			if !strings.Contains(text, "skill_search") {
				t.Errorf("%s at %d: the skill tools are gone:\n%s", client, ctxSize, text)
			}
			server := serverConfigs[client]
			config := readFile(t, filepath.Join(cfg.Env[server[0]], filepath.FromSlash(server[1])))
			if !regexp.MustCompile(`"--context",?\s*(- )?"` + strconv.Itoa(ctxSize) + `"`).MatchString(config) {
				t.Errorf("%s at %d: the server is not told the window:\n%s", client, ctxSize, config)
			}
		}
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
	// With JS libraries the prompt names the packing command.
	if !strings.Contains(prompt, "`svalbard-drive html pack`") || strings.Contains(prompt, "mcp__svalbard__") || !strings.Contains(prompt, "Work only in your starting folder and the drive.") {
		t.Errorf("lean prompt:\n%s", prompt)
	}
	skillsOnly := t.TempDir()
	installRecipes(t, skillsOnly, skillsRecipe)
	if _, prompt := launch(skillsOnly); strings.Contains(prompt, "html pack") {
		t.Errorf("a drive without JS libraries is told to pack pages:\n%s", prompt)
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

// With JS libraries the context names the command that packs a page; a
// drive with skills and no libraries does not mention it.
func TestDriveContextNamesHTMLPackWithLibraries(t *testing.T) {
	with := agent.DriveContext(agent.ContextOptions{ToolPrefix: "mcp__svalbard__", Skills: true, Libraries: true})
	if !strings.Contains(with, "`svalbard-drive html pack`") || !strings.Contains(with, "never link a CDN") {
		t.Errorf("context with libraries:\n%s", with)
	}
	if without := agent.DriveContext(agent.ContextOptions{ToolPrefix: "mcp__svalbard__", Skills: true}); strings.Contains(without, "html pack") {
		t.Errorf("context without libraries names html pack:\n%s", without)
	}
}
