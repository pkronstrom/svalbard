package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// assetLoaders are the files a stylesheet or module may pull in besides code;
// "file" makes esbuild read them, so they count in the closure.
var assetLoaders = map[string]api.Loader{
	".woff2": api.LoaderFile, ".woff": api.LoaderFile, ".ttf": api.LoaderFile, ".otf": api.LoaderFile, ".eot": api.LoaderFile,
	".png": api.LoaderFile, ".jpg": api.LoaderFile, ".gif": api.LoaderFile, ".svg": api.LoaderFile, ".webp": api.LoaderFile,
	".wasm": api.LoaderFile,
}

// bundleClosure bundles each library's entries in the tree and returns every
// file esbuild read, by its slash path in the tree. A library listed in umd
// is not bundled: its build is the one file kept.
func bundleClosure(spec lockSpec, nodes []*node, tree string) (map[string]bool, error) {
	closure := map[string]bool{}
	mains := map[string]string{}
	for _, n := range nodes {
		if n.library && n.umd != "" {
			mains[n.name] = filepath.Join(tree, filepath.FromSlash(n.dir), filepath.FromSlash(n.umd))
		}
	}
	for _, n := range nodes {
		if !n.library || n.umd != "" {
			continue
		}
		entries := spec.Entries[n.name]
		if len(entries) == 0 {
			entries = []string{n.name}
		}
		var lines []string
		for _, entry := range entries {
			lines = append(lines, fmt.Sprintf("import %q;", entry))
		}
		result := api.Build(api.BuildOptions{
			Stdin:         &api.StdinOptions{Contents: strings.Join(lines, "\n"), ResolveDir: tree, Sourcefile: "entries.js", Loader: api.LoaderJS},
			AbsWorkingDir: tree,
			Bundle:        true,
			Write:         false,
			Metafile:      true,
			Outdir:        filepath.Join(tree, ".out"),
			Platform:      api.PlatformBrowser,
			Format:        api.FormatESModule,
			Loader:        assetLoaders,
			Plugins:       []api.Plugin{mainPlugin(mains)},
			LogLevel:      api.LogLevelSilent,
		})
		if len(result.Errors) > 0 {
			var msgs []string
			for _, msg := range result.Errors {
				text := msg.Text
				if msg.Location != nil {
					text = msg.Location.File + ": " + text
				}
				msgs = append(msgs, text)
			}
			return nil, fmt.Errorf("bundling %s (%s): %s", n.name, strings.Join(entries, ", "), strings.Join(msgs, "; "))
		}
		var meta struct {
			Inputs map[string]json.RawMessage `json:"inputs"`
		}
		if err := json.Unmarshal([]byte(result.Metafile), &meta); err != nil {
			return nil, err
		}
		for input := range meta.Inputs {
			if strings.HasPrefix(input, "node_modules/") {
				closure[input] = true
			}
		}
	}
	return closure, nil
}

// mainPlugin resolves a bare import of a package with a self-contained build
// to that file, as `svalbard-drive html pack` does from the archive's index.
func mainPlugin(mains map[string]string) api.Plugin {
	names := make([]string, 0, len(mains))
	for name := range mains {
		names = append(names, name)
	}
	sort.Strings(names)
	return api.Plugin{Name: "svalbard-main", Setup: func(build api.PluginBuild) {
		build.OnResolve(api.OnResolveOptions{Filter: `^[^./]`}, func(args api.OnResolveArgs) (api.OnResolveResult, error) {
			if file, ok := mains[args.Path]; ok {
				return api.OnResolveResult{Path: file}, nil
			}
			return api.OnResolveResult{}, nil
		})
	}}
}
