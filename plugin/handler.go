package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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
		source, err = HandleGitspaceCatalogInstall(logger)
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

	// Filter out internal directories
	var userPlugins []string
	for _, plugin := range plugins {
		if plugin != "data" {
			userPlugins = append(userPlugins, plugin)
		}
	}

	if len(userPlugins) == 0 {
		logger.Info("No plugins installed")
		return nil
	}

	var selectedPlugin string
	err = huh.NewSelect[string]().
		Title("Select a plugin to uninstall").
		Options(createOptionsFromStrings(userPlugins)...).
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

func HandleListInstalledPlugins(logger *logger.RateLimitedLogger) error {
	plugins, err := ListInstalledPlugins(logger)
	if err != nil {
		return fmt.Errorf("failed to list installed plugins: %w", err)
	}

	// Filter out internal directories
	var userPlugins []string
	for _, plugin := range plugins {
		if plugin != "data" {
			userPlugins = append(userPlugins, plugin)
		}
	}

	if len(userPlugins) == 0 {
		logger.Info("No plugins installed")
	} else {
		logger.Info("Installed plugins:")
		for _, plugin := range userPlugins {
			logger.Info("- " + plugin)
		}
	}

	return nil
}

func HandleGitspaceCatalogInstall(logger *logger.RateLimitedLogger) (string, error) {
	logger.Debug("Entering handleGitspaceCatalogInstall")
	owner := "ssotops"
	repo := "gitspace-catalog"
	logger.Debug("Fetching Gitspace Catalog", "owner", owner, "repo", repo)

	ctx := context.Background()
	catalog, err := lib.FetchGitspaceCatalog(ctx, lib.SCMTypeGitHub, "", owner, repo)
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

func HandleRunPlugin(logger *logger.RateLimitedLogger, manager *Manager) error {
	filteredPlugins := manager.GetFilteredPlugins()
	logger.Debug("Discovered plugins (filtered)", "count", len(filteredPlugins))

	if len(filteredPlugins) == 0 {
		logger.Info("No plugins discovered")
		return nil
	}

	var pluginNames []string
	for name := range filteredPlugins {
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

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run the plugin loop
	return runPluginLoop(ctx, logger, manager, selectedPlugin)
}

// plugin_handler.go

// plugin/handler.go

func runPluginLoop(ctx context.Context, logger *logger.RateLimitedLogger, manager *Manager, selectedPlugin string) error {
	// Set up interrupt handling
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interruptChan)

	plugin, ok := manager.plugins[selectedPlugin]
	if !ok {
		return fmt.Errorf("plugin not found: %s", selectedPlugin)
	}

	pluginLogger := plugin.Logger
	var currentMenu []gsplug.MenuOption
	menuStack := [][]gsplug.MenuOption{}

	// Helper function to refresh menu
	refreshMenu := func() error {
		menuResp, err := manager.GetPluginMenu(selectedPlugin)
		if err != nil {
			return fmt.Errorf("failed to get menu: %w", err)
		}

		var newMenu []gsplug.MenuOption
		err = json.Unmarshal(menuResp.MenuData, &newMenu)
		if err != nil {
			return fmt.Errorf("failed to unmarshal menu data: %w", err)
		}

		currentMenu = newMenu
		return nil
	}

	// Initial menu load
	if err := refreshMenu(); err != nil {
		return fmt.Errorf("failed to load initial menu: %w", err)
	}

	for {
		// Check if plugin is still running
		if !manager.IsPluginRunning(selectedPlugin) {
			pluginLogger.Error("Plugin has terminated unexpectedly")
			return fmt.Errorf("plugin terminated unexpectedly")
		}

		// Create menu options including navigation
		options := make([]huh.Option[string], 0, len(currentMenu)+1)
		for _, opt := range currentMenu {
			options = append(options, huh.NewOption(opt.Label, opt.Command))
		}

		// Add navigation options
		if len(menuStack) > 0 {
			options = append(options, huh.NewOption("Go Back", "go_back"))
		}
		options = append(options, huh.NewOption("Exit Plugin", "exit"))

		// Present menu to user
		var selectedCommand string
		err := huh.NewSelect[string]().
			Title("Choose an action").
			Options(options...).
			Value(&selectedCommand).
			Run()

		if err != nil {
			if err == huh.ErrUserAborted {
				return nil
			}
			pluginLogger.Error("Menu selection error", "error", err)
			continue
		}

		// Handle navigation commands
		switch selectedCommand {
		case "exit":
			return nil
		case "go_back":
			if len(menuStack) > 0 {
				currentMenu = menuStack[len(menuStack)-1]
				menuStack = menuStack[:len(menuStack)-1]
				continue
			}
			return nil
		}

		// Find selected menu option
		var selectedOption *gsplug.MenuOption
		var findOption func([]gsplug.MenuOption, string) *gsplug.MenuOption
		findOption = func(menu []gsplug.MenuOption, cmd string) *gsplug.MenuOption {
			for i, opt := range menu {
				if opt.Command == cmd {
					return &menu[i]
				}
				if len(opt.SubMenu) > 0 {
					if subOpt := findOption(opt.SubMenu, cmd); subOpt != nil {
						return subOpt
					}
				}
			}
			return nil
		}

		selectedOption = findOption(currentMenu, selectedCommand)
		if selectedOption == nil {
			pluginLogger.Error("Invalid command selected", "command", selectedCommand)
			continue
		}

		// Handle submenu navigation
		if len(selectedOption.SubMenu) > 0 {
			menuStack = append(menuStack, currentMenu)
			currentMenu = selectedOption.SubMenu
			continue
		}
		// Collect parameters
		params := make(map[string]string)
		for _, param := range selectedOption.Parameters {
			var value string
			prompt := fmt.Sprintf("%s (%s)", param.Name, param.Description)
			if param.Required {
				prompt += " (Required)"
			}

			err := huh.NewInput().
				Title(prompt).
				Value(&value).
				Validate(func(s string) error {
					if param.Required && s == "" {
						return fmt.Errorf("this field is required")
					}
					return nil
				}).
				Run()

			if err != nil {
				if err == huh.ErrUserAborted {
					pluginLogger.Info("User aborted parameter input")
					break
				}
				pluginLogger.Error("Parameter input error", "error", err)
				continue
			}

			if value != "" {
				params[param.Name] = value
			}
		}

		// Execute command
		result, err := manager.ExecuteCommand(selectedPlugin, selectedCommand, params)
		if err != nil {
			pluginLogger.Error("Command execution failed", "error", err)
			fmt.Printf("Error: %v\n", err)

			// Check for plugin termination
			if !manager.IsPluginRunning(selectedPlugin) {
				return fmt.Errorf("plugin terminated during command execution")
			}
		} else {
			if result != "" {
				pluginLogger.Info("Command executed successfully", "result", result)
				fmt.Printf("Result: %s\n", result)
			} else {
				pluginLogger.Info("Command executed successfully")
			}
		}

		// Refresh menu after command execution
		select {
		case <-ctx.Done():
			return nil
		case <-interruptChan:
			pluginLogger.Info("Received interrupt signal")
			return nil
		default:
			if err := refreshMenu(); err != nil {
				pluginLogger.Error("Failed to refresh menu", "error", err)
				return fmt.Errorf("failed to refresh menu after command: %w", err)
			}
		}

		// Brief pause to allow user to read any command output
		time.Sleep(500 * time.Millisecond)
	}
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

func filterPlugins(plugins map[string]string) map[string]string {
	filtered := make(map[string]string)
	for name, path := range plugins {
		if name != "data" {
			filtered[name] = path
		}
	}
	return filtered
}

func executePluginCommand(logger *logger.RateLimitedLogger, manager *Manager, selectedPlugin, selectedCommand string, parameters []gsplug.ParameterInfo) error {
	params := make(map[string]string)
	for _, param := range parameters {
		var value string
		prompt := fmt.Sprintf("%s (%s): ", param.Name, param.Description)
		if param.Required {
			prompt = fmt.Sprintf("%s (Required) ", prompt)
		}
		err := huh.NewInput().
			Title(prompt).
			Value(&value).
			Validate(func(s string) error {
				if param.Required && s == "" {
					return fmt.Errorf("this field is required")
				}
				return nil
			}).
			Run()
		if err != nil {
			return fmt.Errorf("error getting parameter input: %w", err)
		}
		if value != "" {
			params[param.Name] = value
		}
	}

	result, err := manager.ExecuteCommand(selectedPlugin, selectedCommand, params)
	if err != nil {
		return fmt.Errorf("error executing command: %w", err)
	}

	logger.Info("Command result", "result", result)
	fmt.Printf("Result: %s\n", result)
	return nil
}

func getFilteredPluginList(plugins []string) []string {
	var filtered []string
	for _, plugin := range plugins {
		if plugin != "data" {
			filtered = append(filtered, plugin)
		}
	}
	return filtered
}

func createOptionsFromPlugins(plugins []string) []huh.Option[string] {
	filtered := getFilteredPluginList(plugins)
	options := make([]huh.Option[string], len(filtered))
	for i, plugin := range filtered {
		options[i] = huh.NewOption(plugin, plugin)
	}
	return options
}

func refreshMenu(manager *Manager, selectedPlugin string) ([]gsplug.MenuOption, error) {
	menuResp, err := manager.GetPluginMenu(selectedPlugin)
	if err != nil {
		return nil, fmt.Errorf("failed to get menu: %w", err)
	}

	var menu []gsplug.MenuOption
	err = json.Unmarshal(menuResp.MenuData, &menu)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal menu data: %w", err)
	}

	return menu, nil
}
