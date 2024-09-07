package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/ssotspace/gitspace/lib"
)

type TemplatePlugin struct{}

type TemplateConfig struct {
	Template struct {
		Name        string   `hcl:"name,label"`
		Version     string   `hcl:"version"`
		Description string   `hcl:"description"`
		Author      string   `hcl:"author"`
		Files       []string `hcl:"files,optional"`
		Tokens      []struct {
			Name     string   `hcl:"name"`
			Files    []string `hcl:"files"`
			Encoding string   `hcl:"encoding"`
			Phase    string   `hcl:"phase"`
		} `hcl:"tokens,block"`
		ChildTemplates []string `hcl:"child_templates,optional"`
		Parent         string   `hcl:"parent,optional"`
	} `hcl:"template,block"`
}

func (t TemplatePlugin) Run() error {
	// Get the template path from the user
	templatePath, err := lib.GetPathWithCompletion("Enter the path to the template file (default: gs.template.hcl): ")
	if err != nil {
		return fmt.Errorf("error getting template path: %w", err)
	}
	if templatePath == "" {
		templatePath = "gs.template.hcl"
	}

	// Parse the template
	config, err := parseTemplate(templatePath)
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}

	// Print summary of the template
	printTemplateSummary(config)

	// If it's a parent template, parse and print summaries of child templates
	if len(config.Template.ChildTemplates) > 0 {
		for _, childName := range config.Template.ChildTemplates {
			childPath := filepath.Join(filepath.Dir(templatePath), childName, "template.child."+config.Template.Name+"."+childName+".gs.hcl")
			childConfig, err := parseTemplate(childPath)
			if err != nil {
				fmt.Printf("Error parsing child template %s: %v\n", childName, err)
				continue
			}
			fmt.Printf("\nChild Template: %s\n", childName)
			printTemplateSummary(childConfig)
		}
	}

	return nil
}

func (t TemplatePlugin) Name() string {
	return "template"
}

func (t TemplatePlugin) Version() string {
	return "1.0.0"
}

func parseTemplate(path string) (*TemplateConfig, error) {
	var config TemplateConfig
	err := hclsimple.DecodeFile(path, nil, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to decode HCL: %w", err)
	}
	return &config, nil
}

func printTemplateSummary(config *TemplateConfig) {
	fmt.Printf("Template Name: %s\n", config.Template.Name)
	fmt.Printf("Version: %s\n", config.Template.Version)
	fmt.Printf("Description: %s\n", config.Template.Description)
	fmt.Printf("Author: %s\n", config.Template.Author)
	if config.Template.Parent != "" {
		fmt.Printf("Parent: %s\n", config.Template.Parent)
	}
	fmt.Printf("Files: %v\n", config.Template.Files)
	fmt.Printf("Tokens:\n")
	for _, token := range config.Template.Tokens {
		fmt.Printf("  - Name: %s\n", token.Name)
		fmt.Printf("    Files: %v\n", token.Files)
		fmt.Printf("    Encoding: %s\n", token.Encoding)
		fmt.Printf("    Phase: %s\n", token.Phase)
	}
	if len(config.Template.ChildTemplates) > 0 {
		fmt.Printf("Child Templates: %v\n", config.Template.ChildTemplates)
	}
}

// Export the plugin
var Plugin TemplatePlugin
