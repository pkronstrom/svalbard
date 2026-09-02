package builder

import (
	"fmt"
	"html/template"
	"net/url"
	"os"
	"path/filepath"
)

func writeArchiveFailure(output string, source *url.URL, cause error) ([]archivedPage, error) {
	local := archiveLocalPath(source, "")
	path := filepath.Join(output, filepath.FromSlash(local))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	body := fmt.Sprintf(`<!doctype html><html><head><meta charset="utf-8"><title>Unavailable</title></head><body><h1>Unavailable</h1><p>%s</p><p>Source: %q</p></body></html>`, template.HTMLEscapeString(cause.Error()), source.String())
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return nil, err
	}
	return []archivedPage{{URL: source.String(), Path: local, Title: "Unavailable", HTML: true}}, nil
}
