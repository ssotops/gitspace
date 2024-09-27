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

	// Set log level to Debug for detailed logging
	mainLogger.SetLogLevel(log.DebugLevel)

	// Create a slice to store all loggers
	var allLoggers []*logger.RateLimitedLogger
	allLoggers = append(allLoggers, mainLogger)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	printWelcomeMessage()

	config, err := getConfigFromUser(mainLogger)
	if err != nil {
		mainLogger.Error("Error getting config", "error", err)
		return
	}
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
}

func printConfigPath(config *Config) {
	if config != nil && config.Global.Path != "" {
		fmt.Printf("Current config path: %s\n\n", config.Global.Path)
	} else {
		fmt.Println("No config file loaded.")
	}
}
