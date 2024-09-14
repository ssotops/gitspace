package main

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
)

type HelloWorldPlugin struct{}

func (p *HelloWorldPlugin) Run(logger *log.Logger) error {
	logger.Info("Running Hello World plugin")
	fmt.Println("Hello, World!")
	return nil
}

func (p *HelloWorldPlugin) Name() string {
	return "hello_world"
}

func (p *HelloWorldPlugin) Version() string {
	return "1.0.0"
}

func (p *HelloWorldPlugin) GetMenuOption() *huh.Option[string] {
	return &huh.Option[string]{
		Key:   "hello_world",
		Value: "Hello World",
	}
}

// This is the symbol that will be looked up by the plugin system
var Plugin HelloWorldPlugin
