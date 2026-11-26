package builder_test

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/builder/buildertest"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

func TestPipelineScenarioProducesVerifiedArtifact(t *testing.T) {
	recipe := catalog.Item{
		ID:       "sample",
		Type:     "zim",
		Strategy: "build",
		Build: &catalog.BuildSpec{
			Family: "pipeline",
			Output: "sample.zim",
		},
	}
	scenario := buildertest.New(t, recipe)
	source := scenario.Source([]byte("offline artifact"))
	scenario.Recipe.Build.Steps = []catalog.BuildStep{
		{Download: source, Dest: "{output}"},
		{Verify: "{output}", MinSize: 1},
	}

	scenario.Build()
	if scenario.Err != nil {
		t.Fatal(scenario.Err)
	}
	scenario.Artifact("zim/sample.zim", []byte("offline artifact"))
	if len(scenario.Entries) != 1 || scenario.Entries[0].ID != "sample" {
		t.Fatalf("entries = %+v", scenario.Entries)
	}
}

func TestAppBundleAssetsProduceDirectoryArtifact(t *testing.T) {
	recipe := catalog.Item{
		ID:       "offline-app",
		Type:     "app",
		Strategy: "build",
		Build: &catalog.BuildSpec{
			Family: "app-bundle",
		},
	}
	scenario := buildertest.New(t, recipe)
	scenario.Recipe.Build.Assets = []catalog.BuildAsset{
		{URL: scenario.Source([]byte("script")), Dest: "js/app.js"},
		{URL: scenario.Source([]byte("style")), Dest: "css/app.css"},
	}
	scenario.Build()
	if scenario.Err != nil {
		t.Fatal(scenario.Err)
	}
	scenario.
		Artifact("apps/offline-app/js/app.js", []byte("script")).
		Artifact("apps/offline-app/css/app.css", []byte("style"))
}

// source_url downloads to an extensionless file, and real sources use
// extensions such as .war, so the archive format must come from its content.
func TestAppBundleSourceURLUnpacksZipWithoutExtension(t *testing.T) {
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	for name, body := range map[string]string{"index.html": "page", "js/app.js": "script"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	recipe := catalog.Item{
		ID:       "zipped-app",
		Type:     "app",
		Strategy: "build",
		Build:    &catalog.BuildSpec{Family: "app-bundle"},
	}
	scenario := buildertest.New(t, recipe)
	scenario.Recipe.Build.SourceURL = scenario.Source(archive.Bytes()) + "/draw.war"
	scenario.Build()
	if scenario.Err != nil {
		t.Fatal(scenario.Err)
	}
	scenario.
		Artifact("apps/zipped-app/index.html", []byte("page")).
		Artifact("apps/zipped-app/js/app.js", []byte("script"))
}

func TestUnsupportedFamilyFailsBeforeExecution(t *testing.T) {
	scenario := buildertest.New(t, catalog.Item{
		ID:       "unsupported",
		Type:     "zim",
		Strategy: "build",
		Build:    &catalog.BuildSpec{Family: "missing-family"},
	}).Build()
	if scenario.Err == nil {
		t.Fatal("unsupported family succeeded")
	}
}
