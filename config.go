package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/mitchellh/go-homedir"
	"github.com/pelletier/go-toml"
	"github.com/ssotops/gitspace-plugin-sdk/logger"
)

type Config struct {
	Global struct {
		Path                   string `toml:"path"`
		SCM                    string `toml:"scm"`
		Owner                  string `toml:"owner"`
		BaseURL                string `toml:"base_url"`
		EmptyRepoInitialBranch string `toml:"empty_repo_initial_branch"`
	} `toml:"global"`
	Auth struct {
		Type    string `toml:"type"`
		KeyPath string `toml:"key_path"`
	} `toml:"auth"`
	Groups map[string]Group `toml:"groups"`
}

type Group struct {
	Match  string   `toml:"match"`
	Values []string `toml:"values"`
	Type   string   `toml:"type,omitempty"`
}

const (
	managedConfigDir = "/.ssot/gitspace/configs/active" // Where we store our active config
	configBackupDir  = "/.ssot/gitspace/configs/backup" // Where we store backups
	activeConfigFile = "current.toml"                   // The name of our active config file

	// Legacy paths (needed for transition/compatibility)
	configSymlinkDir = "/.ssot/gitspace/.symlinks"    // Legacy symlink directory
	lastConfigPath   = "/.ssot/gitspace/.last_config" // Path to store last used config
)

func getSSHKeyPath(configPath string) (string, error) {
	if strings.HasPrefix(configPath, "$") {
		envVar := strings.TrimPrefix(configPath, "$")
		path := os.Getenv(envVar)
		if path == "" {
			return "", fmt.Errorf("environment variable %s is not set", envVar)
		}
		return path, nil
	}
	return configPath, nil
}

func getCacheDir() (string, error) {
	homeDir, err := homedir.Dir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	cacheDir := filepath.Join(homeDir, ".ssot", "gitspace")
	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}
	return cacheDir, nil
}

func getConfigFromUser(logger *logger.RateLimitedLogger) (*Config, error) {
	defaultPath := "gs.toml"

	var configPath string
	err := huh.NewInput().
		Title("Enter the path to your config file (optional; just hit Enter for default)").
		Placeholder(defaultPath).
		Value(&configPath).
		Run()

	if err != nil {
		return nil, fmt.Errorf("error getting config path: %w", err)
	}

	if configPath == "" {
		configPath = defaultPath
		logger.Info("Using default config file path", "path", configPath)
	}

	// First load and validate the config
	config, err := loadConfig(configPath)
	if err != nil {
		logger.Error("Error reading config file", "error", err, "path", configPath)
		var choice string
		err := huh.NewSelect[string]().
			Title("What would you like to do?").
			Options(
				huh.NewOption("Proceed to Gitspace main menu", "proceed"),
				huh.NewOption("Re-enter the file path", "retry"),
				huh.NewOption("Exit", "exit"),
			).
			Value(&choice).
			Run()

		if err != nil {
			return nil, fmt.Errorf("error getting user choice: %w", err)
		}

		switch choice {
		case "proceed":
			return nil, nil
		case "retry":
			return getConfigFromUser(logger)
		case "exit":
			os.Exit(0)
		}
		return nil, nil
	}

	// If config is valid, install it to our managed directory
	if err := installConfig(logger, configPath); err != nil {
		logger.Error("Failed to install config", "error", err)
		return nil, err
	}

	logger.Info("Config installed and activated successfully", "path", configPath)
	return config, nil
}

func loadConfig(path string) (*Config, error) {
	config := &Config{}
	tree, err := toml.LoadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load TOML file: %w", err)
	}

	err = tree.Unmarshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal TOML: %w", err)
	}

	// Validate required fields
	if config.Global.Path == "" {
		return nil, fmt.Errorf("global.path is required")
	}
	if config.Global.SCM == "" {
		return nil, fmt.Errorf("global.scm is required")
	}
	if config.Global.Owner == "" {
		return nil, fmt.Errorf("global.owner is required")
	}
	if config.Global.EmptyRepoInitialBranch == "" {
		config.Global.EmptyRepoInitialBranch = "master"
	}

	return config, nil
}

// getLastUsedConfig retrieves the path of the last successfully used config file
func getLastUsedConfig(logger *logger.RateLimitedLogger) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	lastConfigFile := filepath.Join(homeDir, lastConfigPath)
	data, err := os.ReadFile(lastConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read last config path: %w", err)
	}

	return string(data), nil
}

// saveLastUsedConfig stores the path of the last successfully used config file
func saveLastUsedConfig(logger *logger.RateLimitedLogger, configPath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	lastConfigDir := filepath.Dir(filepath.Join(homeDir, lastConfigPath))
	if err := os.MkdirAll(lastConfigDir, 0755); err != nil {
		return fmt.Errorf("failed to create last config directory: %w", err)
	}

	lastConfigFile := filepath.Join(homeDir, lastConfigPath)
	if err := os.WriteFile(lastConfigFile, []byte(configPath), 0644); err != nil {
		return fmt.Errorf("failed to write last config path: %w", err)
	}

	return nil
}

// backupConfig creates a backup of the config file and creates a symlink
func backupConfig(logger *logger.RateLimitedLogger, originalPath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create backup directory
	backupDir := filepath.Join(homeDir, configBackupDir)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Create symlink directory
	symlinkDir := filepath.Join(homeDir, configSymlinkDir)
	if err := os.MkdirAll(symlinkDir, 0755); err != nil {
		return fmt.Errorf("failed to create symlink directory: %w", err)
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("gs_%s.toml", timestamp)
	backupPath := filepath.Join(backupDir, backupName)

	// Copy original file to backup
	input, err := os.ReadFile(originalPath)
	if err != nil {
		return fmt.Errorf("failed to read original config: %w", err)
	}

	if err := os.WriteFile(backupPath, input, 0644); err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	// Create symlink to original file
	symlinkPath := filepath.Join(symlinkDir, "current_config.toml")
	// Remove existing symlink if it exists
	os.Remove(symlinkPath)

	absOriginalPath, err := filepath.Abs(originalPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	if err := os.Symlink(absOriginalPath, symlinkPath); err != nil {
		logger.Warn("Failed to create symlink, will use backup file", "error", err)
	}

	return saveLastUsedConfig(logger, originalPath)
}

// getCurrentConfigPath attempts to get the current config file path
func getCurrentConfigPath(logger *logger.RateLimitedLogger) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	activePath := filepath.Join(homeDir, managedConfigDir, activeConfigFile)

	// Check if the active config exists and is valid
	if _, err := os.Stat(activePath); err == nil {
		if isGitspaceConfig(activePath) {
			return activePath, nil
		}
		logger.Debug("Found active config but it's invalid", "path", activePath)
	}

	return "", nil
}

// deleteCurrentConfig removes the current config symlink and backup
func deleteCurrentConfig(logger *logger.RateLimitedLogger) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Remove active config
	activePath := filepath.Join(homeDir, managedConfigDir, activeConfigFile)
	if err := os.Remove(activePath); err != nil && !os.IsNotExist(err) {
		logger.Warn("Failed to remove active config", "error", err)
	}

	logger.Info("Current config deleted successfully")
	return nil
}

// Add a new function to distinguish gitspace configs from other .toml files
func isGitspaceConfig(path string) bool {
	// Check if the file exists
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if info.IsDir() {
		return false
	}

	// Read the file
	config, err := loadConfig(path)
	if err != nil {
		return false
	}

	// A valid gitspace config must have these fields
	return config.Global.SCM != "" &&
		config.Global.Owner != "" &&
		config.Global.Path != ""
}

func installConfig(logger *logger.RateLimitedLogger, sourcePath string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Ensure our managed config directories exist
	activeDir := filepath.Join(homeDir, managedConfigDir)
	backupDir := filepath.Join(homeDir, configBackupDir)

	for _, dir := range []string{activeDir, backupDir} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Read the source config
	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source config: %w", err)
	}

	// Validate it's a proper config before proceeding
	config := &Config{}
	if err := toml.Unmarshal(sourceData, config); err != nil {
		return fmt.Errorf("invalid config file: %w", err)
	}

	// Create backup with timestamp
	backupName := fmt.Sprintf("config_%s.toml", time.Now().Format("20060102_150405"))
	backupPath := filepath.Join(backupDir, backupName)
	if err := os.WriteFile(backupPath, sourceData, 0644); err != nil {
		logger.Warn("Failed to create backup", "error", err)
	}

	// Install as active config
	activePath := filepath.Join(activeDir, activeConfigFile)
	if err := os.WriteFile(activePath, sourceData, 0644); err != nil {
		return fmt.Errorf("failed to write active config: %w", err)
	}

	logger.Info("Config installed successfully",
		"source", sourcePath,
		"active", activePath,
		"backup", backupPath)

	return nil
}
