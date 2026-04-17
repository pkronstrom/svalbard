package dashboard

import "github.com/pkronstrom/svalbard/tui"

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
		base.Body = "Vault summary and sync state."

	case destBrowse:
		base.Title = "Browse"
		base.Body = "Explore the full content catalog. Toggle items on or off\nto manage your vault's desired state."

	case destPlan:
		base.Title = "Plan"
		base.Body = "Review what changes will be made to reconcile desired\nand realized state. Apply changes from here."

	case destImport:
		base.Title = "Import"
		base.Body = "Import local files, URLs, or YouTube content into the\nvault and add them to the desired state."

	case destIndex:
		base.Title = "Index"
		base.Body = "Manage keyword (FTS5) and semantic search indexes.\nKeyword search is fast and exact. Semantic search finds\nconceptually related content using embeddings."

	case destNewVault:
		base.Title = "New Vault"
		base.Body = "Launch the init wizard to create a new vault.\nChoose a path, pick a preset, customize content,\nand confirm."
	}

	return base
}
