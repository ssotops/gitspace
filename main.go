package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
	"github.com/ssotops/gitspace/plugin"
)

func main() {
	logger := initLogger()
	logger.Info("Gitspace starting up")

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	printWelcomeMessage()

	logger.Debug("About to get config from user")
	config, err := getConfigFromUser(logger)
	if err != nil {
		logger.Error("Error getting config", "error", err)
		return
	}
	logger.Debug("Config loaded successfully", "config_path", config.Global.Path)

	logger.Debug("About to ensure plugin directory permissions")
	if err := plugin.EnsurePluginDirectoryPermissions(logger); err != nil {
		logger.Error("Failed to set plugin directory permissions", "error", err)
		logger.Warn("Plugins cannot be loaded due to permission issues in the ~/.ssot/gitspace/plugins directory. Plugin functionality will be limited.")
	}

	logger.Debug("Initializing plugin manager")
	pluginManager := plugin.NewManager()
	err = pluginManager.LoadAllPlugins(logger)
	if err != nil {
		logger.Error("Failed to load plugins", "error", err)
	}

	logger.Debug("Entering main loop")
	for {
		select {
		case <-signalChan:
			logger.Info("Received interrupt signal. Exiting Gitspace...")
			return
		default:
			logger.Debug("Printing config path")
			printConfigPath(config)
			logger.Debug("Handling main menu")
			if handleMainMenu(logger, &config, pluginManager) {
				logger.Info("User chose to quit. Exiting Gitspace...")
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

func initLogger() *log.Logger {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
	})
	logger.Debug("Logger initialized with Debug level")
	return logger
}
