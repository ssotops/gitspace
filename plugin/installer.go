package plugin

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create plugins directory: %w", err)
	}

	return pluginsDir, nil
}

func InstallPlugin(logger *log.Logger, source string) error {
	logger.Debug("Starting plugin installation", "source", source)

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
		catalogItem := strings.TrimPrefix(source, "catalog://")
		return installFromGitspaceCatalog(logger, catalogItem)
	} else if isRemote {
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
		absSource, err := filepath.Abs(source)
		if err != nil {
			return fmt.Errorf("failed to get absolute path of source: %w", err)
		}
		logger.Debug("Absolute source path", "path", absSource)

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

	pluginDir := filepath.Join(pluginsDir, manifest.Metadata.Name)
	logger.Debug("Preparing plugin directory", "path", pluginDir)

	if err := os.RemoveAll(pluginDir); err != nil {
		return fmt.Errorf("failed to remove existing plugin directory: %w", err)
	}

	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	destManifestPath := filepath.Join(pluginDir, "gitspace-plugin.toml")
	logger.Debug("Copying manifest file", "from", manifestPath, "to", destManifestPath)
	if err := copyFile(manifestPath, destManifestPath); err != nil {
		return fmt.Errorf("failed to copy manifest file: %w", err)
	}

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

	rawGitHubURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, defaultBranch, plugin.Path)

	pluginsDir, err := getPluginsDir()
	if err != nil {
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}

	pluginDir := filepath.Join(pluginsDir, catalogItem)
	if err := os.MkdirAll(pluginDir, 0755); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	manifestURL := fmt.Sprintf("%s/gitspace-plugin.toml", rawGitHubURL)
	manifestPath := filepath.Join(pluginDir, "gitspace-plugin.toml")
	err = downloadFile(manifestURL, manifestPath)
	if err != nil {
		return fmt.Errorf("failed to download gitspace-plugin.toml: %w", err)
	}

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

func UninstallPlugin(logger *log.Logger, name string) error {
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

func ListInstalledPlugins(logger *log.Logger) ([]string, error) {
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

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err = copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err = copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func fetchGitspaceCatalog(owner, repo string) (*GitspaceCatalog, error) {
	// TODO: Implement GitHub API call to fetch catalog
	return &GitspaceCatalog{
		Plugins: make(map[string]Plugin),
	}, nil
}

type GitspaceCatalog struct {
	Catalog struct {
		Name        string `toml:"name"`
		Description string `toml:"description"`
		Version     string `toml:"version"`
		LastUpdated struct {
			Date       string `toml:"date"`
			CommitHash string `toml:"commit_hash"`
		} `toml:"last_updated"`
	} `toml:"catalog"`
	Plugins   map[string]Plugin   `toml:"plugins"`
	Templates map[string]Template `toml:"templates"`
}

type MenuItem struct {
	Label   string
	Command string
}

// Update the Plugin struct
type Plugin struct {
	Name        string
	Path        string
	Version     string `toml:"version"`
	Description string `toml:"description"`
	Repository  struct {
		Type string `toml:"type"`
		URL  string `toml:"url"`
	} `toml:"repository"`
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

type CatalogPlugin struct {
	Path string
	// Add other necessary fields
}

type Template struct {
	Version     string `toml:"version,omitempty"`
	Description string `toml:"description,omitempty"`
	Path        string `toml:"path"`
	Repository  struct {
		Type string `toml:"type"`
		URL  string `toml:"url"`
	} `toml:"repository"`
}
