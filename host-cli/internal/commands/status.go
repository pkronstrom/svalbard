package commands

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// StatusItem is one desired item and whether the vault holds it yet.
type StatusItem struct {
	ID        string `json:"id"`
	Type      string `json:"type,omitempty"`
	SizeBytes int64  `json:"size_bytes"`
	// Status is "realized", "pending", or "unavailable" for a pending item
	// the catalog no longer has.
	Status string `json:"status"`
	// Version is one of the Version* states; empty unless realized.
	Version string `json:"version,omitempty"`
}

// StatusReport is the vault state the status command prints.
type StatusReport struct {
	Vault       string   `json:"vault"`
	Preset      string   `json:"preset,omitempty"`
	Platforms   []string `json:"platforms"`
	Desired     int      `json:"desired"`
	Realized    int      `json:"realized"`
	Pending     int      `json:"pending"`
	LastApplied string   `json:"last_applied,omitempty"`
	Updates     []string `json:"updates_available"`
	Withdrawn   []string `json:"withdrawn"`
	// Unknown lists items installed before versions were recorded.
	Unknown     []string `json:"unknown_version"`
	Unavailable []string `json:"unavailable"`
	// OtherPlatform lists selected items with no payload for the vault's
	// platforms; apply skips them.
	OtherPlatform []string     `json:"other_platform"`
	Items         []StatusItem `json:"items"`
	// Usage is what the vault's files hold and take on its disk; nil when
	// not measured.
	Usage *VaultUsage `json:"usage,omitempty"`
}

// BuildStatus summarizes m: each desired item, whether it is realized, and
// how its installed version compares with the catalog's recipe.
func BuildStatus(cat *catalog.Catalog, m manifest.Manifest) StatusReport {
	r := StatusReport{
		Vault:       m.Vault.Name,
		Platforms:   append([]string{}, m.Desired.Options.HostPlatforms...),
		Desired:     len(m.Desired.Items),
		LastApplied: m.Realized.AppliedAt,
		Updates:     []string{},
		Withdrawn:   []string{},
		Unknown:     []string{},
		Unavailable: []string{},
		Items:       []StatusItem{},
	}
	r.OtherPlatform = OtherPlatformItems(cat, m)
	if len(m.Desired.Presets) > 0 {
		r.Preset = m.Desired.Presets[0]
	}

	installed := realizedByID(m)
	for _, id := range m.Desired.Items {
		item := StatusItem{ID: id, Status: "pending"}
		// A multi-platform recipe has one realized entry per platform.
		if entries := installed[id]; len(entries) > 0 {
			item.Type, item.Status = entries[0].Type, "realized"
			for _, e := range entries {
				item.SizeBytes += e.SizeBytes
			}
			item.Version = ItemVersion(cat, entries)
			switch item.Version {
			case VersionUpdate:
				r.Updates = append(r.Updates, id)
			case VersionWithdrawn:
				r.Withdrawn = append(r.Withdrawn, id)
			case VersionUnknown:
				r.Unknown = append(r.Unknown, id)
			}
			r.Realized++
		} else if slices.Contains(r.OtherPlatform, id) {
			item.Status = "other platform"
		} else if len(UnknownItems(cat, []string{id})) > 0 {
			item.Status = "unavailable"
			r.Unavailable = append(r.Unavailable, id)
		}
		r.Items = append(r.Items, item)
	}
	// An installed item is not skipped, whatever the recipe says now.
	r.OtherPlatform = slices.DeleteFunc(r.OtherPlatform, func(id string) bool { return len(installed[id]) > 0 })
	r.Pending = r.Desired - r.Realized - len(r.OtherPlatform) - len(r.Unavailable)
	return r
}

// versionLabels words each version state for the status table.
var versionLabels = map[string]string{
	VersionCurrent:   "current",
	VersionUpdate:    "update available",
	VersionUnknown:   "unknown",
	VersionWithdrawn: "withdrawn, kept",
	VersionLocal:     "local",
	VersionDocuments: "by document",
}

// WriteStatus prints a human-readable summary of vault state including
// desired items, realized items, and what is pending.
func WriteStatus(w io.Writer, cat *catalog.Catalog, m manifest.Manifest) error {
	return WriteStatusReport(w, BuildStatus(cat, m), m)
}

// WriteStatusReport prints r, the status BuildStatus made of m.
func WriteStatusReport(w io.Writer, r StatusReport, m manifest.Manifest) error {
	fmt.Fprintf(w, "Vault: %s\n", r.Vault)
	if r.Preset != "" {
		fmt.Fprintf(w, "Preset: %s\n", r.Preset)
	}
	fmt.Fprintln(w)

	fmt.Fprintf(w, "Desired: %d items\n", r.Desired)
	fmt.Fprintf(w, "Realized: %d items\n", r.Realized)
	fmt.Fprintf(w, "Pending: %d items\n", r.Pending)
	if r.Usage != nil {
		WriteVaultUsage(w, *r.Usage)
	}

	if r.Desired == 0 {
		return nil
	}

	fmt.Fprintln(w)

	// Tab-aligned table
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "  ID\tType\tSize\tStatus\tVersion\n")
	for _, item := range r.Items {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s\n", item.ID, item.Type, humanSize(item.SizeBytes), item.Status, versionLabels[item.Version])
	}
	tw.Flush()

	fmt.Fprintln(w)
	if pending := r.Pending; pending > 0 {
		fmt.Fprintf(w, "Run \"svalbard apply\" to download the %d pending.\n", pending)
	}
	if len(r.Updates) > 0 {
		fmt.Fprintf(w, "%d update(s) available; run \"svalbard update\" to review them.\n", len(r.Updates))
	}
	if len(r.Unknown) > 0 {
		fmt.Fprintf(w, "%d item(s) of unknown version; \"svalbard update --mark-current\" records them as current without downloading.\n", len(r.Unknown))
	}
	if len(r.Withdrawn) > 0 {
		fmt.Fprintf(w, "No longer in the catalog, kept on this vault: %s.\n", strings.Join(r.Withdrawn, ", "))
	}
	if len(r.OtherPlatform) > 0 {
		fmt.Fprintf(w, "Skipped, not available for %s: %s.\n", strings.Join(vaultPlatforms(m), ", "), strings.Join(r.OtherPlatform, ", "))
	}
	if len(r.Unavailable) > 0 {
		fmt.Fprintf(w, "Skipped, not in the catalog on this computer: %s. \"svalbard remove\" deselects them.\n", strings.Join(r.Unavailable, ", "))
	}
	return nil
}

// humanSize converts a byte count to a human-readable string.
func humanSize(bytes int64) string {
	if bytes == 0 {
		return "—"
	}
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	default:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	}
}

// WriteSkipped tells w which selected items an apply of m will leave out,
// and why, so that a skipped item is never a silent one.
func WriteSkipped(w io.Writer, cat *catalog.Catalog, m manifest.Manifest) {
	r := BuildStatus(cat, m)
	if len(r.OtherPlatform) > 0 {
		fmt.Fprintf(w, "note: skipping %s: not available for %s\n", strings.Join(r.OtherPlatform, ", "), strings.Join(vaultPlatforms(m), ", "))
	}
	if len(r.Unavailable) > 0 {
		fmt.Fprintf(w, "note: skipping %s: not in the catalog on this computer\n", strings.Join(r.Unavailable, ", "))
	}
}
