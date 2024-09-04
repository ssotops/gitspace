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
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mitchellh/go-homedir"
	"github.com/ssotspace/gitspace/lib"
	"github.com/zclconf/go-cty/cty"
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

	// Get list of repositories to clone
	repos, err := lib.GetRepositories(config.Repositories.Clone.SCM, config.Repositories.Clone.Owner)
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
				URL:      fmt.Sprintf("git@%s:%s/%s.git", config.Repositories.Clone.SCM, config.Repositories.Clone.Owner, repo),
				Progress: os.Stdout,
				Auth:     sshAuth,
			})
			if err != nil {
				result.Error = err
				logger.Error("Clone failed", "repo", repo, "error", err)
			} else {
				result.Cloned = true
				logger.Info("Clone successful", "repo", repo)
			}
		} else {
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
	}

	err = updateIndexHCL(logger, config, results)
	if err != nil {
		logger.Error("Failed to update index.hcl", "error", err)
	}

	// Print summary table
	printSummaryTable(config, results, repoDir)
}

func syncRepositories(logger *log.Logger, config *Config) {
	logger.Info("Syncing repositories...")

	if config == nil || config.Repositories == nil || config.Repositories.Clone == nil {
		logger.Error("No valid config loaded. Please load a config file first.")
		return
	}

	cacheDir, err := getCacheDir()
	if err != nil {
		logger.Error("Error getting cache directory", "error", err)
		return
	}

	repoDir := filepath.Join(cacheDir, ".repositories", config.Repositories.Clone.SCM, config.Repositories.Clone.Owner)
	baseDir := config.Repositories.GitSpace.Path

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
	}

	err = updateIndexHCL(logger, config, results)
	if err != nil {
		logger.Error("Failed to update index.hcl", "error", err)
	}

	// Print summary table
	printSummaryTable(config, results, repoDir)
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
	return "default"
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
			labelsVal[i] = cty.StringVal(strings.ToLower(label))
		}
		metadataBody.SetAttributeValue("labels", cty.ListVal(labelsVal))
	}

	// Write updated index.hcl
	indexContent := f.Bytes()
	if err := os.WriteFile(indexPath, indexContent, 0644); err != nil {
		return fmt.Errorf("failed to write index.hcl: %w", err)
	}

	logger.Info("Successfully updated index.hcl")
	return nil
}

func filterRepositories(repos []string, config *Config) []string {
	var filtered []string
	cloneConfig := config.Repositories.Clone

	for _, repo := range repos {
		if matchesFilter(repo, cloneConfig.IsExactly) {
			filtered = append(filtered, repo)
			continue
		}

		if matchesFilter(repo, cloneConfig.StartsWith) {
			filtered = append(filtered, repo)
			continue
		}

		if matchesFilter(repo, cloneConfig.EndsWith) {
			filtered = append(filtered, repo)
			continue
		}

		if matchesFilter(repo, cloneConfig.Includes) {
			filtered = append(filtered, repo)
			continue
		}

		if cloneConfig.IsExactly == nil && cloneConfig.StartsWith == nil &&
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
