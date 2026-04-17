// zim-compact reads a ZIM file, resizes images, and extracts content
// ready for zimwriterfs repacking. Redirects are written to a TSV file
// instead of HTML stubs, and MIME types are preserved from the source.
//
// Usage:
//
//	zim-compact [flags] source.zim output-dir
//
// Flags:
//
//	--width      int     Max image width in pixels (default 200)
//	--quality    int     JPEG output quality 1-100 (default 40)
//	--redirects  string  Path to write redirects TSV (default output-dir/../redirects.tsv)
//	--workers    int     Parallel image workers (default NumCPU)
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"sync/atomic"

	_ "golang.org/x/image/webp"

	"github.com/pkronstrom/svalbard/build-tools/pkg/imaging"
	"github.com/stazelabs/gozim/zim"
)

type contentEntry struct {
	path string
	mime string
	data []byte
}

type redirectEntry struct {
	path       string
	title      string
	targetPath string
}

var imageTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/gif":  true,
	"image/webp": true,
}

// Matches redirect target from meta-refresh: ./Target or ../Target, with optional fragment
var metaRefreshRe = regexp.MustCompile(`content=["']0;URL='?\.\.?/([^'"#]+)`)

// isHTMLRedirect detects HTML content entries that contain meta-refresh redirects.
// Returns (target, true) if it's a redirect, ("", true) if it's a redirect we can't parse,
// ("", false) if it's not a redirect.
func isHTMLRedirect(data []byte) (string, bool) {
	if len(data) > 1000 || !bytes.Contains(data, []byte(`http-equiv="refresh"`)) {
		return "", false
	}
	m := metaRefreshRe.FindSubmatch(data)
	if m != nil {
		return string(m[1]), true
	}
	return "", true // still a redirect, just can't parse target
}

func main() {
	width := flag.Uint("width", 200, "Max image width in pixels")
	quality := flag.Int("quality", 40, "JPEG output quality (1-100)")
	redirectsPath := flag.String("redirects", "", "Path to write redirects TSV")
	workers := flag.Int("workers", runtime.NumCPU(), "Parallel image workers")
	flag.Parse()

	if flag.NArg() != 2 {
		fmt.Fprintf(os.Stderr, "Usage: zim-compact [flags] source.zim output-dir\n")
		os.Exit(1)
	}

	srcPath := flag.Arg(0)
	outDir := flag.Arg(1)

	if *redirectsPath == "" {
		*redirectsPath = filepath.Join(filepath.Dir(outDir), "redirects.tsv")
	}

	if err := run(srcPath, outDir, *redirectsPath, *width, *quality, *workers); err != nil {
		log.Fatal(err)
	}
}

func run(srcPath, outDir, redirectsPath string, maxWidth uint, jpegQuality, numWorkers int) error {
	a, err := zim.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open ZIM: %w", err)
	}
	defer a.Close()

	// Print metadata for the caller
	if a.HasMainEntry() {
		main, err := a.MainEntry()
		if err == nil {
			// Resolve redirects to get the actual content path
			if main.IsRedirect() {
				if resolved, err := main.Resolve(); err == nil {
					main = resolved
				}
			}
			fmt.Printf("main_page=%s\n", main.Path())
		}
	}
	if lang, err := a.Metadata("Language"); err == nil {
		fmt.Printf("language=%s\n", lang)
	}
	if title, err := a.Metadata("Title"); err == nil {
		fmt.Printf("title=%s\n", title)
	}
	if desc, err := a.Metadata("Description"); err == nil {
		fmt.Printf("description=%s\n", desc)
	}
	if creator, err := a.Metadata("Creator"); err == nil {
		fmt.Printf("creator=%s\n", creator)
	}

	// Write illustration
	if icon, err := a.Illustration(48); err == nil && len(icon) > 0 {
		if err := writeIllustration(outDir, icon); err != nil {
			return fmt.Errorf("write illustration: %w", err)
		}
		log.Printf("wrote illustration (%d bytes)", len(icon))
	}

	// Collect entries: separate content from redirects
	var contents []contentEntry
	var redirects []redirectEntry

	// Newer ZIM files use namespace 'C'; older ones use 'A' for articles.
	ns := byte('C')
	if a.EntryCountByNamespace('C') == 0 {
		ns = 'A'
	}

	log.Printf("reading entries from %s (%d total, namespace %c)", srcPath, a.EntryCount(), ns)

	for e := range a.EntriesByNamespace(ns) {
		if e.IsRedirect() {
			target, err := e.Resolve()
			if err != nil {
				continue
			}
			redirects = append(redirects, redirectEntry{
				path:       e.Path(),
				title:      e.Title(),
				targetPath: target.Path(),
			})
			continue
		}

		data, err := e.ReadContentCopy()
		if err != nil {
			log.Printf("skip %s: %v", e.Path(), err)
			continue
		}

		// Detect HTML meta-refresh redirects embedded in content entries
		if target, isRedir := isHTMLRedirect(data); isRedir {
			if target != "" {
				redirects = append(redirects, redirectEntry{
					path:       e.Path(),
					title:      e.Title(),
					targetPath: target,
				})
			}
			// Either way, don't write redirect HTML files to disk
			continue
		}

		contents = append(contents, contentEntry{
			path: e.Path(),
			mime: e.MIMEType(),
			data: data,
		})
	}

	log.Printf("found %d content entries, %d redirects", len(contents), len(redirects))

	// Write redirects TSV
	if err := writeRedirects(redirectsPath, redirects); err != nil {
		return fmt.Errorf("write redirects TSV: %w", err)
	}
	log.Printf("wrote %d redirects to %s", len(redirects), redirectsPath)

	// Write content entries, resizing images concurrently
	var written, resized, skipped atomic.Int64
	sem := make(chan struct{}, numWorkers)
	var wg sync.WaitGroup

	for _, entry := range contents {
		wg.Add(1)
		go func(e contentEntry) {
			defer wg.Done()

			outPath := outputPathForEntry(filepath.Join(outDir, e.path), e.mime, false)
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				skipped.Add(1)
				return
			}

			data := e.data

			// Resize images
			if imageTypes[e.mime] && len(data) > 0 {
				sem <- struct{}{}
				if processed, err := resizeImage(data, maxWidth, jpegQuality); err == nil {
					data = processed
					outPath = outputPathForEntry(outPath, e.mime, true)
					resized.Add(1)
				}
				<-sem
			}

			if err := os.WriteFile(outPath, data, 0o644); err != nil {
				skipped.Add(1)
				return
			}
			written.Add(1)

			done := written.Load() + skipped.Load()
			if done%5000 == 0 {
				log.Printf("  %d/%d entries written (%d images resized)", done, len(contents), resized.Load())
			}
		}(entry)
	}
	wg.Wait()

	log.Printf("done: %d written, %d images resized, %d skipped",
		written.Load(), resized.Load(), skipped.Load())
	return nil
}

func outputPathForEntry(outPath, mime string, resized bool) string {
	_ = mime
	_ = resized
	return outPath
}

func writeIllustration(outDir string, icon []byte) error {
	illPath := filepath.Join(outDir, "illustration.png")
	if err := os.MkdirAll(filepath.Dir(illPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(illPath, icon, 0o644); err != nil {
		return err
	}
	return nil
}

func writeRedirects(redirectsPath string, redirects []redirectEntry) error {
	if err := os.MkdirAll(filepath.Dir(redirectsPath), 0o755); err != nil {
		return err
	}
	rFile, err := os.Create(redirectsPath)
	if err != nil {
		return err
	}
	for _, r := range redirects {
		if _, err := fmt.Fprintf(rFile, "%s\t%s\t%s\n", r.path, r.title, r.targetPath); err != nil {
			rFile.Close()
			return err
		}
	}
	if err := rFile.Close(); err != nil {
		return err
	}
	return nil
}

func resizeImage(data []byte, maxWidth uint, jpegQuality int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	if bounds.Dx() < 32 || bounds.Dy() < 32 {
		return nil, fmt.Errorf("too small")
	}
	if uint(bounds.Dx()) <= maxWidth {
		return data, nil // already small enough
	}

	resized := imaging.Resize(img, maxWidth)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, resized, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
