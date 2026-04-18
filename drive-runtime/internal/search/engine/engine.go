package engine

import (
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
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
}

// Engine provides keyword and hybrid search over a search.db.
type Engine struct {
	db *sql.DB
}

// New creates a search engine backed by the given database connection.
func New(db *sql.DB) *Engine {
	return &Engine{db: db}
}

// Keyword runs an FTS5 keyword search.
func (e *Engine) Keyword(query string, limit int) ([]Result, error) {
	ftsQuery := BuildFTSQuery(query)
	rows, err := e.db.Query(
		`SELECT a.id, src.filename, a.path, a.title,
                snippet(articles_fts, 1, '»', '«', '...', 12)
         FROM articles_fts
         JOIN articles a ON a.id = articles_fts.rowid
         JOIN sources src ON src.id = a.source_id
         WHERE articles_fts MATCH ?
         ORDER BY rank
         LIMIT ?`,
		ftsQuery, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("engine: keyword search: %w", err)
	}
	defer rows.Close()

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
		results = append(results, r)
	}
	return results, rows.Err()
}

// Hybrid runs FTS5 keyword and vector search in parallel, merges with RRF, applies MMR.
func (e *Engine) Hybrid(query string, queryVec []float32, limit int) ([]Result, error) {
	// Run FTS5 and vector search in parallel
	type ftsResult struct {
		results []Result
		err     error
	}
	type vecResult struct {
		results []scoredChunk
		err     error
	}

	var wg sync.WaitGroup
	ftsCh := make(chan ftsResult, 1)
	vecCh := make(chan vecResult, 1)

	wg.Add(2)
	go func() {
		defer wg.Done()
		r, err := e.Keyword(query, 100)
		ftsCh <- ftsResult{r, err}
	}()
	go func() {
		defer wg.Done()
		r, err := e.vectorSearch(queryVec, 100)
		vecCh <- vecResult{r, err}
	}()
	wg.Wait()

	fts := <-ftsCh
	vec := <-vecCh

	// FTS5 results
	var ftsResults []Result
	if fts.err == nil {
		ftsResults = fts.results
	}

	// Vector results
	var vecChunks []scoredChunk
	if vec.err == nil {
		vecChunks = vec.results
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

// vectorSearch fetches all embeddings and computes dot product with queryVec.
func (e *Engine) vectorSearch(queryVec []float32, limit int) ([]scoredChunk, error) {
	rows, err := e.db.Query(
		`SELECT e.article_id, e.chunk_header, e.vector
         FROM embeddings e`)
	if err != nil {
		return nil, fmt.Errorf("engine: vector search: %w", err)
	}
	defer rows.Close()

	var results []scoredChunk
	for rows.Next() {
		var articleID int
		var header string
		var vecBlob []byte
		if err := rows.Scan(&articleID, &header, &vecBlob); err != nil {
			continue
		}
		vec := blobToFloat32(vecBlob)
		score := DotProduct(queryVec, vec)
		results = append(results, scoredChunk{
			articleID: articleID,
			header:    header,
			score:     score,
			vector:    vec,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})
	if len(results) > limit {
		results = results[:limit]
	}
	return results, rows.Err()
}

type scoredChunk struct {
	articleID int
	header    string
	score     float64
	vector    []float32
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
	filename := source
	if !strings.HasSuffix(filename, ".zim") {
		filename += ".zim"
	}
	var body sql.NullString
	err := e.db.QueryRow(
		`SELECT a.body FROM articles a
         JOIN sources src ON src.id = a.source_id
         WHERE src.filename = ? AND a.path = ?`,
		filename, path,
	).Scan(&body)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("article not found: %s/%s", source, path)
	}
	if err != nil {
		return "", err
	}
	return body.String, nil
}

// ListSources returns all indexed sources.
func (e *Engine) ListSources() ([]SourceInfo, error) {
	rows, err := e.db.Query(
		`SELECT src.filename, COUNT(a.id) FROM sources src
         LEFT JOIN articles a ON a.source_id = src.id
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

// SourceInfo holds source metadata.
type SourceInfo struct {
	Filename     string `json:"filename"`
	ArticleCount int    `json:"article_count"`
}
