package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHTMLPackWritesOneFile(t *testing.T) {
	drive, work := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(work, "rows.csv"), []byte("a,b\n1,2\n"), 0o644)
	os.WriteFile(filepath.Join(work, "page.html"), []byte(`<html><body><script type="module">import t from "./rows.csv"; document.body.textContent = t;</script></body></html>`), 0o644)
	var stdout, stderr bytes.Buffer
	err := runHTML(context.Background(), &stdout, &stderr, drive, []string{"pack", filepath.Join(work, "page.html"), "-o", filepath.Join(work, "out.html")})
	if err != nil {
		t.Fatalf("%v: %s", err, stderr.String())
	}
	if !strings.Contains(stdout.String(), "out.html (") || !strings.Contains(stdout.String(), "single-file)") {
		t.Errorf("stdout = %q", stdout.String())
	}
	if data, _ := os.ReadFile(filepath.Join(work, "out.html")); !strings.Contains(string(data), "a,b") {
		t.Errorf("out.html = %s", data)
	}
}

func TestHTMLPackPrintsProblemsAndFails(t *testing.T) {
	drive, work := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(work, "page.html"), []byte(`<html><head><script src="https://cdn.jsdelivr.net/npm/chart.js"></script></head></html>`), 0o644)
	var stdout, stderr bytes.Buffer
	err := runHTML(context.Background(), &stdout, &stderr, drive, []string{"pack", filepath.Join(work, "page.html")})
	if err == nil || !strings.Contains(stderr.String(), "remote-resource") || !strings.Contains(stderr.String(), "cdn.jsdelivr.net") {
		t.Fatalf("err = %v, stderr = %q", err, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(work, "page.packed.html")); !os.IsNotExist(err) {
		t.Error("a page with problems was written")
	}
}

func TestHTMLUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := runHTML(context.Background(), &stdout, &stderr, t.TempDir(), []string{"build"}); err == nil || !strings.Contains(stderr.String(), "html pack <page.html>") {
		t.Errorf("err = %v, stderr = %q", err, stderr.String())
	}
	if err := runHTML(context.Background(), &stdout, &stderr, t.TempDir(), []string{"pack"}); err == nil {
		t.Error("pack without a page did not fail")
	}
}
