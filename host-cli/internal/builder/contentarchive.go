package builder

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/net/html"

	"github.com/pkronstrom/svalbard/host-cli/internal/catalog"
	"github.com/pkronstrom/svalbard/host-cli/internal/downloader"
	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
	"github.com/pkronstrom/svalbard/host-cli/internal/toolkit"
)

const maxArchiveResponseBytes = 32 << 20

type archivedPage struct {
	URL        string `json:"url"`
	Path       string `json:"path"`
	HTML       bool   `json:"html"`
	ProjectID  string `json:"project_id,omitempty"`
	SourcePage int    `json:"source_page,omitempty"`
}

type archiveManifest struct {
	Source string         `json:"source"`
	Pages  []archivedPage `json:"pages"`
}

// buildContentArchive copies a bounded static site, or the landing pages linked
// by a PDF, into a ZIM. Browser-only pages stay on dedicated builders.
func buildContentArchive(root string, recipe catalog.Item, _ *catalog.Catalog, opts Options) ([]manifest.RealizedEntry, error) {
	if recipe.Build == nil || recipe.Build.SourceURL == "" {
		return nil, fmt.Errorf("content-archive %s: source_url required", recipe.ID)
	}
	if recipe.Type != "zim" {
		return nil, fmt.Errorf("content-archive %s: type must be zim", recipe.ID)
	}
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	workdir := filepath.Join(root, ".staging", "build", recipe.ID)
	site := filepath.Join(workdir, "site")
	if err := os.RemoveAll(site); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(site, 0o755); err != nil {
		return nil, err
	}

	sources, err := archiveSources(ctx, recipe.Build, workdir)
	if err != nil {
		return nil, fmt.Errorf("content-archive %s: %w", recipe.ID, err)
	}
	pdfLinks := recipe.Build.Config["source_format"] == "pdf-links"
	limit := configInt(recipe.Build.Config, "max_pages", 100)
	if pdfLinks {
		limit = configInt(recipe.Build.Config, "max_pages", 1)
	}
	pages := make([]archivedPage, 0)
	for _, source := range sources {
		client := &http.Client{CheckRedirect: func(request *http.Request, _ []*http.Request) error {
			if request.URL.Scheme != source.URL.Scheme || request.URL.Host != source.URL.Host {
				return http.ErrUseLastResponse
			}
			return nil
		}}
		prefix := ""
		if pdfLinks || len(sources) > 1 {
			prefix = filepath.Join("projects", source.ID)
		}
		copied, err := archiveSite(ctx, client, source.URL, filepath.Join(site, prefix), limit, !pdfLinks)
		if err != nil {
			return nil, fmt.Errorf("content-archive %s: %w", recipe.ID, err)
		}
		if prefix != "" {
			if err := writeArchiveProject(filepath.Join(site, prefix, "project.json"), source, archiveLocalPath(source.URL, "")); err != nil {
				return nil, err
			}
		}
		for index := range copied {
			copied[index].Path = filepath.ToSlash(filepath.Join(prefix, copied[index].Path))
			copied[index].SourcePage = source.Page
			copied[index].ProjectID = source.ID
		}
		pages = append(pages, copied...)
	}
	if pdfLinks || len(sources) > 1 {
		if err := writeArchiveIndex(filepath.Join(site, "index.html"), sources); err != nil {
			return nil, err
		}
	}
	encoded, err := json.MarshalIndent(archiveManifest{Source: recipe.Build.SourceURL, Pages: pages}, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(site, "manifest.json"), encoded, 0o644); err != nil {
		return nil, err
	}

	outputName := recipe.Build.Output
	if outputName == "" {
		outputName = recipe.ID + ".zim"
	}
	output := filepath.Join(root, toolkit.TypeDirs[recipe.Type], outputName)
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return nil, err
	}
	args := []string{
		"--welcome=index.html", "--language=eng", "--title=" + archiveMetadata(recipe, "title", recipe.ID),
		"--description=" + archiveMetadata(recipe, "description", recipe.Description),
		"--publisher=Svalbard", "--name=" + recipe.ID, "--withFullTextIndex",
		fmt.Sprintf("--threads=%d", runtime.NumCPU()), site, output,
	}
	if _, err := runToolCommand(ctx, root, workdir, BaseToolsImage, "zimwriterfs", args); err != nil {
		return nil, err
	}
	if _, err := runToolCommand(ctx, root, workdir, BaseToolsImage, "zimcheck", []string{output}); err != nil {
		return nil, err
	}
	info, err := os.Stat(output)
	if err != nil {
		return nil, err
	}
	checksum, _ := downloader.ComputeSHA256(output)
	return []manifest.RealizedEntry{{
		ID: recipe.ID, Type: recipe.Type, Filename: outputName,
		RelativePath: filepath.Join(toolkit.TypeDirs[recipe.Type], outputName),
		SizeBytes:    info.Size(), ChecksumSHA256: checksum, SourceStrategy: "build",
	}}, nil
}

func archiveSite(ctx context.Context, client *http.Client, source *url.URL, output string, limit int, strictLimit bool) ([]archivedPage, error) {
	queue := []*url.URL{source}
	seen := make(map[string]bool)
	pages := make([]archivedPage, 0, limit)
	htmlPages := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		key := archiveURLKey(current)
		if seen[key] {
			continue
		}
		seen[key] = true
		body, contentType, err := archiveFetch(ctx, client, current)
		if err != nil {
			if current.String() == source.String() {
				return nil, err
			}
			continue
		}
		local := archiveLocalPath(current, contentType)
		destination := filepath.Join(output, filepath.FromSlash(local))
		if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return nil, err
		}
		isHTML := strings.Contains(contentType, "text/html") || strings.HasSuffix(strings.ToLower(current.Path), ".html") || strings.HasSuffix(current.Path, "/")
		if isHTML {
			if htmlPages >= limit {
				if strictLimit {
					return nil, fmt.Errorf("page limit %d reached; raise build.max_pages to continue", limit)
				}
				continue
			}
			htmlPages++
			body, queue = archiveHTML(body, current, source, local, queue)
		}
		if err := os.WriteFile(destination, body, 0o644); err != nil {
			return nil, err
		}
		pages = append(pages, archivedPage{URL: current.String(), Path: local, HTML: isHTML})
	}
	return pages, nil
}

func archiveFetch(ctx context.Context, client *http.Client, target *url.URL) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "svalbard-content-archive/1")
	response, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, "", fmt.Errorf("GET %s: %s", target, response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxArchiveResponseBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(body) > maxArchiveResponseBytes {
		return nil, "", fmt.Errorf("GET %s: response exceeds %d MiB", target, maxArchiveResponseBytes>>20)
	}
	return body, response.Header.Get("Content-Type"), nil
}

func archiveHTML(body []byte, current, source *url.URL, currentPath string, queue []*url.URL) ([]byte, []*url.URL) {
	document, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return body, queue
	}
	var rewrite func(*html.Node)
	rewrite = func(node *html.Node) {
		if node.Type == html.ElementNode {
			for index := range node.Attr {
				attribute := &node.Attr[index]
				if attribute.Key != "href" && attribute.Key != "src" {
					continue
				}
				target, err := current.Parse(attribute.Val)
				if err != nil || target.Scheme != source.Scheme || target.Host != source.Host {
					continue
				}
				target.Fragment = ""
				queue = append(queue, target)
				targetPath := archiveLocalPath(target, "")
				relative, err := filepath.Rel(filepath.Dir(filepath.FromSlash(currentPath)), filepath.FromSlash(targetPath))
				if err == nil {
					attribute.Val = filepath.ToSlash(relative)
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			rewrite(child)
		}
	}
	rewrite(document)
	var rendered bytes.Buffer
	if err := html.Render(&rendered, document); err != nil {
		return body, queue
	}
	return rendered.Bytes(), queue
}

func archiveURL(raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid HTTP URL %q", raw)
	}
	parsed.Fragment = ""
	return parsed, nil
}

func archiveURLKey(target *url.URL) string {
	clone := *target
	clone.Fragment = ""
	return clone.String()
}

func archiveLocalPath(target *url.URL, _ string) string {
	clean := path.Clean("/" + target.EscapedPath())
	if clean == "/" || strings.HasSuffix(target.Path, "/") {
		clean = path.Join(clean, "index.html")
	}
	if target.RawQuery != "" {
		sum := sha256.Sum256([]byte(target.RawQuery))
		ext := path.Ext(clean)
		clean = strings.TrimSuffix(clean, ext) + "-" + hex.EncodeToString(sum[:4]) + ext
	}
	if path.Ext(clean) == "" {
		clean += ".html"
	}
	return strings.TrimPrefix(clean, "/")
}

func archiveMetadata(recipe catalog.Item, key, fallback string) string {
	if recipe.Build.Config[key] != "" {
		return recipe.Build.Config[key]
	}
	return fallback
}
