package htmlpack

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/workspace"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const workspaceNamespace = "svalbard-workspace"

const (
	maxWorkspaceAssetBytes = 8 << 20
	maxWorkspaceReadBytes  = 16 << 20
	maxWorkspaceZIPBytes   = 16 << 20
	maxWorkspaceOperations = 4096
	maxZIPEntries          = 50_000
	maxLibraryDirEntries   = 512
)

// PackWorkspace packs an HTML file and publishes its sibling .packed.html
// through the opened workspace. DriveRoot is the launcher's installed library
// store. All page-supplied filenames stay within ws.
func PackWorkspace(ctx context.Context, driveRoot string, ws *workspace.Workspace, name string) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	page, err := ws.Resolve(name)
	if err != nil {
		return Result{}, err
	}
	ext := path.Ext(page)
	if !strings.EqualFold(ext, ".html") && !strings.EqualFold(ext, ".htm") {
		return Result{}, fmt.Errorf("pack an HTML source ending in .html or .htm, for example notes/page.html")
	}
	out, err := ws.Resolve(strings.TrimSuffix(page, ext) + ".packed.html")
	if err != nil {
		return Result{}, err
	}
	scope := &workspacePack{ws: ws, limits: &workspaceLimits{ctx: ctx, remaining: maxWorkspaceReadBytes, zipRemaining: maxWorkspaceZIPBytes, operations: maxWorkspaceOperations}}
	source, err := scope.read(page)
	if err != nil {
		return Result{}, err
	}
	if dest, err := ws.Stat(out); err == nil {
		if src, err := ws.Stat(page); err == nil && os.SameFile(src, dest) {
			return Result{}, fmt.Errorf("the packed output would replace the source %s. Use a separate source file", page)
		}
	}
	p, err := newPacker(Options{DriveRoot: driveRoot, Page: page, Out: out}, path.Dir(page), scope)
	if err != nil {
		return Result{}, err
	}
	defer p.libs.close()
	output, err := p.render(ctx, source)
	if err != nil {
		return Result{}, err
	}
	if err := ws.WriteFileContext(ctx, out, output); err != nil {
		return Result{}, err
	}
	return Result{Out: out, Bytes: int64(len(output)), Portability: "single-file", Notices: p.notices, Warnings: p.warnings}, nil
}

type workspacePack struct {
	ws     *workspace.Workspace
	limits *workspaceLimits
}

func (s *workspacePack) reference(dir, ref string) (string, error) {
	name := strings.TrimSpace(ref)
	if i := strings.IndexAny(name, "?#"); i >= 0 {
		name = name[:i]
	}
	name, err := url.PathUnescape(name)
	if err != nil || name == "" || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "~") || strings.ContainsAny(name, "\\:\x00") {
		return "", fmt.Errorf("%q must be a relative workspace path", ref)
	}
	target := path.Join(dir, name)
	if target == ".." || strings.HasPrefix(target, "../") {
		return "", fmt.Errorf("%q points outside the workspace", ref)
	}
	return s.ws.Resolve(target)
}

func (s *workspacePack) read(rel string) ([]byte, error) {
	return s.limits.read(func(maxBytes int64) ([]byte, error) {
		return s.ws.ReadFileContext(s.limits.ctx, rel, maxBytes)
	})
}

type workspaceLimits struct {
	ctx          context.Context
	mu           sync.Mutex
	readMu       sync.Mutex
	remaining    int64
	zipRemaining int64
	operations   int
	err          error
}

func (b *workspaceLimits) failure() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.ctx.Err(); err != nil {
		return err
	}
	return b.err
}

func (b *workspaceLimits) step() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.ctx.Err(); err != nil {
		return err
	}
	if b.operations == 0 && b.err == nil {
		b.err = errors.New("the page has too many dependencies or stylesheets. Pack a smaller page")
	}
	if b.err != nil {
		return b.err
	}
	b.operations--
	return nil
}

func (b *workspaceLimits) read(load func(int64) ([]byte, error)) ([]byte, error) {
	b.readMu.Lock()
	defer b.readMu.Unlock()
	if err := b.step(); err != nil {
		return nil, err
	}
	if b.remaining <= 0 {
		return nil, b.fail(errors.New("dependencies exceed the workspace packing input byte limit"))
	}
	data, err := load(min(b.remaining, maxWorkspaceAssetBytes))
	// A failed read may have consumed bytes before a size or checksum
	// error. Stop further reads except when merely probing a missing file.
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		b.fail(err)
	}
	b.remaining -= int64(len(data))
	return data, err
}

func (b *workspaceLimits) fail(err error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err == nil {
		b.err = err
	}
	return b.err
}

func (b *workspaceLimits) readZIP(source io.ReaderAt, data []byte, off int64) (int, error) {
	b.mu.Lock()
	if err := b.ctx.Err(); err != nil {
		b.mu.Unlock()
		return 0, err
	}
	if b.err == nil && b.zipRemaining < int64(len(data)) {
		b.err = errors.New("installed library archives exceed the workspace packing byte limit")
	}
	if b.err != nil {
		err := b.err
		b.mu.Unlock()
		return 0, err
	}
	b.zipRemaining -= int64(len(data))
	b.mu.Unlock()
	return source.ReadAt(data, off)
}

type workspaceZIPReader struct {
	source io.ReaderAt
	limits *workspaceLimits
}

func (r workspaceZIPReader) ReadAt(data []byte, off int64) (int, error) {
	// Reserve the compressed bytes before releasing the lock. Reading a
	// member already holds the uncompressed budget lock, so use a separate
	// lock for the archive budget.
	return r.limits.readZIP(r.source, data, off)
}

type limitedOutput struct{ buffer *bytes.Buffer }

func (w *limitedOutput) Write(data []byte) (int, error) {
	if len(data) > workspace.MaxWriteBytes-w.buffer.Len() {
		return 0, fmt.Errorf("the packed page exceeds the %d byte workspace limit. Use fewer or smaller assets", workspace.MaxWriteBytes)
	}
	return w.buffer.Write(data)
}

func isPreviewScriptURL(ref string) bool {
	// URL parsing removes these ASCII characters even within a scheme.
	ref = strings.ToLower(strings.TrimSpace(strings.NewReplacer("\t", "", "\r", "", "\n", "").Replace(ref)))
	return strings.HasPrefix(ref, "data:") || strings.HasPrefix(ref, "blob:")
}

func checkPreview(doc *html.Node) []Problem {
	for n := range doc.Descendants() {
		if n.Type == html.ElementNode && n.DataAtom == atom.Script {
			for _, key := range []string{"src", "href"} {
				if ref, ok := attr(n, key); ok && isPreviewScriptURL(ref) {
					return []Problem{{Kind: "workspace-preview", Detail: "<script> data or blob URLs are blocked by the workspace preview. Use an inline script or simplify the script contents"}}
				}
			}
		}
	}
	return nil
}

func previewImport(args api.OnResolveArgs) (api.OnResolveResult, error) {
	if args.Kind == api.ResolveCSSURLToken {
		return api.OnResolveResult{Path: args.Path, External: true}, nil
	}
	return api.OnResolveResult{}, errors.New("imports from data or blob URLs are blocked by the workspace preview. Use a local file")
}

func (p *packer) build(opts api.BuildOptions) api.BuildResult {
	if p.scope == nil {
		return api.Build(opts)
	}
	ctx := p.scope.limits.ctx
	if err := p.scope.limits.step(); err != nil {
		return api.BuildResult{Errors: []api.Message{{Text: err.Error()}}}
	}
	// Pass only virtual modules to the bundler. With no resolve directory,
	// esbuild cannot resolve normal imports against the working directory.
	opts.Stdin.ResolveDir = ""
	build, err := api.Context(opts)
	if err != nil {
		return api.BuildResult{Errors: err.Errors}
	}
	defer build.Dispose()
	stop := context.AfterFunc(ctx, build.Cancel)
	defer stop()
	if err := ctx.Err(); err != nil {
		return api.BuildResult{Errors: []api.Message{{Text: err.Error()}}}
	}
	return build.Rebuild()
}

func (p *packer) setupWorkspacePlugin(build api.PluginBuild) {
	build.OnResolve(api.OnResolveOptions{Filter: `.*`}, p.resolveWorkspace)
	build.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: pageNamespace}, func(args api.OnLoadArgs) (api.OnLoadResult, error) {
		code := p.inline[args.Path]
		if err := checkWorkspaceJS(path.Base(p.opts.Page), code, api.LoaderJS); err != nil {
			return api.OnLoadResult{}, err
		}
		return api.OnLoadResult{Contents: &code, Loader: api.LoaderJS}, nil
	})
	build.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: workspaceNamespace}, p.loadWorkspace)
	// A filesystem namespace is never an allowed input, including paths
	// esbuild might add through a resolver branch that skips plugins.
	build.OnLoad(api.OnLoadOptions{Filter: `.*`, Namespace: "file"}, func(api.OnLoadArgs) (api.OnLoadResult, error) {
		return api.OnLoadResult{}, errors.New("the packer only reads files inside the workspace or installed libraries")
	})
}

func (p *packer) resolveWorkspace(args api.OnResolveArgs) (api.OnResolveResult, error) {
	if err := p.scope.limits.step(); err != nil {
		return api.OnResolveResult{}, err
	}
	ref := args.Path
	dir := p.pageDir
	if args.Namespace == workspaceNamespace {
		dir = path.Dir(args.Importer)
	}
	// Only the stdin entry list uses the packer's reserved specifiers.
	if args.Namespace == "" && args.Kind == api.ResolveJSImportStatement {
		if file, ok := strings.CutPrefix(ref, workspaceNamespace+":"); ok {
			return api.OnResolveResult{Path: file, Namespace: workspaceNamespace}, nil
		}
		if _, ok := p.inline[ref]; ok {
			return api.OnResolveResult{Path: ref, Namespace: pageNamespace}, nil
		}
	}
	if isPreviewScriptURL(ref) {
		return previewImport(args)
	}
	if args.Kind == api.ResolveCSSURLToken && strings.HasPrefix(ref, "#") {
		return api.OnResolveResult{Path: ref, External: true}, nil
	}
	css := args.Kind == api.ResolveCSSURLToken || args.Kind == api.ResolveCSSImportRule
	if !isBare(ref) || css {
		target, err := p.scope.reference(dir, ref)
		if err != nil {
			return api.OnResolveResult{}, err
		}
		file, err := p.findWorkspaceFile(target, args.Kind)
		if err == nil {
			return api.OnResolveResult{Path: file, Namespace: workspaceNamespace, PluginData: args.With}, nil
		}
		if !css || !isBare(ref) || !errors.Is(err, fs.ErrNotExist) {
			return api.OnResolveResult{}, fmt.Errorf("%q: %w", ref, err)
		}
	}
	pkg, sub := splitSpecifier(ref)
	a, err := p.libs.owner(pkg)
	if err != nil {
		return api.OnResolveResult{}, err
	}
	return p.resolved(a, func() (string, error) {
		return a.resolvePackage("node_modules/"+pkg, sub, args.Kind == api.ResolveJSRequireCall)
	})
}

func (p *packer) findWorkspaceFile(target string, kind api.ResolveKind) (string, error) {
	names := []string{target}
	if kind == api.ResolveCSSImportRule {
		names = append(names, target+".css")
	} else if kind != api.ResolveCSSURLToken {
		for _, ext := range []string{".tsx", ".ts", ".jsx", ".js", ".css", ".json"} {
			names = append(names, target+ext, path.Join(target, "index"+ext))
		}
	}
	for _, name := range names {
		info, err := p.scope.ws.Stat(name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return "", err
		}
		if info.Mode().IsRegular() {
			return name, nil
		}
	}
	return "", fmt.Errorf("%s: %w", target, fs.ErrNotExist)
}

func (p *packer) loadWorkspace(args api.OnLoadArgs) (api.OnLoadResult, error) {
	data, err := p.scope.read(args.Path)
	if err != nil {
		return api.OnLoadResult{}, err
	}
	loader := p.loaders()[strings.ToLower(path.Ext(args.Path))]
	if attributes, ok := args.PluginData.(map[string]string); ok {
		for key, val := range attributes {
			if key == "type" {
				switch val {
				case "json":
					loader = api.LoaderJSON
					continue
				case "bytes":
					loader = api.LoaderBinary
					continue
				case "text":
					loader = api.LoaderText
					continue
				}
			}
			return api.OnLoadResult{}, fmt.Errorf("unsupported import attribute %s=%q", key, val)
		}
	}
	if loader == api.LoaderNone {
		return api.OnLoadResult{}, fmt.Errorf("%s has no supported file type for bundling", args.Path)
	}
	contents := string(data)
	if isWorkspaceJS(loader) {
		if err := checkWorkspaceJS(args.Path, contents, loader); err != nil {
			return api.OnLoadResult{}, err
		}
		p.addProblems(scanOwnSource(args.Path, contents)...)
	}
	return api.OnLoadResult{Contents: &contents, Loader: loader}, nil
}

func isWorkspaceJS(loader api.Loader) bool {
	return loader == api.LoaderJS || loader == api.LoaderJSX || loader == api.LoaderTS || loader == api.LoaderTSX
}

func checkWorkspaceJS(name, source string, loader api.Loader) error {
	if !isWorkspaceJS(loader) {
		return nil
	}
	// Esbuild resolves computed path imports as filesystem globs before its
	// plugin hooks. Its transform parser diagnoses those same non-literal
	// expressions without running a resolver. Use the same loader, syntax
	// settings and module format as the bundle, and promote those diagnostics.
	result := api.Transform(source, api.TransformOptions{
		Loader:            loader,
		Format:            api.FormatESModule,
		Platform:          api.PlatformBrowser,
		MinifyWhitespace:  true,
		MinifySyntax:      true,
		MinifyIdentifiers: true,
		TsconfigRaw:       "{}",
		Define:            map[string]string{"process.env.NODE_ENV": `"production"`},
		LogLevel:          api.LogLevelSilent,
		LogOverride:       map[string]api.LogLevel{"unsupported-dynamic-import": api.LogLevelWarning, "unsupported-require-call": api.LogLevelWarning},
	})
	for _, warning := range result.Warnings {
		switch warning.ID {
		case "unsupported-dynamic-import":
			return fmt.Errorf("%s: use a literal import path instead of a computed path", name)
		case "unsupported-require-call":
			if strings.Contains(warning.Text, "not a string literal") {
				return fmt.Errorf("%s: use a literal import or require path instead of a computed path", name)
			}
		}
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("%s: %s", name, result.Errors[0].Text)
	}
	return nil
}

type packReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r packReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}
