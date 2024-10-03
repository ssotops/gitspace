// lib/gitea.go

package lib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/sdk/gitea"
	"github.com/pelletier/go-toml/v2"
)

type GiteaProvider struct {
	client *gitea.Client
}

func NewGiteaProvider(baseURL string) (*GiteaProvider, error) {
	token := os.Getenv("GITEA_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITEA_TOKEN environment variable not set")
	}

	client, err := gitea.NewClient(baseURL, gitea.SetToken(token))
	if err != nil {
		return nil, fmt.Errorf("error creating Gitea client: %v", err)
	}

	return &GiteaProvider{client: client}, nil
}

func (g *GiteaProvider) GetLatestRelease(ctx context.Context, owner, repo string) (*Release, error) {
	releases, _, err := g.client.ListReleases(owner, repo, gitea.ListReleasesOptions{})
	if err != nil {
		return nil, err
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}

	latestRelease := releases[0]
	return &Release{
		TagName:     latestRelease.TagName,
		PublishedAt: latestRelease.PublishedAt,
		Body:        latestRelease.Note,
	}, nil
}

func (g *GiteaProvider) FetchRepositories(ctx context.Context, owner string) ([]string, error) {
	var allRepos []string
	page := 1
	perPage := 50

	for {
		repos, _, err := g.client.ListOrgRepos(owner, gitea.ListOrgReposOptions{
			ListOptions: gitea.ListOptions{
				Page:     page,
				PageSize: perPage,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("error fetching repositories: %v", err)
		}

		for _, repo := range repos {
			allRepos = append(allRepos, repo.Name)
		}

		if len(repos) < perPage {
			break
		}
		page++
	}

	return allRepos, nil
}

func (g *GiteaProvider) FetchCatalog(ctx context.Context, owner, repo string) (*Catalog, error) {
	fileContent, _, err := g.client.GetFile(owner, repo, "master", "gitspace-catalog.toml")
	if err != nil {
		return nil, fmt.Errorf("error fetching gitspace-catalog.toml: %v", err)
	}

	var catalog Catalog
	err = toml.Unmarshal(fileContent, &catalog)
	if err != nil {
		return nil, fmt.Errorf("error decoding TOML: %v", err)
	}

	return &catalog, nil
}

func (g *GiteaProvider) DownloadDirectory(ctx context.Context, owner, repo, path, destDir string) error {
	tree, _, err := g.client.GetTrees(owner, repo, "master", true)
	if err != nil {
		return fmt.Errorf("error fetching repository tree: %v", err)
	}

	for _, entry := range tree.Entries {
		if entry.Type == "tree" {
			continue
		}

		if !strings.HasPrefix(entry.Path, path) {
			continue
		}

		fileContent, _, err := g.client.GetFile(owner, repo, "master", entry.Path)
		if err != nil {
			return fmt.Errorf("error fetching file content: %v", err)
		}

		filePath := filepath.Join(destDir, strings.TrimPrefix(entry.Path, path))
		err = os.MkdirAll(filepath.Dir(filePath), 0755)
		if err != nil {
			return fmt.Errorf("error creating directories: %v", err)
		}

		err = os.WriteFile(filePath, fileContent, 0644)
		if err != nil {
			return fmt.Errorf("error writing file: %v", err)
		}
	}

	return nil
}
