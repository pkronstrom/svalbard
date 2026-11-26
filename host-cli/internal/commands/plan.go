package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/planner"
	"github.com/pkronstrom/svalbard/tui"
)

// PlanItem is one pending download or removal with its catalog details.
type PlanItem struct {
	ID              string   `json:"id"`
	Action          string   `json:"action"` // "download" or "remove"
	Type            string   `json:"type,omitempty"`
	Description     string   `json:"description,omitempty"`
	SizeGB          float64  `json:"size_gb"`
	Network         bool     `json:"build_network,omitempty"`
	BuildDownloadGB float64  `json:"build_download_gb,omitempty"`
	BuildWorkGB     float64  `json:"build_work_gb,omitempty"`
	Requires        []string `json:"build_requires,omitempty"`
}

// PlanDetail is a reconciliation plan with sizes and build costs, as the
// plan command and the TUI plan screen show it.
type PlanDetail struct {
	Download []PlanItem `json:"download"`
	Remove   []PlanItem `json:"remove"`
	// Updates lists installed items whose recipe changed. Apply leaves them
	// alone; "svalbard update" acts on them.
	Updates         []PlanItem `json:"updates_available"`
	DownloadGB      float64    `json:"download_gb"`
	RemoveGB        float64    `json:"remove_gb"`
	BuildDownloadGB float64    `json:"build_download_gb"`
	BuildWorkGB     float64    `json:"build_work_gb"`
	ToolsRequired   bool       `json:"tools_required"`
}

// DescribePlan builds the plan for m and fills in each item's catalog
// details, sized for the vault's platforms.
func DescribePlan(cat *catalog.Catalog, m manifest.Manifest) PlanDetail {
	p := planner.Build(m)
	platforms := m.Desired.Options.HostPlatforms
	d := PlanDetail{Download: []PlanItem{}, Remove: []PlanItem{}, Updates: AvailableUpdates(cat, m)}
	for _, id := range p.ToDownload {
		item := PlanItem{ID: id, Action: "download"}
		if recipe, ok := cat.RecipeByID(id); ok {
			item.Type = recipe.Type
			item.SizeGB = recipe.SizeGBFor(platforms)
			item.Description = recipe.Description
			if recipe.Build != nil {
				item.Network = recipe.Build.Network
				item.BuildDownloadGB = recipe.Build.EstimatedDownloadGB
				item.BuildWorkGB = recipe.Build.EstimatedWorkGB
				item.Requires = append([]string(nil), recipe.Build.Requires...)
				d.BuildDownloadGB += item.BuildDownloadGB
				d.BuildWorkGB += item.BuildWorkGB
				d.ToolsRequired = d.ToolsRequired || len(item.Requires) > 0
			}
		}
		d.Download = append(d.Download, item)
		d.DownloadGB += item.SizeGB
	}
	for _, id := range p.ToRemove {
		item := PlanItem{ID: id, Action: "remove"}
		if recipe, ok := cat.RecipeByID(id); ok {
			item.Type = recipe.Type
			item.SizeGB = recipe.SizeGBFor(platforms)
			item.Description = recipe.Description
		}
		d.Remove = append(d.Remove, item)
		d.RemoveGB += item.SizeGB
	}
	return d
}

// WritePlanDetail writes d as text: what apply would download and remove,
// with sizes, and the command that does it.
func WritePlanDetail(w io.Writer, d PlanDetail) error {
	if len(d.Updates) > 0 {
		defer fmt.Fprintf(w, "\n%d update(s) available; run \"svalbard update\" to review them.\n", len(d.Updates))
	}
	if len(d.Download) == 0 && len(d.Remove) == 0 {
		_, err := fmt.Fprintln(w, "Nothing to download or remove.")
		return err
	}
	if len(d.Download) > 0 {
		fmt.Fprintf(w, "download: %d (%s)\n", len(d.Download), tui.FormatSize(d.DownloadGB))
	}
	for _, item := range d.Download {
		line := fmt.Sprintf("  + %s  %s", item.ID, tui.FormatSize(item.SizeGB))
		if len(item.Requires) > 0 {
			line += "  builds with " + strings.Join(item.Requires, ", ")
		}
		fmt.Fprintln(w, line)
	}
	if len(d.Remove) > 0 {
		fmt.Fprintf(w, "remove: %d (%s)\n", len(d.Remove), tui.FormatSize(d.RemoveGB))
	}
	for _, item := range d.Remove {
		fmt.Fprintf(w, "  - %s  %s\n", item.ID, tui.FormatSize(item.SizeGB))
	}
	if d.BuildDownloadGB > 0 || d.BuildWorkGB > 0 {
		fmt.Fprintf(w, "builds: ~%.1f GB of sources, ~%.1f GB of staging space\n", d.BuildDownloadGB, d.BuildWorkGB)
	}
	_, err := fmt.Fprintln(w, "\nRun \"svalbard apply\" to make these changes.")
	return err
}
