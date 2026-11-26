package catalog

import "testing"

func TestFileCountForCountsTreesPerPlatformAndKnowsSingleFiles(t *testing.T) {
	platforms := []string{"macos-arm64", "linux-x86_64"}
	for _, tc := range []struct {
		name  string
		item  Item
		want  int
		known bool
	}{
		{"zim", Item{Type: "zim"}, 1, true},
		{"binary for both", Item{Type: "binary", Platforms: map[string]string{"macos-arm64": "a", "linux-x86_64": "b"}}, 2, true},
		{"measured python tree", Item{Type: "python-package", FileCount: 21977}, 2 * 21977, true},
		{"measured app", Item{Type: "app", FileCount: 3337}, 3337, true},
		{"unmeasured app", Item{Type: "app"}, 0, false},
		{"unmeasured python tree", Item{Type: "python-package"}, 0, false},
	} {
		got, known := tc.item.FileCountFor(platforms)
		if got != tc.want || known != tc.known {
			t.Errorf("%s: FileCountFor = %d, %v; want %d, %v", tc.name, got, known, tc.want, tc.known)
		}
	}
}
