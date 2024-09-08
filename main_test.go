package main

import (
	"os"
	"reflect"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/pelletier/go-toml"
	"github.com/stretchr/testify/assert"
)

func TestFilterRepositories(t *testing.T) {
	repos := []string{"service-a", "api-b", "test-c", "demo-d", "core-lib", "exact-repo-name"}

	testCases := []struct {
		name     string
		config   string
		expected []string
	}{
		{
			name: "Filter by endsWith",
			config: `
[global]
path = "gs"
labels = ["feature", "bug"]
scm = "github.com"
owner = "ssotops"
empty_repo_initial_branch = "main"

[groups.service]
match = "endsWith"
values = ["service", "api"]
`,
			expected: []string{"service-a", "api-b"},
		},
		{
			name: "Filter by startsWith",
			config: `
[global]
path = "gs"
labels = ["feature", "bug"]
scm = "github.com"
owner = "ssotops"
empty_repo_initial_branch = "master"

[groups.test]
match = "startsWith"
values = ["test-", "demo-"]
`,
			expected: []string{"test-c", "demo-d"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var config Config
			tree, err := toml.Load(tc.config)
			assert.NoError(t, err)

			err = tree.Unmarshal(&config)
			assert.NoError(t, err)

			result := filterRepositories(repos, &config)
			assert.Equal(t, tc.expected, result)
		})
	}
}

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

func MockGitClone(url string, path string, auth *ssh.PublicKeys) error {
	return os.MkdirAll(path, 0755)
}

func TestIntegrationEndsWith(t *testing.T) {
	configStr := `
[global]
path = "test_repos"
labels = ["feature", "bug"]
scm = "github.com"
owner = "testowner"
empty_repo_initial_branch = "main"

[groups.tools]
match = "endsWith"
values = ["tool", "tools"]
`

	var config Config
	tree, err := toml.Load(configStr)
	assert.NoError(t, err)

	err = tree.Unmarshal(&config)
	assert.NoError(t, err)

	repos := MockGitHubAPI()
	filteredRepos := filterRepositories(repos, &config)

	expected := []string{"ssotools", "othertool"}
	if !reflect.DeepEqual(filteredRepos, expected) {
		t.Errorf("Expected %v, but got %v", expected, filteredRepos)
	}

	for _, repo := range filteredRepos {
		err := MockGitClone("git@github.com:testowner/"+repo+".git", "test_repos/"+repo, nil)
		if err != nil {
			t.Errorf("Error cloning repository %s: %v", repo, err)
		}
		if _, err := os.Stat("test_repos/" + repo); os.IsNotExist(err) {
			t.Errorf("Repository directory for %s was not created", repo)
		}
	}

	os.RemoveAll("test_repos")
}

func TestIntegrationIncludes(t *testing.T) {
	configStr := `
[global]
path = "test_repos"
labels = ["feature", "bug"]
scm = "github.com"
owner = "testowner"

[groups.includes]
match = "includes"
values = ["test", "utils"]
`

	var config Config
	tree, err := toml.Load(configStr)
	assert.NoError(t, err)

	err = tree.Unmarshal(&config)
	assert.NoError(t, err)

	repos := MockGitHubAPI()
	filteredRepos := filterRepositories(repos, &config)

	expected := []string{"testproject", "dev-utils"}
	if !reflect.DeepEqual(filteredRepos, expected) {
		t.Errorf("Expected %v, but got %v", expected, filteredRepos)
	}

	for _, repo := range filteredRepos {
		err := MockGitClone("git@github.com:testowner/"+repo+".git", "test_repos/"+repo, nil)
		if err != nil {
			t.Errorf("Error cloning repository %s: %v", repo, err)
		}
		if _, err := os.Stat("test_repos/" + repo); os.IsNotExist(err) {
			t.Errorf("Repository directory for %s was not created", repo)
		}
	}

	os.RemoveAll("test_repos")
}

func TestIntegrationIsExactly(t *testing.T) {
	configStr := `
[global]
path = "test_repos"
labels = ["feature", "bug"]
scm = "github.com"
owner = "testowner"

[groups.exact]
match = "isExactly"
values = ["gitspace", "ssotools"]
`

	var config Config
	tree, err := toml.Load(configStr)
	assert.NoError(t, err)

	err = tree.Unmarshal(&config)
	assert.NoError(t, err)

	repos := MockGitHubAPI()
	filteredRepos := filterRepositories(repos, &config)

	expected := []string{"gitspace", "ssotools"}
	if !reflect.DeepEqual(filteredRepos, expected) {
		t.Errorf("Expected %v, but got %v", expected, filteredRepos)
	}

	for _, repo := range filteredRepos {
		err := MockGitClone("git@github.com:testowner/"+repo+".git", "test_repos/"+repo, nil)
		if err != nil {
			t.Errorf("Error cloning repository %s: %v", repo, err)
		}
		if _, err := os.Stat("test_repos/" + repo); os.IsNotExist(err) {
			t.Errorf("Repository directory for %s was not created", repo)
		}
	}

	os.RemoveAll("test_repos")
}

func TestMatchesGroup(t *testing.T) {
	testCases := []struct {
		name     string
		repo     string
		group    Group
		expected bool
	}{
		{
			name: "StartsWith match",
			repo: "test-repo",
			group: Group{
				Match:  "startsWith",
				Values: []string{"test-"},
			},
			expected: true,
		},
		{
			name: "EndsWith match",
			repo: "repo-test",
			group: Group{
				Match:  "endsWith",
				Values: []string{"-test"},
			},
			expected: true,
		},
		{
			name: "Includes match",
			repo: "my-test-repo",
			group: Group{
				Match:  "includes",
				Values: []string{"test"},
			},
			expected: true,
		},
		{
			name: "IsExactly match",
			repo: "exact-repo",
			group: Group{
				Match:  "isExactly",
				Values: []string{"exact-repo"},
			},
			expected: true,
		},
		{
			name: "No match",
			repo: "no-match-repo",
			group: Group{
				Match:  "startsWith",
				Values: []string{"test-"},
			},
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := matchesGroup(tc.repo, tc.group)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetRepoType(t *testing.T) {
	config := &Config{
		Groups: map[string]Group{
			"group1": {
				Match:  "startsWith",
				Values: []string{"test-"},
				Type:   "testType",
			},
			"group2": {
				Match:  "endsWith",
				Values: []string{"-prod"},
				Type:   "prodType",
			},
		},
	}

	testCases := []struct {
		name     string
		repo     string
		expected string
	}{
		{
			name:     "Matching test type",
			repo:     "test-repo",
			expected: "testType",
		},
		{
			name:     "Matching prod type",
			repo:     "repo-prod",
			expected: "prodType",
		},
		{
			name:     "No matching type",
			repo:     "other-repo",
			expected: "default",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getRepoType(config, tc.repo)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetRepoLabels(t *testing.T) {
	config := &Config{
		Global: struct {
			Path   string   `toml:"path"`
			Labels []string `toml:"labels"`
			SCM    string   `toml:"scm"`
			Owner  string   `toml:"owner"`
		}{
			Labels: []string{"global1", "global2"},
		},
		Groups: map[string]Group{
			"group1": {
				Match:  "startsWith",
				Values: []string{"test-"},
				Labels: []string{"test", "dev"},
			},
			"group2": {
				Match:  "endsWith",
				Values: []string{"-prod"},
				Labels: []string{"prod", "stable"},
			},
		},
	}

	testCases := []struct {
		name     string
		repo     string
		expected []string
	}{
		{
			name:     "Test repo labels",
			repo:     "test-repo",
			expected: []string{"global1", "global2", "test", "dev"},
		},
		{
			name:     "Prod repo labels",
			repo:     "repo-prod",
			expected: []string{"global1", "global2", "prod", "stable"},
		},
		{
			name:     "Other repo labels",
			repo:     "other-repo",
			expected: []string{"global1", "global2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := getRepoLabels(config, tc.repo)
			assert.ElementsMatch(t, tc.expected, result)
		})
	}
}

// Add more tests as needed for other functions
