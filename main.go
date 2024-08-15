package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/mitchellh/go-homedir"
)

// Config represents the structure of our HCL configuration file
type Config struct {
	Repositories *struct {
		GitSpace *struct {
			Path string `hcl:"path"`
		} `hcl:"gitspace,block"`
		Clone *struct {
			SCM      string   `hcl:"scm"`
			Owner    string   `hcl:"owner"`
			EndsWith []string `hcl:"endsWith"`
			Auth     *struct {
				Type    string `hcl:"type"`
				KeyPath string `hcl:"keyPath"`
			} `hcl:"auth,block"`
		} `hcl:"clone,block"`
	} `hcl:"repositories,block"`
}

func main() {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
	})

	// Prompt user for config file path
	var configPath string
	err := huh.NewInput().
		Title("Enter the path to your config file").
		Placeholder("./gs.hcl").
		Value(&configPath).
		Run()

	if err != nil {
		logger.Error("Error getting config path", "error", err)
		return
	}

	// If no input was provided, use the default value
	if configPath == "" {
		configPath = "./gs.hcl"
	}

	// Parse the HCL config file
	var config Config
	err = hclsimple.DecodeFile(configPath, nil, &config)
	if err != nil {
		logger.Error("Error parsing config file", "error", err)
		return
	}

	baseDir := config.Repositories.GitSpace.Path
	repoDir := filepath.Join(baseDir, ".repositories")
	err = os.MkdirAll(repoDir, 0755)
	if err != nil {
		logger.Error("Error creating directories", "error", err)
		return
	}

	// Setup SSH auth
	sshKeyPath, err := homedir.Expand(config.Repositories.Clone.Auth.KeyPath)
	if err != nil {
		logger.Error("Error expanding SSH key path", "error", err)
		return
	}
	sshAuth, err := ssh.NewPublicKeysFromFile("git", sshKeyPath, "")
	if err != nil {
		logger.Error("Error setting up SSH auth", "error", err)
		return
	}

	// Get list of repositories to clone
	repos, err := getRepositories(config.Repositories.Clone.SCM, config.Repositories.Clone.Owner)
	if err != nil {
		logger.Error("Error fetching repositories", "error", err)
		return
	}

	// Filter repositories based on endsWith criteria
	filteredRepos := filterRepositories(repos, config.Repositories.Clone.EndsWith)

	// Clone repositories with progress bar
	prog := progress.New(progress.WithDefaultGradient())
	cloneResults := make(map[string]error)
	symlinkedRepos := make([]string, 0)

	boldStyle := lipgloss.NewStyle().Bold(true)

	for i, repo := range filteredRepos {
		fmt.Printf("%s\n", boldStyle.Render(fmt.Sprintf("Cloning %s...", repo)))
		_, err := git.PlainClone(filepath.Join(repoDir, repo), false, &git.CloneOptions{
			URL:      fmt.Sprintf("git@%s:%s/%s.git", config.Repositories.Clone.SCM, config.Repositories.Clone.Owner, repo),
			Progress: os.Stdout,
			Auth:     sshAuth,
		})
		cloneResults[repo] = err
		prog.SetPercent(float64(i+1) / float64(len(filteredRepos)))
		fmt.Print(prog.View())
		fmt.Println() // Add a newline after each clone operation
	}

	// Create symlinks
	for repo, err := range cloneResults {
		if err == nil {
			source := filepath.Join(".repositories", repo)
			target := filepath.Join(baseDir, repo)

			// Remove existing symlink if it exists
			os.Remove(target)

			err := os.Symlink(source, target)
			if err != nil {
				logger.Error("Error creating symlink", "repo", repo, "error", err)
			} else {
				symlinkedRepos = append(symlinkedRepos, repo)
			}
		}
	}

	// Print summary table
	printSummaryTable(config, cloneResults, repoDir, baseDir, symlinkedRepos)
}

func printSummaryTable(config Config, cloneResults map[string]error, repoDir, baseDir string, symlinkedRepos []string) {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	fmt.Println(headerStyle.Render("\nCloning and symlinking summary:"))

	// Define column styles
	repoStyle := lipgloss.NewStyle().Width(20).Align(lipgloss.Left)
	statusStyle := lipgloss.NewStyle().Width(10).Align(lipgloss.Left)
	urlStyle := lipgloss.NewStyle().Width(50).Align(lipgloss.Left)
	errorStyle := lipgloss.NewStyle().Width(50).Align(lipgloss.Left)

	// Print table header
	fmt.Printf("%s %s %s %s\n",
		repoStyle.Render("📁 Repository"),
		statusStyle.Render("✅ Status"),
		urlStyle.Render("🔗 URL"),
		errorStyle.Render("❌ Error"))
	
	fmt.Println(strings.Repeat("-", 130)) // Separator line

	// Print table rows
	for repo, err := range cloneResults {
		status := "Success"
		errorMsg := "-"
		if err != nil {
			status = "Failed"
			errorMsg = err.Error()
		}
		fmt.Printf("%s %s %s %s\n",
			repoStyle.Render(repo),
			statusStyle.Render(status),
			urlStyle.Render(fmt.Sprintf("git@%s:%s/%s.git", config.Repositories.Clone.SCM, config.Repositories.Clone.Owner, repo)),
			errorStyle.Render(errorMsg))
	}

	// Only print symlinked repositories if there are any
	if len(symlinkedRepos) > 0 {
		fmt.Println("\nSymlinked repositories:")
		for _, repo := range symlinkedRepos {
			fmt.Printf("  - %s -> %s\n", repo, filepath.Join(baseDir, repo))
		}
	}

	fmt.Printf("\nSummary of changes:\n")
	fmt.Printf("  Total repositories attempted: %d\n", len(cloneResults))
	fmt.Printf("  Successfully cloned: %d\n", len(symlinkedRepos))
	fmt.Printf("  Failed to clone: %d\n", len(cloneResults)-len(symlinkedRepos))
}

func getRepositories(scm, owner string) ([]string, error) {
	// This is a placeholder. In a real implementation, you would
	// fetch the list of repositories from the SCM (e.g., using GitHub API)
	return []string{"GitSpace", "SSOTSpace", "K1Space", "SCMany"}, nil
}

func filterRepositories(repos []string, endsWith []string) []string {
	var filtered []string
	for _, repo := range repos {
		for _, suffix := range endsWith {
			if strings.HasSuffix(strings.ToLower(repo), strings.ToLower(suffix)) {
				filtered = append(filtered, repo)
				break
			}
		}
	}
	return filtered
}
