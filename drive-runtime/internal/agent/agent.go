package agent

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/binary"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/platform"
)

type LaunchConfig struct {
	Args []string
	Env  map[string]string
}

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

func PrepareClientLaunchConfig(driveRoot, clientName, hostRoot, baseURL, modelName string) (LaunchConfig, error) {
	cfg := LaunchConfig{Env: map[string]string{}}
	runtimeRoot := filepath.Join(driveRoot, ".svalbard", "runtime", clientName)
	configRoot := filepath.Join(runtimeRoot, "config")
	cacheRoot := filepath.Join(runtimeRoot, "cache")
	dataRoot := filepath.Join(runtimeRoot, "data")
	homeRoot := filepath.Join(runtimeRoot, "home")

	switch clientName {
	case "opencode":
		for _, dir := range []string{configRoot, cacheRoot, dataRoot, homeRoot} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return LaunchConfig{}, err
			}
		}
		configPath := filepath.Join(configRoot, "opencode.json")
		content := fmt.Sprintf(`{
  "$schema": "https://opencode.ai/config.json",
  "enabled_providers": ["llama.cpp"],
  "model": "llama.cpp/%[1]s",
  "small_model": "llama.cpp/%[1]s",
  "provider": {
    "llama.cpp": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "llama-server (local)",
      "options": {
        "baseURL": "%[2]s",
        "apiKey": "local"
      },
      "models": {
        "%[1]s": {
          "name": "%[1]s"
        }
      }
    }
  }
}
`, modelName, baseURL)
		if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
			return LaunchConfig{}, err
		}
		cfg.Args = []string{"-m", "llama.cpp/" + modelName}
		cfg.Env = map[string]string{
			"HOME":            homeRoot,
			"XDG_CONFIG_HOME": configRoot,
			"XDG_CACHE_HOME":  cacheRoot,
			"XDG_DATA_HOME":   dataRoot,
			"OPENCODE_CONFIG": configPath,
		}
	case "goose":
		for _, dir := range []string{configRoot, cacheRoot, dataRoot, homeRoot} {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return LaunchConfig{}, err
			}
		}
		cfg.Env = map[string]string{
			"HOME":            homeRoot,
			"XDG_CONFIG_HOME": configRoot,
			"XDG_CACHE_HOME":  cacheRoot,
			"XDG_DATA_HOME":   dataRoot,
			"GOOSE_PROVIDER":  "openai",
			"GOOSE_MODEL":     modelName,
			"OPENAI_API_KEY":  "local",
			"OPENAI_HOST":     hostRoot,
		}
	default:
		cfg.Env = map[string]string{}
	}
	return cfg, nil
}

func Run(ctx context.Context, stdout io.Writer, driveRoot, clientName, selectedModel string) error {
	clientBin, err := binary.Resolve(clientName, driveRoot, platform.Detect)
	if err != nil {
		return fmt.Errorf("%s not found", clientName)
	}
	llamaBin, err := binary.Resolve("llama-server", driveRoot, platform.Detect)
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
	hostRoot := fmt.Sprintf("http://127.0.0.1:%d", port)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/v1", port)
	logFile, logPath, err := openRuntimeLogFile(driveRoot, clientName, "llama-server.log")
	if err != nil {
		return err
	}
	defer logFile.Close()

	fmt.Fprintf(stdout, "Starting llama-server with %s\n", modelName)
	fmt.Fprintf(stdout, "llama-server log: %s\n", logPath)
	llamaCmd := exec.CommandContext(ctx, llamaBin, "-m", model, "--jinja", "--host", "127.0.0.1", "--port", fmt.Sprintf("%d", port))
	llamaCmd.Stdout = logFile
	llamaCmd.Stderr = logFile
	if err := llamaCmd.Start(); err != nil {
		return err
	}
	defer func() {
		if llamaCmd.Process != nil {
			_ = llamaCmd.Process.Kill()
		}
	}()
	if err := waitForHTTPReady(hostRoot + "/health"); err != nil {
		return err
	}

	fmt.Fprintf(stdout, "Launching %s against %s\n", clientName, modelName)
	launchCfg, err := PrepareClientLaunchConfig(driveRoot, clientName, hostRoot, baseURL, modelName)
	if err != nil {
		return err
	}
	clientCmd := exec.CommandContext(ctx, clientBin, launchCfg.Args...)
	clientCmd.Stdin = os.Stdin
	clientCmd.Stdout = os.Stdout
	clientCmd.Stderr = os.Stderr
	clientCmd.Env = append(os.Environ(), envMapToList(ClientEnvironment(baseURL, modelName))...)
	clientCmd.Env = append(clientCmd.Env, envMapToList(launchCfg.Env)...)
	clientCmd.Dir = driveRoot
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

func waitForHTTPReady(url string) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("llama-server did not become healthy in time")
}

func openRuntimeLogFile(driveRoot, clientName, filename string) (*os.File, string, error) {
	runtimeRoot := filepath.Join(driveRoot, ".svalbard", "runtime", clientName)
	if err := os.MkdirAll(runtimeRoot, 0o755); err != nil {
		return nil, "", err
	}
	path := filepath.Join(runtimeRoot, filename)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, "", err
	}
	return file, path, nil
}
