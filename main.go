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
	clonedRepos := make([]string, 0)
	symlinkedRepos := make([]string, 0)

	for i, repo := range filteredRepos {
		fmt.Printf("Cloning %s...\n", repo)
		_, err := git.PlainClone(filepath.Join(repoDir, repo), false, &git.CloneOptions{
			URL:      fmt.Sprintf("git@%s:%s/%s.git", config.Repositories.Clone.SCM, config.Repositories.Clone.Owner, repo),
			Progress: os.Stdout,
			Auth:     sshAuth,
		})
		if err != nil {
			logger.Error("Error cloning repository", "repo", repo, "error", err)
		} else {
			clonedRepos = append(clonedRepos, repo)
		}
		prog.SetPercent(float64(i+1) / float64(len(filteredRepos)))
		fmt.Print(prog.View())
	}
	fmt.Println() // Add a newline after the progress bar

	// Create symlinks
	for _, repo := range clonedRepos {
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

	// Print summary table
	printSummaryTable(config, clonedRepos, repoDir, baseDir)
}

func printSummaryTable(config Config, clonedRepos []string, repoDir, baseDir string) {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	fmt.Println(style.Render("\nCloning and symlinking summary:"))

	// Define column styles
	repoStyle := lipgloss.NewStyle().Width(20).Align(lipgloss.Left)
	urlStyle := lipgloss.NewStyle().Width(50).Align(lipgloss.Left)
	pathStyle := lipgloss.NewStyle().Width(30).Align(lipgloss.Left)
	symlinkStyle := lipgloss.NewStyle().Width(30).Align(lipgloss.Left)

	// Print table header
	fmt.Printf("%s %s %s %s\n",
		repoStyle.Render("Repository"),
		urlStyle.Render("URL"),
		pathStyle.Render("Cloned Path"),
		symlinkStyle.Render("Symlink"))
	
	fmt.Println(strings.Repeat("-", 130)) // Separator line

	// Print table rows
	for _, repo := range clonedRepos {
		fmt.Printf("%s %s %s %s\n",
			repoStyle.Render(repo),
			urlStyle.Render(fmt.Sprintf("git@%s:%s/%s.git", config.Repositories.Clone.SCM, config.Repositories.Clone.Owner, repo)),
			pathStyle.Render(filepath.Join(repoDir, repo)),
			symlinkStyle.Render(filepath.Join(baseDir, repo)))
	}

	fmt.Printf("\nTotal repositories cloned and symlinked: %d\n", len(clonedRepos))
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
