package runtimeagent

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/runtimebinary"
)

func ResolveModel(driveRoot, selected string) (string, error) {
	if selected != "" {
		info, err := os.Stat(selected)
		if err != nil || info.IsDir() {
			return "", fmt.Errorf("model not found: %s", selected)
		}
		return selected, nil
	}

	pattern := filepath.Join(driveRoot, "models", "*.gguf")
	models, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}
	filtered := models[:0]
	for _, model := range models {
		base := strings.ToLower(filepath.Base(model))
		if strings.HasPrefix(base, "._") {
			continue
		}
		if strings.Contains(base, "embed") || strings.Contains(base, "nomic-embed") ||
			strings.Contains(base, "bge-") || strings.Contains(base, "e5-") || strings.Contains(base, "arctic-embed") {
			continue
		}
		filtered = append(filtered, model)
	}
	sort.Strings(filtered)
	if len(filtered) == 0 {
		return "", fmt.Errorf("no chat-capable GGUF models found in models/")
	}
	return filtered[0], nil
}

func ClientEnvironment(baseURL, modelName string) map[string]string {
	return map[string]string{
		"OPENAI_API_KEY":       "local",
		"OPENAI_BASE_URL":      baseURL,
		"OPENAI_API_BASE":      baseURL,
		"OPENAI_MODEL":         modelName,
		"OPENAI_DEFAULT_MODEL": modelName,
	}
}

func Run(ctx context.Context, stdout io.Writer, driveRoot, clientName, selectedModel string) error {
	clientBin, err := runtimebinary.Resolve(clientName, driveRoot, platform.Detect)
	if err != nil {
		return fmt.Errorf("%s not found", clientName)
	}
	llamaBin, err := runtimebinary.Resolve("llama-server", driveRoot, platform.Detect)
	if err != nil {
		return fmt.Errorf("llama-server not found")
	}
	model, err := ResolveModel(driveRoot, selectedModel)
	if err != nil {
		return err
	}
	port, err := findAvailablePort("127.0.0.1", 8082)
	if err != nil {
		return err
	}

	modelName := strings.TrimSuffix(filepath.Base(model), filepath.Ext(model))
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/v1", port)

	fmt.Fprintf(stdout, "Starting llama-server with %s\n", modelName)
	llamaCmd := exec.CommandContext(ctx, llamaBin, "-m", model, "--jinja", "--host", "127.0.0.1", "--port", fmt.Sprintf("%d", port))
	llamaCmd.Stdout = stdout
	llamaCmd.Stderr = stdout
	if err := llamaCmd.Start(); err != nil {
		return err
	}
	defer func() {
		if llamaCmd.Process != nil {
			_ = llamaCmd.Process.Kill()
		}
	}()

	fmt.Fprintf(stdout, "Launching %s against %s\n", clientName, modelName)
	clientCmd := exec.CommandContext(ctx, clientBin)
	clientCmd.Stdin = os.Stdin
	clientCmd.Stdout = os.Stdout
	clientCmd.Stderr = os.Stderr
	clientCmd.Env = append(os.Environ(), envMapToList(ClientEnvironment(baseURL, modelName))...)
	return clientCmd.Run()
}

func envMapToList(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for key, value := range values {
		out = append(out, key+"="+value)
	}
	sort.Strings(out)
	return out
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
