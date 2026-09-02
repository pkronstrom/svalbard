package builder

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestDispatchPrecedence(t *testing.T) {
	tests := []struct {
		name   string
		recipe catalog.Item
		want   string
		ok     bool
	}{
		{
			name: "explicit steps win over python venv",
			recipe: catalog.Item{Build: &catalog.BuildSpec{
				Family: "python-venv",
				Steps:  []catalog.BuildStep{{Verify: "{output_dir}"}},
			}},
			want: "buildPipeline",
			ok:   true,
		},
		{
			name:   "python venv",
			recipe: catalog.Item{Build: &catalog.BuildSpec{Family: "python-venv"}},
			want:   "buildPythonVenv",
			ok:     true,
		},
		{
			name:   "source app bundle",
			recipe: catalog.Item{Build: &catalog.BuildSpec{Family: "app-bundle", SourceURL: "https://example.test/app.zip"}},
			want:   "buildAppBundleAsPipeline",
			ok:     true,
		},
		{
			name:   "content archive",
			recipe: catalog.Item{Build: &catalog.BuildSpec{Family: "content-archive"}},
			want:   "buildContentArchive",
			ok:     true,
		},
		{
			name:   "unsupported family",
			recipe: catalog.Item{Build: &catalog.BuildSpec{Family: "unknown"}},
			ok:     false,
		},
		{
			name: "no build specification",
			ok:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fn, ok := Dispatch(tc.recipe)
			if ok != tc.ok {
				t.Fatalf("Dispatch() handled = %t, want %t", ok, tc.ok)
			}
			if !ok {
				if fn != nil {
					t.Fatal("Dispatch() returned a function for an unsupported recipe")
				}
				return
			}
			name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
			if !strings.HasSuffix(name, "."+tc.want) {
				t.Errorf("Dispatch() = %s, want %s", name, tc.want)
			}
		})
	}
}

func TestEmbeddedMakeItYourselfUsesContentArchive(t *testing.T) {
	cat, err := catalog.NewEmbeddedCatalog()
	if err != nil {
		t.Fatal(err)
	}
	recipe, ok := cat.RecipeByID("makeityourself")
	if !ok || recipe.Build == nil {
		t.Fatal("makeityourself build recipe not found")
	}
	if recipe.Build.Family != "content-archive" {
		t.Fatalf("makeityourself family = %q", recipe.Build.Family)
	}
	fn, ok := Dispatch(recipe)
	if !ok {
		t.Fatal("makeityourself dispatch was not handled")
	}
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()
	if !strings.HasSuffix(name, ".buildContentArchive") {
		t.Fatalf("makeityourself dispatch = %s", name)
	}
}
