package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	defaultPath := "./gs.toml"

	for {
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
				continue
			case "exit":
				os.Exit(0)
			}
		} else {
			logger.Info("Config file loaded successfully", "path", configPath)
			return config, nil
		}
	}
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
