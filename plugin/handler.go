package plugin

import (
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	pb "github.com/ssotops/gitspace-plugin-sdk/proto"
)

func HandleInstallPlugin(logger *log.Logger, manager *Manager) error {
	var installChoice string
	err := huh.NewSelect[string]().
		Title("Choose installation type").
		Options(
			huh.NewOption("Gitspace Catalog", "catalog"),
			huh.NewOption("Local", "local"),
			huh.NewOption("Remote", "remote"),
		).
		Value(&installChoice).
		Run()

	if err != nil {
		return fmt.Errorf("error getting installation type: %w", err)
	}

	var source string

	switch installChoice {
	case "catalog":
		source, err = handleGitspaceCatalogInstall(logger)
		if err != nil {
			return fmt.Errorf("error selecting from Gitspace Catalog: %w", err)
		}
	case "local":
		source, err = getPathWithCompletion("Enter the local plugin source directory")
		if err != nil {
			return fmt.Errorf("error getting local plugin path: %w", err)
		}
	case "remote":
		err = huh.NewInput().
			Title("Enter the remote plugin URL").
			Value(&source).
			Run()
		if err != nil {
			return fmt.Errorf("error getting remote plugin URL: %w", err)
		}
	}

	err = InstallPlugin(logger, manager, source)
	if err != nil {
		return fmt.Errorf("failed to install plugin: %w", err)
	}

	pluginsDir, err := getPluginsDir()
	if err != nil {
		return fmt.Errorf("failed to get plugins directory: %w", err)
	}

	// Load the manifest to get the plugin name
	manifestPath := filepath.Join(pluginsDir, filepath.Base(source), "gitspace-plugin.toml")
	manifest, err := loadPluginManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to load plugin manifest: %w", err)
	}

	pluginName := manifest.Metadata.Name
	pluginPath := filepath.Join(pluginsDir, pluginName, pluginName+".so")

	// Load the plugin
	err = manager.LoadPlugin(pluginName, pluginPath)
	if err != nil {
		return fmt.Errorf("error loading plugin: %w", err)
	}

	logger.Info("Plugin installed and loaded successfully", "name", pluginName)
	return nil
}

func HandleUninstallPlugin(logger *log.Logger, manager *Manager) error {
	plugins, err := ListInstalledPlugins(logger)
	if err != nil {
		return fmt.Errorf("failed to list installed plugins: %w", err)
	}

	if len(plugins) == 0 {
		logger.Info("No plugins installed")
		return nil
	}

	var selectedPlugin string
	err = huh.NewSelect[string]().
		Title("Select a plugin to uninstall").
		Options(createOptionsFromStrings(plugins)...).
		Value(&selectedPlugin).
		Run()

	if err != nil {
		return fmt.Errorf("error selecting plugin to uninstall: %w", err)
	}

	err = UninstallPlugin(logger, selectedPlugin)
	if err != nil {
		return fmt.Errorf("failed to uninstall plugin: %w", err)
	}

	err = manager.UnloadPlugin(selectedPlugin)
	if err != nil {
		return fmt.Errorf("error unloading plugin: %w", err)
	}

	logger.Info("Plugin uninstalled and unloaded successfully", "name", selectedPlugin)
	return nil
}

func HandleListInstalledPlugins(logger *log.Logger) error {
	plugins, err := ListInstalledPlugins(logger)
	if err != nil {
		return fmt.Errorf("failed to list installed plugins: %w", err)
	}

	if len(plugins) == 0 {
		logger.Info("No plugins installed")
	} else {
		logger.Info("Installed plugins:")
		for _, plugin := range plugins {
			logger.Info("- " + plugin)
		}
	}

	return nil
}

func HandleRunPlugin(logger *log.Logger, manager *Manager) error {
	plugins := manager.GetLoadedPlugins()
	logger.Debug("Loaded plugins", "count", len(plugins))

	if len(plugins) == 0 {
		logger.Info("No plugins loaded")
		return nil
	}

	var pluginNames []string
	for name := range plugins {
		pluginNames = append(pluginNames, name)
	}

	var selectedPlugin string
	err := huh.NewSelect[string]().
		Title("Choose a plugin to run").
		Options(createOptionsFromStrings(pluginNames)...).
		Value(&selectedPlugin).
		Run()

	if err != nil {
		return fmt.Errorf("error selecting plugin: %w", err)
	}

	logger.Debug("Selected plugin", "name", selectedPlugin)

	// Get the menu items from the selected plugin
	menuItems, err := manager.GetPluginMenu(selectedPlugin)
	if err != nil {
		logger.Error("Error getting plugin menu", "error", err)
		return fmt.Errorf("error getting plugin menu: %w", err)
	}

	logger.Debug("Got menu items", "count", len(menuItems))

	var selectedCommand string
	err = huh.NewSelect[string]().
		Title("Choose a command to run").
		Options(createOptionsFromMenuItems(menuItems)...).
		Value(&selectedCommand).
		Run()

	if err != nil {
		return fmt.Errorf("error selecting command: %w", err)
	}

	logger.Debug("Selected command", "command", selectedCommand)

	result, err := manager.ExecuteCommand(selectedPlugin, selectedCommand, nil)
	if err != nil {
		logger.Error("Error executing command", "error", err)
		return fmt.Errorf("error executing command: %w", err)
	}

	logger.Info("Command result", "result", result)
	return nil
}

func handleGitspaceCatalogInstall(logger *log.Logger) (string, error) {
	owner := "ssotops"
	repo := "gitspace-catalog"
	catalog, err := fetchGitspaceCatalog(owner, repo)
	if err != nil {
		return "", fmt.Errorf("failed to fetch Gitspace Catalog: %w", err)
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

func createOptionsFromStrings(items []string) []huh.Option[string] {
	options := make([]huh.Option[string], len(items))
	for i, item := range items {
		options[i] = huh.NewOption(item, item)
	}
	return options
}

func createOptionsFromMenuItems(items []*pb.MenuItem) []huh.Option[string] {
	options := make([]huh.Option[string], len(items))
	for i, item := range items {
		options[i] = huh.NewOption(item.Label, item.Command)
	}
	return options
}

func getPluginNames(plugins map[string]*Plugin) []string {
	names := make([]string, 0, len(plugins))
	for name := range plugins {
		names = append(names, name)
	}
	return names
}
