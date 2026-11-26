package toolkit

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

const (
	agentsMDFile = "AGENTS.md"
	// agentsMDMarkerFile records, under .svalbard, the hash of the AGENTS.md
	// Svalbard last wrote: the same record the drive runtime keeps beside the
	// context files it writes for its harnesses (".svalbard-sha256"), kept
	// out of the drive's root, where people look.
	agentsMDMarkerFile = "AGENTS.md.svalbard-sha256"

	// Recipe types whose content agents find with the skill tools. Keep in
	// sync with drive-runtime/internal/mcp.
	skillsRecipeType  = "skills"
	libraryRecipeType = "js-lib"
)

// agentsMDPrefixNote ends the copy at the drive's root, which names the tools
// bare: an agent started there may be any harness.
const agentsMDPrefixNote = "\nYour harness may prefix tool names with the server name `svalbard`.\n"

// agentsMDText is the AGENTS.md at the drive's root: what an agent started on
// the drive has to be told to use it instead of the network. It is the drive
// runtime's context for its own harnesses (drive-runtime/internal/agent,
// DriveContext) without a mount point and with bare tool names; the modules
// share no code, and the golden files in that package's testdata hold the
// two texts together.
func agentsMDText(skills, libraries bool) string {
	var b strings.Builder
	b.WriteString("# Svalbard offline drive\n\n")
	b.WriteString("You run from a Svalbard USB drive. There is no internet. Do not try curl, wget, git clone, pip install, npm install or any other download. They fail.\n\n")
	b.WriteString("The drive has an offline library: reference archives, SQLite databases and maps. For a factual question, look it up with the `svalbard` tools instead of guessing:\n\n")
	b.WriteString("1. `vault_sources` lists what is on the drive. Call it first.\n")
	b.WriteString("2. `search` finds articles by keyword.\n")
	b.WriteString("3. `search_read` reads one article. Pass the `source` and `path` from a search result.\n")
	var building []string
	if skills {
		building = append(building, "To build something (an app, dashboard, map, document, firmware), first find a procedure with `skill_search`, read it with `skill_read` and follow it. Use only the libraries and tools it names; they are on this drive.")
	}
	if libraries {
		building = append(building, "Pack HTML with `svalbard-drive html pack`; never link a CDN.")
	}
	if len(building) > 0 {
		b.WriteString("\n" + strings.Join(building, " ") + "\n")
	}
	b.WriteString("\nRules:\n\n")
	b.WriteString("- Name the source you used.\n")
	b.WriteString("- If the drive has no answer, say so. Do not invent one.\n")
	b.WriteString("- Do not repeat a tool call with the same arguments. If it failed twice, stop and tell the user.\n")
	b.WriteString("- `./run` on the drive is an interactive menu for the user. Do not run it.\n")
	b.WriteString(agentsMDPrefixNote)
	return b.String()
}

// writeAgentsMD keeps the AGENTS.md at the drive's root in step with what is
// installed: written when the drive has an AI harness or skills, removed
// when it has neither. A file the user wrote or edited is left as it is.
func writeAgentsMD(root string, entries []manifest.RealizedEntry, menus map[string]catalog.MenuSpec) error {
	var harness, skills, libraries bool
	for _, entry := range entries {
		switch {
		case entry.Type == skillsRecipeType:
			skills = true
		case entry.Type == libraryRecipeType:
			libraries = true
		case entry.Type == "binary" && menus[entry.ID].Group == "local-ai":
			// The menu launches these as agents; see writeActionsConfig.
			harness = true
		}
	}
	path := filepath.Join(root, agentsMDFile)
	markerPath := filepath.Join(root, svalbardDir, agentsMDMarkerFile)
	if !harness && !skills && !libraries {
		return removeGeneratedFileHost(path, markerPath)
	}
	return writeGeneratedFileHost(path, markerPath, agentsMDText(skills, libraries))
}

// writeGeneratedFileHost keeps a Svalbard-written file current without
// touching the user's edits: the file is replaced only while it still matches
// what Svalbard last wrote, whose hash is recorded at markerPath. It is the
// twin of writeGeneratedFile in drive-runtime/internal/agent.
func writeGeneratedFileHost(path, markerPath, content string) error {
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if !generatedFileUnchanged(existing, markerPath) || string(existing) == content {
			return nil
		}
	case !os.IsNotExist(err):
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	return os.WriteFile(markerPath, []byte(sha256Hex([]byte(content))+"\n"), 0o644)
}

// removeGeneratedFileHost removes a file Svalbard wrote and its record,
// unless the user has edited the file since.
func removeGeneratedFileHost(path, markerPath string) error {
	existing, err := os.ReadFile(path)
	if err == nil && !generatedFileUnchanged(existing, markerPath) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	for _, file := range []string{path, markerPath} {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func generatedFileUnchanged(existing []byte, markerPath string) bool {
	marker, _ := os.ReadFile(markerPath)
	return sha256Hex(existing) == strings.TrimSpace(string(marker))
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
