package imaging

import (
	"image"
	"image/color"
	"testing"
)

func TestToRGBAReturnsExistingRGBA(t *testing.T) {
	original := image.NewRGBA(image.Rect(0, 0, 1, 1))
	if got := ToRGBA(original); got != original {
		t.Fatal("ToRGBA() copied an existing RGBA image")
	}
}

func TestToRGBAConvertsNonZeroBoundsAndPixels(t *testing.T) {
	bounds := image.Rect(5, 7, 7, 9)
	source := image.NewNRGBA(bounds)
	source.SetNRGBA(5, 7, color.NRGBA{R: 0x12, G: 0x34, B: 0x56, A: 0x78})
	source.SetNRGBA(6, 8, color.NRGBA{R: 0xab, G: 0xcd, B: 0xef, A: 0xff})

	got := ToRGBA(source)
	if got.Bounds() != bounds {
		t.Fatalf("ToRGBA() bounds = %v, want %v", got.Bounds(), bounds)
	}
	for _, point := range []image.Point{{X: 5, Y: 7}, {X: 6, Y: 8}} {
		gotColor := got.RGBAAt(point.X, point.Y)
		wantColor := color.RGBAModel.Convert(source.At(point.X, point.Y)).(color.RGBA)
		if gotColor != wantColor {
			t.Errorf("ToRGBA() pixel at %v = %#v, want %#v", point, gotColor, wantColor)
		}
	}
}
