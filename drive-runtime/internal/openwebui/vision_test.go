package openwebui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestVisionProxyMarksOnlyMatchedModelsAndForwardsChat(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			fmt.Fprint(w, `{"data":[{"id":"image-model","info":{"meta":{"capabilities":{"file_upload":true}}}},{"id":"text-model"}]}`)
		case "/v1/chat/completions":
			raw, _ := io.ReadAll(r.Body)
			fmt.Fprint(w, string(raw))
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	target, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(visionProxy(target, map[string]bool{"image-model": true}))
	defer server.Close()
	response, err := http.Get(server.URL + "/v1/models")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload struct {
		Data []struct {
			ID   string
			Info struct {
				Meta struct{ Capabilities map[string]bool }
			}
		}
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data) != 2 || !payload.Data[0].Info.Meta.Capabilities["vision"] ||
		!payload.Data[0].Info.Meta.Capabilities["file_upload"] || payload.Data[1].Info.Meta.Capabilities["vision"] {
		t.Fatalf("unexpected model metadata: %+v", payload)
	}
	const imagePrompt = `{"model":"image-model","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AA=="}}]}]}`
	chat, err := http.Post(server.URL+"/v1/chat/completions", "application/json", strings.NewReader(imagePrompt))
	if err != nil {
		t.Fatal(err)
	}
	defer chat.Body.Close()
	body, err := io.ReadAll(chat.Body)
	if err != nil || string(body) != imagePrompt {
		t.Fatalf("image request changed through the proxy: %q, %v", body, err)
	}
}
