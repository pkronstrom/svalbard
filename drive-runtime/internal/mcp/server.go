package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// Server wraps an mcp-go MCPServer and routes calls to Capability instances.
type Server struct {
	inner *mcpserver.MCPServer
	caps  []Capability
}

// NewServer creates an MCP server that exposes each capability as a tool.
func NewServer(caps ...Capability) *Server {
	inner := mcpserver.NewMCPServer(
		"svalbard-drive",
		"0.2.0",
		mcpserver.WithToolCapabilities(false),
	)

	s := &Server{inner: inner, caps: caps}

	for _, cap := range caps {
		s.registerCapability(cap)
	}

	return s
}

// registerCapability turns a Capability into an mcp-go tool registration.
func (s *Server) registerCapability(cap Capability) {
	for _, action := range cap.Actions() {
		tool := s.buildActionTool(cap, action)
		handler := s.makeHandler(cap, action.Name)
		s.inner.AddTool(tool, handler)
	}
}

func (s *Server) buildActionTool(cap Capability, action ActionDef) gomcp.Tool {
	options := []gomcp.ToolOption{
		gomcp.WithDescription(action.Desc),
		gomcp.WithTitleAnnotation(toolName(cap.Tool(), action.Name)),
		gomcp.WithReadOnlyHintAnnotation(true),
		gomcp.WithDestructiveHintAnnotation(false),
		gomcp.WithIdempotentHintAnnotation(true),
		gomcp.WithOpenWorldHintAnnotation(false),
	}

	for _, param := range action.Params {
		options = append(options, schemaOption(param))
	}

	return gomcp.NewTool(toolName(cap.Tool(), action.Name), options...)
}

// makeHandler returns a ToolHandlerFunc that routes to the capability.
func (s *Server) makeHandler(cap Capability, actionName string) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		params := map[string]any{}
		if raw := req.GetArguments(); raw != nil {
			for k, v := range raw {
				params[k] = v
			}
		}

		result, err := cap.Handle(ctx, actionName, params)
		if err != nil {
			// Return tool-level error (isError=true) so the AI sees the
			// message and can self-correct, rather than a protocol error.
			return gomcp.NewToolResultError(err.Error()), nil
		}

		if result.Data != nil {
			// Not json.Marshal: it writes "<", ">" and "&" as six-character
			// escapes, which a model pays for and reads worse.
			var data bytes.Buffer
			encoder := json.NewEncoder(&data)
			encoder.SetEscapeHTML(false)
			if jErr := encoder.Encode(result.Data); jErr != nil {
				return nil, fmt.Errorf("marshalling result data: %w", jErr)
			}
			toolResult := gomcp.NewToolResultText(strings.TrimSuffix(data.String(), "\n"))
			if result.Text != "" {
				toolResult.Content = append(toolResult.Content, gomcp.TextContent{Type: gomcp.ContentTypeText, Text: result.Text})
			}
			return toolResult, nil
		}
		return gomcp.NewToolResultText(result.Text), nil
	}
}

// DriveCapabilities returns the capabilities the drive's MCP server exposes.
func DriveCapabilities(driveRoot string) []Capability {
	meta, _ := LoadMetadata(driveRoot)
	return []Capability{
		NewSearchCapability(driveRoot, meta),
		NewVaultCapability(driveRoot, meta),
		NewQueryCapability(driveRoot, meta),
	}
}

// DriveToolNames lists the tool names the drive's MCP server registers, for
// clients that only enable tools named up front.
func DriveToolNames(driveRoot string) []string {
	var names []string
	for _, cap := range DriveCapabilities(driveRoot) {
		for _, action := range cap.Actions() {
			names = append(names, toolName(cap.Tool(), action.Name))
		}
		_ = cap.Close()
	}
	return names
}

func toolName(prefix, action string) string {
	if prefix == action {
		return action
	}
	return prefix + "_" + action
}

func schemaOption(param ParamDef) gomcp.ToolOption {
	opts := []gomcp.PropertyOption{
		gomcp.Description(param.Desc),
	}
	if param.Required {
		opts = append(opts, gomcp.Required())
	}
	if len(param.Enum) > 0 {
		opts = append(opts, gomcp.Enum(param.Enum...))
	}
	if param.Default != nil {
		switch value := param.Default.(type) {
		case string:
			opts = append(opts, gomcp.DefaultString(value))
		case int:
			opts = append(opts, gomcp.DefaultNumber(float64(value)))
		case float64:
			opts = append(opts, gomcp.DefaultNumber(value))
		case bool:
			opts = append(opts, gomcp.DefaultBool(value))
		}
	}

	switch param.Type {
	case "integer", "number":
		return gomcp.WithNumber(param.Name, opts...)
	case "boolean":
		return gomcp.WithBoolean(param.Name, opts...)
	default:
		return gomcp.WithString(param.Name, opts...)
	}
}

// ServeStdio starts the server on stdin/stdout (blocking).
func (s *Server) ServeStdio() error {
	return mcpserver.ServeStdio(s.inner)
}

// ServeSSE starts the server as an SSE HTTP server on the given address (blocking).
func (s *Server) ServeSSE(addr string) error {
	sse := mcpserver.NewSSEServer(s.inner, mcpserver.WithBaseURL("http://"+addr))
	return sse.Start(addr)
}

// StreamableHTTPPath is where ServeStreamableHTTP mounts the server.
const StreamableHTTPPath = "/mcp"

// ServeStreamableHTTP serves the server over Streamable HTTP on listener until
// ctx is done. Clients that speak neither stdio nor SSE (Open WebUI) connect
// here. The listener is closed on return.
func (s *Server) ServeStreamableHTTP(ctx context.Context, listener net.Listener) error {
	mux := http.NewServeMux()
	httpServer := &http.Server{Handler: mux}
	streamable := mcpserver.NewStreamableHTTPServer(s.inner, mcpserver.WithStreamableHTTPServer(httpServer))
	mux.Handle(StreamableHTTPPath, streamable)

	done := make(chan error, 1)
	go func() { done <- httpServer.Serve(listener) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := streamable.Shutdown(shutdownCtx); err != nil {
			_ = httpServer.Close()
		}
		if err := <-done; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

// Close shuts down all capabilities.
func (s *Server) Close() error {
	var firstErr error
	for _, cap := range s.caps {
		if err := cap.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
