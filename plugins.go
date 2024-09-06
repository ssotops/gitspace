package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/hashicorp/hcl/v2/hclsimple"
)

type PluginManifest struct {
	Plugin struct {
		Name        string `hcl:"name,label"`
		Version     string `hcl:"version"`
		Description string `hcl:"description,optional"`
		Author      string `hcl:"author,optional"`
		EntryPoint  string `hcl:"entry_point"`
		Source      struct {
			Type       string `hcl:"type,optional"`
			Repository string `hcl:"repository,optional"`
			Branch     string `hcl:"branch,optional"`
			URL        string `hcl:"url,optional"`
			Path       string `hcl:"path,optional"`
		} `hcl:"source,block"`
	} `hcl:"plugin,block"`
}

type GitspacePlugin interface {
	Run() error
	Name() string
	Version() string
}

func getPluginsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	pluginsDir := filepath.Join(homeDir, ".ssot", "gitspace", "plugins")

	// Ensure the plugins directory exists
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create plugins directory: %w", err)
	}

	return pluginsDir, nil
}

func installPlugin(logger *log.Logger, source string) error {
	pluginsDir, err := getPluginsDir()
	if err != nil {
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}
	logger.Debug("Plugins directory", "path", pluginsDir)

	// Get absolute path of the source
	absSource, err := filepath.Abs(source)
	if err != nil {
		return fmt.Errorf("failed to get absolute path of source: %w", err)
	}
	logger.Debug("Absolute source path", "path", absSource)

	sourceInfo, err := os.Stat(absSource)
	if err != nil {
		return fmt.Errorf("failed to get source info: %w", err)
	}

	if sourceInfo.IsDir() {
		// Handle directory installation
		logger.Debug("Installing from directory", "path", absSource)
		return installPluginFromDirectory(logger, absSource, pluginsDir)
	} else if filepath.Ext(absSource) == ".hcl" {
		// Handle .hcl file installation
		logger.Debug("Installing from .hcl file", "path", absSource)
		return installPluginFromManifest(logger, absSource, pluginsDir)
	} else {
		return fmt.Errorf("invalid source: must be a directory or .hcl file")
	}
}

func installPluginFromDirectory(logger *log.Logger, sourceDir, pluginsDir string) error {
	// Find the .hcl file in the source directory
	var manifestPath string
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".hcl" {
			manifestPath = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to find manifest file: %w", err)
	}
	if manifestPath == "" {
		return fmt.Errorf("no .hcl manifest file found in the directory")
	}

	// Install using the found manifest file
	return installPluginFromManifest(logger, manifestPath, pluginsDir)
}

func installPluginFromManifest(logger *log.Logger, manifestPath, pluginsDir string) error {
	logger.Debug("Starting plugin installation", "manifestPath", manifestPath, "pluginsDir", pluginsDir)

	manifest, err := loadPluginManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}
	logger.Debug("Loaded plugin manifest", "name", manifest.Plugin.Name)

	// Create a directory for the plugin in the plugins directory
	pluginDir := filepath.Join(pluginsDir, manifest.Plugin.Name)
	logger.Debug("Preparing plugin directory", "path", pluginDir)

	// Check if pluginDir already exists
	if fileInfo, err := os.Stat(pluginDir); err == nil {
		if fileInfo.IsDir() {
			logger.Warn("Plugin directory already exists, removing it", "path", pluginDir)
			if err := os.RemoveAll(pluginDir); err != nil {
				return fmt.Errorf("failed to remove existing plugin directory: %w", err)
			}
		} else {
			logger.Warn("A file exists with the same name as the plugin directory, removing it", "path", pluginDir)
			if err := os.Remove(pluginDir); err != nil {
				return fmt.Errorf("failed to remove existing file: %w", err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("error checking plugin directory: %w", err)
	}

	logger.Debug("Creating plugin directory", "path", pluginDir)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Copy the manifest file
	destManifestPath := filepath.Join(pluginDir, filepath.Base(manifestPath))
	logger.Debug("Copying manifest file", "from", manifestPath, "to", destManifestPath)
	if err := copyFile(manifestPath, destManifestPath); err != nil {
		return fmt.Errorf("failed to copy manifest file: %w", err)
	}

	// Copy the plugin source files
	sourceDir := filepath.Dir(manifestPath)
	logger.Debug("Copying plugin source files", "from", sourceDir, "to", pluginDir)
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) != ".hcl" {
			relPath, _ := filepath.Rel(sourceDir, path)
			destPath := filepath.Join(pluginDir, relPath)
			logger.Debug("Copying file", "from", path, "to", destPath)
			if err := copyFile(path, destPath); err != nil {
				return fmt.Errorf("failed to copy file %s: %w", relPath, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to copy plugin files: %w", err)
	}

	logger.Info("Plugin installed successfully", "name", manifest.Plugin.Name, "path", pluginDir)
	return nil
}

func loadPluginManifest(path string) (*PluginManifest, error) {
	var manifest PluginManifest
	err := hclsimple.DecodeFile(path, nil, &manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}
	return &manifest, nil
}

func copyFile(src, dst string) error {
	// Ensure the destination directory exists
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	input, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	err = os.WriteFile(dst, input, 0644)
	if err != nil {
		return fmt.Errorf("failed to write destination file: %w", err)
	}

	return nil
}

func uninstallPlugin(logger *log.Logger, name string) error {
	pluginsDir, err := getPluginsDir()
	if err != nil {
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}

	pluginDir := filepath.Join(pluginsDir, name)
	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("failed to remove plugin directory: %w", err)
	}

	logger.Info("Plugin uninstalled successfully", "name", name)
	return nil
}

func printInstalledPlugins(logger *log.Logger) error {
	pluginsDir, err := getPluginsDir()
	if err != nil {
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return fmt.Errorf("failed to read plugins directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Println(entry.Name())
		}
	}

	return nil
}

func loadPlugin(path string) (GitspacePlugin, error) {
	// This is a placeholder implementation. You'll need to implement
	// the actual plugin loading logic based on your plugin system.
	return nil, fmt.Errorf("plugin loading not implemented")
}
