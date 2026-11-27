package openwebui

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	pdfUploadLimitMiB    = 8
	pdfUploadLimit       = pdfUploadLimitMiB * 1024 * 1024
	pdfUploadPageLimit   = 32
	pdfWorkerTimeout     = 15 * time.Second
	pdfWorkerOutputLimit = 32 * 1024 * 1024
)

//go:embed pdf_upload_worker.py
var pdfUploadScript string

type pdfUploads struct {
	URL   string
	Token string
	done  <-chan struct{}
}

type pdfUploadInput struct {
	name, mimeType string
	data           []byte
	isPDF          bool
	pdfBudget      int
}

type pdfUploadDocument struct {
	PageContent string         `json:"page_content"`
	Metadata    map[string]any `json:"metadata"`
}

type pdfUploadExtractor func(context.Context, pdfUploadInput) ([]pdfUploadDocument, error)

func startPDFUploads(ctx context.Context, driveRoot, platformName string, contextSize int) (*pdfUploads, error) {
	worker, err := resolvePDFUploadWorker(driveRoot, platformName)
	if err != nil {
		return nil, err
	}
	if err := worker.check(ctx); err != nil {
		return nil, fmt.Errorf("local attachment worker unavailable: %w", err)
	}
	return servePDFUploads(ctx, contextSize, worker.extract)
}

func resolvePDFUploadWorker(driveRoot, platformName string) (pdfUploadWorker, error) {
	root, err := filepath.Abs(filepath.Join(driveRoot, "runtime", "python", platformName))
	if err != nil {
		return pdfUploadWorker{}, err
	}
	packageDir := filepath.Join(root, "tools", recipeID)
	if info, err := os.Stat(packageDir); err != nil || !info.IsDir() {
		return pdfUploadWorker{}, fmt.Errorf("Open WebUI's Python packages are unavailable for %s", platformName)
	}
	interpreters, err := os.ReadDir(filepath.Join(root, "python"))
	if err != nil {
		return pdfUploadWorker{}, fmt.Errorf("bundled Python is unavailable for %s", platformName)
	}
	// The portable recipe currently pins Python 3.12. Use the executable,
	// since bin/python3 can be a symlink or a wrapper on existing vaults.
	for _, entry := range interpreters {
		name := entry.Name()
		if !entry.IsDir() || !(strings.HasPrefix(name, "cpython-3.12.") || strings.HasPrefix(name, "cpython-3.12-")) {
			continue
		}
		python := filepath.Join(root, "python", name, "bin", "python3.12")
		if info, err := os.Stat(python); err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return pdfUploadWorker{python: python, packageDir: packageDir}, nil
		}
	}
	return pdfUploadWorker{}, fmt.Errorf("bundled Python 3.12 is unavailable for %s", platformName)
}

func servePDFUploads(ctx context.Context, contextSize int, extract pdfUploadExtractor) (*pdfUploads, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	token := hex.EncodeToString(secret[:])
	handler := newPDFUploadHandler(token, contextSize, extract)
	server := &http.Server{
		Handler:           handler,
		BaseContext:       func(net.Listener) context.Context { return ctx },
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      40 * time.Second,
		IdleTimeout:       10 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}
	served := make(chan struct{})
	go func() {
		defer close(served)
		_ = server.Serve(listener)
	}()
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case <-ctx.Done():
		case <-served:
		}
		_ = server.Close()
		<-served
		// Wait for the current handler to finish reaping its worker and
		// removing its files. Closing the server interrupts an in-flight read.
		handler.slot <- struct{}{}
	}()
	return &pdfUploads{URL: "http://" + listener.Addr().String(), Token: token, done: done}, nil
}

type pdfUploadHandler struct {
	authorization string
	pdfBudget     int
	slot          chan struct{}
	extract       pdfUploadExtractor
}

func newPDFUploadHandler(token string, contextSize int, extract pdfUploadExtractor) *pdfUploadHandler {
	return &pdfUploadHandler{
		authorization: "Bearer " + token,
		// Use UTF-8 bytes as a conservative token ceiling, including page
		// labels. Leave most of the smallest model window available for chat.
		pdfBudget: min(contextSize/4, 16*1024),
		slot:      make(chan struct{}, 1),
		extract:   extract,
	}
}

func (h *pdfUploadHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/process" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", http.MethodPut)
		http.Error(w, "Use PUT to process an attachment.", http.StatusMethodNotAllowed)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(h.authorization)) != 1 {
		http.Error(w, "Attachment loader authentication failed.", http.StatusUnauthorized)
		return
	}
	if r.ContentLength > pdfUploadLimit {
		http.Error(w, "Attach a file up to 8 MiB or add it to a document collection.", http.StatusRequestEntityTooLarge)
		return
	}
	select {
	case h.slot <- struct{}{}:
		defer func() { <-h.slot }()
	default:
		http.Error(w, "Another attachment is being processed. Try again when it finishes.", http.StatusServiceUnavailable)
		return
	}
	if r.Context().Err() != nil {
		return
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, pdfUploadLimit))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Attach a file up to 8 MiB or add it to a document collection.", http.StatusRequestEntityTooLarge)
		} else {
			http.Error(w, "The attachment could not be received. Try uploading it again.", http.StatusBadRequest)
		}
		return
	}
	if len(data) == 0 {
		http.Error(w, "The attachment is empty. Choose a file with text.", http.StatusUnprocessableEntity)
		return
	}
	name := uploadName(r.Header.Get("X-Filename"))
	mimeType := r.Header.Get("Content-Type")
	input := pdfUploadInput{name: name, mimeType: mimeType, data: data, isPDF: isPDFUpload(name, mimeType, data), pdfBudget: h.pdfBudget}
	documents, err := h.extract(r.Context(), input)
	if r.Context().Err() != nil {
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(documents)
}

func uploadName(header string) string {
	if decoded, err := url.PathUnescape(header); err == nil {
		header = decoded
	}
	name := path.Base(strings.ReplaceAll(header, `\`, "/"))
	if name == "." || name == "/" || strings.ContainsRune(name, 0) {
		return "attachment"
	}
	return strings.ToValidUTF8(name, "\uFFFD")
}

func isPDFUpload(name, mimeType string, data []byte) bool {
	mediaType, _, _ := mime.ParseMediaType(mimeType)
	return strings.EqualFold(filepath.Ext(name), ".pdf") || mediaType == "application/pdf" ||
		bytes.HasPrefix(bytes.TrimLeft(data, "\t\r\n "), []byte("%PDF-"))
}

type pdfUploadWorker struct {
	python     string
	packageDir string
}

func (p pdfUploadWorker) check(ctx context.Context) error {
	response, err := p.run(ctx, nil, "check")
	if err != nil {
		return err
	}
	if !response.Ready {
		return errors.New("portable Python did not confirm its loaders and resource limits")
	}
	return nil
}

func (p pdfUploadWorker) extract(ctx context.Context, input pdfUploadInput) ([]pdfUploadDocument, error) {
	mode := "slim"
	if input.isPDF {
		mode = "pdf"
	}
	response, err := p.run(ctx, input.data, mode, input.name, input.mimeType, strconv.Itoa(input.pdfBudget), strconv.Itoa(pdfUploadPageLimit))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, pdfUploadError("resources")
	}
	if response.Error != "" {
		return nil, pdfUploadError(response.Error)
	}
	if len(response.Documents) == 0 {
		return nil, pdfUploadError("empty")
	}
	return response.Documents, nil
}

type pdfWorkerResponse struct {
	Ready     bool                `json:"ready"`
	Documents []pdfUploadDocument `json:"documents"`
	Error     string              `json:"error"`
}

func (p pdfUploadWorker) run(ctx context.Context, data []byte, args ...string) (pdfWorkerResponse, error) {
	var response pdfWorkerResponse
	ctx, cancel := context.WithTimeout(ctx, pdfWorkerTimeout)
	defer cancel()
	dir, err := os.MkdirTemp("", "svalbard-upload-")
	if err != nil {
		return response, err
	}
	defer os.RemoveAll(dir)
	// The slim loader module imports Open WebUI's configuration on startup.
	// Give it an empty frontend and private state directories to initialize.
	if err := os.MkdirAll(filepath.Join(dir, "frontend", "static"), 0o700); err != nil {
		return response, err
	}
	// The pinned slim loaders take a filename. Use a fixed private path and
	// keep upload labels out of all filesystem operations.
	if data != nil {
		if err := os.WriteFile(filepath.Join(dir, "upload"), data, 0o600); err != nil {
			return response, err
		}
	}
	cmd := exec.CommandContext(ctx, p.python, append([]string{"-B", "-P", "-c", pdfUploadScript}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(hostEnvironment(),
		"HOME="+dir,
		"DATA_DIR="+filepath.Join(dir, "data"),
		"STATIC_DIR="+filepath.Join(dir, "static"),
		"FRONTEND_BUILD_DIR="+filepath.Join(dir, "frontend"),
		"TMPDIR="+dir,
		"PYTHON_DOTENV_DISABLED=1",
		"PYTHONNOUSERSITE=1",
		"USE_SLIM_DOCKER=true",
		"OFFLINE_MODE=true",
		"OPENBLAS_NUM_THREADS=1",
		"ENABLE_DB_MIGRATIONS=false",
		"WEBUI_AUTH=false",
		"WEBUI_SECRET_KEY=unused-attachment-worker",
		"CORS_ALLOW_ORIGIN=http://127.0.0.1",
	)
	if p.packageDir != "" {
		cmd.Env = append(cmd.Env, "PYTHONPATH="+p.packageDir)
	}
	cmd.WaitDelay = time.Second
	output := &pdfWorkerOutput{}
	cmd.Stdout = output
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return response, ctx.Err()
		}
		return response, err
	}
	err = json.Unmarshal(output.buffer.Bytes(), &response)
	return response, err
}

// Limit the response before retaining or decoding it in the drive process.
type pdfWorkerOutput struct{ buffer bytes.Buffer }

func (o *pdfWorkerOutput) Write(data []byte) (int, error) {
	if len(data) > pdfWorkerOutputLimit-o.buffer.Len() {
		return 0, errors.New("attachment worker response too large")
	}
	return o.buffer.Write(data)
}

func pdfUploadError(code string) error {
	switch code {
	case "encrypted":
		return errors.New("This PDF is encrypted. Save an unlocked copy and upload it again.")
	case "scanned":
		return errors.New("This PDF has no selectable text. Use OCR first or upload a text version.")
	case "pages":
		return fmt.Errorf("Attach a PDF up to %d pages or add it to a document collection", pdfUploadPageLimit)
	case "context":
		return errors.New("This PDF contains too much text for the chat context. Attach a few relevant pages or add it to a document collection.")
	case "pdf_read":
		return errors.New("Could not read this PDF. Open it and save a fresh copy before uploading it again.")
	case "unsupported":
		return errors.New("This attachment format is unsupported. Export it as text, HTML, CSV, or a PDF with selectable text.")
	case "empty":
		return errors.New("The attachment has no text. Choose a file with text.")
	case "resources":
		return errors.New("This attachment exceeded local extraction resource limits. Attach a smaller extract or add the PDF to a document collection.")
	default:
		return errors.New("Could not read this attachment. Save it again and retry.")
	}
}
