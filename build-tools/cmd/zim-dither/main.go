// zim-dither processes images for ZIM recompression.
//
// Modes:
//
//	zim-dither image [flags] input output     — dither a single image
//	zim-dither batch [flags] dir              — dither all images in a directory (in-place)
//
// The ZIM reading/writing is handled by the Python builder via libzim.
// This tool only handles the image processing (resize + dither).
package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	_ "golang.org/x/image/webp"

	"github.com/pkronstrom/svalbard/build-tools/pkg/imaging"
)

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "image":
		cmdImage(os.Args[2:])
	case "batch":
		cmdBatch(os.Args[2:])
	default:
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `Usage:
  zim-dither image [flags] input output   Dither a single image
  zim-dither batch [flags] dir            Dither all images in a directory

Flags:
  --width   int     Max width in pixels (default 400)
  --colors  int     Palette size (default 8)
  --dither  string  Algorithm: bayer (default "bayer")
`)
}

func cmdImage(args []string) {
	fs := flag.NewFlagSet("image", flag.ExitOnError)
	width := fs.Uint("width", 400, "Max image width")
	colors := fs.Int("colors", 8, "Palette size")
	dither := fs.String("dither", "bayer", "Algorithm")
	quality := fs.Int("quality", 60, "JPEG output quality (1-100)")
	fs.Parse(args)

	if fs.NArg() != 2 {
		fmt.Fprintf(os.Stderr, "Usage: zim-dither image [flags] input output\n")
		os.Exit(1)
	}

	if err := processFile(fs.Arg(0), fs.Arg(1), *width, *colors, *dither, *quality); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func cmdBatch(args []string) {
	fs := flag.NewFlagSet("batch", flag.ExitOnError)
	width := fs.Uint("width", 400, "Max image width")
	colors := fs.Int("colors", 8, "Palette size")
	dither := fs.String("dither", "bayer", "Algorithm")
	quality := fs.Int("quality", 60, "JPEG output quality (1-100)")
	workers := fs.Int("workers", runtime.NumCPU(), "Parallel workers")
	fs.Parse(args)

	if fs.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Usage: zim-dither batch [flags] dir\n")
		os.Exit(1)
	}
	dir := fs.Arg(0)

	var paths []string
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if imageExts[strings.ToLower(filepath.Ext(path))] {
			paths = append(paths, path)
		}
		return nil
	})

	log.Printf("found %d images in %s", len(paths), dir)

	var processed, skipped atomic.Int64
	sem := make(chan struct{}, *workers)
	var wg sync.WaitGroup

	for _, p := range paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			outPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".jpg"
			if err := processFile(path, outPath, *width, *colors, *dither, *quality); err != nil {
				skipped.Add(1)
				return
			}
			// Remove original if format changed
			if outPath != path {
				os.Remove(path)
			}
			processed.Add(1)

			done := processed.Load() + skipped.Load()
			if done%200 == 0 {
				log.Printf("  %d/%d images", done, len(paths))
			}
		}(p)
	}
	wg.Wait()

	log.Printf("done: %d dithered, %d skipped", processed.Load(), skipped.Load())
}

func processFile(input, output string, maxWidth uint, numColors int, algorithm string, jpegQuality int) error {
	f, err := os.Open(input)
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("decode %s: %w", input, err)
	}
	f.Close()

	// Skip tiny images
	bounds := img.Bounds()
	if bounds.Dx() < 32 || bounds.Dy() < 32 {
		return fmt.Errorf("too small (%dx%d)", bounds.Dx(), bounds.Dy())
	}

	result := imaging.Process(img, maxWidth, numColors, algorithm)

	out, err := os.Create(output)
	if err != nil {
		return err
	}
	defer out.Close()

	ext := strings.ToLower(filepath.Ext(output))
	if ext == ".png" {
		return png.Encode(out, result)
	}
	return jpeg.Encode(out, result, &jpeg.Options{Quality: jpegQuality})
}
