// Package frontpage serves the drive's browser launcher and owns its services.
package frontpage

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/browse"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/config"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
)

//go:embed ui/*
var assets embed.FS

type dashboard struct {
	ctx         context.Context
	root        string
	port        int
	inv         inventory
	specs       map[string]serviceSpec
	previews    map[string]bool
	archives    map[string]string // installed ZIM filename to its dashboard service key
	kiwix       *browse.Library
	mu          sync.Mutex
	jobs        map[string]*job
	workers     sync.WaitGroup
	wiki        wikiService
	terminalKey string
	searchMu    sync.Mutex
	search      *search.Session
}

type link struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

type state struct {
	Status string `json:"status"`
	Links  []link `json:"links,omitempty"`
	Text   string `json:"text,omitempty"`
}

type job struct {
	state
	cancel context.CancelFunc
	ready  chan struct{}
	done   chan struct{}
}

func Run(ctx context.Context, out io.Writer, root string, cfg config.RuntimeConfig, open func(string) error) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer listener.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	inv, specs := buildInventory(root, cfg)
	d := &dashboard{
		ctx: ctx, root: root, port: listener.Addr().(*net.TCPAddr).Port,
		inv: inv, specs: specs, previews: installedPreviewFiles(inv), jobs: make(map[string]*job),
		archives: make(map[string]string), kiwix: browse.NewLibrary(root), terminalKey: rand.Text(),
	}
	for key, spec := range specs {
		if spec.name == "browse" && spec.target != "" {
			d.archives[spec.target] = key
		}
	}
	host := listener.Addr().String()
	server := &http.Server{
		Handler: d.handler(host), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		IdleTimeout: 60 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	exited := make(chan error, 1)
	go func() { exited <- server.Serve(listener) }()
	address := "http://" + host + "/#terminal-key=" + d.terminalKey
	fmt.Fprintf(out, "Svalbard frontpage: %s\nPress Ctrl+C to stop the dashboard and its services.\n", address)
	if open != nil {
		_ = open(address)
	}
	select {
	case err = <-exited:
	case <-ctx.Done():
	}
	cancel()
	shutdown, stop := context.WithTimeout(context.Background(), 3*time.Second)
	defer stop()
	if shutdownErr := server.Shutdown(shutdown); shutdownErr != nil {
		_ = server.Close()
	}
	d.kiwix.Close()
	d.mu.Lock()
	d.mu.Unlock() // Synchronize with any launch already entering its critical section.
	d.workers.Wait()
	d.searchMu.Lock()
	defer d.searchMu.Unlock()
	if d.search != nil {
		_ = d.search.Close()
	}
	return err
}

func (d *dashboard) handler(host string) http.Handler {
	mux := http.NewServeMux()
	ui, _ := fs.Sub(assets, "ui")
	mux.Handle("GET /", http.FileServerFS(ui))
	mux.HandleFunc("GET /api/inventory", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, d.inv)
	})
	mux.HandleFunc("GET /api/workspace", func(w http.ResponseWriter, _ *http.Request) {
		pages, err := savedPages(d.root)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, pages)
	})
	mux.HandleFunc("GET /api/source", d.serveSource)
	mux.HandleFunc("GET /api/previews/{file}", d.servePreview)
	mux.HandleFunc("GET /api/services", d.states)
	mux.HandleFunc("POST /api/services/{key}", d.start)
	mux.HandleFunc("POST /api/services/{key}/stop", d.stop)
	mux.HandleFunc("POST /api/search", d.runSearch)
	mux.HandleFunc("POST /api/read", d.readResult)
	mux.HandleFunc("GET /api/terminal", d.serveTerminal)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		// xterm.js calculates dynamic font and cell styles in its DOM renderer.
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; font-src 'self'; img-src 'self'; connect-src 'self' ws://"+host+"; frame-src http://localhost:* http://127.0.0.1:*; base-uri 'none'; form-action 'none'; frame-ancestors 'none'")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if r.Host != host {
			http.Error(w, "Unexpected host", http.StatusForbidden)
			return
		}
		// Pages served by an app or in the workspace have their own origins.
		// Requiring the dashboard's origin and a request header keeps those
		// pages and external websites from starting calls through a form.
		if r.Method == http.MethodPost &&
			(r.Header.Get("Origin") != "http://"+host || r.Header.Get("X-Svalbard-Request") != "1") {
			http.Error(w, "This action requires the dashboard", http.StatusForbidden)
			return
		}
		mux.ServeHTTP(w, r)
	})
}

func (d *dashboard) states(w http.ResponseWriter, _ *http.Request) {
	d.mu.Lock()
	states := make(map[string]state, len(d.jobs))
	for key, j := range d.jobs {
		states[key] = j.state
	}
	d.mu.Unlock()
	writeJSON(w, states)
}

func (d *dashboard) start(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	spec, ok := d.specs[key]
	if !ok {
		http.NotFound(w, r)
		return
	}
	var input struct{ Model string }
	if !readJSON(w, r, &input) {
		return
	}
	if input.Model != "" {
		valid := false
		if spec.name == "open-webui" {
			for _, model := range d.inv.Models {
				if model.File == input.Model {
					valid = true
					break
				}
			}
		}
		if !valid {
			http.Error(w, "Choose a model installed on this drive", http.StatusBadRequest)
			return
		}
	}
	d.mu.Lock()
	if d.ctx.Err() != nil {
		d.mu.Unlock()
		http.Error(w, "The dashboard is stopping", http.StatusServiceUnavailable)
		return
	}
	snapshot := d.launchLocked(key, spec, input.Model).state
	d.mu.Unlock()
	writeJSON(w, snapshot)
}

func (d *dashboard) launchLocked(key string, spec serviceSpec, model string) *job {
	j := d.jobs[key]
	if j == nil || j.Status == "stopped" || j.Status == "failed" || j.Status == "done" {
		ctx, cancel := context.WithCancel(d.ctx)
		j = &job{
			state: state{Status: "starting"}, cancel: cancel,
			ready: make(chan struct{}), done: make(chan struct{}),
		}
		d.jobs[key] = j
		d.workers.Add(1)
		go d.runJob(ctx, j, spec, model)
	}
	return j
}

// signalReady is called under d.mu when a job has published a link or reached
// a terminal state. Search requests waiting for the same job can then read it.
func (j *job) signalReady() {
	select {
	case <-j.ready:
	default:
		close(j.ready)
	}
}

func (d *dashboard) runJob(ctx context.Context, j *job, spec serviceSpec, model string) {
	defer d.workers.Done()
	defer close(j.done)
	defer j.cancel()
	report := func(label, url string) {
		d.mu.Lock()
		defer d.mu.Unlock()
		if ctx.Err() == nil {
			// Copy when appending so a response can encode the previous state
			// after releasing the lock.
			links := slices.Clone(j.Links)
			if url == "" {
				j.Links = slices.DeleteFunc(links, func(link link) bool { return link.Label == label })
			} else {
				j.Links = append(links, link{Label: label, URL: url})
				j.Status = "running"
				j.signalReady()
			}
		}
	}
	message, err := d.runService(ctx, spec, model, report)
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case ctx.Err() != nil:
		j.state = state{Status: "stopped"}
	case err != nil:
		j.state = state{Status: "failed", Text: err.Error()}
	case j.Status == "running":
		j.state = state{Status: "stopped", Text: "The service exited."}
	default:
		j.state = state{Status: "done", Text: message}
	}
	j.signalReady()
}

func (d *dashboard) stop(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if _, ok := d.specs[key]; !ok {
		http.NotFound(w, r)
		return
	}
	d.mu.Lock()
	snapshot := state{Status: "stopped"}
	if j := d.jobs[key]; j != nil {
		if j.Status == "running" || j.Status == "starting" {
			j.state = state{Status: "stopping"}
			j.cancel()
		}
		snapshot = j.state
	}
	d.mu.Unlock()
	writeJSON(w, snapshot)
}

func (d *dashboard) withSearch(w http.ResponseWriter, r *http.Request, fn func(*search.Session) error) {
	if !d.inv.HasSearch {
		http.NotFound(w, r)
		return
	}
	d.searchMu.Lock()
	defer d.searchMu.Unlock()
	if r.Context().Err() != nil {
		return
	}
	var err error
	if d.search == nil {
		d.search, err = search.NewBrowserSession(r.Context(), d.root, d.port)
	}
	if err == nil {
		err = fn(d.search)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
}

func (d *dashboard) runSearch(w http.ResponseWriter, r *http.Request) {
	var input struct{ Query string }
	if !readJSON(w, r, &input) {
		return
	}
	query := strings.TrimSpace(input.Query)
	if query == "" || len(query) > 500 {
		http.Error(w, "Enter a search of up to 500 bytes", http.StatusBadRequest)
		return
	}
	type hit struct {
		Title   string `json:"title"`
		Label   string `json:"label"`
		Source  string `json:"source"`
		Path    string `json:"path"`
		Snippet string `json:"snippet"`
	}
	d.withSearch(w, r, func(s *search.Session) error {
		result, err := s.Search(r.Context(), s.Info().BestMode, query, 20)
		if err != nil {
			return err
		}
		hits := make([]hit, 0, len(result.Results))
		for _, row := range result.Results {
			hits = append(hits, hit{row.Title, search.SourceLabel(row.Filename), row.Filename, row.Path, row.Snippet})
		}
		writeJSON(w, struct {
			Hits   []hit  `json:"hits"`
			Status string `json:"status,omitempty"`
		}{hits, result.Status})
		return nil
	})
}

func (d *dashboard) readResult(w http.ResponseWriter, r *http.Request) {
	var input struct{ Source, Path string }
	if !readJSON(w, r, &input) {
		return
	}
	d.withSearch(w, r, func(s *search.Session) error {
		result := search.Result{Filename: input.Source, Path: input.Path}
		service := d.archives[search.ArchiveSource(input.Source)]
		var address string
		var err error
		if service == "" {
			address, err = s.ResultURL(r.Context(), result)
		} else {
			address, err = d.archiveResultURL(r.Context(), service, result)
		}
		if err != nil {
			return err
		}
		writeJSON(w, struct {
			link
			Service string `json:"service,omitempty"`
		}{link{Label: "Open result", URL: address}, service})
		return nil
	})
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(dst) != nil || !errors.Is(decoder.Decode(new(any)), io.EOF) {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
