package frontpage

import (
	"archive/zip"
	"bytes"
	"fmt"
	"testing"
)

func epubFixture(t *testing.T, files map[string]string) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	z, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return z
}

func TestNavContentsReadsNestedTOCOnly(t *testing.T) {
	z := epubFixture(t, map[string]string{"OEBPS/nav.xhtml": `<?xml version="1.0"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops"><body>
<nav epub:type="toc"><ol>
  <li><a href="text/one.xhtml">Book  One&nbsp;</a><ol><li><a href="text/one.xhtml#s2">Chapter <i>2</i></a></li></ol></li>
  <li><a href="https://example.com/x">Remote</a></li>
  <li><a href="../../escape.xhtml">Escape</a></li>
</ol></nav>
<nav epub:type="landmarks"><ol><li><a href="text/cover.xhtml">Cover</a></li></ol></nav>
</body></html>`})
	got := fmt.Sprint(navContents(z, "OEBPS/nav.xhtml"))
	want := "[{Book One OEBPS/text/one.xhtml  0} {Chapter 2 OEBPS/text/one.xhtml s2 1}]"
	if got != want {
		t.Fatalf("navContents = %s, want %s", got, want)
	}
}

func TestNCXContentsFlattensNavMap(t *testing.T) {
	z := epubFixture(t, map[string]string{"toc.ncx": `<ncx><navMap>
  <navPoint><navLabel><text>Part I</text></navLabel><content src="a.xhtml"/>
    <navPoint><navLabel><text>The siege</text></navLabel><content src="a.xhtml#siege"/></navPoint>
  </navPoint>
  <navPoint><navLabel><text>Part II</text></navLabel><content src="b.xhtml"/></navPoint>
</navMap></ncx>`})
	got := fmt.Sprint(ncxContents(z, "toc.ncx"))
	want := "[{Part I a.xhtml  0} {The siege a.xhtml siege 1} {Part II b.xhtml  0}]"
	if got != want {
		t.Fatalf("ncxContents = %s, want %s", got, want)
	}
}
