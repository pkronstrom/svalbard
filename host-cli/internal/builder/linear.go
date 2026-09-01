package builder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RunLinear executes procedures in order and records reusable completion
// fingerprints in the recipe's persistent staging directory.
func RunLinear(ctx context.Context, root, workdir, recipeID string, procedures []Procedure, opts Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	markerDir := filepath.Join(workdir, ".procedures")
	if err := os.MkdirAll(markerDir, 0o755); err != nil {
		return err
	}
	for _, procedure := range procedures {
		if err := ctx.Err(); err != nil {
			return err
		}
		emit(opts, BuildEvent{RecipeID: recipeID, Procedure: procedure.ID, State: EventQueued})
		var cacheKey string
		var inputDigests map[string]string
		if len(procedure.Outputs) > 1 {
			return fmt.Errorf("%s: cacheable procedure must declare one output directory", procedure.ID)
		}
		if len(procedure.Outputs) == 1 && !pathWithin(procedure.Outputs[0], workdir) {
			return fmt.Errorf("%s: cacheable output must be inside recipe staging", procedure.ID)
		}
		if len(procedure.Outputs) == 1 {
			var err error
			cacheKey, inputDigests, err = blockCacheKey(procedure)
			if err != nil {
				return fmt.Errorf("%s: %w", procedure.ID, err)
			}
			hit, err := restoreBlockCache(root, cacheKey, procedure.Outputs[0])
			if err != nil {
				return fmt.Errorf("%s cache restore: %w", procedure.ID, err)
			}
			if hit {
				emit(opts, BuildEvent{RecipeID: recipeID, Procedure: procedure.ID, State: EventSkipped, Message: "cache hit " + procedure.ID})
				continue
			}
		} else if procedureReusable(procedure, filepath.Join(markerDir, procedure.ID)) {
			emit(opts, BuildEvent{RecipeID: recipeID, Procedure: procedure.ID, State: EventSkipped, Message: "reusing " + procedure.ID})
			continue
		}
		emit(opts, BuildEvent{RecipeID: recipeID, Procedure: procedure.ID, State: EventStarted, Message: procedureMessage(procedure)})
		if err := executeProcedure(ctx, root, workdir, procedure); err != nil {
			emit(opts, BuildEvent{RecipeID: recipeID, Procedure: procedure.ID, State: EventFailed, Error: err.Error()})
			return fmt.Errorf("%s: %w", procedure.ID, err)
		}
		if len(procedure.Outputs) == 1 {
			if err := storeBlockCache(root, cacheKey, procedure, inputDigests, procedure.Outputs[0]); err != nil {
				return fmt.Errorf("%s cache store: %w", procedure.ID, err)
			}
		} else if err := os.WriteFile(filepath.Join(markerDir, procedure.ID), []byte(procedure.Fingerprint+"\n"), 0o644); err != nil {
			return err
		}
		emit(opts, BuildEvent{RecipeID: recipeID, Procedure: procedure.ID, State: EventCompleted})
	}
	return nil
}

func executeProcedure(ctx context.Context, root, workdir string, procedure Procedure) error {
	switch procedure.Kind {
	case ProcedureDownload:
		return stepDownload(ctx, procedure.Source, procedure.Destination)
	case ProcedureExtract:
		return stepExtract(procedure.Source, procedure.Destination)
	case ProcedureTool:
		if len(procedure.Outputs) > 0 {
			return stepExecBlock(ctx, root, workdir, procedure)
		}
		return stepExec(ctx, root, workdir, procedure.Tool, procedure.Args, procedure.Image)
	case ProcedureVerify:
		return stepVerify(procedure.Source, procedure.NotEmpty, procedure.MinSize)
	default:
		return fmt.Errorf("unknown procedure kind %q", procedure.Kind)
	}
}
func procedureReusable(procedure Procedure, marker string) bool {
	data, err := os.ReadFile(marker)
	if err != nil || strings.TrimSpace(string(data)) != procedure.Fingerprint {
		return false
	}
	path := ""
	switch procedure.Kind {
	case ProcedureDownload, ProcedureExtract:
		path = procedure.Destination
	case ProcedureVerify:
		path = procedure.Source
	default:
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

func procedureMessage(procedure Procedure) string {
	switch procedure.Kind {
	case ProcedureDownload:
		return "downloading " + filepath.Base(procedure.Destination)
	case ProcedureExtract:
		return "extracting"
	case ProcedureTool:
		return procedure.Tool
	case ProcedureVerify:
		return "verifying " + filepath.Base(procedure.Source)
	default:
		return string(procedure.Kind)
	}
}
