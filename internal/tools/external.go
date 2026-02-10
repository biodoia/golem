package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LoadExternalCommands loads slash commands from script files in paths.
// Each file name becomes the command name, content is executed as a shell script.
func LoadExternalCommands(paths []string) map[string]*Command {
	cmds := map[string]*Command{}
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
			scriptPath := filepath.Join(path, entry.Name())
			localPath := scriptPath
			cmds[name] = &Command{
				Name:        name,
				Description: "External command",
				Usage:       "/" + name,
				Handler: func(ctx context.Context, args []string) (string, error) {
					command := exec.CommandContext(ctx, "sh", append([]string{localPath}, args...)...)
					output, err := command.CombinedOutput()
					return string(output), err
				},
			}
		}
	}
	return cmds
}
