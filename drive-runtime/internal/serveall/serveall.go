package serveall

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

	var commands []*exec.Cmd
	var servers []*http.Server

	if binaries["kiwix-serve"] != "" && hasMatchingFiles(filepath.Join(driveRoot, "zim"), ".zim") {
		port, err := netutil.FindAvailablePort(bind, 8080)
		if err != nil {
			return err
		}
		zims, _ := searchserver.ZIMFiles(driveRoot)
		args := []string{"--port", fmt.Sprintf("%d", port), "--address", bind}
		args = append(args, zims...)
		cmd := exec.CommandContext(ctx, binaries["kiwix-serve"], args...)
		cmd.Stdout = stdout
		cmd.Stderr = stdout
		if err := cmd.Start(); err != nil {
			return err
		}
		commands = append(commands, cmd)
		fmt.Fprintf(stdout, "Kiwix:  http://%s:%d\n", bind, port)
	}

	if binaries["llama-server"] != "" {
		model, err := firstChatModel(driveRoot)
		if err == nil {
			port, err := netutil.FindAvailablePort(bind, 8082)
			if err != nil {
				return err
			}
			llamaArgs := []string{"-m", model, "--port", fmt.Sprintf("%d", port), "--host", bind}
			llamaArgs = append(llamaArgs, llamaserve.ExtraFlags(model, llamaserve.ContextForHost())...)
			cmd := exec.CommandContext(ctx, binaries["llama-server"], llamaArgs...)
			cmd.Stdout = stdout
			cmd.Stderr = stdout
			if err := cmd.Start(); err != nil {
				return err
			}
			commands = append(commands, cmd)
			fmt.Fprintf(stdout, "LLM:    http://%s:%d\n", bind, port)
		}
	}

	filePort, err := netutil.FindAvailablePort(bind, 8083)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", bind, filePort))
	if err != nil {
		return err
	}
	server := &http.Server{Handler: http.FileServer(http.Dir(driveRoot))}
	servers = append(servers, server)
	go func() {
		_ = server.Serve(listener)
	}()
	fmt.Fprintf(stdout, "Files:  http://%s:%d\n", bind, filePort)
	if containsService(plan, "map") {
		fmt.Fprintf(stdout, "Map:    http://%s:%d/apps/map/\n", bind, filePort)
	}

	if containsService(plan, "wiki") {
		// Whatever the other services bind, the wiki stays on this computer:
		// anyone who can reach it can write it. Its pages are files on the
		// drive, which Files above shares read-only like the rest.
		instance, err := wiki.Start(ctx, driveRoot, settings.OpenForAction(driveRoot, stdout))
		if err != nil {
			// The other services are worth having without it.
			fmt.Fprintf(stdout, "Wiki:   not started: %v\n", err)
		} else {
			defer instance.Close()
			fmt.Fprintf(stdout, "Wiki:   %s (local only)\n", instance.URL)
		}
	}

	<-ctx.Done()
	for _, server := range servers {
		_ = server.Close()
	}
	for _, cmd := range commands {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	return nil
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
