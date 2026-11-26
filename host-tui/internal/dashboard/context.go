package dashboard

import (
	"fmt"

	"github.com/pkronstrom/svalbard/tui"
)

// contextForDestination returns the right-pane content for the given
// navigation destination. Each destination shows a live preview of its
// current state when possible, falling back to a description.
func contextForDestination(id string, m Model) tui.DetailPane {
	base := tui.DetailPane{Theme: m.theme}

	switch id {
	case destStatus:
		base.Title = "Status"
		base.Fields = []tui.DetailField{
			{Label: "Vault", Value: m.vaultPath},
		}
		if m.status != nil {
			s := m.status
			if s.PresetName != "" {
				base.Fields = append(base.Fields, tui.DetailField{Label: "Preset", Value: s.PresetName})
			}
			base.Fields = append(base.Fields,
				tui.DetailField{Label: "Desired", Value: fmt.Sprintf("%d items", s.DesiredCount)},
				tui.DetailField{Label: "Realized", Value: fmt.Sprintf("%d items", s.RealizedCount)},
				tui.DetailField{Label: "Pending", Value: fmt.Sprintf("%d items", s.PendingCount)},
			)
			if s.UpdateCount > 0 {
				base.Fields = append(base.Fields, tui.DetailField{Label: "Updates", Value: fmt.Sprintf("%d available", s.UpdateCount)})
			}
			if s.WithdrawnCount > 0 {
				base.Fields = append(base.Fields, tui.DetailField{Label: "Withdrawn", Value: fmt.Sprintf("%d kept", s.WithdrawnCount)})
			}
			if s.DiskUsedGB > 0 {
				base.Fields = append(base.Fields,
					tui.DetailField{Label: "Vault size", Value: tui.FormatSize(s.DiskUsedGB)},
				)
			}
			if s.DiskFreeGB > 0 {
				base.Fields = append(base.Fields,
					tui.DetailField{Label: "Disk free", Value: tui.FormatSize(s.DiskFreeGB)},
				)
			}
			if s.LastApplied != "" {
				base.Fields = append(base.Fields,
					tui.DetailField{Label: "Last apply", Value: s.LastApplied},
				)
			}
		} else {
			base.Body = "Loading vault status..."
		}

	case destBrowse:
		base.Title = "Browse"
		base.Body = "Explore the full content catalog. Toggle items on or off\nto manage your vault's desired state."

	case destPlan:
		base.Title = "Plan"
		base.Body = "Review what changes will be made to reconcile desired\nand realized state. Apply changes from here."

	case destUpdates:
		base.Title = "Updates"
		base.Body = "Installed items whose recipe changed since they were installed.\nEach installed copy is kept until its update is in place.\nItems no longer in the catalog stay on the vault."
		if m.status != nil && m.status.UpdateCount > 0 {
			base.Fields = []tui.DetailField{{Label: "Available", Value: fmt.Sprintf("%d items", m.status.UpdateCount)}}
		}

	case destImport:
		base.Title = "Import"
		base.Body = "Copy a file from this computer into the vault's library\nand select it. Plan then installs it."

	case destIndex:
		base.Title = "Index"
		base.Body = "Manage keyword (FTS5) and semantic search indexes.\nKeyword search is fast and exact. Semantic search finds\nconceptually related content using embeddings."

	case destNewVault:
		base.Title = "New Vault"
		base.Body = "Launch the init wizard to create a new vault.\nChoose a path, pick a preset, customize content,\nand confirm."
	}

	return base
}
