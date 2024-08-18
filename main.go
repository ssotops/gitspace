package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
  "runtime/debug"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/mitchellh/go-homedir"
	"github.com/ssotspace/gitspace/lib"
)

// Config represents the structure of our HCL configuration file
type Config struct {
	Repositories *struct {
		GitSpace *struct {
			Path string `hcl:"path"`
		} `hcl:"gitspace,block"`
		Clone *struct {
			SCM        string   `hcl:"scm"`
			Owner      string   `hcl:"owner"`
			EndsWith   []string `hcl:"endsWith,optional"`
			StartsWith []string `hcl:"startsWith,optional"`
			Includes   []string `hcl:"includes,optional"`
			Names      []string `hcl:"name,optional"`
			Auth       *struct {
				Type    string `hcl:"type"`
				KeyPath string `hcl:"keyPath"`
			} `hcl:"auth,block"`
		} `hcl:"clone,block"`
	} `hcl:"repositories,block"`
}

func getSSHKeyPath(configPath string) (string, error) {
	if strings.HasPrefix(configPath, "$") {
		envVar := strings.TrimPrefix(configPath, "$")
		path := os.Getenv(envVar)
		if path == "" {
			return "", fmt.Errorf("environment variable %s is not set", envVar)
		}
		return path, nil
	}
	return configPath, nil
}

func main() {
	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
	})

	// Create styles for the welcome message
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFD700")). // Gold color
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(1)

	subtitleStyle := lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#87CEEB")) // Sky Blue color

	// Get current version
	version := getCurrentVersion()

	// Print welcome message
	fmt.Println(titleStyle.Render("Welcome to Gitspace!"))
	fmt.Println(subtitleStyle.Render(fmt.Sprintf("Current version: %s", version)))
	fmt.Println()

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

	// Create a menu for user to choose an action
	var choice string
	err = huh.NewSelect[string]().
		Title("Choose an action").
		Options(
			huh.NewOption("Clone Repositories", "clone"),
			huh.NewOption("Upgrade Gitspace", "upgrade"),
		).
		Value(&choice).
		Run()

	if err != nil {
		logger.Error("Error getting user choice", "error", err)
		return
	}

	switch choice {
	case "clone":
		cloneRepositories(logger, &config)
	case "upgrade":
		upgradeGitspace(logger)
	default:
		logger.Error("Invalid choice")
	}
}

func cloneRepositories(logger *log.Logger, config *Config) {
	// This function will contain most of the original main() function logic
	baseDir := config.Repositories.GitSpace.Path
	repoDir := filepath.Join(baseDir, ".repositories")
	err := os.MkdirAll(repoDir, 0755)
	if err != nil {
		logger.Error("Error creating directories", "error", err)
		return
	}

	// Setup SSH auth
	sshKeyPath, err := getSSHKeyPath(config.Repositories.Clone.Auth.KeyPath)
	if err != nil {
		logger.Error("Error getting SSH key path", "error", err)
		return
	}
	sshKeyPath, err = homedir.Expand(sshKeyPath)
	if err != nil {
		logger.Error("Error expanding SSH key path", "error", err)
		return
	}
	sshAuth, err := ssh.NewPublicKeysFromFile("git", sshKeyPath, "")
	if err != nil {
		logger.Error("Error setting up SSH auth", "error", err)
		return
	}

	// Check for GitHub token
	if os.Getenv("GITHUB_TOKEN") == "" {
		logger.Error("GITHUB_TOKEN environment variable not set. Please set it and try again.")
		return
	}

	// Log the configuration
	logger.Info("Configuration loaded",
		"scm", config.Repositories.Clone.SCM,
		"owner", config.Repositories.Clone.Owner,
		"endsWith", config.Repositories.Clone.EndsWith)

	// Get list of repositories to clone
	repos, err := lib.GetRepositories(config.Repositories.Clone.SCM, config.Repositories.Clone.Owner)
	if err != nil {
		logger.Error("Error fetching repositories", "error", err)
		return
	}

	logger.Info("Fetched repositories", "count", len(repos), "repos", repos)

	// Filter repositories based on criteria
	filteredRepos := filterRepositories(repos, config)

	logger.Info("Filtered repositories", "count", len(filteredRepos), "repos", filteredRepos)

	if len(filteredRepos) == 0 {
		logger.Warn("No repositories match the filter criteria")
		return
	}

	// Clone repositories with progress bar
	cloneResults := make(map[string]error)
	symlinkedRepos := make([]string, 0)

	boldStyle := lipgloss.NewStyle().Bold(true)

	for _, repo := range filteredRepos {
		fmt.Printf("%s\n", boldStyle.Render(fmt.Sprintf("Cloning %s...", repo)))
		_, err := git.PlainClone(filepath.Join(repoDir, repo), false, &git.CloneOptions{
			URL:      fmt.Sprintf("git@%s:%s/%s.git", config.Repositories.Clone.SCM, config.Repositories.Clone.Owner, repo),
			Progress: os.Stdout,
			Auth:     sshAuth,
		})
		cloneResults[repo] = err
		if err == nil {
			fmt.Println(boldStyle.Render("Clone successful"))
		} else {
			fmt.Println(boldStyle.Render("Clone failed"))
		}
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

func upgradeGitspace(logger *log.Logger) {
	logger.Info("Upgrading Gitspace...")

	// Define repository details
	repo := "ssotops/gitspace"
	binary := "gitspace"

	// Determine OS and architecture
	osName := runtime.GOOS
	arch := runtime.GOARCH

	// Fetch the latest release information
	logger.Info("Fetching latest release information...")
	releaseInfo, err := fetchLatestReleaseInfo(repo)
	if err != nil {
		logger.Error("Failed to fetch latest release information", "error", err)
		return
	}

	version := releaseInfo.TagName
	logger.Info("Latest version", "version", version)

	// Construct the download URL for the specific asset
	assetName := fmt.Sprintf("%s_%s_%s", binary, osName, arch)
	if osName == "windows" {
		assetName += ".exe"
	}
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, version, assetName)

	// Download the binary
	logger.Info("Downloading new version", "version", version, "os", osName, "arch", arch)
	tempFile, err := downloadBinary(downloadURL)
	if err != nil {
		logger.Error("Failed to download binary", "error", err)
		return
	}
	defer os.Remove(tempFile)

	// Make it executable (skip for Windows)
	if osName != "windows" {
		err = os.Chmod(tempFile, 0755)
		if err != nil {
			logger.Error("Failed to make binary executable", "error", err)
			return
		}
	}

	// Get the path of the current executable
	execPath, err := os.Executable()
	if err != nil {
		logger.Error("Failed to get current executable path", "error", err)
		return
	}

	// Replace the current binary with the new one
	err = os.Rename(tempFile, execPath)
	if err != nil {
		logger.Error("Failed to replace current binary", "error", err)
		return
	}

	logger.Info("Gitspace has been successfully upgraded!", "version", version)
}

type ReleaseInfo struct {
	TagName string `json:"tag_name"`
	ID      int    `json:"id"`
}

func fetchLatestReleaseInfo(repo string) (*ReleaseInfo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var releaseInfo ReleaseInfo
	err = json.Unmarshal(body, &releaseInfo)
	if err != nil {
		return nil, err
	}

	return &releaseInfo, nil
}

func downloadBinary(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	tempFile, err := os.CreateTemp("", "gitspace-*")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		return "", err
	}

	return tempFile.Name(), nil
}

func printSummaryTable(config *Config, cloneResults map[string]error, repoDir, baseDir string, symlinkedRepos []string) {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	fmt.Println(headerStyle.Render("\nCloning and symlinking summary:"))
	fmt.Println() // Add a newline after the header

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

	// Print symlinked repositories table
	if len(symlinkedRepos) > 0 {
		fmt.Println()
		fmt.Println(headerStyle.Render("Symlinked repositories:"))
		fmt.Println() // Add a newline after the header

		// Define column styles for symlink table
		repoStyle := lipgloss.NewStyle().Width(20).Align(lipgloss.Left)
		symlinkStyle := lipgloss.NewStyle().Width(30).Align(lipgloss.Left)
		pathStyle := lipgloss.NewStyle().Width(30).Align(lipgloss.Left)

		// Print table header
		fmt.Printf("%s %s %s\n",
			repoStyle.Render("📁 Repository"),
			symlinkStyle.Render("🔗 Symlink Path"),
			pathStyle.Render("📂 Repository Path"))

		fmt.Println(strings.Repeat("-", 80)) // Separator line

		// Print table rows
		for _, repo := range symlinkedRepos {
			fmt.Printf("%s %s %s\n",
				repoStyle.Render(repo),
				symlinkStyle.Render(filepath.Join(baseDir, repo)),
				pathStyle.Render(filepath.Join(repoDir, repo)))
		}
	}

	fmt.Println()
	fmt.Println(headerStyle.Render("Summary of changes:"))
	fmt.Println() // Add a newline after the header
	totalAttempted := len(cloneResults)
	successfulClones := len(symlinkedRepos)
	fmt.Printf("  Total repositories attempted: %d\n", totalAttempted)
	fmt.Printf("  Successfully cloned: %d/%d\n", successfulClones, totalAttempted)
}

func getRepositories(scm, owner string) ([]string, error) {
	// This is a placeholder. In a real implementation, you would
	// fetch the list of repositories from the SCM (e.g., using GitHub API)
	return []string{"GitSpace", "SSOTSpace", "K1Space", "SCMany"}, nil
}

func filterRepositories(repos []string, config *Config) []string {
	var filtered []string
	cloneConfig := config.Repositories.Clone

	for _, repo := range repos {
		// Check exact names
		if len(cloneConfig.Names) > 0 {
			for _, name := range cloneConfig.Names {
				if strings.EqualFold(repo, name) {
					filtered = append(filtered, repo)
					goto nextRepo
				}
			}
		}

		// Check startsWith
		if len(cloneConfig.StartsWith) > 0 {
			for _, prefix := range cloneConfig.StartsWith {
				if strings.HasPrefix(strings.ToLower(repo), strings.ToLower(prefix)) {
					filtered = append(filtered, repo)
					goto nextRepo
				}
			}
		}

		// Check endsWith
		if len(cloneConfig.EndsWith) > 0 {
			for _, suffix := range cloneConfig.EndsWith {
				if strings.HasSuffix(strings.ToLower(repo), strings.ToLower(suffix)) {
					filtered = append(filtered, repo)
					goto nextRepo
				}
			}
		}

		// Check includes
		if len(cloneConfig.Includes) > 0 {
			for _, substr := range cloneConfig.Includes {
				if strings.Contains(strings.ToLower(repo), strings.ToLower(substr)) {
					filtered = append(filtered, repo)
					goto nextRepo
				}
			}
		}

		// If no filters are specified, include all repositories
		if len(cloneConfig.Names) == 0 && len(cloneConfig.StartsWith) == 0 &&
			len(cloneConfig.EndsWith) == 0 && len(cloneConfig.Includes) == 0 {
			filtered = append(filtered, repo)
		}

	nextRepo:
	}

	return filtered
}

func getCurrentVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			return setting.Value[:7] // Return first 7 characters of the git commit hash
		}
	}
	return "unknown"
}
