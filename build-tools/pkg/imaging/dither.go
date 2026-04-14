package imaging

import (
	"image"
	"image/color"
)

// Dither applies the named dithering algorithm to an image with a palette.
func Dither(img image.Image, pal color.Palette, algorithm string) *image.Paletted {
	switch algorithm {
	case "bayer":
		return BayerDither(img, pal)
	default:
		return BayerDither(img, pal)
	}
}

// Process resizes an image, extracts a palette, and dithers it.
func Process(img image.Image, maxWidth uint, numColors int, algorithm string) *image.Paletted {
	resized := Resize(img, maxWidth)
	pal := ExtractPalette(resized, numColors)
	return Dither(resized, pal, algorithm)
}
