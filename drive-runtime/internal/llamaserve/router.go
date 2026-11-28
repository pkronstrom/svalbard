package llamaserve

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WriteRouterPresets gives each chat model the options used by the
// single-model launcher. llama-server reads these values as INI, with
// unquoted paths and a separate file for the multiline Qwen template.
func WriteRouterPresets(dir, modelDir string, models []string, ctxSize int) (string, int, error) {
	var config strings.Builder
	minimumContext := 0
	config.WriteString("version = 1\n")
	templatePath := filepath.Join(dir, "qwen3.8.jinja")
	wroteTemplate := false
	for _, filename := range models {
		name := strings.TrimSuffix(filename, filepath.Ext(filename))
		path := filepath.Join(modelDir, filename)
		if name == "*" || name == "" || strings.TrimSpace(name) != name || strings.ContainsAny(name, "]:\r\n") {
			return "", 0, fmt.Errorf("model name %q is incompatible with the llama-server preset format", name)
		}
		fmt.Fprintf(&config, "\n[%s]\n", name)
		modelContext := ContextForModel(path, ctxSize)
		if minimumContext == 0 || modelContext < minimumContext {
			minimumContext = modelContext
		}
		options := append([]modelOption{{"--model", path}}, modelOptions(path, modelContext)...)
		for _, option := range options {
			flag, value := strings.TrimPrefix(option.flag, "--"), option.value
			switch flag {
			case "chat-template":
				if !wroteTemplate {
					if err := os.WriteFile(templatePath, []byte(value), 0o600); err != nil {
						return "", 0, err
					}
					wroteTemplate = true
				}
				flag, value = "chat-template-file", templatePath
			default:
				if value == "" {
					value = "true"
				}
			}
			if strings.ContainsAny(value, ";#\r\n") || strings.TrimSpace(value) != value {
				return "", 0, fmt.Errorf("model %q has an incompatible %s value in the llama-server preset", name, flag)
			}
			fmt.Fprintf(&config, "%s = %s\n", flag, value)
		}
	}
	path := filepath.Join(dir, "models.ini")
	if err := os.WriteFile(path, []byte(config.String()), 0o600); err != nil {
		return "", 0, err
	}
	return path, minimumContext, nil
}
