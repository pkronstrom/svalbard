package catalog

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func parseItem(t *testing.T, src string) Item {
	t.Helper()
	var item Item
	if err := yaml.Unmarshal([]byte(src), &item); err != nil {
		t.Fatal(err)
	}
	return item
}

const acquisitionRecipe = `
id: demo
type: binary
description: A demo tool
size_gb: 0.1
strategy: download
platforms:
  linux-x86_64: https://example.com/demo-1.0-linux.tar.gz
  macos-arm64: https://example.com/demo-1.0-mac.tar.gz
platform_sha256:
  macos-arm64: aaaa
tags: [tools]
menu:
  label: Demo
build:
  family: app-bundle
  estimated_download_gb: 1
  quality: "480"
  steps:
    - download: https://example.com/src.zip
    - extract: src.zip
`

func TestAcquisitionHashIgnoresPresentationAndOrder(t *testing.T) {
	base := parseItem(t, acquisitionRecipe).AcquisitionHash()
	if !strings.HasPrefix(base, "acq-v1:sha256:") || len(base) != len("acq-v1:sha256:")+64 {
		t.Fatalf("hash format: %q", base)
	}

	same := map[string]string{
		"description":    strings.Replace(acquisitionRecipe, "A demo tool", "Reworded", 1),
		"size estimate":  strings.Replace(acquisitionRecipe, "size_gb: 0.1", "size_gb: 9", 1),
		"build estimate": strings.Replace(acquisitionRecipe, "estimated_download_gb: 1", "estimated_download_gb: 7", 1),
		"tags and menu":  strings.Replace(strings.Replace(acquisitionRecipe, "[tools]", "[other]", 1), "label: Demo", "label: Renamed", 1),
		"platform key order": strings.Replace(acquisitionRecipe,
			"  linux-x86_64: https://example.com/demo-1.0-linux.tar.gz\n  macos-arm64: https://example.com/demo-1.0-mac.tar.gz\n",
			"  macos-arm64: https://example.com/demo-1.0-mac.tar.gz\n  linux-x86_64: https://example.com/demo-1.0-linux.tar.gz\n", 1),
		"flow syntax": strings.Replace(acquisitionRecipe, "platform_sha256:\n  macos-arm64: aaaa\n", "platform_sha256: {macos-arm64: aaaa}\n", 1),
	}
	for name, src := range same {
		if src == acquisitionRecipe {
			t.Fatalf("%s: test edit did not apply", name)
		}
		if got := parseItem(t, src).AcquisitionHash(); got != base {
			t.Errorf("%s changed the hash", name)
		}
	}
}

func TestAcquisitionHashFollowsWhatGetsInstalled(t *testing.T) {
	base := parseItem(t, acquisitionRecipe).AcquisitionHash()
	changed := map[string]string{
		"platform url":      strings.Replace(acquisitionRecipe, "demo-1.0-mac", "demo-1.1-mac", 1),
		"platform pin":      strings.Replace(acquisitionRecipe, "aaaa", "bbbb", 1),
		"type":              strings.Replace(acquisitionRecipe, "type: binary", "type: app", 1),
		"build step":        strings.Replace(acquisitionRecipe, "src.zip\n    - extract", "src2.zip\n    - extract", 1),
		"build step order":  strings.Replace(acquisitionRecipe, "    - download: https://example.com/src.zip\n    - extract: src.zip\n", "    - extract: src.zip\n    - download: https://example.com/src.zip\n", 1),
		"family config":     strings.Replace(acquisitionRecipe, `quality: "480"`, `quality: "720"`, 1),
		"filename":          acquisitionRecipe + "filename: demo.bin\n",
		"python packages":   acquisitionRecipe + "packages: [esptool==4.8]\n",
		"bundle":            acquisitionRecipe + "bundle:\n  - url: https://example.com/a.tgz\n",
		"embedding params":  acquisitionRecipe + "embedding:\n  dims: 256\n",
		"single url":        acquisitionRecipe + "url: https://example.com/demo.zim\n",
		"single url pinned": acquisitionRecipe + "url: https://example.com/demo.zim\nsha256: cccc\n",
	}
	seen := map[string]string{base: "base"}
	for name, src := range changed {
		if src == acquisitionRecipe {
			t.Fatalf("%s: test edit did not apply", name)
		}
		got := parseItem(t, src).AcquisitionHash()
		if other, dup := seen[got]; dup {
			t.Errorf("%s has the same hash as %s", name, other)
		}
		seen[got] = name
	}
}

func TestRasterBandsParseAndChangeTheAcquisitionHash(t *testing.T) {
	const recipe = `
id: topo
type: pmtiles
strategy: build
build:
  family: raster-sheets
  bbox: "23.8,59.8,26.8,61.7"
  bands:
    - {min_zoom: 0, max_zoom: 8, source: "https://example.test/500k/"}
    - {min_zoom: 13, max_zoom: 14, source: "https://example.test/50k/"}
`
	item := parseItem(t, recipe)
	want := []RasterBand{{0, 8, "https://example.test/500k/"}, {13, 14, "https://example.test/50k/"}}
	if len(item.Build.Bands) != 2 || item.Build.Bands[0] != want[0] || item.Build.Bands[1] != want[1] {
		t.Fatalf("Bands = %+v, want %+v", item.Build.Bands, want)
	}
	if _, ok := item.Build.Config["bands"]; ok {
		t.Error("bands leaked into Config")
	}
	if item.Build.Config["bbox"] != "23.8,59.8,26.8,61.7" {
		t.Errorf("bbox = %q", item.Build.Config["bbox"])
	}
	changed := parseItem(t, strings.Replace(recipe, "max_zoom: 14", "max_zoom: 15", 1))
	if item.AcquisitionHash() == changed.AcquisitionHash() {
		t.Error("changing a band's zoom range kept the acquisition hash")
	}
}

// A bundle recipe installed before include patterns existed keeps its hash:
// the field is left out of the projection when empty.
func TestBundleHashUnchangedWithoutInclude(t *testing.T) {
	item := parseItem(t, `
id: jscad-web
type: app
bundle:
  - url: https://registry.npmjs.org/@jscad/web/-/web-2.6.13.tgz
    sha256: 9ed53c6aefa537e84e36b90a55ca451c3eef50eda6c0916e1f95a42abf5a810f
    strip: package
`)
	if got, want := item.AcquisitionHash(), "acq-v1:sha256:e291d3b1aa1f4c974fbde5b11c86d8f2709690dd24205f0637bf8dd1924e3f0b"; got != want {
		t.Errorf("hash = %s, want %s (as before include patterns)", got, want)
	}
}
