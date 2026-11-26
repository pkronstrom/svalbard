package mcp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/workspace"
)

const (
	// maxReadBytes caps the text one file_read returns. It is about 6,000
	// tokens, a fifth of the context a small local model runs with.
	maxReadBytes = 20_000
	// maxReadFileBytes caps the file file_read and file_edit will load at all.
	maxReadFileBytes = 8 << 20

	maxListEntries   = 200
	defaultListDepth = 2
	maxListDepth     = 5
)

// FilesCapability exposes the drive's workspace folder via the MCP "file"
// tools: list, read, write and edit, with no delete, move or execute. Only the
// browser front end gets it; terminal harnesses bring their own file tools.
type FilesCapability struct {
	ws      *workspace.Workspace
	baseURL string
}

// NewFilesCapability creates the file tools for ws. baseURL is where the
// workspace is served, without a trailing slash; write and edit return links
// below it. Empty means no links. The caller keeps ownership of ws.
func NewFilesCapability(ws *workspace.Workspace, baseURL string) *FilesCapability {
	return &FilesCapability{ws: ws, baseURL: baseURL}
}

func (c *FilesCapability) Tool() string { return "file" }

func (c *FilesCapability) Actions() []ActionDef {
	const pathDesc = "File path inside the workspace, for example: notes/page.html"
	return []ActionDef{
		{
			Name: "list",
			Desc: "List the files and folders in the workspace, the one folder where you can save files. Returns each entry's path, type, size in bytes and modified time.",
			Params: []ParamDef{
				{Name: "path", Type: "string", Desc: "Folder inside the workspace to list, for example: notes. Omit to list the whole workspace."},
				{Name: "depth", Type: "integer", Desc: "How many folder levels to go down, from 1 to 5", Default: defaultListDepth},
			},
		},
		{
			Name: "read",
			// Offered line ranges without this steer, a 4B model pages through a
			// 300-line file twenty lines at a time.
			Desc: "Read a text file from the workspace. Give only the path: a normal page comes back whole. Only a very long file comes back in parts, and then the reply says which start_line to ask for next. Call this before file_edit to copy the exact text you want to replace.",
			Params: []ParamDef{
				{Name: "path", Type: "string", Required: true, Desc: pathDesc},
				{Name: "start_line", Type: "integer", Desc: "Only for a very long file: first line to return, counting from 1", Default: 1},
				{Name: "end_line", Type: "integer", Desc: "Only for a very long file: last line to return. Omit to read to the end."},
			},
		},
		{
			Name:   "write",
			Writes: true,
			Desc:   "Save a text file in the workspace, for example an HTML page. Creates the file, or replaces it completely if it exists. Returns a Markdown link to the file: put that link in your reply exactly as returned so the user can click it. To change part of an existing file use file_edit instead.",
			Params: []ParamDef{
				{Name: "path", Type: "string", Required: true, Desc: pathDesc},
				{Name: "content", Type: "string", Required: true, Desc: "The complete text of the file"},
			},
		},
		{
			Name:   "edit",
			Writes: true,
			Desc:   "Change part of a workspace file: replace one exact piece of text with another. old_text must match the file exactly and occur once; call file_read first and copy it from there. Returns a Markdown link to the file, as file_write does.",
			Params: []ParamDef{
				{Name: "path", Type: "string", Required: true, Desc: pathDesc},
				{Name: "old_text", Type: "string", Required: true, Desc: "Exact text to replace, copied from the file, for example: <h1>Draft</h1>"},
				{Name: "new_text", Type: "string", Required: true, Desc: "Text to put in its place, for example: <h1>Final</h1>"},
				{Name: "replace_all", Type: "boolean", Desc: "Set to true to replace every occurrence when old_text occurs more than once", Default: false},
			},
		},
	}
}

func (c *FilesCapability) Handle(_ context.Context, action string, params map[string]any) (ActionResult, error) {
	var result ActionResult
	var err error
	switch action {
	case "list":
		result, err = c.handleList(params)
	case "read":
		result, err = c.handleRead(params)
	case "write":
		result, err = c.handleWrite(params)
	case "edit":
		result, err = c.handleEdit(params)
	default:
		return ActionResult{}, fmt.Errorf("unknown file action: %s", action)
	}
	return result, nextStep(err)
}

// nextStep adds the tool to call next to a workspace error that has an
// obvious one. The workspace package states what is wrong and knows no tool
// names, so other tool sets can use it.
func nextStep(err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist), errors.Is(err, syscall.ENOTDIR):
		return fmt.Errorf("%v; call file_list to see what is in the workspace", err)
	case errors.Is(err, workspace.ErrIsFolder):
		return fmt.Errorf("%v; list it with file_list", err)
	}
	return err
}

func (c *FilesCapability) Close() error { return nil }

// fileEntry is the JSON shape of one listed file or folder.
type fileEntry struct {
	Path     string `json:"path"`
	Type     string `json:"type"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
}

func (c *FilesCapability) handleList(params map[string]any) (ActionResult, error) {
	rel, err := c.ws.Resolve(getString(params, "path"))
	if err != nil {
		return ActionResult{}, err
	}
	depth := min(max(intParam(params, "depth", defaultListDepth), 1), maxListDepth)
	if info, err := c.ws.Stat(rel); err != nil {
		return ActionResult{}, err
	} else if !info.IsDir() {
		return ActionResult{}, fmt.Errorf("%s is a file, not a folder; read it with file_read", rel)
	}

	var entries []fileEntry
	truncated := false
	var walk func(dir string, level int) error
	walk = func(dir string, level int) error {
		children, err := c.ws.ReadDir(dir)
		if err != nil {
			return err
		}
		slices.SortFunc(children, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
		for _, child := range children {
			if strings.HasPrefix(child.Name(), ".") {
				continue
			}
			childPath := path.Join(dir, child.Name())
			// Stat follows a link that stays inside the workspace and fails
			// on one that leaves it, which is then not listed.
			info, err := c.ws.Stat(childPath)
			if err != nil || !(info.IsDir() || info.Mode().IsRegular()) {
				continue
			}
			if len(entries) == maxListEntries {
				truncated = true
				return nil
			}
			entry := fileEntry{Path: childPath, Type: "file", Size: info.Size(), Modified: info.ModTime().Format("2006-01-02 15:04")}
			if info.IsDir() {
				entry.Type, entry.Size = "folder", 0
			}
			entries = append(entries, entry)
			if info.IsDir() && level < depth {
				if err := walk(childPath, level+1); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk(rel, 1); err != nil {
		return ActionResult{}, err
	}

	switch {
	case len(entries) == 0 && rel == ".":
		// An empty result reads as a failed call to a small model.
		return ActionResult{Text: "The workspace is empty. Save a file with file_write."}, nil
	case len(entries) == 0:
		return ActionResult{Text: fmt.Sprintf("The folder %s is empty.", rel)}, nil
	case truncated:
		return ActionResult{Data: entries, Text: fmt.Sprintf("Only the first %d entries are shown. List one folder to see the rest, for example path=%s", maxListEntries, firstFolder(entries))}, nil
	}
	return ActionResult{Data: entries}, nil
}

func firstFolder(entries []fileEntry) string {
	for _, entry := range entries {
		if entry.Type == "folder" {
			return entry.Path
		}
	}
	return "notes"
}

func (c *FilesCapability) handleRead(params map[string]any) (ActionResult, error) {
	rel, content, err := c.readText(params)
	if err != nil {
		return ActionResult{}, err
	}
	if content == "" {
		return ActionResult{Text: fmt.Sprintf("[%s is empty]", rel)}, nil
	}
	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	total := len(lines)
	start := intParam(params, "start_line", 1)
	end := intParam(params, "end_line", total)
	switch {
	case start < 1:
		return ActionResult{}, fmt.Errorf("start_line must be 1 or more; %s has %d lines", rel, total)
	case start > total:
		return ActionResult{}, fmt.Errorf("start_line %d is past the end; %s has %d lines", start, rel, total)
	case end < start:
		return ActionResult{}, fmt.Errorf("end_line %d is before start_line %d", end, start)
	}
	end = min(end, total)

	var text strings.Builder
	last := start - 1
	for _, line := range lines[start-1 : end] {
		if text.Len() > 0 && text.Len()+len(line) > maxReadBytes {
			break
		}
		text.WriteString(line)
		last++
	}
	switch {
	case text.Len() > maxReadBytes:
		// One line longer than the cap, as in a minified page. Lines are the
		// only unit the tool offers, so the rest of this one is out of reach.
		cut := text.String()[:maxReadBytes]
		for !utf8.ValidString(cut) {
			cut = cut[:len(cut)-1]
		}
		note := fmt.Sprintf("%s line %d of %d is %d bytes long; only its first %d bytes are shown.", rel, start, total, text.Len(), len(cut))
		if start < total {
			note += fmt.Sprintf(" Call file_read again with start_line=%d for the lines after it.", start+1)
		}
		return ActionResult{Text: cut + "\n[" + note + "]"}, nil
	case last < end:
		return ActionResult{Text: fmt.Sprintf("%s\n[%s lines %d-%d of %d. Call file_read again with start_line=%d to read more.]", strings.TrimSuffix(text.String(), "\n"), rel, start, last, total, last+1)}, nil
	case start > 1 || end < total:
		return ActionResult{Text: fmt.Sprintf("%s\n[%s lines %d-%d of %d]", strings.TrimSuffix(text.String(), "\n"), rel, start, last, total)}, nil
	}
	return ActionResult{Text: text.String()}, nil
}

// fileChange is the JSON shape write and edit return. The link is ready-made
// Markdown because a 4B model handed a bare URL often put it in backticks,
// which Open WebUI does not make clickable.
type fileChange struct {
	Path         string `json:"path"`
	Bytes        int    `json:"bytes"`
	Replacements int    `json:"replacements,omitempty"`
	Link         string `json:"link,omitempty"`
}

func (c *FilesCapability) handleWrite(params map[string]any) (ActionResult, error) {
	rel, err := c.requiredPath(params)
	if err != nil {
		return ActionResult{}, err
	}
	content, ok := params["content"].(string)
	if !ok {
		return ActionResult{}, fmt.Errorf("missing required parameter: content (the complete text of the file)")
	}
	if err := c.ws.WriteFile(rel, []byte(content)); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Data: c.change(rel, len(content), 0)}, nil
}

func (c *FilesCapability) handleEdit(params map[string]any) (ActionResult, error) {
	rel, content, err := c.readText(params)
	if err != nil {
		return ActionResult{}, err
	}
	oldText, _ := params["old_text"].(string)
	newText, ok := params["new_text"].(string)
	switch {
	case oldText == "":
		return ActionResult{}, fmt.Errorf("missing required parameter: old_text (the exact text to replace). To replace the whole file use file_write.")
	case !ok:
		return ActionResult{}, fmt.Errorf("missing required parameter: new_text (use an empty string to delete old_text)")
	case oldText == newText:
		return ActionResult{}, fmt.Errorf("old_text and new_text are the same, so nothing would change")
	}

	count := strings.Count(content, oldText)
	replaceAll := boolParam(params, "replace_all")
	switch {
	case count == 0:
		return ActionResult{}, fmt.Errorf("old_text was not found in %s.%s It must match the file exactly, including spaces and line breaks. Call file_read and copy the text from there.", rel, nearMiss(content, oldText))
	case count > 1 && !replaceAll:
		return ActionResult{}, fmt.Errorf("old_text occurs %d times in %s. Add more of the surrounding text so it matches once, or set replace_all to true to replace all %d.", count, rel, count)
	}
	updated := strings.ReplaceAll(content, oldText, newText)
	if err := c.ws.WriteFile(rel, []byte(updated)); err != nil {
		return ActionResult{}, err
	}
	return ActionResult{Data: c.change(rel, len(updated), count)}, nil
}

func (c *FilesCapability) change(rel string, size, replacements int) fileChange {
	change := fileChange{Path: rel, Bytes: size, Replacements: replacements}
	if c.baseURL != "" {
		text := strings.NewReplacer("[", `\[`, "]", `\]`).Replace(rel)
		change.Link = fmt.Sprintf("[%s](%s)", text, workspace.URL(c.baseURL, rel))
	}
	return change
}

// nearMiss names the file line that equals the first line of oldText once
// leading and trailing spaces are ignored. A small model's failed edit is
// nearly always wrong indentation, and seeing the real line saves it from
// reading the file again in pieces.
func nearMiss(content, oldText string) string {
	want := strings.TrimSpace(oldText)
	want, _, _ = strings.Cut(want, "\n")
	want = strings.TrimSpace(want)
	if want == "" {
		return ""
	}
	for i, line := range strings.Split(content, "\n") {
		if strings.TrimSpace(line) == want && len(line) <= 300 {
			return fmt.Sprintf(" Line %d is close, with different spacing: %q.", i+1, strings.TrimRight(line, "\r"))
		}
	}
	return ""
}

func (c *FilesCapability) requiredPath(params map[string]any) (string, error) {
	name := getString(params, "path")
	if name == "" {
		return "", fmt.Errorf("missing required parameter: path, for example: notes/page.html")
	}
	return c.ws.Resolve(name)
}

// readText loads the text file params name, for read and edit.
func (c *FilesCapability) readText(params map[string]any) (rel, content string, err error) {
	rel, err = c.requiredPath(params)
	if err != nil {
		return "", "", err
	}
	data, err := c.ws.ReadFile(rel, maxReadFileBytes)
	if err != nil {
		return "", "", err
	}
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return "", "", fmt.Errorf("%s is not a text file (it is binary or not UTF-8); these tools read text only", rel)
	}
	return rel, string(data), nil
}

// intParam reads a whole number a client sent as a JSON number or, as small
// models sometimes do, as a string.
func intParam(params map[string]any, key string, fallback int) int {
	switch value := params[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
			return parsed
		}
	}
	return fallback
}

func boolParam(params map[string]any, key string) bool {
	switch value := params[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	}
	return false
}
