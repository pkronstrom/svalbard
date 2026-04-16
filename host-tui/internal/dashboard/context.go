package dashboard

import "github.com/pkronstrom/svalbard/tui"

// contextForDestination returns the right-pane content for the given
// navigation destination. Each destination has a title, optional fields,
// and a body paragraph that describes its purpose.
func contextForDestination(id string, m Model) tui.DetailPane {
	base := tui.DetailPane{Theme: m.theme}

	switch id {
	case "overview":
		base.Title = "Overview"
		base.Fields = []tui.DetailField{
			{Label: "Vault", Value: m.vaultPath},
		}
		base.Body = "Vault summary and status."

	case "add":
		base.Title = "Add Content"
		base.Body = "Choose content to add to this vault's desired state."

	case "remove":
		base.Title = "Remove Content"
		base.Body = "Select items to remove from the vault's desired state."

	case "import":
		base.Title = "Import"
		base.Body = "Import local files, URLs, or YouTube content into the vault."

	case "plan":
		base.Title = "Plan"
		base.Body = "Review what changes will be made to reconcile desired and actual state."

	case "apply":
		base.Title = "Apply"
		base.Body = "Execute the reconciliation plan."

	case "presets":
		base.Title = "Presets"
		base.Body = "Browse and apply preset configurations."
	}

	return base
}
