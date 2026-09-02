package builder

import (
	"bytes"
	"net/url"
	"os"
	"path/filepath"

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
				if stripLinks && stripArchiveLink(child) {
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

func stripArchiveLink(node *html.Node) bool {
	if node.Type != html.ElementNode || node.Data != "a" {
		return false
	}
	attributes := node.Attr[:0]
	removed := false
	for _, attribute := range node.Attr {
		if attribute.Key == "href" {
			removed = true
			continue
		}
		attributes = append(attributes, attribute)
	}
	node.Attr = attributes
	return removed
}

func missingArchiveResource(node *html.Node, documentPath, output string) bool {
	if node.Type != html.ElementNode {
		return false
	}
	for _, attribute := range node.Attr {
		if attribute.Key != "src" {
			continue
		}
		target, err := url.Parse(attribute.Val)
		if err != nil || target.IsAbs() || target.Path == "" {
			continue
		}
		local := filepath.Join(filepath.Dir(documentPath), filepath.FromSlash(target.Path))
		if !pathWithin(local, output) {
			return true
		}
		if _, err := os.Stat(local); err != nil {
			return true
		}
	}
	return false
}
