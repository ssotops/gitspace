package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/log"
)

func main() {
	logger := initLogger()
	logger.Info("Gitspace starting up")

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	printWelcomeMessage()

	var config *Config
	var err error

	config, err = getConfigFromUser(logger)
	if err != nil {
		logger.Error("Error getting config", "error", err)
		return
	}
	logger.Debug("Config loaded successfully", "config_path", config.Global.Path)

	for {
		select {
		case <-signalChan:
			logger.Info("Received interrupt signal. Exiting Gitspace...")
			return
		default:
			printConfigPath(config)
			if handleMainMenu(logger, &config) {
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
