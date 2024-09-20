package main

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/ssotops/gitspace-plugin/gsplug"
)

func runPlugin(logger *log.Logger, pluginName string) error {
	pluginsDir, err := getPluginsDir()
	if err != nil {
		logger.Error("Failed to get plugins directory", "error", err)
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}
	logger.Debug("Plugins directory", "path", pluginsDir)

	pluginDir := filepath.Join(pluginsDir, pluginName)
	if needsRebuild(logger, pluginDir) {
		logger.Info("Plugin needs rebuilding", "plugin", pluginName)
		if err := gsplug.BuildPlugin(pluginDir); err != nil {
			logger.Error("Failed to rebuild plugin", "plugin", pluginName, "error", err)
			return fmt.Errorf("failed to rebuild plugin: %w", err)
		}
	}

	pluginPath := filepath.Join(pluginDir, "dist", fmt.Sprintf("%s.so", pluginName))
	logger.Debug("Attempting to open plugin", "path", pluginPath)

	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		logger.Error("Plugin file does not exist", "path", pluginPath)
		return fmt.Errorf("plugin file does not exist: %s", pluginPath)
	}

	p, err := plugin.Open(pluginPath)
	if err != nil {
		logger.Error("Failed to open plugin", "error", err)
		return fmt.Errorf("failed to open plugin: %w", err)
	}

	runSymbol, err := p.Lookup("Run")
	if err != nil {
		logger.Error("Plugin does not implement Run function", "error", err)
		return fmt.Errorf("plugin does not implement Run function: %w", err)
	}

	run, ok := runSymbol.(func() error)
	if !ok {
		logger.Error("Plugin's Run function has wrong signature")
		return fmt.Errorf("plugin's Run function has wrong signature")
	}

	logger.Debug("Executing plugin")
	err = run()
	if err != nil {
		logger.Error("Plugin execution failed", "error", err)
		return fmt.Errorf("plugin execution failed: %w", err)
	}

	logger.Info("Plugin executed successfully")
	return nil
}

func needsRebuild(logger *log.Logger, pluginDir string) bool {
	canonicalDeps, err := gsplug.GetCanonicalDeps()
	if err != nil {
		logger.Warn("Failed to get canonical dependencies", "error", err)
		return true
	}

	goModPath := filepath.Join(pluginDir, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		logger.Warn("Failed to read go.mod", "error", err)
		return true
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "require ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				module := parts[1]
				version := parts[2]
				if canonicalVersion, ok := canonicalDeps.Versions[module]; ok {
					if version != canonicalVersion {
						logger.Debug("Version mismatch detected", "module", module, "plugin_version", version, "canonical_version", canonicalVersion)
						return true
					}
				}
			}
		}
	}

	return false
}
