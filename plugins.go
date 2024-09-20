package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/pelletier/go-toml/v2"
)

type PluginManifest struct {
	Metadata struct {
		Name        string   `toml:"name"`
		Version     string   `toml:"version"`
		Description string   `toml:"description"`
		Author      string   `toml:"author"`
		Tags        []string `toml:"tags"`
	} `toml:"metadata"`
	Sources []struct {
		Path string `toml:"path"`
	} `toml:"sources"`
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
	logger.Debug("Starting plugin installation", "source", source)

	// Trim any leading or trailing whitespace
	source = strings.TrimSpace(source)

	pluginsDir, err := getPluginsDir()
	if err != nil {
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}
	logger.Debug("Plugins directory", "path", pluginsDir)

	isRemote := strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")
	isCatalog := strings.HasPrefix(source, "catalog://")
	logger.Debug("Source type", "isRemote", isRemote, "isCatalog", isCatalog)

	var manifestPath string
	var sourceDir string

	if isCatalog {
		// Handle Gitspace Catalog installation
		catalogItem := strings.TrimPrefix(source, "catalog://")
		return installFromGitspaceCatalog(logger, catalogItem)
	} else if isRemote {
		// Handle remote installation
		tempDir, err := os.MkdirTemp("", "gitspace-plugin-*")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory: %w", err)
		}
		defer os.RemoveAll(tempDir)

		logger.Debug("Cloning remote repository", "source", source, "tempDir", tempDir)
		err = gitClone(source, tempDir)
		if err != nil {
			return fmt.Errorf("failed to clone remote repository: %w", err)
		}

		manifestPath = filepath.Join(tempDir, "gitspace-plugin.toml")
		sourceDir = tempDir
	} else {
		// Handle local installation
		absSource, err := filepath.Abs(source)
		if err != nil {
			return fmt.Errorf("failed to get absolute path of source: %w", err)
		}
		logger.Debug("Absolute source path", "path", absSource)

		// Check if the directory exists
		sourceInfo, err := os.Stat(absSource)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("the specified path does not exist: %s", absSource)
			}
			return fmt.Errorf("failed to get source info: %w", err)
		}

		if !sourceInfo.IsDir() {
			return fmt.Errorf("the specified path is not a directory: %s", absSource)
		}

		manifestPath = filepath.Join(absSource, "gitspace-plugin.toml")
		sourceDir = absSource

		// Check if the manifest file exists
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			return fmt.Errorf("gitspace-plugin.toml not found in the specified directory: %s", absSource)
		}
	}

	logger.Debug("Loading plugin manifest", "path", manifestPath)
	manifest, err := loadPluginManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}
	logger.Debug("Loaded plugin manifest", "name", manifest.Metadata.Name)

	if manifest.Metadata.Name == "" {
		return fmt.Errorf("plugin name is empty in the manifest file")
	}

	// Create a directory for the plugin in the plugins directory
	pluginDir := filepath.Join(pluginsDir, manifest.Metadata.Name)
	logger.Debug("Preparing plugin directory", "path", pluginDir)

	// Remove existing plugin directory if it exists
	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("failed to remove existing plugin directory: %w", err)
	}

	// Create the plugin directory
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Copy the manifest file
	destManifestPath := filepath.Join(pluginDir, "gitspace-plugin.toml")
	logger.Debug("Copying manifest file", "from", manifestPath, "to", destManifestPath)
	if err := copyFile(manifestPath, destManifestPath); err != nil {
		return fmt.Errorf("failed to copy manifest file: %w", err)
	}

	// Copy the plugin source files
	logger.Debug("Copying plugin directory", "from", sourceDir, "to", pluginDir)
	if err := copyDir(sourceDir, pluginDir); err != nil {
		return fmt.Errorf("failed to copy plugin directory: %w", err)
	}

	logger.Info("Plugin installed successfully", "name", manifest.Metadata.Name, "path", pluginDir)
	return nil
}

func installFromGitspaceCatalog(logger *log.Logger, catalogItem string) error {
	owner := "ssotops"
	repo := "gitspace-catalog"
	defaultBranch := "master"
	catalog, err := fetchGitspaceCatalog(owner, repo)
	if err != nil {
		return fmt.Errorf("failed to fetch Gitspace Catalog: %w", err)
	}

	plugin, ok := catalog.Plugins[catalogItem]
	if !ok {
		return fmt.Errorf("plugin %s not found in Gitspace Catalog", catalogItem)
	}

	if plugin.Path == "" {
		return fmt.Errorf("no path found for plugin %s in Gitspace Catalog", catalogItem)
	}

	// Construct the raw GitHub URL for the plugin directory
	rawGitHubURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, defaultBranch, plugin.Path)

	// Get the plugins directory
	pluginsDir, err := getPluginsDir()
	if err != nil {
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}

	// Create a directory for the plugin
	pluginDir := filepath.Join(pluginsDir, catalogItem)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Download the gitspace-plugin.toml file
	manifestURL := fmt.Sprintf("%s/gitspace-plugin.toml", rawGitHubURL)
	manifestPath := filepath.Join(pluginDir, "gitspace-plugin.toml")
	err = downloadFile(manifestURL, manifestPath)
	if err != nil {
		return fmt.Errorf("failed to download gitspace-plugin.toml: %w", err)
	}

	// Download the source files
	manifest, err := loadPluginManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load manifest: %w", err)
	}

	for _, source := range manifest.Sources {
		sourceURL := fmt.Sprintf("%s/%s", rawGitHubURL, source.Path)
		destPath := filepath.Join(pluginDir, source.Path)
		err = downloadFile(sourceURL, destPath)
		if err != nil {
			return fmt.Errorf("failed to download %s: %w", source.Path, err)
		}
	}

	logger.Info("Plugin installed successfully", "name", catalogItem, "path", pluginDir)
	return nil
}

func downloadFile(url string, filepath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func loadPluginManifest(path string) (*PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest PluginManifest
	err = toml.Unmarshal(data, &manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to decode manifest: %w", err)
	}

	if manifest.Metadata.Name == "" {
		return nil, fmt.Errorf("plugin name is missing in the manifest file")
	}

	return &manifest, nil
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

// This function needs to be implemented or imported from the appropriate package
func fetchGitspaceCatalog(owner, repo string) (*GitspaceCatalog, error) {
	// Implementation needed
	return nil, fmt.Errorf("fetchGitspaceCatalog not implemented")
}

// GitspaceCatalog struct needs to be defined
type GitspaceCatalog struct {
	Plugins map[string]CatalogPlugin
}

type CatalogPlugin struct {
	Path string
	// Add other necessary fields
}

func handleGitspaceCatalogInstall(logger *log.Logger) (string, error) {
	owner := "ssotops"
	repo := "gitspace-catalog"
	catalog, err := fetchGitspaceCatalog(owner, repo)
	if err != nil {
		logger.Error("Failed to fetch Gitspace Catalog", "error", err)
		return "", err
	}

	var options []huh.Option[string]
	for name := range catalog.Plugins {
		options = append(options, huh.NewOption(name, "catalog://"+name))
	}

	if len(options) == 0 {
		return "", fmt.Errorf("no plugins found in the catalog")
	}

	var selectedItem string
	err = huh.NewSelect[string]().
		Title("Select a plugin to install").
		Options(options...).
		Value(&selectedItem).
		Run()

	if err != nil {
		return "", fmt.Errorf("failed to select item: %w", err)
	}

	return selectedItem, nil
}

func listInstalledPlugins(pluginsDir string) ([]huh.Option[string], error) {
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugins directory: %w", err)
	}

	var options []huh.Option[string]
	for _, entry := range entries {
		if entry.IsDir() {
			options = append(options, huh.NewOption(entry.Name(), entry.Name()))
		}
	}
	return options, nil
}
