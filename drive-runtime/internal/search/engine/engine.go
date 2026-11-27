package engine

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Result represents a search hit.
type Result struct {
	ArticleID   int
	Filename    string
	Path        string
	Title       string
	Snippet     string
	ChunkHeader string
	Score       float64
	Body        string
	Native      bool // result retrieved from an embedded ZIM index
	// Partial marks a keyword result that lacks some of the query words;
	// Matched lists the ones it has.
	Partial bool
	Matched []string
}

// Engine provides keyword and hybrid search over a search.db.
type Engine struct {
	db Queryer
}

// Queryer is satisfied by both *sql.DB and *sql.Tx, keeping one read snapshot
// across all hybrid SQL reads.
type Queryer interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}

// New creates a search engine backed by the given database connection.
func New(db Queryer) *Engine {
	return &Engine{db: db}
}

// Keyword runs an FTS5 keyword search over the knowledge sources.
func (e *Engine) Keyword(query string, limit int) ([]Result, error) {
	return e.KeywordContext(context.Background(), query, limit, "")
}

// KeywordContext runs FTS5 search with caller cancellation propagated to
// SQLite. scope is a search scope as InScope reads it. Of one skill it
// returns at most sectionsPerSkill sections unless the scope is that skill,
// so that hybrid search, whose candidates these are, reaches the skills
// behind a long one.
func (e *Engine) KeywordContext(ctx context.Context, query string, limit int, scope string, excluded ...string) ([]Result, error) {
	ftsQuery := BuildFTSQuery(query)
	clause, scopeArgs := scopeClause(scope, excluded...)
	skills := e.newSkillBound(ctx, scope)
	scan := limit
	if skills != nil {
		scan = limit + rankedScan
	}
	args := append(append([]any{ftsQuery}, scopeArgs...), scan)
	rows, err := e.db.QueryContext(ctx,
		`SELECT a.id, src.filename, a.path, a.title,
                snippet(articles_fts, 1, '»', '«', '...', 12)
         FROM articles_fts
         JOIN articles a ON a.id = articles_fts.rowid
         JOIN sources src ON src.id = a.source_id
         WHERE articles_fts MATCH ?`+clause+`
         ORDER BY rank
         LIMIT ?`,
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("engine: keyword search: %w", err)
	}
	defer rows.Close()

	var results []Result
	for len(results) < limit && rows.Next() {
		var r Result
		var snippet sql.NullString
		if err := rows.Scan(&r.ArticleID, &r.Filename, &r.Path, &r.Title, &snippet); err != nil {
			continue
		}
		if skip, _ := skills.over(r.Filename); skip {
			continue
		}
		if snippet.Valid {
			r.Snippet = snippet.String
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, ctx.Err()
}

// Hybrid runs FTS5 keyword search, then vector search pre-filtered to FTS
// candidates, merges with RRF, and applies MMR diversity.
func (e *Engine) Hybrid(query string, queryVec []float32, limit int) ([]Result, error) {
	return e.HybridContext(context.Background(), query, queryVec, limit, "")
}

// HybridContext is Hybrid within a search scope, as InScope reads it. Both
// halves keep to the scope: the keyword candidates and the vectors scored.
func (e *Engine) HybridContext(ctx context.Context, query string, queryVec []float32, limit int, scope string, excluded ...string) ([]Result, error) {
	return e.hybridContext(ctx, query, queryVec, limit, scope, false, excluded...)
}

// SelectiveHybridContext searches all vectors in a selective index so
// paraphrases and cross-language hits can enter the keyword ranking.
func (e *Engine) SelectiveHybridContext(ctx context.Context, query string, queryVec []float32, limit int, scope string, excluded ...string) ([]Result, error) {
	return e.hybridContext(ctx, query, queryVec, limit, scope, true, excluded...)
}

func (e *Engine) hybridContext(ctx context.Context, query string, queryVec []float32, limit int, scope string, independent bool, excluded ...string) ([]Result, error) {
	// Selective retrieval uses the regular keyword policy. Weak any-word
	// matches should not boost an incidental semantic hit above a useful one.
	var ftsResults []Result
	var ftsErr error
	if independent {
		matches, err := e.Search(ctx, query, KeywordOptions{Source: scope, Limit: 100, ExcludedArchives: excluded})
		ftsResults, ftsErr = matches.Results, err
	} else {
		ftsResults, ftsErr = e.KeywordContext(ctx, query, 200, scope, excluded...)
	}

	// Step 2: Run vector search filtered to FTS candidate articles.
	// This avoids loading ALL embeddings into memory.
	var candidateIDs []int
	if ftsErr == nil {
		for _, r := range ftsResults {
			candidateIDs = append(candidateIDs, r.ArticleID)
		}
	}

	var vecChunks []scoredChunk
	var vecErr error
	if len(candidateIDs) > 0 && !independent {
		// The candidates are in the scope already.
		vecChunks, vecErr = e.vectorSearchFiltered(queryVec, candidateIDs, 100)
	} else {
		// No FTS hits — fall back to full scan (small corpus or unusual query)
		vecChunks, vecErr = e.vectorSearch(ctx, queryVec, 100, scope, excluded...)
	}
	if vecErr != nil {
		return nil, vecErr
	}
	if independent {
		// The vector list is sorted by similarity. Give each article one
		// vote, keeping its most relevant chunk for the result heading.
		seen := map[int]bool{}
		best := vecChunks[:0]
		for _, chunk := range vecChunks {
			if !seen[chunk.articleID] {
				best = append(best, chunk)
				seen[chunk.articleID] = true
			}
		}
		vecChunks = best
	}

	// Trim FTS results to top 100 for RRF (we fetched 200 for wider pre-filter)
	if len(ftsResults) > 100 {
		ftsResults = ftsResults[:100]
	}

	// RRF merge
	merged := rrfMerge(ftsResults, vecChunks, 60)

	// Dedup to best chunk per article (after RRF)
	deduped := dedupByArticle(merged)

	// MMR diversity
	if len(deduped) > limit {
		// Get vectors for MMR
		vecs := make(map[int][]float32)
		for _, sc := range vecChunks {
			if _, ok := vecs[sc.articleID]; !ok {
				vecs[sc.articleID] = sc.vector
			}
		}
		deduped = applyMMR(deduped, vecs, 0.7, limit)
	}

	if len(deduped) > limit {
		deduped = deduped[:limit]
	}

	// Fetch full article details for final results
	return e.fetchDetails(deduped)
}

// vectorSearchFiltered fetches embeddings only for the given article IDs.
func (e *Engine) vectorSearchFiltered(queryVec []float32, articleIDs []int, limit int) ([]scoredChunk, error) {
	if len(articleIDs) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(articleIDs))
	args := make([]any, len(articleIDs))
	for i, id := range articleIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(
		`SELECT e.article_id, e.chunk_header, e.vector, ''
		 FROM embeddings e
		 WHERE e.article_id IN (%s)`,
		strings.Join(placeholders, ","),
	)

	rows, err := e.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("engine: filtered vector search: %w", err)
	}
	defer rows.Close()

	return e.scanAndScoreVectors(rows, queryVec, limit, nil)
}

// vectorSearch fetches every embedding in the scope and computes dot product
// with queryVec. Used as fallback when FTS5 returns no candidates.
func (e *Engine) vectorSearch(ctx context.Context, queryVec []float32, limit int, scope string, excluded ...string) ([]scoredChunk, error) {
	clause, scopeArgs := scopeClause(scope, excluded...)
	skills := e.newSkillBound(ctx, scope)
	rows, err := e.db.Query(
		`SELECT e.article_id, e.chunk_header, e.vector, src.filename
         FROM embeddings e
         JOIN articles a ON a.id = e.article_id
         JOIN sources src ON src.id = a.source_id
         WHERE 1 = 1`+clause, scopeArgs...)
	if err != nil {
		return nil, fmt.Errorf("engine: vector search: %w", err)
	}
	defer rows.Close()

	return e.scanAndScoreVectors(rows, queryVec, limit, skills)
}

// scanAndScoreVectors scores the rows (article, chunk header, vector, source
// filename) against queryVec and returns the nearest, within the skill bound
// when there is one.
func (e *Engine) scanAndScoreVectors(rows *sql.Rows, queryVec []float32, limit int, skills *skillBound) ([]scoredChunk, error) {
	if err := validVector(queryVec); err != nil {
		return nil, fmt.Errorf("engine: query vector: %w", err)
	}
	var results []scoredChunk
	for rows.Next() {
		var articleID int
		var header, filename string
		var vecBlob []byte
		if err := rows.Scan(&articleID, &header, &vecBlob, &filename); err != nil {
			continue
		}
		vec := blobToFloat32(vecBlob)
		if err := validVector(vec); err != nil {
			return nil, fmt.Errorf("engine: stored vector for article %d: %w", articleID, err)
		}
		if len(vec) != len(queryVec) {
			return nil, fmt.Errorf("engine: vector dimensions differ: query %d, stored %d", len(queryVec), len(vec))
		}
		score := DotProduct(queryVec, vec)
		results = append(results, scoredChunk{
			articleID: articleID,
			header:    header,
			score:     score,
			vector:    vec,
			filename:  filename,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if skills != nil {
		kept := results[:0]
		for _, chunk := range results {
			if skip, _ := skills.over(chunk.filename); !skip {
				kept = append(kept, chunk)
			}
		}
		results = kept
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func validVector(v []float32) error {
	if len(v) == 0 {
		return fmt.Errorf("empty vector")
	}
	for _, value := range v {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("non-finite value")
		}
	}
	return nil
}

type scoredChunk struct {
	articleID int
	header    string
	score     float64
	vector    []float32
	// filename is the chunk's source, known only in a full vector scan.
	filename string
}

type rrfEntry struct {
	articleID int
	header    string
	score     float64
}

func rrfMerge(ftsResults []Result, vecResults []scoredChunk, k float64) []rrfEntry {
	scores := make(map[int]*rrfEntry)

	for rank, r := range ftsResults {
		id := r.ArticleID
		if _, ok := scores[id]; !ok {
			scores[id] = &rrfEntry{articleID: id}
		}
		scores[id].score += 1.0 / (k + float64(rank))
	}

	for rank, sc := range vecResults {
		id := sc.articleID
		if _, ok := scores[id]; !ok {
			scores[id] = &rrfEntry{articleID: id, header: sc.header}
		}
		scores[id].score += 1.0 / (k + float64(rank))
		if scores[id].header == "" {
			scores[id].header = sc.header
		}
	}

	result := make([]rrfEntry, 0, len(scores))
	for _, e := range scores {
		result = append(result, *e)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].score > result[j].score
	})
	return result
}

func dedupByArticle(entries []rrfEntry) []rrfEntry {
	seen := make(map[int]bool)
	var out []rrfEntry
	for _, e := range entries {
		if seen[e.articleID] {
			continue
		}
		seen[e.articleID] = true
		out = append(out, e)
	}
	return out
}

func applyMMR(candidates []rrfEntry, vectors map[int][]float32, lambda float64, limit int) []rrfEntry {
	if len(candidates) == 0 || limit <= 0 {
		return nil
	}

	// Normalize RRF scores to [0,1]
	maxScore := candidates[0].score
	if maxScore <= 0 {
		maxScore = 1
	}

	selected := []rrfEntry{candidates[0]}
	remaining := candidates[1:]

	for len(selected) < limit && len(remaining) > 0 {
		bestIdx := -1
		bestMMR := math.Inf(-1)

		for i, cand := range remaining {
			rel := cand.score / maxScore

			// Max similarity to any selected result
			maxSim := 0.0
			candVec := vectors[cand.articleID]
			if candVec != nil {
				for _, sel := range selected {
					selVec := vectors[sel.articleID]
					if selVec != nil {
						sim := DotProduct(candVec, selVec)
						if sim > maxSim {
							maxSim = sim
						}
					}
				}
			}

			mmr := lambda*rel - (1-lambda)*maxSim
			if mmr > bestMMR {
				bestMMR = mmr
				bestIdx = i
			}
		}

		if bestIdx < 0 {
			break
		}
		selected = append(selected, remaining[bestIdx])
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}
	return selected
}

func (e *Engine) fetchDetails(entries []rrfEntry) ([]Result, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	// Build a VALUES list for ordered retrieval
	valueRows := make([]string, len(entries))
	for i, entry := range entries {
		valueRows[i] = fmt.Sprintf("(%d,%d)", entry.articleID, i)
	}

	query := fmt.Sprintf(
		`WITH ranked(aid, pos) AS (VALUES %s)
         SELECT a.id, src.filename, a.path, a.title, substr(a.body, 1, 200)
         FROM ranked r
         JOIN articles a ON a.id = r.aid
         JOIN sources src ON src.id = a.source_id
         ORDER BY r.pos`,
		strings.Join(valueRows, ","),
	)

	rows, err := e.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("engine: fetch details: %w", err)
	}
	defer rows.Close()

	// Build a map from articleID to chunk header
	headerMap := make(map[int]string)
	scoreMap := make(map[int]float64)
	for _, e := range entries {
		headerMap[e.articleID] = e.header
		scoreMap[e.articleID] = e.score
	}

	var results []Result
	for rows.Next() {
		var r Result
		var snippet sql.NullString
		if err := rows.Scan(&r.ArticleID, &r.Filename, &r.Path, &r.Title, &snippet); err != nil {
			continue
		}
		if snippet.Valid {
			r.Snippet = snippet.String
		}
		r.ChunkHeader = headerMap[r.ArticleID]
		r.Score = scoreMap[r.ArticleID]
		results = append(results, r)
	}
	return results, rows.Err()
}

// BuildFTSQuery converts a user query to an FTS5 MATCH expression.
func BuildFTSQuery(query string) string {
	parts := strings.Fields(query)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.ReplaceAll(part, `"`, `""`)
		part = strings.ReplaceAll(part, `'`, `''`)
		out = append(out, fmt.Sprintf(`"%s"*`, part))
	}
	return strings.Join(out, " ")
}

// DotProduct computes the dot product of two float32 vectors.
func DotProduct(a, b []float32) float64 {
	limit := len(a)
	if len(b) < limit {
		limit = len(b)
	}
	var sum float64
	for i := 0; i < limit; i++ {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}

func blobToFloat32(blob []byte) []float32 {
	if len(blob)%4 != 0 {
		return nil
	}
	out := make([]float32, len(blob)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(blob[i*4 : i*4+4]))
	}
	return out
}

// ReadArticle reads article body by source filename and path.
func (e *Engine) ReadArticle(source, path string) (string, error) {
	// The name as given first: a document is indexed under its path, which
	// ends in its own extension. An archive may be named without ".zim".
	filenames := []string{source}
	if !strings.HasSuffix(source, ".zim") {
		filenames = append(filenames, source+".zim")
	}
	var body sql.NullString
	err := sql.ErrNoRows
	for _, filename := range filenames {
		err = e.db.QueryRow(
			`SELECT a.body FROM articles a
             JOIN sources src ON src.id = a.source_id
             WHERE src.filename = ? AND a.path = ?`,
			filename, path,
		).Scan(&body)
		if err != sql.ErrNoRows {
			break
		}
	}
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("article not found: %s/%s", source, path)
	}
	if err != nil {
		return "", err
	}
	return body.String, nil
}

// ListSources returns the indexed knowledge sources: the archives and the
// documents. Skill files and library quickstarts are not listed.
func (e *Engine) ListSources() ([]SourceInfo, error) {
	rows, err := e.db.Query(
		`SELECT src.filename, COUNT(a.id) FROM sources src
         LEFT JOIN articles a ON a.source_id = src.id
         WHERE NOT ` + skillRoots + `
         GROUP BY src.id ORDER BY src.filename`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []SourceInfo
	for rows.Next() {
		var s SourceInfo
		if err := rows.Scan(&s.Filename, &s.ArticleCount); err != nil {
			continue
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}

// KnowledgeCounts counts the knowledge sources and their articles: what a
// search without a scope covers.
func (e *Engine) KnowledgeCounts() (sources, articles int) {
	_ = e.db.QueryRow(`SELECT count(*) FROM sources src WHERE NOT ` + skillRoots).Scan(&sources)
	_ = e.db.QueryRow(`SELECT count(*) FROM articles a JOIN sources src ON src.id = a.source_id WHERE NOT ` + skillRoots).Scan(&articles)
	return sources, articles
}

// SourceInfo holds source metadata.
type SourceInfo struct {
	Filename     string `json:"filename"`
	ArticleCount int    `json:"article_count"`
}
