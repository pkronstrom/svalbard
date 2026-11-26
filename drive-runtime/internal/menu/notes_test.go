package menu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/tui"
)

func documentsConfig(labels ...string) config.RuntimeConfig {
	items := []config.MenuItem{
		{ID: "wikem", Label: "WikEM", Subheader: "Archives", Action: config.BuiltinAction("browse", map[string]string{"zim": "wikem.zim"})},
	}
	for _, label := range labels {
		items = append(items, config.MenuItem{ID: label, Label: label, Subheader: "Documents", Note: "5 documents", Action: config.BuiltinAction("documents", map[string]string{"path": label})})
	}
	items = append(items, config.MenuItem{ID: "who", Label: "WHO Basic Emergency Care", Subheader: "Documents", Action: config.BuiltinAction("documents", map[string]string{"path": "who.pdf"})})
	return config.RuntimeConfig{Version: 2, Groups: []config.MenuGroup{{ID: "library", Label: "Library", Items: items}}}
}

func groupView(t *testing.T, cfg config.RuntimeConfig, width, selected int) []string {
	t.Helper()
	m := NewModel(cfg, t.TempDir())
	updated, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
	m = updated.(Model)
	m.inGroup = true
	m.activeGroup = cfg.Groups[0].ID
	m.itemSelected = selected
	return strings.Split(m.View(), "\n")
}

func TestGroupRowsShowTheirNotesInOneColumn(t *testing.T) {
	lines := groupView(t, documentsConfig("Car manuals", "Field manuals"), 80, 1)

	want := []string{
		"    WikEM",
		"Documents",
		"  > Car manuals     5 documents",
		"    Field manuals   5 documents",
		"    WHO Basic Emergency Care",
	}
	start := -1
	for i, line := range lines {
		if line == want[0] {
			start = i
		}
	}
	if start < 0 || start+len(want) > len(lines) {
		t.Fatalf("view = %q", lines)
	}
	for i, line := range want {
		if got := strings.TrimRight(lines[start+i], " "); got != line {
			t.Errorf("row %d = %q, want %q", i, got, line)
		}
	}
}

func TestGroupRowNoteNeverWrapsTheRow(t *testing.T) {
	long := "Service and repair manuals for every car, van and tractor the household has owned"
	lines := groupView(t, documentsConfig("Car manuals", long), 80, 0)

	var short, wide string
	for _, line := range lines {
		switch {
		case strings.Contains(line, "Car manuals"):
			short = line
		case strings.Contains(line, "Service and repair"):
			wide = line
		}
	}
	// The long label does not move the column the other rows use.
	if got := strings.TrimRight(short, " "); got != "    Car manuals   5 documents" {
		t.Errorf("short row = %q", got)
	}
	// A row that has no room for its note goes without it.
	if strings.Contains(wide, "documents") {
		t.Errorf("long row keeps a note that cannot fit: %q", wide)
	}

	// With room, a label past the column carries its note right after it.
	for _, line := range groupView(t, documentsConfig("Car manuals", long), 120, 0) {
		if strings.Contains(line, "Service and repair") && !strings.HasSuffix(strings.TrimRight(line, " "), "owned   5 documents") {
			t.Errorf("long row at 120 columns = %q", line)
		}
	}
}

func TestContextForLibraryCountsDocumentsApart(t *testing.T) {
	cfg := documentsConfig("Car manuals", "Field manuals")
	detail := contextForGroup(cfg.Groups[0], tui.DefaultTheme())

	if len(detail.Fields) != 2 {
		t.Fatalf("Fields = %v, want archives and documents", detail.Fields)
	}
	if detail.Fields[0].Label != "Archives" || detail.Fields[0].Value != "1" {
		t.Errorf("Fields[0] = %v, want Archives 1", detail.Fields[0])
	}
	if detail.Fields[1].Label != "Documents" || detail.Fields[1].Value != "3" {
		t.Errorf("Fields[1] = %v, want Documents 3", detail.Fields[1])
	}
}

func TestWorkspaceEntryWithoutAFolderSaysSoInTheMenu(t *testing.T) {
	drive := t.TempDir()
	workspace := config.MenuItem{ID: "workspace", Label: "Workspace", Action: config.BuiltinAction("workspace", nil)}
	m := NewModel(config.RuntimeConfig{}, drive)

	updated, cmd := m.activateItem(workspace)
	got := updated.(Model)
	if cmd != nil {
		t.Fatal("the action was started for a workspace that does not exist")
	}
	if got.lastErr != nil || !strings.Contains(got.status, "Nothing in the workspace yet") {
		t.Errorf("status = %q, err = %v; want the one-line explanation", got.status, got.lastErr)
	}

	if err := os.Mkdir(filepath.Join(drive, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	updated, cmd = m.activateItem(workspace)
	if cmd == nil || updated.(Model).status != "" {
		t.Errorf("with the folder present the action should start: cmd nil = %v, status = %q", cmd == nil, updated.(Model).status)
	}
}
