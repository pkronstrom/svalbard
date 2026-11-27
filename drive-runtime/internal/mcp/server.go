package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gomcp "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/pkronstrom/svalbard/drive-runtime/internal/llamaserve"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/search/engine"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/websearch"
	"github.com/pkronstrom/svalbard/drive-runtime/internal/wiki"
)

// Server wraps an mcp-go MCPServer and routes calls to Capability instances.
type Server struct {
	inner *mcpserver.MCPServer
	caps  []Capability
}

// NewServer creates an MCP server that exposes each capability as a tool.
func NewServer(caps ...Capability) *Server {
	return newServer(caps)
}

func newServer(caps []Capability, options ...mcpserver.ServerOption) *Server {
	inner := mcpserver.NewMCPServer(
		"svalbard-drive",
		"0.2.0",
		append([]mcpserver.ServerOption{mcpserver.WithToolCapabilities(false)}, options...)...,
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
		gomcp.WithReadOnlyHintAnnotation(!action.Writes),
		gomcp.WithDestructiveHintAnnotation(action.Writes),
		gomcp.WithIdempotentHintAnnotation(!action.Writes),
		gomcp.WithOpenWorldHintAnnotation(action.OpenWorld),
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

// DriveOptions is what the drive's MCP server is told about its client.
type DriveOptions struct {
	// Context is the context window of the client's model in tokens, 0 when
	// it is not known (a client the drive did not start). Below the Balanced
	// tier the server offers no wiki tools and reads shorter pieces, so that
	// a lookup leaves the model room to answer.
	Context int
	// Stdio: the client started the server as its own process, so it is on
	// this computer and can open a file on the drive by its whole path.
	Stdio bool
	// WebSearch is passed only by launch paths with a valid local opt-in.
	WebSearch *websearch.Search
}

// NewDriveServer creates the drive's MCP server: its capabilities and, on a
// drive with skills, a line of instructions that points a client to them.
func NewDriveServer(driveRoot string, opts DriveOptions) *Server {
	var options []mcpserver.ServerOption
	if meta, _ := LoadMetadata(driveRoot); skillToolsOffered(meta) {
		options = append(options, mcpserver.WithInstructions(skillServerInstructions))
	}
	return newServer(DriveCapabilities(driveRoot, opts), options...)
}

// DriveCapabilities returns the capabilities the drive's MCP server exposes.
// The skill tools are among them on a drive that has skills or JS libraries
// installed, and the wiki tools on one that has the wiki, from the Balanced
// tier up.
func DriveCapabilities(driveRoot string, opts DriveOptions) []Capability {
	meta, _ := LoadMetadata(driveRoot)
	searchTools := NewSearchCapability(driveRoot, meta)
	searchTools.read = ReadLimitsFor(opts.Context)
	caps := []Capability{searchTools}
	if skillToolsOffered(meta) {
		skillTools := newSharedSkillCapability(driveRoot, meta, searchTools)
		skillTools.AbsolutePaths = opts.Stdio
		caps = append(caps, skillTools)
	}
	caps = append(caps,
		NewVaultCapability(driveRoot, meta),
		NewQueryCapability(driveRoot, meta),
	)
	if wikiInstalled(meta) && !smallContext(opts.Context) {
		caps = append(caps, NewWikiCapability(driveRoot))
	}
	if opts.WebSearch != nil {
		caps = append(caps, NewWebCapability(opts.WebSearch))
	}
	return caps
}

// smallContext is true for a context window below the Balanced tier. There
// the eight wiki tools (1,224 tokens of every request) and a full read left
// pi on the Fast tier (8K) a few hundred tokens to answer after one lookup,
// and writing a wiki page needs room for the page. The wiki stays the user's
// in the browser.
func smallContext(ctxSize int) bool {
	return ctxSize > 0 && ctxSize < llamaserve.CtxBalanced
}

// Recipe types whose content the skill tools search.
const (
	skillsRecipeType  = "skills"
	libraryRecipeType = "js-lib"
)

// SkillsInstalled reports whether the drive has a skills recipe installed,
// for the prompts that tell a model to look for a procedure first.
func SkillsInstalled(driveRoot string) bool {
	meta, _ := LoadMetadata(driveRoot)
	return recipeTypeInstalled(meta, skillsRecipeType)
}

// SkillsIndexed reports whether the skill tools have something to search on
// the drive: its index strategy is not "none", and its search index holds a
// skill file or a library quickstart. Whether search.db exists says neither:
// a vault switched to "none" keeps its old index, and skills installed after
// that are never in it.
func SkillsIndexed(driveRoot string) bool {
	if indexStrategy(driveRoot) == "none" {
		return false
	}
	db, err := openSearchDB(driveRoot)
	if err != nil {
		return false
	}
	defer db.Close()
	sources, err := engine.New(db).SkillSources(context.Background())
	return err == nil && len(sources) > 0
}

// indexStrategy is the vault's chosen index strategy as its manifest records
// it (desired.options.index_strategy), "" for the default.
func indexStrategy(driveRoot string) string {
	data, err := os.ReadFile(filepath.Join(driveRoot, "manifest.yaml"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "index_strategy:"); ok {
			return strings.ToLower(strings.Trim(strings.TrimSpace(value), `"'`))
		}
	}
	return ""
}

// LibrariesInstalled reports whether the drive has JS libraries installed.
func LibrariesInstalled(driveRoot string) bool {
	meta, _ := LoadMetadata(driveRoot)
	return recipeTypeInstalled(meta, libraryRecipeType)
}

// skillToolsOffered is true when apply has installed skills or JS libraries.
// The tools only need the index, but offering them on every drive would add
// their definitions to every prompt for skills that are not there.
func skillToolsOffered(meta DriveMetadata) bool {
	return recipeTypeInstalled(meta, skillsRecipeType) || recipeTypeInstalled(meta, libraryRecipeType)
}

// recipeTypeInstalled reports whether a realized recipe has the type.
// recipes.json lists the realized recipes only.
func recipeTypeInstalled(meta DriveMetadata, recipeType string) bool {
	for _, recipe := range meta.Recipes {
		if recipe.Type == recipeType {
			return true
		}
	}
	return false
}

// WikiOffered reports whether the drive's MCP server offers the wiki tools to
// a model with a context window of ctxSize tokens, for the prompts and
// descriptions that tell a model what it can do.
func WikiOffered(driveRoot string, ctxSize int) bool {
	meta, _ := LoadMetadata(driveRoot)
	return wikiInstalled(meta) && !smallContext(ctxSize)
}

// wikiInstalled is true when apply has installed the wiki's recipe. The
// tools only need the page folder, but offering them on every drive would
// add their definitions to every prompt for a wiki nobody can open.
func wikiInstalled(meta DriveMetadata) bool {
	_, ok := meta.Recipes[wiki.RecipeID]
	return ok
}

// DriveToolNames lists the tool names the drive's MCP server registers for a
// model with a context window of ctxSize tokens, for clients that only enable
// tools named up front.
func DriveToolNames(driveRoot string, opts DriveOptions) []string {
	var names []string
	for _, cap := range DriveCapabilities(driveRoot, opts) {
		for _, action := range cap.Actions() {
			names = append(names, toolName(cap.Tool(), action.Name))
		}
		_ = cap.Close()
	}
	return names
}

// Tools returns the tools the server registers, by name, as a client
// receives them.
func (s *Server) Tools() []gomcp.Tool {
	var tools []gomcp.Tool
	for _, tool := range s.inner.ListTools() {
		tools = append(tools, tool.Tool)
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools
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

// FilesStreamableHTTPPath is where the Open WebUI action mounts the file
// tools, beside the drive's server on the same listener.
const FilesStreamableHTTPPath = "/files/mcp"

// ServeStreamableHTTP serves the server over Streamable HTTP on listener until
// ctx is done. Clients that speak neither stdio nor SSE (Open WebUI) connect
// here. The listener is closed on return.
func (s *Server) ServeStreamableHTTP(ctx context.Context, listener net.Listener) error {
	return ServeStreamableHTTP(ctx, listener, map[string]*Server{StreamableHTTPPath: s})
}

// ServeStreamableHTTP serves each server at its path on one listener until ctx
// is done. A client sees every path as a tool server of its own, so it can
// switch one off and keep the other. The listener is closed on return.
func ServeStreamableHTTP(ctx context.Context, listener net.Listener, servers map[string]*Server) error {
	mux := http.NewServeMux()
	httpServer := &http.Server{Handler: mux}
	var streamables []*mcpserver.StreamableHTTPServer
	for path, server := range servers {
		streamable := mcpserver.NewStreamableHTTPServer(server.inner)
		mux.Handle(path, streamable)
		streamables = append(streamables, streamable)
	}

	done := make(chan error, 1)
	go func() { done <- httpServer.Serve(listener) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		// Sessions hold long-lived streams open; closing them first lets the
		// HTTP server drain.
		for _, streamable := range streamables {
			_ = streamable.Shutdown(shutdownCtx)
		}
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
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
