package lib

import "context"

type SCMProvider interface {
	GetLatestRelease(ctx context.Context, owner, repo string) (*Release, error)
	FetchRepositories(ctx context.Context, owner string) ([]string, error)
	FetchCatalog(ctx context.Context, owner, repo string) (*Catalog, error)
	DownloadDirectory(ctx context.Context, owner, repo, path, destDir string) error
}
