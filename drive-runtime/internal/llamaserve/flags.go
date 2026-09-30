// Package llamaserve derives model-family-specific llama-server flags from a
// GGUF filename and the host's resources, so the on-drive runtime serves each
// model with sane settings without per-model config plumbing. Detection is
// filename-based because the runtime has no catalog access at serve time.
package llamaserve

import (
	_ "embed"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// qwen38ChatTemplate is the Unsloth Qwen3.8 template embedded in the pinned
// GGUFs with two changes that otherwise surface as HTTP 500s in harnesses: a
// system/developer message after the first turn is rendered as a system turn
// instead of raising "System message must be at the beginning" (see
// unsloth/Qwen3.8-27B-GGUF discussion #42), and Pi's "minimal"/"max" thinking
// levels map to the template's "low"/"xhigh" efforts.
//
//go:embed qwen3.8.jinja
var qwen38ChatTemplate string

// Context presets offered by the launch-time picker (in tokens).
const (
	CtxFast     = 8192   // snappier, less memory
	CtxBalanced = 32768  // middle ground
	CtxMax      = 131072 // host ceiling; may be slow / high memory
)

// ExtraFlags returns the llama-server flags implied by a model's filename for
// the given context size: a context cap, the published per-family sampling
// recipe, and any correctness/speed flags the family requires. Callers append
// the result to their base arg list ("-m", model, "--port", ...).
func ExtraFlags(modelPath string, ctxSize int) []string {
	name := strings.ToLower(filepath.Base(modelPath))

	flags := []string{"--ctx-size", strconv.Itoa(ctxSize)}
	for _, param := range modelSampler(name) {
		flags = append(flags, param.flag, param.value)
	}

	// Qwen3.6 GGUF requires a q8_0 KV cache: q4/default desyncs its Gated
	// DeltaNet state and corrupts long generations (Unsloth guidance). Flash
	// attention is required for quantized V-cache and pairs with it.
	if strings.Contains(name, "qwen3.6") || strings.Contains(name, "qwen3.8") {
		flags = append(flags, "--flash-attn", "on", "--cache-type-k", "q8_0", "--cache-type-v", "q8_0")
	}
	if strings.Contains(name, "qwen3.8") {
		flags = append(flags, "--reasoning-effort", "medium", "--reasoning-preserve", "--jinja", "--chat-template", qwen38ChatTemplate)
	}

	if draft, nMax, ok := packagedDraft(modelPath, name); ok {
		flags = append(flags, "--spec-type", "draft-mtp", "--spec-draft-model", draft, "--spec-draft-n-max", nMax, "--parallel", "1")
	}

	return flags
}

type samplerParam struct {
	flag  string
	key   string
	value string
}

// SamplingParams returns the matching model family's OpenAI-compatible
// sampling values. Pi uses these request defaults so they cannot override the
// equivalent llama-server defaults with an incompatible sampler.
func SamplingParams(modelName string) map[string]float64 {
	params := modelSampler(strings.ToLower(modelName))
	if len(params) == 0 {
		return nil
	}
	values := make(map[string]float64, len(params))
	for _, param := range params {
		value, _ := strconv.ParseFloat(param.value, 64)
		values[param.key] = value
	}
	return values
}

func modelSampler(name string) []samplerParam {
	switch {
	case strings.Contains(name, "qwen3.8"):
		return []samplerParam{
			{"--temp", "temperature", "1.0"}, {"--top-k", "top_k", "20"}, {"--top-p", "top_p", "0.95"}, {"--min-p", "min_p", "0.0"},
			{"--presence-penalty", "presence_penalty", "0.0"}, {"--repeat-penalty", "repeat_penalty", "1.0"},
		}
	case strings.Contains(name, "gemma"):
		return []samplerParam{
			{"--temp", "temperature", "1.0"}, {"--top-k", "top_k", "64"}, {"--top-p", "top_p", "0.95"}, {"--min-p", "min_p", "0.0"},
		}
	case strings.Contains(name, "qwen"):
		return []samplerParam{
			{"--temp", "temperature", "0.6"}, {"--top-k", "top_k", "20"}, {"--top-p", "top_p", "0.95"}, {"--min-p", "min_p", "0.0"},
		}
	default:
		return nil
	}
}

// packagedDraft returns the optional Qwen3.8 27B MTP drafter installed under
// models/draft/ (ai/mtp pack) when the target is one of the packaged quants.
// Its presence is the opt-in: the drafter carries the same MTP layer as the
// target, and speculation showed no decode gain on Apple Silicon Metal.
// Gemma 4 drafters are deliberately not matched (slower on Metal).
func packagedDraft(modelPath, name string) (string, string, bool) {
	if !strings.HasPrefix(name, "qwen3.8-27b-") || !strings.HasSuffix(name, ".gguf") ||
		!(strings.Contains(name, "ud-iq4_xs") || strings.Contains(name, "ud-q4_k_m") || strings.Contains(name, "ud-q6_k_xl")) {
		return "", "", false
	}
	draft := filepath.Join(filepath.Dir(modelPath), "draft", "mtp-Qwen3.8-27B-Q4_0.gguf")
	info, err := os.Stat(draft)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", false
	}
	return draft, "2", true
}
