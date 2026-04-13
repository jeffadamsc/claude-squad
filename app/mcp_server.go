package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"claude-squad/log"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// MCPIndexServer exposes the SessionIndexer via MCP tools.
// It runs an HTTP server that Claude Code sessions can connect to.
type MCPIndexServer struct {
	mu                sync.RWMutex
	api               *SessionAPI
	standaloneIndexer *SessionIndexer
	server            *server.StreamableHTTPServer
	listener          net.Listener
	port              int
}

// NewMCPIndexServer creates a new MCP server backed by the SessionAPI's indexers.
func NewMCPIndexServer(api *SessionAPI) *MCPIndexServer {
	return &MCPIndexServer{api: api}
}

// NewMCPIndexServerStandalone creates an MCP server backed by a single indexer.
// Use this for benchmarks or standalone operation without SessionAPI.
func NewMCPIndexServerStandalone(indexer *SessionIndexer) *MCPIndexServer {
	return &MCPIndexServer{
		standaloneIndexer: indexer,
	}
}

// Start starts the MCP HTTP server on a dynamic port.
// Returns the port number for clients to connect to.
func (m *MCPIndexServer) Start() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.listener != nil {
		return m.port, nil // already running
	}

	// Create the MCP server with tool capabilities
	mcpServer := server.NewMCPServer(
		"claude-squad-index",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register tools
	m.registerTools(mcpServer)

	// Create HTTP transport with stateless mode (each request is independent)
	httpServer := server.NewStreamableHTTPServer(mcpServer,
		server.WithStateLess(true),
		server.WithEndpointPath("/mcp"),
	)
	m.server = httpServer

	// Listen on a dynamic port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("failed to listen: %w", err)
	}
	m.listener = listener
	m.port = listener.Addr().(*net.TCPAddr).Port

	// Create HTTP mux
	mux := http.NewServeMux()
	// Per-session endpoint: /mcp/<session-id>
	mux.HandleFunc("/mcp/", func(w http.ResponseWriter, r *http.Request) {
		// Extract session ID from path
		path := strings.TrimPrefix(r.URL.Path, "/mcp/")
		sessionID := strings.Split(path, "/")[0]
		if sessionID == "" {
			http.Error(w, "session ID required in path", http.StatusBadRequest)
			return
		}
		// Store session ID in context for tool handlers
		ctx := context.WithValue(r.Context(), sessionIDKey, sessionID)
		httpServer.ServeHTTP(w, r.WithContext(ctx))
	})

	go func() {
		srv := &http.Server{Handler: mux}
		if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
			if log.ErrorLog != nil {
				log.ErrorLog.Printf("MCP server error: %v", err)
			}
		}
	}()

	if log.InfoLog != nil {
		log.InfoLog.Printf("MCP index server started on port %d", m.port)
	}
	return m.port, nil
}

// Stop stops the MCP server.
func (m *MCPIndexServer) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.listener != nil {
		m.listener.Close()
		m.listener = nil
		m.port = 0
	}
}

// Port returns the port the server is listening on.
func (m *MCPIndexServer) Port() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.port
}

// contextKey is a type for context keys to avoid collisions.
type contextKey string

const sessionIDKey contextKey = "sessionID"

// getSessionID extracts the session ID from context.
func getSessionID(ctx context.Context) string {
	if v := ctx.Value(sessionIDKey); v != nil {
		return v.(string)
	}
	return ""
}

// getIndexer returns the indexer for the given session ID.
// In standalone mode, returns the standalone indexer regardless of session ID.
func (m *MCPIndexServer) getIndexer(sessionID string) (*SessionIndexer, error) {
	if m.standaloneIndexer != nil {
		return m.standaloneIndexer, nil
	}
	if m.api == nil {
		return nil, fmt.Errorf("no API or standalone indexer configured")
	}
	m.api.mu.RLock()
	idx, ok := m.api.indexers[sessionID]
	m.api.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no indexer found for session %q", sessionID)
	}
	return idx, nil
}

// registerTools registers all MCP tools with the server.
func (m *MCPIndexServer) registerTools(s *server.MCPServer) {
	// lookup_symbol - find definitions for a symbol name
	s.AddTool(
		mcp.NewTool("lookup_symbol",
			mcp.WithDescription("Look up symbol definitions by name. Returns file path, line number, kind (function, type, etc.), and scope for each definition found."),
			mcp.WithString("name",
				mcp.Description("The symbol name to look up (exact match)"),
				mcp.Required(),
			),
		),
		m.handleLookupSymbol,
	)

	// list_files - get all tracked files
	s.AddTool(
		mcp.NewTool("list_files",
			mcp.WithDescription("List all git-tracked files in the session's worktree. Returns paths relative to the worktree root."),
			mcp.WithString("pattern",
				mcp.Description("Optional glob pattern to filter files (e.g., '*.go', 'src/**/*.ts')"),
			),
		),
		m.handleListFiles,
	)

	// get_file_outline - get symbols defined in a specific file
	s.AddTool(
		mcp.NewTool("get_file_outline",
			mcp.WithDescription("Get an outline of symbols defined in a specific file. Returns all functions, types, constants, etc. with their line numbers."),
			mcp.WithString("path",
				mcp.Description("File path relative to the worktree root"),
				mcp.Required(),
			),
		),
		m.handleGetFileOutline,
	)

	// read_lines - read specific lines from a file
	s.AddTool(
		mcp.NewTool("read_lines",
			mcp.WithDescription("Read specific lines from a file. More efficient than reading the whole file when you only need a portion."),
			mcp.WithString("path",
				mcp.Description("File path relative to the worktree root"),
				mcp.Required(),
			),
			mcp.WithNumber("start",
				mcp.Description("Starting line number (1-based)"),
				mcp.Required(),
			),
			mcp.WithNumber("end",
				mcp.Description("Ending line number (inclusive)"),
				mcp.Required(),
			),
		),
		m.handleReadLines,
	)

	// search_symbols - search for symbols by prefix or substring
	s.AddTool(
		mcp.NewTool("search_symbols",
			mcp.WithDescription("Search for symbols matching a pattern. Supports prefix matching and case-insensitive search."),
			mcp.WithString("query",
				mcp.Description("Search query (matches symbol names containing this string)"),
				mcp.Required(),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of results to return"),
				mcp.DefaultNumber(50),
			),
		),
		m.handleSearchSymbols,
	)
}

// handleLookupSymbol handles the lookup_symbol tool call.
func (m *MCPIndexServer) handleLookupSymbol(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID := getSessionID(ctx)
	if sessionID == "" {
		return mcp.NewToolResultError("session ID not found in request"), nil
	}

	name, err := req.RequireString("name")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid argument: %v", err)), nil
	}

	idx, err := m.getIndexer(sessionID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	defs := idx.Lookup(name)
	if len(defs) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No definitions found for symbol %q", name)), nil
	}

	// Format as JSON for structured output
	result, _ := json.MarshalIndent(defs, "", "  ")
	return mcp.NewToolResultText(string(result)), nil
}

// handleListFiles handles the list_files tool call.
func (m *MCPIndexServer) handleListFiles(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID := getSessionID(ctx)
	if sessionID == "" {
		return mcp.NewToolResultError("session ID not found in request"), nil
	}

	pattern, _ := req.RequireString("pattern")

	idx, err := m.getIndexer(sessionID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	files := idx.Files()

	// Filter by pattern if provided
	if pattern != "" {
		var filtered []string
		for _, f := range files {
			matched, _ := filepath.Match(pattern, filepath.Base(f))
			if matched {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}

	if len(files) == 0 {
		return mcp.NewToolResultText("No files found"), nil
	}

	return mcp.NewToolResultText(strings.Join(files, "\n")), nil
}

// handleGetFileOutline handles the get_file_outline tool call.
func (m *MCPIndexServer) handleGetFileOutline(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID := getSessionID(ctx)
	if sessionID == "" {
		return mcp.NewToolResultError("session ID not found in request"), nil
	}

	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid argument: %v", err)), nil
	}

	idx, err := m.getIndexer(sessionID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Collect all symbols in the requested file
	allSymbols := idx.AllSymbols()
	var fileSymbols []Definition
	for _, defs := range allSymbols {
		for _, def := range defs {
			if def.File == path {
				fileSymbols = append(fileSymbols, def)
			}
		}
	}

	if len(fileSymbols) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No symbols found in file %q", path)), nil
	}

	// Sort by line number
	sort.Slice(fileSymbols, func(i, j int) bool {
		return fileSymbols[i].Line < fileSymbols[j].Line
	})

	result, _ := json.MarshalIndent(fileSymbols, "", "  ")
	return mcp.NewToolResultText(string(result)), nil
}

// handleReadLines handles the read_lines tool call.
func (m *MCPIndexServer) handleReadLines(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID := getSessionID(ctx)
	if sessionID == "" {
		return mcp.NewToolResultError("session ID not found in request"), nil
	}

	path, err := req.RequireString("path")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid argument: %v", err)), nil
	}

	startF, err := req.RequireFloat("start")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid argument: %v", err)), nil
	}
	start := int(startF)

	endF, err := req.RequireFloat("end")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid argument: %v", err)), nil
	}
	end := int(endF)

	if start < 1 {
		start = 1
	}
	if end < start {
		return mcp.NewToolResultError("end must be >= start"), nil
	}

	// Get the indexer to find the worktree path
	idx, err := m.getIndexer(sessionID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Resolve the full file path
	worktree := idx.Worktree()
	fullPath := filepath.Join(worktree, path)

	// Security check: ensure the path is within the worktree
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid path: %v", err)), nil
	}
	absWorktree, _ := filepath.Abs(worktree)
	if !strings.HasPrefix(absPath, absWorktree) {
		return mcp.NewToolResultError("path escapes worktree"), nil
	}

	// Read the file
	file, err := os.Open(fullPath)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to open file: %v", err)), nil
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum >= start && lineNum <= end {
			lines = append(lines, fmt.Sprintf("%4d: %s", lineNum, scanner.Text()))
		}
		if lineNum > end {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("error reading file: %v", err)), nil
	}

	if len(lines) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No lines found in range %d-%d", start, end)), nil
	}

	return mcp.NewToolResultText(strings.Join(lines, "\n")), nil
}

// handleSearchSymbols handles the search_symbols tool call.
func (m *MCPIndexServer) handleSearchSymbols(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID := getSessionID(ctx)
	if sessionID == "" {
		return mcp.NewToolResultError("session ID not found in request"), nil
	}

	query, err := req.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid argument: %v", err)), nil
	}

	limitF, _ := req.RequireFloat("limit")
	limit := int(limitF)
	if limit <= 0 {
		limit = 50
	}

	idx, err := m.getIndexer(sessionID)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Search for matching symbols (case-insensitive substring match)
	allSymbols := idx.AllSymbols()
	queryLower := strings.ToLower(query)

	var matches []Definition
	for name, defs := range allSymbols {
		if strings.Contains(strings.ToLower(name), queryLower) {
			matches = append(matches, defs...)
			if len(matches) >= limit {
				break
			}
		}
	}

	if len(matches) == 0 {
		return mcp.NewToolResultText(fmt.Sprintf("No symbols found matching %q", query)), nil
	}

	// Truncate to limit
	if len(matches) > limit {
		matches = matches[:limit]
	}

	result, _ := json.MarshalIndent(matches, "", "  ")
	return mcp.NewToolResultText(string(result)), nil
}

// GenerateMCPConfig generates an MCP configuration JSON for a session.
// This can be passed to Claude Code via --mcp-config.
func (m *MCPIndexServer) GenerateMCPConfig(sessionID string) string {
	m.mu.RLock()
	port := m.port
	m.mu.RUnlock()

	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"cs-index": map[string]interface{}{
				"type": "http",
				"url":  fmt.Sprintf("http://127.0.0.1:%d/mcp/%s", port, sessionID),
			},
		},
	}

	data, _ := json.Marshal(config)
	return string(data)
}
