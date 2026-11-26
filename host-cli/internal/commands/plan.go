package commands

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/pkronstrom/svalbard/host-cli/internal/apply"
	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
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
	p, _ := BuildPlan(m, cat)
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

// sized appends an estimated size to label in the given format, when there
// is one: a document collection has none.
func sized(label, format string, gb float64) string {
	if gb <= 0 {
		return label
	}
	return label + fmt.Sprintf(format, tui.FormatSize(gb))
}

// WritePlanDetail prints what apply would download and remove. Follow it
// with WritePlanNext, after anything else the plan reports.
func WritePlanDetail(w io.Writer, d PlanDetail) error {
	if len(d.Download) == 0 && len(d.Remove) == 0 {
		_, err := fmt.Fprintln(w, "Nothing to download or remove.")
		return err
	}
	if len(d.Download) > 0 {
		fmt.Fprintln(w, sized(fmt.Sprintf("download: %d", len(d.Download)), " (%s)", d.DownloadGB))
	}
	for _, item := range d.Download {
		line := sized("  + "+item.ID, "  %s", item.SizeGB)
		if len(item.Requires) > 0 {
			line += "  builds with " + strings.Join(item.Requires, ", ")
		}
		fmt.Fprintln(w, line)
	}
	if len(d.Remove) > 0 {
		fmt.Fprintln(w, sized(fmt.Sprintf("remove: %d", len(d.Remove)), " (%s)", d.RemoveGB))
	}
	for _, item := range d.Remove {
		fmt.Fprintln(w, sized("  - "+item.ID, "  %s", item.SizeGB))
	}
	if d.BuildDownloadGB > 0 || d.BuildWorkGB > 0 {
		fmt.Fprintf(w, "builds: ~%.1f GB of sources, ~%.1f GB of staging space\n", d.BuildDownloadGB, d.BuildWorkGB)
	}
	return nil
}

// WritePlanNext prints what to run next: apply for pending changes, and
// update for installed items whose recipe changed.
func WritePlanNext(w io.Writer, d PlanDetail) {
	if len(d.Download) > 0 || len(d.Remove) > 0 {
		fmt.Fprintln(w, "\nRun \"svalbard apply\" to make these changes.")
	}
	if len(d.Updates) > 0 {
		fmt.Fprintf(w, "\n%d update(s) available; run \"svalbard update\" to review them.\n", len(d.Updates))
	}
}

// vaultPlatforms returns the platforms m targets: the saved ones, or this
// computer's when none were saved, as apply assumes.
func vaultPlatforms(m manifest.Manifest) []string {
	if platforms := m.Desired.Options.HostPlatforms; len(platforms) > 0 {
		return platforms
	}
	return []string{apply.HostPlatform()}
}

// OtherPlatformItems returns the selected items that cannot be installed on
// m because their recipe has payloads only for platforms m does not target,
// such as a Linux-only tool on a vault made for macOS.
func OtherPlatformItems(cat *catalog.Catalog, m manifest.Manifest) []string {
	platforms := vaultPlatforms(m)
	ids := []string{}
	for _, id := range m.Desired.Items {
		recipe, ok := cat.RecipeByID(id)
		if !ok || len(recipe.Platforms) == 0 {
			continue
		}
		if !slices.ContainsFunc(platforms, recipe.SupportsPlatform) {
			ids = append(ids, id)
		}
	}
	return ids
}
