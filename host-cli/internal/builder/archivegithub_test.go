package builder

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestArchiveRepoLinkReadsRepositoriesAndFolders(t *testing.T) {
	for _, test := range []struct {
		link string
		want archiveRepo
		ok   bool
	}{
		{"https://github.com/dekuNukem/duckyPad", archiveRepo{Owner: "dekuNukem", Name: "duckyPad"}, true},
		{"https://github.com/o/r.git", archiveRepo{Owner: "o", Name: "r"}, true},
		{"https://github.com/o/r/releases/latest", archiveRepo{Owner: "o", Name: "r"}, true},
		{"https://github.com/diysynth/STANDALONE-DEVICES/tree/main/LIL", archiveRepo{Owner: "diysynth", Name: "STANDALONE-DEVICES", Ref: "main", Dir: "LIL"}, true},
		{"https://github.com/o/r/blob/main/README.md", archiveRepo{}, false},
		{"https://github.com/o/r/issues", archiveRepo{}, false},
		{"https://github.com/sponsors/o", archiveRepo{}, false},
		{"https://github.com/FundamentalFrequency", archiveRepo{}, false},
		{"https://gitlab.com/o/r", archiveRepo{}, false},
	} {
		target, _ := url.Parse(test.link)
		withArchiveEndpoint(t, &githubWeb, "https://github.com")
		got, ok := archiveRepoLink(target)
		if ok != test.ok || got.Owner != test.want.Owner || got.Name != test.want.Name || got.Ref != test.want.Ref || got.Dir != test.want.Dir {
			t.Errorf("archiveRepoLink(%s) = %+v, %v; want %+v, %v", test.link, got, ok, test.want, test.ok)
		}
	}
}

func TestArchiveRepoSelectionKeepsBuildFilesBeforePictures(t *testing.T) {
	blobs := []githubTreeEntry{
		{Path: "resources/pics/teaser.gif", Size: 18 << 20},
		{Path: "resources/old_firmware.zip", Size: 19 << 20},
		{Path: "pcb/board.kicad_pcb", Size: 2 << 20},
		{Path: "firmware/main.c", Size: 40 << 10},
		{Path: "README.md", Size: 20 << 10},
		{Path: "case/top.stl", Size: 30 << 20},
		{Path: "photo.png", Size: 300 << 10},
		{Path: "docs/build/assembly.md", Size: 1 << 10},
		{Path: "pcb/old/v1.kicad_pcb", Size: 1 << 10},
		{Path: "pcb/gerbers.zip", Size: 1 << 20},
		{Path: "pcb/libs/switch.pretty/mx.kicad_mod", Size: 1 << 10},
		{Path: "video/demo.mp4", Size: 1 << 20},
	}
	selected, omitted := archiveRepoSelection(blobs, 35<<20)
	var kept, left []string
	for _, entry := range selected {
		kept = append(kept, entry.Path)
	}
	for _, entry := range omitted {
		left = append(left, entry.Path)
	}
	if want := []string{"README.md", "docs/build/assembly.md", "firmware/main.c", "pcb/gerbers.zip", "pcb/board.kicad_pcb", "case/top.stl", "photo.png", "pcb/old/v1.kicad_pcb", "pcb/libs/switch.pretty/mx.kicad_mod"}; !slices.Equal(kept, want) {
		t.Errorf("selected = %v, want %v", kept, want)
	}
	if want := []string{"resources/old_firmware.zip", "resources/pics/teaser.gif", "video/demo.mp4"}; !slices.Equal(left, want) {
		t.Errorf("omitted = %v, want %v", left, want)
	}
}

func TestArchiveReaderPacksLinkedRepositories(t *testing.T) {
	var codeload bytes.Buffer
	packer := zip.NewWriter(&codeload)
	for name, content := range map[string]string{"small-main/README.md": "# Small", "small-main/case.scad": "cube(1);", "small-main/.github/ci.yml": "ci"} {
		entry, _ := packer.Create(name)
		_, _ = entry.Write([]byte(content))
	}
	_ = packer.Close()
	var github *httptest.Server
	github = httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		reply := func(value any) { _ = json.NewEncoder(writer).Encode(value) }
		switch request.URL.Path {
		case "/api/repos/o/small":
			reply(map[string]any{"default_branch": "main"})
		case "/api/repos/o/small/git/trees/main":
			reply(map[string]any{"tree": []githubTreeEntry{{Path: "README.md", Type: "blob", Size: 7}, {Path: "case.scad", Type: "blob", Size: 8}}})
		case "/api/repos/o/small/releases/latest":
			reply(map[string]any{"assets": []map[string]any{{"name": "case.stl", "size": 10, "browser_download_url": github.URL + "/download/case.stl"}}})
		case "/download/case.stl":
			_, _ = writer.Write([]byte("solid case"))
		case "/codeload/o/small/zip/main":
			_, _ = writer.Write(codeload.Bytes())
		case "/api/repos/o/big/git/trees/main:hw":
			reply(map[string]any{"tree": []githubTreeEntry{
				{Path: "notes.md", Type: "blob", Size: 100}, {Path: "board.kicad_pcb", Type: "blob", Size: 1000}, {Path: "case.step", Type: "blob", Size: 130},
				{Path: "demo.gif", Type: "blob", Size: 30 << 20}, {Path: "gerbers", Type: "tree"},
			}})
		case "/raw/o/big/main/hw/notes.md":
			_, _ = writer.Write([]byte("# Notes"))
		case "/raw/o/big/main/hw/board.kicad_pcb":
			_, _ = writer.Write([]byte("(kicad_pcb)"))
		case "/raw/o/big/main/hw/case.step":
			_, _ = writer.Write([]byte("version https://git-lfs.github.com/spec/v1\noid sha256:abc\nsize 9\n"))
		case "/media/o/big/main/hw/case.step":
			_, _ = writer.Write([]byte("ISO-10303"))
		case "/o/small":
			writer.Header().Set("Content-Type", "text/html")
			_, _ = writer.Write([]byte(`<html><head><title>o/small</title></head><body><article>` + strings.Repeat("Solder the switches. ", 20) + `</article></body></html>`))
		default:
			http.NotFound(writer, request)
		}
	}))
	defer github.Close()
	withArchiveEndpoint(t, &githubWeb, github.URL)
	withArchiveEndpoint(t, &githubAPI, github.URL+"/api")
	withArchiveEndpoint(t, &githubRaw, github.URL+"/raw")
	withArchiveEndpoint(t, &githubCodeload, github.URL+"/codeload")
	withArchiveEndpoint(t, &githubMedia, github.URL+"/media")
	site := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "text/html")
		_, _ = writer.Write([]byte(`<html><head><title>Pad</title></head><body><article><p>` + strings.Repeat("Build the pad. ", 30) + `</p>
			<a href="` + github.URL + `/o/small">Code</a> <a href="` + github.URL + `/o/big/tree/main/hw">Hardware</a></article></body></html>`))
	}))
	defer site.Close()

	target, _ := url.Parse(site.URL + "/pad")
	output := t.TempDir()
	pages, err := archiveReader(newArchiveFetcher(context.Background()), newArchiveSource(target, 1), output, testArchiveSite, nil)
	if err != nil {
		t.Fatal(err)
	}
	page := readArchivePage(t, filepath.Join(output, "pad.html"))
	for _, want := range []string{"<h2>o/small</h2>", "Solder the switches.", "<h3>Repositories</h3>", "whole repository", "3 of 4 files; 1 left out (30.0 MB)"} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q: %s", want, page)
		}
	}
	zips := make(map[string][]string)
	for _, stored := range pages[1:] {
		reader, err := zip.OpenReader(filepath.Join(output, filepath.FromSlash(stored.Path)))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range reader.File {
			zips[filepath.Base(stored.Path)] = append(zips[filepath.Base(stored.Path)], file.Name)
		}
		for _, file := range reader.File {
			if file.Name == "case.step" {
				body, _ := file.Open()
				content, _ := io.ReadAll(body)
				if string(content) != "ISO-10303" {
					t.Errorf("case.step holds %q, want the LFS file", content)
				}
			}
		}
		reader.Close()
		if !strings.Contains(page, `href="`+stored.Path+`"`) {
			t.Errorf("page does not link %s from its anchor: %s", stored.Path, page)
		}
	}
	for name, want := range map[string][]string{"-small.zip": {"README.md", "case.scad", "releases/case.stl"}, "-big-hw.zip": {"notes.md", "board.kicad_pcb", "case.step", "OMITTED.txt"}} {
		found := false
		for stored, entries := range zips {
			if strings.HasSuffix(stored, name) {
				found = true
				slices.Sort(entries)
				slices.Sort(want)
				if !slices.Equal(entries, want) {
					t.Errorf("%s holds %v, want %v", stored, entries, want)
				}
			}
		}
		if !found {
			t.Errorf("no zip ending %s among %v", name, zips)
		}
	}
	if _, err := os.Stat(filepath.Join(output, "_files")); err != nil {
		t.Fatal(err)
	}
}
