package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/llamaserve"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
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

	updated := upsertGooseExtensionYAML(existing, gooseExtensionBlock("/bin/svalbard-drive", "/tmp/drive", 32768))

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

// What pi's requests are made of, in Qwen3.5 tokens, measured with
// llama-server's /tokenize on what pi 1.6.1 sent through a logging proxy.
const (
	// piOwnTokens is pi's system prompt without the drive's context (656),
	// its four tools (720) and the chat template around them (about 240).
	piOwnTokens = 1650
	// promptCharsPerToken: the drive context measured 3.6 characters a
	// token and the tool definitions 3.9 to 4.0; 3.5 errs high.
	promptCharsPerToken = 3.5
	// pageCharsPerToken: a WikEM page as pi received it from search_read,
	// JSON with its links, was 6,887 characters and 2,460 tokens.
	pageCharsPerToken = 2.8
	// pageLinkChars is one listed link with its title, and pageFrameChars
	// the rest of a read result: source, path, title and a note.
	pageLinkChars, pageFrameChars = 60, 150
	// summaryTokens is what pi's compaction leaves of the earlier turns, as
	// the 4B model writes it.
	summaryTokens = 1000
	// answerTokens is the least an answer needs: a paragraph or two.
	answerTokens = 1024
)

// piPromptTokens is pi's prompt on a drive with every area installed, with
// the drive context and the tool definitions the drive gives pi at ctxSize.
func piPromptTokens(t *testing.T, driveRoot string, ctxSize int) int {
	t.Helper()
	server := mcp.NewDriveServer(driveRoot, mcp.DriveOptions{Context: ctxSize})
	defer server.Close()
	var tools []any
	for _, tool := range server.Tools() {
		// The shape pi sends to an OpenAI-compatible server.
		tools = append(tools, map[string]any{"type": "function", "function": map[string]any{
			"name": PiToolPrefix + tool.Name, "description": tool.Description, "parameters": tool.InputSchema,
		}})
	}
	definitions, err := json.Marshal(tools)
	if err != nil {
		t.Fatal(err)
	}
	chars := len(driveContextFor(driveRoot, PiToolPrefix, ctxSize)) + len(definitions)
	return piOwnTokens + int(float64(chars)/promptCharsPerToken)
}

// pi 1.6.1 sends max_completion_tokens = min(maxTokens, contextWindow -
// prompt - 4096). These are the numbers that leaves for an answer, first on
// a short question and then after a lookup, when pi has compacted: it keeps
// the last tool result whole and puts a summary in place of the rest.
func TestPiHasRoomToAnswerOnEveryTier(t *testing.T) {
	driveRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(driveRoot, ".svalbard"), 0o755); err != nil {
		t.Fatal(err)
	}
	recipes := `{"silverbullet": {"id": "silverbullet", "type": "binary"}, "skills-core": {"id": "skills-core", "type": "skills"}, "js-core": {"id": "js-core", "type": "js-lib"}}`
	if err := os.WriteFile(filepath.Join(driveRoot, ".svalbard", "recipes.json"), []byte(recipes), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, ctx := range []int{llamaserve.CtxFast, llamaserve.CtxBalanced, llamaserve.CtxMax} {
		prompt := piPromptTokens(t, driveRoot, ctx)
		room := min(piMaxTokens(ctx), piContextWindow(ctx)-prompt-piSafetyTokens)
		if room < 2000 {
			t.Errorf("context %d: pi may write %d tokens after a %d-token prompt, want room for an answer", ctx, room, prompt)
		}
		// What pi believes is free must really be free in llama-server's
		// window, with some left for the error in pi's estimate.
		if slack := ctx - (piContextWindow(ctx) - piSafetyTokens); slack < 512 {
			t.Errorf("context %d: pi would fill the window to within %d tokens", ctx, slack)
		}
		// Compaction starts before the real window is full, and not so early
		// that the prompt alone is past it.
		threshold := piContextWindow(ctx) - piReserveTokens(ctx)
		if threshold >= ctx || threshold < prompt+1500 {
			t.Errorf("context %d: pi compacts at %d tokens", ctx, threshold)
		}
		// pi compacts only once the context is past the threshold, so just
		// below it the answer gets what is left between the threshold and
		// what pi fills. That must hold an answer.
		fillable := piContextWindow(ctx) - piSafetyTokens
		if fillable-threshold < answerTokens {
			t.Errorf("context %d: just below pi's threshold of %d an answer gets %d tokens, want %d", ctx, threshold, fillable-threshold, answerTokens)
		}
		// After a lookup and a compaction: the prompt, the recent messages
		// pi keeps (at least the page read last, whole) and the summary.
		// That must be under the threshold, or pi compacts again at the
		// next request, and leave an answer room in what pi may fill.
		read := mcp.ReadLimitsFor(ctx)
		page := int(float64(read.Chars+read.Links*pageLinkChars+pageFrameChars) / pageCharsPerToken)
		compacted := prompt + max(page, piKeepRecentTokens(ctx)) + summaryTokens
		if compacted >= threshold {
			t.Errorf("context %d: after compaction the context is %d tokens (prompt %d, kept %d, summary %d), at or past pi's threshold of %d",
				ctx, compacted, prompt, max(page, piKeepRecentTokens(ctx)), summaryTokens, threshold)
		}
		if compacted+answerTokens > fillable {
			t.Errorf("context %d: after a lookup pi may write %d tokens (prompt %d, page %d, summary %d), want %d",
				ctx, fillable-compacted, prompt, page, summaryTokens, answerTokens)
		}
		t.Logf("context %d: prompt %d, page %d, threshold %d, after compaction %d, answer room %d to %d", ctx, prompt, page, threshold, compacted, fillable-threshold, fillable-compacted)
	}
	// From the Balanced tier up nothing is adjusted.
	if piContextWindow(llamaserve.CtxBalanced) != llamaserve.CtxBalanced || piReserveTokens(llamaserve.CtxBalanced) != 8192 {
		t.Errorf("balanced tier: window %d, reserve %d", piContextWindow(llamaserve.CtxBalanced), piReserveTokens(llamaserve.CtxBalanced))
	}
}
