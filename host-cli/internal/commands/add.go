package commands

import "github.com/pkronstrom/svalbard/host-cli/internal/manifest"

// AddItems appends ids to m.Desired.Items, skipping any that are already present.
// This is a metadata-only operation with no side effects.
func AddItems(m *manifest.Manifest, ids []string) error {
	existing := make(map[string]struct{}, len(m.Desired.Items))
	for _, item := range m.Desired.Items {
		existing[item] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := existing[id]; !ok {
			m.Desired.Items = append(m.Desired.Items, id)
			existing[id] = struct{}{}
		}
	}
	return nil
}
