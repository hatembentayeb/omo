package main

import (
	"omo/pkg/ui"
)

const (
	viewRoot      = "github"
	viewRepos     = "repositories"
	viewPRs       = "pull-requests"
	viewWorkflows = "workflows"
	viewRuns      = "runs"
	viewEnvVars   = "variables"
	viewSecrets   = "secrets"
	viewBranches  = "branches"
	viewReleases  = "releases"
)

func (gv *GitHubView) currentCores() *ui.CoreView {
	switch gv.currentViewName {
	case viewPRs:
		return gv.prsView
	case viewWorkflows:
		return gv.workflowsView
	case viewRuns:
		return gv.runsView
	case viewEnvVars:
		return gv.envVarsView
	case viewSecrets:
		return gv.secretsView
	case viewBranches:
		return gv.branchesView
	case viewReleases:
		return gv.releasesView
	default:
		return gv.reposView
	}
}

func (gv *GitHubView) setViewStack(cores *ui.CoreView, viewName string) {
	if cores == nil {
		return
	}

	stack := []string{viewRoot, viewRepos}
	if viewName != viewRepos {
		stack = append(stack, viewName)
	}
	cores.SetViewStack(stack)
}

func (gv *GitHubView) switchToView(viewName string) {
	pageName := "github-" + viewName
	gv.currentViewName = viewName
	gv.viewPages.SwitchToPage(pageName)

	gv.setViewStack(gv.currentCores(), viewName)
	current := gv.currentCores()
	if current != nil {
		gv.app.SetFocus(current.GetTable())
	}
	go gv.asyncRefresh()
}

func (gv *GitHubView) showRepos() {
	gv.activeRepo = nil
	gv.githubClient.activeRepo = nil
	gv.switchToView(viewRepos)
}

func (gv *GitHubView) showPRs() {
	if gv.activeRepo == nil {
		gv.reposView.Log("[yellow]Select a repository first (Enter)")
		return
	}
	gv.switchToView(viewPRs)
}

func (gv *GitHubView) showWorkflows() {
	if gv.activeRepo == nil {
		gv.reposView.Log("[yellow]Select a repository first (Enter)")
		return
	}
	gv.switchToView(viewWorkflows)
}

func (gv *GitHubView) showRuns() {
	if gv.activeRepo == nil {
		gv.reposView.Log("[yellow]Select a repository first (Enter)")
		return
	}
	gv.switchToView(viewRuns)
}

func (gv *GitHubView) showEnvVars() {
	if gv.activeRepo == nil {
		gv.reposView.Log("[yellow]Select a repository first (Enter)")
		return
	}
	gv.switchToView(viewEnvVars)
}

func (gv *GitHubView) showSecrets() {
	if gv.activeRepo == nil {
		gv.reposView.Log("[yellow]Select a repository first (Enter)")
		return
	}
	gv.switchToView(viewSecrets)
}

func (gv *GitHubView) showBranches() {
	if gv.activeRepo == nil {
		gv.reposView.Log("[yellow]Select a repository first (Enter)")
		return
	}
	gv.switchToView(viewBranches)
}

func (gv *GitHubView) showReleases() {
	if gv.activeRepo == nil {
		gv.reposView.Log("[yellow]Select a repository first (Enter)")
		return
	}
	gv.switchToView(viewReleases)
}
