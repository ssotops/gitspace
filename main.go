package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/mitchellh/go-homedir"
	"github.com/ssotspace/gitspace/lib"
)

var Version string

// Config represents the structure of our HCL configuration file
type Config struct {
	Repositories *struct {
		GitSpace *struct {
			Path string `hcl:"path"`
		} `hcl:"gitspace,block"`
		Labels []string `hcl:"labels,optional"`
		Clone  *struct {
			SCM        string        `hcl:"scm"`
			Owner      string        `hcl:"owner"`
			EndsWith   *FilterConfig `hcl:"endsWith,block"`
			StartsWith *FilterConfig `hcl:"startsWith,block"`
			Includes   *FilterConfig `hcl:"includes,block"`
			Names      *FilterConfig `hcl:"name,block"`
			Auth       *struct {
				Type    string `hcl:"type"`
				KeyPath string `hcl:"keyPath"`
			} `hcl:"auth,block"`
		} `hcl:"clone,block"`
	} `hcl:"repositories,block"`
}

type FilterConfig struct {
	Values     []string `hcl:"values"`
	Repository *struct {
		Type   string   `hcl:"type"`
		Labels []string `hcl:"labels"`
	} `hcl:"repository,block"`
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
		Level:           log.DebugLevel, // Set to DebugLevel to see all logs
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
	config, err := decodeHCLFile(configPath)
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
			huh.NewOption("Sync Labels", "sync"),
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
	case "sync":
		syncLabels(logger, &config)
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
	// Log the configuration
	logger.Info("Configuration loaded",
		"scm", config.Repositories.Clone.SCM,
		"owner", config.Repositories.Clone.Owner)

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

func filterRepositories(repos []string, config *Config) []string {
	var filtered []string
	cloneConfig := config.Repositories.Clone

	for _, repo := range repos {
		// Check exact names
		if matchesFilter(repo, cloneConfig.Names) {
			filtered = append(filtered, repo)
			continue
		}

		// Check startsWith
		if matchesFilter(repo, cloneConfig.StartsWith) {
			filtered = append(filtered, repo)
			continue
		}

		// Check endsWith
		if matchesFilter(repo, cloneConfig.EndsWith) {
			filtered = append(filtered, repo)
			continue
		}

		// Check includes
		if matchesFilter(repo, cloneConfig.Includes) {
			filtered = append(filtered, repo)
			continue
		}

		// If no filters are specified, include all repositories
		if cloneConfig.Names == nil && cloneConfig.StartsWith == nil &&
			cloneConfig.EndsWith == nil && cloneConfig.Includes == nil {
			filtered = append(filtered, repo)
		}
	}

	return filtered
}

func matchesFilter(repo string, filter *FilterConfig) bool {
	if filter == nil || len(filter.Values) == 0 {
		return false
	}
	for _, value := range filter.Values {
		if strings.HasPrefix(strings.ToLower(repo), strings.ToLower(value)) ||
			strings.HasSuffix(strings.ToLower(repo), strings.ToLower(value)) ||
			strings.Contains(strings.ToLower(repo), strings.ToLower(value)) ||
			repo == value {
			return true
		}
	}
	return false
}

func getCurrentVersion() string {
	// Check if Version is set (injected during build)
	if Version != "" {
		return Version
	}

	// Try to get the git commit hash
	hash, err := getGitCommitHash()
	if err == nil && hash != "" {
		return hash[:7] // Return first 7 characters of the git commit hash
	}

	// If git commit hash is not available, try to get it from build info
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value[:7] // Return first 7 characters of the git commit hash
			}
		}
	}

	// If all else fails, return "unknown"
	return "unknown"
}

func getGitCommitHash() (string, error) {
	// Try using git command
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}

	// If git command fails, try using go-git
	repo, err := git.PlainOpen(".")
	if err != nil {
		return "", err
	}

	ref, err := repo.Head()
	if err != nil {
		return "", err
	}

	return ref.Hash().String(), nil
}

func syncLabels(logger *log.Logger, config *Config) {
	// Fetch repositories
	repos, err := lib.GetRepositories(config.Repositories.Clone.SCM, config.Repositories.Clone.Owner)
	if err != nil {
		logger.Error("Error fetching repositories", "error", err)
		return
	}

	// Calculate label changes
	changes := calculateLabelChanges(repos, config)

	// Print summary of changes
	printLabelChangeSummary(changes)

	// Prompt for confirmation
	confirmed := confirmChanges()
	if !confirmed {
		logger.Info("Label sync cancelled by user")
		return
	}

	// Apply changes
	applyLabelChanges(changes, logger)
}

func calculateLabelChanges(repos []string, config *Config) map[string][]string {
	changes := make(map[string][]string)

	for _, repo := range repos {
		// Add global labels
		changes[repo] = append(changes[repo], config.Repositories.Labels...)

		// Check each filter and add corresponding labels
		if config.Repositories.Clone != nil {
			if matchesFilter(repo, config.Repositories.Clone.StartsWith) {
				changes[repo] = append(changes[repo], getLabelsFromFilter(config.Repositories.Clone.StartsWith)...)
			}
			if matchesFilter(repo, config.Repositories.Clone.EndsWith) {
				changes[repo] = append(changes[repo], getLabelsFromFilter(config.Repositories.Clone.EndsWith)...)
			}
			if matchesFilter(repo, config.Repositories.Clone.Includes) {
				changes[repo] = append(changes[repo], getLabelsFromFilter(config.Repositories.Clone.Includes)...)
			}
			if matchesFilter(repo, config.Repositories.Clone.Names) {
				changes[repo] = append(changes[repo], getLabelsFromFilter(config.Repositories.Clone.Names)...)
			}
		}

		// Remove duplicates
		changes[repo] = removeDuplicates(changes[repo])
	}

	return changes
}

func getLabelsFromFilter(filter *FilterConfig) []string {
	if filter != nil && filter.Repository != nil {
		return filter.Repository.Labels
	}
	return []string{}
}

func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func printLabelChangeSummary(changes map[string][]string) {
	fmt.Println("Label Sync Summary:")
	for repo, labels := range changes {
		fmt.Printf("%s:\n", repo)
		for _, label := range labels {
			fmt.Printf("  + %s\n", label)
		}
		fmt.Println()
	}
}

func confirmChanges() bool {
	var confirmed bool
	err := huh.NewConfirm().
		Title("Do you want to apply these changes?").
		Value(&confirmed).
		Run()

	if err != nil {
		fmt.Println("Error getting confirmation:", err)
		return false
	}

	return confirmed
}

func applyLabelChanges(changes map[string][]string, logger *log.Logger) {
	for repo, labels := range changes {
		err := lib.AddLabelsToRepository(repo, labels)
		if err != nil {
			logger.Error("Error applying labels to repository", "repo", repo, "error", err)
		} else {
			logger.Info("Labels applied successfully", "repo", repo, "labels", labels)
		}
	}
}

func decodeHCLFile(filename string) (Config, error) {
	logger := log.New(os.Stderr)
	logger.SetLevel(log.DebugLevel)

	logger.Debug("Reading file", "filename", filename)
	src, err := os.ReadFile(filename)
	if err != nil {
		logger.Error("Failed to read file", "error", err)
		return Config{}, err
	}

	logger.Debug("Parsing HCL config")
	file, diags := hclsyntax.ParseConfig(src, filename, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		logger.Error("Failed to parse HCL", "diagnostics", diags.Error())
		return Config{}, fmt.Errorf("failed to parse HCL: %s", formatDiagnostics(diags))
	}

	var config Config
	logger.Debug("Decoding HCL body")
	decodeDiags := gohcl.DecodeBody(file.Body, nil, &config)
	if decodeDiags.HasErrors() {
		logger.Warn("Failed to decode new format, attempting to decode old format", "diagnostics", decodeDiags.Error())

		// Try to decode using the old format
		var oldConfig struct {
			Repositories *struct {
				GitSpace *struct {
					Path string `hcl:"path"`
				} `hcl:"gitspace,block"`
				Labels []string `hcl:"labels,optional"`
				Clone  *struct {
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

		oldDecodeDiags := gohcl.DecodeBody(file.Body, nil, &oldConfig)
		if oldDecodeDiags.HasErrors() {
			logger.Error("Failed to decode old format config", "diagnostics", oldDecodeDiags.Error())
			return Config{}, fmt.Errorf("failed to decode HCL: %s", formatDiagnostics(oldDecodeDiags))
		}

		// Convert old format to new format
		config.Repositories = &struct {
			GitSpace *struct {
				Path string `hcl:"path"`
			} `hcl:"gitspace,block"`
			Labels []string `hcl:"labels,optional"`
			Clone  *struct {
				SCM        string        `hcl:"scm"`
				Owner      string        `hcl:"owner"`
				EndsWith   *FilterConfig `hcl:"endsWith,block"`
				StartsWith *FilterConfig `hcl:"startsWith,block"`
				Includes   *FilterConfig `hcl:"includes,block"`
				Names      *FilterConfig `hcl:"name,block"`
				Auth       *struct {
					Type    string `hcl:"type"`
					KeyPath string `hcl:"keyPath"`
				} `hcl:"auth,block"`
			} `hcl:"clone,block"`
		}{
			GitSpace: oldConfig.Repositories.GitSpace,
			Labels:   oldConfig.Repositories.Labels,
			Clone: &struct {
				SCM        string        `hcl:"scm"`
				Owner      string        `hcl:"owner"`
				EndsWith   *FilterConfig `hcl:"endsWith,block"`
				StartsWith *FilterConfig `hcl:"startsWith,block"`
				Includes   *FilterConfig `hcl:"includes,block"`
				Names      *FilterConfig `hcl:"name,block"`
				Auth       *struct {
					Type    string `hcl:"type"`
					KeyPath string `hcl:"keyPath"`
				} `hcl:"auth,block"`
			}{
				SCM:   oldConfig.Repositories.Clone.SCM,
				Owner: oldConfig.Repositories.Clone.Owner,
				EndsWith: &FilterConfig{
					Values: oldConfig.Repositories.Clone.EndsWith,
				},
				StartsWith: &FilterConfig{
					Values: oldConfig.Repositories.Clone.StartsWith,
				},
				Includes: &FilterConfig{
					Values: oldConfig.Repositories.Clone.Includes,
				},
				Names: &FilterConfig{
					Values: oldConfig.Repositories.Clone.Names,
				},
				Auth: oldConfig.Repositories.Clone.Auth,
			},
		}

		logger.Info("Successfully decoded old format config")
	} else {
		logger.Info("Successfully decoded new format config")
	}

	logger.Debug("Config decoding completed")
	return config, nil
}

func formatDiagnostics(diags hcl.Diagnostics) string {
	var messages []string
	for _, diag := range diags {
		messages = append(messages, fmt.Sprintf("%s: %s at %s", diag.Severity, diag.Summary, diag.Subject))
		if diag.Detail != "" {
			messages = append(messages, fmt.Sprintf("  Detail: %s", diag.Detail))
		}
	}
	return strings.Join(messages, "\n")
}
