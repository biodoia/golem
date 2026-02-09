// Package mcp provides MCP server integration for Golem
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

// Server represents an MCP server
type Server struct {
	Name        string   `json:"name"`
	Command     string   `json:"command"`
	Args        []string `json:"args,omitempty"`
	Env         []string `json:"env,omitempty"`
	Description string   `json:"description,omitempty"`
	AutoStart   bool     `json:"auto_start,omitempty"`
}

// PreConfiguredServers returns MCP servers pre-configured for Golem
func PreConfiguredServers() []Server {
	return []Server{
		{
			Name:        "filesystem",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-filesystem", "/"},
			Description: "File system operations",
			AutoStart:   true,
		},
		{
			Name:        "web-search",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-brave-search"},
			Description: "Web search via Brave",
			AutoStart:   false,
		},
		{
			Name:        "github",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-github"},
			Description: "GitHub operations",
			AutoStart:   false,
		},
		{
			Name:        "memory",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-memory"},
			Description: "Persistent memory",
			AutoStart:   true,
		},
		{
			Name:        "sqlite",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-sqlite"},
			Description: "SQLite database",
			AutoStart:   false,
		},
		{
			Name:        "puppeteer",
			Command:     "npx",
			Args:        []string{"-y", "@anthropic/mcp-server-puppeteer"},
			Description: "Browser automation",
			AutoStart:   false,
		},
	}
}

// Process represents a running MCP server process
type Process struct {
	Server  Server
	Cmd     *exec.Cmd
	Stdin   io.WriteCloser
	Stdout  io.ReadCloser
	Stderr  io.ReadCloser
	Running bool
	mu      sync.Mutex
}

// Manager manages MCP server processes
type Manager struct {
	servers   map[string]*Process
	mu        sync.RWMutex
}

// NewManager creates a new MCP manager
func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*Process),
	}
}

// Start starts an MCP server
func (m *Manager) Start(ctx context.Context, server Server) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if proc, exists := m.servers[server.Name]; exists && proc.Running {
		return fmt.Errorf("server %s already running", server.Name)
	}

	cmd := exec.CommandContext(ctx, server.Command, server.Args...)
	cmd.Env = append(os.Environ(), server.Env...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start server: %w", err)
	}

	m.servers[server.Name] = &Process{
		Server:  server,
		Cmd:     cmd,
		Stdin:   stdin,
		Stdout:  stdout,
		Stderr:  stderr,
		Running: true,
	}

	return nil
}

// Stop stops an MCP server
func (m *Manager) Stop(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proc, exists := m.servers[name]
	if !exists {
		return fmt.Errorf("server %s not found", name)
	}

	if !proc.Running {
		return nil
	}

	proc.mu.Lock()
	defer proc.mu.Unlock()

	proc.Stdin.Close()
	if err := proc.Cmd.Process.Kill(); err != nil {
		return fmt.Errorf("kill server: %w", err)
	}

	proc.Running = false
	return nil
}

// Call sends a JSON-RPC request to an MCP server
func (m *Manager) Call(name string, method string, params interface{}) (json.RawMessage, error) {
	m.mu.RLock()
	proc, exists := m.servers[name]
	m.mu.RUnlock()

	if !exists || !proc.Running {
		return nil, fmt.Errorf("server %s not running", name)
	}

	proc.mu.Lock()
	defer proc.mu.Unlock()

	// Build JSON-RPC request
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Send request
	if _, err := proc.Stdin.Write(append(reqBytes, '\n')); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	// Read response
	reader := bufio.NewReader(proc.Stdout)
	respBytes, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var resp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(respBytes, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	return resp.Result, nil
}

// ListTools lists tools from an MCP server
func (m *Manager) ListTools(name string) ([]Tool, error) {
	result, err := m.Call(name, "tools/list", nil)
	if err != nil {
		return nil, err
	}

	var tools struct {
		Tools []Tool `json:"tools"`
	}
	if err := json.Unmarshal(result, &tools); err != nil {
		return nil, fmt.Errorf("unmarshal tools: %w", err)
	}

	return tools.Tools, nil
}

// CallTool calls a tool on an MCP server
func (m *Manager) CallTool(serverName, toolName string, args map[string]interface{}) (interface{}, error) {
	params := map[string]interface{}{
		"name":      toolName,
		"arguments": args,
	}

	result, err := m.Call(serverName, "tools/call", params)
	if err != nil {
		return nil, err
	}

	var callResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		} `json:"content"`
	}

	if err := json.Unmarshal(result, &callResult); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	return callResult, nil
}

// Tool represents an MCP tool
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// List returns all registered servers
func (m *Manager) List() []Server {
	m.mu.RLock()
	defer m.mu.RUnlock()

	servers := make([]Server, 0, len(m.servers))
	for _, proc := range m.servers {
		servers = append(servers, proc.Server)
	}
	return servers
}

// Status returns status of all servers
func (m *Manager) Status() map[string]bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := make(map[string]bool)
	for name, proc := range m.servers {
		status[name] = proc.Running
	}
	return status
}

// StartPreConfigured starts all pre-configured servers marked for auto-start
func (m *Manager) StartPreConfigured(ctx context.Context) error {
	for _, server := range PreConfiguredServers() {
		if server.AutoStart {
			if err := m.Start(ctx, server); err != nil {
				// Log but don't fail
				fmt.Printf("Warning: failed to start %s: %v\n", server.Name, err)
			}
		}
	}
	return nil
}
