package frontpage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/apps"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/browse"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/folder"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/inspect"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/maps"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/mcp"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/openwebui"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/serveall"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/share"
)

func (d *dashboard) runService(ctx context.Context, spec serviceSpec, model string, report func(string, string)) (string, error) {
	root := d.root
	opener := func(label string) func(string) error {
		return func(url string) error {
			report(label, url)
			return nil
		}
	}
	switch spec.name {
	case "browse":
		if spec.target != "" && (!isFileName(spec.target) || !strings.EqualFold(path.Ext(spec.target), ".zim")) {
			return "", fmt.Errorf("invalid archive: %s", spec.target)
		}
		return "", browse.RunForBrowser(ctx, io.Discard, root, spec.target, opener("Open in Kiwix"))
	case "apps":
		if !isFileName(spec.target) {
			return "", fmt.Errorf("invalid app: %s", spec.target)
		}
		return "", apps.Run(ctx, io.Discard, root, spec.target, opener("Open app"))
	case "documents":
		return "", folder.DocumentsForDashboard(ctx, io.Discard, root, spec.target, d.port, opener("Open documents"))
	case "maps":
		if _, err := os.Stat(filepath.Join(root, "apps", "map", "index.html")); err != nil {
			return "", fmt.Errorf("the map viewer is missing from this drive")
		}
		return "", maps.Run(ctx, io.Discard, root, opener("Open map"))
	case "workspace":
		if _, missing := folder.WorkspaceMissing(root); missing {
			return "The workspace folder appears when an assistant saves a file there.", nil
		}
		return "", folder.WorkspaceForDashboard(ctx, io.Discard, root, d.port, opener("Browse workspace"))
	case "open-webui":
		if model != "" {
			model = filepath.Join(root, "models", model)
		}
		return "", openwebui.RunForBrowser(ctx, io.Discard, root, model, opener("Open WebUI"))
	case "wiki":
		lease, release, err := d.wiki.acquire(ctx, root)
		if err != nil {
			return "", err
		}
		defer release()
		report("Open Wiki", lease.instance.URL)
		select {
		case <-ctx.Done():
			return "", nil
		case <-lease.done:
			return "", lease.err
		}
	case "inspect":
		var output bytes.Buffer
		err := inspect.Run(&output, root)
		return output.String(), err
	case "serve-all":
		return "", serveall.RunForBrowser(ctx, root, report, func(ctx context.Context) (string, <-chan struct{}, func(), error) {
			lease, release, err := d.wiki.acquire(ctx, root)
			if err != nil {
				return "", nil, nil, err
			}
			return lease.instance.URL, lease.done, release, nil
		})
	case "share":
		return "", share.RunWithURL(ctx, io.Discard, root, func(url string) { report("LAN address", url) })
	case "mcp-serve":
		listener, _, err := netutil.Listen("127.0.0.1", 8090)
		if err != nil {
			return "", err
		}
		srv := mcp.NewDriveServer(root, mcp.DriveOptions{})
		defer srv.Close()
		report("MCP endpoint", "http://"+listener.Addr().String()+mcp.StreamableHTTPPath)
		return "", srv.ServeStreamableHTTP(ctx, listener)
	default:
		return "", fmt.Errorf("unsupported browser action %q", spec.name)
	}
}

func isFileName(value string) bool {
	return filepath.IsLocal(value) && value != "." && !strings.HasPrefix(value, ".") && !strings.ContainsAny(value, "/\\")
}
