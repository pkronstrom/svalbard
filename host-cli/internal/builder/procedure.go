package builder

import "github.com/pkronstrom/svalbard/host-cli/internal/catalog"

// ProcedureKind identifies one effect in a compiled build pipeline.
type ProcedureKind string

const (
	ProcedureDownload ProcedureKind = "download"
	ProcedureExtract  ProcedureKind = "extract"
	ProcedureTool     ProcedureKind = "tool"
	ProcedureVerify   ProcedureKind = "verify"
)

// Procedure is the durable execution seam between YAML and the build engine.
// V1 executes procedures linearly; explicit dependency scheduling is deferred.
type Procedure struct {
	ID          string
	Kind        ProcedureKind
	Source      string
	Destination string
	Tool        string
	Args        []string
	NotEmpty    bool
	MinSize     int64
	Image       string
	Fingerprint string
}

func procedureKind(step catalog.BuildStep) (ProcedureKind, string, error) {
	actions := 0
	kind := ProcedureKind("")
	value := ""
	set := func(candidate ProcedureKind, candidateValue string) {
		if candidateValue == "" {
			return
		}
		actions++
		kind = candidate
		value = candidateValue
	}
	set(ProcedureDownload, step.Download)
	set(ProcedureExtract, step.Extract)
	set(ProcedureTool, step.Exec)
	set(ProcedureTool, step.Tool)
	set(ProcedureVerify, step.Verify)
	if actions != 1 {
		return "", "", &ProcedureShapeError{Actions: actions}
	}
	return kind, value, nil
}

// ProcedureShapeError reports a step with zero or multiple actions.
type ProcedureShapeError struct{ Actions int }

func (e *ProcedureShapeError) Error() string {
	if e.Actions == 0 {
		return "no action specified"
	}
	return "multiple actions specified"
}
