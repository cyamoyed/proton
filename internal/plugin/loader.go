package plugin

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fupeijun/proton/internal/config"
	"gopkg.in/yaml.v3"
)

//go:embed plugins/*.yaml
var builtinPlugins embed.FS

func LoadPlugins(cfg *config.Config) ([]*Plugin, error) {
	var plugins []*Plugin
	var loadErrors []error

	entries, err := builtinPlugins.ReadDir("plugins")
	if err != nil {
		return nil, fmt.Errorf("reading builtin plugins: %w", err)
	}
	for _, entry := range entries {
		path := "plugins/" + entry.Name()
		data, err := builtinPlugins.ReadFile(path)
		if err != nil {
			loadErrors = append(loadErrors, fmt.Errorf("reading builtin plugin %s: %w", path, err))
			continue
		}
		p, err := decodePlugin(path, data)
		if err != nil {
			loadErrors = append(loadErrors, err)
			continue
		}
		plugins = append(plugins, p)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		loadErrors = append(loadErrors, fmt.Errorf("locating user plugin directory: %w", err))
	} else {
		userPluginDir := filepath.Join(homeDir, ".proton", "plugins")
		entries, err := os.ReadDir(userPluginDir)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			loadErrors = append(loadErrors, fmt.Errorf("reading user plugin directory %s: %w", userPluginDir, err))
		} else if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				ext := filepath.Ext(entry.Name())
				if ext != ".yaml" && ext != ".yml" {
					continue
				}
				path := filepath.Join(userPluginDir, entry.Name())
				p, err := loadPluginFile(path)
				if err != nil {
					loadErrors = append(loadErrors, err)
					continue
				}
				plugins = append(plugins, p)
			}
		}
	}

	if cfg != nil {
		for _, path := range cfg.Plugins {
			p, err := loadPluginFile(path)
			if err != nil {
				loadErrors = append(loadErrors, err)
				continue
			}
			plugins = append(plugins, p)
		}
	}

	return plugins, errors.Join(loadErrors...)
}

func loadPluginFile(path string) (*Plugin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading plugin %s: %w", path, err)
	}
	return decodePlugin(path, data)
}

func decodePlugin(path string, data []byte) (*Plugin, error) {
	var p Plugin
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing plugin %s: %w", path, err)
	}
	if p.Name == "" {
		return nil, fmt.Errorf("validating plugin %s: name is required", path)
	}
	if len(p.Detect.Files) == 0 && len(p.Detect.Dirs) == 0 {
		return nil, fmt.Errorf("validating plugin %s: at least one detection file or directory is required", path)
	}
	if len(p.Commands) == 0 {
		return nil, fmt.Errorf("validating plugin %s: at least one command is required", path)
	}
	return &p, nil
}
