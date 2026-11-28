package websearch

import (
	"bytes"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func parseDuckDuckGo(body []byte, limit int) ([]Result, error) {
	page, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("unrecognized HTML response")
	}
	results := make([]Result, 0, limit)
	explicitlyEmpty := false
	for node := range page.Descendants() {
		if node.Type != html.ElementNode {
			continue
		}
		if attribute(node, "id") == "challenge-form" || hasClass(node, "anomaly-modal__modal") {
			return nil, errors.New("provider returned a bot challenge")
		}
		if hasClass(node, "no-results") || hasClass(node, "result--no-result") {
			explicitlyEmpty = true
		}
		if len(results) == limit || !hasClass(node, "result") || hasClass(node, "result--ad") {
			continue
		}
		link, snippet := duckDuckGoFields(node)
		if link != nil {
			results = addResult(results, nodeText(link), duckDuckGoURL(attribute(link, "href")), nodeText(snippet))
		}
	}
	if len(results) == 0 && !explicitlyEmpty {
		return nil, errors.New("no usable results or unrecognized HTML response")
	}
	return results, nil
}

func duckDuckGoFields(row *html.Node) (link, snippet *html.Node) {
	for node := range row.Descendants() {
		if link == nil && node.DataAtom == atom.A && hasClass(node, "result__a") {
			link = node
		}
		if snippet == nil && hasClass(node, "result__snippet") {
			snippet = node
		}
		if link != nil && snippet != nil {
			break
		}
	}
	return link, snippet
}

func duckDuckGoURL(href string) string {
	switch {
	case strings.HasPrefix(href, "//"):
		href = "https:" + href
	case strings.HasPrefix(href, "/l/"):
		href = "https://duckduckgo.com" + href
	}
	target, err := parseWebURL(href)
	if err != nil {
		return ""
	}
	switch strings.ToLower(target.Hostname()) {
	case "duckduckgo.com", "www.duckduckgo.com", "html.duckduckgo.com":
		switch target.Path {
		case "/l", "/l/":
			return target.Query().Get("uddg")
		case "/y.js":
			return ""
		}
	}
	return href
}

// SearXNG and Tavily both return title, url and content in a results array.
func parseJSONResults(body []byte, limit int) ([]Result, error) {
	var response struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
		Unresponsive []json.RawMessage `json:"unresponsive_engines"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.Results == nil {
		return nil, errors.New("unrecognized JSON response")
	}
	results := make([]Result, 0, limit)
	for _, row := range response.Results {
		results = addResult(results, markupText(row.Title), row.URL, markupText(row.Content))
		if len(results) == limit {
			break
		}
	}
	if len(results) == 0 {
		if len(response.Unresponsive) > 0 {
			return nil, errors.New("no usable results and some search engines did not respond")
		}
		if len(response.Results) > 0 {
			return nil, errors.New("no usable results")
		}
	}
	return results, nil
}

func hasClass(node *html.Node, class string) bool {
	return slices.Contains(strings.Fields(attribute(node, "class")), class)
}

func attribute(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func markupText(markup string) string {
	if !strings.ContainsAny(markup, "<&") {
		return markup
	}
	container := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := html.ParseFragment(strings.NewReader(markup), container)
	if err != nil {
		return ""
	}
	for _, node := range nodes {
		container.AppendChild(node)
	}
	return nodeText(container)
}

func nodeText(node *html.Node) string {
	if node == nil {
		return ""
	}
	var text strings.Builder
	appendNodeText(&text, node)
	return text.String()
}

func appendNodeText(text *strings.Builder, node *html.Node) {
	if node.Type == html.TextNode {
		text.WriteString(node.Data)
		return
	}
	switch node.DataAtom {
	case atom.Script, atom.Style, atom.Template, atom.Noscript:
		return
	case atom.Br, atom.Div, atom.P, atom.Li:
		text.WriteByte(' ')
		defer text.WriteByte(' ')
	}
	for child := range node.ChildNodes() {
		appendNodeText(text, child)
	}
}
