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

const (
	libNamespace   = "svalbard-js"
	emptyNamespace = "svalbard-empty"
)

type packer struct {
	opts    Options
	pageDir string
	libs    *libraries
	// used records the library packages bundled: archive id and package
	// folder in it.
	used     map[string]usedPackage
	problems []Problem
	notices  []string
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
	if out, err := filepath.Abs(opts.Out); err == nil && out == page {
		return Result{}, fmt.Errorf("the output would replace the page itself; name another with -o")
	}
	libs, err := loadLibraries(opts.DriveRoot)
	if err != nil {
		return Result{}, err
	}
	defer libs.close()
	p := &packer{opts: opts, pageDir: filepath.Dir(page), libs: libs, used: map[string]usedPackage{}, files: map[string][]byte{}}

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
	if notice := p.licenseNotice(); notice != "" {
		buf.WriteString("\n<!-- licenses: " + strings.ReplaceAll(notice, "--", "- -") + " -->\n")
	}
	output := buf.Bytes()
	if !opts.Dir {
		p.problems = append(p.problems, Check(output)...)
	} else {
		for _, problem := range Check(output) {
			if problem.Kind == "remote-resource" {
				p.problems = append(p.problems, problem)
			}
		}
	}
	if len(p.problems) > 0 {
		return Result{}, &ProblemsError{Problems: dedupe(p.problems)}
	}
	return p.write(output)
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
	result := Result{Out: p.opts.Out, Notices: p.notices}
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
	if err := os.MkdirAll(p.opts.Out, 0o755); err != nil {
		return Result{}, err
	}
	p.files["index.html"] = page
	names := make([]string, 0, len(p.files))
	for name := range p.files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := writeFile(filepath.Join(p.opts.Out, filepath.FromSlash(name)), p.files[name]); err != nil {
			return Result{}, err
		}
		result.Bytes += int64(len(p.files[name]))
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
	p.problems = append(p.problems, Problem{Kind: kind, Detail: fmt.Sprintf(format, args...)})
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
	moduleIndex := 0
	for _, n := range scripts {
		kind, _ := attr(n, "type")
		src, hasSrc := attr(n, "src")
		switch {
		case strings.EqualFold(strings.TrimSpace(kind), "module"):
			moduleIndex++
			code, resolveDir, name, ok := p.moduleSource(n, src, hasSrc, moduleIndex)
			if !ok {
				continue
			}
			js, css, ok := p.bundleModule(code, resolveDir, name)
			if !ok {
				continue
			}
			if css != "" {
				collected = append(collected, css)
			}
			if p.opts.Dir {
				file := fmt.Sprintf("script-%d.js", moduleIndex)
				p.files[file] = []byte(js)
				setAttr(n, "src", "./"+file)
				setText(n, "")
			} else {
				removeAttr(n, "src")
				setText(n, escapeClosing(js, "script"))
			}
		case hasSrc && !isRemote(src) && !isInline(src):
			p.inlineClassicScript(n, src)
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

// moduleSource returns a module script's code and where its imports resolve.
func (p *packer) moduleSource(n *html.Node, src string, hasSrc bool, index int) (code, resolveDir, name string, ok bool) {
	name = fmt.Sprintf("%s (module script %d)", filepath.Base(p.opts.Page), index)
	if !hasSrc {
		return textOf(n), p.pageDir, name, true
	}
	if isRemote(src) {
		p.problem("remote-resource", "<script type=module src=%q> loads from the network; import the library by name instead (skill_search kind=library lists them)", src)
		return "", "", "", false
	}
	file := p.localFile(src)
	data, err := os.ReadFile(file)
	if err != nil {
		p.problem("missing-asset", "<script type=module src=%q>: %s is not there", src, file)
		return "", "", "", false
	}
	return string(data), filepath.Dir(file), filepath.Base(file), true
}

// inlineClassicScript puts a classic script's file into the page: a file
// beside the page, or a library's build named by its package (<script
// src="mermaid">), which sets its global as it would from a CDN.
func (p *packer) inlineClassicScript(n *html.Node, src string) {
	var data []byte
	var name string
	if isBare(src) {
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
		p.used[a.id+":"+dir] = usedPackage{archive: a, dir: dir}
		name = path.Join("lib", a.id, file)
	} else {
		file := p.localFile(src)
		var err error
		if data, err = os.ReadFile(file); err != nil {
			if _, statErr := os.Stat(file); statErr != nil && p.libs.byPackage[src] != nil {
				p.problem("missing-asset", "<script src=%q>: no such file beside the page", src)
			} else {
				p.problem("missing-asset", "<script src=%q>: %s is not there", src, file)
			}
			return
		}
		rel, err := filepath.Rel(p.pageDir, file)
		if err != nil || strings.HasPrefix(rel, "..") {
			rel = filepath.Base(file)
		}
		name = filepath.ToSlash(rel)
	}
	if p.opts.Dir {
		p.files[name] = data
		setAttr(n, "src", "./"+name)
		return
	}
	removeAttr(n, "src")
	setText(n, escapeClosing(string(data), "script"))
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
			text = fmt.Sprintf("%s:%d: %s", m.Location.File, m.Location.Line, text)
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
		build.OnResolve(api.OnResolveOptions{Filter: `^[^./]`, Namespace: "file"}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
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
		p.used[id+":"+dir] = usedPackage{archive: a, dir: dir}
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

func (p *packer) sortedUsed() []usedPackage {
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

// licenseNotice names each bundled package with its version and licence.
func (p *packer) licenseNotice() string {
	var parts []string
	for _, u := range p.sortedUsed() {
		pj, _ := u.archive.packageJSON(u.dir)
		name, version, license := strings.TrimPrefix(u.dir[strings.LastIndex(u.dir, "node_modules/"):], "node_modules/"), "", ""
		if pj != nil {
			if pj.Name != "" {
				name = pj.Name
			}
			version, license = pj.Version, pj.License
		}
		if license == "" {
			license = firstLicenseLine(u.archive, u.dir)
		}
		part := strings.TrimSpace(name + " " + version)
		if license != "" {
			part += " (" + license + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

func firstLicenseLine(a *archive, dir string) string {
	for file := range a.files {
		if path.Dir(file) != dir || !strings.HasPrefix(strings.ToUpper(path.Base(file)), "LICEN") {
			continue
		}
		data, err := a.read(file)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				return line
			}
		}
	}
	return ""
}
