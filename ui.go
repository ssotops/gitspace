package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

func printWelcomeMessage() {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFD700")).
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(1)

	subtitleStyle := lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#87CEEB"))

	version, _ := getCurrentVersion()

	fmt.Println(titleStyle.Render("Welcome to Gitspace!"))
	fmt.Println(subtitleStyle.Render(fmt.Sprintf("Current version: %s", version)))
	fmt.Println()
}

func showMainMenu() string {
	var choice string
	err := huh.NewSelect[string]().
		Title("Choose an action").
		Options(
			huh.NewOption("Repositories", "repositories"),
			huh.NewOption("Symlinks", "symlinks"),
			huh.NewOption("Sync Labels", "sync"),
			huh.NewOption("Plugins", "plugins"), // New option
			huh.NewOption("Gitspace", "gitspace"),
			huh.NewOption("Quit", "quit"),
		).
		Value(&choice).
		Run()

	if err != nil {
		if err == huh.ErrUserAborted {
			return "" // Return empty string on CTRL+C
		}
		fmt.Println("Error getting user choice:", err)
		return ""
	}

	return choice
}

func handleMainMenu(logger *log.Logger, config **Config) bool {
	choice := showMainMenu()

	switch choice {
	case "repositories":
		return handleRepositoriesCommand(logger, *config)
	case "sync":
		syncLabels(logger, *config)
	case "gitspace":
		handleGitspaceCommand(logger, *config)
	case "symlinks":
		handleSymlinksCommand(logger, *config)
	case "plugins": // New case
		handlePluginsCommand(logger, *config)
	case "quit":
		fmt.Println("Exiting Gitspace. Goodbye!")
		return true
	case "":
		// User likely pressed CTRL+C, exit gracefully
		fmt.Println("\nExiting Gitspace. Goodbye!")
		return true
	default:
		logger.Error("Invalid choice")
	}

	return false
}

func handleRepositoriesCommand(logger *log.Logger, config *Config) bool {
	if !ensureConfig(logger, &config) {
		return false
	}
	for {
		var subChoice string
		err := huh.NewSelect[string]().
			Title("Choose a repositories action").
			Options(
				huh.NewOption("Clone", "clone"),
				huh.NewOption("Sync", "sync"),
				huh.NewOption("Go back", "back"),
				huh.NewOption("Quit", "quit"),
			).
			Value(&subChoice).
			Run()

		if err != nil {
			logger.Error("Error getting repositories sub-choice", "error", err)
			return false
		}

		switch subChoice {
		case "clone":
			cloneRepositories(logger, config)
		case "sync":
			syncRepositories(logger, config)
		case "back":
			return false // Go back to main menu
		case "quit":
			return true // Exit the program
		default:
			logger.Error("Invalid repositories sub-choice")
		}
	}
}

func printSymlinkSummary(title string, changes map[string]string) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FFFF"))
	symlinkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00FF"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00"))

	fmt.Println(titleStyle.Render(fmt.Sprintf("\n%s Summary:", title)))
	if len(changes) == 0 {
		fmt.Println("No changes were made.")
	} else {
		for symlink, target := range changes {
			fmt.Printf("  %s -> %s\n", symlinkStyle.Render(symlink), pathStyle.Render(target))
		}
	}
	fmt.Printf("\nTotal changes: %d\n", len(changes))
}

func printSummaryTable(config *Config, results map[string]*RepoResult, repoDir string) {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	repoNameStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))

	fmt.Println(headerStyle.Render("\nRepository Processing Summary:"))
	fmt.Println()

	for _, result := range results {
		fmt.Println(repoNameStyle.Render(result.Name))
		fmt.Println()

		status := "No changes"
		statusEmoji := "✅"
		if result.Error != nil {
			status = "Failed"
			statusEmoji = "❌"
		} else if result.Cloned {
			status = "Cloned"
		} else if result.Updated {
			status = "Updated"
		}

		fmt.Println(infoStyle.Render(fmt.Sprintf("%s Status: %s", statusEmoji, status)))
		fmt.Println(infoStyle.Render(fmt.Sprintf("🔗 Local Symlink: %s", result.LocalSymlink)))
		fmt.Println(infoStyle.Render(fmt.Sprintf("🌐 Global Symlink: %s", result.GlobalSymlink)))

		if result.Error != nil {
			fmt.Println(infoStyle.Render(fmt.Sprintf("❌ Error: %s", result.Error)))
		}

		fmt.Println() // Add an empty line between repositories
	}

	fmt.Println(headerStyle.Render("Summary of changes:"))
	fmt.Println()

	totalRepos := len(results)
	clonedRepos := 0
	updatedRepos := 0
	failedRepos := 0
	localSymlinks := 0
	globalSymlinks := 0

	for _, result := range results {
		if result.Error != nil {
			failedRepos++
		} else if result.Cloned {
			clonedRepos++
		} else if result.Updated {
			updatedRepos++
		}
		if result.LocalSymlink != "" {
			localSymlinks++
		}
		if result.GlobalSymlink != "" {
			globalSymlinks++
		}
	}

	summaryStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	fmt.Println(summaryStyle.Render(fmt.Sprintf("  Total repositories processed: %d", totalRepos)))
	fmt.Println(summaryStyle.Render(fmt.Sprintf("  Newly cloned: %d", clonedRepos)))
	fmt.Println(summaryStyle.Render(fmt.Sprintf("  Updated: %d", updatedRepos)))
	fmt.Println(summaryStyle.Render(fmt.Sprintf("  Failed operations: %d", failedRepos)))
	fmt.Println(summaryStyle.Render(fmt.Sprintf("  Local symlinks created: %d", localSymlinks)))
	fmt.Println(summaryStyle.Render(fmt.Sprintf("  Global symlinks created: %d", globalSymlinks)))
}

func handleConfigPathsCommand(logger *log.Logger) {
	cacheDir, err := getCacheDir()
	if err != nil {
		logger.Error("Error getting cache directory", "error", err)
		return
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FFFF"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00"))
	symlinkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00FF"))

	fmt.Println(titleStyle.Render("\n📂 Cache Directory:"))
	fmt.Printf("   %s\n\n", pathStyle.Render(fmt.Sprintf("cd %s", cacheDir)))

	fmt.Println(titleStyle.Render("📄 Gitspace Config Files:"))
	err = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".hcl" {
			fmt.Printf("   %s\n", pathStyle.Render(path))
		}
		return nil
	})
	if err != nil {
		logger.Error("Error walking through cache directory", "error", err)
	}

	fmt.Println(titleStyle.Render("\n🔗 Gitspace Config Symlinks:"))
	err = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			realPath, err := os.Readlink(path)
			if err != nil {
				logger.Error("Error reading symlink", "path", path, "error", err)
				return nil
			}
			if filepath.Ext(realPath) == ".hcl" {
				fmt.Printf("   %s -> %s\n", symlinkStyle.Render(path), pathStyle.Render(realPath))
			}
		}
		return nil
	})
	if err != nil {
		logger.Error("Error walking through cache directory for symlinks", "error", err)
	}

	fmt.Println() // Add an extra newline for spacing
}

// Add this function to ui.go

func handleConfigCommand(logger *log.Logger) {
	cacheDir, err := getCacheDir()
	if err != nil {
		logger.Error("Error getting cache directory", "error", err)
		return
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FFFF"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00"))
	symlinkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00FF"))

	fmt.Println(titleStyle.Render("\n📂 Cache Directory:"))
	fmt.Printf("   %s\n\n", pathStyle.Render(fmt.Sprintf("cd %s", cacheDir)))

	fmt.Println(titleStyle.Render("📄 Gitspace Config Files:"))
	err = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".hcl" {
			fmt.Printf("   %s\n", pathStyle.Render(path))
		}
		return nil
	})
	if err != nil {
		logger.Error("Error walking through cache directory", "error", err)
	}

	fmt.Println(titleStyle.Render("\n🔗 Gitspace Config Symlinks:"))
	err = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			realPath, err := os.Readlink(path)
			if err != nil {
				logger.Error("Error reading symlink", "path", path, "error", err)
				return nil
			}
			if filepath.Ext(realPath) == ".hcl" {
				fmt.Printf("   %s -> %s\n", symlinkStyle.Render(path), pathStyle.Render(realPath))
			}
		}
		return nil
	})
	if err != nil {
		logger.Error("Error walking through cache directory for symlinks", "error", err)
	}

	fmt.Println() // Add an extra newline for spacing
}

func handleGitspaceCommand(logger *log.Logger, config *Config) {
	for {
		var choice string
		err := huh.NewSelect[string]().
			Title("Choose a Gitspace action").
			Options(
				huh.NewOption("Upgrade Gitspace", "upgrade"),
				huh.NewOption("Print Config Paths", "config_paths"),
				huh.NewOption("Print Version Info", "version_info"),
				huh.NewOption("Load Config", "load_config"),
				huh.NewOption("Go back", "back"),
			).
			Value(&choice).
			Run()

		if err != nil {
			logger.Error("Error getting Gitspace sub-choice", "error", err)
			return
		}

		switch choice {
		case "upgrade":
			upgradeGitspace(logger)
		case "config_paths":
			handleConfigPathsCommand(logger)
		case "version_info":
			printVersionInfo(logger)
		case "load_config":
			newConfig, err := getConfigFromUser(logger)
			if err != nil {
				logger.Error("Error loading config", "error", err)
			} else {
				config = newConfig
				if newConfig != nil && newConfig.Repositories != nil && newConfig.Repositories.GitSpace != nil {
					logger.Info("Config loaded successfully", "path", newConfig.Repositories.GitSpace.Path)
				} else {
					logger.Info("No config file loaded")
				}
			}
		case "back":
			return
		default:
			logger.Error("Invalid Gitspace sub-choice")
		}
	}
}

func handleSymlinksCommand(logger *log.Logger, config *Config) {
	if !ensureConfig(logger, &config) {
		return
	}
	for {
		var choice string
		err := huh.NewSelect[string]().
			Title("Choose a symlinks action").
			Options(
				huh.NewOption("Create local symlinks", "create_local"),
				huh.NewOption("Create global symlinks", "create_global"),
				huh.NewOption("Delete local symlinks", "delete_local"),
				huh.NewOption("Delete global symlinks", "delete_global"),
				huh.NewOption("Go back", "back"),
			).
			Value(&choice).
			Run()

		if err != nil {
			logger.Error("Error getting symlinks sub-choice", "error", err)
			return
		}

		switch choice {
		case "create_local":
			createLocalSymlinks(logger, config)
		case "create_global":
			createGlobalSymlinks(logger, config)
		case "delete_local":
			deleteLocalSymlinks(logger, config)
		case "delete_global":
			deleteGlobalSymlinks(logger, config)
		case "back":
			return
		default:
			logger.Error("Invalid symlinks sub-choice")
		}
	}
}

// ensureConfig checks if a valid config is loaded and prompts the user if it's not.
// It returns true if a valid config is available or newly loaded, false otherwise.
func ensureConfig(logger *log.Logger, config **Config) bool {
	if *config == nil || (*config).Repositories == nil {
		logger.Warn("No valid config loaded")
		var choice string
		err := huh.NewSelect[string]().
			Title("A config file is required for this operation. What would you like to do?").
			Options(
				huh.NewOption("Specify a config file", "specify"),
				huh.NewOption("Go back to main menu", "back"),
				huh.NewOption("Exit", "exit"),
			).
			Value(&choice).
			Run()

		if err != nil {
			logger.Error("Error getting user choice", "error", err)
			return false
		}

		switch choice {
		case "specify":
			newConfig, err := getConfigFromUser(logger)
			if err != nil {
				logger.Error("Error loading config", "error", err)
				return false
			}
			*config = newConfig
			return newConfig != nil
		case "back":
			return false
		case "exit":
			fmt.Println("Exiting Gitspace. Goodbye!")
			os.Exit(0)
		}
		return false
	}
	return true
}

func handlePluginsCommand(logger *log.Logger, config *Config) {
	for {
		var subChoice string
		err := huh.NewSelect[string]().
			Title("Choose a plugins action").
			Options(
				huh.NewOption("Install Plugin", "install"),
				huh.NewOption("Uninstall Plugin", "uninstall"),
				huh.NewOption("Print Installed Plugins", "print"),
				huh.NewOption("Run Plugin", "run"),
				huh.NewOption("Go back", "back"),
			).
			Value(&subChoice).
			Run()

		if err != nil {
			logger.Error("Error getting plugins sub-choice", "error", err)
			return
		}

		switch subChoice {
		case "install":
			var source string
			err := huh.NewInput().
				Title("Enter the plugin source (URL or file path)").
				Value(&source).
				Run()

			if err != nil {
				logger.Error("Error getting plugin source", "error", err)
				continue
			}

			err = installPlugin(logger, source)
			if err != nil {
				logger.Error("Failed to install plugin", "error", err)
			}
		case "uninstall":
			var name string
			err := huh.NewInput().
				Title("Enter the plugin name to uninstall").
				Value(&name).
				Run()

			if err != nil {
				logger.Error("Error getting plugin name", "error", err)
				continue
			}

			err = uninstallPlugin(logger, name)
			if err != nil {
				logger.Error("Failed to uninstall plugin", "error", err)
			}
		case "print":
			err := printInstalledPlugins(logger)
			if err != nil {
				logger.Error("Failed to print installed plugins", "error", err)
			}
		case "run":
			var name string
			err := huh.NewInput().
				Title("Enter the plugin name to run").
				Value(&name).
				Run()

			if err != nil {
				logger.Error("Error getting plugin name", "error", err)
				continue
			}

			pluginsDir, err := getPluginsDir()
			if err != nil {
				logger.Error("Failed to get plugins directory", "error", err)
				continue
			}

			pluginPath := filepath.Join(pluginsDir, name)
			plugin, err := loadPlugin(pluginPath)
			if err != nil {
				logger.Error("Failed to load plugin", "error", err)
				continue
			}

			err = plugin.Run()
			if err != nil {
				logger.Error("Failed to run plugin", "error", err)
			}
		case "back":
			return // Go back to main menu
		default:
			logger.Error("Invalid plugins sub-choice")
		}
	}
}
