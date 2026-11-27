// Package htmlpack turns a page an agent wrote, and the drive's JS libraries
// it imports, into one HTML file that opens by double-click without a
// network, or into a folder for the drive's server when a library cannot
// work from one file. esbuild bundles each module script; bare imports
// resolve inside the drive's library archives (lib/js/<recipe>.zip), read
// in place; imported data files, stylesheets, fonts and images go into the
// page. The result is checked before it is written: nothing may load from
// the network, fetch a file beside it, or point at a file that is not
// there.
package htmlpack

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/evanw/esbuild/pkg/api"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Options says what to pack.
type Options struct {
	// DriveRoot is the drive whose libraries pages import.
	DriveRoot string
	// Page is the HTML file to pack.
	Page string
	// Out is the file to write, or with Dir the folder; empty means beside
	// the page: <name>.packed.html or <name>-packed/.
	Out string
	// Dir writes a folder (index.html and its files) instead of one file,
	// for pages that need the drive's server.
	Dir bool
}

// Result describes what was written.
type Result struct {
	Out         string
	Bytes       int64
	Portability string // "single-file" or "directory"
	Notices     []string
	Warnings    []string
}

// Problem is one reason a packed page would not work offline.
type Problem struct {
	Kind   string // remote-resource, relative-fetch, missing-asset, served-library, build
	Detail string
	// Target is the page-relative file a relative fetch or a missing asset
	// names, when there is one; a folder output copies it.
	Target string
}

func (p Problem) String() string { return p.Kind + ": " + p.Detail }

// ProblemsError is returned when the page cannot be packed as asked; nothing
// is written.
type ProblemsError struct{ Problems []Problem }

func (e *ProblemsError) Error() string {
	lines := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		lines[i] = p.String()
	}
	return strings.Join(lines, "\n")
}

// warnBytes is the size above which a single file is called large.
const warnBytes = 10 << 20

// relativeFetchMessage is the problem a fetch of a file beside the page gets
// in single-file mode.
const relativeFetchMessage = "relative fetch cannot work from file://; import the file instead"

// packedAttr marks a script html pack wrote: library code, or a page's
// bundle. Packing such a page again does not scan it as the page's own code
// (library code holds addresses that load nothing).
const packedAttr = "data-svalbard-packed"

const (
	libNamespace   = "svalbard-js"
	emptyNamespace = "svalbard-empty"
	// pageNamespace holds the page's inline module scripts, which one
	// bundle imports in their order.
	pageNamespace = "svalbard-page"
)

type packer struct {
	opts    Options
	pageDir string
	libs    *libraries
	// used records the library packages bundled: archive id and package
	// folder in it.
	used map[string]usedPackage
	// mu guards used and problems: esbuild calls the plugin from several
	// goroutines.
	mu sync.Mutex
	// inline holds the inline module scripts by their virtual path.
	inline   map[string]string
	problems []Problem
	notices  []string
	warnings []string
	// files are the extra files of a folder output, by path in it.
	files map[string][]byte
}

type usedPackage struct {
	archive *archive
	dir     string
}

// Pack packs opts.Page. When the page cannot work as asked it returns a
// *ProblemsError and writes nothing.
func Pack(ctx context.Context, opts Options) (Result, error) {
	page, err := filepath.Abs(opts.Page)
	if err != nil {
		return Result{}, err
	}
	source, err := os.ReadFile(page)
	if err != nil {
		return Result{}, err
	}
	opts.Page = page
	if opts.Out == "" {
		base := strings.TrimSuffix(filepath.Base(page), filepath.Ext(page))
		if opts.Dir {
			opts.Out = filepath.Join(filepath.Dir(page), base+"-packed")
		} else {
			opts.Out = filepath.Join(filepath.Dir(page), base+".packed.html")
		}
	}
	out, err := filepath.Abs(opts.Out)
	if err != nil {
		return Result{}, err
	}
	opts.Out = out
	if out == page {
		return Result{}, fmt.Errorf("the output would replace the page itself; name another with -o")
	}
	if opts.Dir {
		if err := checkOutputFolder(out, page); err != nil {
			return Result{}, err
		}
	}
	libs, err := loadLibraries(opts.DriveRoot)
	if err != nil {
		return Result{}, err
	}
	defer libs.close()
	p := &packer{opts: opts, pageDir: filepath.Dir(page), libs: libs, used: map[string]usedPackage{}, files: map[string][]byte{}, inline: map[string]string{}, warnings: libs.warnings}

	doc, err := html.Parse(bytes.NewReader(source))
	if err != nil {
		return Result{}, fmt.Errorf("reading %s: %w", filepath.Base(page), err)
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	p.processDocument(doc)
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	p.checkPortability()

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return Result{}, err
	}
	notice, unlicensed := p.licenseNotice()
	if bundled := p.libraryNames(); len(bundled) > 0 {
		p.notices = append(p.notices, "bundled: "+strings.Join(bundled, ", "))
	}
	if notice != "" {
		buf.WriteString("\n<!-- " + notice + "\n-->\n")
	}
	for _, name := range unlicensed {
		p.warnings = append(p.warnings, name+" has no licence text in the drive's archive; the page names its licence only")
	}
	output := buf.Bytes()
	p.problems = append(p.problems, Check(output)...)
	if opts.Dir {
		if js, ok := p.files["script.js"]; ok {
			p.problems = append(p.problems, CheckScript("script.js", string(js))...)
		}
		p.problems = p.copyTargets(p.problems)
	}
	if len(p.problems) > 0 {
		return Result{}, &ProblemsError{Problems: dedupe(p.problems)}
	}
	return p.write(output)
}

// packMarker marks a folder html pack wrote, which a later pack may replace.
const packMarker = ".svalbard-pack"

// checkOutputFolder refuses a folder output that would hold the page or
// replace files that are not a previous pack's.
func checkOutputFolder(out, page string) error {
	if rel, err := filepath.Rel(out, page); err == nil && !strings.HasPrefix(rel, "..") {
		return fmt.Errorf("the output folder %s holds the page itself; name another with -o", out)
	}
	entries, err := os.ReadDir(out)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	if _, err := os.Stat(filepath.Join(out, packMarker)); err != nil {
		return fmt.Errorf("the output folder %s already holds files that html pack did not write; name an empty or new folder with -o", out)
	}
	return nil
}

// copyTargets copies into a folder output the files beside the page that
// its relative fetches and references name, and returns the problems left:
// what is not there, outside the page's folder, or has no file name.
func (p *packer) copyTargets(problems []Problem) []Problem {
	var left []Problem
	for _, problem := range problems {
		if (problem.Kind != "relative-fetch" && problem.Kind != "missing-asset") || problem.Target == "" {
			left = append(left, problem)
			continue
		}
		if own := strings.TrimPrefix(path.Clean(strings.SplitN(problem.Target, "?", 2)[0]), "./"); p.files[own] != nil {
			continue // a file of the output itself
		}
		file := p.localFile(problem.Target)
		rel, err := filepath.Rel(p.pageDir, file)
		data, readErr := os.ReadFile(file)
		if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) || readErr != nil {
			left = append(left, Problem{Kind: "missing-asset", Detail: fmt.Sprintf("%s names %s, which is not a file in the page's folder", problem.Detail, problem.Target)})
			continue
		}
		p.files[filepath.ToSlash(rel)] = data
	}
	return left
}

func dedupe(problems []Problem) []Problem {
	seen := map[Problem]bool{}
	var out []Problem
	for _, p := range problems {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func (p *packer) write(page []byte) (Result, error) {
	result := Result{Out: p.opts.Out, Notices: p.notices, Warnings: p.warnings}
	if !p.opts.Dir {
		if err := writeFile(p.opts.Out, page); err != nil {
			return Result{}, err
		}
		result.Bytes, result.Portability = int64(len(page)), "single-file"
		if result.Bytes > warnBytes {
			result.Warnings = append(result.Warnings, fmt.Sprintf("the page is %s: it opens, but slowly on a small computer; leave out what it does not need", humanBytes(result.Bytes)))
		}
		return result, nil
	}
	// Written whole into a new folder beside the output, then swapped in:
	// files of an earlier pack do not linger and nothing else is touched.
	if err := os.MkdirAll(filepath.Dir(p.opts.Out), 0o755); err != nil {
		return Result{}, err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(p.opts.Out), "."+filepath.Base(p.opts.Out)+".packing-")
	if err != nil {
		return Result{}, err
	}
	defer os.RemoveAll(tmp)
	p.files["index.html"] = page
	p.files[packMarker] = []byte("written by svalbard-drive html pack; a later pack replaces this folder\n")
	names := make([]string, 0, len(p.files))
	for name := range p.files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := writeFile(filepath.Join(tmp, filepath.FromSlash(name)), p.files[name]); err != nil {
			return Result{}, err
		}
		if name != packMarker {
			result.Bytes += int64(len(p.files[name]))
		}
	}
	if err := checkOutputFolder(p.opts.Out, p.opts.Page); err != nil {
		return Result{}, err
	}
	if err := os.RemoveAll(p.opts.Out); err != nil {
		return Result{}, err
	}
	if err := os.Rename(tmp, p.opts.Out); err != nil {
		return Result{}, err
	}
	result.Portability = "directory"
	return result, nil
}

func writeFile(target string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp := target + ".packing"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

func humanBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d bytes", n)
}

func (p *packer) problem(kind, format string, args ...any) {
	p.addProblems(Problem{Kind: kind, Detail: fmt.Sprintf(format, args...)})
}

func (p *packer) addProblems(problems ...Problem) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.problems = append(p.problems, problems...)
}

// ---------------------------------------------------------------------------
// The document
// ---------------------------------------------------------------------------

func attr(n *html.Node, key string) (string, bool) {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val, true
		}
	}
	return "", false
}

func setAttr(n *html.Node, key, val string) {
	for i, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}

func removeAttr(n *html.Node, key string) {
	attrs := n.Attr[:0]
	for _, a := range n.Attr {
		if !strings.EqualFold(a.Key, key) {
			attrs = append(attrs, a)
		}
	}
	n.Attr = attrs
}

func textOf(n *html.Node) string {
	var b strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.TextNode {
			b.WriteString(c.Data)
		}
	}
	return b.String()
}

func setText(n *html.Node, text string) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		n.RemoveChild(c)
		c = next
	}
	n.AppendChild(&html.Node{Type: html.TextNode, Data: text})
}

// isRemote reports whether a reference loads from the network.
func isRemote(ref string) bool {
	ref = strings.ToLower(strings.TrimSpace(ref))
	return strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "//")
}

// isInline reports whether a reference needs no file: data, blob, a
// fragment, javascript: or empty.
func isInline(ref string) bool {
	ref = strings.ToLower(strings.TrimSpace(ref))
	return ref == "" || strings.HasPrefix(ref, "data:") || strings.HasPrefix(ref, "blob:") || strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "javascript:") || strings.HasPrefix(ref, "about:")
}

// localFile returns the file a page-relative reference names, without its
// query or fragment.
func (p *packer) localFile(ref string) string {
	ref, _, _ = strings.Cut(ref, "#")
	ref, _, _ = strings.Cut(ref, "?")
	if strings.HasPrefix(ref, "file://") {
		return strings.TrimPrefix(ref, "file://")
	}
	if filepath.IsAbs(ref) {
		return ref
	}
	return filepath.Join(p.pageDir, filepath.FromSlash(ref))
}

func (p *packer) processDocument(doc *html.Node) {
	var scripts, links, styles, assets []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Script:
				scripts = append(scripts, n)
			case atom.Link:
				links = append(links, n)
			case atom.Style:
				styles = append(styles, n)
			case atom.Img, atom.Source, atom.Audio, atom.Video, atom.Input, atom.Track:
				assets = append(assets, n)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	var collected []string // stylesheets imported by modules
	// Module scripts are deferred and run in order, so one bundle that
	// imports each of them in that order behaves the same, with libraries
	// and shared modules once.
	var modules []*html.Node
	var entries []string
	for _, n := range scripts {
		kind, _ := attr(n, "type")
		src, hasSrc := attr(n, "src")
		switch {
		case strings.EqualFold(strings.TrimSpace(kind), "module"):
			if entry, ok := p.moduleEntry(n, src, hasSrc, len(modules)+1); ok {
				modules = append(modules, n)
				entries = append(entries, entry)
			}
		case hasSrc && !isRemote(src) && !isInline(src):
			p.inlineClassicScript(n, src)
		case !hasSrc && (kind == "" || strings.Contains(strings.ToLower(kind), "javascript")):
			if _, packed := attr(n, packedAttr); !packed {
				p.addProblems(scanOwnSource(filepath.Base(p.opts.Page)+" (script)", textOf(n))...)
			}
			setText(n, stripSourceMaps(textOf(n)))
		}
	}
	if len(modules) > 0 {
		var lines []string
		for _, entry := range entries {
			lines = append(lines, fmt.Sprintf("import %q;", entry))
		}
		if js, css, ok := p.bundleModule(strings.Join(lines, "\n"), p.pageDir, filepath.Base(p.opts.Page)); ok {
			if css != "" {
				collected = append(collected, css)
			}
			first := modules[0]
			setAttr(first, packedAttr, "")
			if p.opts.Dir {
				p.files["script.js"] = []byte(js)
				setAttr(first, "src", "./script.js")
				setText(first, "")
			} else {
				removeAttr(first, "src")
				setScript(first, js)
			}
			for _, n := range modules[1:] {
				n.Parent.RemoveChild(n)
			}
		}
	}
	for _, n := range links {
		rel, _ := attr(n, "rel")
		href, _ := attr(n, "href")
		rels := strings.Fields(strings.ToLower(rel))
		switch {
		case contains(rels, "stylesheet") && !isRemote(href) && !isInline(href):
			css, ok := p.bundleCSS(fmt.Sprintf("@import %q;", href), "link href="+href)
			if !ok {
				continue
			}
			style := &html.Node{Type: html.ElementNode, Data: "style", DataAtom: atom.Style}
			style.AppendChild(&html.Node{Type: html.TextNode, Data: escapeClosing(css, "style")})
			n.Parent.InsertBefore(style, n)
			n.Parent.RemoveChild(n)
		case (contains(rels, "icon") || contains(rels, "apple-touch-icon")) && !isRemote(href) && !isInline(href):
			if ref, ok := p.embedAsset(href); ok {
				setAttr(n, "href", ref)
			}
		}
	}
	for _, n := range styles {
		css, ok := p.bundleCSS(textOf(n), "a <style> block")
		if ok {
			setText(n, escapeClosing(css, "style"))
		}
	}
	for _, n := range assets {
		for _, key := range []string{"src", "poster"} {
			ref, ok := attr(n, key)
			if !ok || isRemote(ref) || isInline(ref) {
				continue
			}
			if embedded, ok := p.embedAsset(ref); ok {
				setAttr(n, key, embedded)
			}
		}
	}
	if len(collected) > 0 {
		p.insertStyles(doc, strings.Join(collected, "\n"))
	}
}

func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

var closingRe = map[string]*regexp.Regexp{
	"script": regexp.MustCompile(`(?i)</(script)`),
	"style":  regexp.MustCompile(`(?i)</(style)`),
}

// escapeClosing keeps "</script" (or "</style") in code from ending the
// element early; "<\/script" means the same in a string, template, regular
// expression or comment.
func escapeClosing(code, tag string) string {
	return closingRe[tag].ReplaceAllString(code, `<\/$1`)
}

var scriptOpenRe = regexp.MustCompile(`(?i)<script[\s/>]`)

// setScript puts code into a script element. Code holding "<!--" and then
// "<script" would put the HTML parser into a state where the closing tag no
// longer ends the script, and no escape is safe in every JS context; such
// code goes in as a data URL instead, which file:// pages may load.
func setScript(n *html.Node, code string) {
	if strings.Contains(code, "<!--") && scriptOpenRe.MatchString(code) {
		setAttr(n, "src", "data:text/javascript;base64,"+base64.StdEncoding.EncodeToString([]byte(code)))
		setText(n, "")
		return
	}
	setText(n, escapeClosing(code, "script"))
}

// moduleEntry returns what the page's bundle imports for a module script:
// an inline script's virtual path, or a script file's path.
func (p *packer) moduleEntry(n *html.Node, src string, hasSrc bool, index int) (string, bool) {
	if !hasSrc {
		name := fmt.Sprintf("%s:%s (module script %d)", pageNamespace, filepath.Base(p.opts.Page), index)
		code := textOf(n)
		p.inline[name] = code
		if _, packed := attr(n, packedAttr); !packed {
			p.addProblems(scanOwnSource(strings.TrimPrefix(name, pageNamespace+":"), code)...)
		}
		return name, true
	}
	if isRemote(src) {
		p.problem("remote-resource", "<script type=module src=%q> loads from the network; import the library by name instead (skill_search kind=library lists them)", src)
		return "", false
	}
	file := p.localFile(src)
	if _, err := os.Stat(file); err != nil {
		p.problem("missing-asset", "<script type=module src=%q>: %s is not there", src, file)
		return "", false
	}
	return file, true // scanned when esbuild loads it
}

// inlineClassicScript puts a classic script's file into the page: a file
// beside the page, or a library's build named by its package (<script
// src="mermaid">), which sets its global as it would from a CDN.
func (p *packer) inlineClassicScript(n *html.Node, src string) {
	var data []byte
	var name string
	// A file beside the page wins: "app.js" names a file before a package.
	_, statErr := os.Stat(p.localFile(src))
	if isBare(src) && statErr != nil {
		pkg, sub := splitSpecifier(src)
		a, err := p.libs.owner(pkg)
		if err != nil {
			p.problem("missing-asset", "<script src=%q>: %v", src, err)
			return
		}
		dir := "node_modules/" + pkg
		file, err := a.resolvePackage(dir, sub, true)
		if err == nil {
			data, err = a.read(file)
		}
		if err != nil {
			p.problem("missing-asset", "<script src=%q>: %v", src, err)
			return
		}
		p.markUsed(a, dir)
		name = path.Join("lib", a.id, file)
		setAttr(n, packedAttr, "")
	} else {
		file := p.localFile(src)
		var err error
		if data, err = os.ReadFile(file); err != nil {
			p.problem("missing-asset", "<script src=%q>: %s is not there", src, file)
			return
		}
		rel, err := filepath.Rel(p.pageDir, file)
		if err != nil || strings.HasPrefix(rel, "..") {
			rel = filepath.Base(file)
		}
		name = filepath.ToSlash(rel)
		p.addProblems(scanOwnSource(name, string(data))...)
	}
	data = []byte(stripSourceMaps(string(data)))
	if p.opts.Dir {
		p.files[name] = data
		setAttr(n, "src", "./"+name)
		return
	}
	removeAttr(n, "src")
	setScript(n, string(data))
}

// embedAsset turns a file beside the page into a data URL, or in a folder
// output copies it there.
func (p *packer) embedAsset(ref string) (string, bool) {
	file := p.localFile(ref)
	data, err := os.ReadFile(file)
	if err != nil {
		p.problem("missing-asset", "%s is referenced but not there (%s)", ref, file)
		return "", false
	}
	if p.opts.Dir {
		rel, err := filepath.Rel(p.pageDir, file)
		if err != nil || strings.HasPrefix(rel, "..") {
			rel = path.Join("assets", filepath.Base(file))
		}
		p.files[filepath.ToSlash(rel)] = data
		return "./" + filepath.ToSlash(rel), true
	}
	kind := mime.TypeByExtension(strings.ToLower(filepath.Ext(file)))
	if kind == "" {
		kind = "application/octet-stream"
	}
	kind, _, _ = strings.Cut(kind, ";")
	return "data:" + kind + ";base64," + base64.StdEncoding.EncodeToString(data), true
}

// insertStyles adds a stylesheet to the head, before its first script.
func (p *packer) insertStyles(doc *html.Node, css string) {
	var head *html.Node
	var find func(*html.Node)
	find = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Head && head == nil {
			head = n
		}
		for c := n.FirstChild; c != nil && head == nil; c = c.NextSibling {
			find(c)
		}
	}
	find(doc)
	if head == nil {
		return // html.Parse always makes one
	}
	style := &html.Node{Type: html.ElementNode, Data: "style", DataAtom: atom.Style}
	if p.opts.Dir {
		p.files["styles.css"] = []byte(css)
		style = &html.Node{Type: html.ElementNode, Data: "link", DataAtom: atom.Link, Attr: []html.Attribute{{Key: "rel", Val: "stylesheet"}, {Key: "href", Val: "./styles.css"}}}
	} else {
		style.AppendChild(&html.Node{Type: html.TextNode, Data: escapeClosing(css, "style")})
	}
	head.AppendChild(style)
}

// ---------------------------------------------------------------------------
// Bundling
// ---------------------------------------------------------------------------

func (p *packer) loaders() map[string]api.Loader {
	asset := api.LoaderDataURL
	binary := api.LoaderBinary
	if p.opts.Dir {
		asset, binary = api.LoaderFile, api.LoaderFile
	}
	return map[string]api.Loader{
		".js": api.LoaderJS, ".mjs": api.LoaderJS, ".cjs": api.LoaderJS, ".jsx": api.LoaderJSX, ".ts": api.LoaderTS,
		".json": api.LoaderJSON, ".geojson": api.LoaderJSON, ".topojson": api.LoaderJSON,
		".css": api.LoaderCSS,
		".csv": api.LoaderText, ".tsv": api.LoaderText, ".txt": api.LoaderText, ".md": api.LoaderText, ".xml": api.LoaderText, ".gpx": api.LoaderText, ".kml": api.LoaderText,
		".woff2": asset, ".woff": asset, ".ttf": asset, ".otf": asset, ".eot": asset,
		".png": asset, ".jpg": asset, ".jpeg": asset, ".gif": asset, ".svg": asset, ".webp": asset, ".avif": asset, ".ico": asset,
		".mp3": asset, ".wav": asset, ".ogg": asset, ".mp4": asset, ".webm": asset,
		".wasm": binary, ".bin": binary, ".stl": binary,
	}
}

func (p *packer) buildOptions() api.BuildOptions {
	return api.BuildOptions{
		Bundle:            true,
		Write:             false,
		Platform:          api.PlatformBrowser,
		MinifyWhitespace:  true,
		MinifySyntax:      true,
		MinifyIdentifiers: true,
		LegalComments:     api.LegalCommentsEndOfFile,
		Charset:           api.CharsetUTF8,
		AbsWorkingDir:     p.pageDir,
		Outdir:            filepath.Join(p.pageDir, ".svalbard-pack"),
		AssetNames:        "assets/[name]-[hash]",
		Loader:            p.loaders(),
		Plugins:           []api.Plugin{p.plugin()},
		LogLevel:          api.LogLevelSilent,
	}
}

// buildErrors turns esbuild's messages into problems.
func (p *packer) buildErrors(where string, messages []api.Message) {
	for _, m := range messages {
		text := m.Text
		if m.Location != nil {
			file := m.Location.File
			if strings.Contains(file, "(module script") || strings.HasPrefix(file, "<stdin>") {
				file = "line" // the script itself, which where names
			} else if id, inner, ok := strings.Cut(strings.TrimPrefix(file, libNamespace+":"), ":"); ok {
				file = id + " " + inner
			} else {
				file = filepath.Base(file)
			}
			text = fmt.Sprintf("%s %d: %s", file, m.Location.Line, text)
		}
		kind := "build"
		if strings.Contains(m.Text, "network") {
			kind = "remote-resource"
		}
		p.problem(kind, "%s: %s", where, text)
	}
}

// bundleModule bundles one module script and returns its JS and the CSS its
// imports brought.
func (p *packer) bundleModule(code, resolveDir, name string) (js, css string, ok bool) {
	opts := p.buildOptions()
	opts.Stdin = &api.StdinOptions{Contents: code, ResolveDir: resolveDir, Sourcefile: name, Loader: api.LoaderJS}
	opts.Format = api.FormatESModule
	result := api.Build(opts)
	if len(result.Errors) > 0 {
		p.buildErrors(name, result.Errors)
		return "", "", false
	}
	for _, f := range result.OutputFiles {
		rel, _ := filepath.Rel(opts.Outdir, f.Path)
		switch {
		case strings.HasSuffix(f.Path, ".js"):
			js += string(f.Contents)
		case strings.HasSuffix(f.Path, ".css"):
			css += string(f.Contents)
		default:
			p.files[filepath.ToSlash(rel)] = f.Contents
		}
	}
	return js, css, true
}

// bundleCSS bundles a stylesheet: its @imports, and the fonts and images its
// url()s name, from beside the page or from a library.
func (p *packer) bundleCSS(css, where string) (string, bool) {
	opts := p.buildOptions()
	opts.Stdin = &api.StdinOptions{Contents: css, ResolveDir: p.pageDir, Sourcefile: where, Loader: api.LoaderCSS}
	result := api.Build(opts)
	if len(result.Errors) > 0 {
		p.buildErrors(where, result.Errors)
		return "", false
	}
	var out string
	for _, f := range result.OutputFiles {
		rel, _ := filepath.Rel(opts.Outdir, f.Path)
		if strings.HasSuffix(f.Path, ".css") {
			out += string(f.Contents)
		} else {
			p.files[filepath.ToSlash(rel)] = f.Contents
		}
	}
	return out, true
}

// plugin resolves imports to the drive's library archives and loads files
// from inside them.
func (p *packer) plugin() api.Plugin {
	return api.Plugin{Name: "svalbard-libraries", Setup: func(build api.PluginBuild) {
		build.OnResolve(api.OnResolveOptions{Filter: `^(https?:)?//`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			return api.OnResolveResult{}, fmt.Errorf("%s loads from the network, which a drive does not have; import the library by name instead (skill_search kind=library lists them)", args.Path)
		})
		build.OnResolve(api.OnResolveOptions{Filter: `.*`, Namespace: libNamespace}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			return p.resolveInArchive(args)
		})
		build.OnResolve(api.OnResolveOptions{Filter: "^" + pageNamespace + ":"}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			return api.OnResolveResult{Path: args.Path, Namespace: pageNamespace}, nil
		})
		build.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: pageNamespace}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
			code := p.inline[args.Path]
			return api.OnLoadResult{Contents: &code, ResolveDir: p.pageDir, Loader: api.LoaderJS}, nil
		})
		// The page's own script files are scanned as esbuild reads them; it
		// loads them itself.
		build.OnLoad(api.OnLoadOptions{Filter: `\.(m?js|cjs|jsx|tsx?)$`, Namespace: "file"}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
			if data, err := os.ReadFile(args.Path); err == nil {
				p.addProblems(scanOwnSource(filepath.Base(args.Path), string(data))...)
			}
			return api.OnLoadResult{}, nil
		})
		build.OnResolve(api.OnResolveOptions{Filter: `^[^./]`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			if !isBare(args.Path) {
				return api.OnResolveResult{}, nil
			}
			if args.Kind == api.ResolveCSSURLToken || args.Kind == api.ResolveCSSImportRule {
				// url(images/x.png) is a file beside the page when there is one.
				clean, _, _ := strings.Cut(args.Path, "?")
				if _, err := os.Stat(filepath.Join(args.ResolveDir, filepath.FromSlash(clean))); err == nil {
					return api.OnResolveResult{}, nil
				}
			}
			pkg, sub := splitSpecifier(args.Path)
			a, err := p.libs.owner(pkg)
			if err != nil {
				return api.OnResolveResult{}, err
			}
			return p.resolved(a, func() (string, error) {
				return a.resolvePackage("node_modules/"+pkg, sub, args.Kind == api.ResolveJSRequireCall)
			})
		})
		build.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: libNamespace}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
			return p.load(args.Path)
		})
		build.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: emptyNamespace}, func(api.OnLoadArgs) (api.OnLoadResult, error) {
			empty := ""
			return api.OnLoadResult{Contents: &empty, Loader: api.LoaderJS}, nil
		})
	}}
}

// resolved wraps a resolution inside an archive as a plugin result.
func (p *packer) resolved(a *archive, resolve func() (string, error)) (api.OnResolveResult, error) {
	file, err := resolve()
	if errors.Is(err, errEmptyModule) {
		return api.OnResolveResult{Path: a.id + ":empty", Namespace: emptyNamespace}, nil
	}
	if err != nil {
		return api.OnResolveResult{}, err
	}
	return api.OnResolveResult{Path: a.id + ":" + file, Namespace: libNamespace}, nil
}

func (p *packer) archiveByID(id string) *archive {
	for _, a := range p.libs.archives {
		if a.id == id {
			return a
		}
	}
	return nil
}

// resolveInArchive resolves an import made by a file inside an archive.
func (p *packer) resolveInArchive(args api.OnResolveArgs) (api.OnResolveResult, error) {
	id, importer, _ := strings.Cut(args.Importer, ":")
	a := p.archiveByID(id)
	if a == nil {
		return api.OnResolveResult{}, fmt.Errorf("no library archive %s", id)
	}
	spec, _, _ := strings.Cut(args.Path, "?")
	spec, _, _ = strings.Cut(spec, "#")
	if strings.HasPrefix(strings.ToLower(spec), "data:") {
		return api.OnResolveResult{Path: args.Path, External: true}, nil
	}
	from := path.Dir(importer)
	if isBare(spec) && !(args.Kind == api.ResolveCSSURLToken && a.isFile(path.Join(from, spec))) {
		return p.resolved(a, func() (string, error) {
			return a.resolveBare(from, spec, args.Kind == api.ResolveJSRequireCall)
		})
	}
	target := path.Join(from, spec)
	return p.resolved(a, func() (string, error) {
		return a.resolveFile(packageDirOf(target), target)
	})
}

// packageDirOf returns the package folder a file in an archive belongs to.
func packageDirOf(file string) string {
	at := strings.LastIndex(file, "node_modules/")
	if at < 0 {
		return ""
	}
	rest := file[at+len("node_modules/"):]
	parts := strings.Split(rest, "/")
	n := 1
	if strings.HasPrefix(parts[0], "@") && len(parts) > 1 {
		n = 2
	}
	return file[:at+len("node_modules/")] + strings.Join(parts[:n], "/")
}

// load reads a file from an archive for esbuild and records its package.
func (p *packer) load(virtual string) (api.OnLoadResult, error) {
	id, file, _ := strings.Cut(virtual, ":")
	a := p.archiveByID(id)
	if a == nil {
		return api.OnLoadResult{}, fmt.Errorf("no library archive %s", id)
	}
	data, err := a.read(file)
	if err != nil {
		return api.OnLoadResult{}, fmt.Errorf("%s: %s: %w", id, file, err)
	}
	if dir := packageDirOf(file); dir != "" {
		p.markUsed(a, dir)
	}
	loader, ok := p.loaders()[strings.ToLower(path.Ext(file))]
	if !ok {
		loader = api.LoaderJS
	}
	contents := string(data)
	return api.OnLoadResult{Contents: &contents, Loader: loader}, nil
}

// ---------------------------------------------------------------------------
// Libraries used
// ---------------------------------------------------------------------------

// checkPortability refuses a single file with a library measured to need
// the drive's server or its folder of files.
func (p *packer) checkPortability() {
	if p.opts.Dir {
		return
	}
	for _, u := range p.sortedUsed() {
		pkg := strings.TrimPrefix(u.dir, "node_modules/")
		lib, ok := p.libs.documented[pkg]
		if !ok || lib.archive != u.archive {
			continue
		}
		switch lib.portability {
		case "served", "directory":
			p.problem("served-library", "%s is measured %q: it does not work from a single file opened by double-click. Pack with --dir and open the folder through the drive's server", pkg, lib.portability)
		}
	}
}

// markUsed records a bundled library package.
func (p *packer) markUsed(a *archive, dir string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.used[a.id+":"+dir] = usedPackage{archive: a, dir: dir}
}

func (p *packer) sortedUsed() []usedPackage {
	p.mu.Lock()
	defer p.mu.Unlock()
	keys := make([]string, 0, len(p.used))
	for key := range p.used {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	used := make([]usedPackage, len(keys))
	for i, key := range keys {
		used[i] = p.used[key]
	}
	return used
}

// libraryNames lists the drive's libraries the page bundled, with versions.
func (p *packer) libraryNames() []string {
	var names []string
	for _, u := range p.sortedUsed() {
		pkg := strings.TrimPrefix(u.dir, "node_modules/")
		if lib, ok := p.libs.documented[pkg]; !ok || lib.archive != u.archive {
			continue
		}
		version := ""
		if pj, _ := u.archive.packageJSON(u.dir); pj != nil {
			version = " " + pj.Version
		}
		names = append(names, pkg+version)
	}
	return names
}

// licenseFileRe names the licence and notice files of a package.
var licenseFileRe = regexp.MustCompile(`(?i)^(licen[cs]e|copying|notice)([.-].*)?$`)

// licenseNotice is the trailing comment of a packed page: each bundled
// package with its version and licence, then the licence texts themselves
// (once each), which MIT, BSD and the like require to travel with a copy.
// It returns the packages that have no licence text in their archive.
func (p *packer) licenseNotice() (string, []string) {
	var names, texts, missing []string
	seen := map[string]bool{}
	for _, u := range p.sortedUsed() {
		pj, _ := u.archive.packageJSON(u.dir)
		name, version, license := strings.TrimPrefix(u.dir[strings.LastIndex(u.dir, "node_modules/"):], "node_modules/"), "", ""
		if pj != nil {
			if pj.Name != "" {
				name = pj.Name
			}
			version, license = pj.Version, pj.License
		}
		part := strings.TrimSpace(name + " " + version)
		if license != "" {
			part += " (" + license + ")"
		}
		names = append(names, part)
		var files []string
		for file := range u.archive.files {
			rel, ok := strings.CutPrefix(file, u.dir+"/")
			if !ok {
				continue
			}
			nested := strings.Contains(rel, "node_modules/")
			if (!strings.Contains(rel, "/") && licenseFileRe.MatchString(rel)) || (!nested && strings.HasSuffix(rel, ".LICENSE.txt")) {
				files = append(files, file)
			}
		}
		sort.Strings(files)
		if len(files) == 0 {
			missing = append(missing, name)
		}
		for _, file := range files {
			data, err := u.archive.read(file)
			text := strings.TrimSpace(string(data))
			if err != nil || text == "" || seen[text] {
				continue
			}
			seen[text] = true
			texts = append(texts, fmt.Sprintf("== %s: %s ==\n%s", part, path.Base(file), text))
		}
	}
	if len(names) == 0 {
		return "", nil
	}
	notice := "licenses: " + strings.Join(names, "; ")
	if len(texts) > 0 {
		notice += "\n\nLicence texts of the bundled packages:\n\n" + strings.Join(texts, "\n\n")
	}
	// Nothing in it may end the comment.
	return strings.NewReplacer("-->", "- ->", "--!>", "- -!>").Replace(notice), missing
}
