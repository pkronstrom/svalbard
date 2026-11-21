package builder

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type instructablesFile struct {
	Image       bool   `json:"image"`
	Name        string `json:"name"`
	DownloadURL string `json:"downloadUrl"`
	Embed       string `json:"embedHtmlCode"`
}

type instructablesModel struct {
	Title    string `json:"title"`
	Supplies string `json:"supplies"`
	Author   struct {
		ScreenName string `json:"screenName"`
	} `json:"author"`
	CoverImage instructablesFile `json:"coverImage"`
	License    struct {
		FullName string `json:"fullName"`
	} `json:"license"`
	SuppliesFiles []instructablesFile `json:"suppliesFiles"`
	Steps         []struct {
		Title string              `json:"title"`
		Body  string              `json:"body"`
		Files []instructablesFile `json:"files"`
	} `json:"steps"`
}

// instructablesArticle reads the JSON the Instructables page itself loads; the
// served HTML is an empty application shell.
func instructablesArticle(fetcher *archiveFetcher, source *url.URL) (*archiveArticle, error) {
	slug := strings.Split(strings.Trim(source.Path, "/"), "/")[0]
	if slug == "" {
		return archiveHTMLArticle(fetcher, source, nil)
	}
	endpoint := &url.URL{Scheme: source.Scheme, Host: source.Host, Path: "/json-api/showInstructableModel", RawQuery: url.Values{"urlString": {slug}}.Encode()}
	body, _, _, err := fetcher.get(endpoint, maxArchiveResponseBytes)
	if err != nil {
		return nil, err
	}
	var model instructablesModel
	if err := json.Unmarshal(body, &model); err != nil {
		return nil, fmt.Errorf("GET %s: %w", endpoint, err)
	}
	if len(model.Steps) == 0 {
		return nil, fmt.Errorf("GET %s: no steps", endpoint)
	}
	article := &archiveArticle{Title: model.Title, Author: model.Author.ScreenName, License: model.License.FullName, Base: source, Image: model.CoverImage.DownloadURL}
	var page strings.Builder
	files := func(list []instructablesFile) {
		for _, file := range list {
			if strings.Contains(file.Embed, "youtube") {
				page.WriteString(file.Embed)
				continue
			}
			if file.DownloadURL == "" {
				continue
			}
			if file.Image {
				fmt.Fprintf(&page, `<img src="%s" alt="%s">`, template.HTMLEscapeString(file.DownloadURL+"?width=1024&fit=bounds"), template.HTMLEscapeString(file.Name))
			} else {
				article.Files = append(article.Files, archiveFile{Name: file.Name, URL: file.DownloadURL})
			}
		}
	}
	for index, step := range model.Steps {
		if index > 0 {
			fmt.Fprintf(&page, "<h2>Step %d: %s</h2>", index, template.HTMLEscapeString(step.Title))
		}
		files(step.Files)
		page.WriteString(step.Body)
		if index == 0 && model.Supplies != "" {
			page.WriteString("<h2>Supplies</h2>")
			files(model.SuppliesFiles)
			page.WriteString(model.Supplies)
		}
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
	return article, nil
}
