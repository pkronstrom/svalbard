package commands

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// WriteStatus prints a human-readable summary of vault state including
// desired items, realized items, and what is pending.
func WriteStatus(w io.Writer, m manifest.Manifest) error {
	// Vault name and preset
	fmt.Fprintf(w, "Vault: %s\n", m.Vault.Name)
	if len(m.Desired.Presets) > 0 {
		fmt.Fprintf(w, "Preset: %s\n", m.Desired.Presets[0])
	}
	fmt.Fprintln(w)

	// Build realized lookup by ID
	realizedByID := make(map[string]manifest.RealizedEntry, len(m.Realized.Entries))
	for _, e := range m.Realized.Entries {
		realizedByID[e.ID] = e
	}

	// Count
	desired := len(m.Desired.Items)
	realized := 0
	for _, id := range m.Desired.Items {
		if _, ok := realizedByID[id]; ok {
			realized++
		}
	}
	pending := desired - realized

	fmt.Fprintf(w, "Desired: %d items\n", desired)
	fmt.Fprintf(w, "Realized: %d items\n", realized)
	fmt.Fprintf(w, "Pending: %d items\n", pending)

	if desired == 0 {
		return nil
	}

	fmt.Fprintln(w)

	// Tab-aligned table
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintf(tw, "  ID\tType\tSize\tStatus\n")
	for _, id := range m.Desired.Items {
		if e, ok := realizedByID[id]; ok {
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", id, e.Type, humanSize(e.SizeBytes), "realized")
		} else {
			fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", id, "", "—", "pending")
		}
	}
	tw.Flush()

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
