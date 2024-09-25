package plugin

import (
	"encoding/json"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/ssotops/gitspace-plugin-sdk/gsplug"
	"github.com/ssotops/gitspace-plugin-sdk/logger"
	pb "github.com/ssotops/gitspace-plugin-sdk/proto"
	"github.com/ssotops/gitspace/lib"
)

func HandleInstallPlugin(logger *logger.RateLimitedLogger, manager *Manager) error {
	logger.Debug("Entering HandleInstallPlugin")
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
		logger.Error("Error getting installation type", "error", err)
		return fmt.Errorf("error getting installation type: %w", err)
	}

	logger.Debug("Installation type selected", "choice", installChoice)

	var source string

	switch installChoice {
	case "catalog":
		logger.Debug("Handling Gitspace Catalog installation")
		source, err = handleGitspaceCatalogInstall(logger)
		if err != nil {
			logger.Error("Error selecting from Gitspace Catalog", "error", err)
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

	logger.Debug("Proceeding with plugin installation", "source", source)
	err = InstallPlugin(logger, manager, source)
	if err != nil {
		logger.Error("Failed to install plugin", "error", err)
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

	// Load the plugin if it's not already loaded
	if !manager.IsPluginLoaded(selectedPlugin) {
		err = manager.LoadPlugin(selectedPlugin)
		if err != nil {
			logger.Error("Failed to load plugin", "name", selectedPlugin, "error", err)
			return fmt.Errorf("failed to load plugin %s: %w", selectedPlugin, err)
		}
	}

	for {
		// Get the plugin menu
		logger.Debug("Getting menu for selected plugin", "plugin", selectedPlugin)
		menuResp, err := manager.GetPluginMenu(selectedPlugin)
		if err != nil {
			logger.Error("Error getting plugin menu", "error", err)
			return fmt.Errorf("error getting plugin menu: %w", err)
		}

		logger.Debug("Received menu response", "dataSize", len(menuResp.MenuData))

		var menuOptions []gsplug.MenuOption
		err = json.Unmarshal(menuResp.MenuData, &menuOptions)
		if err != nil {
			logger.Error("Error unmarshalling menu data", "error", err)
			return fmt.Errorf("error unmarshalling menu data: %w", err)
		}

		logger.Debug("Unmarshalled menu options", "optionsCount", len(menuOptions))

		// Present menu to user
		var selectedCommand string
		err = huh.NewSelect[string]().
			Title("Choose an action").
			Options(func() []huh.Option[string] {
				options := make([]huh.Option[string], len(menuOptions)+1)
				for i, opt := range menuOptions {
					options[i] = huh.NewOption(opt.Label, opt.Command)
				}
				options[len(menuOptions)] = huh.NewOption("Exit plugin", "exit")
				return options
			}()...).
			Value(&selectedCommand).
			Run()

		if err != nil {
			logger.Error("Error running menu", "error", err)
			return fmt.Errorf("error running menu: %w", err)
		}

		if selectedCommand == "exit" {
			logger.Debug("User chose to exit plugin")
			break
		}

		logger.Debug("User selected command", "command", selectedCommand)

		// Find the selected command in the menu options
		var selectedOption *gsplug.MenuOption
		for i, opt := range menuOptions {
			if opt.Command == selectedCommand {
				selectedOption = &menuOptions[i]
				break
			}
		}

		if selectedOption == nil {
			logger.Error("Selected command not found in menu options")
			return fmt.Errorf("selected command not found in menu options")
		}

		// Collect parameters
		params := make(map[string]string)
		for _, param := range selectedOption.Parameters {
			var value string
			err := huh.NewInput().
				Title(fmt.Sprintf("%s (%s)", param.Name, param.Description)).
				Value(&value).
				Run()

			if err != nil {
				logger.Error("Error getting parameter input", "param", param.Name, "error", err)
				return fmt.Errorf("error getting parameter input for %s: %w", param.Name, err)
			}

			params[param.Name] = value
		}

		result, err := manager.ExecuteCommand(selectedPlugin, selectedCommand, params)
		if err != nil {
			logger.Error("Error executing command", "error", err)
			return fmt.Errorf("error executing command: %w", err)
		}

		logger.Info("Command result", "result", result)

		// Ask if the user wants to perform another action
		var continueChoice string
		err = huh.NewSelect[string]().
			Title("Do you want to perform another action with this plugin?").
			Options(
				huh.NewOption("Yes", "yes"),
				huh.NewOption("No, return to main menu", "no"),
			).
			Value(&continueChoice).
			Run()

		if err != nil {
			logger.Error("Error getting continue choice", "error", err)
			return fmt.Errorf("error getting continue choice: %w", err)
		}

		if continueChoice == "no" {
			logger.Debug("User chose to return to main menu")
			break
		}
	}

	return nil
}

func handleGitspaceCatalogInstall(logger *logger.RateLimitedLogger) (string, error) {
	logger.Debug("Entering handleGitspaceCatalogInstall")
	owner := "ssotops"
	repo := "gitspace-catalog"
	logger.Debug("Fetching Gitspace Catalog", "owner", owner, "repo", repo)
	catalog, err := lib.FetchGitspaceCatalog(owner, repo)
	if err != nil {
		logger.Error("Failed to fetch Gitspace Catalog", "error", err)
		return "", fmt.Errorf("failed to fetch Gitspace Catalog: %w", err)
	}

	logger.Debug("Successfully fetched Gitspace Catalog")

	var options []huh.Option[string]
	for name, plugin := range catalog.Plugins {
		options = append(options, huh.NewOption(fmt.Sprintf("%s (%s)", name, plugin.Description), name))
	}

	if len(options) == 0 {
		logger.Warn("No plugins found in the catalog")
		return "", fmt.Errorf("no plugins found in the catalog")
	}

	logger.Debug("Presenting plugin options to user", "optionCount", len(options))

	var selectedItem string
	err = huh.NewSelect[string]().
		Title("Select a plugin to install").
		Options(options...).
		Value(&selectedItem).
		Run()

	if err != nil {
		logger.Error("Failed to select item", "error", err)
		return "", fmt.Errorf("failed to select item: %w", err)
	}

	logger.Debug("User selected plugin", "selectedItem", selectedItem)

	// Construct the full GitHub URL for the selected plugin
	selectedPlugin := catalog.Plugins[selectedItem]
	pluginURL := fmt.Sprintf("https://github.com/%s/%s/tree/main/%s", owner, repo, selectedPlugin.Path)

	logger.Debug("Constructed plugin URL", "url", pluginURL)

	return pluginURL, nil
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
