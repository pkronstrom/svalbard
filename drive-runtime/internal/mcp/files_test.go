package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/workspace"
)

const filesBaseURL = "http://127.0.0.1:8086"

// newFilesCapability returns the file tools on a fresh drive, the workspace
// folder's host path, and a directory outside the drive nothing may reach.
func newFilesCapability(t *testing.T) (cap *mcp.FilesCapability, dir, outside string) {
	t.Helper()
	ws, err := workspace.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close() })
	outside = t.TempDir()
	writeTestFile(t, filepath.Join(outside, "secret.txt"), []byte("secret"))
	return mcp.NewFilesCapability(ws, filesBaseURL), ws.Dir(), outside
}

// callFile runs one file action and returns its result as the client sees it:
// the JSON data, then any text.
func callFile(t *testing.T, cap *mcp.FilesCapability, action string, params map[string]any) (string, error) {
	t.Helper()
	result, err := cap.Handle(context.Background(), action, params)
	if err != nil {
		return "", err
	}
	if result.Data == nil {
		return result.Text, nil
	}
	data, err := json.Marshal(result.Data)
	if err != nil {
		t.Fatal(err)
	}
	return string(data) + result.Text, nil
}

func mustCallFile(t *testing.T, cap *mcp.FilesCapability, action string, params map[string]any) string {
	t.Helper()
	out, err := callFile(t, cap, action, params)
	if err != nil {
		t.Fatalf("%s(%v) error = %v", action, params, err)
	}
	return out
}

func wantFileError(t *testing.T, cap *mcp.FilesCapability, action string, params map[string]any, wantInError ...string) {
	t.Helper()
	out, err := callFile(t, cap, action, params)
	if err == nil {
		t.Errorf("%s(%v) = %q, want an error", action, params, out)
		return
	}
	for _, want := range wantInError {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s(%v) error = %q, want it to mention %q", action, params, err, want)
		}
	}
}

func TestFilesCapabilityActions(t *testing.T) {
	cap, _, _ := newFilesCapability(t)
	if cap.Tool() != "file" {
		t.Errorf("tool name = %q, want file", cap.Tool())
	}
	writes := map[string]bool{}
	for _, action := range cap.Actions() {
		writes[action.Name] = action.Writes
		// Each description carries an example a small model can copy.
		for _, param := range action.Params {
			if param.Required && param.Name != "content" && !strings.Contains(param.Desc, "for example") {
				t.Errorf("%s.%s description has no example: %q", action.Name, param.Name, param.Desc)
			}
		}
	}
	// No delete, move or execute.
	want := map[string]bool{"list": false, "read": false, "write": true, "edit": true}
	if fmt.Sprint(writes) != fmt.Sprint(want) {
		t.Errorf("actions = %v, want %v", writes, want)
	}
	if _, err := cap.Handle(context.Background(), "delete", map[string]any{"path": "page.html"}); err == nil {
		t.Error("unknown action succeeded")
	}
}

func TestFileWriteCreatesTheFileAndReturnsItsLink(t *testing.T) {
	cap, dir, _ := newFilesCapability(t)

	// The link is Markdown the model can paste into its reply as it is.
	out := mustCallFile(t, cap, "write", map[string]any{"path": "reports/first aid.html", "content": "<h1>Bürns</h1>"})
	if want := `{"path":"reports/first aid.html","bytes":15,"link":"[reports/first aid.html](http://127.0.0.1:8086/reports/first%20aid.html)"}`; out != want {
		t.Errorf("write = %s\nwant    %s", out, want)
	}
	// Brackets and parentheses in a name must not end the link early.
	out = mustCallFile(t, cap, "write", map[string]any{"path": "draft [v2] (1).html", "content": "x"})
	if want := `"link":"[draft \\[v2\\] (1).html](http://127.0.0.1:8086/draft%20%5Bv2%5D%20%281%29.html)"`; !strings.Contains(out, want) {
		t.Errorf("write = %s\nwant it to contain %s", out, want)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "reports", "first aid.html")); err != nil || string(data) != "<h1>Bürns</h1>" {
		t.Errorf("file = %q, %v", data, err)
	}

	// A second write replaces the whole file.
	mustCallFile(t, cap, "write", map[string]any{"path": "./reports/first aid.html", "content": ""})
	if data, _ := os.ReadFile(filepath.Join(dir, "reports", "first aid.html")); len(data) != 0 {
		t.Errorf("file after replacement = %q, want it empty", data)
	}

	wantFileError(t, cap, "write", map[string]any{"content": "x"}, "missing required parameter: path")
	wantFileError(t, cap, "write", map[string]any{"path": "page.html"}, "missing required parameter: content")
	wantFileError(t, cap, "write", map[string]any{"path": "reports", "content": "x"}, "is a folder")
	wantFileError(t, cap, "write", map[string]any{"path": "big.txt", "content": strings.Repeat("x", workspace.MaxWriteBytes+1)}, "limit", "shorter")
}

func TestFileToolsRejectEveryPathOutOfTheWorkspace(t *testing.T) {
	cap, dir, outside := newFilesCapability(t)
	writeTestFile(t, filepath.Join(dir, "notes", "page.html"), []byte("page"))
	writeTestFile(t, filepath.Join(filepath.Dir(dir), "manifest.yaml"), []byte("drive"))
	for link, target := range map[string]string{
		"out":             outside,
		"secret-link.txt": filepath.Join(outside, "secret.txt"),
		"dangling.txt":    filepath.Join(outside, "new.txt"),
		"notes/deep":      outside,
	} {
		if err := os.Symlink(target, filepath.Join(dir, link)); err != nil {
			t.Fatal(err)
		}
	}

	for _, path := range []string{
		"/etc/passwd",
		filepath.Join(outside, "secret.txt"),
		"../manifest.yaml",
		"notes/../../manifest.yaml",
		`..\manifest.yaml`,
		"out/secret.txt",
		"out/new.txt",
		"out/new/folder/page.html",
		"secret-link.txt",
		"dangling.txt",
		"notes/deep/secret.txt",
		".hidden",
	} {
		wantFileError(t, cap, "read", map[string]any{"path": path})
		wantFileError(t, cap, "write", map[string]any{"path": path, "content": "escaped"})
		wantFileError(t, cap, "edit", map[string]any{"path": path, "old_text": "secret", "new_text": "escaped"})
		wantFileError(t, cap, "list", map[string]any{"path": path})
	}

	if data, _ := os.ReadFile(filepath.Join(outside, "secret.txt")); string(data) != "secret" {
		t.Errorf("file outside the workspace changed to %q", data)
	}
	if entries, _ := os.ReadDir(outside); len(entries) != 1 {
		t.Errorf("directory outside the workspace now holds %d entries, want 1", len(entries))
	}
	if data, _ := os.ReadFile(filepath.Join(filepath.Dir(dir), "manifest.yaml")); string(data) != "drive" {
		t.Errorf("the drive's manifest changed to %q", data)
	}
	// Links that leave the workspace are not listed either.
	if out := mustCallFile(t, cap, "list", map[string]any{"depth": 5}); strings.Contains(out, "out") || strings.Contains(out, "secret") || strings.Contains(out, "deep") {
		t.Errorf("list shows a link out of the workspace: %s", out)
	}
}

func TestFileListReturnsEntriesToABoundedDepthAndCount(t *testing.T) {
	cap, dir, _ := newFilesCapability(t)
	if out := mustCallFile(t, cap, "list", map[string]any{}); !strings.Contains(out, "workspace is empty") {
		t.Errorf("list of an empty workspace = %q", out)
	}

	writeTestFile(t, filepath.Join(dir, "page.html"), []byte("12345"))
	writeTestFile(t, filepath.Join(dir, "notes", "a.md"), []byte("a"))
	writeTestFile(t, filepath.Join(dir, "notes", "deep", "b.md"), []byte("bb"))
	writeTestFile(t, filepath.Join(dir, "notes", "deep", "deeper", "c.md"), []byte("c"))
	writeTestFile(t, filepath.Join(dir, ".DS_Store"), []byte("x"))
	if err := os.MkdirAll(filepath.Join(dir, "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	type entry struct {
		Path, Type, Modified string
		Size                 int64
	}
	list := func(params map[string]any) []string {
		t.Helper()
		var entries []entry
		if err := json.Unmarshal([]byte(mustCallFile(t, cap, "list", params)), &entries); err != nil {
			t.Fatal(err)
		}
		var lines []string
		for _, e := range entries {
			if len(e.Modified) != len("2026-11-26 15:04") {
				t.Errorf("%s modified = %q", e.Path, e.Modified)
			}
			lines = append(lines, fmt.Sprintf("%s %s %d", e.Type, e.Path, e.Size))
		}
		return lines
	}

	// The default goes two levels down; hidden files are left out.
	want := []string{"folder empty 0", "folder notes 0", "file notes/a.md 1", "folder notes/deep 0", "file page.html 5"}
	if got := list(map[string]any{}); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("list = %v\nwant   %v", got, want)
	}
	if got := list(map[string]any{"depth": float64(1)}); len(got) != 3 {
		t.Errorf("list depth 1 = %v, want the three top entries", got)
	}
	// A model may send the number as a string, and any depth is capped.
	if got := list(map[string]any{"depth": "99"}); len(got) != 8 {
		t.Errorf("list depth 99 = %v, want all eight entries", got)
	}
	want = []string{"file notes/deep/b.md 2", "folder notes/deep/deeper 0"}
	if got := list(map[string]any{"path": "notes/deep", "depth": float64(1)}); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("list notes/deep = %v\nwant              %v", got, want)
	}

	if out := mustCallFile(t, cap, "list", map[string]any{"path": "empty"}); !strings.Contains(out, "empty is empty") {
		t.Errorf("list of an empty folder = %q", out)
	}
	wantFileError(t, cap, "list", map[string]any{"path": "page.html"}, "is a file", "file_read")
	wantFileError(t, cap, "list", map[string]any{"path": "missing"}, "does not exist")

	for i := range 250 {
		writeTestFile(t, filepath.Join(dir, "many", fmt.Sprintf("%03d.txt", i)), nil)
	}
	out := mustCallFile(t, cap, "list", map[string]any{"path": "many"})
	if got := strings.Count(out, `"path"`); got != 200 {
		t.Errorf("list returned %d entries, want the cap of 200", got)
	}
	if !strings.Contains(out, "Only the first 200 entries are shown") {
		t.Errorf("capped list does not say so: %s", out[len(out)-120:])
	}
}

func TestFileReadReturnsTextWholeOrByLineRange(t *testing.T) {
	cap, dir, _ := newFilesCapability(t)
	writeTestFile(t, filepath.Join(dir, "page.html"), []byte("<h1>one</h1>\n<p>two</p>\n<p>three</p>\n"))
	writeTestFile(t, filepath.Join(dir, "empty.txt"), nil)
	if err := os.MkdirAll(filepath.Join(dir, "notes"), 0o755); err != nil {
		t.Fatal(err)
	}

	// A whole file comes back byte for byte, so it can be copied into an edit.
	if out := mustCallFile(t, cap, "read", map[string]any{"path": "page.html"}); out != "<h1>one</h1>\n<p>two</p>\n<p>three</p>\n" {
		t.Errorf("read = %q", out)
	}
	if out := mustCallFile(t, cap, "read", map[string]any{"path": "page.html", "start_line": float64(2), "end_line": float64(2)}); out != "<p>two</p>\n[page.html lines 2-2 of 3]" {
		t.Errorf("read line 2 = %q", out)
	}
	if out := mustCallFile(t, cap, "read", map[string]any{"path": "page.html", "start_line": "2", "end_line": float64(99)}); out != "<p>two</p>\n<p>three</p>\n[page.html lines 2-3 of 3]" {
		t.Errorf("read from line 2 = %q", out)
	}
	if out := mustCallFile(t, cap, "read", map[string]any{"path": "empty.txt"}); out != "[empty.txt is empty]" {
		t.Errorf("read of an empty file = %q", out)
	}

	wantFileError(t, cap, "read", map[string]any{}, "missing required parameter: path")
	wantFileError(t, cap, "read", map[string]any{"path": "missing.html"}, "does not exist", "file_list")
	wantFileError(t, cap, "read", map[string]any{"path": "notes"}, "is a folder", "file_list")
	wantFileError(t, cap, "read", map[string]any{"path": "page.html", "start_line": float64(4)}, "past the end", "3 lines")
	wantFileError(t, cap, "read", map[string]any{"path": "page.html", "start_line": float64(0)}, "1 or more")
	wantFileError(t, cap, "read", map[string]any{"path": "page.html", "start_line": float64(3), "end_line": float64(2)}, "before start_line")
}

func TestFileReadCapsLongFilesAndSaysHowToContinue(t *testing.T) {
	cap, dir, _ := newFilesCapability(t)
	var content strings.Builder
	for i := 1; i <= 2000; i++ {
		fmt.Fprintf(&content, "line %04d of a long file with some text on it\n", i)
	}
	writeTestFile(t, filepath.Join(dir, "long.txt"), []byte(content.String()))

	// Following the note from part to part reassembles the file exactly.
	var rebuilt strings.Builder
	parts := 0
	for start := 1; start != 0; parts++ {
		if parts > 20 {
			t.Fatal("read never reached the end of the file")
		}
		out := mustCallFile(t, cap, "read", map[string]any{"path": "long.txt", "start_line": float64(start)})
		text, note, found := strings.Cut(out, "\n[long.txt lines ")
		if !found {
			t.Fatalf("part %d has no line note: %q", parts, out[max(0, len(out)-100):])
		}
		if len(text) > 20_000 {
			t.Errorf("part %d is %d bytes, over the cap", parts, len(text))
		}
		rebuilt.WriteString(text + "\n")
		var first, last, total, next int
		if _, err := fmt.Sscanf(note, "%d-%d of %d. Call file_read again with start_line=%d to read more.]", &first, &last, &total, &next); err != nil {
			// The last part ends the file and names no next line.
			if _, err := fmt.Sscanf(note, "%d-%d of %d]", &first, &last, &total); err != nil || last != 2000 {
				t.Fatalf("part %d note = %q", parts, note)
			}
			next = 0
		} else if next != last+1 {
			t.Errorf("part %d note = %q, want the next line to follow the last", parts, note)
		}
		if first != start || total != 2000 {
			t.Errorf("part %d note = %q, want it to start at %d of 2000", parts, note, start)
		}
		start = next
	}
	if parts < 2 {
		t.Errorf("file came back in %d part, want several", parts)
	}
	if rebuilt.String() != content.String() {
		t.Error("the parts do not add up to the file")
	}

	// A single line over the cap is cut on a character boundary and says so.
	writeTestFile(t, filepath.Join(dir, "minified.html"), []byte(strings.Repeat("ä", 15_000)))
	out := mustCallFile(t, cap, "read", map[string]any{"path": "minified.html"})
	text, note, _ := strings.Cut(out, "\n[")
	if len(text) > 20_000 || len(text) < 19_990 || strings.Trim(text, "ä") != "" {
		t.Errorf("long line came back as %d bytes, want just under the cap and whole characters", len(text))
	}
	if !strings.Contains(note, "30000 bytes long") || !strings.Contains(note, "only its first") || strings.Contains(note, "start_line") {
		t.Errorf("long line note = %q", note)
	}
	// With more lines after it, the note says how to go on.
	writeTestFile(t, filepath.Join(dir, "minified.html"), []byte(strings.Repeat("a", 25_000)+"\nsecond line\n"))
	out = mustCallFile(t, cap, "read", map[string]any{"path": "minified.html"})
	if !strings.HasSuffix(out, "Call file_read again with start_line=2 for the lines after it.]") {
		t.Errorf("long first line note = %q", out[len(out)-160:])
	}
	if out := mustCallFile(t, cap, "read", map[string]any{"path": "minified.html", "start_line": float64(2)}); out != "second line\n[minified.html lines 2-2 of 2]" {
		t.Errorf("read after the long line = %q", out)
	}
}

func TestFileReadRefusesBinaryFiles(t *testing.T) {
	cap, dir, _ := newFilesCapability(t)
	writeTestFile(t, filepath.Join(dir, "image.png"), []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"))
	writeTestFile(t, filepath.Join(dir, "latin1.txt"), []byte("caf\xe9"))
	writeTestFile(t, filepath.Join(dir, "huge.txt"), make([]byte, 8<<20+1))

	for _, path := range []string{"image.png", "latin1.txt"} {
		wantFileError(t, cap, "read", map[string]any{"path": path}, "not a text file")
		wantFileError(t, cap, "edit", map[string]any{"path": path, "old_text": "PNG", "new_text": "x"}, "not a text file")
	}
	wantFileError(t, cap, "read", map[string]any{"path": "huge.txt"}, "limit for reading")
	if data, _ := os.ReadFile(filepath.Join(dir, "image.png")); !strings.HasPrefix(string(data), "\x89PNG") {
		t.Error("binary file changed")
	}
}

func TestFileEditReplacesOneExactOccurrence(t *testing.T) {
	cap, dir, _ := newFilesCapability(t)
	page := "<h1>Draft</h1>\n<p>one</p>\n<p>one</p>\n"
	path := filepath.Join(dir, "notes", "page.html")
	writeTestFile(t, path, []byte(page))
	read := func() string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	edit := func(oldText, newText string, extra map[string]any) map[string]any {
		params := map[string]any{"path": "notes/page.html", "old_text": oldText, "new_text": newText}
		for key, value := range extra {
			params[key] = value
		}
		return params
	}

	out := mustCallFile(t, cap, "edit", edit("<h1>Draft</h1>", "<h1>Final</h1>", nil))
	if want := `{"path":"notes/page.html","bytes":37,"replacements":1,"link":"[notes/page.html](http://127.0.0.1:8086/notes/page.html)"}`; out != want {
		t.Errorf("edit = %s\nwant   %s", out, want)
	}
	if got := read(); got != "<h1>Final</h1>\n<p>one</p>\n<p>one</p>\n" {
		t.Errorf("file = %q", got)
	}

	// Absent or ambiguous text changes nothing and says what to do.
	before := read()
	wantFileError(t, cap, "edit", edit("<h1>Draft</h1>", "x", nil), "was not found", "file_read")
	wantFileError(t, cap, "edit", edit("<p>one</p>", "<p>two</p>", nil), "occurs 2 times", "replace_all")
	wantFileError(t, cap, "edit", edit("<p>one</p>", "<p>two</p>", map[string]any{"replace_all": false}), "occurs 2 times")
	wantFileError(t, cap, "edit", edit("", "x", nil), "old_text", "file_write")
	wantFileError(t, cap, "edit", edit("Final", "Final", nil), "the same")
	wantFileError(t, cap, "edit", map[string]any{"path": "notes/page.html", "old_text": "Final"}, "new_text")
	wantFileError(t, cap, "edit", map[string]any{"path": "missing.html", "old_text": "a", "new_text": "b"}, "does not exist")
	wantFileError(t, cap, "edit", edit("Final", strings.Repeat("x", workspace.MaxWriteBytes), nil), "limit")
	if got := read(); got != before {
		t.Errorf("a failed edit changed the file to %q", got)
	}

	// Wrong indentation is the usual miss; the error shows the real line.
	mustCallFile(t, cap, "write", map[string]any{"path": "indented.html", "content": "<body>\n    <h1>Title</h1>\n</body>\n"})
	wantFileError(t, cap, "edit", map[string]any{"path": "indented.html", "old_text": "        <h1>Title</h1>\n</body>", "new_text": "x"},
		"was not found", `Line 2 is close, with different spacing: "    <h1>Title</h1>"`)

	// More surrounding text makes one of two occurrences unique.
	mustCallFile(t, cap, "edit", edit("</h1>\n<p>one</p>", "</h1>\n<p>first</p>", nil))
	if got := read(); got != "<h1>Final</h1>\n<p>first</p>\n<p>one</p>\n" {
		t.Errorf("file = %q", got)
	}

	mustCallFile(t, cap, "write", map[string]any{"path": "notes/page.html", "content": "a b a b a"})
	out = mustCallFile(t, cap, "edit", edit("a", "c", map[string]any{"replace_all": "true"}))
	if !strings.Contains(out, `"replacements":3`) || read() != "c b c b c" {
		t.Errorf("replace_all = %s, file %q", out, read())
	}
	// An empty new_text deletes.
	mustCallFile(t, cap, "edit", edit(" b", "", map[string]any{"replace_all": true}))
	if got := read(); got != "c c c" {
		t.Errorf("file = %q", got)
	}
}

func TestFileToolsReturnNoLinkWithoutAServedWorkspace(t *testing.T) {
	ws, err := workspace.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	cap := mcp.NewFilesCapability(ws, "")
	if out := mustCallFile(t, cap, "write", map[string]any{"path": "page.html", "content": "x"}); out != `{"path":"page.html","bytes":1}` {
		t.Errorf("write = %s", out)
	}
}
