package plugin

import (
  "bufio"
	"io"
	"os/exec"
  "github.com/ssotops/gitspace-plugin-sdk/logger"
)

type GitspaceCatalog struct {
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

type MenuItem struct {
	Label   string
	Command string
}

type Plugin struct {
    Name        string
    Path        string
    Version     string `toml:"version"`
    Description string `toml:"description"`
    Repository  struct {
        Type string `toml:"type"`
        URL  string `toml:"url"`
    } `toml:"repository"`
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.ReadCloser
    logger *logger.RateLimitedLogger
}

type CatalogPlugin struct {
	Path string
	// Add other necessary fields
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

type MenuOption struct {
    Label   string `json:"label"`
    Command string `json:"command"`
}

type bufferedWriteCloser struct {
    *bufio.Writer
    closer io.Closer
}
