package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
)

// skillsUsage is the commands, which a mistake in one prints. %[1]s is the
// program as it was run.
const skillsUsage = `  %[1]s skills find <words...> [--kind skill|library] [--limit N]
  %[1]s skills read <id> [--file F] [--section S] [--offset N]

find lists the skills and JS library quickstarts that match the words, one per
line: id, name and description. read prints one by its id; a long one ends
with the command for its next part. Read a skill before building.
`

// skillsOrientation is what "skills" alone prints. For an agent that was not
// started from the drive's menu it is the orientation: what this is, what the
// drive has to build with, and how to find a procedure. It claims only what
// is installed, and names the commands only when they can find something.
func skillsOrientation(driveRoot string) string {
	const drive = "This is a Svalbard offline drive: reference archives, databases, maps and\ntools that work without a network."
	folder := filepath.Join(driveRoot, "skills") + string(filepath.Separator)
	var text string
	switch {
	case mcp.SkillsInstalled(driveRoot):
		text = drive + " Its skills are procedures for building\nthings with what is on it (apps, dashboards, documents, firmware).\n"
	case mcp.LibrariesInstalled(driveRoot):
		text = drive + " Its JS libraries come with quickstarts\nthat say how to build with them.\n"
		folder = filepath.Join(driveRoot, "lib", "js") + string(filepath.Separator)
	default:
		return drive + " It has no skills or JS libraries\ninstalled, so this command has nothing to find.\n"
	}
	if !mcp.SkillsIndexed(driveRoot) {
		return text + "They are not in a search index here, so find and read cannot reach\nthem: list " + folder + " and read the fitting file.\n"
	}
	return text + "\n" + fmt.Sprintf(skillsUsage, shellWord(skillsProgram))
}

// skillsProgram is how the command is spelled in the usage and in a
// continuation line: the bare name when it was found on PATH, as in a
// harness's shell, and the drive's run script when it was run by path, as
// from a mounted stick, where svalbard-drive is not on PATH.
var skillsProgram = "svalbard-drive"

// runSkills is the skills subcommand: the skill_search and skill_read tools
// for an agent that has a shell and no MCP client. It prints what the tools
// return and runs nothing a skill holds.
func runSkills(stdout, stderr io.Writer, driveRoot string, args []string) error {
	usage := func(problem string) error {
		fmt.Fprintf(stderr, skillsUsage, shellWord(skillsProgram))
		return errors.New(problem)
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, skillsOrientation(driveRoot))
		return nil
	}
	words, flags, err := splitSkillArgs(args[1:])
	if err != nil {
		return usage(err.Error())
	}
	params := map[string]any{}
	allowed := map[string]bool{}
	switch args[0] {
	case "find":
		if len(words) == 0 {
			return usage("skills find needs the words to look for")
		}
		params["query"] = strings.Join(words, " ")
		allowed = map[string]bool{"kind": true, "limit": true}
	case "read":
		if len(words) != 1 {
			return usage("skills read needs one id, from skills find")
		}
		params["id"] = words[0]
		allowed = map[string]bool{"file": true, "section": true, "offset": true}
	default:
		return usage(fmt.Sprintf("unknown skills command: %s", args[0]))
	}
	for name, value := range flags {
		if !allowed[name] {
			return usage(fmt.Sprintf("skills %s has no --%s", args[0], name))
		}
		params[name] = value
		if name == "limit" || name == "offset" {
			number, err := strconv.Atoi(value)
			if err != nil {
				return usage(fmt.Sprintf("--%s needs a number, not %q", name, value))
			}
			params[name] = number
		}
	}

	meta, _ := mcp.LoadMetadata(driveRoot)
	tools := mcp.NewSkillCapability(driveRoot, meta)
	// Keyword search only: one command is not worth loading the embedding
	// model for.
	tools.KeywordOnly = true
	tools.AbsolutePaths = true
	defer tools.Close()
	action := map[string]string{"find": "search", "read": "read"}[args[0]]
	result, err := tools.Handle(context.Background(), action, params)
	if err != nil {
		// The tools' messages name the tools; here the commands do the same.
		return errors.New(strings.NewReplacer("skill_search", "svalbard-drive skills find", "skill_read", "svalbard-drive skills read").Replace(err.Error()))
	}
	switch data := result.Data.(type) {
	case []mcp.SkillResult:
		for _, item := range data {
			fmt.Fprintf(stdout, "%s  %s — %s\n", item.ID, item.Title, item.Description)
		}
	case mcp.SkillReadResult:
		fmt.Fprintln(stdout, data.Body)
		if len(data.Files) > 0 {
			fmt.Fprintf(stdout, "\nfiles in %s:\n", data.Folder)
			for _, file := range data.Files {
				fmt.Fprintf(stdout, "  %s\n", file)
			}
		}
		if data.Note != "" {
			fmt.Fprintln(stdout, data.Note)
		}
		if data.NextOffset > 0 {
			command := shellWord(skillsProgram) + " skills read " + shellWord(data.ID)
			for _, name := range []string{"file", "section"} {
				if value := flags[name]; value != "" {
					command += " --" + name + " " + shellWord(value)
				}
			}
			fmt.Fprintf(stdout, "continues: %s --offset %d\n", command, data.NextOffset)
		}
	default:
		// No index, or nothing found: the tool says so in words.
		fmt.Fprintln(stdout, result.Text)
	}
	return nil
}

// splitSkillArgs separates the words of a skills command from its flags,
// which may stand anywhere among them as "--name value" or "--name=value".
func splitSkillArgs(args []string) (words []string, flags map[string]string, err error) {
	flags = map[string]string{}
	for i := 0; i < len(args); i++ {
		name, isFlag := strings.CutPrefix(args[i], "--")
		if !isFlag || name == "" {
			words = append(words, args[i])
			continue
		}
		if key, value, ok := strings.Cut(name, "="); ok {
			flags[key] = value
			continue
		}
		if i+1 >= len(args) {
			return nil, nil, fmt.Errorf("--%s needs a value", name)
		}
		i++
		flags[name] = args[i]
	}
	return words, flags, nil
}

// shellWord quotes a word for the command line when it needs it.
func shellWord(word string) string {
	if word != "" && !strings.ContainsAny(word, " \t'\"\\$`*?[]{}()<>|&;!#~") {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'"'"'`) + "'"
}
