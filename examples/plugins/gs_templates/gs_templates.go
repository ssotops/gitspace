// gs_templates.go
package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

type TemplatePlugin struct{}

func (t TemplatePlugin) Run() error {
	for {
		var choice string
		err := huh.NewSelect[string]().
			Title("Template Plugin Menu").
			Options(
				huh.NewOption("Generate New Template", "generate"),
				huh.NewOption("Install Templates", "install"),
				huh.NewOption("Print Installed Templates Summary", "summary"),
				huh.NewOption("Exit", "exit"),
			).
			Value(&choice).
			Run()

		if err != nil {
			return fmt.Errorf("error getting user choice: %w", err)
		}

		switch choice {
		case "generate":
			err = generateNewTemplate()
		case "install":
			err = installTemplates()
		case "summary":
			err = printInstalledTemplatesSummary()
		case "exit":
			return nil
		}

		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}

func generateNewTemplate() error {
	var templateType string
	err := huh.NewSelect[string]().
		Title("Select template type").
		Options(
			huh.NewOption("Child", "child"),
			huh.NewOption("Parent", "parent"),
		).
		Value(&templateType).
		Run()

	if err != nil {
		return fmt.Errorf("error getting template type: %w", err)
	}

	var name, version, description, author, entryPoint, sourceType, repository, branch string

	err = huh.NewForm().
		Title("Template Information").
		Field(huh.NewInput().Title("Name").Value(&name)).
		Field(huh.NewInput().Title("Version").Value(&version)).
		Field(huh.NewInput().Title("Description").Value(&description)).
		Field(huh.NewInput().Title("Author").Value(&author)).
		Field(huh.NewInput().Title("Entry Point").Value(&entryPoint)).
		Field(huh.NewInput().Title("Source Type").Value(&sourceType)).
		Field(huh.NewInput().Title("Repository").Value(&repository)).
		Field(huh.NewInput().Title("Branch").Value(&branch)).
		Run()

	if err != nil {
		return fmt.Errorf("error getting template information: %w", err)
	}

	// Generate HCL file
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()
	pluginBlock := rootBody.AppendNewBlock("plugin", []string{name})
	pluginBody := pluginBlock.Body()

	pluginBody.SetAttributeValue("version", cty.StringVal(version))
	pluginBody.SetAttributeValue("description", cty.StringVal(description))
	pluginBody.SetAttributeValue("author", cty.StringVal(author))
	pluginBody.SetAttributeValue("entry_point", cty.StringVal(entryPoint))

	sourceBlock := pluginBody.AppendNewBlock("source", nil)
	sourceBody := sourceBlock.Body()
	sourceBody.SetAttributeValue("type", cty.StringVal(sourceType))
	sourceBody.SetAttributeValue("repository", cty.StringVal(repository))
	sourceBody.SetAttributeValue("branch", cty.StringVal(branch))

	fileName := fmt.Sprintf("template.%s.%s.gs.hcl", templateType, name)
	err = os.WriteFile(fileName, f.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("error writing template file: %w", err)
	}

	fmt.Printf("Template generated: %s\n", fileName)
	return nil
}

func installTemplates() error {
	var choice string
	err := huh.NewSelect[string]().
		Title("Select Source").
		Options(
			huh.NewOption("Local Path", "local"),
			huh.NewOption("Remote Repository", "remote"),
		).
		Value(&choice).
		Run()

	if err != nil {
		return fmt.Errorf("error getting source choice: %w", err)
	}

	switch choice {
	case "local":
		path, err := lib.GetPathWithCompletion("Enter local path to template: ")
		if err != nil {
			return fmt.Errorf("error getting local path: %w", err)
		}
		// Here you would implement the logic to install from a local path
		fmt.Printf("Installing template from local path: %s\n", path)
	case "remote":
		var repo string
		err = huh.NewInput().
			Title("Enter repository URL").
			Value(&repo).
			Run()
		if err != nil {
			return fmt.Errorf("error getting repository URL: %w", err)
		}
		// Here you would implement the logic to install from a remote repository
		fmt.Printf("Installing template from remote repository: %s\n", repo)
	}

	return nil
}

func printInstalledTemplatesSummary() error {
	// This function would scan the installed templates directory and print a summary
	fmt.Println("Installed Templates Summary:")
	// Implement the logic to scan and summarize installed templates
	return nil
}

func (t TemplatePlugin) Name() string {
	return "template"
}

func (t TemplatePlugin) Version() string {
	return "1.0.0"
}

// Export the plugin
var Plugin TemplatePlugin
