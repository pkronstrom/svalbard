package llamaserve

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExtraFlags(t *testing.T) {
	ctx := []string{"--ctx-size", "32768"}
	qwenSampling := []string{"--temp", "0.6", "--top-k", "20", "--top-p", "0.95", "--min-p", "0.0"}
	qwen38Sampling := []string{"--temp", "1.0", "--top-k", "20", "--top-p", "0.95", "--min-p", "0.0", "--presence-penalty", "0.0", "--repeat-penalty", "1.0"}
	gemmaSampling := []string{"--temp", "1.0", "--top-k", "64", "--top-p", "0.95", "--min-p", "0.0"}
	qwen36KV := []string{"--flash-attn", "on", "--cache-type-k", "q8_0", "--cache-type-v", "q8_0"}

	join := func(parts ...[]string) []string {
		var out []string
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}

	cases := []struct {
		name string
		path string
		want []string
	}{
		{"qwen3.5 gets ctx + sampling", "/d/models/Qwen3.5-9B-Q4_K_M.gguf", join(ctx, qwenSampling)},
		{"gemma gets ctx + sampling", "/d/models/gemma-4-31B-it-qat-UD-Q4_K_XL.gguf", join(ctx, gemmaSampling)},
		{"qwen3.6 moe adds kv guard", "/d/models/Qwen3.6-35B-A3B-UD-Q4_K_M.gguf", join(ctx, qwenSampling, qwen36KV)},
		{"qwen3.6 mtp filename does not imply a built-in draft", "/d/models/Qwen3.6-27B-MTP-UD-Q4_K_XL.gguf", join(ctx, qwenSampling, qwen36KV)},
		{"case insensitive", "/d/models/qwen3.6-27b-mtp.gguf", join(ctx, qwenSampling, qwen36KV)},
		{"qwen3.8 uses native sampler and medium reasoning", "/d/models/Qwen3.8-27B-UD-Q4_K_M.gguf", join(ctx, qwen38Sampling, qwen36KV, []string{"--reasoning-effort", "medium", "--reasoning-preserve", "--jinja", "--chat-template", qwen38ChatTemplate})},
		{"unrelated mtp filename does not enable speculation", "/d/models/custom-mtp.gguf", ctx},
		{"unknown family gets ctx only", "/d/models/phi-4-Q4_K_M.gguf", ctx},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtraFlags(tc.path, 32768)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("ExtraFlags(%q) =\n  %v\nwant\n  %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestExtraFlagsUsesPackagedDraftOnlyForMatchingTarget(t *testing.T) {
	for _, tc := range []struct{ target, draft, count string }{
		{"Qwen3.8-27B-UD-IQ4_XS.gguf", "mtp-Qwen3.8-27B-Q4_0.gguf", "2"},
		{"Qwen3.8-27B-UD-Q4_K_M.gguf", "mtp-Qwen3.8-27B-Q4_0.gguf", "2"},
		{"Qwen3.8-27B-UD-Q6_K_XL.gguf", "mtp-Qwen3.8-27B-Q4_0.gguf", "2"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			models := filepath.Join(t.TempDir(), "models with spaces")
			target := filepath.Join(models, tc.target)
			draft := filepath.Join(models, "draft", tc.draft)
			if strings.Contains(strings.Join(ExtraFlags(target, 8192), " "), "--spec-draft-model") {
				t.Fatal("missing draft should allow ordinary decoding")
			}
			if err := os.MkdirAll(draft, 0755); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(strings.Join(ExtraFlags(target, 8192), " "), "--spec-draft-model") {
				t.Fatal("draft directory should allow ordinary decoding")
			}
			if err := os.Remove(draft); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(draft), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(draft, []byte("fixture"), 0644); err != nil {
				t.Fatal(err)
			}
			got := ExtraFlags(target, 8192)
			want := []string{"--spec-type", "draft-mtp", "--spec-draft-model", draft, "--spec-draft-n-max", tc.count, "--parallel", "1"}
			if len(got) < len(want) || !reflect.DeepEqual(got[len(got)-len(want):], want) {
				t.Fatalf("packaged MTP flags = %v; want suffix %v", got, want)
			}
			if strings.Contains(strings.Join(ExtraFlags(filepath.Join(models, "other.gguf"), 8192), " "), "--spec-type") {
				t.Fatal("draft must not be attached to a different model")
			}
			if strings.Contains(strings.Join(ExtraFlags(filepath.Join(models, "Qwen3.8-35B-A3B-UD-Q4_K_XL.gguf"), 8192), " "), "--spec-type") {
				t.Fatal("Qwen3.8 27B draft must not be attached to other sizes")
			}
		})
	}
}

// Gemma 4 GGUF drafters decode slower than no drafter on Metal (M4 Max,
// b11146: 12B QAT n-max 4 -25%, n-max 2 at best break-even), so a present
// Gemma drafter must not enable speculation.
func TestExtraFlagsIgnoresGemmaDraft(t *testing.T) {
	models := t.TempDir()
	if err := os.MkdirAll(filepath.Join(models, "draft"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(models, "draft", "mtp-gemma-4-12B-it.gguf"), []byte("fixture"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ExtraFlags(filepath.Join(models, "gemma-4-12B-it-qat-UD-Q4_K_XL.gguf"), 8192), " "); strings.Contains(got, "--spec-type") {
		t.Fatalf("Gemma must decode without speculation, got %s", got)
	}
}

func TestQwen38TemplateAcceptsLaterSystemMessages(t *testing.T) {
	if strings.Contains(qwen38ChatTemplate, "System message must be at the beginning") {
		t.Fatal("embedded Qwen3.8 template still rejects mid-conversation system messages")
	}
	if !strings.Contains(qwen38ChatTemplate, "resolved_reasoning_effort == 'minimal'") {
		t.Fatal("embedded Qwen3.8 template must accept Pi's minimal thinking level")
	}
	if !strings.Contains(qwen38ChatTemplate, "<tool_call>") {
		t.Fatal("embedded Qwen3.8 template lost its tool-call rendering")
	}
}

func TestExtraFlagsThreadsContext(t *testing.T) {
	got := ExtraFlags("/d/models/phi-4.gguf", 8192)
	if len(got) < 2 || got[0] != "--ctx-size" || got[1] != "8192" {
		t.Fatalf("ExtraFlags ctx = %v, want --ctx-size 8192 first", got)
	}
}

func TestContextForHostTiers(t *testing.T) {
	orig := totalRAMBytes
	t.Cleanup(func() { totalRAMBytes = orig })

	cases := []struct {
		name string
		gib  uint64
		want int
	}{
		{"detection failure", 0, CtxFast},
		{"8GB", 8, CtxFast},
		{"16GB", 16, CtxBalanced},
		{"24GB", 24, CtxBalanced},
		{"32GB", 32, CtxBalanced},
		{"64GB", 64, CtxMax},
		{"128GB", 128, CtxMax},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			totalRAMBytes = func() uint64 { return tc.gib * giB }
			if got := ContextForHost(); got != tc.want {
				t.Errorf("ContextForHost() for %dGB = %d, want %d", tc.gib, got, tc.want)
			}
		})
	}
}
