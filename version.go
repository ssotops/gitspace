package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/go-git/go-git/v5"
	"github.com/ssotspace/gitspace/lib"
)

var Version string

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
	releaseInfo, err := lib.GetLatestGitHubRelease("ssotops", "gitspace")
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
