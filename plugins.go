package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"plugin"
	"strings"

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
		return "", err
	}
	return filepath.Join(homeDir, ".ssot", "gitspace", "plugins"), nil
}

func installPlugin(logger *log.Logger, source string) error {
	pluginsDir, err := getPluginsDir()
	if err != nil {
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}

	// Ensure plugins directory exists
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugins directory: %w", err)
	}

	sourceInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("failed to get source info: %w", err)
	}

	if sourceInfo.IsDir() {
		// Handle directory installation
		return installPluginFromDirectory(logger, source, pluginsDir)
	} else if filepath.Ext(source) == ".hcl" {
		// Handle .hcl file installation
		return installPluginFromManifest(logger, source, pluginsDir)
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
	manifest, err := loadPluginManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	// Create a directory for the plugin
	pluginDir := filepath.Join(pluginsDir, manifest.Plugin.Name)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Copy the manifest file
	destManifestPath := filepath.Join(pluginDir, filepath.Base(manifestPath))
	if err := copyFile(manifestPath, destManifestPath); err != nil {
		return fmt.Errorf("failed to copy manifest file: %w", err)
	}

	// Copy the plugin source files
	sourceDir := filepath.Dir(manifestPath)
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) != ".hcl" {
			relPath, _ := filepath.Rel(sourceDir, path)
			destPath := filepath.Join(pluginDir, relPath)
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

func printInstalledPlugins(logger *log.Logger) error {
	pluginsDir, err := getPluginsDir()
	if err != nil {
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}

	plugins, err := os.ReadDir(pluginsDir)
	if err != nil {
		return fmt.Errorf("failed to read plugins directory: %w", err)
	}

	fmt.Println("Installed plugins:")
	for _, p := range plugins {
		if p.IsDir() {
			manifestPath := filepath.Join(pluginsDir, p.Name(), p.Name()+".hcl")
			manifest, err := loadPluginManifest(manifestPath)
			if err != nil {
				logger.Warn("Failed to load plugin manifest", "plugin", p.Name(), "error", err)
				continue
			}
			fmt.Printf("- %s (v%s): %s\n", manifest.Plugin.Name, manifest.Plugin.Version, manifest.Plugin.Description)
		}
	}

	return nil
}

func loadPluginManifest(path string) (*PluginManifest, error) {
	var manifest PluginManifest
	err := hclsimple.DecodeFile(path, nil, &manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	// Validate required fields
	var missingFields []string
	if manifest.Plugin.Name == "" {
		missingFields = append(missingFields, "name")
	}
	if manifest.Plugin.Version == "" {
		missingFields = append(missingFields, "version")
	}
	if manifest.Plugin.EntryPoint == "" {
		missingFields = append(missingFields, "entry_point")
	}

	if len(missingFields) > 0 {
		return nil, fmt.Errorf("manifest is missing required fields: %s", strings.Join(missingFields, ", "))
	}

	return &manifest, nil
}

func isURL(str string) bool {
	return strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://")
}

func downloadFile(url, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func loadPlugin(path string) (GitspacePlugin, error) {
	manifestPath := filepath.Join(path, filepath.Base(path)+".hcl")
	manifest, err := loadPluginManifest(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin manifest: %w", err)
	}

	p, err := plugin.Open(filepath.Join(path, manifest.Plugin.EntryPoint))
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin: %w", err)
	}

	symPlugin, err := p.Lookup("Plugin")
	if err != nil {
		return nil, fmt.Errorf("failed to lookup 'Plugin' symbol: %w", err)
	}

	var gp GitspacePlugin
	gp, ok := symPlugin.(GitspacePlugin)
	if !ok {
		return nil, fmt.Errorf("unexpected type from module symbol")
	}

	return gp, nil
}

func uninstallPlugin(logger *log.Logger, pluginName string) error {
    pluginsDir, err := getPluginsDir()
    if err != nil {
        return fmt.Errorf("failed to get plugins directory: %w", err)
    }

    pluginDir := filepath.Join(pluginsDir, pluginName)
    
    // Check if the plugin directory exists
    if _, err := os.Stat(pluginDir); os.IsNotExist(err) {
        return fmt.Errorf("plugin '%s' is not installed", pluginName)
    }

    // Remove the plugin directory
    err = os.RemoveAll(pluginDir)
    if err != nil {
        return fmt.Errorf("failed to remove plugin directory: %w", err)
    }

    logger.Info("Plugin uninstalled successfully", "name", pluginName)
    return nil
}
