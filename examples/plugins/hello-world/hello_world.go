package main

import (
	"fmt"
)

type HelloWorldPlugin struct{}

func (p *HelloWorldPlugin) Run() error {
	fmt.Println("Hello, World!")
	return nil
}

func (p *HelloWorldPlugin) Name() string {
	return "hello_world"
}

func (p *HelloWorldPlugin) Version() string {
	return "1.0.0"
}

// This is the symbol that will be looked up by the plugin system
var Plugin HelloWorldPlugin
