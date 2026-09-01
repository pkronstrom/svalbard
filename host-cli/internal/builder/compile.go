package builder

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

// CompileProcedures validates and resolves YAML steps into stable linear procedures.
func CompileProcedures(steps []catalog.BuildStep, vars map[string]string) ([]Procedure, error) {
	procedures := make([]Procedure, 0, len(steps))
	for index, step := range steps {
		kind, action, err := procedureKind(step)
		if err != nil {
			return nil, fmt.Errorf("step %d: %w", index+1, err)
		}
		args := make([]string, len(step.Args))
		for i, arg := range step.Args {
			args[i] = resolve(arg, vars)
		}
		procedure := Procedure{
			ID:       fmt.Sprintf("%03d-%s", index+1, kind),
			Kind:     kind,
			Args:     args,
			NotEmpty: step.NotEmpty,
			MinSize:  step.MinSize,
			Image:    step.DockerImage,
		}
		switch kind {
		case ProcedureDownload:
			procedure.Source = resolve(action, vars)
			procedure.Destination = resolve(step.Dest, vars)
		case ProcedureExtract:
			procedure.Source = resolve(action, vars)
			procedure.Destination = resolve(step.Dest, vars)
		case ProcedureTool:
			procedure.Tool = resolve(action, vars)
		case ProcedureVerify:
			procedure.Source = resolve(action, vars)
		}
		procedure.Fingerprint, err = procedureFingerprint(procedure)
		if err != nil {
			return nil, fmt.Errorf("step %d fingerprint: %w", index+1, err)
		}
		procedures = append(procedures, procedure)
	}
	return procedures, nil
}

func procedureFingerprint(procedure Procedure) (string, error) {
	copy := procedure
	copy.Fingerprint = ""
	data, err := json.Marshal(copy)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
