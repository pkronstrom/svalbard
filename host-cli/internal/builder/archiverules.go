package builder

import (
	"fmt"
	"strings"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
)

type compiledArchiveRule struct {
	domain  string
	content cascadia.Selector
	remove  []cascadia.Selector
}

func compileArchiveRules(rules []catalog.ArchiveRule) ([]compiledArchiveRule, error) {
	compiled := make([]compiledArchiveRule, 0, len(rules))
	for _, rule := range rules {
		domain := strings.ToLower(strings.TrimSpace(rule.Domain))
		if domain == "" {
			return nil, fmt.Errorf("archive rule domain required")
		}
		var content cascadia.Selector
		if rule.Content != "" {
			selector, err := cascadia.Compile(rule.Content)
			if err != nil {
				return nil, fmt.Errorf("archive rule %s content selector %q: %w", domain, rule.Content, err)
			}
			content = selector
		}
		selectors := make([]cascadia.Selector, 0, len(rule.Remove))
		for _, raw := range rule.Remove {
			selector, err := cascadia.Compile(raw)
			if err != nil {
				return nil, fmt.Errorf("archive rule %s selector %q: %w", domain, raw, err)
			}
			selectors = append(selectors, selector)
		}
		compiled = append(compiled, compiledArchiveRule{domain: domain, content: content, remove: selectors})
	}
	return compiled, nil
}

func archiveRuleFor(host string, rules []compiledArchiveRule) *compiledArchiveRule {
	host = strings.ToLower(host)
	for index := range rules {
		domain := rules[index].domain
		if host == domain || strings.HasPrefix(domain, "*.") && strings.HasSuffix(host, domain[1:]) {
			return &rules[index]
		}
	}
	return nil
}

func applyArchiveRule(document *html.Node, rule *compiledArchiveRule) {
	if rule == nil {
		return
	}
	if rule.content != nil {
		selectArchiveContent(document, cascadia.Query(document, rule.content))
	}
	for _, selector := range rule.remove {
		for _, node := range cascadia.QueryAll(document, selector) {
			if node.Parent != nil {
				node.Parent.RemoveChild(node)
			}
		}
	}
}

func selectArchiveContent(document, selected *html.Node) {
	if selected == nil {
		return
	}
	body := archiveBody(document)
	if body == nil || selected == body {
		return
	}
	if selected.Parent != nil {
		selected.Parent.RemoveChild(selected)
	}
	for child := body.FirstChild; child != nil; {
		next := child.NextSibling
		body.RemoveChild(child)
		child = next
	}
	body.AppendChild(selected)
}

func archiveBody(node *html.Node) *html.Node {
	if node.Type == html.ElementNode && node.Data == "body" {
		return node
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if body := archiveBody(child); body != nil {
			return body
		}
	}
	return nil
}
