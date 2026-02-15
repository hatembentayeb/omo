package main

import (
	"omo/pkg/ui"
)

const (
	viewRoot     = "git"
	viewRepos    = "repos"
	viewStatus   = "status"
	viewCommits  = "commits"
	viewBranches = "branches"
	viewRemotes  = "remotes"
	viewStash    = "stash"
	viewTags     = "tags"
	viewDiff     = "diff"
)

func (gv *GitView) currentCores() *ui.CoreView {
	switch gv.currentViewName {
	case viewStatus:
		return gv.statusView
	case viewCommits:
		return gv.commitsView
	case viewBranches:
		return gv.branchesView
	case viewRemotes:
		return gv.remotesView
	case viewStash:
		return gv.stashView
	case viewTags:
		return gv.tagsView
	default:
		return gv.reposView
	}
}

func (gv *GitView) setViewStack(cores *ui.CoreView, viewName string) {
	if cores == nil {
		return
	}

	stack := []string{viewRoot, viewRepos}
	if viewName != viewRepos {
		stack = append(stack, viewName)
	}
	cores.SetViewStack(stack)
}

func (gv *GitView) switchToView(viewName string) {
	pageName := "git-" + viewName
	gv.currentViewName = viewName
	gv.viewPages.SwitchToPage(pageName)

	gv.setViewStack(gv.currentCores(), viewName)
	gv.refresh()
	current := gv.currentCores()
	if current != nil {
		gv.app.SetFocus(current.GetTable())
	}
}

func (gv *GitView) showRepos() {
	gv.switchToView(viewRepos)
}

func (gv *GitView) showStatus() {
	gv.switchToView(viewStatus)
}

func (gv *GitView) showCommits() {
	gv.switchToView(viewCommits)
}

func (gv *GitView) showBranches() {
	gv.switchToView(viewBranches)
}

func (gv *GitView) showRemotes() {
	gv.switchToView(viewRemotes)
}

func (gv *GitView) showStash() {
	gv.switchToView(viewStash)
}

func (gv *GitView) showTags() {
	gv.switchToView(viewTags)
}
