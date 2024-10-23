package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/ssotops/gitspace-plugin-sdk/logger"
	"github.com/ssotops/gitspace/lib"
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

func InstallPlugin(logger *logger.RateLimitedLogger, manager *Manager, source string) error {
	logger.Debug("Starting plugin installation", "source", source)

	// Ensure plugin directory permissions
	if err := EnsurePluginDirectoryPermissions(logger); err != nil {
		return fmt.Errorf("failed to ensure plugin directory permissions: %w", err)
	}

	source = strings.TrimSpace(source)
	pluginsDir, err := getPluginsDir()
	if err != nil {
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}

	isGitspaceCatalog := strings.HasPrefix(source, "https://github.com/ssotops/gitspace-catalog/tree/main/")
	isRemote := strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://")

	var sourceDir string

	if isRemote {
		tempDir, err := os.MkdirTemp("", "gitspace-plugin-*")
		if err != nil {
			return fmt.Errorf("failed to create temporary directory: %w", err)
		}
		defer os.RemoveAll(tempDir)

		if isGitspaceCatalog {
			if err := downloadFromGitspaceCatalog(logger, source, tempDir); err != nil {
				return err
			}
		} else {
			if err := gitClone(source, tempDir); err != nil {
				return err
			}
		}
		sourceDir = tempDir
	} else {
		absSource, err := filepath.Abs(source)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %w", err)
		}
		sourceDir = absSource
	}

	// Load and validate manifest
	manifest, err := loadPluginManifest(filepath.Join(sourceDir, "gitspace-plugin.toml"))
	if err != nil {
		return fmt.Errorf("failed to load plugin manifest: %w", err)
	}

	pluginName := manifest.Metadata.Name
	destDir := filepath.Join(pluginsDir, pluginName)

	// Set up Go module
	logger.Debug("Setting up Go module", "dir", sourceDir)
	modInit := exec.Command("go", "mod", "init", fmt.Sprintf("github.com/ssotops/gitspace-catalog/plugins/%s", pluginName))
	modInit.Dir = sourceDir
	if output, err := modInit.CombinedOutput(); err != nil {
		logger.Debug("Module init output", "output", string(output))
		// Ignore error if module already exists
	}

	// Remove any existing replacements
	logger.Debug("Removing existing replacements")
	modEdit := exec.Command("go", "mod", "edit", "-dropreplace", "github.com/ssotops/gitspace-plugin-sdk")
	modEdit.Dir = sourceDir
	if output, err := modEdit.CombinedOutput(); err != nil {
		logger.Debug("Module edit output", "output", string(output))
		// Ignore error if no replacements exist
	}

	// Get latest dependencies
	logger.Debug("Getting latest dependencies")
	getCmd := exec.Command("go", "get", "github.com/ssotops/gitspace-plugin-sdk@latest")
	getCmd.Dir = sourceDir
	if output, err := getCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to get dependencies: %w\nOutput: %s", err, output)
	}

	// Tidy up modules
	logger.Debug("Tidying modules")
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = sourceDir
	if output, err := tidyCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to tidy modules: %w\nOutput: %s", err, output)
	}

	// Build the plugin
	logger.Info("Building plugin", "name", pluginName)
	buildCmd := exec.Command("go", "build", "-o", pluginName)
	buildCmd.Dir = sourceDir
	buildCmd.Env = append(os.Environ(), "GO111MODULE=on")

	output, err := buildCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to build plugin: %w\nOutput: %s", err, output)
	}

	// Create plugin directory and install files
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	// Copy and make executable the plugin binary
	binaryPath := filepath.Join(sourceDir, pluginName)
	destBinaryPath := filepath.Join(destDir, pluginName)
	if err := copyFile(binaryPath, destBinaryPath); err != nil {
		return fmt.Errorf("failed to copy plugin binary: %w", err)
	}
	if err := os.Chmod(destBinaryPath, 0755); err != nil {
		return fmt.Errorf("failed to make plugin executable: %w", err)
	}

	// Create data directory and copy support files
	dataDir := filepath.Join(pluginsDir, "data", pluginName)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := copyDir(sourceDir, dataDir); err != nil {
		return fmt.Errorf("failed to copy plugin files: %w", err)
	}

	// Add to discovered plugins
	manager.AddDiscoveredPlugin(pluginName, destBinaryPath)

	logger.Info("Plugin installed successfully", "name", pluginName)
	return nil
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

func downloadFromGitspaceCatalog(logger *logger.RateLimitedLogger, source, tempDir string) error {
	parts := strings.Split(strings.TrimPrefix(source, "https://github.com/"), "/")
	if len(parts) < 5 {
		return fmt.Errorf("invalid Gitspace Catalog URL: %s", source)
	}

	owner := parts[0]
	repo := parts[1]
	path := strings.Join(parts[4:], "/")

	logger.Debug("Downloading from Gitspace Catalog",
		"owner", owner,
		"repo", repo,
		"path", path,
		"dest", tempDir)

	ctx := context.Background()
	return lib.DownloadDirectory(ctx, lib.SCMTypeGitHub, "", owner, repo, path, tempDir)
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

func UninstallPlugin(logger *logger.RateLimitedLogger, name string) error {
	pluginsDir, err := getPluginsDir()
	if err != nil {
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}

	// Remove plugin directory
	pluginDir := filepath.Join(pluginsDir, name)
	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("failed to remove plugin directory: %w", err)
	}

	// Remove data directory
	dataDir := filepath.Join(pluginsDir, "data", name)
	if err := os.RemoveAll(dataDir); err != nil {
		logger.Warn("Failed to remove plugin data directory", "error", err)
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
		if entry.IsDir() && entry.Name() != "data" {
			plugins = append(plugins, entry.Name())
		}
	}

	return plugins, nil
}
