package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/llamaserve"
)

func TestOpenRuntimeLogFileCreatesClientScopedLog(t *testing.T) {
	driveRoot := t.TempDir()

	file, path, err := openRuntimeLogFile(driveRoot, "opencode", "llama-server.log")
	if err != nil {
		t.Fatalf("openRuntimeLogFile() error = %v", err)
	}
	defer file.Close()

	want := filepath.Join(driveRoot, ".svalbard", "runtime", "opencode", "llama-server.log")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if _, err := os.Stat(filepath.Dir(path)); err != nil {
		t.Fatalf("runtime log dir missing: %v", err)
	}
}

func TestUpsertGooseExtensionYAMLPreservesExistingExtensions(t *testing.T) {
	existing := `extensions:
  developer:
    enabled: true
    type: platform
name: test
`

	updated := upsertGooseExtensionYAML(existing, gooseExtensionBlock("/bin/svalbard-drive", "/tmp/drive"))

	if !strings.Contains(updated, "  developer:") {
		t.Fatalf("updated config lost existing extension:\n%s", updated)
	}
	if !strings.Contains(updated, "  svalbard:") {
		t.Fatalf("updated config missing svalbard extension:\n%s", updated)
	}
	if !strings.Contains(updated, "name: test") {
		t.Fatalf("updated config lost top-level keys:\n%s", updated)
	}
}

func TestOmpIsRefusedBelowTheBalancedContext(t *testing.T) {
	err := checkClientContext("omp", 8192)
	if err == nil {
		t.Fatal("omp accepted at 8K, where its prompt leaves no room for work")
	}
	for _, want := range []string{"32K", "8K", "Balanced", "pi"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if err := checkClientContext("omp", 32768); err != nil {
		t.Errorf("omp refused at 32K: %v", err)
	}
	if err := checkClientContext("pi", 8192); err != nil {
		t.Errorf("pi refused at 8K: %v", err)
	}
}

// pi 1.6.1 sends max_completion_tokens = min(maxTokens, contextWindow -
// prompt - 4096). These are the numbers that leaves for an answer.
func TestPiHasRoomToAnswerOnEveryTier(t *testing.T) {
	const wikiAndSkillsPrompt = 4950 // pi's own estimate of its first request on a drive with both
	for _, ctx := range []int{llamaserve.CtxFast, llamaserve.CtxBalanced, llamaserve.CtxMax} {
		room := min(piMaxTokens(ctx), piContextWindow(ctx)-wikiAndSkillsPrompt-piSafetyTokens)
		if room < 2000 {
			t.Errorf("context %d: pi may write %d tokens after a %d-token prompt, want room for an answer", ctx, room, wikiAndSkillsPrompt)
		}
		// What pi believes is free must really be free in llama-server's
		// window, with some left for the error in pi's estimate.
		if slack := ctx - (piContextWindow(ctx) - piSafetyTokens); slack < 512 {
			t.Errorf("context %d: pi would fill the window to within %d tokens", ctx, slack)
		}
		// Compaction starts before the real window is full, and not so early
		// that the prompt alone is past it.
		threshold := piContextWindow(ctx) - piReserveTokens(ctx)
		if threshold >= ctx || threshold < wikiAndSkillsPrompt+1500 {
			t.Errorf("context %d: pi compacts at %d tokens", ctx, threshold)
		}
	}
	// From the Balanced tier up nothing is adjusted.
	if piContextWindow(llamaserve.CtxBalanced) != llamaserve.CtxBalanced || piReserveTokens(llamaserve.CtxBalanced) != 8192 {
		t.Errorf("balanced tier: window %d, reserve %d", piContextWindow(llamaserve.CtxBalanced), piReserveTokens(llamaserve.CtxBalanced))
	}
}
