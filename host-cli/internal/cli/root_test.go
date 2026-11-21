package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	hosttui "github.com/pkronstrom/svalbard/host-tui"

	"github.com/pkronstrom/svalbard/host-cli/internal/commands"
)

func TestApplyCommandPreservesDurableApplyWhenIndexingFails(t *testing.T) {
	root := t.TempDir()
	if err := commands.InitVaultWithOptions(root, nil, "selected", "fi", nil); err != nil {
		t.Fatal(err)
	}
	// An unmanaged corrupt archive makes indexing fail after acquisition saves.
	if err := os.MkdirAll(filepath.Join(root, "zim"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "zim", "broken.zim"), []byte("invalid archive"), 0644); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"apply", "--vault", root})
	cmd.SetOut(&output)
	cmd.SetErr(io.Discard)
	err := cmd.ExecuteContext(context.Background())
	if err == nil || !strings.Contains(err.Error(), "apply completed but indexing failed") {
		t.Fatalf("post-apply error = %v", err)
	}
	saved, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if saved.Realized.AppliedAt == "" || saved.Desired.Presets[0] != "selected" {
		t.Fatalf("index failure lost durable apply state: %+v", saved)
	}
	if strings.Contains(output.String(), "Apply complete") {
		t.Fatalf("index failure printed success: %q", output.String())
	}
}

func TestDashboardShouldIndexOnlySkipsExplicitNone(t *testing.T) {
	root := t.TempDir()
	if err := commands.InitVaultWithOptions(root, nil, "", "", nil); err != nil {
		t.Fatal(err)
	}
	deps := buildDashboardDeps(root, nil)
	for _, strategy := range []string{"", "full-text", "full", "none", " NONE ", "unknown"} {
		m, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		m.Desired.Options.IndexStrategy = strategy
		if err := manifest.Save(filepath.Join(root, "manifest.yaml"), m); err != nil {
			t.Fatal(err)
		}
		want := !strings.EqualFold(strings.TrimSpace(strategy), "none")
		if got := deps.ShouldIndex(root); got != want {
			t.Fatalf("strategy %q: ShouldIndex = %v, want %v", strategy, got, want)
		}
	}
	if !deps.ShouldIndex(t.TempDir()) {
		t.Fatal("missing manifest must reach runner error reporting")
	}
}

func TestNewRootCommandHasHardResetCommands(t *testing.T) {
	cmd := NewRootCommand()
	got := map[string]bool{}
	for _, child := range cmd.Commands() {
		got[child.Name()] = true
	}
	for _, name := range []string{"init", "add", "remove", "plan", "apply", "import", "preset", "status", "index"} {
		if !got[name] {
			t.Fatalf("missing command %q", name)
		}
	}
}

func TestMapSemanticIndexEventPreservesGlobalEvents(t *testing.T) {
	ev := mapSemanticIndexEvent(commands.SemanticProgress{
		Status: "starting",
		Detail: "Starting embedding server...",
	}, true)

	if ev.File != "" {
		t.Fatalf("global semantic event file = %q, want empty", ev.File)
	}
	if ev.Detail != "Starting embedding server..." {
		t.Fatalf("global semantic event detail = %q", ev.Detail)
	}
}

func TestApplyCommandAndDashboardAdapterPreserveCancellation(t *testing.T) {
	root := t.TempDir()
	if err := commands.InitVaultWithOptions(root, nil, "selected", "fi", nil); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"apply", "--vault", root})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.ExecuteContext(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("CLI command context lost: %v", err)
	}
	deps := buildDashboardDeps(root, nil)
	if err := deps.RunApply(ctx, func(hosttui.ApplyEvent) {}); !errors.Is(err, context.Canceled) {
		t.Fatalf("dashboard context lost: %v", err)
	}
	saved, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if saved.Realized.AppliedAt != "" || saved.Desired.Presets[0] != "selected" {
		t.Fatalf("cancelled command changed state: %+v", saved)
	}
}

type fixtureTransport func(*http.Request) (*http.Response, error)

func (f fixtureTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestApplyCommandInterruptSavesPartialBeforeReturning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sending interrupt to this process is unavailable on Windows")
	}
	t.Setenv("SVALBARD_CACHE_DIRS", "")
	cat, err := catalog.LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	recipe, ok := cat.RecipeByID("qwen-0.8b")
	if !ok {
		t.Fatal("fixture recipe unavailable")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"fixture"`)
		w.Header().Set("Content-Length", "100000")
		_, _ = w.Write([]byte("partial bytes"))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	original := http.DefaultTransport
	http.DefaultTransport = fixtureTransport(func(r *http.Request) (*http.Response, error) {
		copy := r.Clone(r.Context())
		copy.URL = endpoint
		copy.Host = endpoint.Host
		return original.RoundTrip(copy)
	})
	t.Cleanup(func() { http.DefaultTransport = original })
	root := t.TempDir()
	if err := commands.InitVaultWithOptions(root, []string{recipe.ID}, "selected", "fi", nil); err != nil {
		t.Fatal(err)
	}
	source, err := url.Parse(recipe.URL)
	if err != nil {
		t.Fatal(err)
	}
	final := filepath.Join(root, "models", filepath.Base(source.Path))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := NewRootCommand()
	cmd.SetArgs([]string{"apply", "--vault", root})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	done := make(chan error, 1)
	go func() { done <- cmd.ExecuteContext(ctx) }()
	for {
		if info, err := os.Stat(final + ".part"); err == nil && info.Size() > 0 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("CLI never persisted partial bytes")
		case <-time.After(5 * time.Millisecond):
		}
	}
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("interrupt cause lost: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("interrupt did not finish saving")
	}
	saved, err := manifest.Load(filepath.Join(root, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Realized.Entries) != 0 || saved.Realized.AppliedAt != "" || len(saved.Desired.Items) != 1 {
		t.Fatalf("interrupted CLI saved incorrect state: %+v", saved)
	}
	if _, err := os.Stat(final); !os.IsNotExist(err) {
		t.Fatalf("interrupted bytes exposed as installed: %v", err)
	}
	if _, err := os.Stat(final + ".download.json"); err != nil {
		t.Fatalf("recovery identity lost: %v", err)
	}
}

func TestCustomizedPresetSnapshotSurvivesCatalogChangeAndReopen(t *testing.T) {
	configRoot := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	presetDir := filepath.Join(configRoot, "svalbard", "presets")
	if err := os.MkdirAll(presetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	presetPath := filepath.Join(presetDir, "phase3-snapshot.yaml")
	const presetText = "name: phase3-snapshot\nregion: custom\nkind: preset\nsources: [dufs, wikiciv]\n"
	if err := os.WriteFile(presetPath, []byte(presetText), 0o644); err != nil {
		t.Fatal(err)
	}
	vault := filepath.Join(t.TempDir(), "vault")
	cfg, err := buildWizardConfig(vault, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	var preset hosttui.PresetOption
	for _, candidate := range cfg.Presets {
		if candidate.Name == "phase3-snapshot" {
			preset = candidate
		}
	}
	if !preset.Custom || !reflect.DeepEqual(preset.SourceIDs, []string{"dufs", "wikiciv"}) {
		t.Fatalf("local preset not correctly offered: %+v", preset)
	}
	// Remove wikiciv and add a different ZIM. The checked host callback adds
	// its required tool; false selections must not survive this boundary.
	explicit := map[string]bool{"dufs": true, "wikiciv": false, "permacomputing": true}
	auto, missing := cfg.ResolveDeps(explicit)
	if len(missing) != 0 || !reflect.DeepEqual(auto, map[string]bool{"kiwix-serve": true}) {
		t.Fatalf("incorrect dependency expansion: auto=%v missing=%v", auto, missing)
	}
	var confirmed []string
	for id, selected := range explicit {
		if selected {
			confirmed = append(confirmed, id)
		}
	}
	for id := range auto {
		confirmed = append(confirmed, id)
	}
	sort.Strings(confirmed)
	want := []string{"dufs", "kiwix-serve", "permacomputing"}
	if !reflect.DeepEqual(confirmed, want) {
		t.Fatalf("confirmed IDs = %v, want %v", confirmed, want)
	}
	// Every recipe must be represented in the UI even if it has no pack.
	visible := make(map[string]bool)
	for _, group := range cfg.PackGroups {
		for _, pack := range group.Packs {
			for _, source := range pack.Sources {
				visible[source.ID] = true
			}
		}
	}
	for _, id := range confirmed {
		if !visible[id] {
			t.Fatalf("confirmed recipe %q is hidden from picker/review", id)
		}
	}
	if _, err := os.Stat(filepath.Join(vault, "manifest.yaml")); !os.IsNotExist(err) {
		t.Fatalf("configuration preparation wrote a manifest: %v", err)
	}
	deps := buildDashboardDeps(vault, &cfg)
	platforms := []string{"macos-arm64", "linux-x86_64"}
	if err := deps.InitVault(vault, confirmed, preset.Name, preset.Region, platforms); err != nil {
		t.Fatal(err)
	}
	saved, err := manifest.Load(filepath.Join(vault, "manifest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved.Desired.Items, want) ||
		!reflect.DeepEqual(saved.Desired.Presets, []string{preset.Name}) ||
		saved.Desired.Options.Region != preset.Region ||
		!reflect.DeepEqual(saved.Desired.Options.HostPlatforms, platforms) {
		t.Fatalf("init did not save the reviewed snapshot/options: %+v", saved.Desired)
	}
	if len(saved.Realized.Entries) != 0 || saved.Realized.AppliedAt != "" {
		t.Fatalf("init prematurely realized selections: %+v", saved.Realized)
	}
	original, err := os.ReadFile(presetPath)
	if err != nil || string(original) != presetText {
		t.Fatalf("customization changed the preset file: %q, %v", original, err)
	}
	// Catalog changes are not desired-state edits. A fresh host config now
	// offers the removed item, but reopening must retain the saved snapshot.
	if err := os.WriteFile(presetPath, []byte("name: phase3-snapshot\nregion: custom\nkind: preset\nsources: [wikiciv]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	newCfg, err := buildWizardConfig(vault, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	changed := false
	for _, candidate := range newCfg.Presets {
		changed = changed || (candidate.Name == preset.Name && reflect.DeepEqual(candidate.SourceIDs, []string{"wikiciv"}))
	}
	if !changed {
		t.Fatal("fixture did not actually change the offered preset")
	}
	reopened := buildDashboardDeps(vault, &newCfg)
	loaded, err := reopened.LoadDesiredItems()
	if err != nil || !reflect.DeepEqual(loaded, want) {
		t.Fatalf("reopen expanded the changed preset: %v, %v", loaded, err)
	}
	pending, err := reopened.LoadPlan()
	if err != nil {
		t.Fatal(err)
	}
	var pendingIDs []string
	for _, item := range pending.ToDownload {
		pendingIDs = append(pendingIDs, item.ID)
	}
	if !reflect.DeepEqual(pendingIDs, want) || len(pending.ToRemove) != 0 {
		t.Fatalf("reopened pending plan differs from saved selections: %+v", pending)
	}
	status, err := reopened.LoadStatus()
	if err != nil || status.DesiredCount != len(want) || status.PendingCount != len(want) || status.RealizedCount != 0 {
		t.Fatalf("reopened status disagrees with saved selections: %+v, %v", status, err)
	}
}
