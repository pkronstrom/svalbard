package openwebui

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPDFUploadRequest(t *testing.T) {
	var calls int
	handler := newPDFUploadHandler("secret", 8192, func(_ context.Context, input pdfUploadInput) ([]pdfUploadDocument, error) {
		calls++
		if input.name != "road map.pdf" || input.mimeType != "application/pdf" || !input.isPDF || string(input.data) != "%PDF-test" {
			t.Fatalf("unexpected worker input: %+v", input)
		}
		if input.pdfBudget != 2048 {
			t.Fatalf("unexpected PDF budget: %d", input.pdfBudget)
		}
		return []pdfUploadDocument{{PageContent: "[PDF page 2]\nRoad map", Metadata: map[string]any{"page": 2}}}, nil
	})

	for _, tc := range []struct {
		name, method, path, token string
		body                      []byte
		unknownLength             bool
		status                    int
	}{
		{"other path", "PUT", "/other", "Bearer secret", nil, false, http.StatusNotFound},
		{"other method", "POST", "/process", "Bearer secret", nil, false, http.StatusMethodNotAllowed},
		{"wrong token", "PUT", "/process", "Bearer wrong", nil, false, http.StatusUnauthorized},
		{"empty", "PUT", "/process", "Bearer secret", nil, false, http.StatusUnprocessableEntity},
		{"oversized", "PUT", "/process", "Bearer secret", make([]byte, pdfUploadLimitMiB*1024*1024+1), false, http.StatusRequestEntityTooLarge},
		{"oversized streamed", "PUT", "/process", "Bearer secret", make([]byte, pdfUploadLimitMiB*1024*1024+1), true, http.StatusRequestEntityTooLarge},
		{"uploaded", "PUT", "/process", "Bearer secret", []byte("%PDF-test"), false, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(tc.body))
			if tc.unknownLength {
				request.ContentLength = -1
			}
			request.Header.Set("Authorization", tc.token)
			request.Header.Set("Content-Type", "application/pdf")
			request.Header.Set("X-Filename", "..%2Froad%20map.pdf")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d: %s", response.Code, tc.status, response.Body)
			}
			if tc.status == http.StatusOK {
				var documents []pdfUploadDocument
				if err := json.Unmarshal(response.Body.Bytes(), &documents); err != nil || len(documents) != 1 || documents[0].PageContent != "[PDF page 2]\nRoad map" {
					t.Fatalf("unexpected external loader response: %s (%v)", response.Body, err)
				}
			}
		})
	}
	if calls != 1 {
		t.Fatalf("worker called %d times, want 1", calls)
	}
}

func TestResolvePDFUploadWorker(t *testing.T) {
	driveRoot := filepath.Join(t.TempDir(), "Vault [test]")
	platform := "macos-arm64"
	base := filepath.Join(driveRoot, "runtime", "python", platform)
	tools := filepath.Join(base, "tools", "open-webui")
	if err := os.MkdirAll(tools, 0o755); err != nil {
		t.Fatal(err)
	}
	interpreter := filepath.Join(base, "python", "cpython-3.12.14-macos-aarch64-none", "bin", "python3.12")
	for _, name := range []string{
		interpreter,
		filepath.Join(base, "python", "cpython-3.11.1-macos-aarch64-none", "bin", "python3.11"),
		filepath.Join(base, "python", "cpython-3.12.14-macos-aarch64-none", "bin", "python3"),
	} {
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	worker, err := resolvePDFUploadWorker(driveRoot, platform)
	if err != nil || worker.python != interpreter || worker.packageDir != tools {
		t.Fatalf("worker = %+v, err = %v", worker, err)
	}
	if _, err := resolvePDFUploadWorker(driveRoot, "linux-arm64"); err == nil {
		t.Fatal("worker available for a platform without any packaged files")
	}
}

func TestPDFUploadsTerminateWithLaunch(t *testing.T) {
	ctx, stopLaunch := context.WithCancel(context.Background())
	defer stopLaunch()
	entered := make(chan struct{})
	stopped := make(chan struct{})
	uploads, err := servePDFUploads(ctx, 8192, func(ctx context.Context, _ pdfUploadInput) ([]pdfUploadDocument, error) {
		close(entered)
		<-ctx.Done()
		close(stopped)
		return nil, ctx.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	result := make(chan error, 1)
	request := uploadTestRequest(t, uploads, "manual.pdf", "application/pdf", "first")
	go func() {
		response, err := client.Do(request)
		if response != nil {
			response.Body.Close()
		}
		result <- err
	}()
	waitForTestChannel(t, entered)
	response, err := client.Do(uploadTestRequest(t, uploads, "manual.pdf", "application/pdf", "second"))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("concurrent upload status = %d, want 503", response.StatusCode)
	}
	stopLaunch()
	waitForTestChannel(t, uploads.done)
	waitForTestChannel(t, stopped)
	<-result
	if _, err := client.Do(uploadTestRequest(t, uploads, "manual.pdf", "application/pdf", "third")); err == nil {
		t.Fatal("listener still accepts uploads after shutdown")
	}
}

func TestPDFUploadWorkerCancelsAndCleansUp(t *testing.T) {
	scratch := t.TempDir()
	t.Setenv("TMPDIR", scratch)
	work := t.TempDir()
	launcher := filepath.Join(work, "python")
	script := "#!/bin/sh\ntest -f upload || exit 1\ntouch \"$0.started\"\nexec /bin/sleep 30\n"
	if err := os.WriteFile(launcher, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	worker := pdfUploadWorker{python: launcher}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	exited := make(chan error, 1)
	go func() {
		_, err := worker.run(ctx, []byte("document"), "pdf")
		exited <- err
	}()
	for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(launcher + ".started"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker did not start")
		}
	}
	cancel()
	select {
	case err := <-exited:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("worker error = %v, want cancellation", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit after cancellation")
	}
	if entries, err := os.ReadDir(scratch); err != nil || len(entries) != 0 {
		t.Fatalf("worker scratch remains after cancellation: %v (%v)", entries, err)
	}
}

func TestPDFUploadWorkerOutputLimit(t *testing.T) {
	output := &pdfWorkerOutput{}
	data := strings.NewReader(strings.Repeat("x", pdfWorkerOutputLimit+1))
	if _, err := io.Copy(output, io.LimitReader(data, int64(data.Len()))); err == nil {
		t.Fatal("worker responses exceeding the output limit were accepted")
	}
}

func TestPDFUploadWorker(t *testing.T) {
	launcher := os.Getenv("SVALBARD_TEST_OPEN_WEBUI_PYTHON")
	if launcher == "" {
		t.Skip("set SVALBARD_TEST_OPEN_WEBUI_PYTHON to Python with the pinned Open WebUI slim packages and pypdf")
	}
	launcher, err := filepath.Abs(launcher)
	if err != nil {
		t.Fatal(err)
	}
	worker := pdfUploadWorker{python: launcher, packageDir: os.Getenv("SVALBARD_TEST_OPEN_WEBUI_PACKAGES")}
	if err := worker.check(t.Context()); err != nil {
		t.Fatal(err)
	}

	var compressed bytes.Buffer
	compressor := zlib.NewWriter(&compressed)
	for range 20 {
		if _, err := compressor.Write(make([]byte, 1024*1024)); err != nil {
			t.Fatal(err)
		}
	}
	if err := compressor.Close(); err != nil {
		t.Fatal(err)
	}
	var manyPages []string
	for range 33 {
		manyPages = append(manyPages, "BT /F1 12 Tf (A page) Tj ET")
	}
	image := "q\nBI /W 1 /H 1 /BPC 8 /CS /G ID \x00\nEI\nQ"
	mixed := makePDF([]string{image, "BT /F1 12 Tf (Page two answer) Tj ET"}, "")
	encrypt := exec.CommandContext(t.Context(), launcher, "-c", "import io, sys; from pypdf import PdfWriter; p = PdfWriter(); p.add_blank_page(612, 792); p.encrypt('password'); out = io.BytesIO(); p.write(out); sys.stdout.buffer.write(out.getvalue())")
	encrypt.Env = append(os.Environ(), "PYTHONPATH="+worker.packageDir)
	encrypted, err := encrypt.Output()
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, mime, data string
		budget           int
		want             []string
		wantError        string
	}{
		{"small.pdf", "application/pdf", mixed, 2048, []string{"[PDF page 2]\nPage two answer"}, ""},
		{"mislabeled.txt", "text/plain", mixed, 2048, []string{"[PDF page 2]\nPage two answer"}, ""},
		{"too-long.pdf", "application/pdf", mixed, 27, nil, "context"},
		{"corrupt.pdf", "application/pdf", "%PDF-1.4\ncorrupt", 2048, nil, "read"},
		{"scan.pdf", "application/pdf", makePDF([]string{image}, ""), 2048, nil, "selectable text"},
		{"encrypted.pdf", "application/pdf", string(encrypted), 2048, nil, "encrypted"},
		{"too-many-pages.pdf", "application/pdf", makePDF(manyPages, ""), 2048, nil, "32 pages"},
		{"expands.pdf", "application/pdf", makePDF([]string{compressed.String()}, "/Filter /FlateDecode"), 2048, nil, "resource limits"},
		{"expands-form.pdf", "application/pdf", makePDF([]string{"BT /F1 12 Tf (Visible text) Tj ET /Form Do"}, "", compressed.String()), 2048, nil, "resource limits"},
		{"text.txt", "text/plain", "Multilingual café\r\n", 2048, []string{"Multilingual café\n"}, ""},
		{"bom.txt", "text/plain", "\xff\xfeC\x00a\x00f\x00\xe9\x00", 2048, []string{"Café"}, ""},
		{"table.csv", "text/csv", "name,level\nAda,3\nLin,4\n", 2048, []string{"name: Ada\nlevel: 3", "name: Lin\nlevel: 4"}, ""},
		{"page.html", "text/html", "<html><title>Title</title><p>café &amp; tea</p><script>not text</script></html>", 2048, []string{"Titlecafé & tea"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := pdfUploadInput{name: tc.name, mimeType: tc.mime, data: []byte(tc.data), pdfBudget: tc.budget}
			input.isPDF = isPDFUpload(input.name, input.mimeType, input.data)
			documents, err := worker.extract(t.Context(), input)
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("error = %v, want %q", err, tc.wantError)
				}
				return
			}
			if err != nil || len(documents) != len(tc.want) {
				t.Fatalf("documents = %v, err = %v", documents, err)
			}
			for i, want := range tc.want {
				if documents[i].PageContent != want {
					t.Errorf("document %d = %q, want %q", i, documents[i].PageContent, want)
				}
				metadata := documents[i].Metadata
				if input.isPDF && (metadata["page"] != float64(1) || metadata["page_label"] != float64(2)) {
					t.Errorf("document %d metadata = %v, want page index 1 and label 2", i, metadata)
				}
			}
		})
	}
}

func uploadTestRequest(t *testing.T, uploads *pdfUploads, filename, mime, body string) *http.Request {
	t.Helper()
	request, err := http.NewRequest(http.MethodPut, uploads.URL+"/process", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+uploads.Token)
	request.Header.Set("Content-Type", mime)
	request.Header.Set("X-Filename", filename)
	return request
}

func waitForTestChannel(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for upload state")
	}
}

func makePDF(pageStreams []string, filters string, compressedForm ...string) string {
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"}
	resources := "/Font << /F1 3 0 R >>"
	if len(compressedForm) != 0 {
		form := compressedForm[0]
		resources += " /XObject << /Form 4 0 R >>"
		objects = append(objects, fmt.Sprintf("<< /Type /XObject /Subtype /Form /BBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Length %d /Filter /FlateDecode >>\nstream\n%s\nendstream", len(form), form))
	}
	var children strings.Builder
	for _, stream := range pageStreams {
		first := len(objects) + 1
		fmt.Fprintf(&children, "%d 0 R ", first)
		objects = append(objects, fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << %s >> /Contents %d 0 R >>", resources, first+1))
		objects = append(objects, fmt.Sprintf("<< /Length %d %s >>\nstream\n%s\nendstream", len(stream), filters, stream))
	}
	objects[1] = fmt.Sprintf("<< /Type /Pages /Count %d /Kids [%s] >>", len(pageStreams), children.String())
	var file strings.Builder
	file.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, object := range objects {
		offsets = append(offsets, file.Len())
		fmt.Fprintf(&file, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	xref := file.Len()
	fmt.Fprintf(&file, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&file, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&file, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return file.String()
}
