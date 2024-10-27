// plugin/utils.go

package plugin

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/ssotops/gitspace-plugin-sdk/logger"
)

// NewBufferedWriteCloser creates a new buffered write closer with a logger
func NewBufferedWriteCloser(w io.WriteCloser, l *logger.RateLimitedLogger) *bufferedWriteCloser {
	return &bufferedWriteCloser{
		Writer: bufio.NewWriterSize(w, 1024*1024), // 1MB buffer
		closer: w,
		mu:     sync.Mutex{},
		closed: false,
		logger: l,
	}
}

// getPluginsDir returns the path to the plugins directory and ensures it exists
func getPluginsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}
	pluginsDir := filepath.Join(homeDir, ".ssot", "gitspace", "plugins")

	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create plugins directory: %w", err)
	}

	return pluginsDir, nil
}

// gitClone clones a git repository to the specified destination path
func gitClone(url, destPath string) error {
	cmd := exec.Command("git", "clone", url, destPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// createSymlink creates a symlink at target pointing to source
func createSymlink(source, target string) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory for symlink: %w", err)
	}

	// Remove existing symlink if it exists
	os.Remove(target)

	// Create new symlink
	return os.Symlink(source, target)
}

// removeDuplicates removes duplicate strings from a slice while preserving order
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

// ensureDirectory ensures a directory exists with the correct permissions
func ensureDirectory(path string, mode os.FileMode) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

// fileExists checks if a file exists and is not a directory
func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// dirExists checks if a directory exists
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func copyDir(src string, dst string, log *logger.RateLimitedLogger) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Error("Error walking directory",
				"path", path,
				"error", err)
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			log.Error("Error getting relative path",
				"source", src,
				"path", path,
				"error", err)
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			log.Debug("Creating directory",
				"path", dstPath)
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath, log)
	})
}

func copyFile(src, dst string, log *logger.RateLimitedLogger) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		log.Error("Error getting file stats",
			"source", src,
			"error", err)
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		log.Error("Source is not a regular file",
			"source", src,
			"mode", sourceFileStat.Mode())
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		log.Error("Error opening source file",
			"source", src,
			"error", err)
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		log.Error("Error creating destination file",
			"destination", dst,
			"error", err)
		return err
	}
	defer destination.Close()

	log.Debug("Copying file",
		"source", src,
		"destination", dst)
	_, err = io.Copy(destination, source)
	if err != nil {
		log.Error("Error copying file",
			"source", src,
			"destination", dst,
			"error", err)
		return err
	}

	err = os.Chmod(dst, sourceFileStat.Mode())
	if err != nil {
		log.Error("Error setting file permissions",
			"destination", dst,
			"mode", sourceFileStat.Mode(),
			"error", err)
		return err
	}

	log.Debug("File copied successfully",
		"source", src,
		"destination", dst)
	return nil
}
