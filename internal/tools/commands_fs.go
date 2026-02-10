package tools

import (
	"os"
	"path/filepath"
	"strings"
)

// DiscoverCommands scans paths for script files and returns their names.
func DiscoverCommands(paths []string) []string {
	var result []string
	for _, path := range paths {
		entries, err := os.ReadDir(path)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			result = append(result, name)
		}
	}
	return result
}
