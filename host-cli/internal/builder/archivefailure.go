package builder

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
)

func writeArchiveFailure(output string, source archiveSource, cause error, site archiveSiteInfo) ([]archivedPage, error) {
	local := archiveEntryPath(source.URL)
	title := source.Label
	if title == "" {
		title = "Unavailable"
	}
	path := filepath.Join(output, filepath.FromSlash(local))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	var body bytes.Buffer
	archivePageHead(&body, title, archiveRelative(local, site.stylesheet), "project")
	fmt.Fprintf(&body, `<header><p class="back"><a href="%s">%s</a></p><h1>%s</h1><p class="source">Source: %s</p></header><main><p>Unavailable: this project could not be archived (%s).</p></main></body></html>`,
		template.HTMLEscapeString(archiveRelative(local, site.home)), template.HTMLEscapeString(site.title), template.HTMLEscapeString(title), template.HTMLEscapeString(source.URL.String()), template.HTMLEscapeString(cause.Error()))
	if err := os.WriteFile(path, body.Bytes(), 0o644); err != nil {
		return nil, err
	}
	return []archivedPage{{URL: source.URL.String(), Path: local, Title: title, HTML: true}}, nil
}
