package imaging

import (
	"image"
	"image/color"
	"math"
	"math/rand"
	"sort"

	"github.com/nfnt/resize"
)

// Resize scales an image to fit within maxWidth, preserving aspect ratio.
func Resize(img image.Image, maxWidth uint) image.Image {
	bounds := img.Bounds()
	if uint(bounds.Dx()) <= maxWidth {
		return img
	}
	return resize.Resize(maxWidth, 0, img, resize.Lanczos3)
}

// ToRGBA converts any image to RGBA.
func ToRGBA(img image.Image) *image.RGBA {
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}
	bounds := img.Bounds()
	out := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			out.Set(x, y, img.At(x, y))
		}
	}
	return out
}

// ExtractPalette builds a palette from the image using k-means clustering.
func ExtractPalette(img image.Image, numColors int) color.Palette {
	bounds := img.Bounds()

	// Sample pixels (every 3rd pixel for speed)
	var samples [][3]float64
	for y := bounds.Min.Y; y < bounds.Max.Y; y += 3 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 3 {
			r, g, b, _ := img.At(x, y).RGBA()
			samples = append(samples, [3]float64{float64(r >> 8), float64(g >> 8), float64(b >> 8)})
		}
	}

	if len(samples) == 0 {
		return color.Palette{color.Black}
	}

	// k-means++ seeding
	rng := rand.New(rand.NewSource(42))
	centroids := make([][3]float64, 0, numColors)
	centroids = append(centroids, samples[rng.Intn(len(samples))])

	for len(centroids) < numColors {
		dists := make([]float64, len(samples))
		totalDist := 0.0
		for i, s := range samples {
			minD := math.MaxFloat64
			for _, c := range centroids {
				d := colorDistSq(s, c)
				if d < minD {
					minD = d
				}
			}
			dists[i] = minD
			totalDist += minD
		}
		if totalDist == 0 {
			break
		}
		target := rng.Float64() * totalDist
		cumulative := 0.0
		for i, d := range dists {
			cumulative += d
			if cumulative >= target {
				centroids = append(centroids, samples[i])
				break
			}
		}
	}

	// Iterate k-means (10 iterations)
	assignments := make([]int, len(samples))
	for iter := 0; iter < 10; iter++ {
		for i, s := range samples {
			bestIdx := 0
			bestDist := math.MaxFloat64
			for j, c := range centroids {
				d := colorDistSq(s, c)
				if d < bestDist {
					bestDist = d
					bestIdx = j
				}
			}
			assignments[i] = bestIdx
		}

		sums := make([][3]float64, len(centroids))
		counts := make([]int, len(centroids))
		for i, s := range samples {
			ci := assignments[i]
			sums[ci][0] += s[0]
			sums[ci][1] += s[1]
			sums[ci][2] += s[2]
			counts[ci]++
		}
		for j := range centroids {
			if counts[j] > 0 {
				centroids[j][0] = sums[j][0] / float64(counts[j])
				centroids[j][1] = sums[j][1] / float64(counts[j])
				centroids[j][2] = sums[j][2] / float64(counts[j])
			}
		}
	}

	pal := make(color.Palette, 0, len(centroids))
	for _, c := range centroids {
		pal = append(pal, color.RGBA{
			R: clampByte(c[0]),
			G: clampByte(c[1]),
			B: clampByte(c[2]),
			A: 0xff,
		})
	}

	// Sort palette by luminance for consistent output
	sort.Slice(pal, func(i, j int) bool {
		ri, gi, bi, _ := pal[i].RGBA()
		rj, gj, bj, _ := pal[j].RGBA()
		li := 0.299*float64(ri) + 0.587*float64(gi) + 0.114*float64(bi)
		lj := 0.299*float64(rj) + 0.587*float64(gj) + 0.114*float64(bj)
		return li < lj
	})

	return pal
}

func colorDistSq(a, b [3]float64) float64 {
	dr := a[0] - b[0]
	dg := a[1] - b[1]
	db := a[2] - b[2]
	return dr*dr + dg*dg + db*db
}

func clampByte(v float64) uint8 {
	v = math.Round(v)
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
