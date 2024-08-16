package main

import (
	"context"
	"fmt"
	"os"

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

	// Get the project root directory
	src := client.Host().Directory(".")

	// Build the project
	build := client.Container().
		From("golang:1.23.0").  // Updated to use the latest stable Go version
		WithDirectory("/src", src).
		WithWorkdir("/src").
		WithExec([]string{"go", "build", "-o", "gitspace"})

	// Run tests
	test := build.WithExec([]string{"go", "test", "./..."})

	// If tests pass, create a GitHub release
	if _, err := test.Sync(ctx); err == nil {
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
	release, _, err := client.Repositories.CreateRelease(ctx, "ssotspace", "gitspace", &github.RepositoryRelease{
		TagName: github.String("v1.0.0"), // You might want to dynamically generate this
		Name:    github.String("Release v1.0.0"),
		Body:    github.String("Description of the release"),
		Draft:   github.Bool(false),
		Prerelease: github.Bool(false),
	})

	if err != nil {
		return err
	}

	fmt.Printf("Release created: %s\n", *release.HTMLURL)
	return nil
}
