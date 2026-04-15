package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsHTMLRedirectMatchesDotAndDotDot(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		target  string
		isRedir bool
	}{
		{
			name:    "dot slash",
			data:    []byte(`<meta http-equiv="refresh" content="0;URL='./Target_Page'">`),
			target:  "Target_Page",
			isRedir: true,
		},
		{
			name:    "dot dot slash",
			data:    []byte(`<meta http-equiv="refresh" content="0;URL='../Target_Page'">`),
			target:  "Target_Page",
			isRedir: true,
		},
		{
			name:    "not a redirect",
			data:    []byte(`<html><body>Hello</body></html>`),
			target:  "",
			isRedir: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, isRedir := isHTMLRedirect(tt.data)
			if target != tt.target || isRedir != tt.isRedir {
				t.Fatalf("got (%q, %v), want (%q, %v)", target, isRedir, tt.target, tt.isRedir)
			}
		})
	}
}

func TestWriteIllustrationReturnsErrorWhenParentIsFile(t *testing.T) {
	tmpDir := t.TempDir()
	blockingPath := filepath.Join(tmpDir, "blocked")
	if err := os.WriteFile(blockingPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}

	if err := writeIllustration(blockingPath, []byte("png")); err == nil {
		t.Fatal("expected error")
	}
}

func TestWriteRedirectsReturnsErrorWhenParentIsFile(t *testing.T) {
	tmpDir := t.TempDir()
	blockingPath := filepath.Join(tmpDir, "blocked")
	if err := os.WriteFile(blockingPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}

	redirectsPath := filepath.Join(blockingPath, "redirects.tsv")
	redirects := []redirectEntry{{path: "A", title: "A", targetPath: "B"}}
	if err := writeRedirects(redirectsPath, redirects); err == nil {
		t.Fatal("expected error")
	}
}
