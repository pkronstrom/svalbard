package search

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/ncruces/go-sqlite3/driver"

	drivebinary "github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/browser"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/server"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/workspace"
)

type SessionInfo struct {
	SourceCount      int
	ArticleCount     int
	NativeZIMCount   int
	BestMode         Mode
	HybridEnabled    bool
	EmbeddingModelID string
}

type SearchResponse struct {
	Results       []Result
	EffectiveMode Mode
	Status        string
}

type Session struct {
	driveRoot     string
	db            *sql.DB
	eng           *engine.Engine
	caps          Capabilities
	info          SessionInfo
	opener        func(string) error
	dashboardPort int

	mu           sync.Mutex
	embedBackend *SemanticBackend
	kiwixServer  *exec.Cmd
	kiwixPort    int
	documents    *documentServer // started by the first document that is opened
}

func NewSession(driveRoot string, opener func(string) error) (*Session, error) {
	return newSession(context.Background(), driveRoot, opener)
}

// newSession keeps the public constructor compatible while allowing Run to
// stop capability detection before model hashing or binary startup begins.
func newSession(ctx context.Context, driveRoot string, opener func(string) error) (*Session, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if opener == nil {
		opener = browser.Open
	}

	dbPath := filepath.Join(driveRoot, "data", "search.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("search index not found. run 'svalbard index' to build it")
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if err != nil {
		return nil, fmt.Errorf("opening search.db: %w", err)
	}

	caps, articleCount, sourceCount, err := detectCapabilities(ctx, driveRoot, db)
	if err != nil {
		db.Close()
		return nil, err
	}

	// If embeddings were created with a specific model but that model isn't on the drive, disable hybrid.
	if caps.EmbeddingModelID != "" && caps.EmbeddingModel == "" {
		caps.HasEmbeddingData = false
	}

	bestMode := BestMode(caps)
	eng := engine.New(db)
	groups, err := eng.NativeZIMGroups(ctx, "")
	if err != nil {
		db.Close()
		return nil, err
	}
	var nativeCount int
	for _, group := range groups {
		nativeCount += len(group.Books)
	}
	return &Session{
		driveRoot: driveRoot,
		db:        db,
		eng:       eng,
		caps:      caps,
		info: SessionInfo{
			SourceCount:      sourceCount,
			ArticleCount:     articleCount,
			NativeZIMCount:   nativeCount,
			BestMode:         bestMode,
			HybridEnabled:    bestMode == ModeHybrid,
			EmbeddingModelID: caps.EmbeddingModelID,
		},
		opener:       opener,
		embedBackend: NewSemanticBackend(driveRoot),
		kiwixPort:    8080,
	}, nil
}

// NewBrowserSession opens a search session whose document server can be
// framed by the dashboard.
func NewBrowserSession(ctx context.Context, driveRoot string, dashboardPort int) (*Session, error) {
	session, err := newSession(ctx, driveRoot, nil)
	if err != nil {
		return nil, err
	}
	session.dashboardPort = dashboardPort
	return session, nil
}

func (s *Session) Info() SessionInfo {
	return s.info
}

func (s *Session) Search(ctx context.Context, mode Mode, query string, limit int, excluded ...string) (SearchResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return SearchResponse{}, err
	}

	effectiveMode := mode
	status := ""
	var results []Result
	var err error

	if effectiveMode == ModeHybrid {
		contract, complete := LoadCompleteContract(s.db)
		if !complete {
			effectiveMode = ModeKeyword
			status = "Hybrid unavailable, fell back to keyword"
		} else if queryVec, embedErr := s.embedBackend.Embed(ctx, contract, query); embedErr != nil {
			effectiveMode = ModeKeyword
			status = "Hybrid unavailable, fell back to keyword"
		} else {
			var usable bool
			results, usable, embedErr = HybridSnapshot(ctx, s.db, contract, query, queryVec, limit, "", excluded...)
			if !usable {
				effectiveMode = ModeKeyword
				status = "Hybrid unavailable, fell back to keyword"
			}
			if embedErr != nil || err != nil || len(results) == 0 {
				effectiveMode = ModeKeyword
				if embedErr != nil || err != nil {
					status = "Hybrid search failed, fell back to keyword"
				}
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return SearchResponse{}, err
	}

	var matches engine.KeywordMatches
	if effectiveMode == ModeKeyword {
		matches, err = s.eng.Search(ctx, query, engine.KeywordOptions{Limit: limit, ExcludedArchives: excluded})
		if err != nil {
			return SearchResponse{}, err
		}
		if err := ctx.Err(); err != nil {
			return SearchResponse{}, err
		}
		results = matches.Results
	}

	nativePort := func(ctx context.Context) (int, error) {
		err := s.ensureKiwix(ctx)
		return s.kiwixPort, err
	}
	results, native, nativeErr := MergeZIMKeywords(ctx, s.eng, nativePort, query, "", limit, results, excluded...)
	if err := ctx.Err(); err != nil {
		return SearchResponse{}, err
	}
	var notes []string
	if status != "" {
		notes = append(notes, status)
	}
	if nativeErr != nil {
		notes = append(notes, "Some embedded ZIM indexes were unavailable: "+nativeErr.Error())
	}
	if effectiveMode == ModeKeyword && !native {
		// Partial matches follow the complete ones; say where they start, and
		// name the words that are nowhere in the index.
		if partial := len(results) - matches.Exact; partial > 0 && matches.Exact > 0 {
			notes = append(notes, fmt.Sprintf("First %d have every word; the rest are partial matches", matches.Exact))
		} else if partial > 0 {
			notes = append(notes, "No result has every word; showing partial matches")
		}
		if len(matches.Missing) > 0 {
			notes = append(notes, "No article mentions: "+strings.Join(matches.Missing, ", "))
		}
	}
	status = strings.Join(notes, ". ")

	return SearchResponse{
		Results:       results,
		EffectiveMode: effectiveMode,
		Status:        status,
	}, nil
}

func (s *Session) OpenResult(result Result) error {
	address, err := s.ResultURL(context.Background(), result)
	if err != nil {
		return err
	}
	return s.opener(address)
}

// ResultURL starts the appropriate reader if needed and returns the URL for a
// result. Its servers stay available until the session closes.
func (s *Session) ResultURL(ctx context.Context, result Result) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// A document opens in the browser's own PDF viewer at the page of the
	// hit; kiwix-serve only knows ZIM archives.
	if IsDocumentSource(result.Filename) && !engine.IsEmbeddedSource(result.Filename) {
		if s.documents == nil {
			server, err := startDocumentServer(s.driveRoot, s.dashboardPort)
			if err != nil {
				return "", fmt.Errorf("serving documents: %w", err)
			}
			s.documents = server
		}
		page, _ := engine.PageNumber(result.Path)
		return s.documents.pageURL(result.Filename, page), nil
	}

	if err := s.ensureKiwix(ctx); err != nil {
		return "", err
	}
	return ArchiveResultURL(fmt.Sprintf("http://localhost:%d", s.kiwixPort), "", result), nil
}

// ArchiveSource returns the ZIM that holds a search source, including a
// document embedded in an archive. For a standalone document it is empty.
func ArchiveSource(filename string) string {
	if archive, _, ok := engine.EmbeddedArchive(filename); ok {
		return archive
	}
	if strings.ContainsAny(filename, "/\\") || !strings.EqualFold(filepath.Ext(filename), ".zim") {
		return ""
	}
	return filename
}

// ArchiveResultURL gives the article address on a Kiwix server that already
// serves the archive. book is Kiwix's catalog alias, when available.
func ArchiveResultURL(base, book string, result Result) string {
	if engine.IsEmbeddedSource(result.Filename) {
		// A video transcript hit opens the video at the spoken time.
		if ms, ok := engine.VideoTime(result.Path); ok {
			if book == "" {
				return embeddedURL(base, result.Filename, 0) + videoFragment(ms)
			}
			return embeddedBookURL(base, book, result.Filename, 0) + videoFragment(ms)
		}
		page, _ := engine.PageNumber(result.Path)
		if book == "" {
			return embeddedURL(base, result.Filename, page)
		}
		return embeddedBookURL(base, book, result.Filename, page)
	}
	if book == "" {
		book = strings.TrimSuffix(result.Filename, filepath.Ext(result.Filename))
	}
	article := strings.TrimLeft(result.Path, "/")
	return workspace.URL(base+"/content", book+"/"+article)
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.embedBackend != nil {
		_ = s.embedBackend.Close()
	}
	if s.documents != nil {
		s.documents.Close()
		s.documents = nil
	}
	if s.kiwixServer != nil && s.kiwixServer.Process != nil {
		_ = s.kiwixServer.Process.Kill()
		_ = s.kiwixServer.Wait()
		s.kiwixServer = nil
	}
	if s.db != nil {
		s.db.Close()
	}
	return nil
}

// EnsureKiwix starts the kiwix-serve process if it is not already running.
func (s *Session) EnsureKiwix(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureKiwix(ctx)
}

// KiwixPort returns the port the kiwix-serve process is listening on.
func (s *Session) KiwixPort() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kiwixPort
}

func (s *Session) ensureKiwix(ctx context.Context) error {
	if s.kiwixServer != nil && s.kiwixServer.Process != nil {
		if s.kiwixServer.ProcessState == nil || !s.kiwixServer.ProcessState.Exited() {
			return nil
		}
		s.kiwixServer = nil
	}
	port, err := netutil.FindAvailablePort("127.0.0.1", 8080)
	if err != nil {
		return err
	}
	kiwixBin, err := drivebinary.Resolve("kiwix-serve", s.driveRoot)
	if err != nil {
		return err
	}
	zims, err := server.ZIMFiles(s.driveRoot)
	if err != nil {
		return err
	}
	languages, err := s.eng.NativeZIMLanguages(ctx)
	if err != nil {
		return err
	}
	cmd, err := server.StartKiwixForSearch(ctx, kiwixBin, zims, port, languages)
	if err != nil {
		return err
	}
	s.kiwixPort = port
	s.kiwixServer = cmd
	return nil
}
