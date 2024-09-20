package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/charmbracelet/log"
	gitspace_plugin "github.com/ssotops/gitspace-plugin"
)

type PluginLoader struct {
	plugins     []gitspace_plugin.GitspacePlugin
	loadingDone bool
	mu          sync.Mutex
	logger      *log.Logger
}

func NewPluginLoader(logger *log.Logger) *PluginLoader {
	return &PluginLoader{
		logger: logger,
	}
}

func (pl *PluginLoader) StartLoading() {
	go func() {
		plugins, err := loadAllPlugins(pl.logger)
		pl.mu.Lock()
		defer pl.mu.Unlock()
		if err != nil {
			pl.logger.Warn("Error loading plugins", "error", err)
		} else {
			pl.plugins = plugins
		}
		pl.loadingDone = true
	}()
}

func (pl *PluginLoader) GetPlugins() []gitspace_plugin.GitspacePlugin {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return pl.plugins
}

func (pl *PluginLoader) IsLoadingDone() bool {
	pl.mu.Lock()
	defer pl.mu.Unlock()
	return pl.loadingDone
}

func main() {
	testMode := flag.Bool("test-plugin", false, "Run in plugin test mode")
	pluginName := flag.String("plugin", "", "Name of the plugin to test")
	printDeps := flag.Bool("print-deps", false, "Print current dependencies")
	updatePlugins := flag.Bool("update-plugins", false, "Update plugin dependencies")
	flag.Parse()

	logger := initLogger()
	logger.Info("Gitspace starting up")

	if *printDeps {
		printCurrentDependencies(logger)
		return
	}

	if *updatePlugins {
		updateAllPlugins(logger)
		return
	}

	if *testMode {
		if *pluginName == "" {
			fmt.Println("Please specify a plugin name with -plugin flag")
			os.Exit(1)
		}
		testPlugin(*pluginName, logger)
		return
	}

	currentDeps, err := gitspace_plugin.GetCurrentDependencies()
	if err != nil {
		logger.Fatal("Failed to get current dependencies", "error", err)
	}

	gitspace_plugin.SetSharedDependencies(currentDeps)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	printWelcomeMessage()

	config, err := getConfigFromUser(logger)
	if err != nil {
		logger.Error("Error getting config", "error", err)
		return
	}
	logger.Debug("Config loaded successfully", "config_path", config.Global.Path)

	pluginLoader := NewPluginLoader(logger)
	pluginLoader.StartLoading()

	for {
		select {
		case <-signalChan:
			logger.Info("Received interrupt signal. Exiting Gitspace...")
			return
		default:
			printConfigPath(config)
			if handleMainMenu(logger, &config, pluginLoader) {
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
		Level:           log.InfoLevel,
	})
	logger.Debug("Logger initialized with Debug level")
	return logger
}

func printCurrentDependencies(logger *log.Logger) {
	deps, err := gitspace_plugin.GetCurrentDependencies()
	if err != nil {
		logger.Error("Error getting dependencies", "error", err)
		os.Exit(1)
	}
	for dep, version := range deps {
		fmt.Printf("%s %s\n", dep, version)
	}
}

func updateAllPlugins(logger *log.Logger) {
	logger.Info("Updating plugin dependencies...")
	plugins, err := loadAllPlugins(logger)
	if err != nil {
		logger.Error("Error loading plugins", "error", err)
		os.Exit(1)
	}

	for _, plugin := range plugins {
		err := gitspace_plugin.UpdatePluginDependencies(plugin)
		if err != nil {
			logger.Error("Error updating plugin", "plugin", plugin.Name(), "error", err)
		} else {
			logger.Info("Plugin updated successfully", "plugin", plugin.Name())
		}
	}
	logger.Info("Plugin update process completed")
}
