package builder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestArchivePDFURLsExtractsHTTPLinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "links.pdf")
	if err := os.WriteFile(path, minimalTextPDF("https://example.test/project"), 0o644); err != nil {
		t.Fatal(err)
	}
	sources, err := archivePDFSources(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].URL.String() != "https://example.test/project" || sources[0].Page != 1 {
		t.Fatalf("archivePDFSources() = %#v", sources)
	}
}

func TestArchiveURLsFromTextDeduplicatesAndTrimsPunctuation(t *testing.T) {
	urls := archiveURLsFromText("See https://example.test/a, https://example.test/a and https://other.test/b.")
	if len(urls) != 2 || urls[0].String() != "https://example.test/a" || urls[1].String() != "https://other.test/b" {
		t.Fatalf("archiveURLsFromText() = %#v", urls)
	}
}

func TestLimitArchiveSourcesPreservesSourceOrder(t *testing.T) {
	sources := []archiveSource{{ID: "first"}, {ID: "second"}, {ID: "third"}}
	got := limitArchiveSources(sources, 2)
	if len(got) != 2 || got[0].ID != "first" || got[1].ID != "second" {
		t.Fatalf("limitArchiveSources() = %#v", got)
	}
}

func minimalTextPDF(text string) []byte {
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R /Annots [6 0 R] >>",
		fmt.Sprintf("<< /Length %d >>\nstream\nBT /F1 12 Tf 72 720 Td (%s) Tj ET\nendstream", len("BT /F1 12 Tf 72 720 Td ("+text+") Tj ET"), text),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		fmt.Sprintf("<< /Type /Annot /Subtype /Link /Rect [0 0 1 1] /A << /S /URI /URI (%s) >> >>", text),
	}
	var document bytes.Buffer
	document.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = document.Len()
		fmt.Fprintf(&document, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xref := document.Len()
	fmt.Fprintf(&document, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&document, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&document, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return document.Bytes()
}
