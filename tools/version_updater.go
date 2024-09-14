package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func UpdateVersions() error {
	fmt.Println("Updating main Gitspace go.mod...")
	if err := updateGoMod("go.mod"); err != nil {
		return fmt.Errorf("failed to update main go.mod: %w", err)
	}

	fmt.Println("Updating local plugins...")
	if err := updateLocalPlugins("./examples/plugins"); err != nil {
		return fmt.Errorf("failed to update local plugins: %w", err)
	}

	fmt.Println("Updating catalog plugins...")
	return updateCatalogPlugins()
}

func updateLocalPlugins(pluginsDir string) error {
	plugins, err := os.ReadDir(pluginsDir)
	if err != nil {
		return fmt.Errorf("error reading local plugins directory: %w", err)
	}

	for _, plugin := range plugins {
		if plugin.IsDir() {
			pluginGoMod := filepath.Join(pluginsDir, plugin.Name(), "go.mod")
			fmt.Printf("Updating %s...\n", pluginGoMod)
			if err := updateGoMod(pluginGoMod); err != nil {
				return fmt.Errorf("failed to update %s: %w", pluginGoMod, err)
			}
		}
	}
	return nil
}

func updateCatalogPlugins() error {
	fmt.Println("Creating temporary directory for gitspace-catalog...")
	tempDir, err := os.MkdirTemp("", "gitspace-catalog")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Println("Cloning gitspace-catalog repository...")
	_, err = git.PlainClone(tempDir, false, &git.CloneOptions{
		URL: "https://github.com/ssotops/gitspace-catalog.git",
	})
	if err != nil {
		return fmt.Errorf("failed to clone gitspace-catalog: %w", err)
	}

	pluginsDir := filepath.Join(tempDir, "plugins")
	plugins, err := os.ReadDir(pluginsDir)
	if err != nil {
		return fmt.Errorf("error reading catalog plugins directory: %w", err)
	}

	for _, plugin := range plugins {
		if plugin.IsDir() {
			pluginGoMod := filepath.Join(pluginsDir, plugin.Name(), "go.mod")
			fmt.Printf("Updating %s...\n", pluginGoMod)
			if err := updateGoMod(pluginGoMod); err != nil {
				return fmt.Errorf("failed to update %s: %w", pluginGoMod, err)
			}
		}
	}

	fmt.Println("Opening git repository...")
	r, err := git.PlainOpen(tempDir)
	if err != nil {
		return fmt.Errorf("failed to open git repo: %w", err)
	}

	w, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	fmt.Println("Checking for changes...")
	status, err := w.Status()
	if err != nil {
		return fmt.Errorf("failed to get worktree status: %w", err)
	}

	if status.IsClean() {
		fmt.Println("No changes to commit in gitspace-catalog")
		return nil
	}

	fmt.Println("Creating new branch...")
	branchName := fmt.Sprintf("update-versions-%d", time.Now().Unix())
	err = w.Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.NewBranchReferenceName(branchName),
	})
	if err != nil {
		return fmt.Errorf("failed to create new branch: %w", err)
	}

	fmt.Println("Committing changes...")
	_, err = w.Add(".")
	if err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	_, err = w.Commit("Update Charm library versions", &git.CommitOptions{})
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	fmt.Println("Pushing changes...")
	err = r.Push(&git.PushOptions{})
	if err != nil {
		return fmt.Errorf("failed to push changes: %w", err)
	}

	fmt.Printf("Changes pushed to branch '%s' in gitspace-catalog\n", branchName)
	fmt.Println("Please create a pull request to merge these changes.")

	return nil
}

func updateGoMod(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading %s: %w", path, err)
	}

	lines := strings.Split(string(content), "\n")
	updated := false

	for i, line := range lines {
		if strings.Contains(line, "github.com/charmbracelet/bubbles") {
			lines[i] = fmt.Sprintf("\tgithub.com/charmbracelet/bubbles %s", BubblesVersion)
			updated = true
		} else if strings.Contains(line, "github.com/charmbracelet/bubbletea") {
			lines[i] = fmt.Sprintf("\tgithub.com/charmbracelet/bubbletea %s", BubbleteaVersion)
			updated = true
		} else if strings.Contains(line, "github.com/charmbracelet/huh") {
			lines[i] = fmt.Sprintf("\tgithub.com/charmbracelet/huh %s", HuhVersion)
			updated = true
		} else if strings.Contains(line, "github.com/charmbracelet/lipgloss") {
			lines[i] = fmt.Sprintf("\tgithub.com/charmbracelet/lipgloss %s", LipglossVersion)
			updated = true
		} else if strings.Contains(line, "github.com/charmbracelet/log") {
			lines[i] = fmt.Sprintf("\tgithub.com/charmbracelet/log %s", LogVersion)
			updated = true
		}
	}

	if updated {
		err = os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
		if err != nil {
			return fmt.Errorf("error writing %s: %w", path, err)
		}
		fmt.Printf("Updated %s\n", path)
	} else {
		fmt.Printf("No updates needed for %s\n", path)
	}

	return nil
}
