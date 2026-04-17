package search

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	drivebinary "github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/browser"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

type SessionInfo struct {
	SourceCount      int
	ArticleCount     int
	BestMode         Mode
	SemanticEnabled  bool
	EmbeddingModelID string
}

type SearchResponse struct {
	Results       []Result
	EffectiveMode Mode
	Status        string
}

type Session struct {
	driveRoot string
	dbPath    string
	sqliteBin string
	caps      Capabilities
	info      SessionInfo
	opener    func(string) error

	mu          sync.Mutex
	embedServer *exec.Cmd
	embedPort   int
	kiwixServer *exec.Cmd
	kiwixPort   int
}

func NewSession(driveRoot string, opener func(string) error) (*Session, error) {
	if opener == nil {
		opener = browser.Open
	}

	dbPath := filepath.Join(driveRoot, "data", "search.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("search index not found. run 'svalbard index' to build it")
	}

	sqliteBin, err := drivebinary.Resolve("sqlite3", driveRoot, platform.Detect)
	if err != nil {
		return nil, fmt.Errorf("sqlite3 not found")
	}

	caps, articleCount, sourceCount, err := detectCapabilities(driveRoot, dbPath, sqliteBin)
	if err != nil {
		return nil, err
	}

	// If embeddings were created with a specific model but that model isn't on the drive, disable semantic.
	if caps.EmbeddingModelID != "" && caps.EmbeddingModel == "" {
		caps.HasEmbeddingData = false
	}

	bestMode := BestMode(caps)
	return &Session{
		driveRoot: driveRoot,
		dbPath:    dbPath,
		sqliteBin: sqliteBin,
		caps:      caps,
		info: SessionInfo{
			SourceCount:      sourceCount,
			ArticleCount:     articleCount,
			BestMode:         bestMode,
			SemanticEnabled:  bestMode == ModeSemantic,
			EmbeddingModelID: caps.EmbeddingModelID,
		},
		opener:    opener,
		embedPort: 8085,
		kiwixPort: 8080,
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

	effectiveMode := mode
	status := ""
	var results []Result
	var err error

	if effectiveMode == ModeSemantic {
		if err := s.ensureEmbedServer(ctx); err != nil {
			effectiveMode = ModeKeyword
			status = "Semantic unavailable, fell back to keyword"
		} else {
			results, err = semanticSearch(s.sqliteBin, s.dbPath, query, s.info.ArticleCount, s.embedPort, limit)
			if err != nil || len(results) == 0 {
				effectiveMode = ModeKeyword
				if err != nil {
					status = "Semantic search failed, fell back to keyword"
				}
			}
		}
	}

	if effectiveMode == ModeKeyword {
		results, err = keywordSearch(s.sqliteBin, s.dbPath, query, limit)
		if err != nil {
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

	if s.embedServer != nil && s.embedServer.Process != nil {
		_ = s.embedServer.Process.Kill()
		_, _ = s.embedServer.Process.Wait()
		s.embedServer = nil
	}
	if s.kiwixServer != nil && s.kiwixServer.Process != nil {
		_ = s.kiwixServer.Process.Kill()
		_, _ = s.kiwixServer.Process.Wait()
		s.kiwixServer = nil
	}
	return nil
}

// EnsureKiwix starts the kiwix-serve process if it is not already running.
func (s *Session) EnsureKiwix(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.kiwixServer != nil {
		return nil
	}
	port, _ := netutil.FindAvailablePort("127.0.0.1", 8080)
	cmd, err := startKiwix(ctx, s.driveRoot, port)
	if err != nil {
		return err
	}
	s.kiwixServer = cmd
	s.kiwixPort = port
	return nil
}

// KiwixPort returns the port the kiwix-serve process is listening on.
func (s *Session) KiwixPort() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kiwixPort
}

func (s *Session) ensureEmbedServer(ctx context.Context) error {
	if s.embedServer != nil && s.embedServer.Process != nil {
		if s.embedServer.ProcessState == nil || !s.embedServer.ProcessState.Exited() {
			return nil
		}
		s.embedServer = nil
	}
	if !s.caps.HasLlamaServer || s.caps.EmbeddingModel == "" {
		return fmt.Errorf("semantic backend unavailable")
	}
	port, err := netutil.FindAvailablePort("127.0.0.1", 8085)
	if err != nil {
		return err
	}
	cmd, err := startEmbeddingServer(ctx, mustResolveLlamaServer(s.driveRoot), s.caps.EmbeddingModel, port)
	if err != nil {
		return err
	}
	s.embedPort = port
	s.embedServer = cmd
	return nil
}

func mustResolveLlamaServer(driveRoot string) string {
	path, _ := drivebinary.Resolve("llama-server", driveRoot, platform.Detect)
	return path
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
	cmd, err := startKiwix(ctx, s.driveRoot, port)
	if err != nil {
		return err
	}
	s.kiwixPort = port
	s.kiwixServer = cmd
	return nil
}
