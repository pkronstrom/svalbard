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
