package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
  "runtime/debug"
	"strings"

	"github.com/go-git/go-git/v5"
  "github.com/ssotops/gitspace/logger"
)

type ReleaseInfo struct {
	TagName string `json:"tag_name"`
	ID      int    `json:"id"`
}

var Version string

func getCurrentVersion() (string, string) {
	if Version != "" {
		return Version, ""
	}

	hash, err := getGitCommitHash()
	if err == nil && hash != "" {
		return hash[:7], hash
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value[:7], setting.Value
			}
		}
	}

	return "unknown", ""
}

func getGitCommitHash() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output)), nil
	}

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

func upgradeGitspace(logger *logger.RateLimitedLogger) {
	logger.Info("Upgrading Gitspace...")

	repo := "ssotops/gitspace"
	binary := "gitspace"

	osName := runtime.GOOS
	arch := runtime.GOARCH

	releaseInfo, err := fetchLatestReleaseInfo(repo)
	if err != nil {
		logger.Error("Failed to fetch latest release information", "error", err)
		return
	}

	version := releaseInfo.TagName
	logger.Info("Latest version", "version", version)

	assetName := fmt.Sprintf("%s_%s_%s", binary, osName, arch)
	if osName == "windows" {
		assetName += ".exe"
	}
	downloadURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, version, assetName)

	tempFile, err := downloadBinary(downloadURL)
	if err != nil {
		logger.Error("Failed to download binary", "error", err)
		return
	}
	defer os.Remove(tempFile)

	if osName != "windows" {
		err = os.Chmod(tempFile, 0755)
		if err != nil {
			logger.Error("Failed to make binary executable", "error", err)
			return
		}
	}

	execPath, err := os.Executable()
	if err != nil {
		logger.Error("Failed to get current executable path", "error", err)
		return
	}

	err = os.Rename(tempFile, execPath)
	if err != nil {
		logger.Error("Failed to replace current binary", "error", err)
		return
	}

	logger.Info("Gitspace has been successfully upgraded!", "version", version)
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

func printVersionInfo(logger *logger.RateLimitedLogger) {
    version, commitHash := getCurrentVersion()
    logger.Info("Current version", "version", version)
    if commitHash != "" {
        logger.Info("Commit hash", "hash", commitHash)
    }

    releaseInfo, err := fetchLatestReleaseInfo("ssotops/gitspace")
    if err != nil {
        logger.Error("Failed to fetch latest release information", "error", err)
        return
    }

    logger.Info("Latest version", "version", releaseInfo.TagName)
}
