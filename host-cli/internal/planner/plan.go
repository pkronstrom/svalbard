package planner

import "github.com/pkronstrom/svalbard/host-cli/internal/manifest"

// Plan describes the reconciliation actions needed to bring the realized
// state in line with the desired state.
type Plan struct {
	ToDownload []string
	ToRemove   []string
	Unmanaged  []string
}

// Build computes a reconciliation plan by comparing the desired items
// in the manifest against the realized entries.
func Build(m manifest.Manifest) Plan {
	desired := make(map[string]struct{}, len(m.Desired.Items))
	for _, id := range m.Desired.Items {
		desired[id] = struct{}{}
	}

	realized := make(map[string]struct{}, len(m.Realized.Entries))
	for _, e := range m.Realized.Entries {
		realized[e.ID] = struct{}{}
	}

	var p Plan

	// Entries present in realized but not in desired need removal.
	for _, e := range m.Realized.Entries {
		if _, ok := desired[e.ID]; !ok {
			p.ToRemove = append(p.ToRemove, e.ID)
		}
	}

	// Items in desired but not yet realized need downloading.
	for _, id := range m.Desired.Items {
		if _, ok := realized[id]; !ok {
			p.ToDownload = append(p.ToDownload, id)
		}
	}

	return p
}

// BuildWithDisk computes a reconciliation plan and additionally detects
// unmanaged files by comparing the realized entries against the actual
// files present on disk.
func BuildWithDisk(m manifest.Manifest, onDisk []string) Plan {
	p := Build(m)

	managed := make(map[string]struct{}, len(m.Realized.Entries))
	for _, e := range m.Realized.Entries {
		managed[e.RelativePath] = struct{}{}
	}

	for _, path := range onDisk {
		if _, ok := managed[path]; !ok {
			p.Unmanaged = append(p.Unmanaged, path)
		}
	}

	return p
}
