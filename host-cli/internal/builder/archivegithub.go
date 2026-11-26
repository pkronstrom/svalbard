package builder

import (
	"archive/zip"
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/andybalholm/cascadia"
	"golang.org/x/net/html"
)

// The GitHub endpoints a linked repository is read from; tests point them at
// a local server.
var (
	githubWeb      = "https://github.com"
	githubAPI      = "https://api.github.com"
	githubRaw      = "https://raw.githubusercontent.com"
	githubCodeload = "https://codeload.github.com"
	githubMedia    = "https://media.githubusercontent.com/media"
)

// githubLFSPointer starts the stub Git LFS keeps in a repository in place of
// a large file; the file itself is served from githubMedia.
var githubLFSPointer = []byte("version https://git-lfs.github.com/spec/")

const (
	// maxArchiveRepos bounds the repositories one project takes in: its own
	// and the ones its page names, not every library it mentions.
	maxArchiveRepos = 3
	// archiveRepoWhole is the largest repository kept whole. A larger one
	// keeps its docs, design files and firmware, smallest first, up to
	// archiveRepoSelected.
	archiveRepoWhole    = 25 << 20
	archiveRepoSelected = 50 << 20
	archiveRepoFileMax  = 40 << 20
	archiveRepoMaxFiles = 800
	archiveRepoImageMax = 2 << 20
)

// archiveRepo is a GitHub repository, or one folder of it, that a project
// links to.
type archiveRepo struct {
	Owner, Name, Ref, Dir string
	anchors               []*html.Node
}

func (repo archiveRepo) label() string {
	return strings.TrimSuffix(repo.Owner+"/"+repo.Name+"/"+repo.Dir, "/")
}

func (repo archiveRepo) page() *url.URL {
	page, _ := url.Parse(githubWeb)
	if repo.Dir != "" {
		return page.JoinPath(repo.Owner, repo.Name, "tree", repo.Ref, repo.Dir)
	}
	return page.JoinPath(repo.Owner, repo.Name)
}

// githubReservedOwners are github.com paths that are site pages, not users.
var githubReservedOwners = map[string]bool{
	"about": true, "account": true, "apps": true, "codespaces": true, "collections": true, "contact": true, "customer-stories": true,
	"dashboard": true, "enterprise": true, "explore": true, "features": true, "join": true, "login": true, "marketplace": true,
	"new": true, "notifications": true, "orgs": true, "pricing": true, "readme": true, "search": true, "security": true,
	"settings": true, "signup": true, "site": true, "sponsors": true, "team": true, "topics": true, "trending": true, "users": true,
}

// archiveRepoLink reads a link to a repository, its releases or one of its
// folders.
func archiveRepoLink(target *url.URL) (archiveRepo, bool) {
	web, err := url.Parse(githubWeb)
	if err != nil || !strings.EqualFold(strings.TrimPrefix(target.Host, "www."), web.Host) {
		return archiveRepo{}, false
	}
	parts := strings.Split(strings.Trim(target.Path, "/"), "/")
	if len(parts) < 2 || githubReservedOwners[strings.ToLower(parts[0])] || parts[1] == "" {
		return archiveRepo{}, false
	}
	repo := archiveRepo{Owner: parts[0], Name: strings.TrimSuffix(parts[1], ".git")}
	switch {
	case len(parts) == 2, parts[2] == "releases":
	case parts[2] == "tree" && len(parts) >= 4:
		repo.Ref, repo.Dir = parts[3], strings.Join(parts[4:], "/")
	default:
		return archiveRepo{}, false
	}
	return repo, true
}

// archiveRepos lists the repositories a project uses: its own page first if
// that is one, then those its text links, each once, with the anchors that
// name it.
func archiveRepos(article *archiveArticle, source *url.URL) []archiveRepo {
	var repos []archiveRepo
	index := make(map[string]int)
	add := func(target *url.URL, anchor *html.Node) {
		repo, ok := archiveRepoLink(target)
		if !ok {
			return
		}
		key := strings.ToLower(repo.Owner + "/" + repo.Name)
		position, known := index[key]
		if !known {
			if len(repos) >= maxArchiveRepos {
				return
			}
			position = len(repos)
			index[key] = position
			repos = append(repos, repo)
		}
		if anchor != nil {
			repos[position].anchors = append(repos[position].anchors, anchor)
		}
	}
	add(source, nil)
	for _, anchor := range cascadia.QueryAll(article.Content, cascadia.MustCompile("a[href]")) {
		if target, err := article.Base.Parse(archiveAttribute(anchor, "href")); err == nil {
			add(target, anchor)
		}
	}
	return repos
}

// appendArchiveRepoReadmes adds the README of each linked repository as a
// section, since that is where a build guide usually continues.
func appendArchiveRepoReadmes(fetcher *archiveFetcher, article *archiveArticle, repos []archiveRepo, source *url.URL) {
	own, _ := archiveRepoLink(source)
	for _, repo := range repos {
		if fetcher.ctx.Err() != nil {
			return
		}
		if strings.EqualFold(repo.Owner+"/"+repo.Name, own.Owner+"/"+own.Name) {
			continue
		}
		rule := archiveRuleFor("github.com", fetcher.rules)
		part, err := archiveHTMLArticle(fetcher, repo.page(), rule)
		if err != nil {
			continue
		}
		// Its docs and wiki, where an assembly guide usually is.
		appendArchiveSubpages(fetcher, part, rule)
		pruneArchiveChrome(part.Content)
		appendArchiveSection(article, part, repo.label())
	}
}

type githubTreeEntry struct {
	Path string `json:"path"`
	Type string `json:"type"`
	Size int    `json:"size"`
}

// archiveStoreRepo packs a repository, or the linked folder of one, into a
// zip of at most budget bytes: whole when it is small, otherwise the files a
// build needs, with the rest listed in OMITTED.txt. The latest release's
// design and firmware files go under releases/.
func archiveStoreRepo(fetcher *archiveFetcher, repo archiveRepo, output string, budget int) (archiveStoredFile, error) {
	api, err := url.Parse(githubAPI)
	if err != nil {
		return archiveStoredFile{}, err
	}
	api = api.JoinPath("repos", repo.Owner, repo.Name)
	if repo.Ref == "" {
		var info struct {
			DefaultBranch string `json:"default_branch"`
		}
		if err := archiveGetJSON(fetcher, api, &info); err != nil {
			return archiveStoredFile{}, err
		}
		repo.Ref = info.DefaultBranch
	}
	treeish := repo.Ref
	if repo.Dir != "" {
		treeish += ":" + repo.Dir
	}
	treeURL := api.JoinPath("git", "trees", treeish)
	treeURL.RawQuery = "recursive=1"
	var tree struct {
		Truncated bool              `json:"truncated"`
		Tree      []githubTreeEntry `json:"tree"`
	}
	if err := archiveGetJSON(fetcher, treeURL, &tree); err != nil {
		return archiveStoredFile{}, err
	}
	if tree.Truncated {
		return archiveStoredFile{}, fmt.Errorf("%s: too many files to list", repo.label())
	}
	var blobs []githubTreeEntry
	total, lfs := 0, false
	for _, entry := range tree.Tree {
		if entry.Type == "blob" && path.Base(entry.Path) == ".gitattributes" {
			lfs = lfs || archiveRepoUsesLFS(fetcher, repo, entry.Path)
		}
		if entry.Type == "blob" && !archiveRepoSkippedPath(entry.Path) {
			blobs = append(blobs, entry)
			total += entry.Size
		}
	}
	limit := min(archiveRepoSelected, budget)
	whole := total <= min(archiveRepoWhole, limit)

	var packed bytes.Buffer
	writer := zip.NewWriter(&packed)
	size, kept := 0, 0
	var omitted []githubTreeEntry
	// GitHub's zip holds only the stubs of LFS files.
	if whole && repo.Dir == "" && !lfs && len(blobs) > 0 {
		if size, kept, err = archiveCopyRepoZip(fetcher, writer, repo, limit); err != nil || kept == 0 {
			size, kept = 0, 0
			packed.Reset()
			writer = zip.NewWriter(&packed)
		}
	}
	if kept == 0 {
		selected := blobs
		if !whole {
			selected, omitted = archiveRepoSelection(blobs, limit)
		}
		size, kept, omitted = archiveFetchRepoFiles(fetcher, writer, repo, selected, omitted, limit)
	}
	if kept == 0 {
		return archiveStoredFile{}, fmt.Errorf("%s: no files", repo.label())
	}
	if repo.Dir == "" {
		size += archiveFetchReleaseFiles(fetcher, writer, api, limit-size)
	}
	if len(omitted) > 0 {
		archiveWriteOmitted(writer, omitted)
	}
	if err := writer.Close(); err != nil {
		return archiveStoredFile{}, err
	}
	name := archiveFileName(strings.ReplaceAll(strings.TrimSuffix(repo.Name+"/"+repo.Dir, "/"), "/", "-")) + ".zip"
	sum := sha256.Sum256([]byte(repo.label()))
	local := "_files/" + hex.EncodeToString(sum[:3]) + "-" + name
	destination := filepath.Join(output, filepath.FromSlash(local))
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return archiveStoredFile{}, err
	}
	if err := os.WriteFile(destination, packed.Bytes(), 0o644); err != nil {
		return archiveStoredFile{}, err
	}
	note := "whole repository"
	if repo.Dir != "" {
		note = "whole folder"
	}
	if len(omitted) > 0 {
		left := 0
		for _, entry := range omitted {
			left += entry.Size
		}
		note = fmt.Sprintf("%d of %d files; %d left out (%s), listed in OMITTED.txt", kept, kept+len(omitted), len(omitted), archiveSize(left))
	}
	return archiveStoredFile{Name: name, Local: local, Size: packed.Len(), Repo: repo.label(), Note: note}, nil
}

func archiveGetJSON(fetcher *archiveFetcher, target *url.URL, value any) error {
	body, _, _, err := fetcher.get(target, maxArchiveResponseBytes)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, value)
}

// archiveCopyRepoZip copies GitHub's zip of the whole repository, without
// its top folder.
func archiveCopyRepoZip(fetcher *archiveFetcher, writer *zip.Writer, repo archiveRepo, limit int) (int, int, error) {
	codeload, err := url.Parse(githubCodeload)
	if err != nil {
		return 0, 0, err
	}
	body, _, _, err := fetcher.get(codeload.JoinPath(repo.Owner, repo.Name, "zip", repo.Ref), limit)
	if err != nil {
		return 0, 0, err
	}
	reader, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return 0, 0, err
	}
	kept := 0
	for _, file := range reader.File {
		_, name, _ := strings.Cut(file.Name, "/")
		if name == "" || strings.HasSuffix(name, "/") || archiveRepoSkippedPath(name) {
			continue
		}
		header := file.FileHeader
		header.Name = name
		raw, err := file.OpenRaw()
		if err != nil {
			return 0, 0, err
		}
		entry, err := writer.CreateRaw(&header)
		if err != nil {
			return 0, 0, err
		}
		if _, err := io.Copy(entry, raw); err != nil {
			return 0, 0, err
		}
		kept++
	}
	return len(body), kept, nil
}

// archiveRepoUsesLFS reports whether a .gitattributes file stores files
// in Git LFS.
func archiveRepoUsesLFS(fetcher *archiveFetcher, repo archiveRepo, name string) bool {
	raw, err := url.Parse(githubRaw)
	if err != nil {
		return false
	}
	segments := append([]string{repo.Owner, repo.Name, repo.Ref}, strings.Split(path.Join(repo.Dir, name), "/")...)
	body, _, _, err := fetcher.get(raw.JoinPath(segments...), 1<<20)
	return err == nil && bytes.Contains(body, []byte("filter=lfs"))
}

// archiveFetchRepoFiles adds the selected files one by one, up to limit
// bytes, fetching LFS files in place of their stubs; any that fail or do not
// fit join the omitted ones.
func archiveFetchRepoFiles(fetcher *archiveFetcher, writer *zip.Writer, repo archiveRepo, selected, omitted []githubTreeEntry, limit int) (int, int, []githubTreeEntry) {
	raw, err := url.Parse(githubRaw)
	if err != nil {
		return 0, 0, append(omitted, selected...)
	}
	media, err := url.Parse(githubMedia)
	if err != nil {
		return 0, 0, append(omitted, selected...)
	}
	size, kept := 0, 0
	for index, entry := range selected {
		if fetcher.ctx.Err() != nil || index >= archiveRepoMaxFiles {
			omitted = append(omitted, selected[index:]...)
			break
		}
		segments := append([]string{repo.Owner, repo.Name, repo.Ref}, strings.Split(path.Join(repo.Dir, entry.Path), "/")...)
		body, _, _, err := fetcher.get(raw.JoinPath(segments...), archiveRepoFileMax)
		if err == nil && bytes.HasPrefix(body, githubLFSPointer) {
			entry.Size = archiveLFSSize(body)
			if entry.Size > archiveRepoFileMax || size+entry.Size > limit {
				omitted = append(omitted, entry)
				continue
			}
			body, _, _, err = fetcher.get(media.JoinPath(segments...), archiveRepoFileMax)
		}
		if err == nil {
			err = archiveZipBytes(writer, entry.Path, body)
		}
		if err != nil {
			omitted = append(omitted, entry)
			continue
		}
		size += len(body)
		kept++
	}
	return size, kept, omitted
}

// archiveLFSSize reads the size of the file an LFS stub stands for.
func archiveLFSSize(pointer []byte) int {
	for _, line := range strings.Split(string(pointer), "\n") {
		if value, ok := strings.CutPrefix(line, "size "); ok {
			if size, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				return size
			}
		}
	}
	return archiveRepoFileMax + 1
}

// archiveFetchReleaseFiles adds the latest release's design and firmware
// files that fit in budget, and reports their size.
func archiveFetchReleaseFiles(fetcher *archiveFetcher, writer *zip.Writer, api *url.URL, budget int) int {
	var release struct {
		Assets []struct {
			Name string `json:"name"`
			Size int    `json:"size"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if archiveGetJSON(fetcher, api.JoinPath("releases", "latest"), &release) != nil {
		return 0
	}
	size := 0
	for _, asset := range release.Assets {
		extension := strings.ToLower(path.Ext(asset.Name))
		if !archiveFileExtensions[extension] && archiveRepoClass(asset.Name) != 2 || asset.Size > min(archiveRepoFileMax, budget-size) {
			continue
		}
		target, err := url.Parse(asset.URL)
		if err != nil {
			continue
		}
		body, _, _, err := fetcher.get(target, archiveRepoFileMax)
		if err != nil || !archiveFileMatches(body, asset.Name) || archiveZipBytes(writer, "releases/"+archiveFileName(asset.Name), body) != nil {
			continue
		}
		size += len(body)
	}
	return size
}

func archiveZipBytes(writer *zip.Writer, name string, body []byte) error {
	entry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
	if err != nil {
		return err
	}
	_, err = entry.Write(body)
	return err
}

func archiveWriteOmitted(writer *zip.Writer, omitted []githubTreeEntry) {
	slices.SortFunc(omitted, func(a, b githubTreeEntry) int { return strings.Compare(a.Path, b.Path) })
	var list strings.Builder
	list.WriteString("Files of the repository left out of this archive, to keep it small.\n\n")
	for _, entry := range omitted {
		fmt.Fprintf(&list, "%s\t%s\n", archiveSize(entry.Size), entry.Path)
	}
	archiveZipBytes(writer, "OMITTED.txt", []byte(list.String()))
}

// archiveRepoSkippedPath names tooling and vendored code a build does not
// need from the repository.
func archiveRepoSkippedPath(name string) bool {
	for _, segment := range strings.Split(name, "/") {
		switch strings.ToLower(segment) {
		case "node_modules", "vendor", "third_party", "thirdparty", "__pycache__", "cmsis":
			return true
		}
		// A microcontroller vendor's driver library, copied into the project.
		if strings.HasSuffix(strings.ToLower(segment), "_hal_driver") {
			return true
		}
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	return false
}

var archiveRepoClasses = func() map[string]int {
	classes := make(map[string]int)
	for class, extensions := range [][]string{
		// Docs: the build guide.
		{".md", ".markdown", ".rst", ".adoc", ".pdf"},
		// Design files, PCBs and firmware source.
		{".stl", ".3mf", ".step", ".stp", ".obj", ".dxf", ".svg", ".scad", ".f3d", ".f3z", ".fcstd", ".iges", ".igs", ".skp", ".gcode",
			".sldprt", ".sldasm", ".slddrw", ".ipt", ".iam", ".x_t", ".3dm",
			".pcbdoc", ".schdoc", ".prjpcb", ".pcblib", ".schlib", ".outjob",
			".kicad_pcb", ".kicad_sch", ".kicad_pro", ".kicad_sym", ".kicad_mod", ".kicad_dru", ".kicad_wks", ".sch", ".brd", ".lib", ".dcm", ".net",
			".gbr", ".gtl", ".gbl", ".gto", ".gbo", ".gts", ".gbs", ".gtp", ".gbp", ".gko", ".gm1", ".gml", ".drl", ".xln", ".pos", ".csv", ".xlsx", ".ods",
			".ino", ".c", ".h", ".cpp", ".hpp", ".cc", ".py", ".rs", ".go", ".js", ".ts", ".sh", ".json", ".yaml", ".yml", ".toml", ".ini", ".cfg", ".conf",
			".keymap", ".overlay", ".dts", ".dtsi", ".mk", ".ld", ".cmake", ".txt"},
		// Firmware builds and packed files.
		{".uf2", ".hex", ".bin", ".elf", ".dfu", ".zip", ".7z"},
		// Pictures the docs show.
		{".png", ".jpg", ".jpeg", ".webp", ".gif"},
	} {
		for _, extension := range extensions {
			classes[extension] = class
		}
	}
	for _, name := range []string{"makefile", "cmakelists.txt", "kconfig", "license", "licence", "copying", "readme"} {
		classes[name] = 0
	}
	return classes
}()

// archiveRepoClass ranks a file by how much a build needs it, or -1 for
// files it does not.
func archiveRepoClass(name string) int {
	base := strings.ToLower(path.Base(name))
	if class, ok := archiveRepoClasses[base]; ok {
		return class
	}
	if class, ok := archiveRepoClasses[path.Ext(base)]; ok {
		return class
	}
	return -1
}

// archiveRepoSelection keeps the files a build needs, docs first, then
// design files and source, firmware builds and pictures, the top folders'
// and smaller files first within each, up to limit bytes.
func archiveRepoSelection(blobs []githubTreeEntry, limit int) (selected, omitted []githubTreeEntry) {
	ranked := slices.Clone(blobs)
	slices.SortStableFunc(ranked, func(a, b githubTreeEntry) int {
		return cmp.Or(cmp.Compare(archiveRepoRank(a), archiveRepoRank(b)),
			cmp.Compare(strings.Count(a.Path, "/"), strings.Count(b.Path, "/")), cmp.Compare(a.Size, b.Size))
	})
	total := 0
	for _, entry := range ranked {
		class := archiveRepoClass(entry.Path)
		if class < 0 || class == 3 && entry.Size > archiveRepoImageMax || entry.Size > archiveRepoFileMax ||
			total+entry.Size > limit || len(selected) >= archiveRepoMaxFiles {
			omitted = append(omitted, entry)
			continue
		}
		total += entry.Size
		selected = append(selected, entry)
	}
	return selected, omitted
}

// archiveRepoLater names folders of earlier versions, tests and component
// libraries, kept only when there is room after the project's own files.
var archiveRepoLater = map[string]bool{
	"old": true, "legacy": true, "archive": true, "archived": true, "deprecated": true, "obsolete": true,
	"unused": true, "backup": true, "test": true, "tests": true,
	"lib": true, "libs": true, "library": true, "libraries": true, "footprints": true, "symbols": true, "packages3d": true,
}

// archiveRepoFabrication matches packed files named for making the thing:
// gerbers, fabrication outputs, print files.
var archiveRepoFabrication = regexp.MustCompile(`(?i)gerber|fab|production|manufactur|jlc|pcbway|stl|step|cad|print`)

func archiveRepoRank(entry githubTreeEntry) int {
	class := archiveRepoClass(entry.Path)
	if class < 0 {
		return 8
	}
	if class == 2 && archiveRepoFabrication.MatchString(path.Base(entry.Path)) {
		class = 1
	}
	for _, segment := range strings.Split(path.Dir(entry.Path), "/") {
		segment = strings.ToLower(segment)
		if archiveRepoLater[segment] || strings.HasPrefix(segment, "old_") || strings.HasSuffix(segment, "_old") ||
			strings.HasSuffix(segment, ".pretty") || strings.HasSuffix(segment, ".3dshapes") {
			return class + 4
		}
	}
	return class
}
