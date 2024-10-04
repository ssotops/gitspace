package lib

import "time"

type Release struct {
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
	Body        string    `json:"body"`
}

type Catalog struct {
	Catalog struct {
		Name        string `toml:"name"`
		Description string `toml:"description"`
		Version     string `toml:"version"`
		LastUpdated struct {
			Date       string `toml:"date"`
			CommitHash string `toml:"commit_hash"`
		} `toml:"last_updated"`
	} `toml:"catalog"`
	Plugins   map[string]Plugin   `toml:"plugins"`
	Templates map[string]Template `toml:"templates"`
}

type Plugin struct {
	Version     string `toml:"version"`
	Description string `toml:"description"`
	Path        string `toml:"path"`
	Repository  struct {
		Type string `toml:"type"`
		URL  string `toml:"url"`
	} `toml:"repository"`
}

type Template struct {
	Version     string `toml:"version,omitempty"`
	Description string `toml:"description,omitempty"`
	Path        string `toml:"path"`
	Repository  struct {
		Type string `toml:"type"`
		URL  string `toml:"url"`
	} `toml:"repository"`
}
