package gsplugin

import (
    "github.com/charmbracelet/huh"
    "github.com/charmbracelet/log"
)

type GitspacePlugin interface {
	Name() string
	Version() string
	Description() string
	Run(logger *log.Logger) error
	GetMenuOption() *huh.Option[string]
	Standalone(args []string) error
	SetConfig(config PluginConfig)
}

// PluginMetadata contains additional information about the plugin
type PluginMetadata struct {
	Name        string   `toml:"name"`
	Version     string   `toml:"version"`
	Description string   `toml:"description"`
	Author      string   `toml:"author"`
	Tags        []string `toml:"tags"`
}

// PluginConfig contains the configuration for the plugin
type PluginConfig struct {
	Metadata PluginMetadata `toml:"metadata"`
	Menu     struct {
		Title string `toml:"title"`
		Key   string `toml:"key"`
	} `toml:"menu"`
}

type PluginManifest struct {
	Plugin struct {
		Name        string `toml:"name"`
		Version     string `toml:"version"`
		Description string `toml:"description,omitempty"`
		Author      string `toml:"author,omitempty"`
		Sources     []struct {
			Path       string `toml:"path"`
			EntryPoint string `toml:"entry_point"`
			Repository struct {
				Type   string `toml:"type,omitempty"`
				URL    string `toml:"url,omitempty"`
				Branch string `toml:"branch,omitempty"`
			} `toml:"repository,omitempty"`
		} `toml:"sources"`
	} `toml:"plugin"`
}
