package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/mitchellh/go-homedir"
	"github.com/pelletier/go-toml/v2"
	"github.com/ssotops/gitspace-plugin-sdk/logger"
	"github.com/ssotops/gitspace/lib"
	gossh "golang.org/x/crypto/ssh" // Add this import
)

type RepoResult struct {
	Name          string
	Cloned        bool
	Updated       bool
	LocalSymlink  string
	GlobalSymlink string
	Error         error
}

func cloneRepositories(logger *logger.RateLimitedLogger, config *Config) {
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

	// Check for appropriate authentication based on SCM type
	switch lib.SCMType(config.Global.SCM) {
	case lib.SCMTypeGitHub:
		if os.Getenv("GITHUB_TOKEN") == "" {
			logger.Error("GITHUB_TOKEN environment variable not set. Please set it and try again.")
			return
		}
	case lib.SCMTypeGitea:
		// For Gitea, we're using SSH authentication, so we don't need to check for a token
		// However, we might want to verify the SSH key exists
		if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
			logger.Error("SSH key not found. Please ensure the key exists at the specified path.", "path", sshKeyPath)
			return
		}
	default:
		logger.Error("Unsupported SCM type", "type", config.Global.SCM)
		return
	}

	// Get list of repositories to clone
	ctx := context.Background()
	repos, err := lib.GetRepositories(ctx, lib.SCMType(config.Global.SCM), config.Global.BaseURL, config.Global.Owner)
	if err != nil {
		logger.Error("Error fetching repositories", "error", err)
		return
	}

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
			err := cloneRepo(repoPath, config.Global.SCM, config.Global.Owner, repo, sshAuth, sshKeyPath, config.Global.EmptyRepoInitialBranch, logger)
			if err != nil {
				result.Error = err
				logger.Error("Clone failed", "repo", repo, "error", err)
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

// func cloneRepo(repoPath, scm, owner, repo string, sshAuth *ssh.PublicKeys, sshKeyPath, initialBranch string, logger *logger.RateLimitedLogger) error {
// 	repoURL := fmt.Sprintf("https://%s/%s/%s.git", scm, owner, repo)

// 	cloneOptions := &git.CloneOptions{
// 		URL:      repoURL,
// 		Progress: os.Stdout,
// 	}

// 	if sshAuth != nil {
// 		repoURL = fmt.Sprintf("git@%s:%s/%s.git", scm, owner, repo)
// 		cloneOptions.URL = repoURL
// 		cloneOptions.Auth = sshAuth
// 	}

// 	// First, try to clone normally
// 	_, err := git.PlainClone(repoPath, false, cloneOptions)

// 	if err != nil {
// 		if strings.Contains(err.Error(), "remote repository is empty") {
// 			// If the repository is empty, use Git commands to clone it
// 			logger.Info("Cloning empty repository", "repo", repo)
// 			return cloneEmptyRepo(repoPath, repoURL, sshKeyPath, initialBranch, logger)
// 		}
// 		return err
// 	}

// 	return nil
// }

func cloneRepo(repoPath, scm, owner, repo string, sshAuth *ssh.PublicKeys, sshKeyPath, initialBranch string, logger *logger.RateLimitedLogger) error {
	repoURL := fmt.Sprintf("ssh://scmtea/%s/%s.git", owner, repo)
	logger.Debug("Cloning repo", "url", repoURL, "path", repoPath)

	// Create a custom HostKeyCallback function that always returns nil
	sshAuth.HostKeyCallback = func(hostname string, remote net.Addr, key gossh.PublicKey) error {
		return nil
	}

	cloneOptions := &git.CloneOptions{
		URL:      repoURL,
		Progress: os.Stdout,
		Auth:     sshAuth,
	}

	_, err := git.PlainClone(repoPath, false, cloneOptions)
	if err != nil {
		logger.Error("Clone failed", "error", err, "url", repoURL)
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	return nil
}

func cloneEmptyRepo(repoPath, repoURL, sshKeyPath, initialBranch string, logger *logger.RateLimitedLogger) error {
	// Create the repository directory
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return fmt.Errorf("failed to create repository directory: %w", err)
	}

	// Initialize the repository
	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to initialize repository: %w, output: %s", err, output)
	}

	// Add the remote
	cmd = exec.Command("git", "remote", "add", "origin", repoURL)
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add remote: %w, output: %s", err, output)
	}

	// Create and checkout the initial branch
	cmd = exec.Command("git", "checkout", "-b", initialBranch)
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create and checkout initial branch: %w, output: %s", err, output)
	}

	// Create an initial empty commit
	cmd = exec.Command("git", "commit", "--allow-empty", "-m", "Initial empty commit")
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create initial commit: %w, output: %s", err, output)
	}

	// Push the initial commit
	cmd = exec.Command("git", "push", "-u", "origin", initialBranch)
	cmd.Dir = repoPath
	cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s", sshKeyPath))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to push initial commit: %w, output: %s", err, output)
	}

	return nil
}

func updateIndexTOML(logger *logger.RateLimitedLogger, config *Config, repoResults map[string]*RepoResult) error {
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

func syncRepositories(logger *logger.RateLimitedLogger, config *Config) {
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
	ctx := context.Background()
	repos, err := lib.GetRepositories(ctx, lib.SCMType(config.Global.SCM), "", config.Global.Owner)
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
			fmt.Printf("DEBUG: Matched repo '%s' to type '%s'\n", repo, group.Type)
			return group.Type
		}
	}
	fmt.Printf("DEBUG: No specific type found for repo '%s', using default\n", repo)
	return "default"
}

func filterRepositories(repos []string, config *Config) []string {
	var filtered []string

	fmt.Printf("DEBUG: Filtering %d repositories\n", len(repos))
	fmt.Printf("DEBUG: Config: %+v\n", config)

	for _, repo := range repos {
		fmt.Printf("DEBUG: Checking repo: %s\n", repo)
		for groupName, group := range config.Groups {
			fmt.Printf("DEBUG: Against group '%s': %+v\n", groupName, group)
			if matchesFilter(repo, group) {
				fmt.Printf("DEBUG: MATCH - Adding repo '%s' to filtered list\n", repo)
				filtered = append(filtered, repo)
				break
			}
		}
	}

	fmt.Printf("DEBUG: Filtered repositories: %v\n", filtered)
	return filtered
}

func matchesFilter(repo string, group Group) bool {
	fmt.Printf("DEBUG: Matching repo '%s' against group: %+v\n", repo, group)
	switch group.Match {
	case "endsWith":
		for _, value := range group.Values {
			fmt.Printf("DEBUG: Checking if '%s' ends with '%s'\n", repo, value)
			repoLower := strings.ToLower(repo)
			valueLower := strings.ToLower(value)
			if strings.HasSuffix(repoLower, valueLower) {
				fmt.Printf("DEBUG: MATCH FOUND - repo '%s' ends with '%s'\n", repo, value)
				return true
			}
			// Check if the repo name ends with the value followed by a hyphen and any characters
			if strings.HasSuffix(repoLower, valueLower+"-") || strings.Contains(repoLower, valueLower+"-") {
				fmt.Printf("DEBUG: MATCH FOUND - repo '%s' contains '%s-'\n", repo, value)
				return true
			}
			fmt.Printf("DEBUG: NO MATCH - repo '%s' does not end with or contain '%s-'\n", repo, value)
		}
	case "startsWith":
		for _, value := range group.Values {
			if strings.HasPrefix(strings.ToLower(repo), strings.ToLower(value)) {
				fmt.Printf("DEBUG: MATCH FOUND - repo '%s' starts with '%s'\n", repo, value)
				return true
			}
		}
	case "includes":
		for _, value := range group.Values {
			if strings.Contains(strings.ToLower(repo), strings.ToLower(value)) {
				fmt.Printf("DEBUG: MATCH FOUND - repo '%s' includes '%s'\n", repo, value)
				return true
			}
		}
	case "isExactly":
		for _, value := range group.Values {
			if strings.EqualFold(repo, value) {
				fmt.Printf("DEBUG: MATCH FOUND - repo '%s' is exactly '%s'\n", repo, value)
				return true
			}
		}
	}
	fmt.Printf("DEBUG: No match found for repo '%s'\n", repo)
	return false
}
