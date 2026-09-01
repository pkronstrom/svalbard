package apply

import (
	"fmt"
	"sort"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func jobLevels(jobs []downloadJob, realized []manifest.RealizedEntry) ([][]downloadJob, error) {
	byID := make(map[string]downloadJob, len(jobs))
	for _, job := range jobs {
		byID[job.id] = job
	}
	done := make(map[string]bool, len(realized))
	for _, entry := range realized {
		done[entry.ID] = true
	}

	remaining := make(map[string]downloadJob, len(byID))
	for id, job := range byID {
		remaining[id] = job
	}
	var levels [][]downloadJob
	for len(remaining) > 0 {
		var ready []downloadJob
		for id, job := range remaining {
			dependencies := recipeDependencies(job.recipe)
			blocked := false
			for _, dependency := range dependencies {
				if done[dependency] {
					continue
				}
				if _, scheduled := byID[dependency]; !scheduled {
					return nil, fmt.Errorf("%s requires unavailable recipe %s", id, dependency)
				}
				blocked = true
			}
			if !blocked {
				ready = append(ready, job)
			}
		}
		if len(ready) == 0 {
			ids := make([]string, 0, len(remaining))
			for id := range remaining {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			return nil, fmt.Errorf("recipe dependency cycle: %v", ids)
		}
		sort.Slice(ready, func(i, j int) bool { return ready[i].id < ready[j].id })
		levels = append(levels, ready)
		for _, job := range ready {
			delete(remaining, job.id)
			done[job.id] = true
		}
	}
	return levels, nil
}

func recipeDependencies(recipe catalog.Item) []string {
	if recipe.Deps != nil {
		return recipe.Deps
	}
	switch recipe.Type {
	case "python-package":
		if recipe.Venv != "" {
			return []string{recipe.Venv}
		}
	case "python-venv":
		return []string{"uv"}
	}
	return nil
}
