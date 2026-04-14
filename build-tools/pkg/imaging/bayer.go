package imaging

import (
	"image"
	"image/color"
)

// Bayer 4x4 threshold matrix, normalized to 0-1 range.
var bayerMatrix = [4][4]float64{
	{0.0 / 16, 8.0 / 16, 2.0 / 16, 10.0 / 16},
	{12.0 / 16, 4.0 / 16, 14.0 / 16, 6.0 / 16},
	{3.0 / 16, 11.0 / 16, 1.0 / 16, 9.0 / 16},
	{15.0 / 16, 7.0 / 16, 13.0 / 16, 5.0 / 16},
}

// BayerDither applies Bayer 4x4 ordered dithering with a given palette.
func BayerDither(img image.Image, pal color.Palette) *image.Paletted {
	rgba := ToRGBA(img)
	bounds := rgba.Bounds()
	out := image.NewPaletted(bounds, pal)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := rgba.At(x, y).RGBA()

			threshold := bayerMatrix[y%4][x%4] - 0.5
			spread := 64.0

			nr := float64(r>>8) + threshold*spread
			ng := float64(g>>8) + threshold*spread
			nb := float64(b>>8) + threshold*spread

			adjusted := color.RGBA{
				R: clampByte(nr),
				G: clampByte(ng),
				B: clampByte(nb),
				A: uint8(a >> 8),
			}
			out.Set(x, y, pal.Convert(adjusted))
		}
	}
	return out
}
