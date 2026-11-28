package llamaserve

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRouterUsesEachModelsDeclaredContext(t *testing.T) {
	dir := t.TempDir()
	for name, count := range map[string]uint32{"qwen.gguf": 262144, "inkling.gguf": 1048576} {
		var header bytes.Buffer
		put := func(value any) {
			t.Helper()
			if err := binary.Write(&header, binary.LittleEndian, value); err != nil {
				t.Fatal(err)
			}
		}
		putString := func(value string) {
			put(uint64(len(value)))
			header.WriteString(value)
		}
		header.WriteString("GGUF")
		put(uint32(3))
		put(uint64(0))
		put(uint64(3))
		putString("example.context_length")
		put(uint32(4))
		put(count)
		putString("tokenizer.pre")
		put(uint32(9))
		put(uint32(8))
		put(uint64(1))
		putString("example")
		putString("general.architecture")
		put(uint32(8))
		putString("example")
		if err := os.WriteFile(filepath.Join(dir, name), header.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		want      string
		requested int
	}{{"ctx-size = 524288", Ctx512K}, {"ctx-size = 1048576", CtxModelMax}} {
		preset, floor, err := WriteRouterPresets(dir, dir, []string{"qwen.gguf", "inkling.gguf"}, tc.requested)
		if err != nil || floor != 262144 {
			t.Fatalf("router contexts: floor=%d, err=%v", floor, err)
		}
		data, err := os.ReadFile(preset)
		if err != nil {
			t.Fatal(err)
		}
		config := string(data)
		qwen, inkling, _ := strings.Cut(config, "[inkling]")
		if !strings.Contains(qwen, "ctx-size = 262144") || !strings.Contains(inkling, tc.want) {
			t.Fatalf("router did not clamp each model individually:\n%s", config)
		}
	}
}
