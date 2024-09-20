package main

import (
	"fmt"
  "os"
	"path/filepath"
	"plugin"

	"github.com/charmbracelet/log"
)

func runPlugin(logger *log.Logger, pluginName string) error {
	pluginsDir, err := getPluginsDir()
	if err != nil {
		logger.Error("Failed to get plugins directory", "error", err)
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}
	logger.Debug("Plugins directory", "path", pluginsDir)

	// Update the plugin path to include the 'dist' directory
	pluginPath := filepath.Join(pluginsDir, pluginName, "dist", fmt.Sprintf("%s.so", pluginName))
	logger.Debug("Attempting to open plugin", "path", pluginPath)

	_, err = os.Stat(pluginPath)
	if os.IsNotExist(err) {
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
