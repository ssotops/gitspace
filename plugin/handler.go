package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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

func HandleGitspaceCatalogInstall(logger *logger.RateLimitedLogger) (string, error) {
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

func runPluginLoop(ctx context.Context, logger *logger.RateLimitedLogger, manager *Manager, selectedPlugin string) error {
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(interruptChan)

	var currentMenu []gsplug.MenuOption
	menuStack := [][]gsplug.MenuOption{}

	for {
		menuDone := make(chan struct{})
		var selectedCommand string

		go func() {
			defer close(menuDone)

			if len(currentMenu) == 0 {
				logger.Debug("Getting menu for selected plugin", "plugin", selectedPlugin)
				menuResp, err := manager.GetPluginMenu(selectedPlugin)
				if err != nil {
					logger.Error("Error getting plugin menu", "error", err)
					return
				}

				logger.Debug("Received menu response", "dataSize", len(menuResp.MenuData))

				err = json.Unmarshal(menuResp.MenuData, &currentMenu)
				if err != nil {
					logger.Error("Error unmarshalling menu data", "error", err)
					return
				}
			}

			logger.Debug("Presenting menu options to user", "optionsCount", len(currentMenu))

			options := make([]huh.Option[string], 0, len(currentMenu)+1)
			for _, opt := range currentMenu {
				options = append(options, huh.NewOption(opt.Label, opt.Command))
			}
			if len(menuStack) > 0 {
				options = append(options, huh.NewOption("Go Back", "go_back"))
			} else {
				options = append(options, huh.NewOption("Exit plugin", "exit"))
			}

			err := huh.NewSelect[string]().
				Title("Choose an action").
				Options(options...).
				Value(&selectedCommand).
				Run()

			if err != nil {
				if err == huh.ErrUserAborted {
					logger.Debug("User aborted menu selection")
					selectedCommand = "exit"
				} else {
					logger.Error("Error running menu", "error", err)
				}
			}
		}()

		select {
		case <-ctx.Done():
			return nil
		case <-interruptChan:
			logger.Info("Received interrupt signal. Returning to previous menu...")
			return nil
		case <-menuDone:
			if selectedCommand == "exit" {
				logger.Debug("User chose to exit plugin")
				return nil
			}

			if selectedCommand == "go_back" {
				if len(menuStack) > 0 {
					currentMenu = menuStack[len(menuStack)-1]
					menuStack = menuStack[:len(menuStack)-1]
					continue
				} else {
					logger.Debug("No previous menu, exiting plugin")
					return nil
				}
			}

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

			if selectedOption != nil {
				if len(selectedOption.SubMenu) > 0 {
					menuStack = append(menuStack, currentMenu)
					currentMenu = selectedOption.SubMenu
					continue
				}

				// Execute the selected command
				result, err := manager.ExecuteCommand(selectedPlugin, selectedCommand, map[string]string{})
				if err != nil {
					logger.Error("Error executing command", "error", err)
					fmt.Printf("Error: %v\n", err)
				} else {
					logger.Info("Command executed successfully", "result", result)
					fmt.Printf("Result: %s\n", result)
				}
			} else {
				logger.Error("Selected command not found in menu options", "command", selectedCommand)
			}

			// Clear the current menu to fetch a fresh menu on the next iteration
			currentMenu = nil
		}
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
