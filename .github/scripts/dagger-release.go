package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"dagger.io/dagger"
	"github.com/google/go-github/v39/github"
	"golang.org/x/oauth2"
)

func main() {
	if err := publishRelease(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func publishRelease(ctx context.Context) error {
	// Initialize the Dagger client
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	// Get the absolute path of the project root directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %v", err)
	}
	projectRoot := filepath.Dir(filepath.Dir(wd)) // Go up two levels from the scripts directory

	// Get the project root directory
	src := client.Host().Directory(projectRoot)

	// Build the project
	build := client.Container().
		From("golang:1.23.0").
		WithDirectory("/src", src).
		WithWorkdir("/src").
		WithExec([]string{"go", "build", "-o", "gitspace"})

	// Run tests
	test := build.WithExec([]string{"go", "test", "./..."})

	// If tests pass, create a GitHub release
	if _, err := test.Sync(ctx); err == nil {
		fmt.Println("Tests passed. Creating GitHub release...")
		if err := createGitHubRelease(ctx); err != nil {
			return fmt.Errorf("failed to create GitHub release: %v", err)
		}
	} else {
		return fmt.Errorf("tests failed: %v", err)
	}

	return nil
}

func createGitHubRelease(ctx context.Context) error {
	// GitHub authentication
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN environment variable is not set")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Create the release
	release, resp, err := client.Repositories.CreateRelease(ctx, "ssotspace", "gitspace", &github.RepositoryRelease{
		TagName: github.String("v1.0.0"), // You might want to dynamically generate this
		Name:    github.String("Release v1.0.0"),
		Body:    github.String("Description of the release"),
		Draft:   github.Bool(false),
		Prerelease: github.Bool(false),
	})

	if err != nil {
		if resp != nil {
			fmt.Printf("GitHub API response status: %s\n", resp.Status)
			fmt.Printf("GitHub API response body: %s\n", resp.Body)
		}
		return fmt.Errorf("GitHub API error: %v", err)
	}

	fmt.Printf("Release created: %s\n", *release.HTMLURL)
	return nil
}
