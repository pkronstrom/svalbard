package llamaserve

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestModelFlagsUseOnlyTheMatchingInstalledProjector(t *testing.T) {
	models := t.TempDir()
	vision := filepath.Join(models, "vision")
	if err := os.Mkdir(vision, 0o755); err != nil {
		t.Fatal(err)
	}
	for model, projector := range map[string]string{
		"Inkling-2-Mini-UD-Q4_K_M.gguf": "mmproj-Inkling-2-Mini-F16.gguf",
		"Qwen_Qwen3.5-4B-Q4_K_M.gguf":   "mmproj-Qwen3.5-4B-BF16.gguf",
		"Qwen3.8-27B-UD-IQ4_XS.gguf":    "mmproj-Qwen3.8-27B-F16.gguf",
	} {
		path := filepath.Join(vision, projector)
		if err := os.WriteFile(path, []byte("sample"), 0o600); err != nil {
			t.Fatal(err)
		}
		flags := ExtraFlags(filepath.Join(models, model), 8192)
		at := slices.Index(flags, "--mmproj")
		if at < 0 || flags[at+1] != path {
			t.Fatalf("%s flags = %q, want projector %s", model, flags, path)
		}
	}
	if flags := ExtraFlags(filepath.Join(models, "Text-only.gguf"), 8192); slices.Contains(flags, "--mmproj") {
		t.Errorf("text model got a vision projector: %q", flags)
	}
}
