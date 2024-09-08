package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/mitchellh/go-homedir"
	"github.com/pelletier/go-toml/v2"
	"github.com/ssotspace/gitspace/lib"
)

type RepoResult struct {
	Name          string
	Cloned        bool
	Updated       bool
	LocalSymlink  string
	GlobalSymlink string
	Error         error
}

func cloneRepositories(logger *log.Logger, config *Config) {
	cacheDir, err := getCacheDir()
	if err != nil {
		logger.Error("Error getting cache directory", "error", err)
		return
	}

	baseDir := config.Global.Path
	repoDir := filepath.Join(cacheDir, ".repositories", config.Global.SCM, config.Global.Owner)
	err = os.MkdirAll(repoDir, 0755)
	if err != nil {
		logger.Error("Error creating directories", "error", err)
		return
	}

	// Setup SSH auth
	sshKeyPath, err := getSSHKeyPath(config.Auth.KeyPath)
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

	// Get list of repositories to clone
	repos, err := lib.GetRepositories(config.Global.SCM, config.Global.Owner)
	if err != nil {
		logger.Error("Error fetching repositories", "error", err)
		return
	}

	// Filter repositories based on criteria
	filteredRepos := filterRepositories(repos, config)

	if len(filteredRepos) == 0 {
		logger.Warn("No repositories match the filter criteria")
		return
	}

	// Clone or update repositories
	results := make(map[string]*RepoResult)

	for _, repo := range filteredRepos {
		repoPath := filepath.Join(repoDir, repo)
		result := &RepoResult{Name: repo}
		results[repo] = result

		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			// Clone the repository if it doesn't exist
			_, err := git.PlainClone(repoPath, false, &git.CloneOptions{
				URL:      fmt.Sprintf("git@%s:%s/%s.git", config.Global.SCM, config.Global.Owner, repo),
				Progress: os.Stdout,
				Auth:     sshAuth,
			})
			if err != nil {
				if strings.Contains(err.Error(), "remote repository is empty") {
					result.Cloned = true
					logger.Info("Cloned empty repository", "repo", repo)
				} else {
					result.Error = err
					logger.Error("Clone failed", "repo", repo, "error", err)
				}
			} else {
				result.Cloned = true
				logger.Info("Clone successful", "repo", repo)
			}
		} else {
			// Update existing repository
			r, err := git.PlainOpen(repoPath)
			if err != nil {
				result.Error = err
				logger.Error("Failed to open existing repository", "repo", repo, "error", err)
				continue
			}

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

		// Create local symlink
		localSymlinkPath := filepath.Join(baseDir, repo)
		err = createSymlink(repoPath, localSymlinkPath)
		if err != nil {
			logger.Error("Error creating local symlink", "repo", repo, "error", err)
		} else {
			result.LocalSymlink = localSymlinkPath
		}

		// Create global symlink
		globalSymlinkPath := filepath.Join(cacheDir, config.Global.SCM, config.Global.Owner, repo)
		err = createSymlink(repoPath, globalSymlinkPath)
		if err != nil {
			logger.Error("Error creating global symlink", "repo", repo, "error", err)
		} else {
			result.GlobalSymlink = globalSymlinkPath
		}
	}

	err = updateIndexTOML(logger, config, results)
	if err != nil {
		logger.Error("Failed to update index.toml", "error", err)
	}

	// Print summary table
	printSummaryTable(config, results, repoDir)
}

func updateIndexTOML(logger *log.Logger, config *Config, repoResults map[string]*RepoResult) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return fmt.Errorf("failed to get cache directory: %w", err)
	}

	indexPath := filepath.Join(cacheDir, "index.toml")
	configsDir := filepath.Join(cacheDir, ".configs")

	// Ensure .configs directory exists
	if err := os.MkdirAll(configsDir, 0755); err != nil {
		return fmt.Errorf("failed to create .configs directory: %w", err)
	}

	// Create a new TOML tree
	indexData := make(map[string]interface{})

	// Add lastUpdated
	now := time.Now()
	indexData["lastUpdated"] = now.Format(time.RFC3339)

	// Create repositories section
	repositories := make(map[string]interface{})
	scm := make(map[string]interface{})
	owners := make(map[string]interface{})
	repos := make(map[string]interface{})

	// Get the current working directory
	pwd, err := os.Getwd()
	if err != nil {
		logger.Error("Failed to get current working directory", "error", err)
		pwd = "" // Use empty string if we can't get the current directory
	}

	// Read the original config file once
	originalConfigPath := filepath.Join(pwd, "gs.toml") // Assuming the config file is always named gs.toml
	originalConfigContent, err := os.ReadFile(originalConfigPath)
	if err != nil {
		logger.Error("Failed to read original config file", "path", originalConfigPath, "error", err)
		// Continue execution, but log the error
	}

	// Create a single backup file for all repositories
	backupFileName := fmt.Sprintf("%s_%s_%s.toml", config.Global.SCM, config.Global.Owner, now.Format("20060102_150405"))
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
		repoData := make(map[string]interface{})
		repoData["configPath"] = originalConfigPath
		repoData["backupPath"] = backupPath

		if result.Cloned {
			repoData["lastCloned"] = now.Format(time.RFC3339)
		}
		if result.Updated {
			repoData["lastSynced"] = now.Format(time.RFC3339)
		}

		// Add repository type
		repoType := getRepoType(config, repo)
		repoData["type"] = repoType

		// Add metadata
		metadata := make(map[string]interface{})

		// Set url (formerly URI)
		url := fmt.Sprintf("https://%s/%s/%s", config.Global.SCM, config.Global.Owner, repo)
		metadata["url"] = url

		// Set labels (lowercase)
		labels := getRepoLabels(config, repo)
		lowercaseLabels := make([]string, len(labels))
		for i, label := range labels {
			lowercaseLabels[i] = strings.ToLower(label)
		}
		metadata["labels"] = lowercaseLabels

		repoData["metadata"] = metadata
		repos[repo] = repoData
	}

	owners[config.Global.Owner] = repos
	scm[config.Global.SCM] = owners
	repositories["repositories"] = scm
	indexData["repositories"] = repositories

	// Write updated index.toml
	f, err := os.Create(indexPath)
	if err != nil {
		return fmt.Errorf("failed to create index.toml: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	err = encoder.Encode(indexData)
	if err != nil {
		return fmt.Errorf("failed to encode TOML: %w", err)
	}

	logger.Info("Successfully updated index.toml")
	return nil
}

func syncRepositories(logger *log.Logger, config *Config) {
	logger.Info("Syncing repositories...")

	if config == nil || config.Global.SCM == "" || config.Global.Owner == "" {
		logger.Error("No valid config loaded. Please load a config file first.")
		return
	}

	cacheDir, err := getCacheDir()
	if err != nil {
		logger.Error("Error getting cache directory", "error", err)
		return
	}

	repoDir := filepath.Join(cacheDir, ".repositories", config.Global.SCM, config.Global.Owner)
	baseDir := config.Global.Path

	// Setup SSH auth
	sshKeyPath, err := getSSHKeyPath(config.Auth.KeyPath)
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
	repos, err := lib.GetRepositories(config.Global.SCM, config.Global.Owner)
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

		// Create local symlink
		localSymlinkPath := filepath.Join(baseDir, repo)
		err = createSymlink(repoPath, localSymlinkPath)
		if err != nil {
			logger.Error("Error creating local symlink", "repo", repo, "error", err)
		} else {
			result.LocalSymlink = localSymlinkPath
		}

		// Create global symlink
		globalSymlinkPath := filepath.Join(cacheDir, config.Global.SCM, config.Global.Owner, repo)
		err = createSymlink(repoPath, globalSymlinkPath)
		if err != nil {
			logger.Error("Error creating global symlink", "repo", repo, "error", err)
		} else {
			result.GlobalSymlink = globalSymlinkPath
		}
	}

	err = updateIndexTOML(logger, config, results)
	if err != nil {
		logger.Error("Failed to update index.toml", "error", err)
	}

	// Print summary table
	printSummaryTable(config, results, repoDir)
}

func getRepoType(config *Config, repo string) string {
	for _, group := range config.Groups {
		if matchesFilter(repo, group) && group.Type != "" {
			return group.Type
		}
	}
	return "default"
}

func getRepoLabels(config *Config, repo string) []string {
	var labels []string
	labels = append(labels, config.Global.Labels...)

	for _, group := range config.Groups {
		if matchesFilter(repo, group) {
			labels = append(labels, group.Labels...)
		}
	}

	return removeDuplicates(labels)
}

func filterRepositories(repos []string, config *Config) []string {
	var filtered []string

	for _, repo := range repos {
		for _, group := range config.Groups {
			if matchesFilter(repo, group) {
				filtered = append(filtered, repo)
				break
			}
		}
	}

	return filtered
}

func matchesGroup(repo string, group Group) bool {
	switch group.Match {
	case "startsWith":
		for _, value := range group.Values {
			if strings.HasPrefix(repo, value) {
				return true
			}
		}
	case "endsWith":
		for _, value := range group.Values {
			if strings.HasSuffix(repo, value) {
				return true
			}
		}
	case "includes":
		for _, value := range group.Values {
			if strings.Contains(repo, value) {
				return true
			}
		}
	case "isExactly":
		for _, value := range group.Values {
			if repo == value {
				return true
			}
		}
	}
	return false
}

func matchesFilter(repo string, group Group) bool {
	switch group.Match {
	case "startsWith":
		for _, value := range group.Values {
			if strings.HasPrefix(strings.ToLower(repo), strings.ToLower(value)) {
				return true
			}
		}
	case "endsWith":
		for _, value := range group.Values {
			if strings.HasSuffix(strings.ToLower(repo), strings.ToLower(value)) {
				return true
			}
		}
	case "includes":
		for _, value := range group.Values {
			if strings.Contains(strings.ToLower(repo), strings.ToLower(value)) {
				return true
			}
		}
	case "isExactly":
		for _, value := range group.Values {
			if strings.EqualFold(repo, value) {
				return true
			}
		}
	}
	return false
}
