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

	// Set up signal handling for graceful shutdown
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

	// Main menu loop
	for {
		select {
		case <-signalChan:
			fmt.Println("\nReceived interrupt signal. Exiting Gitspace...")
			return
		default:
			printConfigPath(config)
			if handleMainMenu(logger, &config) {
				return // Exit the program if user chose to quit
			}
		}
	}
}

func printConfigPath(config *Config) {
	if config != nil && config.Repositories != nil && config.Repositories.GitSpace != nil && config.Repositories.GitSpace.Path != "" {
		fmt.Printf("Current config path: %s\n\n", config.Repositories.GitSpace.Path)
	} else {
		fmt.Println("No config file loaded.\n")
	}
}

func initLogger() *log.Logger {
	return log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
	})
}
