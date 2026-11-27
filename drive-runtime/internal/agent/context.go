package agent

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/wiki"
)

// What each harness puts before the name of a tool of the MCP server it
// knows as "svalbard". A small model calls a tool by the name it is told, so
// the drive context names tools the way the harness lists them. Each was read
// from the tool list the pinned version sent to the model, and each context
// file's place from that file's text arriving in the same request.
const (
	// PiToolPrefix: pi 1.6.1 names MCP tools mcp__<server>__<tool>.
	PiToolPrefix = "mcp__svalbard__"
	// OmpToolPrefix: oh-my-pi names them mcp__<server>_<tool>.
	OmpToolPrefix = "mcp__svalbard_"
	// OpencodeToolPrefix: opencode 1.19.6 names them <server>_<tool>.
	OpencodeToolPrefix = "svalbard_"
	// GooseToolPrefix: goose 1.61.1 names them <extension>__<tool>.
	GooseToolPrefix = "svalbard__"
)

// ContextOptions says which drive, harness and installed areas a drive
// context is written for.
type ContextOptions struct {
	// DriveRoot is where the drive is mounted. Empty leaves the path out.
	DriveRoot string
	// ToolPrefix is put before every tool name; empty gives the bare names.
	ToolPrefix string
	// Wiki: the drive's server offers the wiki tools. LivePages: they may
	// write pages that compute something.
	Wiki, LivePages bool
	// Skills: a skills recipe is installed.
	Skills bool
	// NoIndex: the skills are not in a search index (the vault's strategy
	// is "none", or it was never built), so the skill tools find nothing and
	// the skills are read as files.
	NoIndex bool
	// Libraries: a js-lib recipe is installed, so pages are packed with the
	// drive's libraries.
	Libraries bool
}

// DriveContext is the context file of a shell harness on the drive: what a
// small model has to be told to use the drive instead of the network. Areas
// the drive does not have are left out. It is the one builder of the text:
// each harness's file is this, with that harness's tool names. Nothing is
// written to the drive's root, where a harness started there would read it a
// second time.
func DriveContext(opts ContextOptions) string {
	tool := func(name string) string { return "`" + opts.ToolPrefix + name + "`" }
	where := ""
	if opts.DriveRoot != "" {
		where = " at `" + opts.DriveRoot + "`"
	}
	tools := "`svalbard`"
	if opts.ToolPrefix != "" {
		tools = "`" + opts.ToolPrefix + "`"
	}

	var b strings.Builder
	b.WriteString("# Svalbard offline drive\n\n")
	b.WriteString("You run from a Svalbard USB drive" + where + ". There is no internet. Do not try curl, wget, git clone, pip install, npm install or any other download.\n\n")
	b.WriteString("The drive has an offline library: reference archives, SQLite databases and maps. For a factual question, look it up with the " + tools + " tools instead of guessing:\n\n")
	b.WriteString("1. " + tool("vault_sources") + " lists what the drive has. Call it first.\n")
	// How to word a query and how a long article continues is in the tools'
	// own descriptions, which the model has beside this text; saying it
	// twice cost 40 tokens of every prompt.
	b.WriteString("2. " + tool("search") + " finds articles by keyword.\n")
	b.WriteString("3. " + tool("search_read") + " reads one article. Pass a search result's `source` and `path`.\n")
	if opts.Skills {
		b.WriteString("\n" + buildingContext(opts) + "\n")
	} else if opts.Libraries {
		b.WriteString("\n" + librariesContext(opts) + "\n")
	}
	if opts.Wiki {
		// The wiki is the one thing on the drive a model may write. Without
		// being told, a model that has just read "there is no internet" and
		// a list of reference tools does not try to save anything.
		b.WriteString("\nThe drive also has a wiki, the user's own notebook, which you can write. To save what you found, create a page with " + tool("wiki_create") + "; " + tool("wiki_search") + " and " + tool("wiki_read") + " look in the notes already there.\n")
		if opts.LivePages {
			// Told only in the tool's description, a 4B model made up a
			// syntax; the help page holds the one that works.
			b.WriteString("A wiki page can update by itself (a live list of pages, a count, open tasks). Before writing such a page, read the page `" + wiki.HelpPage + "` with " + tool("wiki_read") + " and copy its syntax exactly.\n")
		}
	}
	b.WriteString("\nRules:\n\n")
	b.WriteString("- Name the source you used.\n")
	b.WriteString("- If the drive has no answer, say so. Do not invent one.\n")
	b.WriteString("- Do not repeat a tool call with the same arguments. If it failed twice, stop and tell the user.\n")
	b.WriteString("- `./run` on the drive is an interactive menu for the user. Do not run it.\n")
	b.WriteString("- " + stayInPlace + "\n")
	return b.String()
}

// stayInPlace keeps a harness's shell where it belongs. Nothing confines it:
// a 4B model that could not find a skill's template ran find over the user's
// home folder.
const stayInPlace = "Work only in your starting folder and the drive."

// buildingContext sends a model that is asked to make something to the
// drive's skills first. Left to itself a small model improvises, and writes
// a page that loads its libraries from a CDN. Without a search index the
// skill tools have nothing to search, and the model is sent to the skills'
// folder. With JS libraries installed it names the command that packs a page
// with them: left out, a model links a CDN.
func buildingContext(opts ContextOptions) string {
	const what = "To build something (an app, dashboard, map, document, firmware), "
	only := " Use only the libraries and tools it names."
	if opts.Libraries {
		only += " Pack HTML with `svalbard-drive html pack`; never link a CDN."
	}
	if opts.NoIndex {
		folder := "skills/"
		if opts.DriveRoot != "" {
			folder = filepath.ToSlash(filepath.Join(opts.DriveRoot, "skills")) + "/"
		}
		return what + "first read a procedure: with no search index, list `" + folder + "` and read the fitting SKILL.md." + only
	}
	return what + "first find a procedure with `" + opts.ToolPrefix + "skill_search`, read it with `" + opts.ToolPrefix + "skill_read` and follow it." + only
}

// librariesContext is the building paragraph of a drive with JS libraries
// and no skills: the libraries' quickstarts and the packing command.
func librariesContext(opts ContextOptions) string {
	const pack = " and pack the page with `svalbard-drive html pack`; never link a CDN."
	if opts.NoIndex {
		folder := "lib/js/*/docs/"
		if opts.DriveRoot != "" {
			folder = filepath.ToSlash(filepath.Join(opts.DriveRoot, "lib", "js")) + "/*/docs/"
		}
		return "To build a web page, read the quickstarts of the drive's JS libraries in `" + folder + "`, import them by name" + pack
	}
	return "To build a web page, find the drive's JS libraries with `" + opts.ToolPrefix + "skill_search` (kind library), import them by name" + pack
}

// driveContextFor is the context for a harness launched from the drive at
// driveRoot with a model of ctxSize tokens, with the areas that drive has
// installed now and its MCP server offers at that size.
func driveContextFor(driveRoot, toolPrefix string, ctxSize int) string {
	hasWiki := mcp.WikiOffered(driveRoot, ctxSize)
	return DriveContext(ContextOptions{
		DriveRoot:  driveRoot,
		ToolPrefix: toolPrefix,
		Wiki:       hasWiki,
		LivePages:  hasWiki && wiki.LivePages(driveRoot),
		Skills:     mcp.SkillsInstalled(driveRoot),
		NoIndex:    !mcp.SkillsIndexed(driveRoot),
		Libraries:  mcp.LibrariesInstalled(driveRoot),
	})
}

// drivePathEnv puts the drive's tools first on a harness's PATH, so that what
// a skill tells an agent to run in its shell resolves: the binaries of this
// platform (bin/<platform>, as the activate script adds it) and
// svalbard-drive itself. DRIVE_ROOT is how svalbard-drive finds the drive
// from there.
func drivePathEnv(driveRoot, runtimeBinary string) map[string]string {
	env := map[string]string{"DRIVE_ROOT": driveRoot}
	var dirs []string
	if platformName, err := platform.Detect(); err == nil {
		dirs = append(dirs, filepath.Join(driveRoot, "bin", platformName))
	}
	dirs = append(dirs, filepath.Dir(runtimeBinary))
	if inherited := os.Getenv("PATH"); inherited != "" {
		dirs = append(dirs, inherited)
	}
	env["PATH"] = strings.Join(dirs, string(os.PathListSeparator))
	return env
}
