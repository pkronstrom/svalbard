package htmlpack

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

// Module resolution inside a library archive, as Node and esbuild resolve in
// a node_modules tree: a bare import walks up the importing file's own
// node_modules chain, so a package nested for a different version is found
// before the top-level one, and a package resolves through its exports
// conditions, then its browser, module and main fields.

// errEmptyModule is a browser-field replacement with false.
var errEmptyModule = errors.New("empty module")

// splitSpecifier splits a bare import into its package name and the subpath
// after it ("@jscad/modeling/src/x" → "@jscad/modeling", "src/x").
func splitSpecifier(spec string) (pkg, sub string) {
	parts := strings.SplitN(spec, "/", 3)
	if strings.HasPrefix(spec, "@") && len(parts) >= 2 {
		pkg = parts[0] + "/" + parts[1]
		if len(parts) == 3 {
			sub = parts[2]
		}
		return pkg, sub
	}
	pkg, sub, _ = strings.Cut(spec, "/")
	return pkg, sub
}

// isBare reports whether an import path names a package.
func isBare(spec string) bool {
	if spec == "" || strings.HasPrefix(spec, ".") || strings.HasPrefix(spec, "/") || strings.HasPrefix(spec, "#") {
		return false
	}
	if i := strings.Index(spec, ":"); i > 0 && !strings.Contains(spec[:i], "/") {
		return false // data:, http:, node: and other schemes
	}
	return true
}

// packageDir finds pkg for a file in folder from, walking up node_modules
// folders within the archive.
func (a *archive) packageDir(from, pkg string) (string, bool) {
	dir := from
	for {
		if !strings.HasSuffix(dir, "node_modules") {
			candidate := path.Join(dir, "node_modules", pkg)
			if dir == "" {
				candidate = "node_modules/" + pkg
			}
			if a.isDir(candidate) {
				return candidate, true
			}
		}
		if dir == "" || dir == "." {
			return "", false
		}
		dir = path.Dir(dir)
		if dir == "." {
			dir = ""
		}
	}
}

// resolveBare resolves a bare import made by a file in folder from.
func (a *archive) resolveBare(from, spec string, require bool) (string, error) {
	pkg, sub := splitSpecifier(spec)
	dir, ok := a.packageDir(from, pkg)
	if !ok {
		return "", fmt.Errorf("%s: %q is not installed beside %s", a.id, pkg, from)
	}
	return a.resolvePackage(dir, sub, require)
}

// resolvePackage resolves subpath sub of the package in dir.
func (a *archive) resolvePackage(dir, sub string, require bool) (string, error) {
	if main, ok := a.mains[dir]; ok && (sub == "" || sub == main) {
		return path.Join(dir, main), nil
	}
	pj, err := a.packageJSON(dir)
	if err != nil {
		return "", err
	}
	if pj != nil && pj.Exports != nil {
		target, err := resolveExports(pj.Exports, sub, conditions(require))
		if err != nil {
			return "", fmt.Errorf("%s: %s: %w", a.id, dir, err)
		}
		file := path.Join(dir, target)
		if a.isFile(file) {
			return a.browserMapped(dir, file)
		}
		return "", fmt.Errorf("%s: %s exports %q as %s, which is not installed", a.id, dir, "./"+sub, target)
	}
	if sub != "" {
		return a.resolveFile(dir, path.Join(dir, sub))
	}
	if pj != nil {
		for _, field := range []string{pj.BrowserEntry, pj.Module, pj.Main} {
			if field == "" {
				continue
			}
			if file, err := a.resolveFile(dir, path.Join(dir, field)); err == nil {
				return file, nil
			}
		}
	}
	return a.resolveFile(dir, path.Join(dir, "index"))
}

// conditions are the export conditions esbuild uses for the browser.
func conditions(require bool) []string {
	if require {
		return []string{"browser", "require", "module", "default"}
	}
	return []string{"browser", "import", "module", "default"}
}

// resolveFile finds a file as Node does: the path itself, with an
// extension, or a folder's index. pkgDir applies its browser field.
func (a *archive) resolveFile(pkgDir, p string) (string, error) {
	p = path.Clean(p)
	candidates := []string{p}
	for _, ext := range []string{".js", ".mjs", ".cjs", ".json", ".css"} {
		candidates = append(candidates, p+ext)
	}
	for _, index := range []string{"index.js", "index.mjs", "index.cjs", "index.json"} {
		candidates = append(candidates, path.Join(p, index))
	}
	for _, c := range candidates {
		if a.isFile(c) {
			return a.browserMapped(pkgDir, c)
		}
	}
	return "", fmt.Errorf("%s: %s is not installed", a.id, p)
}

// browserMapped applies a package's browser-field replacements to a file.
func (a *archive) browserMapped(pkgDir, file string) (string, error) {
	pj, _ := a.packageJSON(pkgDir)
	if pj == nil || pj.BrowserMap == nil {
		return file, nil
	}
	rel := "./" + strings.TrimPrefix(file, pkgDir+"/")
	for key, value := range pj.BrowserMap {
		if path.Clean(key) != path.Clean(rel) && path.Clean(key)+path.Ext(rel) != path.Clean(rel) {
			continue
		}
		switch v := value.(type) {
		case bool:
			if !v {
				return "", errEmptyModule
			}
		case string:
			return path.Join(pkgDir, v), nil
		}
	}
	return file, nil
}

// resolveExports finds subpath sub (without "./") in a package's exports.
func resolveExports(exports *orderedValue, sub string, conds []string) (string, error) {
	want := "."
	if sub != "" {
		want = "./" + sub
	}
	// Exports that are not a map of subpaths are the "." entry.
	isSubpathMap := exports.values != nil && len(exports.keys) > 0 && strings.HasPrefix(exports.keys[0], ".")
	if !isSubpathMap {
		if want != "." {
			return "", fmt.Errorf("subpath %q is not exported", want)
		}
		return resolveTarget(exports, "", conds)
	}
	if v, ok := exports.values[want]; ok && !strings.Contains(want, "*") {
		return resolveTarget(v, "", conds)
	}
	// Patterns: the longest prefix before "*" wins.
	best, bestKey, bestRest := (*orderedValue)(nil), "", ""
	for _, key := range exports.keys {
		prefix, suffix, ok := strings.Cut(key, "*")
		if !ok || !strings.HasPrefix(want, prefix) || !strings.HasSuffix(want, suffix) || len(want) < len(prefix)+len(suffix) {
			continue
		}
		if len(prefix) > len(bestKey) {
			best, bestKey, bestRest = exports.values[key], prefix, want[len(prefix):len(want)-len(suffix)]
		}
	}
	if best != nil {
		return resolveTarget(best, bestRest, conds)
	}
	// Old-style folder exports ("./lib/": "./lib/").
	for _, key := range exports.keys {
		if strings.HasSuffix(key, "/") && strings.HasPrefix(want, key) {
			target, err := resolveTarget(exports.values[key], "", conds)
			if err != nil {
				return "", err
			}
			return target + strings.TrimPrefix(want, key), nil
		}
	}
	return "", fmt.Errorf("subpath %q is not exported", want)
}

// resolveTarget picks a target by conditions, in the order the package
// lists them, and fills a pattern's "*".
func resolveTarget(v *orderedValue, star string, conds []string) (string, error) {
	switch {
	case v == nil || v.null:
		return "", fmt.Errorf("the export is null")
	case v.str != nil:
		target := strings.ReplaceAll(*v.str, "*", star)
		if !strings.HasPrefix(target, "./") {
			return "", fmt.Errorf("export target %q does not start with ./", target)
		}
		return strings.TrimPrefix(target, "./"), nil
	case v.array != nil:
		for _, item := range v.array {
			if target, err := resolveTarget(item, star, conds); err == nil {
				return target, nil
			}
		}
		return "", fmt.Errorf("no export target in the list works")
	default:
		for _, key := range v.keys {
			for _, c := range conds {
				if key == c {
					if target, err := resolveTarget(v.values[key], star, conds); err == nil {
						return target, nil
					}
				}
			}
		}
		return "", fmt.Errorf("no export condition matches %v", conds)
	}
}
