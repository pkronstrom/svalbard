package imaging

import (
	"image"
)

// Process resizes an image, extracts a palette, and dithers it.
func Process(img image.Image, maxWidth uint, numColors int) *image.Paletted {
	resized := Resize(img, maxWidth)
	pal := ExtractPalette(resized, numColors)
	return BayerDither(resized, pal)
}
