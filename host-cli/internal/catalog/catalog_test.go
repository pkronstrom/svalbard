package catalog

import (
	"testing"
)

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

func TestPresetNamesReturnsSorted(t *testing.T) {
	cat := NewTestCatalog(t)

	names := cat.PresetNames()

	if len(names) != 2 {
		t.Fatalf("expected 2 preset names, got %d", len(names))
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

	names := cat.PresetNames()
	if len(names) == 0 {
		t.Fatal("expected at least one preset, got 0")
	}

	// Verify the well-known "default-32" preset exists.
	found := false
	for _, name := range names {
		if name == "default-32" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected preset %q to exist; presets: %v", "default-32", names)
	}
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
