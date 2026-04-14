// zim-dither reprocesses a ZIM file by resizing and dithering all images.
//
// Pipeline: zimdump → walk images → resize + dither → rewrite → zimwriterfs
//
// Requires zimdump and zimwriterfs on PATH (available in svalbard-tools Docker).
package main

import (
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/pkronstrom/svalbard/build-tools/pkg/imaging"
)

var imageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
}

func main() {
	width := flag.Uint("width", 400, "Max image width in pixels")
	colors := flag.Int("colors", 8, "Palette size for dithering")
	dither := flag.String("dither", "bayer", "Dithering algorithm (bayer)")
	workers := flag.Int("workers", runtime.NumCPU(), "Parallel image workers")
	verbose := flag.Bool("verbose", false, "Print progress")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: zim-dither [flags] input.zim output.zim\n\n")
		fmt.Fprintf(os.Stderr, "Reprocess a ZIM file by resizing and dithering all images.\n\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	args := flag.Args()
	if len(args) != 2 {
		flag.Usage()
		os.Exit(1)
	}
	inputZim := args[0]
	outputZim := args[1]

	// Create work directory
	workDir, err := os.MkdirTemp("", "zim-dither-*")
	if err != nil {
		log.Fatalf("creating temp dir: %v", err)
	}
	defer os.RemoveAll(workDir)

	extractDir := filepath.Join(workDir, "extracted")

	// Step 1: Extract ZIM
	log.Printf("extracting %s ...", inputZim)
	cmd := exec.Command("zimdump", "--dir", extractDir, inputZim)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("zimdump failed: %v", err)
	}

	// Step 2: Find and process images
	var imagePaths []string
	err = filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if imageExts[ext] {
			imagePaths = append(imagePaths, path)
		}
		return nil
	})
	if err != nil {
		log.Fatalf("walking extracted dir: %v", err)
	}

	log.Printf("found %d images to process", len(imagePaths))

	var processed atomic.Int64
	var skipped atomic.Int64
	total := len(imagePaths)

	// Process images in parallel
	sem := make(chan struct{}, *workers)
	var wg sync.WaitGroup

	for _, imgPath := range imagePaths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			if err := processImage(path, *width, *colors, *dither); err != nil {
				if *verbose {
					log.Printf("  skip %s: %v", filepath.Base(path), err)
				}
				skipped.Add(1)
			} else {
				processed.Add(1)
			}

			done := processed.Load() + skipped.Load()
			if *verbose && done%100 == 0 {
				log.Printf("  %d/%d images processed", done, total)
			}
		}(imgPath)
	}
	wg.Wait()

	log.Printf("processed %d images, skipped %d", processed.Load(), skipped.Load())

	// Step 3: Repack into ZIM
	log.Printf("repacking into %s ...", outputZim)

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputZim), 0o755); err != nil {
		log.Fatalf("creating output dir: %v", err)
	}

	cmd = exec.Command("zimwriterfs",
		"--welcome", "index.html",
		"--illustration", "48x48.png",
		"--language", "eng",
		"--title", "Wikipedia (dithered)",
		"--description", "Wikipedia with dithered images",
		"--name", "wikipedia-dithered",
		extractDir,
		outputZim,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("zimwriterfs failed: %v", err)
	}

	// Report sizes
	inInfo, _ := os.Stat(inputZim)
	outInfo, _ := os.Stat(outputZim)
	if inInfo != nil && outInfo != nil {
		ratio := float64(outInfo.Size()) / float64(inInfo.Size()) * 100
		log.Printf("done: %s → %s (%.1f%% of original)",
			formatSize(inInfo.Size()), formatSize(outInfo.Size()), ratio)
	}
}

func processImage(path string, maxWidth uint, numColors int, algorithm string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	f.Close()

	// Skip tiny images (icons, spacers)
	bounds := img.Bounds()
	if bounds.Dx() < 32 || bounds.Dy() < 32 {
		return nil // leave as-is
	}

	result := imaging.Process(img, maxWidth, numColors, algorithm)

	// Write as indexed PNG (replaces original regardless of format)
	outPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".png"
	out, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	defer out.Close()

	if err := png.Encode(out, result); err != nil {
		return fmt.Errorf("encode png: %w", err)
	}

	// Remove original if we changed the extension
	if outPath != path {
		os.Remove(path)
	}

	return nil
}

func formatSize(bytes int64) string {
	const (
		mb = 1024 * 1024
		gb = 1024 * 1024 * 1024
	)
	if bytes >= gb {
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(gb))
	}
	return fmt.Sprintf("%.1f MB", float64(bytes)/float64(mb))
}
