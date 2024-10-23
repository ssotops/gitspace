package plugin

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
)

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
