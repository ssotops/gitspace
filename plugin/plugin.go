package plugin

import (
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
)

// GitspacePlugin defines the interface that all plugins must implement
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
