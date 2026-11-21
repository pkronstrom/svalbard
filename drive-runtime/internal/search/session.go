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
)

type SessionInfo struct {
	SourceCount      int
	ArticleCount     int
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
	driveRoot string
	db        *sql.DB
	eng       *engine.Engine
	caps      Capabilities
	info      SessionInfo
	opener    func(string) error

	mu           sync.Mutex
	embedBackend *SemanticBackend
	kiwixServer  *exec.Cmd
	kiwixPort    int
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
	return &Session{
		driveRoot: driveRoot,
		db:        db,
		eng:       engine.New(db),
		caps:      caps,
		info: SessionInfo{
			SourceCount:      sourceCount,
			ArticleCount:     articleCount,
			BestMode:         bestMode,
			HybridEnabled:    bestMode == ModeHybrid,
			EmbeddingModelID: caps.EmbeddingModelID,
		},
		opener:       opener,
		embedBackend: NewSemanticBackend(driveRoot),
		kiwixPort:    8080,
	}, nil
}

func (s *Session) Info() SessionInfo {
	return s.info
}

func (s *Session) Search(ctx context.Context, mode Mode, query string, limit int) (SearchResponse, error) {
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
			results, usable, embedErr = HybridSnapshot(ctx, s.db, contract, query, queryVec, limit)
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

	if effectiveMode == ModeKeyword {
		results, err = s.eng.KeywordContext(ctx, query, limit)
		if err != nil {
			return SearchResponse{}, err
		}
		if err := ctx.Err(); err != nil {
			return SearchResponse{}, err
		}
	}

	return SearchResponse{
		Results:       results,
		EffectiveMode: effectiveMode,
		Status:        status,
	}, nil
}

func (s *Session) OpenResult(result Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureKiwix(context.Background()); err != nil {
		return err
	}
	book := strings.TrimSuffix(result.Filename, ".zim")
	path := strings.TrimLeft(result.Path, "/")
	url := fmt.Sprintf("http://localhost:%d/content/%s/%s", s.kiwixPort, book, path)
	return s.opener(url)
}

func (s *Session) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.embedBackend != nil {
		_ = s.embedBackend.Close()
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
	zims, err := filepath.Glob(filepath.Join(s.driveRoot, "zim", "*.zim"))
	if err != nil {
		return err
	}
	cmd, err := server.StartKiwix(ctx, kiwixBin, zims, port)
	if err != nil {
		return err
	}
	s.kiwixPort = port
	s.kiwixServer = cmd
	return nil
}
