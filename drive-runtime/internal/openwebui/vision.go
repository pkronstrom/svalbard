package openwebui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
)

// visionProxy adds the per-model metadata that Open WebUI uses to show the
// image control. Completions and all other endpoints pass through to llama.cpp.
func visionProxy(upstream *url.URL, visionModels map[string]bool) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.Request.Method != http.MethodGet || resp.Request.URL.Path != "/v1/models" ||
			resp.StatusCode != http.StatusOK {
			return nil
		}
		return addVisionMetadata(resp, visionModels)
	}
	return proxy
}

func addVisionMetadata(resp *http.Response, visionModels map[string]bool) error {
	defer resp.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&payload); err != nil {
		return err
	}
	if models, ok := payload["data"].([]any); ok {
		for _, value := range models {
			model, ok := value.(map[string]any)
			if !ok {
				continue
			}
			id, _ := model["id"].(string)
			if !visionModels[id] {
				continue
			}
			info := objectField(model, "info")
			meta := objectField(info, "meta")
			objectField(meta, "capabilities")["vision"] = true
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	return nil
}

func objectField(parent map[string]any, field string) map[string]any {
	child, ok := parent[field].(map[string]any)
	if !ok || child == nil {
		child = make(map[string]any)
		parent[field] = child
	}
	return child
}
