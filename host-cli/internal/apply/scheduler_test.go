package apply

import (
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

func TestJobLevelsOrdersPythonRuntimeDependencies(t *testing.T) {
	jobs := []downloadJob{
		{id: "svalbard-python", recipe: catalog.Item{ID: "svalbard-python", Type: "python-venv"}},
		{id: "uv", recipe: catalog.Item{ID: "uv", Type: "binary"}},
		{id: "other", recipe: catalog.Item{ID: "other", Type: "binary"}},
	}
	levels, err := jobLevels(jobs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(levels) != 2 {
		t.Fatalf("levels = %v", levelIDs(levels))
	}
	if got := strings.Join(levelIDs(levels)[0], ","); got != "other,uv" {
		t.Fatalf("first level = %s", got)
	}
	if got := strings.Join(levelIDs(levels)[1], ","); got != "svalbard-python" {
		t.Fatalf("second level = %s", got)
	}
}

func TestJobLevelsAcceptsRealizedDependency(t *testing.T) {
	jobs := []downloadJob{{id: "svalbard-python", recipe: catalog.Item{ID: "svalbard-python", Type: "python-venv"}}}
	levels, err := jobLevels(jobs, []manifest.RealizedEntry{{ID: "uv"}})
	if err != nil || len(levels) != 1 {
		t.Fatalf("levels = %v, err = %v", levelIDs(levels), err)
	}
}

func TestJobLevelsRejectsMissingDependency(t *testing.T) {
	jobs := []downloadJob{{id: "dependent", recipe: catalog.Item{ID: "dependent", Deps: []string{"missing"}}}}
	if _, err := jobLevels(jobs, nil); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("error = %v", err)
	}
}

func TestJobLevelsRejectsCycle(t *testing.T) {
	jobs := []downloadJob{
		{id: "a", recipe: catalog.Item{ID: "a", Deps: []string{"b"}}},
		{id: "b", recipe: catalog.Item{ID: "b", Deps: []string{"a"}}},
	}
	if _, err := jobLevels(jobs, nil); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("error = %v", err)
	}
}

func levelIDs(levels [][]downloadJob) [][]string {
	result := make([][]string, len(levels))
	for i, level := range levels {
		for _, job := range level {
			result[i] = append(result[i], job.id)
		}
	}
	return result
}
