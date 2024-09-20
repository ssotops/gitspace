package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type pathCompletionModel struct {
	textInput   textinput.Model
	choices     []string
	showChoices bool
}

func (m pathCompletionModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m pathCompletionModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			return m, tea.Quit
		case tea.KeyTab:
			m.choices = completePathInternal(m.textInput.Value())
			if len(m.choices) == 1 {
				m.textInput.SetValue(m.choices[0])
				m.textInput.CursorEnd()
			}
			m.showChoices = true
		default:
			m.showChoices = false
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m pathCompletionModel) View() string {
	view := fmt.Sprintf("Enter path: %s\n", m.textInput.View())
	if m.showChoices && len(m.choices) > 1 {
		view += "Completions:\n"
		for _, choice := range m.choices {
			view += fmt.Sprintf("  %s\n", choice)
		}
	}
	return view
}

func completePathInternal(input string) []string {
	dir := filepath.Dir(input)
	base := filepath.Base(input)

	files, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var matches []string
	for _, file := range files {
		name := file.Name()
		if strings.HasPrefix(name, base) {
			fullPath := filepath.Join(dir, name)
			if file.IsDir() {
				fullPath += string(os.PathSeparator)
			}
			matches = append(matches, fullPath)
		}
	}

	return matches
}

func getPathWithCompletion(prompt string) (string, error) {
	ti := textinput.New()
	ti.Placeholder = prompt
	ti.Focus()

	m := pathCompletionModel{
		textInput: ti,
	}

	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return "", err
	}

	finalPath := strings.TrimSpace(finalModel.(pathCompletionModel).textInput.Value())
	return finalPath, nil
}
