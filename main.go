package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/ssotops/gitspace-plugin-sdk/logger"
	"github.com/ssotops/gitspace/plugin"
)

func main() {
	mainLogger, err := logger.NewRateLimitedLogger("gitspace")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create logger: %v\n", err)
		os.Exit(1)
	}

	mainLogger.Info("Gitspace starting up")
	mainLogger.SetLogLevel(log.DebugLevel)

	var allLoggers []*logger.RateLimitedLogger
	allLoggers = append(allLoggers, mainLogger)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	printWelcomeMessage()

	// Initialize variables to track configuration state
	var config *Config

	// Try to load existing config
	currentPath, err := getCurrentConfigPath(mainLogger)
	if err != nil {
		mainLogger.Warn("Error checking for existing config", "error", err)
		// Continue to prompt user
	}

	if currentPath == "" {
		mainLogger.Debug("No valid config found, prompting user")
		config, err = getConfigFromUser(mainLogger)
		if err != nil {
			mainLogger.Error("Error getting config from user", "error", err)
			return
		}
	} else {
		mainLogger.Debug("Found existing config", "path", currentPath)
		config, err = loadConfig(currentPath)
		if err != nil {
			mainLogger.Warn("Failed to load existing config, prompting user", "error", err)
			config, err = getConfigFromUser(mainLogger)
			if err != nil {
				mainLogger.Error("Error getting config from user", "error", err)
				return
			}
		} else {
			mainLogger.Info("Successfully loaded existing config", "path", currentPath)
		}
	}

	// Only proceed with plugin initialization if we have a valid config
	if config != nil {
		mainLogger.Debug("Config loaded successfully", "config_path", config.Global.Path)

		// Initialize the plugin manager
		pluginManager := plugin.NewManager(mainLogger)
		err = pluginManager.DiscoverPlugins()
		if err != nil {
			mainLogger.Error("Failed to discover plugins", "error", err)
		}

		// Set up a deferred function to print the log summary
		defer func() {
			// Add plugin loggers to allLoggers
			for _, p := range pluginManager.GetLoadedPlugins() {
				allLoggers = append(allLoggers, p.Logger)
			}
			logger.PrintLogSummary(allLoggers)
		}()

		// Main event loop
		for {
			select {
			case <-signalChan:
				mainLogger.Info("Received interrupt signal. Exiting Gitspace...")
				return
			default:
				printConfigPath(config)
				if handleMainMenu(mainLogger, &config, pluginManager) {
					mainLogger.Info("User chose to quit. Exiting Gitspace...")
					return
				}
			}
		}
	} else {
		// If we have no config, still allow access to limited functionality
		pluginManager := plugin.NewManager(mainLogger)
		defer func() {
			for _, p := range pluginManager.GetLoadedPlugins() {
				allLoggers = append(allLoggers, p.Logger)
			}
			logger.PrintLogSummary(allLoggers)
		}()

		for {
			select {
			case <-signalChan:
				mainLogger.Info("Received interrupt signal. Exiting Gitspace...")
				return
			default:
				printConfigPath(config)
				if handleMainMenu(mainLogger, &config, pluginManager) {
					mainLogger.Info("User chose to quit. Exiting Gitspace...")
					return
				}
				// If a config was loaded during the menu interaction, break the inner loop
				// to reinitialize with the new config
				if config != nil {
					break
				}
			}
			// If we got a config, break the outer loop to restart with full functionality
			if config != nil {
				main()
				return
			}
		}
	}
}

func printConfigPath(config *Config) {
	if config != nil && config.Global.Path != "" {
		fmt.Printf("Current config path: %s\n\n", config.Global.Path)
	} else {
		fmt.Println("No config file loaded.")
	}
}
