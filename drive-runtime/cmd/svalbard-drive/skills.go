package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
)

const skillsUsage = `Usage:
  svalbard-drive skills find <words...> [--kind skill|library] [--limit N]
  svalbard-drive skills read <id> [--file F] [--section S] [--offset N]

find lists the skills and JS library quickstarts on this drive that match the
words, one per line: id, name and description. read prints one of them by its
id; a long one comes in parts, and the last line then names the next command.
`

// runSkills is the skills subcommand: the skill_search and skill_read tools
// for an agent that has a shell and no MCP client. It prints what the tools
// return and runs nothing a skill holds.
func runSkills(stdout, stderr io.Writer, driveRoot string, args []string) error {
	usage := func(problem string) error {
		fmt.Fprint(stderr, skillsUsage)
		return errors.New(problem)
	}
	if len(args) == 0 {
		return usage("skills needs a command: find or read")
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
			fmt.Fprintf(stdout, "\nfiles in %s:\n", data.ID)
			for _, file := range data.Files {
				fmt.Fprintf(stdout, "  %s\n", file)
			}
		}
		if data.Note != "" {
			fmt.Fprintln(stdout, data.Note)
		}
		if data.NextOffset > 0 {
			command := "svalbard-drive skills read " + shellWord(data.ID)
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
