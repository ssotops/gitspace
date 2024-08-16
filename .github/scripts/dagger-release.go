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
	client, err := dagger.Connect(ctx, dagger.WithLogOutput(os.Stdout))
	if err != nil {
		return err
	}
	defer client.Close()

	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %v", err)
	}
	projectRoot := filepath.Dir(filepath.Dir(wd))

	src := client.Host().Directory(projectRoot)

	// Build for multiple architectures
	architectures := []struct {
		goos   string
		goarch string
	}{
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"windows", "amd64"},
	}

	for _, arch := range architectures {
		build := client.Container().
			From("golang:1.23.0").
			WithDirectory("/src", src).
			WithWorkdir("/src").
			WithEnvVariable("GOOS", arch.goos).
			WithEnvVariable("GOARCH", arch.goarch).
			WithExec([]string{"go", "build", "-o", fmt.Sprintf("gitspace_%s_%s", arch.goos, arch.goarch)})

		// Export the binary
		_, err := build.File(fmt.Sprintf("gitspace_%s_%s", arch.goos, arch.goarch)).Export(ctx, fmt.Sprintf("gitspace_%s_%s", arch.goos, arch.goarch))
		if err != nil {
			return fmt.Errorf("failed to export binary for %s_%s: %v", arch.goos, arch.goarch, err)
		}
	}

	// Run tests
	test := client.Container().
		From("golang:1.23.0").
		WithDirectory("/src", src).
		WithWorkdir("/src").
		WithExec([]string{"go", "test", "./..."})

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
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN environment variable is not set")
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	release, _, err := client.Repositories.CreateRelease(ctx, "ssotops", "gitspace", &github.RepositoryRelease{
		TagName:    github.String("v1.0.0"),
		Name:       github.String("Release v1.0.0"),
		Body:       github.String("Description of the release"),
		Draft:      github.Bool(false),
		Prerelease: github.Bool(false),
	})
	if err != nil {
		return fmt.Errorf("failed to create GitHub release: %v", err)
	}

	// Upload binaries
	architectures := []struct {
		goos   string
		goarch string
	}{
		{"darwin", "amd64"},
		{"darwin", "arm64"},
		{"linux", "amd64"},
		{"linux", "arm64"},
		{"windows", "amd64"},
	}

	for _, arch := range architectures {
		filename := fmt.Sprintf("gitspace_%s_%s", arch.goos, arch.goarch)
		if arch.goos == "windows" {
			filename += ".exe"
		}

		file, err := os.Open(filename)
		if err != nil {
			return fmt.Errorf("failed to open binary %s: %v", filename, err)
		}
		defer file.Close()

		_, _, err = client.Repositories.UploadReleaseAsset(ctx, "ssotops", "gitspace", *release.ID, &github.UploadOptions{
			Name: filename,
		}, file)
		if err != nil {
			return fmt.Errorf("failed to upload asset %s: %v", filename, err)
		}
	}

	fmt.Printf("Release created: %s\n", *release.HTMLURL)
	return nil
}
