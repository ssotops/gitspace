package main

import (
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
)

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

func getCacheDirOrDefault(logger *log.Logger) string {
	cacheDir, err := getCacheDir()
	if err != nil {
		logger.Error("Error getting cache directory", "error", err)
		return filepath.Join(os.TempDir(), ".ssot", "gitspace")
	}
	return cacheDir
}
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
