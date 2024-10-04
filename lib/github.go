package lib

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-github/v39/github"
	"github.com/pelletier/go-toml/v2"
	"golang.org/x/oauth2"
)

type GitHubProvider struct {
	client *github.Client
}

func NewGitHubProvider() (*GitHubProvider, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable not set")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	client := github.NewClient(tc)

	return &GitHubProvider{client: client}, nil
}

func (g *GitHubProvider) GetLatestRelease(ctx context.Context, owner, repo string) (*Release, error) {
	release, _, err := g.client.Repositories.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	return &Release{
		TagName:     release.GetTagName(),
		PublishedAt: release.GetPublishedAt().Time,
		Body:        release.GetBody(),
	}, nil
}

func (g *GitHubProvider) FetchRepositories(ctx context.Context, owner string) ([]string, error) {
	var allRepos []string
	opts := &github.RepositoryListByOrgOptions{ListOptions: github.ListOptions{PerPage: 100}}

	for {
		repos, resp, err := g.client.Repositories.ListByOrg(ctx, owner, opts)
		if err != nil {
			return nil, fmt.Errorf("error fetching repositories: %v", err)
		}

		for _, repo := range repos {
			allRepos = append(allRepos, repo.GetName())
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allRepos, nil
}

func (g *GitHubProvider) FetchCatalog(ctx context.Context, owner, repo string) (*Catalog, error) {
	fileContent, _, _, err := g.client.Repositories.GetContents(ctx, owner, repo, "gitspace-catalog.toml", nil)
	if err != nil {
		return nil, fmt.Errorf("error fetching gitspace-catalog.toml: %v", err)
	}

	content, err := fileContent.GetContent()
	if err != nil {
		return nil, fmt.Errorf("error decoding file content: %v", err)
	}

	var catalog Catalog
	err = toml.Unmarshal([]byte(content), &catalog)
	if err != nil {
		return nil, fmt.Errorf("error decoding TOML: %v", err)
	}

	return &catalog, nil
}

func (g *GitHubProvider) DownloadDirectory(ctx context.Context, owner, repo, path, destDir string) error {
	_, directoryContent, _, err := g.client.Repositories.GetContents(ctx, owner, repo, path, nil)
	if err != nil {
		return fmt.Errorf("error fetching directory contents: %v", err)
	}

	for _, file := range directoryContent {
		if *file.Type == "dir" {
			err = g.DownloadDirectory(ctx, owner, repo, *file.Path, filepath.Join(destDir, *file.Name))
			if err != nil {
				return err
			}
		} else {
			fileContent, _, _, err := g.client.Repositories.GetContents(ctx, owner, repo, *file.Path, nil)
			if err != nil {
				return fmt.Errorf("error fetching file content: %v", err)
			}

			content, err := fileContent.GetContent()
			if err != nil {
				return fmt.Errorf("error decoding file content: %v", err)
			}

			filePath := filepath.Join(destDir, *file.Name)
			err = os.WriteFile(filePath, []byte(content), 0644)
			if err != nil {
				return fmt.Errorf("error writing file: %v", err)
			}
		}
	}

	return nil
}
