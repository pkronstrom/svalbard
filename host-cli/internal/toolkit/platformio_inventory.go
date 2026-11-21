package toolkit

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

type platformIOInventory struct {
	Version  int                         `json:"version"`
	Packages []platformIOInventoryRecord `json:"packages"`
}

type platformIOInventoryRecord struct {
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
	Owner    string `json:"owner"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Platform string `json:"platform"`
}

type expectedPlatformIOArchive struct {
	id       string
	basename string
	owner    string
	kind     string
	name     string
	platform string
}

func writePlatformIOInventory(root string, entries []manifest.RealizedEntry, recipes map[string]catalog.Item, platforms []string) error {
	if len(platforms) == 0 {
		platforms = realizedPlatformIOPlatforms(entries)
	}
	if _, selected := recipes["platformio-espressif32"]; selected {
		if err := validatePlatformIOPythonClosure(root, entries, platforms); err != nil {
			return err
		}
	}
	expected, err := expectedPlatformIOArchives(recipes, platforms)
	if err != nil {
		return err
	}
	path := filepath.Join(root, svalbardDir, "platformio.json")
	if len(expected) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	records := make([]platformIOInventoryRecord, 0, len(expected))
	for _, want := range expected {
		var matched *manifest.RealizedEntry
		for i := range entries {
			e := &entries[i]
			if e.ID != want.id || e.Filename != want.basename || e.ChecksumSHA256 == "" {
				continue
			}
			if platformIOEntryPath(e.RelativePath, want.platform) || legacyPlatformIOEntryPath(e.RelativePath) {
				if matched != nil {
					return fmt.Errorf("PlatformIO archive %s for %s is ambiguous; reprovision the closure", want.basename, want.platform)
				}
				matched = e
			}
		}
		if matched == nil {
			return fmt.Errorf("PlatformIO closure missing %s for %s; reprovision the esp32-dev pack", want.name, want.platform)
		}
		if _, err := os.Stat(filepath.Join(root, matched.RelativePath)); err != nil {
			return fmt.Errorf("PlatformIO archive %s is not realized at %s; reprovision the esp32-dev pack", want.basename, matched.RelativePath)
		}
		records = append(records, platformIOInventoryRecord{
			Path: filepath.ToSlash(matched.RelativePath), SHA256: matched.ChecksumSHA256,
			Owner: want.owner, Name: want.name, Kind: want.kind, Platform: want.platform,
		})
	}
	data, err := json.Marshal(platformIOInventory{Version: 1, Packages: records})
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Join(root, svalbardDir), ".platformio-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func validatePlatformIOPythonClosure(root string, entries []manifest.RealizedEntry, platforms []string) error {
	for _, platform := range platforms {
		for _, id := range []string{"platformio-core", "espidf-python"} {
			want := filepath.ToSlash(filepath.Join("runtime", "python", platform, "tools", id))
			found := false
			for _, entry := range entries {
				if entry.ID == id && filepath.ToSlash(entry.RelativePath) == want {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("PlatformIO Python closure missing %s for %s; reprovision the esp32-dev pack", id, platform)
			}
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(want)))
			if err != nil || !info.IsDir() {
				return fmt.Errorf("PlatformIO Python tree %s for %s is missing; reprovision the esp32-dev pack", id, platform)
			}
		}
	}
	return nil
}

func expectedPlatformIOArchives(recipes map[string]catalog.Item, platforms []string) ([]expectedPlatformIOArchive, error) {
	wantedPlatforms := make(map[string]bool, len(platforms))
	for _, platform := range platforms {
		wantedPlatforms[normalizePlatform(platform)] = true
	}
	var expected []expectedPlatformIOArchive
	for id, recipe := range recipes {
		if recipe.URL != "" {
			archive, ok := parsePlatformIOURL(recipe.URL)
			if !ok {
				return nil, fmt.Errorf("PlatformIO recipe %s has invalid registry URL", id)
			}
			expected = append(expected, expectedPlatformIOArchive{id: id, basename: archive.basename, owner: archive.owner, kind: archive.kind, name: archive.name})
		}
		for rawPlatform, rawURL := range recipe.Platforms {
			if len(wantedPlatforms) > 0 && !wantedPlatforms[normalizePlatform(rawPlatform)] {
				continue
			}
			archive, ok := parsePlatformIOURL(rawURL)
			if !ok {
				return nil, fmt.Errorf("PlatformIO recipe %s has invalid registry URL", id)
			}
			expected = append(expected, expectedPlatformIOArchive{id: id, basename: archive.basename, owner: archive.owner, kind: archive.kind, name: archive.name, platform: normalizePlatform(rawPlatform)})
		}
	}
	sort.Slice(expected, func(i, j int) bool {
		if expected[i].id != expected[j].id {
			return expected[i].id < expected[j].id
		}
		if expected[i].platform != expected[j].platform {
			return expected[i].platform < expected[j].platform
		}
		return expected[i].basename < expected[j].basename
	})
	return expected, nil
}

func realizedPlatformIOPlatforms(entries []manifest.RealizedEntry) []string {
	platforms := make(map[string]bool)
	const prefix = "tools/platformio/packages/"
	for _, entry := range entries {
		rel := filepath.ToSlash(entry.RelativePath)
		if strings.HasPrefix(rel, prefix) {
			parts := strings.Split(strings.TrimPrefix(rel, prefix), "/")
			if len(parts) > 1 {
				platforms[parts[0]] = true
			}
		}
	}
	result := make([]string, 0, len(platforms))
	for platform := range platforms {
		result = append(result, platform)
	}
	sort.Strings(result)
	return result
}

type parsedPlatformIOURL struct{ owner, kind, name, basename string }

func parsePlatformIOURL(raw string) (parsedPlatformIOURL, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host != "dl.registry.platformio.org" {
		return parsedPlatformIOURL{}, false
	}
	p := strings.Split(strings.Trim(u.EscapedPath(), "/"), "/")
	if len(p) != 6 || p[0] != "download" || p[1] == "" || (p[2] != "tool" && p[2] != "platform") || p[3] == "" || p[4] == "" || p[5] == "" {
		return parsedPlatformIOURL{}, false
	}
	return parsedPlatformIOURL{owner: p[1], kind: p[2], name: p[3], basename: p[5]}, true
}

func normalizePlatform(platform string) string {
	platform = strings.ReplaceAll(platform, "darwin-", "macos-")
	platform = strings.ReplaceAll(platform, "-aarch64", "-arm64")
	platform = strings.ReplaceAll(platform, "-amd64", "-x86_64")
	return platform
}

func platformIOEntryPath(rel, platform string) bool {
	base := "tools/platformio/packages"
	dir := filepath.ToSlash(filepath.Dir(rel))
	if platform == "" {
		return dir == base
	}
	return dir == base+"/"+platform
}

func legacyPlatformIOEntryPath(rel string) bool {
	return !strings.Contains(filepath.ToSlash(rel), "/")
}
