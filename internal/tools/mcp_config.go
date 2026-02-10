package tools

import (
	"encoding/json"
	"os"
)

type MCPConfig struct {
	Servers []map[string]interface{} `json:"mcpServers"`
}

func LoadMCPConfig(path string) (*MCPConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg MCPConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
