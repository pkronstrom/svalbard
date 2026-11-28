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
	CtxFast     = 8192 // snappier, less memory
	Ctx16K      = 16384
	CtxBalanced = 32768 // middle ground
	Ctx64K      = 65536
	CtxMax      = 131072 // legacy 128K setting
	Ctx256K     = 262144
	Ctx512K     = 524288
	CtxModelMax = -1 // resolve to the model's declared context before launch
)

// ExtraFlags returns the llama-server flags implied by a model's filename for
// the given context size: a context cap, a single slot, the published
// per-family sampling recipe, and any correctness/speed flags the family
// requires. Callers append the result to their base arg list ("-m", model,
// "--port", ...).
//
// The default auto slot count (4) shares one ctxSize KV pool between slots, so
// two overlapping long requests (a harness firing parallel calls, two LAN
// users) both fail with "Context size has been exceeded" (b11840). One slot
// queues the second request and gives each the full context.
func ExtraFlags(modelPath string, ctxSize int) []string {
	var flags []string
	for _, option := range modelOptions(modelPath, ctxSize) {
		flags = append(flags, option.flag)
		if option.value != "" {
			flags = append(flags, option.value)
		}
	}
	return flags
}

type modelOption struct {
	flag  string
	value string // empty for switches that take no argument
}

func modelOptions(modelPath string, ctxSize int) []modelOption {
	name := strings.ToLower(filepath.Base(modelPath))

	options := []modelOption{{"--ctx-size", strconv.Itoa(ctxSize)}, {"--parallel", "1"}}
	for _, param := range modelSampler(name) {
		options = append(options, modelOption{param.flag, param.value})
	}

	// Qwen3.6 GGUF requires a q8_0 KV cache: q4/default desyncs its Gated
	// DeltaNet state and corrupts long generations (Unsloth guidance). Flash
	// attention is required for quantized V-cache and pairs with it.
	if strings.Contains(name, "qwen3.6") || strings.Contains(name, "qwen3.8") {
		options = append(options, modelOption{"--flash-attn", "on"}, modelOption{"--cache-type-k", "q8_0"}, modelOption{"--cache-type-v", "q8_0"})
	}
	if strings.Contains(name, "qwen3.8") {
		options = append(options, modelOption{"--reasoning-effort", "medium"}, modelOption{"--reasoning-preserve", ""}, modelOption{"--jinja", ""}, modelOption{"--chat-template", qwen38ChatTemplate})
	}
	if strings.Contains(name, "inkling-2") {
		options = append(options, modelOption{"--jinja", ""})
	}

	if draft, mode, nMax, ok := packagedDraft(modelPath, name); ok {
		options = append(options, modelOption{"--spec-type", mode}, modelOption{"--spec-draft-model", draft}, modelOption{"--spec-draft-n-max", nMax})
	}
	if projector := VisionProjector(modelPath); projector != "" {
		options = append(options, modelOption{"--mmproj", projector})
	}

	return options
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
	case strings.Contains(name, "inkling-2"):
		return []samplerParam{
			{"--temp", "temperature", "1.0"}, {"--top-k", "top_k", "0"}, {"--top-p", "top_p", "1.0"}, {"--min-p", "min_p", "0.0"},
		}
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

// packagedDraft enables speculation when a supported target has its optional
// companion installed under models/draft/.
func packagedDraft(modelPath, name string) (path, mode, nMax string, ok bool) {
	var filename string
	switch {
	case name == "inkling-2-mini-ud-q4_k_m.gguf":
		filename, mode, nMax = "dspark-Inkling-2-Mini-Q8_0.gguf", "draft-dspark", "5"
	case strings.HasPrefix(name, "qwen3.8-27b-") && strings.HasSuffix(name, ".gguf") &&
		(strings.Contains(name, "ud-iq4_xs") || strings.Contains(name, "ud-q4_k_m") || strings.Contains(name, "ud-q6_k_xl")):
		filename, mode, nMax = "mtp-Qwen3.8-27B-Q4_0.gguf", "draft-mtp", "2"
	default:
		return "", "", "", false
	}
	path = filepath.Join(filepath.Dir(modelPath), "draft", filename)
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", "", false
	}
	return path, mode, nMax, true
}
