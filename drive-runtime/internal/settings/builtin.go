package settings

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/llamaserve"
)

// Keys of the built-in settings.
const (
	KeyDefaultModel = "ai.default_model"
	KeyContext      = "ai.context"
)

// Ask is the choice that leaves a built-in setting to be decided at launch.
const Ask = "ask"

const askLabel = "ask every time"

const modelExt = ".gguf"

// builtinGroups returns the settings compiled into the runtime. The models are
// read from the drive, so the choices follow what is installed.
func builtinGroups(driveRoot string) []Group {
	modelChoices := []string{Ask}
	models, _ := llamaserve.ChatModels(driveRoot)
	for _, file := range models {
		if name := strings.TrimSuffix(file, modelExt); name != Ask {
			modelChoices = append(modelChoices, name)
		}
	}
	return []Group{{
		Owner: "ai",
		Label: "Local AI",
		Settings: []Setting{
			{
				Owner: "ai", Name: "default_model", Type: TypeChoice, Default: Ask,
				Label:        "Default model",
				Description:  "what Chat and the AI clients start with.",
				Choices:      modelChoices,
				ChoiceLabels: map[string]string{Ask: askLabel},
			},
			{
				Owner: "ai", Name: "context", Type: TypeChoice, Default: Ask,
				Label:       "Context size",
				Description: "how much of the conversation the model keeps in view. Larger is slower and needs more memory.",
				Choices:     []string{Ask, "auto", "fast", "balanced", "max"},
				ChoiceLabels: map[string]string{
					Ask:        askLabel,
					"auto":     "Auto",
					"fast":     "Fast",
					"balanced": "Balanced",
					"max":      "Max",
				},
				ChoiceNotes: map[string]string{
					"auto":     "sized for this computer's memory",
					"fast":     fmtTokens(llamaserve.CtxFast) + ", snappier, less memory",
					"balanced": fmtTokens(llamaserve.CtxBalanced),
					"max":      fmtTokens(llamaserve.CtxMax) + ", may be slow, high memory",
				},
			},
		},
	}}
}

func fmtTokens(tokens int) string {
	return fmt.Sprintf("%dK", tokens/1024)
}

// ContextSize returns the context window for a launch: the size ai.context
// fixes, or what ask returns when the setting leaves the choice to the user.
// autoCtx is the host-adaptive size, as ask takes it.
func (s *Store) ContextSize(autoCtx int, ask func(autoCtx int) int) int {
	switch s.String(KeyContext) {
	case "auto":
		return autoCtx
	case "fast":
		return llamaserve.CtxFast
	case "balanced":
		return llamaserve.CtxBalanced
	case "max":
		return llamaserve.CtxMax
	default:
		return ask(autoCtx)
	}
}

// ResolveModel returns the model file an action starts: the one it names,
// else ai.default_model, else whatever llamaserve.ResolveModel picks. A
// default model that has left the drive is reported on out in one line.
func (s *Store) ResolveModel(out io.Writer, selected string) (string, error) {
	if selected == "" {
		if name := s.savedDefaultModel(); name != "" {
			if path, err := llamaserve.ResolveModel(s.driveRoot, filepath.Join(s.driveRoot, "models", name+modelExt)); err == nil {
				return path, nil
			}
			fmt.Fprintf(out, "Default model %s is no longer on the drive; starting as if none were set.\n", name)
		}
	}
	return llamaserve.ResolveModel(s.driveRoot, selected)
}

// savedDefaultModel returns the model named by ai.default_model, whether or
// not it is still among the choices, and "" for ask.
func (s *Store) savedDefaultModel() string {
	setting, ok := s.Lookup(KeyDefaultModel)
	if !ok {
		return ""
	}
	stored, _ := s.stored(setting)
	name, _ := stored.(string)
	if name == Ask || strings.ContainsAny(name, `/\`) {
		return ""
	}
	return name
}
