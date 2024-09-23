package main

import (
	"fmt"
	"os"
	"path/filepath"

  "github.com/ssotops/gitspace/logger"
)

func createLocalSymlinks(logger *logger.RateLimitedLogger, config *Config) {
	changes := make(map[string]string)
	baseDir := config.Global.Path
	repoDir := filepath.Join(getCacheDirOrDefault(logger), ".repositories", config.Global.SCM, config.Global.Owner)

	err := filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && info.Name() != filepath.Base(repoDir) {
			relPath, _ := filepath.Rel(repoDir, path)
			symlink := filepath.Join(baseDir, relPath)
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

func createGlobalSymlinks(logger *logger.RateLimitedLogger, config *Config) {
	changes := make(map[string]string)
	globalDir, err := getGlobalSymlinkDir(config)
	if err != nil {
		logger.Error("Error getting global symlink directory", "error", err)
		return
	}
	repoDir := filepath.Join(getCacheDirOrDefault(logger), ".repositories", config.Global.SCM, config.Global.Owner)

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

func deleteLocalSymlinks(logger *logger.RateLimitedLogger, config *Config) {
	changes := make(map[string]string)
	baseDir := config.Global.Path

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

func deleteGlobalSymlinks(logger *logger.RateLimitedLogger, config *Config) {
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
	return filepath.Join(cacheDir, config.Global.SCM, config.Global.Owner), nil
}

func createSymlink(source, target string) error {
	os.MkdirAll(filepath.Dir(target), 0755) // Ensure parent directory exists
	os.Remove(target)                       // Remove existing symlink if it exists
	return os.Symlink(source, target)
}
