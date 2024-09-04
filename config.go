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
	Repositories *struct {
		GitSpace *struct {
			Path string `hcl:"path"`
		} `hcl:"gitspace,block"`
		Labels []string `hcl:"labels,optional"`
		Clone  *struct {
			SCM        string        `hcl:"scm"`
			Owner      string        `hcl:"owner"`
			EndsWith   *FilterConfig `hcl:"endsWith,block"`
			StartsWith *FilterConfig `hcl:"startsWith,block"`
			Includes   *FilterConfig `hcl:"includes,block"`
			IsExactly  *FilterConfig `hcl:"isExactly,block"`
			Auth       *struct {
				Type    string `hcl:"type"`
				KeyPath string `hcl:"keyPath"`
			} `hcl:"auth,block"`
		} `hcl:"clone,block"`
	} `hcl:"repositories,block"`
}

type FilterConfig struct {
	Values     []string `hcl:"values"`
	Repository *struct {
		Type   string   `hcl:"type"`
		Labels []string `hcl:"labels"`
	} `hcl:"repository,block"`
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

	return config, nil
}

func formatDiagnostics(diags hcl.Diagnostics) string {
	var messages []string
	for _, diag := range diags {
		severityStr := ""
		switch diag.Severity {
		case hcl.DiagError:
			severityStr = "Error"
		case hcl.DiagWarning:
			severityStr = "Warning"
		default:
			severityStr = "Unknown"
		}

		messages = append(messages, fmt.Sprintf("%s: %s at %s", severityStr, diag.Summary, diag.Subject))
		if diag.Detail != "" {
			messages = append(messages, fmt.Sprintf("  Detail: %s", diag.Detail))
		}
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
