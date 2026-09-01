package builder

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestBlockCacheKeyChangesWithInputContent(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input")
	if err := os.MkdirAll(input, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(input, "page.html")
	if err := os.WriteFile(file, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	procedure := Procedure{ID: "render", Fingerprint: "params", Inputs: []string{input}}
	first, _, err := blockCacheKey(procedure)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, _, err := blockCacheKey(procedure)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("cache key did not change with input content")
	}
}

func TestStoreAndRestoreBlockCacheCopiesIndependentOutput(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input")
	output := filepath.Join(root, "output")
	if err := os.MkdirAll(input, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(input, "source"), []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "artifact"), []byte("built"), 0o644); err != nil {
		t.Fatal(err)
	}
	procedure := Procedure{ID: "build", Fingerprint: "params", Inputs: []string{input}, Outputs: []string{output}}
	key, inputs, err := blockCacheKey(procedure)
	if err != nil {
		t.Fatal(err)
	}
	if err := storeBlockCache(root, key, procedure, inputs, output); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(output); err != nil {
		t.Fatal(err)
	}
	hit, err := restoreBlockCache(root, key, output)
	if err != nil || !hit {
		t.Fatalf("cache hit = %t, err = %v", hit, err)
	}
	if data, err := os.ReadFile(filepath.Join(output, "artifact")); err != nil || string(data) != "built" {
		t.Fatalf("restored artifact = %q, err = %v", data, err)
	}
	if err := os.RemoveAll(filepath.Join(root, ".staging", "cache")); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(output, "artifact")); err != nil || string(data) != "built" {
		t.Fatalf("artifact depends on cache: %q, err = %v", data, err)
	}
}

func TestRestoreRejectsInvalidManifest(t *testing.T) {
	root := t.TempDir()
	key := "invalid"
	entry := filepath.Join(root, ".staging", "cache", key)
	if err := os.MkdirAll(entry, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entry, "manifest.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	hit, err := restoreBlockCache(root, key, filepath.Join(root, "output"))
	if err != nil || hit {
		t.Fatalf("cache hit = %t, err = %v", hit, err)
	}
}

func TestBlockDockerArgsMountsOnlyDeclaredPaths(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "input")
	output := filepath.Join(root, "output")
	work := filepath.Join(root, ".staging", "work")
	for _, dir := range []string{input, output, work} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	args, err := blockDockerArgs(root, work, output, []string{input}, []string{filepath.Join(input, "page")})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{root + ":/vault", root + ":/vault:rw"} {
		for _, arg := range args {
			if arg == forbidden {
				t.Fatalf("broad vault mount present: %v", args)
			}
		}
	}
	translated := translateBlockArgs(work, output, []string{input}, []string{filepath.Join(input, "page"), filepath.Join(output, "artifact")})
	if translated[0] != "/input/0/page" || translated[1] != "/output/artifact" {
		t.Fatalf("translated args = %v", translated)
	}
}

func TestRunLinearCachesDirectoryOutputByInputContent(t *testing.T) {
	root := t.TempDir()
	workdir := filepath.Join(root, ".staging", "build", "archive")
	archive := filepath.Join(root, "source.zip")
	output := filepath.Join(workdir, "output")
	writeZip(t, archive, "page.txt", "first")
	compile := func() []Procedure {
		procedures, err := CompileProcedures([]catalog.BuildStep{{
			Extract: archive, Dest: output,
			Inputs: []string{archive}, Outputs: []string{output},
		}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		return procedures
	}

	var first, second []EventState
	if err := RunLinear(context.Background(), root, workdir, "archive", compile(), Options{OnEvent: func(event BuildEvent) {
		first = append(first, event.State)
	}}); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(output); err != nil {
		t.Fatal(err)
	}
	if err := RunLinear(context.Background(), root, workdir, "archive", compile(), Options{OnEvent: func(event BuildEvent) {
		second = append(second, event.State)
	}}); err != nil {
		t.Fatal(err)
	}
	if want := []EventState{EventQueued, EventSkipped}; !reflect.DeepEqual(second, want) {
		t.Fatalf("cache-hit states = %v, want %v; first = %v", second, want, first)
	}
	if data, err := os.ReadFile(filepath.Join(output, "page.txt")); err != nil || string(data) != "first" {
		t.Fatalf("restored output = %q, err = %v", data, err)
	}

	writeZip(t, archive, "page.txt", "second")
	if err := RunLinear(context.Background(), root, workdir, "archive", compile(), Options{}); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(output, "page.txt")); err != nil || string(data) != "second" {
		t.Fatalf("rebuilt output = %q, err = %v", data, err)
	}
}

func writeZip(t *testing.T, path, name, content string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	writer := zip.NewWriter(file)
	entry, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := entry.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
