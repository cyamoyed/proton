package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Plugins []string `yaml:"plugins"`
}

func LoadConfig() *Config {
	cfg := &Config{
		Plugins: []string{},
	}

	// Load global config
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalConfigPath := filepath.Join(homeDir, ".proton", "config.yaml")
		if data, err := os.ReadFile(globalConfigPath); err == nil {
			var globalCfg Config
			if err := yaml.Unmarshal(data, &globalCfg); err == nil {
				cfg.Plugins = append(cfg.Plugins, globalCfg.Plugins...)
			}
		}
	}

	// Load project-level config
	if data, err := os.ReadFile(".proton.yaml"); err == nil {
		var projectCfg Config
		if err := yaml.Unmarshal(data, &projectCfg); err == nil {
			cfg.Plugins = append(cfg.Plugins, projectCfg.Plugins...)
		}
	}

	return cfg
}
