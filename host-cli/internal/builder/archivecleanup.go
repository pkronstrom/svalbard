package builder

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

func pruneMissingArchiveResources(output string, pages []archivedPage, stripLinks bool) error {
	for _, page := range pages {
		if !page.HTML {
			continue
		}
		path := filepath.Join(output, filepath.FromSlash(page.Path))
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		document, err := html.Parse(bytes.NewReader(body))
		if err != nil {
			return err
		}
		changed := false
		var prune func(*html.Node)
		prune = func(node *html.Node) {
			for child := node.FirstChild; child != nil; {
				next := child.NextSibling
				if stripLinks && stripArchiveLink(child, path, output) {
					changed = true
				}
				if missingArchiveResource(child, path, output) {
					node.RemoveChild(child)
					changed = true
				} else {
					prune(child)
				}
				child = next
			}
		}
		prune(document)
		if !changed {
			continue
		}
		var rendered bytes.Buffer
		if err := html.Render(&rendered, document); err != nil {
			return err
		}
		if err := os.WriteFile(path, rendered.Bytes(), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// stripArchiveLink removes a link unless it points at a file in the archive.
func stripArchiveLink(node *html.Node, documentPath, output string) bool {
	if node.Type != html.ElementNode || node.Data != "a" {
		return false
	}
	attributes := node.Attr[:0]
	removed := false
	for _, attribute := range node.Attr {
		if attribute.Key == "href" && !archiveLocalTarget(attribute.Val, documentPath, output) {
			removed = true
			continue
		}
		attributes = append(attributes, attribute)
	}
	node.Attr = attributes
	return removed
}

func archiveLocalTarget(reference, documentPath, output string) bool {
	target, err := url.Parse(reference)
	if err != nil || target.IsAbs() || target.Host != "" || target.Path == "" || strings.HasPrefix(target.Path, "/") {
		return false
	}
	local := filepath.Join(filepath.Dir(documentPath), filepath.FromSlash(target.Path))
	if !pathWithin(local, output) {
		return false
	}
	info, err := os.Stat(local)
	return err == nil && info.Mode().IsRegular()
}

func missingArchiveResource(node *html.Node, documentPath, output string) bool {
	if node.Type != html.ElementNode {
		return false
	}
	for _, attribute := range node.Attr {
		if attribute.Key != "src" {
			continue
		}
		if !archiveLocalTarget(attribute.Val, documentPath, output) {
			return true
		}
	}
	return false
}
