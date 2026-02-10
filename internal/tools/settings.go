package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Settings struct {
	Model string `json:"model"`
	Provider string `json:"provider"`
	Theme string `json:"theme"`
}

func LoadSettings(path string) (Settings, error) {
	var s Settings
	data, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, err
	}
	return s, nil
}

func DefaultSettingsPath() string {
	return filepath.Join(os.Getenv("HOME"), ".golem", "settings.json")
}
