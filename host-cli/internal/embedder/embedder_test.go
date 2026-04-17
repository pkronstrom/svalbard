package embedder

import (
	"math"
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
