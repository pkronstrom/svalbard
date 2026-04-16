package commands

import "github.com/pkronstrom/svalbard/host-cli/internal/manifest"

// RemoveItems filters m.Desired.Items to exclude any ids in the given list.
// This is a metadata-only operation with no side effects.
func RemoveItems(m *manifest.Manifest, ids []string) error {
	toRemove := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		toRemove[id] = struct{}{}
	}
	filtered := make([]string, 0, len(m.Desired.Items))
	for _, item := range m.Desired.Items {
		if _, ok := toRemove[item]; !ok {
			filtered = append(filtered, item)
		}
	}
	m.Desired.Items = filtered
	return nil
}
