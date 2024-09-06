package main

import (
	"fmt"
)

type HelloWorldPlugin struct{}

func (h HelloWorldPlugin) Run() error {
	fmt.Println("Hello, World!")
	return nil
}

func (h HelloWorldPlugin) Name() string {
	return "hello_world"
}

func (h HelloWorldPlugin) Version() string {
	return "1.0.0"
}

// Export the plugin
var Plugin HelloWorldPlugin
