package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Settings struct {
	APIKey       string `json:"api_key"`
	Model        string `json:"model"`
	Provider     string `json:"provider"`
	Theme        string `json:"theme"`
	MCPConfig    string `json:"mcp_config"`
	CommandsPath string `json:"commands_path"`
}

func DefaultSettings() Settings {
	return Settings{
		Model:    "glm-4-32b-0414",
		Provider: "zai",
		Theme:    "cyberpunk",
	}
}

func Load() (Settings, error) {
	settings := DefaultSettings()
	if apiKey := os.Getenv("ZAI_API_KEY"); apiKey != "" {
		settings.APIKey = apiKey
	}
	if apiKey := os.Getenv("ZHIPU_API_KEY"); apiKey != "" {
		settings.APIKey = apiKey
	}

	path := filepath.Join(os.Getenv("HOME"), ".golem", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return settings, nil
		}
		return settings, err
	}

	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, err
	}

	return settings, nil
}

func CommandsSearchPaths(custom string) []string {
	paths := []string{
		filepath.Join(os.Getenv("HOME"), ".golem", "commands"),
		".golem/commands",
	}
	if custom != "" {
		paths = append([]string{custom}, paths...)
	}
	return paths
}

func MCPConfigPath(custom string) string {
	if custom != "" {
		return custom
	}
	return ".mcp.json"
}
