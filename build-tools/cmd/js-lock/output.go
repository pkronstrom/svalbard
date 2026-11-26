package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// The archive's layout must match host-cli/internal/apply (writeLibraryArchive)
// so the size recorded here is the size apply writes.
const libraryIndexFile = "svalbard-js.json"

var archiveTime = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)

type archiveInfo struct {
	files int
	bytes int64
}

// archiveSize builds the recipe's archive in memory, as apply will, and
// returns its file count and size.
func archiveSize(id string, sources []source, tree string) (archiveInfo, error) {
	mains := map[string]string{}
	var files []string
	for _, s := range sources {
		if s.node.umd != "" {
			mains[s.node.dir] = path.Clean(s.node.umd)
		}
		for _, rel := range s.include {
			files = append(files, s.node.dir+"/"+rel)
		}
	}
	sort.Strings(files)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	add := func(name string, r io.Reader) error {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: archiveTime})
		if err != nil {
			return err
		}
		_, err = io.Copy(w, r)
		return err
	}
	index, err := json.Marshal(struct {
		Recipe string            `json:"recipe"`
		Main   map[string]string `json:"main"`
	}{id, mains})
	if err != nil {
		return archiveInfo{}, err
	}
	if err := add(libraryIndexFile, bytes.NewReader(index)); err != nil {
		return archiveInfo{}, err
	}
	for _, name := range files {
		f, err := os.Open(filepath.Join(tree, filepath.FromSlash(name)))
		if err != nil {
			return archiveInfo{}, err
		}
		err = add(name, f)
		f.Close()
		if err != nil {
			return archiveInfo{}, err
		}
	}
	if err := zw.Close(); err != nil {
		return archiveInfo{}, err
	}
	return archiveInfo{files: len(files) + 1, bytes: int64(buf.Len())}, nil
}

// escapeGlob makes a file name an include pattern that matches only itself.
func escapeGlob(name string) string {
	return strings.NewReplacer(`\`, `\\`, `*`, `\*`, `?`, `\?`, `[`, `\[`).Replace(name)
}

func scalar(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: value}
}

func mapping(pairs ...any) *yaml.Node {
	m := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i < len(pairs); i += 2 {
		value, ok := pairs[i+1].(*yaml.Node)
		if !ok {
			value = scalar(fmt.Sprint(pairs[i+1]))
		}
		m.Content = append(m.Content, scalar(pairs[i].(string)), value)
	}
	return m
}

// field returns the value node of key in a mapping, or nil.
func field(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// setField replaces or appends key in a mapping.
func setField(m *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content[i+1] = value
			return
		}
	}
	m.Content = append(m.Content, scalar(key), value)
}

// packageLicense reads the licence a package.json in the tree declares.
func packageLicense(n *node, tree string) string {
	data, err := os.ReadFile(filepath.Join(tree, filepath.FromSlash(n.dir), "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		License any `json:"license"`
	}
	if json.Unmarshal(data, &pkg) != nil {
		return ""
	}
	switch v := pkg.License.(type) {
	case string:
		return v
	case map[string]any:
		if t, ok := v["type"].(string); ok {
			return t
		}
	}
	return ""
}

func libraryDocName(pkg string) string { return strings.ReplaceAll(pkg, "/", "__") + ".md" }

// writeRecipe writes the generated parts of the recipe (bundle, archive and
// each library's version) into outPath, keeping every other field of an
// existing file, through a temporary file.
func writeRecipe(outPath string, spec lockSpec, nodes []*node, sources []source, archive archiveInfo) error {
	doc := &yaml.Node{Kind: yaml.DocumentNode}
	if data, err := os.ReadFile(outPath); err == nil {
		if err := yaml.Unmarshal(data, doc); err != nil {
			return fmt.Errorf("%s: %w", outPath, err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{mapping("id", spec.ID, "type", "js-lib", "description", "")}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("%s: not a mapping", outPath)
	}
	libs := field(root, "libraries")
	if libs == nil || libs.Kind != yaml.SequenceNode {
		libs = &yaml.Node{Kind: yaml.SequenceNode}
	}
	for _, n := range nodes {
		if !n.library {
			continue
		}
		var entry *yaml.Node
		for _, item := range libs.Content {
			if p := field(item, "package"); p != nil && p.Value == n.name {
				entry = item
			}
		}
		if entry == nil {
			entry = mapping("package", n.name, "version", n.version, "license", n.license, "description", "", "entry", n.name, "quickstart", libraryDocName(n.name))
			libs.Content = append(libs.Content, entry)
		}
		setField(entry, "version", scalar(n.version))
		if lic := field(entry, "license"); lic == nil || lic.Value == "" {
			setField(entry, "license", scalar(n.license))
		}
		setField(entry, "quickstart", scalar(libraryDocName(n.name)))
	}
	setField(root, "libraries", libs)

	bundle := &yaml.Node{Kind: yaml.SequenceNode}
	for _, s := range sources {
		include := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
		for _, rel := range s.include {
			include.Content = append(include.Content, scalar(escapeGlob(rel)))
		}
		item := mapping("url", s.node.url, "sha256", s.node.sha256, "strip", s.node.strip, "dest", s.node.dir, "include", include)
		if s.node.umd != "" {
			setField(item, "main", scalar(s.node.umd))
		}
		setField(item, "extracted", mapping("files", strconv.Itoa(len(s.include)), "bytes", strconv.FormatInt(s.bytes, 10)))
		bundle.Content = append(bundle.Content, item)
	}
	setField(root, "bundle", bundle)
	setField(root, "archive", mapping("files", strconv.Itoa(archive.files), "bytes", strconv.FormatInt(archive.bytes, 10)))

	var buf bytes.Buffer
	buf.WriteString("# Generated by build-tools/cmd/js-lock from " + spec.ID + ".lock.json: bundle, archive and\n# each library's version. Edit the lock spec and run js-lock again; the other\n# fields are kept.\n")
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(stripHeader(doc)); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	tmp := outPath + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, outPath)
}

// stripHeader drops the generated header comment of a file written before,
// so it is not repeated.
func stripHeader(doc *yaml.Node) *yaml.Node {
	if strings.HasPrefix(doc.HeadComment, "# Generated by build-tools/cmd/js-lock") {
		doc.HeadComment = ""
	}
	if len(doc.Content) > 0 && strings.HasPrefix(doc.Content[0].HeadComment, "# Generated by build-tools/cmd/js-lock") {
		doc.Content[0].HeadComment = ""
	}
	return doc
}
