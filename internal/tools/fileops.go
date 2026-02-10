package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// FileReadResult contains the result of a read operation
type FileReadResult struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Exists   bool   `json:"exists"`
	IsDir    bool   `json:"is_dir"`
	ReadError string `json:"read_error,omitempty"`
}

// FileWriteResult contains the result of a write operation
type FileWriteResult struct {
	Path       string `json:"path"`
	Written    int    `json:"bytes_written"`
	Success    bool   `json:"success"`
	WriteError string `json:"write_error,omitempty"`
}

// FileEditResult contains the result of an edit operation
type FileEditResult struct {
	Path       string `json:"path"`
	Success    bool   `json:"success"`
	Replacements int  `json:"replacements"`
	EditError  string `json:"edit_error,omitempty"`
}

// ReadFile reads a file and returns its contents
func ReadFile(ctx context.Context, path string) (*FileReadResult, error) {
	result := &FileReadResult{Path: path}

	// Resolve path
	resolved, err := filepath.Abs(path)
	if err != nil {
		result.ReadError = err.Error()
		return result, nil
	}
	result.Path = resolved

	// Check if file exists
	info, err := os.Stat(resolved)
	if os.IsNotExist(err) {
		result.Exists = false
		return result, nil
	}
	if err != nil {
		result.ReadError = err.Error()
		return result, nil
	}

	result.Exists = true
	result.IsDir = info.IsDir()

	if result.IsDir {
		return result, nil
	}

	// Read file contents
	content, err := os.ReadFile(resolved)
	if err != nil {
		result.ReadError = err.Error()
		return result, nil
	}

	result.Content = string(content)
	return result, nil
}

// WriteFile writes content to a file (creates or overwrites)
func WriteFile(ctx context.Context, path string, content string) (*FileWriteResult, error) {
	result := &FileWriteResult{Path: path}

	// Resolve path
	resolved, err := filepath.Abs(path)
	if err != nil {
		result.WriteError = err.Error()
		return result, nil
	}
	result.Path = resolved

	// Ensure directory exists
	dir := filepath.Dir(resolved)
	if err := os.MkdirAll(dir, 0755); err != nil {
		result.WriteError = err.Error()
		return result, nil
	}

	// Write file
	err = os.WriteFile(resolved, []byte(content), 0644)
	if err != nil {
		result.WriteError = err.Error()
		return result, nil
	}

	result.Written = len(content)
	result.Success = true
	return result, nil
}

// EditFile makes precise text replacements in a file
// oldText must match exactly (including whitespace)
func EditFile(ctx context.Context, path string, oldText string, newText string) (*FileEditResult, error) {
	result := &FileEditResult{Path: path}

	// Read the file first
	readResult, err := ReadFile(ctx, path)
	if err != nil {
		result.EditError = err.Error()
		return result, nil
	}

	if !readResult.Exists {
		result.EditError = "file does not exist"
		return result, nil
	}

	if readResult.IsDir {
		result.EditError = "cannot edit a directory"
		return result, nil
	}

	if readResult.ReadError != "" {
		result.EditError = readResult.ReadError
		return result, nil
	}

	// Check if oldText exists
	if !strings.Contains(readResult.Content, oldText) {
		result.EditError = "oldText not found in file"
		return result, nil
	}

	// Count replacements
	count := strings.Count(readResult.Content, oldText)

	// Perform replacement
	newContent := strings.ReplaceAll(readResult.Content, oldText, newText)

	// Write back
	writeResult, err := WriteFile(ctx, path, newContent)
	if err != nil {
		result.EditError = err.Error()
		return result, nil
	}

	if !writeResult.Success {
		result.EditError = writeResult.WriteError
		return result, nil
	}

	result.Success = true
	result.Replacements = count
	return result, nil
}

// GlobResult contains glob pattern matching results
type GlobResult struct {
	Pattern string   `json:"pattern"`
	Paths   []string `json:"paths"`
	Count   int      `json:"count"`
}

// Glob finds files matching a pattern
func Glob(ctx context.Context, pattern string) (*GlobResult, error) {
	result := &GlobResult{Pattern: pattern}

	// Expand ~ to home directory
	pattern = expandHome(pattern)

	matches, err := filepath.Glob(pattern)
	if err != nil {
		return result, nil
	}

	result.Paths = matches
	result.Count = len(matches)
	return result, nil
}

// expandHome expands ~ to the user's home directory
func expandHome(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// ListDir lists directory contents
func ListDir(ctx context.Context, path string) ([]string, error) {
	// Resolve path
	resolved, err := filepath.Abs(expandHome(path))
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(resolved)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		result = append(result, name)
	}

	return result, nil
}

// FileExists checks if a file or directory exists
func FileExists(ctx context.Context, path string) (bool, error) {
	_, err := os.Stat(expandHome(path))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}
