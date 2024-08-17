package main

import (
	"os"
	"reflect"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

// MockGitHubAPI simulates fetching repositories from GitHub
func MockGitHubAPI() []string {
	return []string{
		"gitspace",
		"ssotools",
		"gitrepo",
		"othertool",
		"testproject",
		"dev-utils",
	}
}

// MockGitClone simulates cloning a repository
func MockGitClone(url string, path string, auth *ssh.PublicKeys) error {
	// In a real scenario, this would clone the repository
	// For testing, we'll just create an empty directory
	return os.MkdirAll(path, 0755)
}

func TestIntegrationEndsWith(t *testing.T) {
	config := Config{
		Repositories: &struct {
			GitSpace *struct {
				Path string `hcl:"path"`
			} `hcl:"gitspace,block"`
			Clone *struct {
				SCM        string   `hcl:"scm"`
				Owner      string   `hcl:"owner"`
				EndsWith   []string `hcl:"endsWith,optional"`
				StartsWith []string `hcl:"startsWith,optional"`
				Includes   []string `hcl:"includes,optional"`
				Names      []string `hcl:"name,optional"`
				Auth       *struct {
					Type    string `hcl:"type"`
					KeyPath string `hcl:"keyPath"`
				} `hcl:"auth,block"`
			} `hcl:"clone,block"`
		}{
			GitSpace: &struct {
				Path string `hcl:"path"`
			}{Path: "test_repos"},
			Clone: &struct {
				SCM        string   `hcl:"scm"`
				Owner      string   `hcl:"owner"`
				EndsWith   []string `hcl:"endsWith,optional"`
				StartsWith []string `hcl:"startsWith,optional"`
				Includes   []string `hcl:"includes,optional"`
				Names      []string `hcl:"name,optional"`
				Auth       *struct {
					Type    string `hcl:"type"`
					KeyPath string `hcl:"keyPath"`
				} `hcl:"auth,block"`
			}{
				SCM:      "github.com",
				Owner:    "testowner",
				EndsWith: []string{"tool", "tools"}, // Add both "tool" and "tools" to catch both cases
			},
		},
	}

	repos := MockGitHubAPI()
	filteredRepos := filterRepositories(repos, &config)

	expected := []string{"ssotools", "othertool"}
	if !reflect.DeepEqual(filteredRepos, expected) {
		t.Errorf("Expected %v, but got %v", expected, filteredRepos)
	}

	// Test mock cloning
	for _, repo := range filteredRepos {
		err := MockGitClone("git@github.com:testowner/"+repo+".git", "test_repos/"+repo, nil)
		if err != nil {
			t.Errorf("Error cloning repository %s: %v", repo, err)
		}
		if _, err := os.Stat("test_repos/" + repo); os.IsNotExist(err) {
			t.Errorf("Repository directory for %s was not created", repo)
		}
	}

	// Cleanup
	os.RemoveAll("test_repos")
}

func TestIntegrationIncludes(t *testing.T) {
	config := Config{
		Repositories: &struct {
			GitSpace *struct {
				Path string `hcl:"path"`
			} `hcl:"gitspace,block"`
			Clone *struct {
				SCM        string   `hcl:"scm"`
				Owner      string   `hcl:"owner"`
				EndsWith   []string `hcl:"endsWith,optional"`
				StartsWith []string `hcl:"startsWith,optional"`
				Includes   []string `hcl:"includes,optional"`
				Names      []string `hcl:"name,optional"`
				Auth       *struct {
					Type    string `hcl:"type"`
					KeyPath string `hcl:"keyPath"`
				} `hcl:"auth,block"`
			} `hcl:"clone,block"`
		}{
			GitSpace: &struct {
				Path string `hcl:"path"`
			}{Path: "test_repos"},
			Clone: &struct {
				SCM        string   `hcl:"scm"`
				Owner      string   `hcl:"owner"`
				EndsWith   []string `hcl:"endsWith,optional"`
				StartsWith []string `hcl:"startsWith,optional"`
				Includes   []string `hcl:"includes,optional"`
				Names      []string `hcl:"name,optional"`
				Auth       *struct {
					Type    string `hcl:"type"`
					KeyPath string `hcl:"keyPath"`
				} `hcl:"auth,block"`
			}{
				SCM:      "github.com",
				Owner:    "testowner",
				Includes: []string{"test", "utils"},
			},
		},
	}

	repos := MockGitHubAPI()
	filteredRepos := filterRepositories(repos, &config)

	expected := []string{"testproject", "dev-utils"}
	if !reflect.DeepEqual(filteredRepos, expected) {
		t.Errorf("Expected %v, but got %v", expected, filteredRepos)
	}

	// Test mock cloning
	for _, repo := range filteredRepos {
		err := MockGitClone("git@github.com:testowner/"+repo+".git", "test_repos/"+repo, nil)
		if err != nil {
			t.Errorf("Error cloning repository %s: %v", repo, err)
		}
		if _, err := os.Stat("test_repos/" + repo); os.IsNotExist(err) {
			t.Errorf("Repository directory for %s was not created", repo)
		}
	}

	// Cleanup
	os.RemoveAll("test_repos")
}

func TestIntegrationName(t *testing.T) {
	config := Config{
		Repositories: &struct {
			GitSpace *struct {
				Path string `hcl:"path"`
			} `hcl:"gitspace,block"`
			Clone *struct {
				SCM        string   `hcl:"scm"`
				Owner      string   `hcl:"owner"`
				EndsWith   []string `hcl:"endsWith,optional"`
				StartsWith []string `hcl:"startsWith,optional"`
				Includes   []string `hcl:"includes,optional"`
				Names      []string `hcl:"name,optional"`
				Auth       *struct {
					Type    string `hcl:"type"`
					KeyPath string `hcl:"keyPath"`
				} `hcl:"auth,block"`
			} `hcl:"clone,block"`
		}{
			GitSpace: &struct {
				Path string `hcl:"path"`
			}{Path: "test_repos"},
			Clone: &struct {
				SCM        string   `hcl:"scm"`
				Owner      string   `hcl:"owner"`
				EndsWith   []string `hcl:"endsWith,optional"`
				StartsWith []string `hcl:"startsWith,optional"`
				Includes   []string `hcl:"includes,optional"`
				Names      []string `hcl:"name,optional"`
				Auth       *struct {
					Type    string `hcl:"type"`
					KeyPath string `hcl:"keyPath"`
				} `hcl:"auth,block"`
			}{
				SCM:   "github.com",
				Owner: "testowner",
				Names: []string{"gitspace", "ssotools"},
			},
		},
	}

	repos := MockGitHubAPI()
	filteredRepos := filterRepositories(repos, &config)

	expected := []string{"gitspace", "ssotools"}
	if !reflect.DeepEqual(filteredRepos, expected) {
		t.Errorf("Expected %v, but got %v", expected, filteredRepos)
	}

	// Test mock cloning
	for _, repo := range filteredRepos {
		err := MockGitClone("git@github.com:testowner/"+repo+".git", "test_repos/"+repo, nil)
		if err != nil {
			t.Errorf("Error cloning repository %s: %v", repo, err)
		}
		if _, err := os.Stat("test_repos/" + repo); os.IsNotExist(err) {
			t.Errorf("Repository directory for %s was not created", repo)
		}
	}

	// Cleanup
	os.RemoveAll("test_repos")
}
