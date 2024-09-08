package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mitchellh/go-homedir"
)

type Config struct {
	Gitspace struct {
		Path   string   `hcl:"path"`
		Labels []string `hcl:"labels,optional"`
		Clone  *struct {
			SCM        string                     `hcl:"scm"`
			Owner      string                     `hcl:"owner"`
			Auth       *AuthConfig                `hcl:"auth,block"`
			StartsWith map[string]*DirectiveGroup `hcl:"startsWith,block"`
			EndsWith   map[string]*DirectiveGroup `hcl:"endsWith,block"`
			Includes   map[string]*DirectiveGroup `hcl:"includes,block"`
			IsExactly  map[string]*DirectiveGroup `hcl:"isExactly,block"`
		} `hcl:"clone,block"`
	} `hcl:"gitspace,block"`
}

type AuthConfig struct {
	Type    string `hcl:"type"`
	KeyPath string `hcl:"keyPath"`
}

type DirectiveGroup struct {
	Values []string `hcl:"values"`
	Type   string   `hcl:"type,optional"`
	Labels []string `hcl:"labels,optional"`
}

type GroupConfig struct {
	Name   string   `hcl:"name,label"`
	Values []string `hcl:"values"`
	Type   string   `hcl:"type,optional"`
	Labels []string `hcl:"labels,optional"`
}

type IndexHCL struct {
	LastUpdated  string                     `hcl:"lastUpdated"`
	Repositories map[string]SCMRepositories `hcl:"repositories"`
}

type SCMRepositories struct {
	Owners map[string]OwnerRepositories `hcl:"owners"`
}

type OwnerRepositories struct {
	Repos map[string]RepoInfo `hcl:"repos"`
}

type RepoInfo struct {
	ConfigPath string    `hcl:"configPath"`
	BackupPath string    `hcl:"backupPath"`
	LastCloned time.Time `hcl:"lastCloned"`
	LastSynced time.Time `hcl:"lastSynced"`
}

func getConfigFromUser(logger *log.Logger) (*Config, error) {
	defaultPath := "./gs.hcl"

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

		// If the input is empty, use the default path
		if configPath == "" {
			configPath = defaultPath
			logger.Info("Using default config file path", "path", configPath)
		}

		config, err := decodeHCLFile(configPath)
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
			return &config, nil
		}
	}
}

func decodeHCLFile(filename string) (Config, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read file: %w", err)
	}

	file, diags := hclsyntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return Config{}, fmt.Errorf("failed to parse HCL: %s", formatDiagnostics(diags))
	}

	var config Config
	decodeDiags := gohcl.DecodeBody(file.Body, nil, &config)
	if decodeDiags.HasErrors() {
		return Config{}, fmt.Errorf("failed to decode HCL: %s", formatDiagnostics(decodeDiags))
	}

	// Validate required fields
	if config.Gitspace.Path == "" {
		return Config{}, fmt.Errorf("gitspace.path is required")
	}
	if config.Gitspace.Clone == nil {
		return Config{}, fmt.Errorf("gitspace.clone block is required")
	}
	if config.Gitspace.Clone.SCM == "" {
		return Config{}, fmt.Errorf("gitspace.clone.scm is required")
	}
	if config.Gitspace.Clone.Owner == "" {
		return Config{}, fmt.Errorf("gitspace.clone.owner is required")
	}

	return config, nil
}

func formatDiagnostics(diags hcl.Diagnostics) string {
	var messages []string
	for _, diag := range diags {
		severity := "Error"
		if diag.Severity == hcl.DiagWarning {
			severity = "Warning"
		}
		messages = append(messages, fmt.Sprintf("%s: %s (%s)", severity, diag.Summary, diag.Detail))
	}
	return strings.Join(messages, "\n")
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
