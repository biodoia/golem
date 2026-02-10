// Package tools provides GOLEM agent tools for file and shell operations
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/biodoia/golem/pkg/zhipu"
)

// AgentFileToolsRegistry returns all file-related tools for the agent
func AgentFileToolsRegistry() []zhipu.Tool {
	return []zhipu.Tool{
		AgentReadFileTool(),
		AgentWriteFileTool(),
		AgentListDirTool(),
		AgentSearchFilesTool(),
	}
}

// AgentReadFileTool returns the read_file tool definition
func AgentReadFileTool() zhipu.Tool {
	return zhipu.Tool{
		Type: "function",
		Function: &zhipu.Function{
			Name:        "read_file",
			Description: "Read the contents of a file at the specified path. Returns the file content as a string. Use for text files only.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The absolute or relative path to the file to read",
					},
					"max_lines": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of lines to read (0 = unlimited)",
					},
					"offset": map[string]interface{}{
						"type":        "integer",
						"description": "Line number to start reading from (0-indexed)",
					},
				},
				"required": []string{"path"},
			},
		},
	}
}

// AgentReadFileParams are the parameters for read_file
type AgentReadFileParams struct {
	Path     string `json:"path"`
	MaxLines int    `json:"max_lines,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

// AgentReadFile executes the read_file tool
func AgentReadFile(argsJSON string) (string, error) {
	var params AgentReadFileParams
	if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Use existing ReadFile function
	result, err := ReadFile(context.Background(), params.Path)
	if err != nil {
		return "", err
	}

	if result.ReadError != "" {
		return "", fmt.Errorf(result.ReadError)
	}

	if !result.Exists {
		return "", fmt.Errorf("file not found: %s", params.Path)
	}

	if result.IsDir {
		return "", fmt.Errorf("path is a directory: %s", params.Path)
	}

	content := result.Content

	// Apply offset and max_lines if specified
	if params.Offset > 0 || params.MaxLines > 0 {
		lines := strings.Split(content, "\n")

		start := params.Offset
		if start >= len(lines) {
			return "", fmt.Errorf("offset %d exceeds file length %d lines", start, len(lines))
		}

		end := len(lines)
		if params.MaxLines > 0 && start+params.MaxLines < end {
			end = start + params.MaxLines
		}

		content = strings.Join(lines[start:end], "\n")
	}

	return content, nil
}

// AgentWriteFileTool returns the write_file tool definition
func AgentWriteFileTool() zhipu.Tool {
	return zhipu.Tool{
		Type: "function",
		Function: &zhipu.Function{
			Name:        "write_file",
			Description: "Write content to a file. Creates the file if it doesn't exist, overwrites if it does. Automatically creates parent directories.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The absolute or relative path to write to",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The content to write to the file",
					},
					"append": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, append to file instead of overwriting",
					},
				},
				"required": []string{"path", "content"},
			},
		},
	}
}

// AgentWriteFileParams are the parameters for write_file
type AgentWriteFileParams struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Append  bool   `json:"append,omitempty"`
}

// AgentWriteFile executes the write_file tool
func AgentWriteFile(argsJSON string) (string, error) {
	var params AgentWriteFileParams
	if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	if params.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	if params.Append {
		// Handle append mode separately since WriteFile doesn't support it
		dir := filepath.Dir(params.Path)
		if dir != "." && dir != "/" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "", fmt.Errorf("create directories: %w", err)
			}
		}

		f, err := os.OpenFile(params.Path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return "", fmt.Errorf("open file: %w", err)
		}
		defer f.Close()

		n, err := f.WriteString(params.Content)
		if err != nil {
			return "", fmt.Errorf("write file: %w", err)
		}
		return fmt.Sprintf("Successfully appended %d bytes to %s", n, params.Path), nil
	}

	// Use existing WriteFile function
	result, err := WriteFile(context.Background(), params.Path, params.Content)
	if err != nil {
		return "", err
	}

	if result.WriteError != "" {
		return "", fmt.Errorf(result.WriteError)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", result.Written, params.Path), nil
}

// AgentListDirTool returns the list_dir tool definition
func AgentListDirTool() zhipu.Tool {
	return zhipu.Tool{
		Type: "function",
		Function: &zhipu.Function{
			Name:        "list_dir",
			Description: "List the contents of a directory. Returns files and subdirectories with their types and sizes.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The directory path to list",
					},
					"recursive": map[string]interface{}{
						"type":        "boolean",
						"description": "If true, list recursively (be careful with large directories)",
					},
					"max_depth": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum depth for recursive listing (default: 3)",
					},
				},
				"required": []string{"path"},
			},
		},
	}
}

// AgentListDirParams are the parameters for list_dir
type AgentListDirParams struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive,omitempty"`
	MaxDepth  int    `json:"max_depth,omitempty"`
}

// AgentDirEntry represents a directory entry
type AgentDirEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Type  string `json:"type"` // "file" or "dir"
	Size  int64  `json:"size,omitempty"`
	Mode  string `json:"mode"`
	IsDir bool   `json:"is_dir"`
}

// AgentListDir executes the list_dir tool
func AgentListDir(argsJSON string) (string, error) {
	var params AgentListDirParams
	if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	if params.Path == "" {
		params.Path = "."
	}

	if params.MaxDepth == 0 {
		params.MaxDepth = 3
	}

	var entries []AgentDirEntry

	if params.Recursive {
		err := agentWalkDir(params.Path, params.Path, 0, params.MaxDepth, &entries)
		if err != nil {
			return "", err
		}
	} else {
		dirEntries, err := os.ReadDir(params.Path)
		if err != nil {
			return "", fmt.Errorf("read dir: %w", err)
		}

		for _, de := range dirEntries {
			info, err := de.Info()
			if err != nil {
				continue
			}

			entryType := "file"
			if de.IsDir() {
				entryType = "dir"
			}

			entries = append(entries, AgentDirEntry{
				Name:  de.Name(),
				Path:  filepath.Join(params.Path, de.Name()),
				Type:  entryType,
				Size:  info.Size(),
				Mode:  info.Mode().String(),
				IsDir: de.IsDir(),
			})
		}
	}

	result, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal result: %w", err)
	}

	return string(result), nil
}

func agentWalkDir(basePath, currentPath string, depth, maxDepth int, entries *[]AgentDirEntry) error {
	if depth > maxDepth {
		return nil
	}

	dirEntries, err := os.ReadDir(currentPath)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", currentPath, err)
	}

	for _, de := range dirEntries {
		info, err := de.Info()
		if err != nil {
			continue
		}

		fullPath := filepath.Join(currentPath, de.Name())
		entryType := "file"
		if de.IsDir() {
			entryType = "dir"
		}

		*entries = append(*entries, AgentDirEntry{
			Name:  de.Name(),
			Path:  fullPath,
			Type:  entryType,
			Size:  info.Size(),
			Mode:  info.Mode().String(),
			IsDir: de.IsDir(),
		})

		if de.IsDir() && depth < maxDepth {
			agentWalkDir(basePath, fullPath, depth+1, maxDepth, entries)
		}
	}

	return nil
}

// AgentSearchFilesTool returns the search_files tool definition
func AgentSearchFilesTool() zhipu.Tool {
	return zhipu.Tool{
		Type: "function",
		Function: &zhipu.Function{
			Name:        "search_files",
			Description: "Search for files matching a pattern or containing specific text. Returns matching file paths.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The directory to search in",
					},
					"pattern": map[string]interface{}{
						"type":        "string",
						"description": "Glob pattern to match filenames (e.g., '*.go', '*.md')",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "Text to search for inside files (case-insensitive)",
					},
					"max_results": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results to return (default: 50)",
					},
				},
				"required": []string{"path"},
			},
		},
	}
}

// AgentSearchFilesParams are the parameters for search_files
type AgentSearchFilesParams struct {
	Path       string `json:"path"`
	Pattern    string `json:"pattern,omitempty"`
	Content    string `json:"content,omitempty"`
	MaxResults int    `json:"max_results,omitempty"`
}

// AgentSearchResult represents a search result
type AgentSearchResult struct {
	Path    string   `json:"path"`
	Matches []string `json:"matches,omitempty"` // Line matches if content search
}

// AgentSearchFiles executes the search_files tool
func AgentSearchFiles(argsJSON string) (string, error) {
	var params AgentSearchFilesParams
	if err := json.Unmarshal([]byte(argsJSON), &params); err != nil {
		return "", fmt.Errorf("parse args: %w", err)
	}

	if params.Path == "" {
		params.Path = "."
	}

	if params.MaxResults == 0 {
		params.MaxResults = 50
	}

	var results []AgentSearchResult
	contentLower := strings.ToLower(params.Content)

	err := filepath.WalkDir(params.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		if d.IsDir() {
			// Skip common non-useful directories
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" || name == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		if len(results) >= params.MaxResults {
			return filepath.SkipAll
		}

		// Check pattern match
		if params.Pattern != "" {
			matched, err := filepath.Match(params.Pattern, d.Name())
			if err != nil || !matched {
				return nil
			}
		}

		result := AgentSearchResult{Path: path}

		// Check content match
		if params.Content != "" {
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}

			lines := strings.Split(string(content), "\n")
			for i, line := range lines {
				if strings.Contains(strings.ToLower(line), contentLower) {
					match := fmt.Sprintf("L%d: %s", i+1, strings.TrimSpace(line))
					if len(match) > 200 {
						match = match[:200] + "..."
					}
					result.Matches = append(result.Matches, match)
					if len(result.Matches) >= 5 {
						break // Max 5 matches per file
					}
				}
			}

			if len(result.Matches) == 0 {
				return nil // No content match
			}
		}

		results = append(results, result)
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("walk dir: %w", err)
	}

	output, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal results: %w", err)
	}

	return string(output), nil
}

// ExecuteAgentFileTool dispatches a file tool call by name (for agent use)
func ExecuteAgentFileTool(name, argsJSON string) (string, error) {
	switch name {
	case "read_file":
		return AgentReadFile(argsJSON)
	case "write_file":
		return AgentWriteFile(argsJSON)
	case "list_dir":
		return AgentListDir(argsJSON)
	case "search_files":
		return AgentSearchFiles(argsJSON)
	default:
		return "", fmt.Errorf("unknown file tool: %s", name)
	}
}
