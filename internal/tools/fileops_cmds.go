package tools

import (
	"context"
	"fmt"
	"strings"
)

// Command handlers for file operations

// ReadCommand reads a file
func ReadCommand(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: /read <path>")
	}

	path := args[0]
	result, err := ReadFile(ctx, path)
	if err != nil {
		return "", err
	}

	if !result.Exists {
		return fmt.Sprintf("File not found: %s", path), nil
	}

	if result.IsDir {
		return fmt.Sprintf("Path is a directory: %s", path), nil
	}

	if result.ReadError != "" {
		return fmt.Sprintf("Error reading file: %s", result.ReadError), nil
	}

	return fmt.Sprintf("=== %s ===\n%s", result.Path, result.Content), nil
}

// WriteCommand writes to a file
func WriteCommand(ctx context.Context, args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("usage: /write <path> <content>")
	}

	path := args[0]
	content := strings.Join(args[1:], " ")

	result, err := WriteFile(ctx, path, content)
	if err != nil {
		return "", err
	}

	if !result.Success {
		return fmt.Sprintf("Error writing file: %s", result.WriteError), nil
	}

	return fmt.Sprintf("Written %d bytes to %s", result.Written, result.Path), nil
}

// EditCommand edits a file
func EditCommand(ctx context.Context, args []string) (string, error) {
	if len(args) < 3 {
		return "", fmt.Errorf("usage: /edit <path> <old_text> <new_text>")
	}

	path := args[0]
	oldText := args[1]
	newText := strings.Join(args[2:], " ")

	result, err := EditFile(ctx, path, oldText, newText)
	if err != nil {
		return "", err
	}

	if !result.Success {
		return fmt.Sprintf("Error editing file: %s", result.EditError), nil
	}

	return fmt.Sprintf("Made %d replacement(s) in %s", result.Replacements, result.Path), nil
}

// GlobCommand finds files matching a pattern
func GlobCommand(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: /glob <pattern>")
	}

	pattern := args[0]
	result, err := Glob(ctx, pattern)
	if err != nil {
		return "", err
	}

	if result.Count == 0 {
		return fmt.Sprintf("No files matching: %s", pattern), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Found %d file(s) matching '%s':\n", result.Count, pattern))
	for _, p := range result.Paths {
		b.WriteString(fmt.Sprintf("  - %s\n", p))
	}

	return b.String(), nil
}

// LsCommand lists directory contents
func LsCommand(ctx context.Context, args []string) (string, error) {
	path := "."
	if len(args) >= 1 {
		path = args[0]
	}

	entries, err := ListDir(ctx, path)
	if err != nil {
		return "", fmt.Errorf("error listing directory: %w", err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Contents of '%s':\n", path))
	for _, entry := range entries {
		b.WriteString(fmt.Sprintf("  %s\n", entry))
	}

	return b.String(), nil
}

// CatCommand is an alias for ReadCommand
func CatCommand(ctx context.Context, args []string) (string, error) {
	return ReadCommand(ctx, args)
}

// ExistsCommand checks if a file exists
func ExistsCommand(ctx context.Context, args []string) (string, error) {
	if len(args) < 1 {
		return "", fmt.Errorf("usage: /exists <path>")
	}

	path := args[0]
	exists, err := FileExists(ctx, path)
	if err != nil {
		return "", err
	}

	if exists {
		return fmt.Sprintf("Path exists: %s", path), nil
	}
	return fmt.Sprintf("Path does not exist: %s", path), nil
}
