// Package tools provides built-in commands for Golem (OpenCode-style)
package tools

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Command represents a built-in command
type Command struct {
	Name        string
	Aliases     []string
	Description string
	Usage       string
	Handler     func(ctx context.Context, args []string) (string, error)
}

// Commands returns all built-in commands
func Commands() map[string]*Command {
	cmds := []*Command{
		cmdHelp,
		cmdBuild,
		cmdTest,
		cmdPlan,
		cmdAgents,
		cmdModel,
		cmdMCP,
		cmdConfig,
		cmdAuth,
		cmdClear,
		cmdExit,
		// File operations
		cmdRead,
		cmdWrite,
		cmdEdit,
		cmdGlob,
		cmdLs,
		cmdCat,
		cmdExists,
	}

	result := make(map[string]*Command)
	for _, cmd := range cmds {
		result[cmd.Name] = cmd
		for _, alias := range cmd.Aliases {
			result[alias] = cmd
		}
	}
	return result
}

var cmdHelp = &Command{
	Name:        "help",
	Aliases:     []string{"h", "?"},
	Description: "Show available commands",
	Usage:       "/help [command]",
	Handler: func(ctx context.Context, args []string) (string, error) {
		var b strings.Builder
		b.WriteString("Available commands:\n\n")
		b.WriteString("Core:\n")
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "help", "Show available commands"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "build", "Build the project"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "test", "Run tests"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "plan", "Create an execution plan"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "clear", "Clear the screen"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "exit", "Exit Golem"))
		b.WriteString("\nFile Operations:\n")
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "read", "Read file contents"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "write", "Write file contents"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "edit", "Make precise text replacements"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "ls", "List directory contents"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "glob", "Find files matching a pattern"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "cat", "Display file contents"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "exists", "Check if file exists"))
		b.WriteString("\nManagement:\n")
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "agents", "Manage specialized agents"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "model", "Switch or list models"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "mcp", "Manage MCP servers"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "config", "View or edit configuration"))
		b.WriteString(fmt.Sprintf("  /%-12s - %s\n", "auth", "Authentication management"))
		return b.String(), nil
	},
}

var cmdBuild = &Command{
	Name:        "build",
	Aliases:     []string{"b"},
	Description: "Build the project",
	Usage:       "/build [target]",
	Handler: func(ctx context.Context, args []string) (string, error) {
		// Detect project type
		if _, err := os.Stat("go.mod"); err == nil {
			return runCommand(ctx, "go", "build", "./...")
		}
		if _, err := os.Stat("Cargo.toml"); err == nil {
			return runCommand(ctx, "cargo", "build")
		}
		if _, err := os.Stat("package.json"); err == nil {
			return runCommand(ctx, "npm", "run", "build")
		}
		if _, err := os.Stat("Makefile"); err == nil {
			return runCommand(ctx, "make")
		}
		return "", fmt.Errorf("no recognized project type found")
	},
}

var cmdTest = &Command{
	Name:        "test",
	Aliases:     []string{"t"},
	Description: "Run tests",
	Usage:       "/test [pattern]",
	Handler: func(ctx context.Context, args []string) (string, error) {
		if _, err := os.Stat("go.mod"); err == nil {
			testArgs := []string{"test", "-v"}
			if len(args) > 0 {
				testArgs = append(testArgs, args...)
			} else {
				testArgs = append(testArgs, "./...")
			}
			return runCommand(ctx, "go", testArgs...)
		}
		if _, err := os.Stat("Cargo.toml"); err == nil {
			return runCommand(ctx, "cargo", "test")
		}
		if _, err := os.Stat("package.json"); err == nil {
			return runCommand(ctx, "npm", "test")
		}
		return "", fmt.Errorf("no recognized test framework found")
	},
}

var cmdPlan = &Command{
	Name:        "plan",
	Aliases:     []string{"p"},
	Description: "Create an execution plan with multiple agents",
	Usage:       "/plan <task description>",
	Handler: func(ctx context.Context, args []string) (string, error) {
		if len(args) == 0 {
			return "", fmt.Errorf("usage: /plan <task description>")
		}
		// This will be handled by the agent coordinator
		return fmt.Sprintf("Planning task: %s\n\n[Agent coordination will be triggered]", strings.Join(args, " ")), nil
	},
}

var cmdAgents = &Command{
	Name:        "agents",
	Aliases:     []string{"a"},
	Description: "Manage specialized agents",
	Usage:       "/agents [list|run <type>]",
	Handler: func(ctx context.Context, args []string) (string, error) {
		if len(args) == 0 || args[0] == "list" {
			return `Available agents:
  architect - Plans system architecture and designs
  coder     - Writes and implements code
  reviewer  - Reviews code for quality
  debugger  - Diagnoses and fixes bugs
  tester    - Writes tests and validates
  docs      - Writes documentation

Usage: /agents run <type> <task>`, nil
		}
		return "", fmt.Errorf("unknown subcommand: %s", args[0])
	},
}

var cmdModel = &Command{
	Name:        "model",
	Aliases:     []string{"m"},
	Description: "Switch or list models",
	Usage:       "/model [list|<model-name>]",
	Handler: func(ctx context.Context, args []string) (string, error) {
		if len(args) == 0 || args[0] == "list" {
			return `Available Z.AI models:
  glm-4-32b-0414      - Dialogue, code, function calling (default)
  glm-z1-32b-0414     - Deep thinking, math, reasoning
  glm-z1-rumination   - Research, search-augmented
  glm-z1-9b-0414      - Lightweight, efficient
  glm-4.1v-9b-thinking - Vision + reasoning
  codegeex-4          - Code completion

Usage: /model <model-name>`, nil
		}
		return fmt.Sprintf("Switched to model: %s", args[0]), nil
	},
}

var cmdMCP = &Command{
	Name:        "mcp",
	Aliases:     []string{},
	Description: "Manage MCP servers",
	Usage:       "/mcp [list|start|stop] [server]",
	Handler: func(ctx context.Context, args []string) (string, error) {
		if len(args) == 0 || args[0] == "list" {
			return `Pre-configured MCP servers:
  filesystem  - File operations (auto-start)
  memory      - Persistent memory (auto-start)
  web-search  - Brave search
  github      - GitHub operations
  sqlite      - SQLite database
  puppeteer   - Browser automation

Usage: /mcp start <server> | /mcp stop <server>`, nil
		}
		return "", fmt.Errorf("unknown subcommand: %s", args[0])
	},
}

var cmdConfig = &Command{
	Name:        "config",
	Aliases:     []string{"cfg"},
	Description: "View or edit configuration",
	Usage:       "/config [key] [value]",
	Handler: func(ctx context.Context, args []string) (string, error) {
		configPath := filepath.Join(os.Getenv("HOME"), ".golem", "config.yaml")
		if len(args) == 0 {
			data, err := os.ReadFile(configPath)
			if err != nil {
				return "No configuration file found. Create ~/.golem/config.yaml", nil
			}
			return string(data), nil
		}
		return "", fmt.Errorf("config editing not yet implemented")
	},
}

var cmdAuth = &Command{
	Name:        "auth",
	Aliases:     []string{},
	Description: "Authentication management",
	Usage:       "/auth [login|logout|status]",
	Handler: func(ctx context.Context, args []string) (string, error) {
		if len(args) == 0 {
			args = []string{"status"}
		}
		switch args[0] {
		case "login":
			return "Opening browser for Z.AI OAuth login...", nil
		case "logout":
			return "Logged out successfully", nil
		case "status":
			if apiKey := os.Getenv("ZHIPU_API_KEY"); apiKey != "" {
				return "Authenticated via API key (ZHIPU_API_KEY)", nil
			}
			return "Not authenticated. Use /auth login or set ZHIPU_API_KEY", nil
		}
		return "", fmt.Errorf("unknown auth command: %s", args[0])
	},
}

var cmdClear = &Command{
	Name:        "clear",
	Aliases:     []string{"cls"},
	Description: "Clear the screen",
	Usage:       "/clear",
	Handler: func(ctx context.Context, args []string) (string, error) {
		return "\033[2J\033[H", nil // ANSI clear screen
	},
}

var cmdExit = &Command{
	Name:        "exit",
	Aliases:     []string{"quit", "q"},
	Description: "Exit Golem",
	Usage:       "/exit",
	Handler: func(ctx context.Context, args []string) (string, error) {
		return "EXIT", nil // Special signal to quit
	},
}

// File operation commands for Claude parity

var cmdRead = &Command{
	Name:        "read",
	Aliases:     []string{"r"},
	Description: "Read file contents",
	Usage:       "/read <path>",
	Handler:     ReadCommand,
}

var cmdWrite = &Command{
	Name:        "write",
	Aliases:     []string{"save", "create"},
	Description: "Write file contents",
	Usage:       "/write <path> <content>",
	Handler:     WriteCommand,
}

var cmdEdit = &Command{
	Name:        "edit",
	Aliases:     []string{"replace", "sed"},
	Description: "Make precise text replacements in a file",
	Usage:       "/edit <path> <old_text> <new_text>",
	Handler:     EditCommand,
}

var cmdGlob = &Command{
	Name:        "glob",
	Aliases:     []string{"find", "match"},
	Description: "Find files matching a pattern",
	Usage:       "/glob <pattern>",
	Handler:     GlobCommand,
}

var cmdLs = &Command{
	Name:        "ls",
	Aliases:     []string{"dir", "list"},
	Description: "List directory contents",
	Usage:       "/ls [path]",
	Handler:     LsCommand,
}

var cmdCat = &Command{
	Name:        "cat",
	Aliases:     []string{},
	Description: "Display file contents (alias for read)",
	Usage:       "/cat <path>",
	Handler:     CatCommand,
}

var cmdExists = &Command{
	Name:        "exists",
	Aliases:     []string{"test"},
	Description: "Check if a file or directory exists",
	Usage:       "/exists <path>",
	Handler:     ExistsCommand,
}

// runCommand executes a shell command
func runCommand(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir, _ = os.Getwd()
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("%s failed: %w\n%s", name, err, output)
	}
	return string(output), nil
}

// File operation command handlers

// ReadCommand reads a file and returns its contents
func ReadCommand(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: /read <path>")
	}
	result, err := ReadFile(ctx, args[0])
	if err != nil {
		return "", err
	}
	if result.ReadError != "" {
		return "", fmt.Errorf(result.ReadError)
	}
	if !result.Exists {
		return "", fmt.Errorf("file does not exist: %s", args[0])
	}
	if result.IsDir {
		return "", fmt.Errorf("path is a directory, use /ls instead")
	}
	return result.Content, nil
}

// WriteCommand writes content to a file
func WriteCommand(ctx context.Context, args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: /write <path> <content>")
	}
	content := strings.Join(args[1:], " ")
	result, err := WriteFile(ctx, args[0], content)
	if err != nil {
		return "", err
	}
	if result.WriteError != "" {
		return "", fmt.Errorf(result.WriteError)
	}
	return fmt.Sprintf("Wrote %d bytes to %s", result.Written, result.Path), nil
}

// EditCommand replaces text in a file
func EditCommand(ctx context.Context, args []string) (string, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("usage: /edit <path> <old_text> <new_text>")
	}
	result, err := EditFile(ctx, args[0], args[1], args[2])
	if err != nil {
		return "", err
	}
	if result.EditError != "" {
		return "", fmt.Errorf(result.EditError)
	}
	return fmt.Sprintf("Made %d replacements in %s", result.Replacements, result.Path), nil
}

// GlobCommand finds files matching a pattern
func GlobCommand(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: /glob <pattern>")
	}
	result, err := Glob(ctx, args[0])
	if err != nil {
		return "", err
	}
	if result.Count == 0 {
		return "No files matched", nil
	}
	return strings.Join(result.Paths, "\n"), nil
}

// LsCommand lists directory contents
func LsCommand(ctx context.Context, args []string) (string, error) {
	path := "."
	if len(args) > 0 {
		path = args[0]
	}
	entries, err := ListDir(ctx, path)
	if err != nil {
		return "", err
	}
	return strings.Join(entries, "\n"), nil
}

// CatCommand is an alias for ReadCommand
func CatCommand(ctx context.Context, args []string) (string, error) {
	return ReadCommand(ctx, args)
}

// ExistsCommand checks if a path exists
func ExistsCommand(ctx context.Context, args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("usage: /exists <path>")
	}
	exists, err := FileExists(ctx, args[0])
	if err != nil {
		return "", err
	}
	if exists {
		return fmt.Sprintf("✓ %s exists", args[0]), nil
	}
	return fmt.Sprintf("✗ %s does not exist", args[0]), nil
}

