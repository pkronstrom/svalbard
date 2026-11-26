package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type lockSpec struct {
	ID       string                 `json:"id"`
	Packages map[string]string      `json:"packages"`
	Entries  map[string][]string    `json:"entries"`
	Include  map[string][]string    `json:"include"`
	UMD      map[string]string      `json:"umd"`
	Tarballs map[string]tarballSpec `json:"tarballs"`
}

type tarballSpec struct {
	URL     string `json:"url"`
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

type locker struct {
	registry string
	cache    string
	client   *http.Client
	log      io.Writer

	metadata map[string]*packageMetadata
}

// packageMetadata is the part of the registry's document for a package that
// resolution needs.
type packageMetadata struct {
	DistTags map[string]string          `json:"dist-tags"`
	Versions map[string]versionMetadata `json:"versions"`
}

type versionMetadata struct {
	Version      string            `json:"version"`
	Dependencies map[string]string `json:"dependencies"`
	Dist         struct {
		Tarball   string `json:"tarball"`
		Integrity string `json:"integrity"`
		Shasum    string `json:"shasum"`
	} `json:"dist"`
}

// node is one package placed in the tree.
type node struct {
	name    string
	version string
	dir     string // node_modules/a or node_modules/a/node_modules/c
	deps    map[string]string
	library bool
	umd     string // self-contained build, for a library listed in the spec's umd

	url       string
	integrity string
	shasum    string
	sha256    string // expected, for a direct tarball
	direct    bool

	license string // from its package.json

	archive string            // downloaded tarball
	strip   string            // its top folder ("package")
	files   map[string]string // every file below strip → its path in the tree
}

func (l *locker) logf(format string, args ...any) {
	if l.log != nil {
		fmt.Fprintf(l.log, format+"\n", args...)
	}
}

func (l *locker) run(ctx context.Context, specPath, outPath string) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	var spec lockSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("%s: %w", specPath, err)
	}
	if spec.ID == "" || len(spec.Packages)+len(spec.Tarballs) == 0 {
		return fmt.Errorf("%s: needs an id and packages", specPath)
	}
	l.metadata = map[string]*packageMetadata{}

	nodes, err := l.resolve(ctx, spec)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp("", "js-lock-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	tree := filepath.Join(work, "tree")
	for _, n := range nodes {
		if err := l.fetch(ctx, n); err != nil {
			return err
		}
		if err := unpack(n, tree); err != nil {
			return fmt.Errorf("%s@%s: %w", n.name, n.version, err)
		}
		n.license = packageLicense(n, tree)
	}
	closure, err := bundleClosure(spec, nodes, tree)
	if err != nil {
		return err
	}
	sources, err := chooseFiles(spec, nodes, closure, tree)
	if err != nil {
		return err
	}
	archive, err := archiveSize(spec.ID, sources, tree)
	if err != nil {
		return err
	}
	return writeRecipe(outPath, spec, nodes, sources, archive)
}

// resolve places the spec's packages at the top of the tree and their
// dependencies where npm would: at the top when no other version is there,
// otherwise below the package that needs them.
func (l *locker) resolve(ctx context.Context, spec lockSpec) ([]*node, error) {
	placed := map[string]*node{} // by dir
	var order []*node
	type want struct {
		from, name, rng string
	}
	var queue []want

	names := make([]string, 0, len(spec.Packages)+len(spec.Tarballs))
	for name := range spec.Packages {
		names = append(names, name)
	}
	for name := range spec.Tarballs {
		if _, dup := spec.Packages[name]; dup {
			return nil, fmt.Errorf("%s is both a package and a tarball", name)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		n := &node{name: name, dir: "node_modules/" + name, library: true, umd: spec.UMD[name]}
		if tb, ok := spec.Tarballs[name]; ok {
			if tb.URL == "" || tb.Version == "" {
				return nil, fmt.Errorf("tarball %s needs a url and a version", name)
			}
			n.version, n.url, n.sha256, n.direct = tb.Version, tb.URL, strings.ToLower(tb.SHA256), true
		} else {
			rng, err := parseRange(spec.Packages[name])
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			meta, err := l.pick(ctx, name, rng)
			if err != nil {
				return nil, err
			}
			n.version, n.deps = meta.Version, meta.Dependencies
			n.url, n.integrity, n.shasum = meta.Dist.Tarball, meta.Dist.Integrity, meta.Dist.Shasum
			if v, _ := parseVersion(meta.Version); !sameMajorAsRange(v, spec.Packages[name]) {
				return nil, fmt.Errorf("%s resolved to %s, outside the major %s pins", name, meta.Version, spec.Packages[name])
			}
		}
		placed[n.dir] = n
		order = append(order, n)
	}
	// A direct tarball's dependencies come from its own package.json.
	for _, n := range order {
		if n.direct && n.umd == "" {
			if err := l.fetch(ctx, n); err != nil {
				return nil, err
			}
			deps, err := tarballDependencies(n.archive)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", n.name, err)
			}
			n.deps = deps
		}
	}
	enqueue := func(n *node) {
		if n.umd != "" {
			return // a self-contained build brings no dependencies
		}
		deps := make([]string, 0, len(n.deps))
		for dep := range n.deps {
			deps = append(deps, dep)
		}
		sort.Strings(deps)
		for _, dep := range deps {
			queue = append(queue, want{from: n.dir, name: dep, rng: n.deps[dep]})
		}
	}
	for _, n := range order {
		enqueue(n)
	}
	for len(queue) > 0 {
		w := queue[0]
		queue = queue[1:]
		rng, err := parseRange(w.rng)
		if err != nil {
			return nil, fmt.Errorf("%s needs %s@%s: %w", w.from, w.name, w.rng, err)
		}
		// Walk up from the package that needs it, as Node resolves.
		var found *node
		for dir := w.from; ; dir = parentPackageDir(dir) {
			if n, ok := placed[dir+"/node_modules/"+w.name]; ok {
				found = n
				break
			}
			if dir == "" {
				break
			}
		}
		if found == nil {
			found = placed["node_modules/"+w.name]
		}
		if found != nil {
			if v, ok := parseVersion(found.version); ok && rng.allows(v) {
				continue
			}
		}
		meta, err := l.pick(ctx, w.name, rng)
		if err != nil {
			return nil, fmt.Errorf("%s needs %s: %w", w.from, w.name, err)
		}
		dir := "node_modules/" + w.name
		if _, taken := placed[dir]; taken {
			dir = w.from + "/node_modules/" + w.name
		}
		n := &node{name: w.name, version: meta.Version, dir: dir, deps: meta.Dependencies,
			url: meta.Dist.Tarball, integrity: meta.Dist.Integrity, shasum: meta.Dist.Shasum}
		placed[dir] = n
		order = append(order, n)
		enqueue(n)
	}
	sort.Slice(order, func(i, j int) bool { return order[i].dir < order[j].dir })
	return order, nil
}

// parentPackageDir is the package folder above dir in the tree:
// node_modules/a/node_modules/b → node_modules/a, node_modules/a → "".
func parentPackageDir(dir string) string {
	cut := strings.LastIndex(dir, "/node_modules/")
	if cut < 0 {
		return ""
	}
	return dir[:cut]
}

// sameMajorAsRange reports whether v stays inside the major an x-range or
// caret range names ("5.x", "^5.2"); other ranges are not checked.
func sameMajorAsRange(v version, rng string) bool {
	rng = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(rng), "^"))
	major, _, _ := strings.Cut(rng, ".")
	var want int
	if _, err := fmt.Sscanf(major, "%d", &want); err != nil {
		return true
	}
	return v.major == want
}

func (l *locker) pick(ctx context.Context, name string, rng versionRange) (versionMetadata, error) {
	meta, ok := l.metadata[name]
	if !ok {
		u := strings.TrimRight(l.registry, "/") + "/" + strings.Replace(url.PathEscape(name), "%40", "@", 1)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return versionMetadata{}, err
		}
		req.Header.Set("Accept", "application/vnd.npm.install-v1+json; q=1.0, application/json; q=0.8")
		resp, err := l.client.Do(req)
		if err != nil {
			return versionMetadata{}, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return versionMetadata{}, fmt.Errorf("registry: %s: %s", name, resp.Status)
		}
		meta = &packageMetadata{}
		if err := json.NewDecoder(resp.Body).Decode(meta); err != nil {
			return versionMetadata{}, fmt.Errorf("registry: %s: %w", name, err)
		}
		l.metadata[name] = meta
	}
	// The version tagged latest wins when the range allows it, as npm
	// picks: some projects publish newer builds that are not latest.
	if tagged, ok := meta.Versions[meta.DistTags["latest"]]; ok {
		if v, ok := parseVersion(tagged.Version); ok && rng.allows(v) {
			return tagged, nil
		}
	}
	available := make([]string, 0, len(meta.Versions))
	for v := range meta.Versions {
		available = append(available, v)
	}
	best, found := maxSatisfying(available, rng)
	if !found {
		return versionMetadata{}, fmt.Errorf("no version of %s matches %s", name, rng.raw)
	}
	return meta.Versions[best.raw], nil
}

// fetch downloads a node's tarball into the cache, unless a checked copy is
// there, and verifies it: against the registry's sha512 integrity (or sha1
// shasum), and against the spec's sha256 for a direct tarball.
func (l *locker) fetch(ctx context.Context, n *node) error {
	if n.archive != "" {
		return nil
	}
	if err := os.MkdirAll(l.cache, 0o755); err != nil {
		return err
	}
	target := filepath.Join(l.cache, strings.ReplaceAll(n.name, "/", "_")+"-"+n.version+".tgz")
	data, err := os.ReadFile(target)
	if err != nil || verify(n, data) != nil {
		l.logf("downloading %s", n.url)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.url, nil)
		if err != nil {
			return err
		}
		resp, err := l.client.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("%s: %s", n.url, resp.Status)
		}
		if data, err = io.ReadAll(resp.Body); err != nil {
			return err
		}
		if err := verify(n, data); err != nil {
			return err
		}
		if err := os.WriteFile(target+".part", data, 0o644); err != nil {
			return err
		}
		if err := os.Rename(target+".part", target); err != nil {
			return err
		}
	}
	sum := sha256.Sum256(data)
	n.sha256 = hex.EncodeToString(sum[:])
	n.archive = target
	return nil
}

func verify(n *node, data []byte) error {
	if n.direct {
		if n.sha256 == "" {
			return nil
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != n.sha256 {
			return fmt.Errorf("%s: sha256 %s, the spec pins %s", n.url, got, n.sha256)
		}
		return nil
	}
	if algo, digest, ok := strings.Cut(n.integrity, "-"); ok && algo == "sha512" {
		sum := sha512.Sum512(data)
		if got := base64.StdEncoding.EncodeToString(sum[:]); got != digest {
			return fmt.Errorf("%s@%s: the tarball does not match the registry's sha512 integrity", n.name, n.version)
		}
		return nil
	}
	if n.shasum != "" {
		sum := sha1.Sum(data)
		if got := hex.EncodeToString(sum[:]); !strings.EqualFold(got, n.shasum) {
			return fmt.Errorf("%s@%s: the tarball does not match the registry's sha1 shasum", n.name, n.version)
		}
		return nil
	}
	return fmt.Errorf("%s@%s: the registry gives no integrity to check the tarball against", n.name, n.version)
}

// walkTarball calls fn for every regular file of a gzipped tarball.
func walkTarball(archive string, fn func(name string, r io.Reader) error) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		name := strings.TrimPrefix(path.Clean("/"+hdr.Name), "/")
		if err := fn(name, tr); err != nil {
			return err
		}
	}
}

func tarballDependencies(archive string) (map[string]string, error) {
	var deps map[string]string
	found := false
	err := walkTarball(archive, func(name string, r io.Reader) error {
		if strings.Count(name, "/") != 1 || path.Base(name) != "package.json" || found {
			return nil
		}
		found = true
		var pkg struct {
			Dependencies map[string]string `json:"dependencies"`
		}
		if err := json.NewDecoder(r).Decode(&pkg); err != nil {
			return err
		}
		deps = pkg.Dependencies
		return nil
	})
	if err == nil && !found {
		err = errors.New("no package.json in the tarball")
	}
	return deps, err
}

// unpack writes a node's whole package into the tree at its dir.
func unpack(n *node, tree string) error {
	n.files = map[string]string{}
	tops := map[string]bool{}
	if err := walkTarball(n.archive, func(name string, _ io.Reader) error {
		top, _, _ := strings.Cut(name, "/")
		tops[top] = true
		return nil
	}); err != nil {
		return err
	}
	if len(tops) != 1 {
		return fmt.Errorf("the tarball has %d top folders, want one", len(tops))
	}
	for top := range tops {
		n.strip = top
	}
	return walkTarball(n.archive, func(name string, r io.Reader) error {
		rel, ok := strings.CutPrefix(name, n.strip+"/")
		if !ok || rel == "" {
			return nil
		}
		target := filepath.Join(tree, filepath.FromSlash(n.dir), filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, r); err != nil {
			out.Close()
			return err
		}
		n.files[rel] = n.dir + "/" + rel
		return out.Close()
	})
}

var licenseRe = regexp.MustCompile(`(?i)^(licen[cs]e|copying|notice)([.-].*)?$`)
var readmeRe = regexp.MustCompile(`(?i)^readme(\.(md|markdown|txt))?$`)

// source is one node's trimmed bundle entry.
type source struct {
	node    *node
	include []string
	bytes   int64
}

// chooseFiles picks each node's files: the closure, package.json, licences,
// a library's README and the spec's extra patterns. A node that contributes
// nothing to any closure is dropped, a library never is.
func chooseFiles(spec lockSpec, nodes []*node, closure map[string]bool, tree string) ([]source, error) {
	var sources []source
	for _, n := range nodes {
		keep := map[string]bool{}
		for rel, full := range n.files {
			if closure[full] {
				keep[rel] = true
			}
		}
		if n.umd != "" {
			if _, ok := n.files[n.umd]; !ok {
				return nil, fmt.Errorf("%s has no %s for its self-contained build", n.name, n.umd)
			}
			keep[n.umd] = true
		}
		for _, pattern := range spec.Include[n.name] {
			matched := false
			for rel := range n.files {
				if matchPattern(pattern, rel) {
					keep[rel], matched = true, true
				}
			}
			if !matched {
				return nil, fmt.Errorf("%s: include pattern %q matches no file", n.name, pattern)
			}
		}
		if len(keep) == 0 && !n.library {
			continue // nothing a page reads: Node-only, or unused in the browser
		}
		if len(keep) == 0 {
			return nil, fmt.Errorf("library %s has no file in its closure; check its entries", n.name)
		}
		for rel := range n.files {
			top := !strings.Contains(rel, "/")
			if rel == "package.json" || (top && licenseRe.MatchString(rel)) || (top && n.library && readmeRe.MatchString(rel)) {
				keep[rel] = true
			}
		}
		s := source{node: n}
		for rel := range keep {
			s.include = append(s.include, rel)
			info, err := os.Stat(filepath.Join(tree, filepath.FromSlash(n.files[rel])))
			if err != nil {
				return nil, err
			}
			s.bytes += info.Size()
		}
		sort.Strings(s.include)
		sources = append(sources, s)
	}
	return sources, nil
}

// matchPattern is the bundle include rule: path.Match per segment, and a
// final "**" for everything below.
func matchPattern(pattern, rel string) bool {
	ps := strings.Split(strings.Trim(pattern, "/"), "/")
	rs := strings.Split(rel, "/")
	if last := len(ps) - 1; ps[last] == "**" {
		if len(rs) <= last {
			return false
		}
		ps, rs = ps[:last], rs[:last]
	}
	if len(ps) != len(rs) {
		return false
	}
	for i := range ps {
		if ok, _ := path.Match(ps[i], rs[i]); !ok {
			return false
		}
	}
	return true
}
