package openwebui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAgentToolsEnvironmentMakesOnlineSearchAndPDFAttachmentsExplicit(t *testing.T) {
	env := Environment(Launch{
		ContextSize: 8192, Port: 8084,
		MCPURL: "http://127.0.0.1:8090/mcp", WebSearchURL: "http://127.0.0.1:8090/web/mcp",
		WebSearchProvider: "SearXNG", PDFUploadURL: "http://127.0.0.1:8087", PDFUploadToken: "test-token",
	})
	for key, want := range map[string]string{
		"ENABLE_WEB_SEARCH": "False", "OFFLINE_MODE": "true",
		"CONTENT_EXTRACTION_ENGINE": "external", "EXTERNAL_DOCUMENT_LOADER_URL": "http://127.0.0.1:8087",
		"EXTERNAL_DOCUMENT_LOADER_API_KEY": "test-token", "VIEW_FILE_DEFAULT_MAX_CHARS": "2048",
	} {
		if env[key] != want {
			t.Errorf("%s = %q, want %q", key, env[key], want)
		}
	}
	metadata := struct {
		Capabilities map[string]bool `json:"capabilities"`
	}{}
	if err := json.Unmarshal([]byte(env["DEFAULT_MODEL_METADATA"]), &metadata); err != nil {
		t.Fatal(err)
	}
	if metadata.Capabilities["file_context"] || env["RAG_FULL_CONTEXT"] == "True" {
		t.Error("full context injection would remove native attachment tools and overflow small models")
	}
	if !strings.Contains(env["DEFAULT_INTERFACE_SETTINGS"], "view_file") {
		t.Error("browser chats should use native attachment tools without embeddings")
	}
	if !strings.Contains(env["TOOL_SERVER_CONNECTIONS"], "Web search (SearXNG)") {
		t.Errorf("missing named web search connection: %s", env["TOOL_SERVER_CONNECTIONS"])
	}
}
