package cli

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

// skillText is the agent skill: how to run this CLI without its TUI.
//
//go:embed skill/SKILL.md
var skillText string

// skillDirs maps an agent to its skills directory under the user's home.
var skillDirs = map[string]string{
	"claude":   ".claude/skills",
	"codex":    ".codex/skills",
	"pi":       ".pi/agent/skills",
	"opencode": ".config/opencode/skills",
}

func skillAgents() []string {
	names := make([]string, 0, len(skillDirs))
	for name := range skillDirs {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// newSkillCommand prints or installs the agent skill. Nothing installs it
// automatically.
func newSkillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Teach a coding agent to run svalbard: print or install its skill",
	}

	printCmd := &cobra.Command{
		Use:   "print",
		Short: "Print the skill",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprint(cmd.OutOrStdout(), skillText)
			return err
		},
	}

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Write the skill where an agent looks for skills",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			agent, _ := cmd.Flags().GetString("agent")
			dir, _ := cmd.Flags().GetString("dir")
			switch {
			case agent != "" && dir != "":
				return fmt.Errorf("give --agent or --dir, not both")
			case agent != "":
				rel, ok := skillDirs[agent]
				if !ok {
					return fmt.Errorf("unknown agent %q; use %s, or --dir <skills directory>", agent, strings.Join(skillAgents(), ", "))
				}
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				dir = filepath.Join(home, rel)
			case dir == "":
				return fmt.Errorf("name the agent: --agent %s, or --dir <skills directory>", strings.Join(skillAgents(), "|"))
			}
			path := filepath.Join(dir, "svalbard", "SKILL.md")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, []byte(skillText), 0o644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Installed the svalbard skill at %s\n", path)
			return nil
		},
	}
	installCmd.Flags().String("agent", "", "agent to install for: "+strings.Join(skillAgents(), ", "))
	installCmd.Flags().String("dir", "", "skills directory to install into instead")

	cmd.AddCommand(printCmd, installCmd)
	return cmd
}
