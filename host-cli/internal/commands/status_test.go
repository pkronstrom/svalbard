package commands

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestWriteStatusShowsAllItems(t *testing.T) {
	m := manifest.New("test-vault")
	m.Desired.Presets = []string{"default-32"}
	m.Desired.Items = []string{"wikipedia-en-nopic", "ifixit"}
	m.Realized.Entries = []manifest.RealizedEntry{
		{ID: "wikipedia-en-nopic", Type: "zim", SizeBytes: 4500000000},
	}

	var buf bytes.Buffer
	if err := WriteStatus(&buf, m); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	if !strings.Contains(out, "test-vault") {
		t.Error("missing vault name")
	}
	if !strings.Contains(out, "wikipedia-en-nopic") {
		t.Error("missing realized item")
	}
	if !strings.Contains(out, "ifixit") {
		t.Error("missing pending item")
	}
	if !strings.Contains(out, "realized") {
		t.Error("missing realized status")
	}
	if !strings.Contains(out, "pending") {
		t.Error("missing pending status")
	}
}

func TestWriteStatusEmptyVault(t *testing.T) {
	m := manifest.New("empty")
	var buf bytes.Buffer
	if err := WriteStatus(&buf, m); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Desired: 0") {
		t.Error("should show 0 desired")
	}
}

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "—"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{4500000000, "4.2 GB"},
	}
	for _, tt := range tests {
		got := humanSize(tt.bytes)
		if got != tt.want {
			t.Errorf("humanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}
