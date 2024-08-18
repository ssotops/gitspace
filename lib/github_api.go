// lib/github_api.go

package lib

import (
	"context"
	"fmt"
	"os"
  "strings"

	"github.com/google/go-github/v39/github"
	"github.com/charmbracelet/log"
	"golang.org/x/oauth2"
)

func FetchGitHubRepositories(owner string) ([]string, error) {
	ctx := context.Background()
	
	// Use GitHub token for authentication
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable not set")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	log.Info("Fetching GitHub repositories", "owner", owner)

	var allRepos []*github.Repository
	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		repos, resp, err := client.Repositories.ListByOrg(ctx, owner, opts)
		if err != nil {
			log.Error("Error fetching repositories", "error", err)
			if _, ok := err.(*github.RateLimitError); ok {
				log.Error("Hit rate limit", "error", err)
			}
			if errResp, ok := err.(*github.ErrorResponse); ok {
				log.Error("GitHub API error", "statusCode", errResp.Response.StatusCode, "message", errResp.Message)
			}
			return nil, fmt.Errorf("error fetching repositories: %v", err)
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	var repoNames []string
	for _, repo := range allRepos {
		repoNames = append(repoNames, repo.GetName())
	}

	log.Info("Fetched GitHub repositories", "count", len(repoNames))

	return repoNames, nil
}

func GetRepositories(scm, owner string) ([]string, error) {
	switch scm {
	case "github.com":
		return FetchGitHubRepositories(owner)
	// Add cases for other SCMs here in the future
	default:
		return nil, fmt.Errorf("unsupported SCM: %s", scm)
	}
}

func AddLabelsToRepository(repo string, labels []string) error {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	owner, repoName := splitOwnerRepo(repo)

	_, _, err := client.Issues.AddLabelsToIssue(ctx, owner, repoName, 0, labels)
	return err
}

func splitOwnerRepo(fullName string) (string, string) {
	parts := strings.Split(fullName, "/")
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "", fullName
}
