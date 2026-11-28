// Package pdfreader serves the offline PDF viewer on the document server's origin.
package pdfreader

import (
	"archive/zip"
	"bytes"
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"sync"
)

// The vendor archive contains the unmodified pdfjs-dist 5.6.205 display and
// viewer modules, worker, image decoders, fonts, character maps and licenses.
// These files are inflated on request and do not load with the dashboard.
//
//go:embed ui/* pdfjs-5.6.205.zip
var assets embed.FS

const Prefix = "/.svalbard-reader/"

var handler = sync.OnceValue(newHandler)

func newHandler() http.Handler {
	ui, err := fs.Sub(assets, "ui")
	if err != nil {
		panic(err)
	}
	data, err := assets.ReadFile("pdfjs-5.6.205.zip")
	if err != nil {
		panic(err)
	}
	vendor, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		panic(err)
	}
	mux := http.NewServeMux()
	mux.Handle(Prefix+"vendor/", http.StripPrefix(Prefix+"vendor/", http.FileServerFS(vendor)))
	mux.Handle(Prefix, http.StripPrefix(Prefix, http.FileServerFS(ui)))
	return mux
}

func Serve(w http.ResponseWriter, r *http.Request, ancestor string) {
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self' 'wasm-unsafe-eval'; worker-src 'self'; connect-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' blob: data:; frame-ancestors "+ancestor+"; base-uri 'none'; form-action 'none'")
	if strings.HasSuffix(r.URL.Path, ".mjs") {
		w.Header().Set("Content-Type", "text/javascript")
	}
	handler().ServeHTTP(w, r)
}
