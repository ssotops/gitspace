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

	config, err := getConfigFromUser(logger)
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
			if handleMainMenu(logger, &config) {
				return // Exit the program if user chose to quit
			}
		}
	}
}

func initLogger() *log.Logger {
	return log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
		Level:           log.DebugLevel,
	})
}
