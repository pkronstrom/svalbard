package runtimeserveall

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

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimebinary"
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
	return plan
}

func Run(ctx context.Context, stdout io.Writer, driveRoot, bind string) error {
	if bind == "" {
		bind = "127.0.0.1"
	}
	binaries := map[string]string{}
	if path, err := runtimebinary.Resolve("kiwix-serve", driveRoot, platform.Detect); err == nil {
		binaries["kiwix-serve"] = path
	}
	if path, err := runtimebinary.Resolve("llama-server", driveRoot, platform.Detect); err == nil {
		binaries["llama-server"] = path
	}
	plan := PlanServices(driveRoot, binaries)

	var commands []*exec.Cmd
	var servers []*http.Server

	if binaries["kiwix-serve"] != "" && hasMatchingFiles(filepath.Join(driveRoot, "zim"), ".zim") {
		port, err := findAvailablePort(bind, 8080)
		if err != nil {
			return err
		}
		zims, _ := filepath.Glob(filepath.Join(driveRoot, "zim", "*.zim"))
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
			port, err := findAvailablePort(bind, 8082)
			if err != nil {
				return err
			}
			cmd := exec.CommandContext(ctx, binaries["llama-server"], "-m", model, "--port", fmt.Sprintf("%d", port), "--host", bind)
			cmd.Stdout = stdout
			cmd.Stderr = stdout
			if err := cmd.Start(); err != nil {
				return err
			}
			commands = append(commands, cmd)
			fmt.Fprintf(stdout, "LLM:    http://%s:%d\n", bind, port)
		}
	}

	filePort, err := findAvailablePort(bind, 8083)
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
		base := strings.ToLower(filepath.Base(model))
		if strings.Contains(base, "embed") || strings.Contains(base, "nomic-embed") ||
			strings.Contains(base, "bge-") || strings.Contains(base, "e5-") || strings.Contains(base, "arctic-embed") {
			continue
		}
		return model, nil
	}
	return "", fmt.Errorf("no chat model found")
}

func findAvailablePort(host string, preferred int) (int, error) {
	for port := preferred; port < preferred+20; port++ {
		listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
		if err == nil {
			listener.Close()
			return port, nil
		}
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
