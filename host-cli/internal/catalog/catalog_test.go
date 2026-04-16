package catalog

import (
	"testing"
)

func assertRealCatalog(t *testing.T, cat *Catalog) {
	t.Helper()

	names := cat.PresetNames()
	if len(names) < 10 {
		t.Fatalf("expected real catalog with many presets, got %d", len(names))
	}

	for _, want := range []string{"default-32", "default-64", "finland-32"} {
		found := false
		for _, name := range names {
			if name == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected preset %q to exist; presets: %v", want, names)
		}
	}

	item, ok := cat.RecipeByID("wikipedia-en-nopic")
	if !ok {
		t.Fatal("expected to find recipe wikipedia-en-nopic")
	}

	if item.Type != "zim" {
		t.Errorf("Type: expected %q, got %q", "zim", item.Type)
	}
	if item.SizeGB != 48.0 {
		t.Errorf("SizeGB: expected %f, got %f", 48.0, item.SizeGB)
	}
	if item.URLPattern != "https://download.kiwix.org/zim/wikipedia/wikipedia_en_all_nopic_{date}.zim" {
		t.Errorf("URLPattern: unexpected value %q", item.URLPattern)
	}
	if item.Description != "English Wikipedia without pictures" {
		t.Errorf("Description: expected %q, got %q", "English Wikipedia without pictures", item.Description)
	}
	if item.License == nil {
		t.Fatal("License: expected non-nil for wikipedia-en-nopic")
	}
	if item.License.ID != "CC-BY-SA-3.0" {
		t.Errorf("License.ID: expected %q, got %q", "CC-BY-SA-3.0", item.License.ID)
	}
	if len(item.Tags) < 5 {
		t.Errorf("Tags: expected several tags, got %d", len(item.Tags))
	}
	if item.Menu == nil {
		t.Error("Menu: expected non-nil for wikipedia-en-nopic")
	}
}

// --- Struct field parsing tests ---

func TestPresetParsesNewFields(t *testing.T) {
	cat := NewTestCatalog(t)

	preset, ok := cat.presets["test-pack"]
	if !ok {
		t.Fatal("expected test-pack preset to exist")
	}
	if preset.Kind != "pack" {
		t.Errorf("Kind: expected %q, got %q", "pack", preset.Kind)
	}
	if preset.DisplayGroup != "Tools" {
		t.Errorf("DisplayGroup: expected %q, got %q", "Tools", preset.DisplayGroup)
	}
	if preset.Description != "Test pack with a small recipe" {
		t.Errorf("Description: expected %q, got %q", "Test pack with a small recipe", preset.Description)
	}
	if preset.TargetSizeGB != 1 {
		t.Errorf("TargetSizeGB: expected 1, got %f", preset.TargetSizeGB)
	}
}

func TestPresetParsesExtends(t *testing.T) {
	cat := NewTestCatalog(t)

	preset, ok := cat.presets["default-32"]
	if !ok {
		t.Fatal("expected default-32 preset to exist")
	}
	if len(preset.Extends) != 1 || preset.Extends[0] != "test-pack" {
		t.Errorf("Extends: expected [test-pack], got %v", preset.Extends)
	}
}

// --- Extends resolution tests ---

func TestResolvePresetExpandsRecipeIDs(t *testing.T) {
	cat := NewTestCatalog(t)

	preset, err := cat.ResolvePreset("default-32")
	if err != nil {
		t.Fatalf("ResolvePreset: %v", err)
	}

	if len(preset.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(preset.Items))
	}

	ids := map[string]bool{}
	for _, item := range preset.Items {
		ids[item.ID] = true
	}

	if !ids["wikipedia-en-nopic"] {
		t.Error("expected item wikipedia-en-nopic")
	}
	if !ids["ifixit"] {
		t.Error("expected item ifixit")
	}
}

func TestResolvePresetFlattensExtendsChain(t *testing.T) {
	cat := NewTestCatalog(t)

	// default-128 extends default-32 which extends test-pack.
	// test-pack has [ifixit]
	// default-32 has [wikipedia-en-nopic, ifixit] (ifixit deduped from test-pack)
	// default-128 has own [ifixit] which is already inherited
	// Final: ifixit, wikipedia-en-nopic (inherited from chain, deduped)
	preset, err := cat.ResolvePreset("default-128")
	if err != nil {
		t.Fatalf("ResolvePreset: %v", err)
	}

	ids := map[string]bool{}
	for _, item := range preset.Items {
		ids[item.ID] = true
	}

	if !ids["wikipedia-en-nopic"] {
		t.Error("expected item wikipedia-en-nopic from extends chain")
	}
	if !ids["ifixit"] {
		t.Error("expected item ifixit from extends chain")
	}
	if len(preset.Items) != 2 {
		t.Errorf("expected 2 deduplicated items, got %d", len(preset.Items))
	}
}

func TestResolvePresetDeduplicatesSources(t *testing.T) {
	cat := NewTestCatalog(t)

	// default-32 extends test-pack (which has ifixit), and also lists ifixit directly.
	// ifixit should appear only once.
	preset, err := cat.ResolvePreset("default-32")
	if err != nil {
		t.Fatalf("ResolvePreset: %v", err)
	}

	count := 0
	for _, item := range preset.Items {
		if item.ID == "ifixit" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected ifixit exactly once, got %d", count)
	}
}

func TestResolvePresetToleratesMissingRecipes(t *testing.T) {
	cat := NewTestCatalog(t)

	// Add a preset that references a nonexistent recipe.
	cat.presets["with-missing"] = Preset{
		Name:    "with-missing",
		Sources: []string{"nonexistent-recipe", "ifixit"},
	}

	preset, err := cat.ResolvePreset("with-missing")
	if err != nil {
		t.Fatalf("expected no error for missing recipe, got: %v", err)
	}
	if len(preset.Items) != 1 {
		t.Fatalf("expected 1 item (skipping missing), got %d", len(preset.Items))
	}
	if preset.Items[0].ID != "ifixit" {
		t.Errorf("expected ifixit, got %q", preset.Items[0].ID)
	}
}

func TestResolvePresetDetectsCircularExtends(t *testing.T) {
	cat := NewTestCatalog(t)

	// Create a circular extends chain.
	cat.presets["circular-a"] = Preset{
		Name:    "circular-a",
		Extends: []string{"circular-b"},
		Sources: []string{"ifixit"},
	}
	cat.presets["circular-b"] = Preset{
		Name:    "circular-b",
		Extends: []string{"circular-a"},
		Sources: []string{"ifixit"},
	}

	_, err := cat.ResolvePreset("circular-a")
	if err == nil {
		t.Fatal("expected error for circular extends, got nil")
	}
}

func TestResolvePresetHandlesDiamondExtends(t *testing.T) {
	cat := NewTestCatalog(t)

	// Diamond: D extends B and C, both B and C extend A.
	cat.presets["diamond-a"] = Preset{
		Name:    "diamond-a",
		Sources: []string{"ifixit"},
	}
	cat.presets["diamond-b"] = Preset{
		Name:    "diamond-b",
		Extends: []string{"diamond-a"},
		Sources: []string{"wikipedia-en-nopic"},
	}
	cat.presets["diamond-c"] = Preset{
		Name:    "diamond-c",
		Extends: []string{"diamond-a"},
	}
	cat.presets["diamond-d"] = Preset{
		Name:    "diamond-d",
		Extends: []string{"diamond-b", "diamond-c"},
	}

	preset, err := cat.ResolvePreset("diamond-d")
	if err != nil {
		t.Fatalf("expected diamond extends to work, got: %v", err)
	}

	ids := map[string]bool{}
	for _, item := range preset.Items {
		ids[item.ID] = true
	}
	if !ids["ifixit"] {
		t.Error("expected ifixit from diamond ancestor")
	}
	if !ids["wikipedia-en-nopic"] {
		t.Error("expected wikipedia-en-nopic from diamond-b")
	}
	if len(preset.Items) != 2 {
		t.Errorf("expected 2 deduplicated items, got %d", len(preset.Items))
	}
}

func TestResolvePresetToleratesMissingExtendsPreset(t *testing.T) {
	cat := NewTestCatalog(t)

	cat.presets["extends-missing"] = Preset{
		Name:    "extends-missing",
		Extends: []string{"does-not-exist"},
		Sources: []string{"ifixit"},
	}

	preset, err := cat.ResolvePreset("extends-missing")
	if err != nil {
		t.Fatalf("expected no error when extends target is missing, got: %v", err)
	}
	if len(preset.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(preset.Items))
	}
}

// --- Packs tests ---

func TestPacksReturnsPacks(t *testing.T) {
	cat := NewTestCatalog(t)

	packs := cat.Packs()
	if len(packs) != 1 {
		t.Fatalf("expected 1 pack, got %d", len(packs))
	}
	if packs[0].Name != "test-pack" {
		t.Errorf("expected pack name %q, got %q", "test-pack", packs[0].Name)
	}
	if packs[0].Kind != "pack" {
		t.Errorf("expected kind %q, got %q", "pack", packs[0].Kind)
	}
}

func TestPacksSortedByDisplayGroupThenName(t *testing.T) {
	cat := NewTestCatalog(t)

	// Add more packs for sort testing.
	cat.presets["zeta-pack"] = Preset{Name: "zeta-pack", Kind: "pack", DisplayGroup: "Alpha"}
	cat.presets["alpha-pack"] = Preset{Name: "alpha-pack", Kind: "pack", DisplayGroup: "Alpha"}
	cat.presets["beta-pack"] = Preset{Name: "beta-pack", Kind: "pack", DisplayGroup: "Beta"}

	packs := cat.Packs()
	if len(packs) != 4 {
		t.Fatalf("expected 4 packs, got %d", len(packs))
	}

	// Expected order: Alpha/alpha-pack, Alpha/zeta-pack, Beta/beta-pack, Tools/test-pack
	expected := []string{"alpha-pack", "zeta-pack", "beta-pack", "test-pack"}
	for i, want := range expected {
		if packs[i].Name != want {
			t.Errorf("packs[%d]: expected %q, got %q", i, want, packs[i].Name)
		}
	}
}

// --- PresetsForRegion tests ---

func TestPresetsForRegionFilters(t *testing.T) {
	cat := NewTestCatalog(t)

	presets := cat.PresetsForRegion("default")
	// default-32 (32GB) and default-128 (128GB) are non-pack, non-test, region=default
	if len(presets) != 2 {
		t.Fatalf("expected 2 presets for default region, got %d", len(presets))
	}
	// Should be sorted by TargetSizeGB.
	if presets[0].Name != "default-32" {
		t.Errorf("expected first preset %q, got %q", "default-32", presets[0].Name)
	}
	if presets[1].Name != "default-128" {
		t.Errorf("expected second preset %q, got %q", "default-128", presets[1].Name)
	}
}

func TestPresetsForRegionExcludesTestPrefix(t *testing.T) {
	cat := NewTestCatalog(t)

	presets := cat.PresetsForRegion("default")
	for _, p := range presets {
		if p.Name == "test-small" {
			t.Error("expected test-small to be excluded (test- prefix)")
		}
	}
}

func TestPresetsForRegionExcludesPacks(t *testing.T) {
	cat := NewTestCatalog(t)

	presets := cat.PresetsForRegion("default")
	for _, p := range presets {
		if p.Kind == "pack" {
			t.Errorf("expected packs to be excluded, found pack %q", p.Name)
		}
	}
}

func TestPresetsForRegionSortedByTargetSizeGB(t *testing.T) {
	cat := NewTestCatalog(t)

	presets := cat.PresetsForRegion("default")
	for i := 1; i < len(presets); i++ {
		if presets[i].TargetSizeGB < presets[i-1].TargetSizeGB {
			t.Errorf("presets not sorted by TargetSizeGB: %q (%.0f) before %q (%.0f)",
				presets[i-1].Name, presets[i-1].TargetSizeGB,
				presets[i].Name, presets[i].TargetSizeGB)
		}
	}
}

// --- Regions tests ---

func TestRegionsReturnsDistinctSorted(t *testing.T) {
	cat := NewTestCatalog(t)

	regions := cat.Regions()
	if len(regions) != 2 {
		t.Fatalf("expected 2 regions, got %d: %v", len(regions), regions)
	}
	if regions[0] != "default" {
		t.Errorf("expected first region %q, got %q", "default", regions[0])
	}
	if regions[1] != "finland" {
		t.Errorf("expected second region %q, got %q", "finland", regions[1])
	}
}

func TestRegionsExcludesTestPresets(t *testing.T) {
	cat := NewTestCatalog(t)

	// test-small has region=default, but it should not be the only source
	// of "default" region. Let's verify test-only regions don't leak through.
	// Add a test-only preset with a unique region.
	cat.presets["test-special"] = Preset{
		Name:   "test-special",
		Region: "special-test-region",
	}

	regions := cat.Regions()
	for _, r := range regions {
		if r == "special-test-region" {
			t.Error("expected test-only region to be excluded")
		}
	}
}

func TestRegionsExcludesPacks(t *testing.T) {
	cat := NewTestCatalog(t)

	// Add a pack with a region (unusual but possible).
	cat.presets["region-pack"] = Preset{
		Name:   "region-pack",
		Kind:   "pack",
		Region: "pack-only-region",
	}

	regions := cat.Regions()
	for _, r := range regions {
		if r == "pack-only-region" {
			t.Error("expected pack region to be excluded")
		}
	}
}

// --- ContentSizeGB tests ---

func TestContentSizeGBSumsItems(t *testing.T) {
	p := Preset{
		Items: []Item{
			{ID: "a", SizeGB: 1.5},
			{ID: "b", SizeGB: 2.5},
			{ID: "c", SizeGB: 0.5},
		},
	}
	got := p.ContentSizeGB()
	want := 4.5
	if got != want {
		t.Errorf("ContentSizeGB: expected %f, got %f", want, got)
	}
}

func TestContentSizeGBZeroWhenNoItems(t *testing.T) {
	p := Preset{}
	if got := p.ContentSizeGB(); got != 0 {
		t.Errorf("ContentSizeGB: expected 0, got %f", got)
	}
}

// --- Existing tests (updated for new test fixtures) ---

func TestPresetNamesReturnsSorted(t *testing.T) {
	cat := NewTestCatalog(t)

	names := cat.PresetNames()

	// We now have 5 test presets: default-128, default-32, finland-32, test-pack, test-small
	if len(names) != 5 {
		t.Fatalf("expected 5 preset names, got %d: %v", len(names), names)
	}
	if names[0] != "default-128" {
		t.Errorf("expected first name %q, got %q", "default-128", names[0])
	}
	if names[1] != "default-32" {
		t.Errorf("expected second name %q, got %q", "default-32", names[1])
	}
}

func TestRecipeByIDFindsKnown(t *testing.T) {
	cat := NewTestCatalog(t)

	item, ok := cat.RecipeByID("wikipedia-en-nopic")
	if !ok {
		t.Fatal("expected to find recipe wikipedia-en-nopic")
	}
	if item.Type != "zim" {
		t.Errorf("expected type %q, got %q", "zim", item.Type)
	}
	if item.SizeGB != 4.5 {
		t.Errorf("expected size_gb 4.5, got %f", item.SizeGB)
	}
	if item.Description != "English Wikipedia without images" {
		t.Errorf("expected description %q, got %q", "English Wikipedia without images", item.Description)
	}
}

func TestResolvePresetErrorsOnUnknown(t *testing.T) {
	cat := NewTestCatalog(t)

	_, err := cat.ResolvePreset("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown preset, got nil")
	}
}

func TestDefaultCatalogLoadsRealRecipes(t *testing.T) {
	cat, err := NewDefaultCatalog()
	if err != nil {
		t.Fatalf("NewDefaultCatalog: %v", err)
	}

	assertRealCatalog(t, cat)
}

func TestDefaultCatalogRecipeHasRealFields(t *testing.T) {
	cat, err := NewDefaultCatalog()
	if err != nil {
		t.Fatalf("NewDefaultCatalog: %v", err)
	}

	item, ok := cat.RecipeByID("wikipedia-en-nopic")
	if !ok {
		t.Fatal("expected to find recipe wikipedia-en-nopic in real catalog")
	}

	if item.Type != "zim" {
		t.Errorf("Type: expected %q, got %q", "zim", item.Type)
	}
	if item.SizeGB <= 0 {
		t.Errorf("SizeGB: expected > 0, got %f", item.SizeGB)
	}
	if item.URLPattern == "" {
		t.Errorf("URLPattern: expected non-empty for wikipedia-en-nopic")
	}
	if item.Description == "" {
		t.Errorf("Description: expected non-empty, got empty")
	}
	if item.License == nil {
		t.Fatal("License: expected non-nil for wikipedia-en-nopic")
	}
	if item.License.ID == "" {
		t.Error("License.ID: expected non-empty")
	}
	if len(item.Tags) == 0 {
		t.Error("Tags: expected at least one tag")
	}
	if item.Menu == nil {
		t.Error("Menu: expected non-nil for wikipedia-en-nopic")
	}
}

func TestEmbeddedCatalogLoadsRealRecipes(t *testing.T) {
	cat, err := NewEmbeddedCatalog()
	if err != nil {
		t.Fatalf("NewEmbeddedCatalog: %v", err)
	}

	assertRealCatalog(t, cat)
}

func TestLoadCatalogLoadsRealRecipes(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	assertRealCatalog(t, cat)
}

func TestAllRecipesReturnsAll(t *testing.T) {
	cat := NewTestCatalog(t)

	recipes := cat.AllRecipes()
	if len(recipes) != 2 {
		t.Fatalf("expected 2 recipes from test catalog, got %d", len(recipes))
	}

	ids := make(map[string]bool)
	for _, r := range recipes {
		ids[r.ID] = true
	}
	if !ids["wikipedia-en-nopic"] {
		t.Error("expected recipe wikipedia-en-nopic in AllRecipes")
	}
	if !ids["ifixit"] {
		t.Error("expected recipe ifixit in AllRecipes")
	}
}

// --- Real catalog integration tests ---

func TestLoadCatalogParsesPresetFields(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	// Verify a pack has its fields parsed.
	preset, ok := cat.presets["core"]
	if !ok {
		t.Fatal("expected core pack to exist")
	}
	if preset.Kind != "pack" {
		t.Errorf("Kind: expected %q, got %q", "pack", preset.Kind)
	}
	if preset.DisplayGroup != "Core" {
		t.Errorf("DisplayGroup: expected %q, got %q", "Core", preset.DisplayGroup)
	}
	if preset.Description == "" {
		t.Error("Description: expected non-empty for core pack")
	}
	if preset.TargetSizeGB != 1 {
		t.Errorf("TargetSizeGB: expected 1, got %f", preset.TargetSizeGB)
	}

	// Verify a regular preset has extends parsed.
	d64, ok := cat.presets["default-64"]
	if !ok {
		t.Fatal("expected default-64 to exist")
	}
	if len(d64.Extends) == 0 {
		t.Error("expected default-64 to have extends")
	}
	if d64.Extends[0] != "default-32" {
		t.Errorf("expected default-64 to extend default-32, got %v", d64.Extends)
	}
}

func TestLoadCatalogResolvePresetWithExtends(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	// default-64 extends default-32 which extends tools-base.
	// Resolving should include sources from the entire chain.
	preset, err := cat.ResolvePreset("default-64")
	if err != nil {
		t.Fatalf("ResolvePreset(default-64): %v", err)
	}

	if len(preset.Items) == 0 {
		t.Fatal("expected resolved items for default-64")
	}

	// Items from default-32's parents and own sources should all be present.
	ids := make(map[string]bool)
	for _, item := range preset.Items {
		ids[item.ID] = true
	}

	// wikiciv is in default-32 (inherited from there)
	if !ids["wikiciv"] {
		t.Error("expected wikiciv from default-32 chain")
	}
}

func TestLoadCatalogPacks(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	packs := cat.Packs()
	if len(packs) == 0 {
		t.Fatal("expected at least one pack")
	}

	// All packs should have Kind=="pack".
	for _, p := range packs {
		if p.Kind != "pack" {
			t.Errorf("pack %q has Kind %q, expected %q", p.Name, p.Kind, "pack")
		}
	}

	// Verify sort order: DisplayGroup then Name.
	for i := 1; i < len(packs); i++ {
		prev, curr := packs[i-1], packs[i]
		if prev.DisplayGroup > curr.DisplayGroup {
			t.Errorf("packs not sorted by DisplayGroup: %q (%s) before %q (%s)",
				prev.Name, prev.DisplayGroup, curr.Name, curr.DisplayGroup)
		}
		if prev.DisplayGroup == curr.DisplayGroup && prev.Name > curr.Name {
			t.Errorf("packs not sorted by Name within group: %q before %q",
				prev.Name, curr.Name)
		}
	}
}

func TestLoadCatalogRegions(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	regions := cat.Regions()
	if len(regions) < 2 {
		t.Fatalf("expected at least 2 regions, got %d: %v", len(regions), regions)
	}

	// Should include default and finland.
	found := make(map[string]bool)
	for _, r := range regions {
		found[r] = true
	}
	if !found["default"] {
		t.Error("expected region 'default'")
	}
	if !found["finland"] {
		t.Error("expected region 'finland'")
	}

	// Should be sorted.
	for i := 1; i < len(regions); i++ {
		if regions[i] < regions[i-1] {
			t.Errorf("regions not sorted: %q before %q", regions[i-1], regions[i])
		}
	}
}

func TestLoadCatalogPresetsForRegion(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	presets := cat.PresetsForRegion("default")
	if len(presets) == 0 {
		t.Fatal("expected presets for default region")
	}

	for _, p := range presets {
		if p.Region != "default" {
			t.Errorf("expected region %q, got %q for %q", "default", p.Region, p.Name)
		}
		if p.Kind == "pack" {
			t.Errorf("pack %q should not appear in PresetsForRegion", p.Name)
		}
		if len(p.Name) > 5 && p.Name[:5] == "test-" {
			t.Errorf("test preset %q should not appear in PresetsForRegion", p.Name)
		}
	}

	// Should be sorted by TargetSizeGB.
	for i := 1; i < len(presets); i++ {
		if presets[i].TargetSizeGB < presets[i-1].TargetSizeGB {
			t.Errorf("presets not sorted by TargetSizeGB: %q (%.0f) before %q (%.0f)",
				presets[i-1].Name, presets[i-1].TargetSizeGB,
				presets[i].Name, presets[i].TargetSizeGB)
		}
	}
}

func TestLoadCatalogContentSizeGB(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	preset, err := cat.ResolvePreset("default-32")
	if err != nil {
		t.Fatalf("ResolvePreset: %v", err)
	}

	size := preset.ContentSizeGB()
	if size <= 0 {
		t.Errorf("expected positive ContentSizeGB for default-32, got %f", size)
	}
}

func TestLoadCatalogSourceRemovalWithDash(t *testing.T) {
	cat, err := LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}

	// default-64 has "-wikipedia-en-top-nopic" which removes the inherited source.
	preset, err := cat.ResolvePreset("default-64")
	if err != nil {
		t.Fatalf("ResolvePreset(default-64): %v", err)
	}

	for _, item := range preset.Items {
		if item.ID == "wikipedia-en-top-nopic" {
			t.Error("expected wikipedia-en-top-nopic to be removed by dash prefix in default-64")
		}
	}
}
