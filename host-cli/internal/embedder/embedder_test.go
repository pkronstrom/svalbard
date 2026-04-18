package embedder

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestVectorBlobRoundTrip(t *testing.T) {
	original := []float32{0.1, -0.5, 3.14, 0, math.MaxFloat32}
	blob := VectorToBlob(original)

	if len(blob) != len(original)*4 {
		t.Fatalf("blob length = %d, want %d", len(blob), len(original)*4)
	}

	restored := BlobToVector(blob)
	if len(restored) != len(original) {
		t.Fatalf("restored length = %d, want %d", len(restored), len(original))
	}
	for i := range original {
		if restored[i] != original[i] {
			t.Errorf("restored[%d] = %v, want %v", i, restored[i], original[i])
		}
	}
}

func TestVectorBlobEmpty(t *testing.T) {
	blob := VectorToBlob(nil)
	if len(blob) != 0 {
		t.Fatalf("empty vector blob = %d bytes", len(blob))
	}
	restored := BlobToVector(nil)
	if len(restored) != 0 {
		t.Fatalf("empty blob restored = %d elements", len(restored))
	}
}

func TestParseEmbeddingFlat(t *testing.T) {
	raw := []byte(`[0.1, 0.2, 0.3]`)
	vec, err := parseEmbedding(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 3 {
		t.Fatalf("len = %d", len(vec))
	}
	if vec[0] < 0.09 || vec[0] > 0.11 {
		t.Errorf("vec[0] = %v", vec[0])
	}
}

func TestParseEmbeddingNested(t *testing.T) {
	raw := []byte(`[[0.4, 0.5]]`)
	vec, err := parseEmbedding(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 2 {
		t.Fatalf("len = %d", len(vec))
	}
}

func TestTruncateDims(t *testing.T) {
	vec := []float32{3.0, 4.0, 99.0, 99.0}
	result := TruncateDims(vec, 2)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	// 3/5=0.6, 4/5=0.8 (3-4-5 triangle)
	if math.Abs(float64(result[0])-0.6) > 1e-5 {
		t.Errorf("result[0] = %f, want 0.6", result[0])
	}
	if math.Abs(float64(result[1])-0.8) > 1e-5 {
		t.Errorf("result[1] = %f, want 0.8", result[1])
	}
	// Check magnitude is 1.0
	var mag float64
	for _, v := range result {
		mag += float64(v) * float64(v)
	}
	if math.Abs(mag-1.0) > 1e-6 {
		t.Errorf("not unit vector: magnitude^2 = %f", mag)
	}
}

func TestTruncateDimsNoTruncation(t *testing.T) {
	vec := []float32{0.6, 0.8}
	result := TruncateDims(vec, 10)
	if len(result) != 2 {
		t.Fatalf("should not grow: len = %d", len(result))
	}
}

func TestTruncateDimsZeroVector(t *testing.T) {
	vec := []float32{0, 0, 0}
	result := TruncateDims(vec, 2)
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}
	// Zero vector stays zero
	for _, v := range result {
		if v != 0 {
			t.Errorf("expected zero, got %f", v)
		}
	}
}

func TestFindEmbeddingModel(t *testing.T) {
	dir := t.TempDir()
	embedDir := filepath.Join(dir, "models", "embed")
	os.MkdirAll(embedDir, 0o755)

	// No model — should error.
	_, err := FindEmbeddingModel(dir)
	if err == nil {
		t.Fatal("expected error when no model present")
	}

	// Add a model.
	modelPath := filepath.Join(embedDir, "nomic-embed-text-v1.5.Q8_0.gguf")
	os.WriteFile(modelPath, []byte("fake"), 0o644)

	found, err := FindEmbeddingModel(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != modelPath {
		t.Errorf("found = %q, want %q", found, modelPath)
	}

	// macOS resource fork files should be skipped.
	dotPath := filepath.Join(embedDir, "._nomic-embed-text-v1.5.Q8_0.gguf")
	os.WriteFile(dotPath, []byte("fake"), 0o644)
	os.Remove(modelPath)

	_, err = FindEmbeddingModel(dir)
	if err == nil {
		t.Fatal("expected error when only ._ file present")
	}
}
