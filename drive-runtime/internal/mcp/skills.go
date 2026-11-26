package mcp

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/search"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
)

// Skills are folders of markdown under skills/ that tell an agent how to do
// something on this drive (a SKILL.md and the files it points to), and
// quickstarts of the JS libraries under lib/js/<recipe>/docs/. The host
// indexes both in search.db, one article per "##" section, where the search
// tool leaves them alone. These two tools search and read only them, from
// the index. Nothing here runs what a skill holds.

const (
	skillKind   = "skill"
	libraryKind = "library"

	defaultSkillLimit = 5
	maxSkillLimit     = 20

	// skillCandidates is how many matching sections are grouped into
	// results. A long skill matches with many sections; this many leaves room
	// for the others behind it.
	skillCandidates = 200
	// hybridSkillCandidates bounds the same for hybrid search, whose
	// diversity pass compares every candidate with every result kept.
	hybridSkillCandidates = 50

	// skillDescriptionChars caps a description in search results.
	skillDescriptionChars = 300
	// skillFilesListed caps the files a read lists, and skillFilesWalked the
	// entries looked at to find them.
	skillFilesListed = 40
	skillFilesWalked = 2000
	// skillNamesListed caps the names an empty search lists per kind.
	skillNamesListed = 20
	// frontmatterBytes is how much of a file is read for its frontmatter.
	frontmatterBytes = 64 * 1024
	// byteOrderMark starts some files written on Windows.
	byteOrderMark = string(rune(0xFEFF))
)

// skillServerInstructions is what the drive's server tells a client that
// connects to a drive with skills.
const skillServerInstructions = "Procedures for building things on this drive (apps, dashboards, maps, documents, firmware) are found with skill_search and read with skill_read. Read the skill before starting."

// noSkillIndex answers both tools on a drive without a search index.
const noSkillIndex = "This vault has no search index, so its skills and library quickstarts cannot be searched or read with this tool. They are files on the drive under skills/ and lib/js/. The computer that manages the drive builds the index with 'svalbard index'."

// SkillCapability exposes the skill tools: skill_search and skill_read.
type SkillCapability struct {
	driveRoot string
	meta      DriveMetadata

	mu  sync.Mutex
	db  *sql.DB
	eng *engine.Engine

	// embedBackend embeds queries for hybrid search. The drive's server
	// shares the search tool's, so one embedding model is loaded, not two.
	embedBackend *search.SemanticBackend
	ownsBackend  bool
}

// NewSkillCapability creates the skill tools for the given drive.
func NewSkillCapability(driveRoot string, meta DriveMetadata) *SkillCapability {
	return &SkillCapability{driveRoot: driveRoot, meta: meta, embedBackend: search.NewSemanticBackend(driveRoot), ownsBackend: true}
}

// newSharedSkillCapability is NewSkillCapability with the embedding backend
// of another capability, which stays responsible for closing it.
func newSharedSkillCapability(driveRoot string, meta DriveMetadata, backend *search.SemanticBackend) *SkillCapability {
	return &SkillCapability{driveRoot: driveRoot, meta: meta, embedBackend: backend}
}

func (c *SkillCapability) Tool() string { return "skill" }

func (c *SkillCapability) Actions() []ActionDef {
	return []ActionDef{
		{
			Name: "search",
			Desc: "Find a procedure (a skill) or a JS library's quickstart on this drive, for building something: an app, dashboard, map, document, firmware. Say what you want to make in a few words. Each result is one skill or library; pass its id to skill_read and follow what it says. Not for facts: use search.",
			Params: []ParamDef{
				{Name: "query", Type: "string", Required: true, Desc: "What to build or do, for example: chart from a CSV file"},
				{Name: "kind", Type: "string", Desc: "Only skills (procedures) or only libraries", Enum: []string{skillKind, libraryKind}},
				{Name: "limit", Type: "integer", Desc: "Maximum results, 1 to 20", Default: defaultSkillLimit},
			},
		},
		{
			Name: "read",
			Desc: "Read a skill or a library quickstart found with skill_search. A long one comes in parts: when the result has \"continues\", call again with the offset it names. \"files\" lists the skill's other files: pass a markdown one as file to read it; templates and scripts are files on the drive below the skill's id.",
			Params: []ParamDef{
				{Name: "id", Type: "string", Required: true, Desc: "The id from skill_search"},
				{Name: "file", Type: "string", Desc: "A markdown file of the skill, as listed in \"files\""},
				{Name: "section", Type: "string", Desc: "Only one section, by its heading or number"},
				{Name: "offset", Type: "integer", Desc: "Character to start from, for the next part", Default: 0},
			},
		},
	}
}

func (c *SkillCapability) Handle(ctx context.Context, action string, params map[string]any) (ActionResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return ActionResult{}, err
	}
	switch action {
	case "search":
		return c.handleSearch(ctx, params)
	case "read":
		return c.handleRead(ctx, params)
	default:
		return ActionResult{}, fmt.Errorf("unknown skill action: %s", action)
	}
}

func (c *SkillCapability) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db != nil {
		c.db.Close()
		c.db, c.eng = nil, nil
	}
	if c.ownsBackend && c.embedBackend != nil {
		_ = c.embedBackend.Close()
	}
	return nil
}

// getDB opens the index on first use. A failure is not kept: the index may
// be built while the server runs.
func (c *SkillCapability) getDB() (*sql.DB, *engine.Engine, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.db == nil {
		db, err := openSearchDB(c.driveRoot)
		if err != nil {
			return nil, nil, err
		}
		c.db, c.eng = db, engine.New(db)
	}
	return c.db, c.eng, nil
}

// SkillResult is one skill or library in a search: the JSON a skill_search
// returns an array of.
type SkillResult struct {
	// ID is what skill_read takes: the skill's folder on the drive, or the
	// path of a library's quickstart.
	ID          string `json:"id"`
	Kind        string `json:"kind"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	// File is set when the best match is in a file beside SKILL.md.
	File string `json:"file,omitempty"`
	// HasScripts says the skill brings scripts to run, under scripts/.
	HasScripts  bool     `json:"has_scripts,omitempty"`
	License     string   `json:"license,omitempty"`
	Version     string   `json:"version,omitempty"`
	Portability string   `json:"portability,omitempty"`
	Requires    []string `json:"requires,omitempty"`
}

func (c *SkillCapability) handleSearch(ctx context.Context, params map[string]any) (ActionResult, error) {
	query := strings.TrimSpace(getString(params, "query"))
	if query == "" {
		return ActionResult{}, fmt.Errorf("missing required parameter: query")
	}
	scope, kind := engine.SkillScope, getString(params, "kind")
	switch kind {
	case "":
	case skillKind:
		scope = engine.SkillsDir + "/"
	case libraryKind:
		scope = engine.LibraryDocsDir + "/"
	default:
		return ActionResult{}, fmt.Errorf("unknown kind %q: use skill or library, or leave it out for both", kind)
	}
	limit := min(max(intParam(params, "limit", defaultSkillLimit), 1), maxSkillLimit)

	db, eng, err := c.getDB()
	if err != nil {
		return ActionResult{Text: noSkillIndex}, nil
	}
	catalog, err := loadSkillCatalog(ctx, eng)
	if err != nil {
		return ActionResult{}, err
	}
	if len(catalog.entries) == 0 {
		return ActionResult{Text: "The search index of this drive holds no skills or library quickstarts. If there are files under skills/ or lib/js/, the computer that manages the drive indexes them with 'svalbard index'."}, nil
	}
	hits, err := c.matchingSections(ctx, db, eng, query, scope)
	if err != nil {
		return ActionResult{}, err
	}

	// One result per skill or library: its best-ranked section stands for it.
	var items []SkillResult
	seen := map[string]bool{}
	for _, hit := range hits {
		entry, file := catalog.entryOf(hit.Filename)
		if seen[entry.ID] {
			continue
		}
		seen[entry.ID] = true
		item := c.describe(ctx, eng, entry)
		item.File = file
		if items = append(items, item); len(items) == limit {
			break
		}
	}
	if len(items) == 0 {
		return ActionResult{Text: "Nothing found. " + catalog.holdings(kind)}, nil
	}
	return ActionResult{Data: items}, nil
}

// matchingSections ranks the sections in scope: by hybrid search when the
// index was built for it and the query can be embedded, by keyword otherwise.
func (c *SkillCapability) matchingSections(ctx context.Context, db *sql.DB, eng *engine.Engine, query, scope string) ([]engine.Result, error) {
	if contract, complete := search.LoadCompleteContract(db); complete {
		if vector, err := c.embedBackend.Embed(ctx, contract, query); err == nil {
			results, usable, err := search.HybridSnapshot(ctx, db, contract, query, vector, hybridSkillCandidates, scope)
			if err == nil && usable && len(results) > 0 {
				return results, ctx.Err()
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	matches, err := eng.Search(ctx, query, engine.KeywordOptions{Source: scope, Limit: skillCandidates, AnyWord: true})
	if err != nil {
		return nil, fmt.Errorf("skill search failed: %w", err)
	}
	return matches.Results, ctx.Err()
}

// describe fills a result from the entry file's frontmatter and the skill's
// folder as they are on the drive now. The index holds only the text: a file
// that is gone leaves the fields empty and the index still names the skill.
func (c *SkillCapability) describe(ctx context.Context, eng *engine.Engine, entry skillEntry) SkillResult {
	front := readFrontmatter(filepath.Join(c.driveRoot, filepath.FromSlash(entry.File)))
	item := SkillResult{
		ID:          entry.ID,
		Kind:        entry.Kind,
		Title:       firstNonEmpty(front.Name, entry.Title, path.Base(entry.ID)),
		Description: opening(strings.Join(strings.Fields(front.Description), " "), skillDescriptionChars),
		License:     front.License,
		Version:     front.Version,
		Portability: front.Portability,
		Requires:    front.Requires,
	}
	if item.Description == "" {
		if sections, err := eng.Sections(ctx, entry.File); err == nil && len(sections) > 0 {
			item.Description = opening(strings.Join(strings.Fields(sections[0].Text), " "), skillDescriptionChars)
		}
	}
	if entry.Folder != "" {
		scripts, _ := os.ReadDir(filepath.Join(c.driveRoot, filepath.FromSlash(entry.Folder), "scripts"))
		for _, script := range scripts {
			item.HasScripts = item.HasScripts || !strings.HasPrefix(script.Name(), ".")
		}
	}
	return item
}

// SkillReadResult is the JSON shape of a skill_read.
type SkillReadResult struct {
	ID string `json:"id"`
	// File is set when the text is a file beside the entry file.
	File  string `json:"file,omitempty"`
	Title string `json:"title"`
	Body  string `json:"body"`
	// Continues tells the caller how to get the next part.
	Continues string `json:"continues,omitempty"`
	// NextOffset is the offset Continues names, 0 when nothing follows.
	NextOffset int `json:"-"`
	// Sections names the headings of a file that did not fit in one part.
	Sections []string `json:"sections,omitempty"`
	// Files are the skill's other files, relative to its folder.
	Files []string `json:"files,omitempty"`
	Note  string   `json:"note,omitempty"`
}

func (c *SkillCapability) handleRead(ctx context.Context, params map[string]any) (ActionResult, error) {
	id, err := cleanSkillPath(getString(params, "id"))
	if err != nil || id == "" {
		return ActionResult{}, fmt.Errorf("skill_read needs the id of a skill or library from skill_search, for example skills/skills-core/data-dashboard")
	}
	_, eng, err := c.getDB()
	if err != nil {
		return ActionResult{Text: noSkillIndex}, nil
	}
	catalog, err := loadSkillCatalog(ctx, eng)
	if err != nil {
		return ActionResult{}, err
	}
	entry, err := catalog.resolve(id)
	if err != nil {
		return ActionResult{}, err
	}

	result := SkillReadResult{ID: entry.ID, Title: firstNonEmpty(entry.Title, path.Base(entry.ID))}
	file := entry.File
	if requested := getString(params, "file"); requested != "" {
		rel, err := cleanSkillPath(requested)
		if err != nil || rel == "" || strings.HasPrefix(requested, "/") {
			return ActionResult{}, fmt.Errorf("file %q is not a file of %s: name one from \"files\"", requested, entry.ID)
		}
		if entry.Folder == "" {
			return ActionResult{}, fmt.Errorf("%s is one file and has no others", entry.ID)
		}
		if target := entry.Folder + "/" + rel; target != entry.File {
			title, indexed := catalog.titles[target]
			if !indexed {
				return ActionResult{}, c.notIndexedError(entry, target)
			}
			file, result.File, result.Title = target, rel, firstNonEmpty(title, path.Base(target))
		}
	}

	offset := max(intParam(params, "offset", 0), 0)
	text, err := readMarkdownSource(ctx, eng, file, params["section"])
	if err != nil {
		return ActionResult{}, err
	}
	body, next, total := pageSlice(text.body, offset, readChars)
	if offset > 0 && offset >= total {
		return ActionResult{}, fmt.Errorf("offset %d is past the end (%d characters)", offset, total)
	}
	result.Body = body
	if next < total {
		result.Continues = fmt.Sprintf("Characters %d to %d of %d. Call skill_read again with the same arguments and offset %d for the next part.", offset, next, total, next)
		result.NextOffset = next
		if offset == 0 && !text.oneSection {
			result.Sections = text.headings
		}
	}
	if offset == 0 && file == entry.File && entry.Folder != "" && !text.oneSection {
		var more int
		result.Files, more = c.otherFiles(entry)
		if more > 0 {
			result.Note = fmt.Sprintf("%d more files are not listed; they are on the drive below %s.", more, entry.Folder)
		}
	}
	return ActionResult{Data: result}, nil
}

// notIndexedError explains a file of a skill that skill_read cannot return.
func (c *SkillCapability) notIndexedError(entry skillEntry, target string) error {
	if info, err := os.Stat(filepath.Join(c.driveRoot, filepath.FromSlash(target))); err == nil && !info.IsDir() {
		return fmt.Errorf("%s is not markdown, so skill_read does not hold it. It is a file on the drive: %s", path.Base(target), target)
	}
	files, _ := c.otherFiles(entry)
	if len(files) == 0 {
		return fmt.Errorf("%s has no file %q", entry.ID, strings.TrimPrefix(target, entry.Folder+"/"))
	}
	return fmt.Errorf("%s has no file %q. Its files: %s", entry.ID, strings.TrimPrefix(target, entry.Folder+"/"), strings.Join(files, ", "))
}

// otherFiles lists the files of a skill's folder beside its entry file, as
// they are on the drive, and how many more there are than it lists. A folder
// that is gone has none.
func (c *SkillCapability) otherFiles(entry skillEntry) (files []string, more int) {
	base := filepath.Join(c.driveRoot, filepath.FromSlash(entry.Folder))
	walked := 0
	_ = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if walked++; walked > skillFilesWalked {
			return filepath.SkipAll
		}
		if p != base && strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(base, p)
		if err != nil {
			return nil
		}
		if rel = filepath.ToSlash(rel); entry.Folder+"/"+rel != entry.File {
			files = append(files, rel)
		}
		return nil
	})
	sort.Strings(files)
	if len(files) > skillFilesListed {
		files, more = files[:skillFilesListed], len(files)-skillFilesListed
	}
	return files, more
}

// cleanSkillPath tidies a path a model gave: no surrounding space or slashes.
// One that climbs out with "..", or is written with backslashes, is refused.
func cleanSkillPath(given string) (string, error) {
	given = strings.Trim(strings.TrimSpace(given), "/")
	if given == "" {
		return "", nil
	}
	if strings.ContainsAny(given, "\\\x00") {
		return "", fmt.Errorf("invalid path %q", given)
	}
	for _, segment := range strings.Split(given, "/") {
		if segment == ".." {
			return "", fmt.Errorf("invalid path %q", given)
		}
	}
	return path.Clean(given), nil
}

// skillEntry is a skill or a library quickstart as agents name it.
type skillEntry struct {
	// ID is the skill's folder (skills/skills-core/data-dashboard) or the
	// quickstart's path (lib/js/js-core/docs/chartjs.md).
	ID   string
	Kind string
	// File is the indexed file that describes it: SKILL.md or the quickstart.
	File  string
	Title string
	// Folder is the skill's folder on the drive; a quickstart has none.
	Folder string
}

// skillCatalog is what the index holds of skills and library quickstarts.
type skillCatalog struct {
	entries []skillEntry
	// folders maps a skill's folder to its entry in entries.
	folders map[string]int
	// titles maps every indexed skill file to the title it gives itself.
	titles map[string]string
}

func loadSkillCatalog(ctx context.Context, eng *engine.Engine) (*skillCatalog, error) {
	sources, err := eng.SkillSources(ctx)
	if err != nil {
		return nil, err
	}
	catalog := &skillCatalog{folders: map[string]int{}, titles: make(map[string]string, len(sources))}
	for _, source := range sources {
		catalog.titles[source.Filename] = source.Title
		switch {
		case strings.HasPrefix(source.Filename, engine.LibraryDocsDir+"/"):
			catalog.entries = append(catalog.entries, skillEntry{ID: source.Filename, Kind: libraryKind, File: source.Filename, Title: source.Title})
		case strings.EqualFold(path.Base(source.Filename), "SKILL.md"):
			folder := path.Dir(source.Filename)
			catalog.folders[folder] = len(catalog.entries)
			catalog.entries = append(catalog.entries, skillEntry{ID: folder, Kind: skillKind, File: source.Filename, Title: source.Title, Folder: folder})
		}
	}
	// Markdown under skills/ that no SKILL.md is above stands for itself.
	for _, source := range sources {
		if entry, _ := catalog.entryOf(source.Filename); entry.ID == source.Filename && entry.Kind == skillKind {
			catalog.entries = append(catalog.entries, entry)
		}
	}
	return catalog, nil
}

// entryOf returns the skill or library an indexed file belongs to: for a
// file inside a skill's folder, the nearest folder above it with a SKILL.md.
// file is its path within that folder when it is not the entry file itself.
func (c *skillCatalog) entryOf(filename string) (entry skillEntry, file string) {
	if strings.HasPrefix(filename, engine.LibraryDocsDir+"/") {
		return skillEntry{ID: filename, Kind: libraryKind, File: filename, Title: c.titles[filename]}, ""
	}
	for folder := path.Dir(filename); folder != engine.SkillsDir && folder != "." && folder != "/"; folder = path.Dir(folder) {
		if at, ok := c.folders[folder]; ok {
			entry = c.entries[at]
			if filename != entry.File {
				file = strings.TrimPrefix(filename, folder+"/")
			}
			return entry, file
		}
	}
	return skillEntry{ID: filename, Kind: skillKind, File: filename, Title: c.titles[filename]}, ""
}

// resolve finds the entry an id names. The id skill_search gave always
// works; so do the forms a model hands back: the SKILL.md path, another
// case, or only the name of the skill or library when one alone has it.
func (c *skillCatalog) resolve(id string) (skillEntry, error) {
	var named []skillEntry
	for _, entry := range c.entries {
		if strings.EqualFold(entry.ID, id) || strings.EqualFold(entry.File, id) {
			return entry, nil
		}
		base := path.Base(entry.ID)
		if strings.EqualFold(base, id) || strings.EqualFold(strings.TrimSuffix(base, path.Ext(base)), id) || strings.EqualFold(entry.Title, id) {
			named = append(named, entry)
		}
	}
	switch {
	case len(named) == 1:
		return named[0], nil
	case len(named) > 1:
		ids := make([]string, len(named))
		for i, entry := range named {
			ids[i] = entry.ID
		}
		return skillEntry{}, fmt.Errorf("%q names more than one: %s. Pass one of these ids.", id, strings.Join(ids, ", "))
	}
	return skillEntry{}, fmt.Errorf("unknown skill or library %q. Find one with skill_search and pass its id.", id)
}

// holdings names the skills and libraries the index has, for a search that
// found none: a model that sees the list stops looking for what is not there.
func (c *skillCatalog) holdings(kind string) string {
	var parts []string
	for _, group := range [][2]string{{skillKind, "Skills"}, {libraryKind, "Libraries"}} {
		if kind != "" && kind != group[0] {
			continue
		}
		var names []string
		for _, entry := range c.entries {
			if entry.Kind == group[0] {
				names = append(names, firstNonEmpty(entry.Title, path.Base(entry.ID)))
			}
		}
		if len(names) == 0 {
			continue
		}
		more := ""
		if extra := len(names) - skillNamesListed; extra > 0 {
			names, more = names[:skillNamesListed], fmt.Sprintf(", and %d more", extra)
		}
		parts = append(parts, group[1]+" on this drive: "+strings.Join(names, ", ")+more+".")
	}
	if len(parts) == 0 {
		return "This drive has none of that kind."
	}
	return strings.Join(parts, " ")
}

// markdownText is an indexed markdown file, or one section of it, as text.
type markdownText struct {
	body string
	// title is the section's own title when one section was asked for.
	title string
	// number is that section's number, 0 for the whole file.
	number int
	// headings are the file's section headings, in order.
	headings   []string
	oneSection bool
}

// readMarkdownSource puts an indexed skill file back together from its
// sections. section narrows it to one: a number, "section/N", or a heading.
func readMarkdownSource(ctx context.Context, eng *engine.Engine, filename string, section any) (markdownText, error) {
	sections, err := eng.Sections(ctx, filename)
	if err != nil {
		return markdownText{}, err
	}
	if len(sections) == 0 {
		return markdownText{}, fmt.Errorf("%s has no text in the search index", filename)
	}
	// A section's title is the file's title and its heading: "name, Steps".
	// A long section continues under the same heading, marked.
	headingOf := func(title string) string {
		if _, heading, ok := strings.Cut(title, ", "); ok {
			return heading
		}
		return title
	}
	var text markdownText
	for _, part := range sections {
		if heading := headingOf(part.Title); !strings.HasSuffix(heading, " (cont.)") {
			text.headings = append(text.headings, heading)
		}
	}

	wanted, isNumber := sectionNumber(section)
	name, _ := section.(string)
	name = strings.TrimSpace(name)
	if !isNumber && name == "" {
		bodies := make([]string, len(sections))
		for i, part := range sections {
			bodies[i] = part.Text
		}
		text.body = strings.Join(bodies, "\n\n")
		return text, nil
	}

	text.oneSection = true
	var bodies []string
	for _, part := range sections {
		heading := headingOf(part.Title)
		if isNumber && part.Number == wanted || !isNumber && strings.EqualFold(strings.TrimSuffix(heading, " (cont.)"), name) {
			if len(bodies) == 0 {
				text.title, text.number = part.Title, part.Number
			}
			bodies = append(bodies, part.Text)
		}
	}
	if len(bodies) == 0 {
		return markdownText{}, fmt.Errorf("%s has no section %q. Its sections: %s", filename, fmt.Sprint(section), strings.Join(text.headings, ", "))
	}
	text.body = strings.Join(bodies, "\n\n")
	return text, nil
}

// sectionNumber reads a section given by number: 2, "2" or "section/2".
func sectionNumber(section any) (int, bool) {
	switch value := section.(type) {
	case float64:
		return int(value), value >= 1
	case int:
		return value, value >= 1
	case string:
		value = strings.TrimPrefix(strings.TrimLeft(strings.ToLower(strings.TrimSpace(value)), "/"), "section/")
		number, err := strconv.Atoi(value)
		return number, err == nil && number >= 1
	}
	return 0, false
}

// readSkillSource serves search_read for a skill file or library quickstart,
// which a search scoped to the skills returns like any other source: one
// section by its path, or the whole file without one.
func (c *SearchCapability) readSkillSource(ctx context.Context, source, rawPath string, offset int) (ActionResult, error) {
	_, eng, err := c.getDB()
	if err != nil {
		return ActionResult{}, err
	}
	var section any
	if rawPath = strings.TrimSpace(rawPath); rawPath != "" {
		if _, ok := sectionNumber(rawPath); !ok {
			return ActionResult{}, fmt.Errorf("%s is read by section: %q is not one. Use a path like section/2, or omit path for the whole file.", source, rawPath)
		}
		section = rawPath
	}
	text, err := readMarkdownSource(ctx, eng, source, section)
	if err != nil {
		return ActionResult{}, err
	}
	body, next, total := pageSlice(text.body, offset, readChars)
	if offset > 0 && offset >= total {
		return ActionResult{}, fmt.Errorf("offset %d is past the end (%d characters)", offset, total)
	}
	result := readResult{Source: source, Title: text.title, Body: body, File: source}
	if text.oneSection {
		result.Path = engine.SectionPath(text.number)
	} else {
		sources, _ := eng.SkillSources(ctx)
		for _, indexed := range sources {
			if indexed.Filename == source {
				result.Title = indexed.Title
			}
		}
	}
	if next < total {
		result.Continues = fmt.Sprintf("Characters %d to %d of %d. Call search_read with the same source and path and offset %d for the next part.", offset, next, total, next)
	}
	return ActionResult{Data: result}, nil
}

// frontmatter is what a skill's or quickstart's leading "---" block says
// about it.
type frontmatter struct {
	Name, Description, License, Version, Portability string
	Requires                                         []string
}

// readFrontmatter reads the frontmatter of a markdown file on the drive. A
// file that is missing, or has none, gives the zero value. It reads the
// plain shapes skills are written in and is not a YAML parser: one-line
// values, a folded or literal block for a long description, a list inline or
// one item a line, and version and license below "metadata:".
func readFrontmatter(file string) frontmatter {
	var front frontmatter
	f, err := os.Open(file)
	if err != nil {
		return front
	}
	defer f.Close()
	scanner := bufio.NewScanner(io.LimitReader(f, frontmatterBytes))
	scanner.Buffer(make([]byte, 0, 64*1024), frontmatterBytes)
	if !scanner.Scan() || strings.TrimRight(strings.TrimPrefix(scanner.Text(), byteOrderMark), " \t\r") != "---" {
		return front
	}
	fields := map[string]*string{
		"name": &front.Name, "description": &front.Description, "license": &front.License,
		"version": &front.Version, "portability": &front.Portability,
	}
	// block is the field a folded value or a list is being gathered for.
	var block *string
	var list *[]string
	parent := ""
	closed := false
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), " \t\r")
		if line == "---" || line == "..." {
			closed = true
			break
		}
		indented := strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if item, ok := strings.CutPrefix(trimmed, "- "); ok && list != nil {
			*list = append(*list, unquote(item))
			continue
		}
		if indented && block != nil {
			*block = strings.TrimSpace(*block + " " + trimmed)
			continue
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		block, list = nil, nil
		if indented {
			// One level of nesting is read: version and license of metadata.
			if target := fields[key]; parent == "metadata" && target != nil && *target == "" && (key == "version" || key == "license") {
				*target = unquote(value)
			}
			continue
		}
		parent = key
		if key == "requires" {
			if inner, ok := strings.CutPrefix(value, "["); ok {
				for _, item := range strings.Split(strings.TrimSuffix(inner, "]"), ",") {
					if item = unquote(strings.TrimSpace(item)); item != "" {
						front.Requires = append(front.Requires, item)
					}
				}
			} else if value == "" {
				list = &front.Requires
			} else {
				front.Requires = []string{unquote(value)}
			}
			continue
		}
		target := fields[key]
		if target == nil {
			continue
		}
		if value == "" || strings.Trim(value, ">|+-") == "" {
			*target, block = "", target
			continue
		}
		*target = unquote(value)
	}
	if !closed {
		return frontmatter{}
	}
	return front
}

func unquote(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		return value[1 : len(value)-1]
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
