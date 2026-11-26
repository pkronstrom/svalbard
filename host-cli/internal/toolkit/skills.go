package toolkit

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pkronstrom/svalbard/host-cli/internal/manifest"
)

// embeddedSkills holds the Svalbard-authored agent skills, one directory per
// skills recipe ID, one folder per skill below it. README.md at the top holds
// the authoring rules and is not written to vaults.
//
//go:embed skills
var embeddedSkills embed.FS

// skillTree is the tree writeSkills copies from; tests replace it.
var skillTree = func() fs.FS {
	sub, err := fs.Sub(embeddedSkills, "skills")
	if err != nil {
		panic(err)
	}
	return sub
}

// EmbeddedSkills returns the embedded skill tree, rooted above the recipe
// directories.
func EmbeddedSkills() fs.FS { return skillTree() }

// skillRecordFile lists, per written skill folder, the SHA-256 of each file as
// Svalbard last wrote it, so an apply can tell a local edit from an older
// Svalbard version. Hidden, so the index and skill_read skip it.
const skillRecordFile = ".svalbard-sha256"

// SkillMeta is a SKILL.md's frontmatter.
type SkillMeta struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Requires    []string `yaml:"requires"`
}

// ParseSkillMeta reads the YAML frontmatter that opens a SKILL.md.
func ParseSkillMeta(data []byte) (SkillMeta, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return SkillMeta{}, errors.New("no frontmatter")
	}
	body := text[len("---\n"):]
	end := strings.Index(body, "\n---\n")
	if end < 0 {
		if strings.HasSuffix(body, "\n---") {
			end = len(body) - len("\n---")
		} else {
			return SkillMeta{}, errors.New("frontmatter is not closed")
		}
	}
	var meta SkillMeta
	if err := yaml.Unmarshal([]byte(body[:end]), &meta); err != nil {
		return SkillMeta{}, fmt.Errorf("frontmatter: %w", err)
	}
	return meta, nil
}

// RequirementsMet reports whether every requirement is realized: a recipe ID
// matches an entry with that ID, "type:X" any entry of type X.
func RequirementsMet(requires []string, entries []manifest.RealizedEntry) bool {
	for _, req := range requires {
		met := false
		for _, e := range entries {
			if t, ok := strings.CutPrefix(req, "type:"); ok {
				met = e.Type == t
			} else {
				met = e.ID == req
			}
			if met {
				break
			}
		}
		if !met {
			return false
		}
	}
	return true
}

// SkillsReport says what writeSkills did, by vault-relative slash path.
type SkillsReport struct {
	Written  []string // skill folders present after the write
	Removed  []string // skill folders deleted because their requirements went
	Restored []string // files whose local edits were replaced
}

// writeSkills writes the embedded skills of each realized skills recipe into
// its directory: every skill whose requires are realized, and nothing else.
// The directory is Svalbard's, so a skill folder that no longer qualifies is
// deleted and local edits inside a written one are replaced (and reported in
// Restored). A file is written only when its bytes differ, so the index does
// not read unchanged skills again. The directories of embedded recipes that
// are no longer realized are removed, but never skills/ itself: a missing
// index root is not a prune, so it must stay for the index to prune them.
func writeSkills(root string, entries []manifest.RealizedEntry, skills fs.FS) (SkillsReport, error) {
	var report SkillsReport
	realized := make(map[string]bool)
	for _, entry := range entries {
		if entry.Type != "skills" {
			continue
		}
		if info, err := fs.Stat(skills, entry.ID); err != nil || !info.IsDir() {
			continue // not an embedded skills recipe
		}
		realized[entry.ID] = true
		if err := writeRecipeSkills(root, entry, entries, skills, &report); err != nil {
			return report, err
		}
	}

	recipes, err := fs.ReadDir(skills, ".")
	if err != nil {
		return report, err
	}
	for _, recipe := range recipes {
		if !recipe.IsDir() || realized[recipe.Name()] {
			continue
		}
		dir := filepath.Join(root, TypeDirs["skills"], recipe.Name())
		if _, err := os.Lstat(dir); err != nil {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			return report, err
		}
	}
	return report, nil
}

func writeRecipeSkills(root string, entry manifest.RealizedEntry, entries []manifest.RealizedEntry, skills fs.FS, report *SkillsReport) error {
	dir := filepath.Join(root, entry.RelativePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	folders, err := fs.ReadDir(skills, entry.ID)
	if err != nil {
		return err
	}
	written := make(map[string]bool)
	for _, folder := range folders {
		if !folder.IsDir() {
			continue
		}
		source := path.Join(entry.ID, folder.Name())
		data, err := fs.ReadFile(skills, path.Join(source, "SKILL.md"))
		if err != nil {
			return fmt.Errorf("skill %s: %w", source, err)
		}
		meta, err := ParseSkillMeta(data)
		if err != nil {
			return fmt.Errorf("skill %s: %w", source, err)
		}
		if !RequirementsMet(meta.Requires, entries) {
			continue
		}
		rel := path.Join(filepath.ToSlash(entry.RelativePath), folder.Name())
		restored, err := syncSkillFolder(filepath.Join(dir, folder.Name()), skills, source)
		if err != nil {
			return fmt.Errorf("writing skill %s: %w", rel, err)
		}
		for _, file := range restored {
			report.Restored = append(report.Restored, path.Join(rel, file))
		}
		report.Written = append(report.Written, rel)
		written[folder.Name()] = true
	}

	children, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, child := range children {
		if written[child.Name()] || strings.HasPrefix(child.Name(), ".") {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, child.Name())); err != nil {
			return err
		}
		if child.IsDir() {
			report.Removed = append(report.Removed, path.Join(filepath.ToSlash(entry.RelativePath), child.Name()))
		}
	}
	return nil
}

// syncSkillFolder makes dest hold exactly the files of source in skills,
// writing only those that differ, and returns the files (relative, slash
// separated, sorted) whose state on disk was not what Svalbard last wrote.
func syncSkillFolder(dest string, skills fs.FS, source string) ([]string, error) {
	recorded := readSkillRecord(filepath.Join(dest, skillRecordFile))
	want := make(map[string][]byte)
	err := fs.WalkDir(skills, source, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(skills, p)
		if err != nil {
			return err
		}
		rel, _ := strings.CutPrefix(p, source+"/")
		want[rel] = data
		return nil
	})
	if err != nil {
		return nil, err
	}

	restored := make(map[string]bool)
	// What is on disk: files Svalbard did not write go, and edited or
	// deleted ones count as restored.
	if _, err := os.Stat(dest); err == nil {
		err := filepath.WalkDir(dest, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if p == dest {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".svalbard-tmp-") {
				return os.Remove(p) // left by an interrupted write
			}
			// Hidden files are the system's (.DS_Store, the ._ files macOS
			// writes on exFAT) or Svalbard's record: never a local edit, and
			// the index skips them. Embedded skills have none.
			if strings.HasPrefix(d.Name(), ".") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(dest, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if _, ok := want[rel]; !ok {
				// A file an older version wrote is dropped quietly; any
				// other is the user's.
				if sum, err := fileSHA256(p); err != nil || sum != recorded[rel] {
					restored[rel] = true
				}
				return os.Remove(p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	for rel := range recorded {
		if _, ok := want[rel]; !ok {
			continue // an older version's file, already removed above or gone
		}
		if _, err := os.Lstat(filepath.Join(dest, filepath.FromSlash(rel))); os.IsNotExist(err) {
			restored[rel] = true
		}
	}

	names := make([]string, 0, len(want))
	for rel := range want {
		names = append(names, rel)
	}
	sort.Strings(names)
	record := make([]string, 0, len(names))
	for _, rel := range names {
		data := want[rel]
		sum := sha256.Sum256(data)
		record = append(record, hex.EncodeToString(sum[:])+"  "+rel)
		target := filepath.Join(dest, filepath.FromSlash(rel))
		current, err := os.ReadFile(target)
		if err == nil && bytes.Equal(current, data) {
			if err := ensureSkillMode(target, rel); err != nil {
				return nil, err
			}
			continue
		}
		if err == nil {
			got := sha256.Sum256(current)
			if hex.EncodeToString(got[:]) != recorded[rel] {
				restored[rel] = true
			}
		}
		if err := writeFileAtomic(target, data, skillFileMode(rel)); err != nil {
			return nil, err
		}
	}
	if err := removeEmptyDirs(dest); err != nil {
		return nil, err
	}
	recordData := []byte(strings.Join(record, "\n") + "\n")
	recordPath := filepath.Join(dest, skillRecordFile)
	if current, err := os.ReadFile(recordPath); err != nil || !bytes.Equal(current, recordData) {
		if err := writeFileAtomic(recordPath, recordData, 0o644); err != nil {
			return nil, err
		}
	}

	files := make([]string, 0, len(restored))
	for rel := range restored {
		files = append(files, rel)
	}
	sort.Strings(files)
	return files, nil
}

func fileSHA256(p string) (string, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func readSkillRecord(p string) map[string]string {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	record := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		sum, rel, ok := strings.Cut(line, "  ")
		if ok {
			record[rel] = sum
		}
	}
	return record
}

// skillFileMode makes shell scripts and anything under scripts/ executable.
func skillFileMode(rel string) os.FileMode {
	if strings.HasSuffix(rel, ".sh") || strings.HasPrefix(rel, "scripts/") {
		return 0o755
	}
	return 0o644
}

func ensureSkillMode(target, rel string) error {
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if want := skillFileMode(rel); info.Mode().Perm() != want {
		// exFAT and FAT report fixed modes and may refuse chmod; the
		// scripts run through sh and py-docs either way.
		_ = os.Chmod(target, want)
	}
	return nil
}

// writeFileAtomic replaces target through a hidden temporary file beside it,
// so a reader never sees half a file.
func writeFileAtomic(target string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".svalbard-tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	_ = os.Chmod(name, mode) // see ensureSkillMode
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, target); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// removeEmptyDirs deletes the empty directories below dir, deepest first.
func removeEmptyDirs(dir string) error {
	var dirs []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p != dir {
			dirs = append(dirs, p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	slices.Reverse(dirs)
	for _, d := range dirs {
		if children, err := os.ReadDir(d); err == nil && len(children) == 0 {
			if err := os.Remove(d); err != nil {
				return err
			}
		}
	}
	return nil
}

// skillNoticesFor returns one notice per skill with restored files.
func skillNoticesFor(report SkillsReport) []string {
	var notices []string
	seen := make(map[string]bool)
	for _, file := range report.Restored {
		parts := strings.SplitN(file, "/", 4) // skills/<recipe>/<skill>/<file>
		if len(parts) < 4 || seen[parts[2]] {
			continue
		}
		seen[parts[2]] = true
		notices = append(notices, "restored svalbard skill "+parts[2]+" (local edits replaced)")
	}
	return notices
}
