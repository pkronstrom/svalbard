package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

// ToolInfo describes a registered MCP tool for introspection / testing.
type ToolInfo struct {
	Name        string
	Description string
	Actions     []string
}

// Server wraps an mcp-go MCPServer and routes calls to Capability instances.
type Server struct {
	inner *mcpserver.MCPServer
	caps  []Capability
}

// NewServer creates an MCP server that exposes each capability as a tool.
func NewServer(caps ...Capability) *Server {
	inner := mcpserver.NewMCPServer(
		"svalbard-drive",
		"0.1.0",
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
	actions := cap.Actions()
	actionNames := make([]string, len(actions))
	for i, a := range actions {
		actionNames[i] = a.Name
	}

	// Build the tool's input schema: { action: enum, params: object }
	tool := gomcp.NewTool(
		cap.Tool(),
		gomcp.WithDescription(cap.Description()),
		gomcp.WithString("action",
			gomcp.Required(),
			gomcp.Description("Action to perform"),
			gomcp.Enum(actionNames...),
		),
		gomcp.WithObject("params",
			gomcp.Description("Action parameters"),
		),
	)

	handler := s.makeHandler(cap)
	s.inner.AddTool(tool, handler)
}

// makeHandler returns a ToolHandlerFunc that routes to the capability.
func (s *Server) makeHandler(cap Capability) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		action := req.GetString("action", "")
		if action == "" {
			return gomcp.NewToolResultError("missing required parameter: action"), nil
		}

		params := map[string]any{}
		if raw := req.GetArguments(); raw != nil {
			if p, ok := raw["params"]; ok {
				if m, ok := p.(map[string]any); ok {
					params = m
				}
			}
		}

		result, err := cap.Handle(ctx, action, params)
		if err != nil {
			return nil, err
		}

		if result.Data != nil {
			data, jErr := json.Marshal(result.Data)
			if jErr != nil {
				return nil, fmt.Errorf("marshalling result data: %w", jErr)
			}
			return gomcp.NewToolResultText(string(data)), nil
		}
		return gomcp.NewToolResultText(result.Text), nil
	}
}

// Tools returns information about every registered tool (for testing).
func (s *Server) Tools() []ToolInfo {
	tools := s.inner.ListTools()
	out := make([]ToolInfo, 0, len(tools))
	for name, st := range tools {
		var actions []string
		// Extract action enum from the input schema
		if props, ok := st.Tool.InputSchema.Properties["action"]; ok {
			if propMap, ok := props.(map[string]any); ok {
				if enumRaw, ok := propMap["enum"]; ok {
					if enumSlice, ok := enumRaw.([]string); ok {
						actions = enumSlice
					}
				}
			}
		}
		out = append(out, ToolInfo{
			Name:        name,
			Description: st.Tool.Description,
			Actions:     actions,
		})
	}
	return out
}

// ServeStdio starts the server on stdin/stdout (blocking).
func (s *Server) ServeStdio() error {
	return mcpserver.ServeStdio(s.inner)
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
