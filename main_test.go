package main

import (
	"os"
	"reflect"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/stretchr/testify/assert"
)

func TestFilterRepositories(t *testing.T) {
	repos := []string{"service-a", "api-b", "test-c", "demo-d", "core-lib", "exact-repo-name"}

	testCases := []struct {
		name     string
		config   *Config
		expected []string
	}{
		{
			name: "Filter by EndsWith",
			config: &Config{
				Repositories: &struct {
					GitSpace *struct {
						Path string `hcl:"path"`
					} `hcl:"gitspace,block"`
					Labels []string `hcl:"labels,optional"`
					Clone  *struct {
						SCM        string        `hcl:"scm"`
						Owner      string        `hcl:"owner"`
						EndsWith   *FilterConfig `hcl:"endsWith,block"`
						StartsWith *FilterConfig `hcl:"startsWith,block"`
						Includes   *FilterConfig `hcl:"includes,block"`
						Names      *FilterConfig `hcl:"name,block"`
						Auth       *struct {
							Type    string `hcl:"type"`
							KeyPath string `hcl:"keyPath"`
						} `hcl:"auth,block"`
					} `hcl:"clone,block"`
				}{
					Clone: &struct {
						SCM        string        `hcl:"scm"`
						Owner      string        `hcl:"owner"`
						EndsWith   *FilterConfig `hcl:"endsWith,block"`
						StartsWith *FilterConfig `hcl:"startsWith,block"`
						Includes   *FilterConfig `hcl:"includes,block"`
						Names      *FilterConfig `hcl:"name,block"`
						Auth       *struct {
							Type    string `hcl:"type"`
							KeyPath string `hcl:"keyPath"`
						} `hcl:"auth,block"`
					}{
						EndsWith: &FilterConfig{Values: []string{"service", "api"}},
					},
				},
			},
			expected: []string{"service-a", "api-b"},
		},
		{
			name: "Filter by StartsWith",
			config: &Config{
				Repositories: &struct {
					GitSpace *struct {
						Path string `hcl:"path"`
					} `hcl:"gitspace,block"`
					Labels []string `hcl:"labels,optional"`
					Clone  *struct {
						SCM        string        `hcl:"scm"`
						Owner      string        `hcl:"owner"`
						EndsWith   *FilterConfig `hcl:"endsWith,block"`
						StartsWith *FilterConfig `hcl:"startsWith,block"`
						Includes   *FilterConfig `hcl:"includes,block"`
						Names      *FilterConfig `hcl:"name,block"`
						Auth       *struct {
							Type    string `hcl:"type"`
							KeyPath string `hcl:"keyPath"`
						} `hcl:"auth,block"`
					} `hcl:"clone,block"`
				}{
					Clone: &struct {
						SCM        string        `hcl:"scm"`
						Owner      string        `hcl:"owner"`
						EndsWith   *FilterConfig `hcl:"endsWith,block"`
						StartsWith *FilterConfig `hcl:"startsWith,block"`
						Includes   *FilterConfig `hcl:"includes,block"`
						Names      *FilterConfig `hcl:"name,block"`
						Auth       *struct {
							Type    string `hcl:"type"`
							KeyPath string `hcl:"keyPath"`
						} `hcl:"auth,block"`
					}{
						StartsWith: &FilterConfig{Values: []string{"test-", "demo-"}},
					},
				},
			},
			expected: []string{"test-c", "demo-d"},
		},
		{
			name: "Filter by Includes",
			config: &Config{
				Repositories: &struct {
					GitSpace *struct {
						Path string `hcl:"path"`
					} `hcl:"gitspace,block"`
					Labels []string `hcl:"labels,optional"`
					Clone  *struct {
						SCM        string        `hcl:"scm"`
						Owner      string        `hcl:"owner"`
						EndsWith   *FilterConfig `hcl:"endsWith,block"`
						StartsWith *FilterConfig `hcl:"startsWith,block"`
						Includes   *FilterConfig `hcl:"includes,block"`
						Names      *FilterConfig `hcl:"name,block"`
						Auth       *struct {
							Type    string `hcl:"type"`
							KeyPath string `hcl:"keyPath"`
						} `hcl:"auth,block"`
					} `hcl:"clone,block"`
				}{
					Clone: &struct {
						SCM        string        `hcl:"scm"`
						Owner      string        `hcl:"owner"`
						EndsWith   *FilterConfig `hcl:"endsWith,block"`
						StartsWith *FilterConfig `hcl:"startsWith,block"`
						Includes   *FilterConfig `hcl:"includes,block"`
						Names      *FilterConfig `hcl:"name,block"`
						Auth       *struct {
							Type    string `hcl:"type"`
							KeyPath string `hcl:"keyPath"`
						} `hcl:"auth,block"`
					}{
						Includes: &FilterConfig{Values: []string{"core", "lib"}},
					},
				},
			},
			expected: []string{"core-lib"},
		},
		{
			name: "Filter by Names",
			config: &Config{
				Repositories: &struct {
					GitSpace *struct {
						Path string `hcl:"path"`
					} `hcl:"gitspace,block"`
					Labels []string `hcl:"labels,optional"`
					Clone  *struct {
						SCM        string        `hcl:"scm"`
						Owner      string        `hcl:"owner"`
						EndsWith   *FilterConfig `hcl:"endsWith,block"`
						StartsWith *FilterConfig `hcl:"startsWith,block"`
						Includes   *FilterConfig `hcl:"includes,block"`
						Names      *FilterConfig `hcl:"name,block"`
						Auth       *struct {
							Type    string `hcl:"type"`
							KeyPath string `hcl:"keyPath"`
						} `hcl:"auth,block"`
					} `hcl:"clone,block"`
				}{
					Clone: &struct {
						SCM        string        `hcl:"scm"`
						Owner      string        `hcl:"owner"`
						EndsWith   *FilterConfig `hcl:"endsWith,block"`
						StartsWith *FilterConfig `hcl:"startsWith,block"`
						Includes   *FilterConfig `hcl:"includes,block"`
						Names      *FilterConfig `hcl:"name,block"`
						Auth       *struct {
							Type    string `hcl:"type"`
							KeyPath string `hcl:"keyPath"`
						} `hcl:"auth,block"`
					}{
						Names: &FilterConfig{Values: []string{"exact-repo-name"}},
					},
				},
			},
			expected: []string{"exact-repo-name"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := filterRepositories(repos, tc.config)
			assert.Equal(t, tc.expected, result)
		})
	}
}

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
			Labels []string `hcl:"labels,optional"`
			Clone  *struct {
				SCM        string        `hcl:"scm"`
				Owner      string        `hcl:"owner"`
				EndsWith   *FilterConfig `hcl:"endsWith,block"`
				StartsWith *FilterConfig `hcl:"startsWith,block"`
				Includes   *FilterConfig `hcl:"includes,block"`
				Names      *FilterConfig `hcl:"name,block"`
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
				SCM        string        `hcl:"scm"`
				Owner      string        `hcl:"owner"`
				EndsWith   *FilterConfig `hcl:"endsWith,block"`
				StartsWith *FilterConfig `hcl:"startsWith,block"`
				Includes   *FilterConfig `hcl:"includes,block"`
				Names      *FilterConfig `hcl:"name,block"`
				Auth       *struct {
					Type    string `hcl:"type"`
					KeyPath string `hcl:"keyPath"`
				} `hcl:"auth,block"`
			}{
				SCM:      "github.com",
				Owner:    "testowner",
				EndsWith: &FilterConfig{Values: []string{"tool", "tools"}},
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
			Labels []string `hcl:"labels,optional"`
			Clone  *struct {
				SCM        string        `hcl:"scm"`
				Owner      string        `hcl:"owner"`
				EndsWith   *FilterConfig `hcl:"endsWith,block"`
				StartsWith *FilterConfig `hcl:"startsWith,block"`
				Includes   *FilterConfig `hcl:"includes,block"`
				Names      *FilterConfig `hcl:"name,block"`
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
				SCM        string        `hcl:"scm"`
				Owner      string        `hcl:"owner"`
				EndsWith   *FilterConfig `hcl:"endsWith,block"`
				StartsWith *FilterConfig `hcl:"startsWith,block"`
				Includes   *FilterConfig `hcl:"includes,block"`
				Names      *FilterConfig `hcl:"name,block"`
				Auth       *struct {
					Type    string `hcl:"type"`
					KeyPath string `hcl:"keyPath"`
				} `hcl:"auth,block"`
			}{
				SCM:      "github.com",
				Owner:    "testowner",
				Includes: &FilterConfig{Values: []string{"test", "utils"}},
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
			Labels []string `hcl:"labels,optional"`
			Clone  *struct {
				SCM        string        `hcl:"scm"`
				Owner      string        `hcl:"owner"`
				EndsWith   *FilterConfig `hcl:"endsWith,block"`
				StartsWith *FilterConfig `hcl:"startsWith,block"`
				Includes   *FilterConfig `hcl:"includes,block"`
				Names      *FilterConfig `hcl:"name,block"`
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
				SCM        string        `hcl:"scm"`
				Owner      string        `hcl:"owner"`
				EndsWith   *FilterConfig `hcl:"endsWith,block"`
				StartsWith *FilterConfig `hcl:"startsWith,block"`
				Includes   *FilterConfig `hcl:"includes,block"`
				Names      *FilterConfig `hcl:"name,block"`
				Auth       *struct {
					Type    string `hcl:"type"`
					KeyPath string `hcl:"keyPath"`
				} `hcl:"auth,block"`
			}{
				SCM:   "github.com",
				Owner: "testowner",
				Names: &FilterConfig{Values: []string{"gitspace", "ssotools"}},
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
