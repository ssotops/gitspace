// lib/github_api.go

package lib

import (
  "encoding/json"
	"context"
	"fmt"
	"os"
	"strings"
  "net/http"
	"time"

	"github.com/charmbracelet/log"
	"github.com/google/go-github/v39/github"
	"golang.org/x/oauth2"
)

// GitHubRelease represents the structure of a GitHub release
type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
	Body        string    `json:"body"`
}

// GetLatestGitHubRelease fetches the latest release information from GitHub
func GetLatestGitHubRelease(owner, repo string) (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var release GitHubRelease
	err = json.NewDecoder(resp.Body).Decode(&release)
	if err != nil {
		return nil, err
	}

	return &release, nil
}

// ExtractCommitHash attempts to extract the commit hash from the release body
func ExtractCommitHash(releaseBody string) string {
	lines := strings.Split(releaseBody, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Commit:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Commit:"))
		}
	}
	return ""
}

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

func AddLabelsToRepository(owner, repo string, labels []string) error {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: os.Getenv("GITHUB_TOKEN")},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	for _, label := range labels {
		_, _, err := client.Issues.CreateLabel(ctx, owner, repo, &github.Label{Name: &label})
		if err != nil {
			// If the label already exists, ignore the error
			if strings.Contains(err.Error(), "already_exists") {
				log.Info("Label already exists", "repo", repo, "label", label)
				continue
			}
			return fmt.Errorf("error creating label %s for %s/%s: %v", label, owner, repo, err)
		}
		log.Info("Label created successfully", "repo", repo, "label", label)
	}

	return nil
}
