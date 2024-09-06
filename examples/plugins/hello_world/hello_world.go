//go:build plugin
// +build plugin
package main

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
)

var HelloWorld helloworldPlugin

type helloworldPlugin struct{}

func (h helloworldPlugin) Run() error {
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		PaddingTop(1).
		PaddingBottom(1).
		PaddingLeft(4).
		PaddingRight(4)

	fmt.Println(style.Render("Hello, World!"))
	return nil
}

func (h helloworldPlugin) Name() string {
	return "hello_world"
}

func (h helloworldPlugin) Version() string {
	return "1.0.0"
}
