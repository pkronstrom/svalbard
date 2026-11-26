package engine

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

const (
	// maxQueryWords bounds the per-word queries a pasted paragraph can cause.
	maxQueryWords = 12

	// keywordRank weighs a word in the title four times one in the body, so
	// the article about a subject outranks those that mention it.
	keywordRank = "bm25(4.0, 1.0)"

	// pagesPerDocument is how many pages of one document a search returns
	// unless it is limited to that document. Every page's title carries the
	// document's title, so a query that names a document (or a subject a
	// document is named after) matches all of its pages in the weighted
	// column; unchecked, a 400-page manual fills every result and buries the
	// articles of other sources.
	pagesPerDocument = 3

	// rankedScan bounds the rows read past the limit while pages over
	// pagesPerDocument are skipped.
	rankedScan = 2000
)

// stopWords are dropped from a query so a question typed as a sentence still
// finds its subject. They are English only: in the any-word ranking a common
// word of another language already counts for little.
var stopWords = map[string]bool{
	"a": true, "about": true, "an": true, "and": true, "are": true, "as": true, "at": true,
	"be": true, "by": true, "can": true, "do": true, "does": true, "for": true, "from": true,
	"how": true, "i": true, "in": true, "is": true, "it": true, "of": true, "on": true,
	"or": true, "should": true, "that": true, "the": true, "this": true, "to": true,
	"was": true, "what": true, "when": true, "where": true, "which": true, "who": true,
	"why": true, "with": true,
}

// KeywordOptions narrows a keyword search.
type KeywordOptions struct {
	// Source is a search scope as InScope reads it: a filename from the
	// sources table, a folder ending in "/", SkillScope, or empty for every
	// knowledge source.
	Source string
	Limit  int
}

// skillRoots is the SQL condition that a source joined as src is a skill
// file or a library quickstart: IsSkillSource in SQL. GLOB compares case as
// Go does, and neither root holds a character GLOB reads as a pattern.
const skillRoots = `(src.filename GLOB '` + SkillsDir + `/*' OR src.filename GLOB '` + LibraryDocsDir + `/*')`

// scopeClause is the SQL condition for a search scope on the sources table
// joined as src, with its arguments. It reads a scope as InScope does.
func scopeClause(scope string) (string, []any) {
	switch {
	case scope == "":
		return ` AND NOT ` + skillRoots, nil
	case scope == SkillScope:
		return ` AND ` + skillRoots, nil
	case strings.HasSuffix(scope, "/"):
		// substr, not LIKE: a folder name may hold "%" or "_".
		return ` AND substr(src.filename, 1, length(?)) = ?`, []any{scope, scope}
	}
	return ` AND src.filename = ?`, []any{scope}
}

// KeywordMatches is the outcome of Search.
type KeywordMatches struct {
	// Results has articles containing every word first, then partial matches.
	Results []Result
	// Exact is how many leading results contain every word.
	Exact int
	// Words are the words searched for, in query order.
	Words []string
	// Missing are the words no article in the searched sources contains. It
	// is worked out only when the search did not fill its limit with complete
	// matches, which is when a caller needs to know why.
	Missing []string
	// MorePages names the documents that have further matching pages than
	// the pagesPerDocument returned; a search limited to one shows them.
	MorePages []string
}

// QueryWords returns the words a query is searched by: split on spaces and
// list punctuation, stripped of surrounding punctuation, without repeats and
// without stop words unless nothing else is left.
func QueryWords(query string) []string {
	fields := strings.FieldsFunc(query, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",;/|()[]{}", r)
	})
	seen := make(map[string]bool, len(fields))
	var words, content []string
	for _, field := range fields {
		word := strings.TrimFunc(field, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
		key := strings.ToLower(word)
		if word == "" || seen[key] {
			continue
		}
		seen[key] = true
		words = append(words, word)
		if !stopWords[key] {
			content = append(content, word)
		}
	}
	if len(content) > 0 {
		words = content
	}
	if len(words) > maxQueryWords {
		words = words[:maxQueryWords]
	}
	return words
}

// ftsTerm is one word as an FTS5 expression: the word itself or anything it
// begins. Naming the whole word as well lets an article that has it outrank
// one that only has a longer word ("lyme" before "lymecycline"). A plural
// loses its final "s" in the prefix, which then matches the singular too.
func ftsTerm(word string) string {
	quoted := strings.ReplaceAll(word, `"`, `""`)
	prefix := quoted
	if runes := []rune(prefix); len(runes) >= 5 && (runes[len(runes)-1] == 's' || runes[len(runes)-1] == 'S') {
		prefix = string(runes[:len(runes)-1])
	}
	return `("` + quoted + `" OR "` + prefix + `"*)`
}

func ftsTerms(words []string, operator string) string {
	terms := make([]string, len(words))
	for i, word := range words {
		terms[i] = ftsTerm(word)
	}
	return strings.Join(terms, operator)
}

// Search runs a keyword search that degrades instead of coming back empty.
// Articles containing every word come first. When they do not fill the limit,
// articles containing most of the words follow, those with the most words
// first and the title-weighted rank deciding ties.
func (e *Engine) Search(ctx context.Context, query string, opts KeywordOptions) (KeywordMatches, error) {
	matches := KeywordMatches{Words: QueryWords(query)}
	if len(matches.Words) == 0 || opts.Limit <= 0 {
		return matches, ctx.Err()
	}

	exact, more, err := e.ranked(ctx, ftsTerms(matches.Words, " AND "), opts.Source, opts.Limit)
	if err != nil {
		return matches, err
	}
	matches.MorePages = more
	matches.Results, matches.Exact = exact, len(exact)
	if len(exact) >= opts.Limit {
		return matches, ctx.Err()
	}
	if matches.Missing, err = e.missingWords(ctx, matches.Words, opts.Source); err != nil {
		return matches, err
	}
	// Two words have no "most": an article with one of them is a search for
	// that word alone.
	if len(matches.Words)-len(matches.Missing) < 2 || len(matches.Words) < 3 {
		return matches, ctx.Err()
	}

	// More candidates than are returned, so that an article with many of the
	// words can overtake one that repeats a single rare word.
	candidates, _, err := e.ranked(ctx, ftsTerms(matches.Words, " OR "), opts.Source, max(3*opts.Limit, 30))
	if err != nil {
		return matches, err
	}
	isExact := make(map[int]bool, len(exact))
	for _, result := range exact {
		isExact[result.ArticleID] = true
	}
	partial := candidates[:0]
	for _, result := range candidates {
		if !isExact[result.ArticleID] {
			partial = append(partial, result)
		}
	}
	if err := e.markMatchedWords(ctx, matches.Words, partial); err != nil {
		return matches, err
	}
	// An article with half of the words or fewer is about something else.
	enough := partial[:0]
	for _, result := range partial {
		if 2*len(result.Matched) > len(matches.Words) {
			enough = append(enough, result)
		}
	}
	partial = enough
	sort.SliceStable(partial, func(i, j int) bool { return len(partial[i].Matched) > len(partial[j].Matched) })
	if room := opts.Limit - len(exact); len(partial) > room {
		partial = partial[:room]
	}
	matches.Results = append(matches.Results, partial...)
	return matches, ctx.Err()
}

// ranked runs one FTS5 query ordered by the title-weighted rank. Unless the
// scope is a single document, it keeps at most pagesPerDocument pages of any
// one document and returns the documents that had more.
func (e *Engine) ranked(ctx context.Context, ftsQuery, source string, limit int) ([]Result, []string, error) {
	statement := `SELECT a.id, src.filename, a.path, a.title,
                snippet(articles_fts, 1, '»', '«', '...', 12)
         FROM articles_fts
         JOIN articles a ON a.id = articles_fts.rowid
         JOIN sources src ON src.id = a.source_id
         WHERE articles_fts MATCH ? AND rank MATCH '` + keywordRank + `'`
	clause, scopeArgs := scopeClause(source)
	statement += clause
	args := append([]any{ftsQuery}, scopeArgs...)
	// FTS5 hands the rows out in rank order, so reading stops as soon as the
	// limit is filled; the generous LIMIT only bounds the skipping.
	capped := !IsDocumentSource(source) || strings.HasSuffix(source, "/")
	scan := limit
	if capped {
		scan = limit + rankedScan
	}
	rows, err := e.db.QueryContext(ctx, statement+` ORDER BY rank LIMIT ?`, append(args, scan)...)
	if err != nil {
		return nil, nil, fmt.Errorf("engine: keyword search: %w", err)
	}
	defer rows.Close()

	var results []Result
	var more []string
	pages := map[string]int{}
	for len(results) < limit && rows.Next() {
		var r Result
		var snippet sql.NullString
		if err := rows.Scan(&r.ArticleID, &r.Filename, &r.Path, &r.Title, &snippet); err != nil {
			return nil, nil, err
		}
		if capped && IsDocumentSource(r.Filename) {
			pages[r.Filename]++
			if pages[r.Filename] > pagesPerDocument {
				if pages[r.Filename] == pagesPerDocument+1 {
					more = append(more, r.Filename)
				}
				continue
			}
		}
		r.Snippet = snippet.String
		results = append(results, r)
	}
	return results, more, rows.Err()
}

// missingWords returns the words that no article of the searched sources
// contains, so a caller can tell "not on this drive" from "not together".
func (e *Engine) missingWords(ctx context.Context, words []string, source string) ([]string, error) {
	clause, scopeArgs := scopeClause(source)
	statement := `SELECT 1 FROM articles_fts
         JOIN articles a ON a.id = articles_fts.rowid
         JOIN sources src ON src.id = a.source_id
         WHERE articles_fts MATCH ?` + clause + ` LIMIT 1`
	var missing []string
	for _, word := range words {
		args := append([]any{ftsTerm(word)}, scopeArgs...)
		rows, err := e.db.QueryContext(ctx, statement, args...)
		if err != nil {
			return nil, fmt.Errorf("engine: missing words: %w", err)
		}
		found := rows.Next()
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		if !found {
			missing = append(missing, word)
		}
	}
	return missing, nil
}

// markMatchedWords records on each partial result which query words it
// contains. The index answers, one query per word, so a word counts exactly
// when the search itself would have matched it.
func (e *Engine) markMatchedWords(ctx context.Context, words []string, results []Result) error {
	if len(results) == 0 {
		return nil
	}
	position := make(map[int]int, len(results))
	placeholders := make([]string, len(results))
	args := make([]any, 1, len(results)+1)
	for i := range results {
		results[i].Partial = true
		position[results[i].ArticleID] = i
		placeholders[i] = "?"
		args = append(args, results[i].ArticleID)
	}
	statement := `SELECT rowid FROM articles_fts WHERE articles_fts MATCH ? AND rowid IN (` + strings.Join(placeholders, ",") + `)`
	for _, word := range words {
		args[0] = ftsTerm(word)
		rows, err := e.db.QueryContext(ctx, statement, args...)
		if err != nil {
			return fmt.Errorf("engine: matched words: %w", err)
		}
		for rows.Next() {
			var id int
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			results[position[id]].Matched = append(results[position[id]].Matched, word)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// SourceFilenames lists the indexed sources without counting their articles.
func (e *Engine) SourceFilenames(ctx context.Context) ([]string, error) {
	rows, err := e.db.QueryContext(ctx, `SELECT filename FROM sources ORDER BY filename`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// IndexedPath is an article's path and title as the index stores them.
type IndexedPath struct {
	Path  string
	Title string
}

// nearPathRows bounds how many resembling paths NearPaths considers. With
// fewer rows than this it has seen them all and may call a match unique.
const nearPathRows = 50

// NearPaths looks an article path up the forgiving way, for a reader that was
// handed a path the source does not have. match is the indexed path equal to
// one of candidates, ignoring case and a leading slash, or failing that the
// only indexed path that ends with one of them (a path given without its
// leading directories). near holds up to limit indexed paths that contain
// segment, closest in length first.
//
// The articles table has no index on path, so this reads every row of the
// source. It belongs on the miss path, not in front of every read.
func (e *Engine) NearPaths(ctx context.Context, source string, candidates []string, segment string, limit int) (match IndexedPath, near []IndexedPath, err error) {
	if len(candidates) == 0 || segment == "" {
		return IndexedPath{}, nil, nil
	}
	wanted := make(map[string]int, len(candidates))
	for i, candidate := range candidates {
		key := strings.ToLower(strings.TrimLeft(candidate, "/"))
		if _, seen := wanted[key]; !seen {
			wanted[key] = i
		}
	}
	// LIKE compares ASCII case-insensitively and finds every candidate too,
	// because each of them contains the segment.
	pattern := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(segment) + "%"
	rows, err := e.db.QueryContext(ctx,
		`SELECT a.path, a.title FROM articles a
         JOIN sources src ON src.id = a.source_id
         WHERE src.filename = ? AND a.path LIKE ? ESCAPE '\'
         ORDER BY abs(length(a.path) - ?) LIMIT ?`,
		source, pattern, len(candidates[0])+1, nearPathRows)
	if err != nil {
		return IndexedPath{}, nil, fmt.Errorf("engine: near paths: %w", err)
	}
	defer rows.Close()

	best := len(candidates)
	var endings []IndexedPath
	seen := 0
	for rows.Next() {
		var found IndexedPath
		if err := rows.Scan(&found.Path, &found.Title); err != nil {
			return IndexedPath{}, nil, err
		}
		seen++
		key := strings.ToLower(strings.TrimLeft(found.Path, "/"))
		if at, ok := wanted[key]; ok && at < best {
			match, best = found, at
		}
		for candidate := range wanted {
			if strings.HasSuffix(key, "/"+candidate) {
				endings = append(endings, found)
				break
			}
		}
		if len(near) < limit {
			near = append(near, found)
		}
	}
	if match.Path == "" && len(endings) == 1 && seen < nearPathRows {
		match = endings[0]
	}
	return match, near, rows.Err()
}
