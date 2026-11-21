package builder

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// thingiverseAPI is the Thingiverse REST API; tests point it at a local server.
var thingiverseAPI = "https://api.thingiverse.com"

var thingiverseThingPath = regexp.MustCompile(`^/thing:(\d+)`)

func thingiverseAPIHost() string {
	if endpoint, err := url.Parse(thingiverseAPI); err == nil {
		return endpoint.Host
	}
	return ""
}

type thingiverseImage struct {
	Sizes []struct {
		Type string `json:"type"`
		Size string `json:"size"`
		URL  string `json:"url"`
	} `json:"sizes"`
}

// thingiverseArticle reads a thing through the official API, which needs
// THINGIVERSE_TOKEN; without it the page shell is all there is.
func thingiverseArticle(fetcher *archiveFetcher, source *url.URL) (*archiveArticle, error) {
	match := thingiverseThingPath.FindStringSubmatch(source.Path)
	if match == nil || fetcher.authorization[thingiverseAPIHost()] == "" {
		return archiveHTMLArticle(fetcher, source, nil)
	}
	var thing struct {
		Name    string `json:"name"`
		License string `json:"license"`
		Creator struct {
			Name string `json:"name"`
		} `json:"creator"`
		Details      string `json:"details"`
		Description  string `json:"description_html"`
		Instructions string `json:"instructions_html"`
	}
	if err := thingiverseGet(fetcher, "/things/"+match[1], &thing); err != nil {
		return nil, err
	}
	var images []thingiverseImage
	_ = thingiverseGet(fetcher, "/things/"+match[1]+"/images", &images)
	var files []struct {
		Name        string `json:"name"`
		Size        int    `json:"size"`
		DownloadURL string `json:"download_url"`
	}
	_ = thingiverseGet(fetcher, "/things/"+match[1]+"/files", &files)

	article := &archiveArticle{Title: thing.Name, Author: thing.Creator.Name, License: thing.License, Base: source}
	text := thing.Details
	if strings.TrimSpace(archiveFragmentText(text)) == "" {
		text = thing.Description + thing.Instructions
	}
	var page strings.Builder
	for index, image := range images {
		if index == 1 {
			page.WriteString(text)
			page.WriteString("<h2>Pictures</h2>")
		}
		if address := thingiverseImageURL(image); address != "" {
			fmt.Fprintf(&page, `<img src="%s" alt="">`, template.HTMLEscapeString(address))
		}
	}
	if len(images) < 2 {
		page.WriteString(text)
	}
	article.Content = archiveContainer()
	nodes, err := html.ParseFragment(strings.NewReader(page.String()), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return nil, err
	}
	for _, node := range nodes {
		article.Content.AppendChild(node)
	}
	archiveMarkVideoEmbeds(fetcher, article, article.Content)
	pruneArchiveElements(article.Content)
	for _, file := range files {
		if file.DownloadURL != "" && file.Size <= maxArchiveResponseBytes {
			article.Files = append(article.Files, archiveFile{Name: file.Name, URL: file.DownloadURL})
		}
	}
	return article, nil
}

// thingiverseImageURL prefers the original upload over resized previews.
func thingiverseImageURL(image thingiverseImage) string {
	fallback := ""
	for _, size := range image.Sizes {
		if strings.HasPrefix(size.URL, "https://cdn.thingiverse.com/assets/") {
			return size.URL
		}
		if fallback == "" || size.Type == "display" && size.Size == "large" {
			fallback = size.URL
		}
	}
	return fallback
}

func thingiverseGet(fetcher *archiveFetcher, path string, result any) error {
	endpoint, err := url.Parse(thingiverseAPI + path)
	if err != nil {
		return err
	}
	body, _, _, err := fetcher.get(endpoint, maxArchiveResponseBytes)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("thingiverse API %s: %w", path, err)
	}
	return nil
}

func archiveFragmentText(fragment string) string {
	nodes, err := html.ParseFragment(strings.NewReader(fragment), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return ""
	}
	var text strings.Builder
	for _, node := range nodes {
		text.WriteString(archiveNodeText(node))
	}
	return text.String()
}
