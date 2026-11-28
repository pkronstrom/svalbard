// Package frontpage serves the drive's browser launcher and owns its services.
package frontpage

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"crypto/subtle"
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
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/workspace"
)

type dashboard struct {
	ui             fs.FS
	desktop        *desktopSession
	ctx            context.Context
	root           string
	port           int
	inv            inventory
	specs          map[string]serviceSpec
	previews       map[string]bool
	archives       map[string]string // installed ZIM filename to its dashboard service key
	kiwix          *browse.Library
	mu             sync.Mutex
	jobs           map[string]*job
	workers        sync.WaitGroup
	wiki           wikiService
	settingsMu     sync.Mutex
	lifecycleMu    sync.Mutex
	draining       bool
	prepared       bool
	terminals      int
	resourceLabels map[string][]string
	terminalKey    string
	searchMu       sync.Mutex
	search         *search.Session
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
	started time.Time
	model   string
	events  []string
	pid     int
	state
	cancel context.CancelFunc
	ready  chan struct{}
	done   chan struct{}
}

func Run(ctx context.Context, out io.Writer, root string, cfg config.RuntimeConfig, open func(string) error) error {
	return runDashboard(ctx, out, root, cfg, open, nil)
}

func runDashboard(ctx context.Context, out io.Writer, root string, cfg config.RuntimeConfig, open func(string) error, desktop *desktopSession) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	archive, err := zip.OpenReader(filepath.Join(root, ".svalbard", "web-launcher.zip"))
	if err != nil {
		return fmt.Errorf("open dashboard assets: %w. Add web-launcher with the Svalbard host and apply it to this vault", err)
	}
	defer archive.Close()
	for _, name := range []string{"index.html", "frontpage.js", "style.css", "vendor/atlas/mono.woff2"} {
		if _, err := fs.Stat(archive, name); err != nil {
			return fmt.Errorf("the dashboard package is missing %s. Update web-launcher on this vault", name)
		}
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
		ui: archive, desktop: desktop, ctx: ctx, root: root, port: listener.Addr().(*net.TCPAddr).Port,
		inv: inv, specs: specs, previews: installedPreviewFiles(inv), jobs: make(map[string]*job),
		archives: make(map[string]string), kiwix: browse.NewLibrary(root), terminalKey: rand.Text(),
	}
	d.resourceLabels = map[string][]string{}
	for _, group := range inv.Groups {
		for _, item := range group.Items {
			if item.Service != "" {
				d.resourceLabels[item.Service] = appendUnique(d.resourceLabels[item.Service], item.Label)
			}
		}
	}
	for key, spec := range specs {
		if spec.name == "browse" && spec.target != "" {
			d.archives[spec.target] = key
		}
	}
	if inv.HasSearch {
		d.specs["search-documents"] = serviceSpec{name: "documents", target: "."}
	}
	host := listener.Addr().String()
	server := &http.Server{
		Handler: d.handler(host), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		IdleTimeout: 60 * time.Second,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	exited := make(chan error, 1)
	go func() { exited <- server.Serve(listener) }()
	address := "http://" + host + "/"
	if desktop != nil {
		if err := desktop.ready(ctx, address, d.terminalKey, cancel); err != nil {
			_ = server.Close()
			return err
		}
		defer desktop.close()
	}
	if desktop == nil || desktop.foreground {
		address += "#terminal-key=" + d.terminalKey
		fmt.Fprintf(out, "Svalbard frontpage: %s\nPress Ctrl+C to stop the dashboard and its services.\n", address)
		if open != nil {
			_ = open(address)
		}
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
	mux.Handle("GET /", http.FileServerFS(d.ui))
	if d.desktop != nil {
		mux.HandleFunc("POST /browser-launch", d.desktop.redeem)
	}
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
	mux.HandleFunc("GET /api/preferences", d.preferences)
	mux.HandleFunc("POST /api/preferences", d.preferences)
	mux.HandleFunc("GET /api/ebook", d.ebook)
	mux.HandleFunc("GET /api/dataset", d.dataset)
	mux.HandleFunc("GET /api/download", d.download)
	mux.HandleFunc("GET /api/source", d.serveSource)
	mux.HandleFunc("GET /api/previews/{file}", d.servePreview)
	mux.HandleFunc("GET /api/services", d.states)
	mux.HandleFunc("GET /api/registry", d.registry)
	mux.HandleFunc("POST /api/lifecycle", d.lifecycle)
	mux.HandleFunc("POST /api/drive-users", d.inspectDriveUsers)
	mux.HandleFunc("POST /api/services/{key}", d.start)
	mux.HandleFunc("POST /api/services/{key}/stop", d.stop)
	mux.HandleFunc("GET /api/archives/{key}/{part}", func(w http.ResponseWriter, r *http.Request) {
		if part := r.PathValue("part"); part != "article" && part != "image" {
			http.NotFound(w, r)
			return
		}
		d.archiveContent(w, r)
	})
	mux.HandleFunc("POST /api/archives/{key}/search", d.searchArchive)
	mux.HandleFunc("POST /api/search", d.runSearch)
	mux.HandleFunc("POST /api/read", d.readResult)
	mux.HandleFunc("POST /api/unlock", func(w http.ResponseWriter, r *http.Request) {
		if d.terminalKey == "" || subtle.ConstantTimeCompare([]byte(d.terminalKey), []byte(r.Header.Get("X-Svalbard-Key"))) != 1 {
			http.Error(w, "Invalid launch link", http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /api/terminal", d.serveTerminal)
	archiveHost := fmt.Sprintf("localhost:%d", d.port)
	archiveFrames := d.archiveFrameHandler("http://" + host)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Host == archiveHost {
			archiveFrames.ServeHTTP(w, r)
			return
		}
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
		if r.Method == http.MethodPost && !(d.desktop != nil && r.URL.Path == "/browser-launch") &&
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
	if d.ctx.Err() != nil || d.draining {
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
			state: state{Status: "starting"}, cancel: cancel, started: time.Now().UTC(), model: model,
			ready: make(chan struct{}), done: make(chan struct{}),
		}
		j.events = []string{j.started.Format(time.RFC3339Nano) + " · Start requested"}
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
				if j.Status != "running" {
					j.events = append(j.events, time.Now().UTC().Format(time.RFC3339Nano)+" · Ready")
				}
				j.Status = "running"
				j.signalReady()
			}
		}
	}
	message, err := d.runService(ctx, spec, model, report, func(pid int) { d.mu.Lock(); j.pid = pid; d.mu.Unlock() })
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
	j.events = append(j.events, time.Now().UTC().Format(time.RFC3339Nano)+" · "+j.Status)
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
	d.mu.Lock()
	draining := d.draining
	d.mu.Unlock()
	if draining {
		http.Error(w, "Drive is preparing for removal", 409)
		return
	}
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
	result := search.Result{Filename: input.Source, Path: input.Path}
	respond := func(address, service string) {
		writeJSON(w, struct {
			link
			Service string `json:"service,omitempty"`
		}{link{Label: "Open result", URL: address}, service})
	}
	if key := d.archives[search.ArchiveSource(input.Source)]; key != "" {
		address, err := d.archiveResultURL(r.Context(), key, result)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		respond(address, key)
		return
	}
	if rel, ok := strings.CutPrefix(input.Source, "books/"); ok && fs.ValidPath(rel) && engine.IsDocumentFile(rel) {
		key := "search-documents"
		for id, spec := range d.specs {
			if spec.name == "documents" && spec.target == rel {
				key = id
				break
			}
		}
		if _, ok := d.specs[key]; !ok {
			http.NotFound(w, r)
			return
		}
		address, err := d.readyURL(r.Context(), key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		link := address.String()
		if key == "search-documents" {
			link = workspace.URL(strings.TrimRight(link, "/"), rel)
		}
		if page, ok := engine.PageNumber(input.Path); ok && strings.HasSuffix(strings.ToLower(rel), ".pdf") {
			link += fmt.Sprintf("#page=%d", page)
		}
		respond(link, key)
		return
	}
	d.withSearch(w, r, func(s *search.Session) error {
		address, err := s.ResultURL(r.Context(), result)
		if err == nil {
			respond(address, "")
		}
		return err
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
