package serveall

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/llamaserve"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/netutil"
	searchserver "github.com/pkronstrom/svalbard/drive-runtime/internal/search/server"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/settings"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/wiki"
)

type Service struct {
	Name string
	Bind string
}

func PlanServices(driveRoot string, binaries map[string]string) []Service {
	var plan []Service
	if binaries["kiwix-serve"] != "" && hasMatchingFiles(filepath.Join(driveRoot, "zim"), ".zim") {
		plan = append(plan, Service{Name: "kiwix"})
	}
	if binaries["llama-server"] != "" && hasChatModel(driveRoot) {
		plan = append(plan, Service{Name: "llm"})
	}
	plan = append(plan, Service{Name: "files"})
	if _, err := os.Stat(filepath.Join(driveRoot, "apps", "map", "index.html")); err == nil {
		plan = append(plan, Service{Name: "map"})
	}
	if binaries["silverbullet"] != "" {
		// The wiki has no login, so it never follows the bind address.
		plan = append(plan, Service{Name: "wiki", Bind: "127.0.0.1"})
	}
	return plan
}

func Run(ctx context.Context, stdout io.Writer, driveRoot, bind string) error {
	return run(ctx, stdout, driveRoot, bind, nil, nil)
}

// RunForBrowser starts the same services on loopback. It reports child
// servers once they answer and lets the dashboard share a single wiki.
func RunForBrowser(ctx context.Context, driveRoot string, ready func(string, string), openWiki func(context.Context) (string, <-chan struct{}, func(), error)) error {
	return run(ctx, io.Discard, driveRoot, "127.0.0.1", ready, openWiki)
}

func run(ctx context.Context, stdout io.Writer, driveRoot, bind string, ready func(string, string), openWiki func(context.Context) (string, <-chan struct{}, func(), error)) error {
	if bind == "" {
		bind = "127.0.0.1"
	}
	binaries := map[string]string{}
	if path, err := binary.Resolve("kiwix-serve", driveRoot); err == nil {
		binaries["kiwix-serve"] = path
	}
	if path, err := binary.Resolve("llama-server", driveRoot); err == nil {
		binaries["llama-server"] = path
	}
	if path, err := binary.ResolveOnDrive("silverbullet", driveRoot); err == nil {
		binaries["silverbullet"] = path
	}
	plan := PlanServices(driveRoot, binaries)

	ctx, cancel := context.WithCancel(ctx)
	var children sync.WaitGroup
	var releasePorts []func()
	failures := make(chan error, 2)
	defer func() {
		cancel()
		children.Wait()
		for _, release := range releasePorts {
			release()
		}
	}()
	publish := func(label, url string) {
		if ready != nil {
			ready(label, url)
		}
	}
	childPort := func(preferred int) (int, func(), error) {
		if ready != nil {
			return netutil.ReserveEphemeralPort(bind)
		}
		port, err := netutil.FindAvailablePort(bind, preferred)
		return port, func() {}, err
	}
	start := func(cmd *exec.Cmd, label, url, healthPath string) error {
		cmd.Stdout, cmd.Stderr = stdout, stdout
		if err := cmd.Start(); err != nil {
			return err
		}
		children.Go(func() { watchChild(ctx, cmd, label, url, healthPath, ready, failures) })
		return nil
	}

	if binaries["kiwix-serve"] != "" && hasMatchingFiles(filepath.Join(driveRoot, "zim"), ".zim") {
		port, release, err := childPort(8080)
		if err != nil {
			return err
		}
		releasePorts = append(releasePorts, release)
		zims, _ := searchserver.ZIMFiles(driveRoot)
		args := []string{"--port", fmt.Sprintf("%d", port), "--address", bind}
		args = append(args, zims...)
		url := fmt.Sprintf("http://%s:%d", bind, port)
		if err := start(exec.CommandContext(ctx, binaries["kiwix-serve"], args...), "Kiwix", url, "/"); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Kiwix:  %s\n", url)
	}

	if binaries["llama-server"] != "" {
		model, err := firstChatModel(driveRoot)
		if err == nil {
			port, release, err := childPort(8082)
			if err != nil {
				return err
			}
			releasePorts = append(releasePorts, release)
			llamaArgs := []string{"-m", model, "--port", fmt.Sprintf("%d", port), "--host", bind}
			llamaArgs = append(llamaArgs, llamaserve.ExtraFlags(model, llamaserve.ContextForHost())...)
			url := fmt.Sprintf("http://%s:%d", bind, port)
			if err := start(exec.CommandContext(ctx, binaries["llama-server"], llamaArgs...), "Model server", url, "/health"); err != nil {
				return err
			}
			fmt.Fprintf(stdout, "LLM:    %s\n", url)
		}
	}

	listener, filePort, err := netutil.Listen(bind, 8083)
	if err != nil {
		return err
	}
	defer listener.Close()
	server := &http.Server{Handler: http.FileServer(http.Dir(driveRoot))}
	defer server.Close()
	go func() {
		_ = server.Serve(listener)
	}()
	filesURL := fmt.Sprintf("http://%s:%d", bind, filePort)
	fmt.Fprintf(stdout, "Files:  %s\n", filesURL)
	publish("Browse drive files", filesURL)
	if containsService(plan, "map") {
		fmt.Fprintf(stdout, "Map:    %s/apps/map/\n", filesURL)
		publish("Map", filesURL+"/apps/map/")
	}

	if containsService(plan, "wiki") {
		// The wiki remains on loopback even when the other services bind
		// elsewhere. Anyone who can reach its editor can write to it.
		if openWiki == nil {
			openWiki = func(ctx context.Context) (string, <-chan struct{}, func(), error) {
				instance, err := wiki.Start(ctx, driveRoot, settings.OpenForAction(driveRoot, stdout))
				if err != nil {
					return "", nil, nil, err
				}
				return instance.URL, nil, instance.Close, nil
			}
		}
		address, exited, release, err := openWiki(ctx)
		if err != nil {
			fmt.Fprintf(stdout, "Wiki:   not started: %v\n", err)
		} else {
			defer release()
			fmt.Fprintf(stdout, "Wiki:   %s (local only)\n", address)
			publish("Wiki (local only)", address)
			if exited != nil {
				children.Go(func() {
					select {
					case <-exited:
						publish("Wiki (local only)", "")
					case <-ctx.Done():
					}
				})
			}
		}
	}

	select {
	case <-ctx.Done():
		return nil
	case err := <-failures:
		return err
	}
}

func watchChild(ctx context.Context, cmd *exec.Cmd, label, url, healthPath string, ready func(string, string), failures chan<- error) {
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	if ready == nil {
		<-exited
		return
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	probe := ticker.C
	client := &http.Client{
		Timeout:       time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	defer client.CloseIdleConnections()
	for {
		select {
		case <-ctx.Done():
			<-exited
			return
		case err := <-exited:
			if ctx.Err() == nil {
				if err == nil {
					err = fmt.Errorf("process exited")
				}
				failures <- fmt.Errorf("%s stopped: %w", label, err)
			}
			return
		case <-probe:
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+healthPath, nil)
			if err != nil {
				continue
			}
			response, err := client.Do(req)
			if err != nil {
				continue
			}
			response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 400 {
				ready(label, url)
				ticker.Stop()
				probe = nil
			}
		}
	}
}

func containsService(plan []Service, name string) bool {
	for _, service := range plan {
		if service.Name == name {
			return true
		}
	}
	return false
}

func hasMatchingFiles(root, ext string) bool {
	matches, _ := filepath.Glob(filepath.Join(root, "*"+ext))
	return len(matches) > 0
}

func hasChatModel(driveRoot string) bool {
	_, err := firstChatModel(driveRoot)
	return err == nil
}

func firstChatModel(driveRoot string) (string, error) {
	models, err := filepath.Glob(filepath.Join(driveRoot, "models", "*.gguf"))
	if err != nil {
		return "", err
	}
	for _, model := range models {
		base := filepath.Base(model)
		if strings.HasPrefix(base, "._") {
			continue
		}
		return model, nil
	}
	return "", fmt.Errorf("no chat model found")
}
