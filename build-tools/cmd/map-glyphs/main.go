// Command map-glyphs renders a font into MapLibre glyph ranges: protobuf
// files of 24 px signed-distance-field glyphs, one file per 256 code points.
//
//	go run ./cmd/map-glyphs -name "EB Garamond Regular" -out DIR -ranges 0-1279,7680-8447 font.ttf
//	go run ./cmd/map-glyphs -dump file.pbf
//
// The bitmaps follow the layout MapLibre expects from fontnik-built glyphs:
// a 3 px buffer around each glyph, an 8 px distance radius and the edge at
// 0.75 of the byte range.
package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"
)

const (
	fontSize = 24
	buffer   = 3
	radius   = 8
	cutoff   = 0.25
	// The outline is rasterised at this multiple of the output grid so
	// distances are measured below the pixel size.
	supersample = 8
)

type glyph struct {
	id            rune
	bitmap        []byte
	width, height uint32
	left, top     int32
	advance       uint32
}

func main() {
	name := flag.String("name", "", "fontstack name, such as \"EB Garamond Regular\"")
	out := flag.String("out", "", "output directory; files go in OUT/NAME/START-END.pbf")
	ranges := flag.String("ranges", "0-255", "comma-separated code point spans to cover, in whole 256-point ranges")
	dump := flag.String("dump", "", "print the glyphs of a .pbf file and exit")
	flag.Parse()
	if *dump != "" {
		if err := dumpFile(*dump); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if *name == "" || *out == "" || flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: map-glyphs -name NAME -out DIR [-ranges 0-255,...] font.ttf")
		os.Exit(2)
	}
	if err := run(*name, *out, *ranges, flag.Arg(0)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(name, out, spans, fontPath string) error {
	data, err := os.ReadFile(fontPath)
	if err != nil {
		return err
	}
	f, err := sfnt.Parse(data)
	if err != nil {
		return err
	}
	starts, err := rangeStarts(spans)
	if err != nil {
		return err
	}
	dir := filepath.Join(out, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var buf sfnt.Buffer
	metrics, err := f.Metrics(&buf, fixed.I(fontSize), font.HintingNone)
	if err != nil {
		return err
	}
	ascent := metrics.Ascent.Ceil()
	total := 0
	for _, start := range starts {
		var glyphs []glyph
		for r := rune(start); r < rune(start+256); r++ {
			g, ok, err := render(f, &buf, r, ascent)
			if err != nil {
				return fmt.Errorf("U+%04X: %w", r, err)
			}
			if ok {
				glyphs = append(glyphs, g)
			}
		}
		rangeName := fmt.Sprintf("%d-%d", start, start+255)
		if err := os.WriteFile(filepath.Join(dir, rangeName+".pbf"), encode(name, rangeName, glyphs), 0o644); err != nil {
			return err
		}
		total += len(glyphs)
	}
	fmt.Printf("%s: %d glyphs in %d ranges\n", name, total, len(starts))
	return nil
}

func rangeStarts(spans string) ([]int, error) {
	seen := map[int]bool{}
	var starts []int
	for _, span := range strings.Split(spans, ",") {
		lo, hi, ok := strings.Cut(strings.TrimSpace(span), "-")
		if !ok {
			hi = lo
		}
		a, err1 := strconv.Atoi(lo)
		b, err2 := strconv.Atoi(hi)
		if err1 != nil || err2 != nil || a < 0 || b < a || b > 0xFFFF {
			return nil, fmt.Errorf("invalid range %q", span)
		}
		for s := a / 256 * 256; s <= b; s += 256 {
			if !seen[s] {
				seen[s] = true
				starts = append(starts, s)
			}
		}
	}
	return starts, nil
}

// render draws one glyph. A code point the font lacks is skipped, so the
// map falls back to drawing nothing rather than the font's .notdef box.
func render(f *sfnt.Font, buf *sfnt.Buffer, r rune, ascent int) (glyph, bool, error) {
	index, err := f.GlyphIndex(buf, r)
	if err != nil || index == 0 {
		return glyph{}, false, nil
	}
	ppem := fixed.I(fontSize)
	advance, err := f.GlyphAdvance(buf, index, ppem, font.HintingNone)
	if err != nil {
		return glyph{}, false, err
	}
	// fontnik truncates the advance and rounds the box; MapLibre's spacing
	// and baseline assume that convention.
	g := glyph{id: r, advance: uint32(max(0, advance.Floor())), top: int32(-ascent)}
	segments, err := f.LoadGlyph(buf, index, ppem, nil)
	if err != nil {
		return glyph{}, false, err
	}
	bounds := segments.Bounds()
	if len(segments) == 0 || bounds.Empty() {
		return g, true, nil // a space: an advance and no bitmap
	}
	// sfnt's y axis points down. The buffer holds any part of the outline
	// that rounding the box leaves outside it.
	x0, y0 := bounds.Min.X.Round(), bounds.Min.Y.Round()
	g.width = uint32(max(0, (bounds.Max.X - bounds.Min.X).Round()))
	g.height = uint32(max(0, (bounds.Max.Y - bounds.Min.Y).Round()))
	g.left = int32(x0)
	g.top = int32(-y0 - ascent)

	w, h := int(g.width)+2*buffer, int(g.height)+2*buffer
	sw, sh := w*supersample, h*supersample
	raster := vector.NewRasterizer(sw, sh)
	ox := float32(buffer-x0) * supersample
	oy := float32(buffer-y0) * supersample
	pt := func(p fixed.Point26_6) (float32, float32) {
		return float32(p.X)/64*supersample + ox, float32(p.Y)/64*supersample + oy
	}
	for _, s := range segments {
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			raster.MoveTo(pt(s.Args[0]))
		case sfnt.SegmentOpLineTo:
			raster.LineTo(pt(s.Args[0]))
		case sfnt.SegmentOpQuadTo:
			ax, ay := pt(s.Args[0])
			bx, by := pt(s.Args[1])
			raster.QuadTo(ax, ay, bx, by)
		case sfnt.SegmentOpCubeTo:
			ax, ay := pt(s.Args[0])
			bx, by := pt(s.Args[1])
			cx, cy := pt(s.Args[2])
			raster.CubeTo(ax, ay, bx, by, cx, cy)
		}
	}
	raster.ClosePath()
	coverage := image.NewAlpha(image.Rect(0, 0, sw, sh))
	raster.Draw(coverage, coverage.Bounds(), image.Opaque, image.Point{})
	g.bitmap = distanceField(coverage, w, h)
	return g, true, nil
}

// distanceField samples the signed distance to the outline at each output
// pixel centre, in output pixels, and maps it to a byte with the edge at 191.
func distanceField(coverage *image.Alpha, w, h int) []byte {
	sw, sh := coverage.Rect.Dx(), coverage.Rect.Dy()
	inside := make([]float64, sw*sh)
	outside := make([]float64, sw*sh)
	for i, a := range coverage.Pix {
		if a >= 128 {
			inside[i], outside[i] = math.Inf(1), 0
		} else {
			inside[i], outside[i] = 0, math.Inf(1)
		}
	}
	// outside[i] becomes the squared distance from an outside pixel to the
	// glyph; inside[i] the squared distance from an inside pixel to the edge.
	edt(outside, sw, sh)
	edt(inside, sw, sh)
	bitmap := make([]byte, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*supersample+supersample/2)*sw + x*supersample + supersample/2
			d := (math.Sqrt(outside[i]) - math.Sqrt(inside[i])) / supersample
			v := 255 - 255*(d/radius+cutoff)
			bitmap[y*w+x] = byte(math.Round(math.Max(0, math.Min(255, v))))
		}
	}
	return bitmap
}

// edt replaces each value with the squared Euclidean distance to the nearest
// zero, by Felzenszwalb and Huttenlocher's separable transform.
func edt(grid []float64, w, h int) {
	n := max(w, h)
	f := make([]float64, n)
	d := make([]float64, n)
	v := make([]int, n)
	z := make([]float64, n+1)
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			f[y] = grid[y*w+x]
		}
		edt1(f[:h], d[:h], v, z)
		for y := 0; y < h; y++ {
			grid[y*w+x] = d[y]
		}
	}
	for y := 0; y < h; y++ {
		copy(f[:w], grid[y*w:(y+1)*w])
		edt1(f[:w], d[:w], v, z)
		copy(grid[y*w:(y+1)*w], d[:w])
	}
}

func edt1(f, d []float64, v []int, z []float64) {
	n := len(f)
	k := 0
	v[0] = 0
	z[0], z[1] = math.Inf(-1), math.Inf(1)
	for q := 1; q < n; q++ {
		if math.IsInf(f[q], 1) {
			continue
		}
		for {
			r := v[k]
			if math.IsInf(f[r], 1) {
				v[k] = q
				z[k], z[k+1] = math.Inf(-1), math.Inf(1)
				break
			}
			s := ((f[q] + float64(q*q)) - (f[r] + float64(r*r))) / float64(2*q-2*r)
			if s <= z[k] {
				if k == 0 {
					v[0] = q
					z[0], z[1] = math.Inf(-1), math.Inf(1)
					break
				}
				k--
				continue
			}
			k++
			v[k] = q
			z[k], z[k+1] = s, math.Inf(1)
			break
		}
	}
	k = 0
	for q := 0; q < n; q++ {
		for z[k+1] < float64(q) {
			k++
		}
		r := v[k]
		dq := float64(q - r)
		d[q] = dq*dq + f[r]
	}
}

// Protocol buffer encoding of MapLibre's glyphs.proto: glyphs { stacks = 1 },
// fontstack { name = 1, range = 2, glyphs = 3 }, glyph { id = 1, bitmap = 2,
// width = 3, height = 4, left = 5 (sint32), top = 6 (sint32), advance = 7 }.
func encode(name, rangeName string, glyphs []glyph) []byte {
	var stack []byte
	stack = appendBytes(stack, 1, []byte(name))
	stack = appendBytes(stack, 2, []byte(rangeName))
	for _, g := range glyphs {
		var m []byte
		m = appendVarint(m, 1, uint64(g.id))
		if len(g.bitmap) > 0 {
			m = appendBytes(m, 2, g.bitmap)
		}
		m = appendVarint(m, 3, uint64(g.width))
		m = appendVarint(m, 4, uint64(g.height))
		m = appendVarint(m, 5, zigzag(g.left))
		m = appendVarint(m, 6, zigzag(g.top))
		m = appendVarint(m, 7, uint64(g.advance))
		stack = appendBytes(stack, 3, m)
	}
	return appendBytes(nil, 1, stack)
}

func zigzag(v int32) uint64 { return uint64(uint32((v << 1) ^ (v >> 31))) }

func appendUvarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func appendVarint(b []byte, field int, v uint64) []byte {
	return appendUvarint(appendUvarint(b, uint64(field)<<3), v)
}

func appendBytes(b []byte, field int, v []byte) []byte {
	b = appendUvarint(appendUvarint(b, uint64(field)<<3|2), uint64(len(v)))
	return append(b, v...)
}

// dumpFile prints one line per glyph: code point, size, bearings, advance and
// the mean bitmap value, for comparison against a reference file.
func dumpFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return walk(data, func(field int, value uint64, bytes []byte) error {
		if field != 1 {
			return nil
		}
		return walk(bytes, func(field int, value uint64, bytes []byte) error {
			switch field {
			case 1, 2:
				fmt.Printf("# %s\n", bytes)
			case 3:
				var g glyph
				if err := walk(bytes, func(field int, value uint64, b []byte) error {
					switch field {
					case 1:
						g.id = rune(value)
					case 2:
						g.bitmap = b
					case 3:
						g.width = uint32(value)
					case 4:
						g.height = uint32(value)
					case 5:
						g.left = unzigzag(value)
					case 6:
						g.top = unzigzag(value)
					case 7:
						g.advance = uint32(value)
					}
					return nil
				}); err != nil {
					return err
				}
				sum := 0
				for _, v := range g.bitmap {
					sum += int(v)
				}
				mean := 0
				if len(g.bitmap) > 0 {
					mean = sum / len(g.bitmap)
				}
				fmt.Printf("U+%04X %q w=%d h=%d left=%d top=%d adv=%d bytes=%d mean=%d\n", g.id, g.id, g.width, g.height, g.left, g.top, g.advance, len(g.bitmap), mean)
			}
			return nil
		})
	})
}

func unzigzag(v uint64) int32 { return int32(v>>1) ^ -int32(v&1) }

func uvarint(b []byte) (uint64, int) {
	var v uint64
	for i, c := range b {
		if i == 10 {
			return 0, -1
		}
		v |= uint64(c&0x7F) << (7 * i)
		if c < 0x80 {
			return v, i + 1
		}
	}
	return 0, -1
}

func walk(b []byte, visit func(field int, value uint64, bytes []byte) error) error {
	for len(b) > 0 {
		key, n := uvarint(b)
		if n < 0 {
			return errors.New("malformed protobuf")
		}
		b = b[n:]
		field, wire := int(key>>3), key&7
		switch wire {
		case 0:
			v, n := uvarint(b)
			if n < 0 {
				return errors.New("malformed varint")
			}
			b = b[n:]
			if err := visit(field, v, nil); err != nil {
				return err
			}
		case 2:
			l, n := uvarint(b)
			if n < 0 || uint64(len(b)-n) < l {
				return errors.New("malformed length")
			}
			if err := visit(field, 0, b[n:n+int(l)]); err != nil {
				return err
			}
			b = b[n+int(l):]
		default:
			return fmt.Errorf("unsupported wire type %d", wire)
		}
	}
	return nil
}
