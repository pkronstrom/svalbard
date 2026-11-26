// Package wiki is the drive's wiki: plain Markdown pages in <drive>/wiki,
// which the user edits in SilverBullet and the local agents edit through the
// MCP server's wiki tools. The pages are the only wiki data on the drive
// besides the trash; SilverBullet's own state is rebuilt on every start.
package wiki

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// RecipeID is the recipe that installs SilverBullet. The wiki tools are
	// offered on a drive that has it.
	RecipeID = "silverbullet"

	// DirName is the page folder, at the drive's root. Page "A/B" is the
	// file wiki/A/B.md, which is SilverBullet's own rule.
	DirName = "wiki"

	// PreferredPort is where the launcher serves the wiki when the port is
	// free. portSpan is how many ports upward netutil.Listen tries before it
	// takes any free one; RunningURL looks at the same ones.
	PreferredPort = 8088
	portSpan      = 20

	// folderHeader carries the page folder on the launcher's answer to
	// pingPath, so a tool can tell this drive's wiki from another one's.
	folderHeader = "X-Svalbard-Wiki"
	pingPath     = "/.ping"
)

// Dir is the page folder of the drive at driveRoot.
func Dir(driveRoot string) string { return filepath.Join(driveRoot, DirName) }

// TrashDir is where a page an agent deletes goes. It is outside the page
// folder, so SilverBullet does not show it, and on the same volume, so the
// delete is a rename.
func TrashDir(driveRoot string) string {
	return filepath.Join(driveRoot, ".svalbard", "state", "wiki-trash")
}

// TrashPath is TrashDir as the user finds it on the drive.
const TrashPath = ".svalbard/state/wiki-trash"

const seedPage = `# Wiki

This is the wiki on your Svalbard drive: your own notes, next to the reference library.

- Every page is a plain Markdown file in the ` + "`wiki`" + ` folder of the drive. The files can be read and edited without this app.
- Link to a page by typing its name in double square brackets, like ` + "`[[Garden plan]]`" + `. Opening a link to a page that does not exist yet creates it.
- The AI clients on the drive (Open WebUI, pi, Goose, opencode) can read, search and write these pages. Ask one to "summarise ... into a wiki page" and the page appears here.
- A page an AI client deletes is not erased. It is moved to ` + "`" + TrashPath + "`" + ` on the drive; move the file back into ` + "`wiki`" + ` to restore it.

This is the start page. Change it as you like.
`

// Seed writes the start page into the page folder dir when the folder holds
// nothing yet. A folder with any page in it is left alone, so a start page
// the user deleted stays deleted.
func Seed(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		// The host's own litter (.DS_Store, ._name) does not count.
		if !strings.HasPrefix(entry.Name(), ".") {
			return nil
		}
	}
	return os.WriteFile(filepath.Join(dir, "index.md"), []byte(seedPage), 0o644)
}

// PageURL is the address of the page name in the wiki served at base.
func PageURL(base, name string) string {
	segments := strings.Split(name, "/")
	for i, segment := range segments {
		segments[i] = url.PathEscape(segment)
	}
	return base + "/" + strings.Join(segments, "/")
}

// RunningURL returns the address of the wiki a launcher is serving for the
// drive at driveRoot, without a trailing slash, or "" when none is. The
// launcher and the MCP server are separate processes with nothing on the
// drive between them: the launcher's ports are asked instead, and the one
// that names this drive's page folder is it.
func RunningURL(driveRoot string) string {
	return runningURL(driveRoot, PreferredPort, portSpan)
}

// runningURL asks the span ports from first upward.
func runningURL(driveRoot string, first, span int) string {
	folder := resolvedDir(driveRoot)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	// No keep-alive: these are one-off questions to ports that mostly refuse.
	// No redirects: whatever else listens on one of these ports must not
	// send the question somewhere else.
	client := &http.Client{
		Transport:     &http.Transport{DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	found := make(chan int, span)
	for port := first; port < first+span; port++ {
		go func() {
			request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d%s", port, pingPath), nil)
			if err != nil {
				found <- 0
				return
			}
			response, err := client.Do(request)
			if err != nil {
				found <- 0
				return
			}
			response.Body.Close()
			if response.StatusCode == http.StatusOK && response.Header.Get(folderHeader) == folder {
				found <- port
				return
			}
			found <- 0
		}()
	}
	for range span {
		if port := <-found; port != 0 {
			// Spelled "localhost": a small model copying "127.0.0.1:8088"
			// into its reply drops digits often enough to matter.
			return fmt.Sprintf("http://localhost:%d", port)
		}
	}
	return ""
}

// resolvedDir is the page folder with links resolved, the form the launcher
// announces and RunningURL compares with. A stick reached through two
// spellings of one path (/tmp and /private/tmp) is still one wiki.
func resolvedDir(driveRoot string) string {
	dir := Dir(driveRoot)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		return resolved
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}
