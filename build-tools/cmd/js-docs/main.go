// js-docs writes the reference file of each library of a js-lib recipe: the
// project's own documentation at the pinned version, joined into one
// Markdown file that host-cli embeds and apply writes beside the library's
// quickstart.
//
//	js-docs -spec recipes/js/js-core.lock.json -recipe recipes/js/js-core.yaml \
//	        -out host-cli/internal/toolkit/libdocs/js-core
//
// Each file starts with the package's README from its pinned npm tarball
// (the one js-lock verified, read from the same cache). Then, per the spec's
// "docs" entry for the library: the Markdown files of its repository at the
// tag of the pinned version ("repo", "ref", "include", "exclude"), and type
// definitions from the tarball ("types") where the project publishes no
// Markdown docs. Every file becomes "##" sections titled by the file, so the
// drive's search finds them one section at a time. The recipe's
// libraries[].reference records what went in and its licence.
//
//	"docs": {
//	  "chart.js": {"repo": "chartjs/Chart.js", "ref": "v{version}", "include": ["docs/**/*.md"],
//	               "exclude": ["docs/samples/**"], "license": "MIT"},
//	  "dompurify": {"types": ["dist/purify.es.d.mts"], "license": "MPL-2.0 OR Apache-2.0"}
//	}
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type docsSpec struct {
	ID   string                    `json:"id"`
	Docs map[string]libraryDocSpec `json:"docs"`
}

type libraryDocSpec struct {
	Repo    string   `json:"repo"`
	Ref     string   `json:"ref"`
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
	Types   []string `json:"types"`
	License string   `json:"license"`
	// Names lists file names matching a tarball pattern as one section,
	// for libraries whose API is a set of names (Lucide's icons).
	Names     string `json:"names"`
	NamesNote string `json:"names_note"`
	// NoReadme leaves the tarball's README out, when the docs repeat it.
	NoReadme bool `json:"no_readme"`
	// Gap says what the project's own docs are, when they could not be
	// joined (HTML or generated pages), for the checkpoint.
	Gap string `json:"gap"`
}

type generator struct {
	cache  string
	client *http.Client
	github string // API base
	raw    string // raw file base
}

func main() {
	specPath := flag.String("spec", "", "lock spec with a docs section")
	recipePath := flag.String("recipe", "", "the recipe js-lock wrote")
	outDir := flag.String("out", "", "folder for the reference files")
	cache := flag.String("cache", "", "js-lock's tarball cache (default: the user cache)")
	flag.Parse()
	if *specPath == "" || *recipePath == "" || *outDir == "" {
		flag.Usage()
		os.Exit(2)
	}
	if *cache == "" {
		dir, err := os.UserCacheDir()
		if err != nil {
			log.Fatal(err)
		}
		*cache = filepath.Join(dir, "svalbard", "js-lock")
	}
	g := &generator{cache: *cache, client: &http.Client{Timeout: 2 * time.Minute}, github: "https://api.github.com", raw: "https://raw.githubusercontent.com"}
	if err := g.run(context.Background(), *specPath, *recipePath, *outDir); err != nil {
		fmt.Fprintln(os.Stderr, "js-docs:", err)
		os.Exit(1)
	}
}

type recipeLibrary struct {
	pkg, version, license string
	tarball               string // cache file
}

func (g *generator) run(ctx context.Context, specPath, recipePath, outDir string) error {
	var spec docsSpec
	data, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		return err
	}
	recipeData, err := os.ReadFile(recipePath)
	if err != nil {
		return err
	}
	doc := &yaml.Node{}
	if err := yaml.Unmarshal(recipeData, doc); err != nil {
		return err
	}
	root := doc.Content[0]
	libs := field(root, "libraries")
	if libs == nil {
		return fmt.Errorf("%s has no libraries", recipePath)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	written := map[string]bool{}
	for _, item := range libs.Content {
		lib := recipeLibrary{pkg: value(item, "package"), version: value(item, "version"), license: value(item, "license")}
		lib.tarball = filepath.Join(g.cache, strings.ReplaceAll(lib.pkg, "/", "_")+"-"+lib.version+".tgz")
		ds := spec.Docs[lib.pkg]
		text, summary, err := g.reference(ctx, lib, ds)
		if err != nil {
			return fmt.Errorf("%s: %w", lib.pkg, err)
		}
		name := strings.ReplaceAll(lib.pkg, "/", "__") + ".reference.md"
		if err := os.WriteFile(filepath.Join(outDir, name), []byte(text), 0o644); err != nil {
			return err
		}
		written[name] = true
		setField(item, "reference", summary+fmt.Sprintf(", %d KB", (len(text)+512)/1024))
		fmt.Fprintf(os.Stderr, "%s: %d KB (%s)\n", name, (len(text)+512)/1024, summary)
	}
	// Reference files of libraries the recipe no longer has.
	entries, _ := os.ReadDir(outDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".reference.md") && !written[e.Name()] {
			os.Remove(filepath.Join(outDir, e.Name()))
		}
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return err
	}
	enc.Close()
	return os.WriteFile(recipePath, buf.Bytes(), 0o644)
}

// reference builds one library's file and a one-line summary of its sources.
func (g *generator) reference(ctx context.Context, lib recipeLibrary, ds libraryDocSpec) (string, string, error) {
	files, err := tarballFiles(lib.tarball)
	if err != nil {
		return "", "", err
	}
	license := ds.License
	if license == "" {
		license = lib.license
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s %s reference\n\n%s\n\n", lib.pkg, lib.version, offlineBanner)
	var parts []string
	if !ds.NoReadme {
		for _, name := range []string{"README.md", "readme.md", "Readme.md", "README.markdown", "README"} {
			if text, ok := files[name]; ok {
				appendMarkdown(&b, "README", text)
				parts = append(parts, "README")
				break
			}
		}
	}
	if ds.Repo != "" {
		ref := strings.ReplaceAll(ds.Ref, "{version}", lib.version)
		if !strings.Contains(ref, lib.version) {
			return "", "", fmt.Errorf("docs ref %s is not the pinned version %s", ref, lib.version)
		}
		paths, err := g.repoMarkdown(ctx, ds.Repo, ref, ds.Include, ds.Exclude)
		if err != nil {
			return "", "", err
		}
		if len(paths) == 0 {
			return "", "", fmt.Errorf("no Markdown in %s at %s matches %v", ds.Repo, ref, ds.Include)
		}
		for _, p := range paths {
			text, err := g.rawFile(ctx, ds.Repo, ref, p)
			if err != nil {
				return "", "", err
			}
			appendMarkdown(&b, strings.TrimSuffix(path.Base(p), ".md"), text)
		}
		parts = append(parts, fmt.Sprintf("%d Markdown files (%s) of github.com/%s at %s", len(paths), strings.Join(ds.Include, ", "), ds.Repo, ref))
	}
	if len(ds.Types) > 0 {
		var names []string
		for name := range files {
			for _, pattern := range ds.Types {
				if matchGlob(pattern, name) {
					names = append(names, name)
					break
				}
			}
		}
		sort.Strings(names)
		if len(names) == 0 {
			return "", "", fmt.Errorf("no type definitions match %v", ds.Types)
		}
		fmt.Fprintf(&b, "## Types\n\nThe package's type definitions, %d files.\n\n", len(names))
		for _, name := range names {
			fmt.Fprintf(&b, "### %s\n\n```ts\n%s\n```\n\n", name, markNetworkLines(strings.TrimSpace(files[name])))
		}
		parts = append(parts, fmt.Sprintf("types (%s)", strings.Join(ds.Types, ", ")))
	}
	if ds.Names != "" {
		var names []string
		for name := range files {
			if matchGlob(ds.Names, name) {
				base := path.Base(name)
				names = append(names, strings.TrimSuffix(base, path.Ext(base)))
			}
		}
		sort.Strings(names)
		fmt.Fprintf(&b, "## Names\n\n%s\n\n%s\n\n", ds.NamesNote, strings.Join(names, ", "))
		parts = append(parts, fmt.Sprintf("%d names", len(names)))
	}
	if len(parts) == 0 {
		return "", "", errors.New("nothing to put in the reference file")
	}
	summary := strings.Join(parts, ", ") + "; " + license
	if ds.Gap != "" {
		summary += "; gap: " + ds.Gap
	}
	return b.String(), summary, nil
}

// offlineBanner opens every reference file: upstream docs tell readers to
// install packages and link CDNs, which a drive without a network cannot.
const offlineBanner = "This is the project's own documentation, joined at the version on this drive. Here nothing is installed or downloaded: import the package by name in a module script and pack the page with `svalbard-drive html pack`. Code blocks marked `not-for-this-stick` and lines starting \"Not for this drive:\" show package managers, CDN links or downloads; this drive does not use them."

// networkRules find what a reader of the docs must not do on the drive: run
// a package manager, download, or load a script, stylesheet or module from
// a network address. They follow host-cli's skill lint (skills_lint.go).
var networkRules = []*regexp.Regexp{
	regexp.MustCompile(`(^|[^\w.-])(npm|pnpm|npx)\b`),
	regexp.MustCompile(`\b(yarn|bun)\s+(add|install|x|dlx)\b`),
	regexp.MustCompile(`(^|[^\w.-])(curl|wget)\b`),
	regexp.MustCompile(`\bpip3?\s+install\b`),
	regexp.MustCompile(`\bgit\s+(clone|fetch|pull)\b`),
	regexp.MustCompile(`(?i)\bsrcset\s*=\s*["']?[^"'>]*(https?:)?//`),
	regexp.MustCompile(`(?i)@import\s+(url\(\s*)?["']?(https?:)?//`),
	regexp.MustCompile(`(?i)\bsrc\s*=\s*["']?(https?:)?//`),
	regexp.MustCompile(`(?i)<link\b[^>]*\bhref\s*=\s*["']?(https?:)?//`),
	regexp.MustCompile(`\bimport\b[^;]*?\bfrom\s*["'](https?:)?//`),
	regexp.MustCompile(`\bimport\s*\(\s*["'](https?:)?//`),
	regexp.MustCompile("\\bfetch\\s*\\(\\s*[\"'`](https?:)?//"),
	regexp.MustCompile(`(?i)\burl\(\s*["']?(https?:)?//`),
	regexp.MustCompile("\\bWebSocket\\s*\\(\\s*[\"'`]wss?://"),
}

func usesNetwork(line string) bool {
	for _, rule := range networkRules {
		if rule.MatchString(line) {
			return true
		}
	}
	return false
}

// markNetworkUse marks a file's network instructions: a fenced block holding
// one gets the not-for-this-stick info string, a line outside fences the
// prefix "Not for this drive: ".
func markNetworkUse(text string) string {
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimLeft(lines[i], " ")
		marker := fenceMarker(trimmed)
		if marker == "" {
			if usesNetwork(lines[i]) {
				lines[i] = "Not for this drive: " + lines[i]
			}
			continue
		}
		end := len(lines)
		network := false
		for j := i + 1; j < len(lines); j++ {
			t := strings.TrimLeft(lines[j], " ")
			if strings.HasPrefix(t, marker) && strings.Trim(t, marker[:1]+" \t") == "" {
				end = j
				break
			}
			network = network || usesNetwork(lines[j])
		}
		if network {
			indent := lines[i][:len(lines[i])-len(trimmed)]
			lines[i] = indent + marker + "not-for-this-stick"
		}
		i = end
	}
	return strings.Join(lines, "\n")
}

// markNetworkLines prefixes each line of code that uses the network, for
// type definitions whose comments show examples with URLs.
func markNetworkLines(code string) string {
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		if usesNetwork(line) {
			lines[i] = "Not for this drive: " + line
		}
	}
	return strings.Join(lines, "\n")
}

// fenceMarker returns the run of backticks or tildes that opens a fence.
func fenceMarker(line string) string {
	for _, c := range []string{"`", "~"} {
		if n := len(line) - len(strings.TrimLeft(line, c)); n >= 3 {
			return strings.Repeat(c, n)
		}
	}
	return ""
}

var anchorRe = regexp.MustCompile(`\s*\{#[^}]*\}\s*$`)

var (
	frontmatterRe = regexp.MustCompile(`(?s)\A---\n.*?\n---\n`)
	scriptRe      = regexp.MustCompile(`(?s)<script[^>]*>.*?</script>`)
	styleRe       = regexp.MustCompile(`(?s)<style[^>]*>.*?</style>`)
	commentRe     = regexp.MustCompile(`(?s)<!--.*?-->`)
)

// appendMarkdown adds one file as sections: its text before the first "##"
// under "## <title>", each "## S" as "## <title>: S", deeper headings one
// level lower than "##" stays. Headings inside code fences are left alone.
func appendMarkdown(b *strings.Builder, fallback, text string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = frontmatterRe.ReplaceAllString(text, "")
	text = scriptRe.ReplaceAllString(text, "")
	text = styleRe.ReplaceAllString(text, "")
	text = commentRe.ReplaceAllString(text, "")
	text = markNetworkUse(text)
	lines := strings.Split(text, "\n")
	title := fallback
	start := 0
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if t, ok := strings.CutPrefix(line, "# "); ok {
			title = anchorRe.ReplaceAllString(strings.TrimSpace(t), "")
			start = i + 1
		}
		break
	}
	fmt.Fprintf(b, "## %s\n\n", title)
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	fence := ""
	for _, line := range lines[start:] {
		trimmed := strings.TrimLeft(line, " ")
		if fence == "" && (strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~")) {
			fence = trimmed[:3]
		} else if fence != "" && strings.HasPrefix(trimmed, fence) {
			fence = ""
		} else if fence == "" {
			if h, ok := strings.CutPrefix(line, "## "); ok {
				line = "## " + title + ": " + anchorRe.ReplaceAllString(strings.TrimSpace(h), "")
			} else if h, ok := strings.CutPrefix(line, "# "); ok {
				line = "## " + title + ": " + anchorRe.ReplaceAllString(strings.TrimSpace(h), "")
			} else if strings.HasPrefix(line, "#") {
				line = anchorRe.ReplaceAllString(line, "")
			}
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	b.WriteString("\n")
}

func tarballFiles(archive string) (map[string]string, error) {
	f, err := os.Open(archive)
	if err != nil {
		return nil, fmt.Errorf("%w (run js-lock first: it fills the cache)", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(gz)
	files := map[string]string{}
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := strings.TrimPrefix(path.Clean("/"+hdr.Name), "/")
		_, rel, ok := strings.Cut(name, "/")
		if !ok {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return nil, err
		}
		files[rel] = string(data)
	}
}

func (g *generator) get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "svalbard-js-docs")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" && strings.HasPrefix(url, g.github) {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// repoMarkdown lists the repository's Markdown files at ref that match an
// include pattern and no exclude pattern, sorted.
func (g *generator) repoMarkdown(ctx context.Context, repo, ref string, include, exclude []string) ([]string, error) {
	data, err := g.get(ctx, fmt.Sprintf("%s/repos/%s/git/trees/%s?recursive=1", g.github, repo, ref))
	if err != nil {
		return nil, err
	}
	var tree struct {
		Tree []struct {
			Path, Type string
		}
		Truncated bool
	}
	if err := json.Unmarshal(data, &tree); err != nil {
		return nil, err
	}
	if tree.Truncated {
		return nil, fmt.Errorf("the tree of %s at %s is too large to list in one request", repo, ref)
	}
	var paths []string
	for _, e := range tree.Tree {
		if e.Type != "blob" || !strings.HasSuffix(e.Path, ".md") {
			continue
		}
		matches := func(patterns []string) bool {
			for _, p := range patterns {
				if matchGlob(p, e.Path) {
					return true
				}
			}
			return false
		}
		if matches(include) && !matches(exclude) {
			paths = append(paths, e.Path)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func (g *generator) rawFile(ctx context.Context, repo, ref, p string) (string, error) {
	data, err := g.get(ctx, fmt.Sprintf("%s/%s/%s/%s", g.raw, repo, ref, p))
	return string(data), err
}

// matchGlob matches a slash path against a pattern where "**" is any number
// of segments and other segments use path.Match.
func matchGlob(pattern, name string) bool {
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchSegments(ps, ns []string) bool {
	for len(ps) > 0 {
		if ps[0] == "**" {
			for i := 0; i <= len(ns); i++ {
				if matchSegments(ps[1:], ns[i:]) {
					return true
				}
			}
			return false
		}
		if len(ns) == 0 {
			return false
		}
		if ok, _ := path.Match(ps[0], ns[0]); !ok {
			return false
		}
		ps, ns = ps[1:], ns[1:]
	}
	return len(ns) == 0
}

func field(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func value(m *yaml.Node, key string) string {
	if n := field(m, key); n != nil {
		return n.Value
	}
	return ""
}

func setField(m *yaml.Node, key, v string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = &yaml.Node{Kind: yaml.ScalarNode, Value: v}
			return
		}
	}
	m.Content = append(m.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, &yaml.Node{Kind: yaml.ScalarNode, Value: v})
}
