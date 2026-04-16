package menu

import (
	"fmt"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/tui"
)

// contextForGroup returns a DetailPane appropriate for the given menu group.
// Certain group IDs receive extra fields (e.g. item counts) while others
// get only a title and description body.
func contextForGroup(group config.MenuGroup, theme tui.Theme) tui.DetailPane {
	base := tui.DetailPane{
		Theme: theme,
		Title: group.Label,
		Body:  group.Description,
	}

	switch group.ID {
	case "browse", "library":
		base.Fields = []tui.DetailField{
			{Label: "Archives", Value: fmt.Sprintf("%d", len(group.Items))},
		}
	case "maps":
		base.Fields = []tui.DetailField{
			{Label: "Layers", Value: fmt.Sprintf("%d", len(group.Items))},
		}
	case "apps", "tools":
		base.Fields = []tui.DetailField{
			{Label: "Tools", Value: fmt.Sprintf("%d", len(group.Items))},
		}
	}

	return base
}
