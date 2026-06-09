// Package llamaserve derives model-family-specific llama-server flags from a
// GGUF filename and the host's resources, so the on-drive runtime serves each
// model with sane settings without per-model config plumbing. Detection is
// filename-based because the runtime has no catalog access at serve time.
package llamaserve

import (
	"path/filepath"
	"strconv"
	"strings"
)

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

	// Published sampling recipes. llama.cpp's defaults (temp 0.8, top-k 40,
	// min-p 0.05) suit neither family, so set the vendor-recommended values for
	// any request that doesn't override them (notably the chat web UI).
	switch {
	case strings.Contains(name, "gemma"):
		// Google's Gemma 4 recipe: https://huggingface.co/google/gemma-4-31B-it
		flags = append(flags, "--temp", "1.0", "--top-k", "64", "--top-p", "0.95", "--min-p", "0.0")
	case strings.Contains(name, "qwen"):
		// Qwen's thinking/coding recipe (3.5 + 3.6): temp 0.6, top-k 20, min-p 0.
		flags = append(flags, "--temp", "0.6", "--top-k", "20", "--top-p", "0.95", "--min-p", "0.0")
	}

	// Qwen3.6 GGUF requires a q8_0 KV cache: q4/default desyncs its Gated
	// DeltaNet state and corrupts long generations (Unsloth guidance). Flash
	// attention is required for quantized V-cache and pairs with it.
	if strings.Contains(name, "qwen3.6") {
		flags = append(flags, "--flash-attn", "on", "--cache-type-k", "q8_0", "--cache-type-v", "q8_0")
	}

	// MTP-tagged builds carry a built-in multi-token-prediction head; enable
	// llama.cpp speculative decoding (ggml-org/llama.cpp#22673). It requires
	// --parallel 1, which is already the single-user default on the drive.
	if strings.Contains(name, "mtp") {
		flags = append(flags, "--spec-type", "draft-mtp", "--spec-draft-n-max", "2")
	}

	return flags
}
