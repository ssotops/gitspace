package plugin

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/huh"
	pb "github.com/ssotops/gitspace-plugin-sdk/proto"
	"github.com/ssotops/gitspace/logger"
)

func HandleInstallPlugin(logger *logger.RateLimitedLogger, manager *Manager) error {
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

	logger.Info("Plugin installed successfully")
	return nil
}
func HandleUninstallPlugin(logger *logger.RateLimitedLogger, manager *Manager) error {
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
    logger.Info("Plugin installed successfully")
    return nil
}


func HandleListInstalledPlugins(logger *logger.RateLimitedLogger) error {
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

func HandleRunPlugin(logger *logger.RateLimitedLogger, manager *Manager) error {
	discoveredPlugins := manager.GetDiscoveredPlugins()
	logger.Debug("Discovered plugins", "count", len(discoveredPlugins))

	if len(discoveredPlugins) == 0 {
		logger.Info("No plugins discovered")
		return nil
	}

	var pluginNames []string
	for name := range discoveredPlugins {
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

	// Load the plugin
	err = manager.LoadPlugin(selectedPlugin)
	if err != nil {
		logger.Error("Failed to load plugin", "name", selectedPlugin, "error", err)
		return fmt.Errorf("failed to load plugin %s: %w", selectedPlugin, err)
	}

	menuResp, err := manager.GetPluginMenu(selectedPlugin)
	if err != nil {
		logger.Error("Error getting plugin menu", "error", err)
		return fmt.Errorf("error getting plugin menu: %w", err)
	}

	var menuOptions []MenuOption
	err = json.Unmarshal(menuResp.MenuData, &menuOptions)
	if err != nil {
		logger.Error("Error unmarshalling menu data", "error", err)
		return fmt.Errorf("error unmarshalling menu data: %w", err)
	}

	var selectedCommand string
	err = huh.NewSelect[string]().
		Title("Choose an action").
		Options(func() []huh.Option[string] {
			options := make([]huh.Option[string], len(menuOptions))
			for i, opt := range menuOptions {
				options[i] = huh.NewOption(opt.Label, opt.Command)
			}
			return options
		}()...).
		Value(&selectedCommand).
		Run()

	if err != nil {
		logger.Error("Error running menu", "error", err)
		return fmt.Errorf("error running menu: %w", err)
	}

	// Execute the selected command
	result, err := manager.ExecuteCommand(selectedPlugin, selectedCommand, nil)
	if err != nil {
		logger.Error("Error executing command", "error", err)
		return fmt.Errorf("error executing command: %w", err)
	}

	logger.Info("Command result", "result", result)
	return nil
}

func handleGitspaceCatalogInstall(logger *logger.RateLimitedLogger) (string, error) {
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
