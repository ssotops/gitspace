package main

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/ssotspace/gitspace/lib"
)

func syncLabels(logger *log.Logger, config *Config) {
	if !ensureConfig(logger, &config) {
		return
	}
	repos, err := lib.GetRepositories(config.Gitspace.Clone.SCM, config.Gitspace.Clone.Owner)
	if err != nil {
		logger.Error("Error fetching repositories", "error", err)
		return
	}

	changes := calculateLabelChanges(repos, config)

	printLabelChangeSummary(changes)

	confirmed := confirmChanges()
	if !confirmed {
		logger.Info("Label sync cancelled by user")
		return
	}

	applyLabelChanges(changes, logger, config.Gitspace.Clone.Owner)
}

func calculateLabelChanges(repos []string, config *Config) map[string][]string {
	changes := make(map[string][]string)

	for _, repo := range repos {
		changes[repo] = append(changes[repo], config.Gitspace.Labels...)

		if config.Gitspace.Clone != nil {
			changes[repo] = append(changes[repo], getLabelsForRepo(repo, config)...)
		}

		changes[repo] = removeDuplicates(changes[repo])
	}

	return changes
}

func getLabelsForRepo(repo string, config *Config) []string {
	var labels []string

	if matchesDirectiveGroup(repo, config.Gitspace.Clone.StartsWith) {
		labels = append(labels, getLabelsFromDirectiveGroup(config.Gitspace.Clone.StartsWith, repo)...)
	}
	if matchesDirectiveGroup(repo, config.Gitspace.Clone.EndsWith) {
		labels = append(labels, getLabelsFromDirectiveGroup(config.Gitspace.Clone.EndsWith, repo)...)
	}
	if matchesDirectiveGroup(repo, config.Gitspace.Clone.Includes) {
		labels = append(labels, getLabelsFromDirectiveGroup(config.Gitspace.Clone.Includes, repo)...)
	}
	if matchesDirectiveGroup(repo, config.Gitspace.Clone.IsExactly) {
		labels = append(labels, getLabelsFromDirectiveGroup(config.Gitspace.Clone.IsExactly, repo)...)
	}

	return labels
}

func getLabelsFromDirectiveGroup(directive map[string]*DirectiveGroup, repo string) []string {
	for _, group := range directive {
		if matchesGroup(repo, group) {
			return group.Labels
		}
	}
	return []string{}
}

func printLabelChangeSummary(changes map[string][]string) {
	fmt.Println("Label Sync Summary:")
	for repo, labels := range changes {
		fmt.Printf("%s:\n", repo)
		for _, label := range labels {
			fmt.Printf("  + %s\n", label)
		}
		fmt.Println()
	}
}

func confirmChanges() bool {
	var confirmed bool
	err := huh.NewConfirm().
		Title("Do you want to apply these changes?").
		Value(&confirmed).
		Run()

	if err != nil {
		fmt.Println("Error getting confirmation:", err)
		return false
	}

	return confirmed
}

func applyLabelChanges(changes map[string][]string, logger *log.Logger, owner string) {
	for repo, labels := range changes {
		err := lib.AddLabelsToRepository(owner, repo, labels)
		if err != nil {
			logger.Error("Error applying labels to repository", "repo", repo, "error", err)
		} else {
			logger.Info("Labels applied successfully", "repo", repo, "labels", labels)
		}
	}
}
