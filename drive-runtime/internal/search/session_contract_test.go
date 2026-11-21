package search

import (
	"testing"
)

func TestShapeQueryTruncatesAndNormalizesPermittedNomicVector(t *testing.T) {
	contract := SemanticContract{Version: 1, Model: "models/embed/model.gguf", ModelSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Dimensions: 2, Matryoshka: true}
	got, err := shapeQuery([]float32{3, 4, 9}, contract)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != 0.6 || got[1] != 0.8 {
		t.Fatalf("shaped vector = %v", got)
	}
}
