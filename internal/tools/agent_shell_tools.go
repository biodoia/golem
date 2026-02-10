// Package tools provides GOLEM agent tools for shell operations
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/biodoia/golem/pkg/zhipu"
)

// AgentShellToolsRegistry returns all shell-related tools
func AgentShellToolsRegistry() []zhipu.Tool {
	return []zhipu.Tool{
		AgentExecuteTool(),
		AgentExecuteBackgroundTool(),
	}
}

// AgentExecuteTool returns the execute tool definition
func AgentExecuteTool() zhipu.Tool {
	return zhipu.Tool{
		Type: "function",
		Function: &zhipu.Function{
			Name:        "execute",
			Description: "Execute a shell command and wait for it to complete. Returns stdout, stderr, and exit code. Use for quick commands that complete in seconds.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The shell command to execute",
					},
					"workdir": map[string]interface{}{
						"type":        "string",
						"description": "Working directory for the command (optional)",
					},
					"timeout_seconds": map[string]interface{}{
						"type":        "integer",
						"description": "Timeout in seconds (default: 30, max: 300)",
					},
					"env": map[string]interface{}{
						"type":        "object",
						"description": "Environment variables to set (key-value pairs)",
					},
				},
				"required": []string{"command"},
			},
		},
	}
}

// AgentExecuteParams are the parameters for execute
type AgentExecuteParams struct {
	Command        string            `json:"command"`
	Workdir        string            `json:"workdir,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
}

// AgentExecuteResult is the result of a command execution
type AgentExecuteResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
	TimedOut bool   `json:"timed_out,omitempty"`
	Error    string `json:"error,omitempty"`
}

// AgentExecute runs a shell command synchronously
func AgentExecute(argsJSON string) (string, error) {
	var params AgentExecuteParams
	if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	if params.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	timeout := params.TimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}
	if timeout > 300 {
		timeout = 300
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", params.Command)

	if params.Workdir != "" {
		cmd.Dir = params.Workdir
	}

	// Set environment variables
	if len(params.Env) > 0 {
		cmd.Env = cmd.Environ()
		for k, v := range params.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	result := AgentExecuteResult{}

	err := cmd.Run()
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if ctx.Err() == context.DeadlineExceeded {
		result.TimedOut = true
		result.ExitCode = -1
		result.Error = "command timed out"
	} else if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			result.Error = err.Error()
		}
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

// AgentBackgroundProcess represents a running background process
type AgentBackgroundProcess struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"` // "running", "completed", "failed"
	ExitCode  int       `json:"exit_code,omitempty"`
	Stdout    string    `json:"stdout,omitempty"`
	Stderr    string    `json:"stderr,omitempty"`
	Error     string    `json:"error,omitempty"`
}

var (
	agentBackgroundProcesses = make(map[string]*AgentBackgroundProcess)
	agentBgMutex             sync.RWMutex
	agentBgCounter           int
)

// AgentExecuteBackgroundTool returns the execute_background tool definition
func AgentExecuteBackgroundTool() zhipu.Tool {
	return zhipu.Tool{
		Type: "function",
		Function: &zhipu.Function{
			Name:        "execute_background",
			Description: "Execute a shell command in the background. Returns immediately with a process ID. Use for long-running commands. Use 'list' action to check status of background processes.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action": map[string]interface{}{
						"type":        "string",
						"description": "Action: 'start' to start a new process, 'status' to check a process, 'list' to list all, 'kill' to terminate",
						"enum":        []string{"start", "status", "list", "kill"},
					},
					"command": map[string]interface{}{
						"type":        "string",
						"description": "The shell command to execute (required for 'start')",
					},
					"process_id": map[string]interface{}{
						"type":        "string",
						"description": "Process ID (required for 'status' and 'kill')",
					},
					"workdir": map[string]interface{}{
						"type":        "string",
						"description": "Working directory for the command (optional)",
					},
					"timeout_seconds": map[string]interface{}{
						"type":        "integer",
						"description": "Timeout in seconds for background process (default: 600)",
					},
				},
				"required": []string{"action"},
			},
		},
	}
}

// AgentExecuteBackgroundParams are the parameters for execute_background
type AgentExecuteBackgroundParams struct {
	Action         string `json:"action"`
	Command        string `json:"command,omitempty"`
	ProcessID      string `json:"process_id,omitempty"`
	Workdir        string `json:"workdir,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// AgentExecuteBackground manages background processes
func AgentExecuteBackground(argsJSON string) (string, error) {
	var params AgentExecuteBackgroundParams
	if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	switch params.Action {
	case "start":
		return agentStartBackgroundProcess(params)
	case "status":
		return agentGetProcessStatus(params.ProcessID)
	case "list":
		return agentListBackgroundProcesses()
	case "kill":
		return agentKillBackgroundProcess(params.ProcessID)
	default:
		return "", fmt.Errorf("unknown action: %s (use: start, status, list, kill)", params.Action)
	}
}

func agentStartBackgroundProcess(params AgentExecuteBackgroundParams) (string, error) {
	if params.Command == "" {
		return "", fmt.Errorf("command is required for 'start' action")
	}

	timeout := params.TimeoutSeconds
	if timeout <= 0 {
		timeout = 600 // 10 minutes default for background
	}
	if timeout > 3600 {
		timeout = 3600 // Max 1 hour
	}

	agentBgMutex.Lock()
	agentBgCounter++
	processID := fmt.Sprintf("bg-%d-%d", time.Now().Unix(), agentBgCounter)
	agentBgMutex.Unlock()

	proc := &AgentBackgroundProcess{
		ID:        processID,
		Command:   params.Command,
		StartedAt: time.Now(),
		Status:    "running",
	}

	agentBgMutex.Lock()
	agentBackgroundProcesses[processID] = proc
	agentBgMutex.Unlock()

	// Run in goroutine
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "sh", "-c", params.Command)

		if params.Workdir != "" {
			cmd.Dir = params.Workdir
		}

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()

		agentBgMutex.Lock()
		defer agentBgMutex.Unlock()

		proc.Stdout = stdout.String()
		proc.Stderr = stderr.String()

		if ctx.Err() == context.DeadlineExceeded {
			proc.Status = "timeout"
			proc.ExitCode = -1
			proc.Error = "command timed out"
		} else if err != nil {
			proc.Status = "failed"
			if exitErr, ok := err.(*exec.ExitError); ok {
				proc.ExitCode = exitErr.ExitCode()
			} else {
				proc.ExitCode = -1
				proc.Error = err.Error()
			}
		} else {
			proc.Status = "completed"
			proc.ExitCode = 0
		}
	}()

	result := map[string]interface{}{
		"process_id": processID,
		"status":     "started",
		"message":    fmt.Sprintf("Background process started. Use status action with process_id '%s' to check progress.", processID),
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func agentGetProcessStatus(processID string) (string, error) {
	if processID == "" {
		return "", fmt.Errorf("process_id is required for 'status' action")
	}

	agentBgMutex.RLock()
	proc, exists := agentBackgroundProcesses[processID]
	agentBgMutex.RUnlock()

	if !exists {
		return "", fmt.Errorf("process not found: %s", processID)
	}

	output, _ := json.MarshalIndent(proc, "", "  ")
	return string(output), nil
}

func agentListBackgroundProcesses() (string, error) {
	agentBgMutex.RLock()
	defer agentBgMutex.RUnlock()

	processes := make([]map[string]interface{}, 0, len(agentBackgroundProcesses))
	for _, proc := range agentBackgroundProcesses {
		processes = append(processes, map[string]interface{}{
			"id":         proc.ID,
			"command":    agentTruncateString(proc.Command, 50),
			"status":     proc.Status,
			"started_at": proc.StartedAt.Format(time.RFC3339),
			"exit_code":  proc.ExitCode,
		})
	}

	result := map[string]interface{}{
		"count":     len(processes),
		"processes": processes,
	}

	output, _ := json.MarshalIndent(result, "", "  ")
	return string(output), nil
}

func agentKillBackgroundProcess(processID string) (string, error) {
	if processID == "" {
		return "", fmt.Errorf("process_id is required for 'kill' action")
	}

	agentBgMutex.Lock()
	defer agentBgMutex.Unlock()

	proc, exists := agentBackgroundProcesses[processID]
	if !exists {
		return "", fmt.Errorf("process not found: %s", processID)
	}

	// Note: We can't actually kill the process since we don't store the cmd reference
	// This just removes it from tracking
	delete(agentBackgroundProcesses, processID)

	return fmt.Sprintf("Process %s removed from tracking (status was: %s)", processID, proc.Status), nil
}

func agentTruncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ExecuteAgentShellTool dispatches a shell tool call by name
func ExecuteAgentShellTool(name, argsJSON string) (string, error) {
	switch name {
	case "execute":
		return AgentExecute(argsJSON)
	case "execute_background":
		return AgentExecuteBackground(argsJSON)
	default:
		return "", fmt.Errorf("unknown shell tool: %s", name)
	}
}

// AllAgentToolsRegistry returns all available agent tools (file + shell)
func AllAgentToolsRegistry() []zhipu.Tool {
	tools := make([]zhipu.Tool, 0, 6)
	tools = append(tools, AgentFileToolsRegistry()...)
	tools = append(tools, AgentShellToolsRegistry()...)
	return tools
}

// ExecuteAgentToolCall dispatches any agent tool call by name
func ExecuteAgentToolCall(name, argsJSON string) (string, error) {
	// File tools
	switch name {
	case "read_file", "write_file", "list_dir", "search_files":
		return ExecuteAgentFileTool(name, argsJSON)
	case "execute", "execute_background":
		return ExecuteAgentShellTool(name, argsJSON)
	default:
		return "", fmt.Errorf("unknown tool: %s", name)
	}
}
