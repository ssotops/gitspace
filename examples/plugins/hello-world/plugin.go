package main

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/ssotops/gitspace/gsplugin"
)

type HelloWorldPlugin struct {
	config gsplugin.PluginConfig
}

var Plugin HelloWorldPlugin

func (p HelloWorldPlugin) Name() string {
	return "hello-world"
}

func (p HelloWorldPlugin) Version() string {
	return "1.0.0"
}

func (p HelloWorldPlugin) Description() string {
	return "A simple Hello World plugin for Gitspace"
}

func (p HelloWorldPlugin) Run(logger *log.Logger) error {
	logger.Info("Hello from the Hello World plugin!")
	return nil
}

func (p HelloWorldPlugin) GetMenuOption() *huh.Option[string] {
	return &huh.Option[string]{
		Key:   "hello-world",
		Value: "Hello World",
	}
}

func (p HelloWorldPlugin) Standalone(args []string) error {
	fmt.Println("Hello from the standalone Hello World plugin!")
	return nil
}

func (p *HelloWorldPlugin) SetConfig(config gsplugin.PluginConfig) {
	p.config = config
}
