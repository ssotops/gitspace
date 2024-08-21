package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mitchellh/go-homedir"
	"github.com/ssotspace/gitspace/lib"
	"github.com/zclconf/go-cty/cty"
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
			IsExactly  *FilterConfig `hcl:"isExactly,block"`
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

type IndexHCL struct {
	LastUpdated  string                     `hcl:"lastUpdated"`
	Repositories map[string]SCMRepositories `hcl:"repositories"`
}

type SCMRepositories struct {
	Owners map[string]OwnerRepositories `hcl:"owners"`
}

type OwnerRepositories struct {
	Repos map[string]RepoInfo `hcl:"repos"`
}

type RepoInfo struct {
	ConfigPath string    `hcl:"configPath"`
	BackupPath string    `hcl:"backupPath"`
	LastCloned time.Time `hcl:"lastCloned"`
	LastSynced time.Time `hcl:"lastSynced"`
}

func updateIndexHCL(logger *log.Logger, config *Config, repoResults map[string]*RepoResult) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return fmt.Errorf("failed to get cache directory: %w", err)
	}

	indexPath := filepath.Join(cacheDir, "index.hcl")
	configsDir := filepath.Join(cacheDir, ".configs")

	// Ensure .configs directory exists
	if err := os.MkdirAll(configsDir, 0755); err != nil {
		return fmt.Errorf("failed to create .configs directory: %w", err)
	}

	// Create a new empty HCL file
	f := hclwrite.NewEmptyFile()
	rootBody := f.Body()

	// Add lastUpdated
	now := time.Now()
	rootBody.SetAttributeValue("lastUpdated", cty.StringVal(now.Format(time.RFC3339)))

	// Create repositories block
	reposBlock := rootBody.AppendNewBlock("repositories", nil)
	reposBody := reposBlock.Body()

	scm := config.Repositories.Clone.SCM
	owner := config.Repositories.Clone.Owner

	// Create SCM block
	scmBlock := reposBody.AppendNewBlock(scm, nil)
	scmBody := scmBlock.Body()

	// Create owner block
	ownerBlock := scmBody.AppendNewBlock("owners", nil)
	ownerBody := ownerBlock.Body()

	// Create repos block
	reposBlock = ownerBody.AppendNewBlock(owner, nil)
	reposBody = reposBlock.Body()

	// Get the current working directory
	pwd, err := os.Getwd()
	if err != nil {
		logger.Error("Failed to get current working directory", "error", err)
		pwd = "" // Use empty string if we can't get the current directory
	}

	// Read the original config file once
	originalConfigPath := filepath.Join(pwd, "gs.hcl") // Assuming the config file is always named gs.hcl
	originalConfigContent, err := os.ReadFile(originalConfigPath)
	if err != nil {
		logger.Error("Failed to read original config file", "path", originalConfigPath, "error", err)
		// Continue execution, but log the error
	}

	// Create a single backup file for all repositories
	backupFileName := fmt.Sprintf("%s_%s_%s.hcl", scm, owner, now.Format("20060102_150405"))
	backupPath := filepath.Join(configsDir, backupFileName)

	// Create backup file
	if len(originalConfigContent) > 0 {
		if err := os.WriteFile(backupPath, originalConfigContent, 0644); err != nil {
			logger.Error("Failed to write config backup", "path", backupPath, "error", err)
			// Continue execution, but log the error
		} else {
			logger.Info("Created backup config file", "path", backupPath)
		}
	} else {
		logger.Warn("Skipped creating backup file due to empty original config")
	}

	for repo, result := range repoResults {
		// Create repo block
		repoBlock := reposBody.AppendNewBlock(repo, nil)
		repoBody := repoBlock.Body()

		repoBody.SetAttributeValue("configPath", cty.StringVal(originalConfigPath))
		repoBody.SetAttributeValue("backupPath", cty.StringVal(backupPath))

		if result.Cloned {
			repoBody.SetAttributeValue("lastCloned", cty.StringVal(now.Format(time.RFC3339)))
		}
		if result.Updated {
			repoBody.SetAttributeValue("lastSynced", cty.StringVal(now.Format(time.RFC3339)))
		}

		// Add repository type
		repoType := getRepoType(config, repo)
		repoBody.SetAttributeValue("type", cty.StringVal(repoType))

		// Add metadata
		metadataBlock := repoBody.AppendNewBlock("metadata", nil)
		metadataBody := metadataBlock.Body()

		// Set url (formerly URI)
		url := fmt.Sprintf("https://%s/%s/%s", scm, owner, repo)
		metadataBody.SetAttributeValue("url", cty.StringVal(url))

		// Set labels (lowercase)
		labels := getRepoLabels(config, repo)
		labelsVal := make([]cty.Value, len(labels))
		for i, label := range labels {
			labelsVal[i] = cty.StringVal(label)
		}
		metadataBody.SetAttributeValue("labels", cty.ListVal(labelsVal))

		// RepoType removed as requested
	}

	// Write updated index.hcl
	indexContent := f.Bytes()
	if err := os.WriteFile(indexPath, indexContent, 0644); err != nil {
		return fmt.Errorf("failed to write index.hcl: %w", err)
	}

	logger.Info("Successfully updated index.hcl")
	return nil
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
		Level:           log.DebugLevel,
	})

	// Set up signal handling for graceful shutdown
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// Create styles for the welcome message
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFD700")).
		BorderStyle(lipgloss.RoundedBorder()).
		Padding(1)

	subtitleStyle := lipgloss.NewStyle().
		Italic(true).
		Foreground(lipgloss.Color("#87CEEB"))

	// Get current version
	version, _ := getCurrentVersion()

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

	// Main menu loop
	for {
		select {
		case <-signalChan:
			fmt.Println("\nReceived interrupt signal. Exiting Gitspace...")
			return
		default:
			choice := showMainMenu()

			switch choice {
			case "repositories":
				if handleRepositoriesCommand(logger, &config) {
					return // Exit the program if user chose to quit
				}
			case "sync":
				syncLabels(logger, &config)
			case "gitspace":
				handleGitspaceCommand(logger, &config)
			case "symlinks":
				handleSymlinksCommand(logger, &config)
			case "quit":
				fmt.Println("Exiting Gitspace. Goodbye!")
				return
			case "":
				// User likely pressed CTRL+C, exit gracefully
				fmt.Println("\nExiting Gitspace. Goodbye!")
				return
			default:
				logger.Error("Invalid choice")
			}
		}
	}
}

func showMainMenu() string {
	var choice string
	err := huh.NewSelect[string]().
		Title("Choose an action").
		Options(
			huh.NewOption("Repositories", "repositories"),
			huh.NewOption("Symlinks", "symlinks"),
			huh.NewOption("Sync Labels", "sync"),
			huh.NewOption("Gitspace", "gitspace"),
			huh.NewOption("Quit", "quit"),
		).
		Value(&choice).
		Run()

	if err != nil {
		if err == huh.ErrUserAborted {
			return "" // Return empty string on CTRL+C
		}
		fmt.Println("Error getting user choice:", err)
		return ""
	}

	return choice
}

func handleGitspaceCommand(logger *log.Logger, config *Config) {
	for {
		var choice string
		err := huh.NewSelect[string]().
			Title("Choose a Gitspace action").
			Options(
				huh.NewOption("Upgrade Gitspace", "upgrade"),
				huh.NewOption("Print Config Paths", "config_paths"),
				huh.NewOption("Print Version Info", "version_info"),
				huh.NewOption("Go back", "back"),
			).
			Value(&choice).
			Run()

		if err != nil {
			logger.Error("Error getting Gitspace sub-choice", "error", err)
			return
		}

		switch choice {
		case "upgrade":
			upgradeGitspace(logger)
		case "config_paths":
			handleConfigPathsCommand(logger)
		case "version_info":
			printVersionInfo(logger)
		case "back":
			return
		default:
			logger.Error("Invalid Gitspace sub-choice")
		}
	}
}

func handleConfigPathsCommand(logger *log.Logger) {
	cacheDir, err := getCacheDir()
	if err != nil {
		logger.Error("Error getting cache directory", "error", err)
		return
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FFFF"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00"))
	symlinkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00FF"))

	fmt.Println(titleStyle.Render("\n📂 Cache Directory:"))
	fmt.Printf("   %s\n\n", pathStyle.Render(fmt.Sprintf("cd %s", cacheDir)))

	fmt.Println(titleStyle.Render("📄 Gitspace Config Files:"))
	err = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".hcl" {
			fmt.Printf("   %s\n", pathStyle.Render(path))
		}
		return nil
	})
	if err != nil {
		logger.Error("Error walking through cache directory", "error", err)
	}

	fmt.Println(titleStyle.Render("\n🔗 Gitspace Config Symlinks:"))
	err = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			realPath, err := os.Readlink(path)
			if err != nil {
				logger.Error("Error reading symlink", "path", path, "error", err)
				return nil
			}
			if filepath.Ext(realPath) == ".hcl" {
				fmt.Printf("   %s -> %s\n", symlinkStyle.Render(path), pathStyle.Render(realPath))
			}
		}
		return nil
	})
	if err != nil {
		logger.Error("Error walking through cache directory for symlinks", "error", err)
	}

	fmt.Println() // Add an extra newline for spacing
}

func handleConfigCommand(logger *log.Logger) {
	cacheDir, err := getCacheDir()
	if err != nil {
		logger.Error("Error getting cache directory", "error", err)
		return
	}

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FFFF"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00"))
	symlinkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00FF"))

	fmt.Println(titleStyle.Render("\n📂 Cache Directory:"))
	fmt.Printf("   %s\n\n", pathStyle.Render(fmt.Sprintf("cd %s", cacheDir)))

	fmt.Println(titleStyle.Render("📄 Gitspace Config Files:"))
	err = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".hcl" {
			fmt.Printf("   %s\n", pathStyle.Render(path))
		}
		return nil
	})
	if err != nil {
		logger.Error("Error walking through cache directory", "error", err)
	}

	fmt.Println(titleStyle.Render("\n🔗 Gitspace Config Symlinks:"))
	err = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			realPath, err := os.Readlink(path)
			if err != nil {
				logger.Error("Error reading symlink", "path", path, "error", err)
				return nil
			}
			if filepath.Ext(realPath) == ".hcl" {
				fmt.Printf("   %s -> %s\n", symlinkStyle.Render(path), pathStyle.Render(realPath))
			}
		}
		return nil
	})
	if err != nil {
		logger.Error("Error walking through cache directory for symlinks", "error", err)
	}

	fmt.Println() // Add an extra newline for spacing
}

func handleRepositoriesCommand(logger *log.Logger, config *Config) bool {
	for {
		var subChoice string
		err := huh.NewSelect[string]().
			Title("Choose a repositories action").
			Options(
				huh.NewOption("Clone", "clone"),
				huh.NewOption("Sync", "sync"),
				huh.NewOption("Go back", "back"),
				huh.NewOption("Quit", "quit"),
			).
			Value(&subChoice).
			Run()

		if err != nil {
			logger.Error("Error getting repositories sub-choice", "error", err)
			return false
		}

		switch subChoice {
		case "clone":
			cloneRepositories(logger, config)
		case "sync":
			syncRepositories(logger, config)
		case "back":
			return false // Go back to main menu
		case "quit":
			return true // Exit the program
		default:
			logger.Error("Invalid repositories sub-choice")
		}
	}
}

func syncRepositories(logger *log.Logger, config *Config) {
	logger.Info("Syncing repositories...")

	cacheDir, err := getCacheDir()
	if err != nil {
		logger.Error("Error getting cache directory", "error", err)
		return
	}

	repoDir := filepath.Join(cacheDir, ".repositories", config.Repositories.Clone.SCM, config.Repositories.Clone.Owner)

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

	// Get list of repositories to sync
	repos, err := lib.GetRepositories(config.Repositories.Clone.SCM, config.Repositories.Clone.Owner)
	if err != nil {
		logger.Error("Error fetching repositories", "error", err)
		return
	}

	// Filter repositories based on criteria
	filteredRepos := filterRepositories(repos, config)

	results := make(map[string]*RepoResult)

	for _, repo := range filteredRepos {
		repoPath := filepath.Join(repoDir, repo)
		result := &RepoResult{Name: repo}
		results[repo] = result

		// Check if the repository exists locally
		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			logger.Info("Repository not found locally, skipping", "repo", repo)
			continue
		}

		// Open the existing repository
		r, err := git.PlainOpen(repoPath)
		if err != nil {
			result.Error = err
			logger.Error("Failed to open existing repository", "repo", repo, "error", err)
			continue
		}

		// Fetch updates
		err = r.Fetch(&git.FetchOptions{
			Auth:     sshAuth,
			Progress: os.Stdout,
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			result.Error = err
			logger.Error("Fetch failed", "repo", repo, "error", err)
		} else {
			result.Updated = true
			logger.Info("Fetch successful", "repo", repo)
		}
	}

	err = updateIndexHCL(logger, config, results)
	if err != nil {
		logger.Error("Failed to update index.hcl", "error", err)
	}

	// Print summary table
	printSummaryTable(config, results, repoDir)
}

func cloneRepositories(logger *log.Logger, config *Config) {
	cacheDir, err := getCacheDir()
	if err != nil {
		logger.Error("Error getting cache directory", "error", err)
		return
	}

	baseDir := config.Repositories.GitSpace.Path
	repoDir := filepath.Join(cacheDir, ".repositories", config.Repositories.Clone.SCM, config.Repositories.Clone.Owner)
	err = os.MkdirAll(repoDir, 0755)
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

	// Clone or update repositories
	results := make(map[string]*RepoResult)
	boldStyle := lipgloss.NewStyle().Bold(true)

	for _, repo := range filteredRepos {
		repoPath := filepath.Join(repoDir, repo)
		result := &RepoResult{Name: repo}
		results[repo] = result

		fmt.Printf("%s\n", boldStyle.Render(fmt.Sprintf("Processing %s...", repo)))

		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			// Clone the repository if it doesn't exist
			_, err := git.PlainClone(repoPath, false, &git.CloneOptions{
				URL:      fmt.Sprintf("git@%s:%s/%s.git", config.Repositories.Clone.SCM, config.Repositories.Clone.Owner, repo),
				Progress: os.Stdout,
				Auth:     sshAuth,
			})
			if err != nil {
				result.Error = err
				fmt.Println(boldStyle.Render("Clone failed"))
			} else {
				result.Cloned = true
				fmt.Println(boldStyle.Render("Clone successful"))
			}
		} else {
			// Open the existing repository
			r, err := git.PlainOpen(repoPath)
			if err != nil {
				result.Error = err
				fmt.Println(boldStyle.Render("Failed to open existing repository"))
				continue
			}

			// Fetch updates
			err = r.Fetch(&git.FetchOptions{
				Auth:     sshAuth,
				Progress: os.Stdout,
			})
			if err != nil && err != git.NoErrAlreadyUpToDate {
				result.Error = err
				fmt.Println(boldStyle.Render("Fetch failed"))
			} else {
				result.Updated = true
				fmt.Println(boldStyle.Render("Fetch successful"))
			}
		}

		// Create local symlink
		localSymlinkPath := filepath.Join(baseDir, repo)
		err = createSymlink(repoPath, localSymlinkPath)
		if err != nil {
			logger.Error("Error creating local symlink", "repo", repo, "error", err)
		} else {
			result.LocalSymlink = localSymlinkPath
		}

		// Create global symlink
		globalSymlinkPath := filepath.Join(cacheDir, config.Repositories.Clone.SCM, config.Repositories.Clone.Owner, repo)
		err = createSymlink(repoPath, globalSymlinkPath)
		if err != nil {
			logger.Error("Error creating global symlink", "repo", repo, "error", err)
		} else {
			result.GlobalSymlink = globalSymlinkPath
		}

		fmt.Println() // Add a newline after each repository operation
	}

	err = updateIndexHCL(logger, config, results)
	if err != nil {
		logger.Error("Failed to update index.hcl", "error", err)
	}

	// Print summary table
	printSummaryTable(config, results, repoDir)
}

func createSymlink(source, target string) error {
	os.MkdirAll(filepath.Dir(target), 0755) // Ensure parent directory exists
	os.Remove(target)                       // Remove existing symlink if it exists
	return os.Symlink(source, target)
}

type RepoResult struct {
	Name          string
	Cloned        bool
	Updated       bool
	LocalSymlink  string
	GlobalSymlink string
	Error         error
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

func printVersionInfo(logger *log.Logger) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FFFF"))
	infoStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00"))

	// Local install information
	localVersion, localCommit := getCurrentVersion()

	fmt.Println(titleStyle.Render("\nLocal Install:"))
	fmt.Printf("Version: %s\n", infoStyle.Render(localVersion))
	if localCommit != "" {
		fmt.Printf("Commit Hash: %s\n", infoStyle.Render(localCommit))
	}

	// Remote/latest version information
	remoteRelease, err := lib.GetLatestGitHubRelease("ssotops", "gitspace")
	if err != nil {
		logger.Error("Error fetching remote version info", "error", err)
		return
	}

	fmt.Println(titleStyle.Render("\nRemote/Latest Version:"))
	fmt.Printf("Version: %s\n", infoStyle.Render(remoteRelease.TagName))
	fmt.Printf("Released: %s\n", infoStyle.Render(remoteRelease.PublishedAt.Format(time.RFC3339)))

	// Extract commit hash from release body if available
	commitHash := lib.ExtractCommitHash(remoteRelease.Body)
	if commitHash != "" {
		fmt.Printf("Commit Hash: %s\n", infoStyle.Render(commitHash))
	} else {
		fmt.Println("Commit Hash: Not available")
	}
}

func printSummaryTable(config *Config, results map[string]*RepoResult, repoDir string) {
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	fmt.Println(headerStyle.Render("\nRepository Processing Summary:"))
	fmt.Println() // Add a newline after the header

	// Define column styles
	repoStyle := lipgloss.NewStyle().Width(20).Align(lipgloss.Left)
	statusStyle := lipgloss.NewStyle().Width(15).Align(lipgloss.Left)
	localSymlinkStyle := lipgloss.NewStyle().Width(30).Align(lipgloss.Left)
	globalSymlinkStyle := lipgloss.NewStyle().Width(30).Align(lipgloss.Left)
	errorStyle := lipgloss.NewStyle().Width(30).Align(lipgloss.Left)

	// Print table header
	fmt.Printf("%s %s %s %s %s\n",
		repoStyle.Render("📁 Repository"),
		statusStyle.Render("✅ Status"),
		localSymlinkStyle.Render("🔗 Local Symlink"),
		globalSymlinkStyle.Render("🔗 Global Symlink"),
		errorStyle.Render("❌ Error"))

	fmt.Println(strings.Repeat("-", 125)) // Separator line

	// Print table rows
	for _, result := range results {
		status := "No changes"
		if result.Cloned {
			status = "Cloned"
		} else if result.Updated {
			status = "Updated"
		}

		localSymlink := "-"
		if result.LocalSymlink != "" {
			localSymlink = result.LocalSymlink
		}

		globalSymlink := "-"
		if result.GlobalSymlink != "" {
			globalSymlink = result.GlobalSymlink
		}

		errorMsg := "-"
		if result.Error != nil {
			errorMsg = result.Error.Error()
		}

		fmt.Printf("%s %s %s %s %s\n",
			repoStyle.Render(result.Name),
			statusStyle.Render(status),
			localSymlinkStyle.Render(localSymlink),
			globalSymlinkStyle.Render(globalSymlink),
			errorStyle.Render(errorMsg))
	}

	fmt.Println()
	fmt.Println(headerStyle.Render("Summary of changes:"))
	fmt.Println() // Add a newline after the header

	totalRepos := len(results)
	clonedRepos := 0
	updatedRepos := 0
	failedRepos := 0
	localSymlinks := 0
	globalSymlinks := 0

	for _, result := range results {
		if result.Cloned {
			clonedRepos++
		} else if result.Updated {
			updatedRepos++
		}
		if result.Error != nil {
			failedRepos++
		}
		if result.LocalSymlink != "" {
			localSymlinks++
		}
		if result.GlobalSymlink != "" {
			globalSymlinks++
		}
	}

	fmt.Printf("  Total repositories processed: %d\n", totalRepos)
	fmt.Printf("  Newly cloned: %d\n", clonedRepos)
	fmt.Printf("  Updated: %d\n", updatedRepos)
	fmt.Printf("  Failed operations: %d\n", failedRepos)
	fmt.Printf("  Local symlinks created: %d\n", localSymlinks)
	fmt.Printf("  Global symlinks created: %d\n", globalSymlinks)
}

func filterRepositories(repos []string, config *Config) []string {
	var filtered []string
	cloneConfig := config.Repositories.Clone

	for _, repo := range repos {
		// Check exact names
		if matchesFilter(repo, cloneConfig.IsExactly) { // Changed from Names to IsExactly
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
		if cloneConfig.IsExactly == nil && cloneConfig.StartsWith == nil && // Changed from Names to IsExactly
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

func getCurrentVersion() (string, string) {
	// Check if Version is set (injected during build)
	if Version != "" {
		return Version, ""
	}

	// Try to get the git commit hash
	hash, err := getGitCommitHash()
	if err == nil && hash != "" {
		return hash[:7], hash // Return first 7 characters as version, full hash as commit
	}

	// If git commit hash is not available, try to get it from build info
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value[:7], setting.Value
			}
		}
	}

	// If all else fails, return "unknown"
	return "unknown", ""
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
	applyLabelChanges(changes, logger, config.Repositories.Clone.Owner)
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
			if matchesFilter(repo, config.Repositories.Clone.IsExactly) { // Changed from Names to IsExactly
				changes[repo] = append(changes[repo], getLabelsFromFilter(config.Repositories.Clone.IsExactly)...)
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

func applyLabelChanges(changes map[string][]string, logger *log.Logger, owner string) {
	for repo, labels := range changes {
		err := lib.AddLabelsToRepository(owner, repo, labels)
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
		logger.Error("Failed to decode config", "diagnostics", decodeDiags.Error())
		return Config{}, fmt.Errorf("failed to decode HCL: %s", formatDiagnostics(decodeDiags))
	}

	logger.Info("Successfully decoded config")
	logger.Debug("Config decoding completed")
	return config, nil
}

func formatDiagnostics(diags hcl.Diagnostics) string {
	var messages []string
	for _, diag := range diags {
		severityStr := ""
		switch diag.Severity {
		case hcl.DiagError:
			severityStr = "Error"
		case hcl.DiagWarning:
			severityStr = "Warning"
		default:
			severityStr = "Unknown"
		}

		messages = append(messages, fmt.Sprintf("%s: %s at %s", severityStr, diag.Summary, diag.Subject))
		if diag.Detail != "" {
			messages = append(messages, fmt.Sprintf("  Detail: %s", diag.Detail))
		}
	}
	return strings.Join(messages, "\n")
}

func getCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	cacheDir := filepath.Join(homeDir, ".ssot", "gitspace")
	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}
	return cacheDir, nil
}

func handleSymlinksCommand(logger *log.Logger, config *Config) {
	for {
		var choice string
		err := huh.NewSelect[string]().
			Title("Choose a symlinks action").
			Options(
				huh.NewOption("Create local symlinks", "create_local"),
				huh.NewOption("Create global symlinks", "create_global"),
				huh.NewOption("Delete local symlinks", "delete_local"),
				huh.NewOption("Delete global symlinks", "delete_global"),
				huh.NewOption("Go back", "back"),
			).
			Value(&choice).
			Run()

		if err != nil {
			logger.Error("Error getting symlinks sub-choice", "error", err)
			return
		}

		switch choice {
		case "create_local":
			createLocalSymlinks(logger, config)
		case "create_global":
			createGlobalSymlinks(logger, config)
		case "delete_local":
			deleteLocalSymlinks(logger, config)
		case "delete_global":
			deleteGlobalSymlinks(logger, config)
		case "back":
			return
		default:
			logger.Error("Invalid symlinks sub-choice")
		}
	}
}

func createLocalSymlinks(logger *log.Logger, config *Config) {
	changes := make(map[string]string)
	baseDir := config.Repositories.GitSpace.Path
	repoDir := filepath.Join(getCacheDirOrDefault(logger), ".repositories", config.Repositories.Clone.SCM, config.Repositories.Clone.Owner)

	err := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() != filepath.Base(repoDir) {
			relPath, _ := filepath.Rel(repoDir, path)
			symlink := filepath.Join(baseDir, relPath) // Remove "gs" from this path
			err := os.MkdirAll(filepath.Dir(symlink), 0755)
			if err != nil {
				logger.Error("Error creating directory for local symlink", "path", symlink, "error", err)
				return nil
			}
			err = os.Symlink(path, symlink)
			if err != nil {
				logger.Error("Error creating local symlink", "path", path, "error", err)
			} else {
				changes[symlink] = path
			}
			return filepath.SkipDir // Skip subdirectories
		}
		return nil
	})

	if err != nil {
		logger.Error("Error walking through repository directory", "error", err)
	}

	printSymlinkSummary("Created local symlinks", changes)
}

func createGlobalSymlinks(logger *log.Logger, config *Config) {
	changes := make(map[string]string)
	globalDir, err := getGlobalSymlinkDir(config)
	if err != nil {
		logger.Error("Error getting global symlink directory", "error", err)
		return
	}
	repoDir := filepath.Join(getCacheDirOrDefault(logger), ".repositories", config.Repositories.Clone.SCM, config.Repositories.Clone.Owner)

	err = filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() != filepath.Base(repoDir) {
			relPath, _ := filepath.Rel(repoDir, path)
			symlink := filepath.Join(globalDir, relPath)
			err := os.MkdirAll(filepath.Dir(symlink), 0755)
			if err != nil {
				logger.Error("Error creating directory for global symlink", "path", symlink, "error", err)
				return nil
			}
			err = os.Symlink(path, symlink)
			if err != nil {
				logger.Error("Error creating global symlink", "path", path, "error", err)
			} else {
				changes[symlink] = path
			}
			return filepath.SkipDir // Skip subdirectories
		}
		return nil
	})

	if err != nil {
		logger.Error("Error walking through repository directory", "error", err)
	}

	printSymlinkSummary("Created global symlinks", changes)
}

func deleteLocalSymlinks(logger *log.Logger, config *Config) {
	changes := make(map[string]string)
	baseDir := config.Repositories.GitSpace.Path

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			realPath, _ := os.Readlink(path)
			err := os.Remove(path)
			if err != nil {
				logger.Error("Error deleting local symlink", "path", path, "error", err)
			} else {
				changes[path] = realPath
			}
		}
		return nil
	})

	if err != nil {
		logger.Error("Error walking through local directory", "error", err)
	}

	printSymlinkSummary("Deleted local symlinks", changes)
}

func deleteGlobalSymlinks(logger *log.Logger, config *Config) {
	changes := make(map[string]string)
	globalDir, err := getGlobalSymlinkDir(config)
	if err != nil {
		logger.Error("Error getting global symlink directory", "error", err)
		return
	}

	err = filepath.Walk(globalDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			realPath, _ := os.Readlink(path)
			err := os.Remove(path)
			if err != nil {
				logger.Error("Error deleting global symlink", "path", path, "error", err)
			} else {
				changes[path] = realPath
			}
		}
		return nil
	})

	if err != nil {
		logger.Error("Error walking through global directory", "error", err)
	}

	printSymlinkSummary("Deleted global symlinks", changes)
}

func getGlobalSymlinkDir(config *Config) (string, error) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return "", fmt.Errorf("failed to get cache directory: %w", err)
	}
	return filepath.Join(cacheDir, config.Repositories.Clone.SCM, config.Repositories.Clone.Owner), nil
}

func printSymlinkSummary(title string, changes map[string]string) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FFFF"))
	symlinkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00FF"))
	pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00"))

	fmt.Println(titleStyle.Render(fmt.Sprintf("\n%s Summary:", title)))
	if len(changes) == 0 {
		fmt.Println("No changes were made.")
	} else {
		for symlink, target := range changes {
			fmt.Printf("  %s -> %s\n", symlinkStyle.Render(symlink), pathStyle.Render(target))
		}
	}
	fmt.Printf("\nTotal changes: %d\n", len(changes))
}

func getCacheDirOrDefault(logger *log.Logger) string {
	cacheDir, err := getCacheDir()
	if err != nil {
		logger.Error("Error getting cache directory", "error", err)
		return filepath.Join(os.TempDir(), ".ssot", "gitspace")
	}
	return cacheDir
}

func getRepoType(config *Config, repo string) string {
	if matchesFilter(repo, config.Repositories.Clone.StartsWith) {
		return config.Repositories.Clone.StartsWith.Repository.Type
	}
	if matchesFilter(repo, config.Repositories.Clone.EndsWith) {
		return config.Repositories.Clone.EndsWith.Repository.Type
	}
	if matchesFilter(repo, config.Repositories.Clone.Includes) {
		return config.Repositories.Clone.Includes.Repository.Type
	}
	if matchesFilter(repo, config.Repositories.Clone.IsExactly) {
		return config.Repositories.Clone.IsExactly.Repository.Type
	}
	return "default" // or any default type you prefer
}

func getRepoLabels(config *Config, repo string) []string {
	var labels []string
	labels = append(labels, config.Repositories.Labels...)

	if matchesFilter(repo, config.Repositories.Clone.StartsWith) {
		labels = append(labels, config.Repositories.Clone.StartsWith.Repository.Labels...)
	}
	if matchesFilter(repo, config.Repositories.Clone.EndsWith) {
		labels = append(labels, config.Repositories.Clone.EndsWith.Repository.Labels...)
	}
	if matchesFilter(repo, config.Repositories.Clone.Includes) {
		labels = append(labels, config.Repositories.Clone.Includes.Repository.Labels...)
	}
	if matchesFilter(repo, config.Repositories.Clone.IsExactly) {
		labels = append(labels, config.Repositories.Clone.IsExactly.Repository.Labels...)
	}

	return removeDuplicates(labels)
}
