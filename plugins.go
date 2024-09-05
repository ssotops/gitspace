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
	Name        string `hcl:"name,label"`
	Version     string `hcl:"version"`
	Description string `hcl:"description"`
	Author      string `hcl:"author"`
	EntryPoint  string `hcl:"entry_point"`
	Source      struct {
		Type       string `hcl:"type"`
		Repository string `hcl:"repository,optional"`
		Branch     string `hcl:"branch,optional"`
		URL        string `hcl:"url,optional"`
		Path       string `hcl:"path,optional"`
	} `hcl:"source,block"`
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

	// Download or copy the plugin based on the source type
	var pluginPath string
	if isURL(source) {
		pluginPath = filepath.Join(pluginsDir, filepath.Base(source))
		if err := downloadFile(source, pluginPath); err != nil {
			return fmt.Errorf("failed to download plugin: %w", err)
		}
	} else {
		pluginPath = filepath.Join(pluginsDir, filepath.Base(source))
		if err := copyFile(source, pluginPath); err != nil {
			return fmt.Errorf("failed to copy plugin: %w", err)
		}
	}

	logger.Info("Plugin installed successfully", "path", pluginPath)
	return nil
}

func uninstallPlugin(logger *log.Logger, name string) error {
	pluginsDir, err := getPluginsDir()
	if err != nil {
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}

	pluginPath := filepath.Join(pluginsDir, name)
	if err := os.Remove(pluginPath); err != nil {
		return fmt.Errorf("failed to remove plugin: %w", err)
	}

	logger.Info("Plugin uninstalled successfully", "name", name)
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
			continue
		}
		manifest, err := loadPluginManifest(filepath.Join(pluginsDir, p.Name()))
		if err != nil {
			logger.Warn("Failed to load plugin manifest", "plugin", p.Name(), "error", err)
			continue
		}
		fmt.Printf("- %s (v%s): %s\n", manifest.Name, manifest.Version, manifest.Description)
	}

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
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin: %w", err)
	}

	symPlugin, err := p.Lookup("HelloWorld")
	if err != nil {
		return nil, fmt.Errorf("failed to lookup 'HelloWorld' symbol: %w", err)
	}

	var gp GitspacePlugin
	gp, ok := symPlugin.(GitspacePlugin)
	if !ok {
		return nil, fmt.Errorf("unexpected type from module symbol")
	}

	return gp, nil
}
