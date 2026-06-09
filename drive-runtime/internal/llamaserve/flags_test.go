package llamaserve

import (
	"reflect"
	"testing"
)

func TestExtraFlags(t *testing.T) {
	ctx := []string{"--ctx-size", "32768"}
	qwenSampling := []string{"--temp", "0.6", "--top-k", "20", "--top-p", "0.95", "--min-p", "0.0"}
	gemmaSampling := []string{"--temp", "1.0", "--top-k", "64", "--top-p", "0.95", "--min-p", "0.0"}
	qwen36KV := []string{"--flash-attn", "on", "--cache-type-k", "q8_0", "--cache-type-v", "q8_0"}
	mtp := []string{"--spec-type", "draft-mtp", "--spec-draft-n-max", "2"}

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
		{"qwen3.6 mtp adds kv guard + spec", "/d/models/Qwen3.6-27B-MTP-UD-Q4_K_XL.gguf", join(ctx, qwenSampling, qwen36KV, mtp)},
		{"case insensitive", "/d/models/qwen3.6-27b-mtp.gguf", join(ctx, qwenSampling, qwen36KV, mtp)},
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
		{"24GB", 24, 65536},
		{"32GB", 32, 65536},
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
