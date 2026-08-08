package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Plugins []string `yaml:"plugins"`
}

// LoadConfig loads global configuration and configuration belonging to the
// directory Proton was asked to scan. Relative plugin paths are resolved from
// the configuration file that declares them.
func LoadConfig(projectDir string) (*Config, error) {
	cfg := &Config{Plugins: []string{}}
	var loadErrors []error

	// Load global config
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalConfigPath := filepath.Join(homeDir, ".proton", "config.yaml")
		if err := appendConfig(cfg, globalConfigPath); err != nil {
			loadErrors = append(loadErrors, err)
		}
	}

	// Load project-level config
	if projectDir == "" {
		projectDir, _ = os.Getwd()
	}
	if err := appendConfig(cfg, filepath.Join(projectDir, ".proton.yaml")); err != nil {
		loadErrors = append(loadErrors, err)
	}

	return cfg, errors.Join(loadErrors...)
}

func appendConfig(target *Config, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading config %s: %w", configPath, err)
	}
	var loaded Config
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("parsing config %s: %w", configPath, err)
	}
	baseDir := filepath.Dir(configPath)
	for _, pluginPath := range loaded.Plugins {
		if !filepath.IsAbs(pluginPath) {
			pluginPath = filepath.Join(baseDir, pluginPath)
		}
		target.Plugins = append(target.Plugins, filepath.Clean(pluginPath))
	}
	return nil
}
