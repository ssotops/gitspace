package plugin

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/ssotops/gitspace/logger"
)

type PluginManifest struct {
	Metadata struct {
		Name        string `toml:"name"`
		Version     string `toml:"version"`
		Description string `toml:"description"`
	} `toml:"metadata"`
	Sources []struct {
		Path       string `toml:"path"`
		EntryPoint string `toml:"entry_point"`
	} `toml:"sources"`
}

func getPluginsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	pluginsDir := filepath.Join(homeDir, ".ssot", "gitspace", "plugins")

	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create plugins directory: %w", err)
	}

	return pluginsDir, nil
}

func InstallPlugin(logger *logger.RateLimitedLogger, manager *Manager, source string) error {
	logger.Debug("Starting plugin installation", "source", source)

	// Ensure plugin directory permissions are correct
	err := EnsurePluginDirectoryPermissions(logger)
	if err != nil {
		logger.Error("Failed to ensure plugin directory permissions", "error", err)
		return fmt.Errorf("failed to ensure plugin directory permissions: %w", err)
	}

	source = strings.TrimSpace(source)

	pluginsDir, err := getPluginsDir()
	if err != nil {
		logger.Error("Failed to get plugins directory", "error", err)
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}
	logger.Debug("Plugins directory", "path", pluginsDir)

	isRemote := strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")
	isCatalog := strings.HasPrefix(source, "catalog://")
	logger.Debug("Source type", "isRemote", isRemote, "isCatalog", isCatalog)

	if isCatalog {
		catalogItem := strings.TrimPrefix(source, "catalog://")
		logger.Debug("Installing from Gitspace Catalog", "item", catalogItem)
		return installFromGitspaceCatalog(logger, catalogItem)
	}

	var sourceDir string

	if isRemote {
		logger.Debug("Processing remote source")
		tempDir, err := os.MkdirTemp("", "gitspace-plugin-*")
		if err != nil {
			logger.Error("Failed to create temporary directory", "error", err)
			return fmt.Errorf("failed to create temporary directory: %w", err)
		}
		defer os.RemoveAll(tempDir)

		logger.Debug("Cloning remote repository", "source", source, "tempDir", tempDir)
		err = gitClone(source, tempDir)
		if err != nil {
			logger.Error("Failed to clone remote repository", "error", err)
			return fmt.Errorf("failed to clone remote repository: %w", err)
		}

		sourceDir = tempDir
	} else {
		logger.Debug("Processing local source")
		absSource, err := filepath.Abs(source)
		if err != nil {
			logger.Error("Failed to get absolute path of source", "error", err)
			return fmt.Errorf("failed to get absolute path of source: %w", err)
		}
		logger.Debug("Absolute source path", "path", absSource)

		sourceInfo, err := os.Stat(absSource)
		if err != nil {
			if os.IsNotExist(err) {
				logger.Error("Specified path does not exist", "path", absSource)
				return fmt.Errorf("the specified path does not exist: %s", absSource)
			}
			logger.Error("Failed to get source info", "error", err)
			return fmt.Errorf("failed to get source info: %w", err)
		}

		if !sourceInfo.IsDir() {
			logger.Error("Specified path is not a directory", "path", absSource)
			return fmt.Errorf("the specified path is not a directory: %s", absSource)
		}

		sourceDir = absSource
	}

	manifestPath := filepath.Join(sourceDir, "gitspace-plugin.toml")
	logger.Debug("Attempting to load plugin manifest", "path", manifestPath)
	manifest, err := loadPluginManifest(manifestPath)
	if err != nil {
		logger.Error("Failed to load plugin manifest", "error", err)
		return fmt.Errorf("failed to load plugin manifest: %w", err)
	}
	logger.Debug("Successfully loaded plugin manifest")

	pluginName := manifest.Metadata.Name
	logger.Debug("Plugin name from manifest", "name", pluginName)

	// Copy plugin files to plugins directory
	destDir := filepath.Join(pluginsDir, pluginName)
	logger.Debug("Copying plugin files", "from", sourceDir, "to", destDir)
	err = copyDir(sourceDir, destDir)
	if err != nil {
		logger.Error("Failed to copy plugin files", "error", err)
		return fmt.Errorf("failed to copy plugin files: %w", err)
	}
	logger.Debug("Successfully copied plugin files")

	err = EnsurePluginDirectoryPermissions(logger)
	if err != nil {
		logger.Error("Failed to ensure plugin directory permissions after copying files", "error", err)
		return fmt.Errorf("failed to ensure plugin directory permissions after copying files: %w", err)
	}

	// After successfully copying files and setting permissions
	pluginExecutable := filepath.Join(destDir, pluginName)
	logger.Debug("Adding plugin to discovered plugins", "name", pluginName, "path", pluginExecutable)
	manager.AddDiscoveredPlugin(pluginName, pluginExecutable)

	// Load the plugin
	logger.Debug("Attempting to load plugin", "name", pluginName)
	err = manager.LoadPlugin(pluginName)
	if err != nil {
		logger.Error("Error loading plugin", "name", pluginName, "error", err)
		return fmt.Errorf("error loading plugin: %w", err)
	}
	logger.Debug("Successfully loaded plugin", "name", pluginName)

	logger.Info("Plugin installed and loaded successfully", "name", pluginName)
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

func UninstallPlugin(logger *logger.RateLimitedLogger, name string) error {
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

func ListInstalledPlugins(logger *logger.RateLimitedLogger) ([]string, error) {
	pluginsDir, err := getPluginsDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get plugins directory: %w", err)
	}

	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugins directory: %w", err)
	}

	var plugins []string
	for _, entry := range entries {
		if entry.IsDir() {
			plugins = append(plugins, entry.Name())
		}
	}

	return plugins, nil
}

// Helper functions
func gitClone(url, destPath string) error {
	cmd := exec.Command("git", "clone", url, destPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, string(output))
	}
	return nil
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

func copyDir(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

func fetchGitspaceCatalog(owner, repo string) (*GitspaceCatalog, error) {
	// TODO: Implement GitHub API call to fetch catalog
	return &GitspaceCatalog{
		Plugins: make(map[string]Plugin),
	}, nil
}

func installFromGitspaceCatalog(logger *logger.RateLimitedLogger, catalogItem string) error {
	// For now, we'll just log that this feature is not implemented
	logger.Info("Installation from Gitspace Catalog is not implemented yet")
	return fmt.Errorf("installation from Gitspace Catalog is not implemented yet")
}