package lib

import (
	"context"
	"fmt"
)

type SCMType string

const (
	SCMTypeGitHub SCMType = "github"
	SCMTypeGitea  SCMType = "gitea"
)

func GetSCMProvider(scmType SCMType, baseURL string) (SCMProvider, error) {
	switch scmType {
	case SCMTypeGitHub:
		return NewGitHubProvider()
	case SCMTypeGitea:
		return NewGiteaProvider(baseURL)
	default:
		return nil, fmt.Errorf("unsupported SCM type: %s", scmType)
	}
}

// Wrapper functions to maintain compatibility with existing code

func GetLatestRelease(ctx context.Context, scmType SCMType, baseURL, owner, repo string) (*Release, error) {
	provider, err := GetSCMProvider(scmType, baseURL)
	if err != nil {
		return nil, err
	}
	return provider.GetLatestRelease(ctx, owner, repo)
}

func GetRepositories(ctx context.Context, scmType SCMType, baseURL, owner string) ([]string, error) {
	provider, err := GetSCMProvider(scmType, baseURL)
	if err != nil {
		return nil, err
	}
	return provider.FetchRepositories(ctx, owner)
}

func FetchGitspaceCatalog(ctx context.Context, scmType SCMType, baseURL, owner, repo string) (*Catalog, error) {
	provider, err := GetSCMProvider(scmType, baseURL)
	if err != nil {
		return nil, err
	}
	return provider.FetchCatalog(ctx, owner, repo)
}

func DownloadDirectory(ctx context.Context, scmType SCMType, baseURL, owner, repo, path, destDir string) error {
	provider, err := GetSCMProvider(scmType, baseURL)
	if err != nil {
		return err
	}
	return provider.DownloadDirectory(ctx, owner, repo, path, destDir)
}
