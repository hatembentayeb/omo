package main

import (
	"fmt"
	"time"

	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// GitRepository represents a Git repository
type GitRepository struct {
	Name        string
	Path        string
	Branch      string
	Status      string
	LastCommit  string
	Modified    int
	Staged      int
	Untracked   int
	lastUpdated time.Time
}

// GitView manages the UI for interacting with Git repositories
type GitView struct {
	app             *tview.Application
	pages           *tview.Pages
	viewPages       *tview.Pages
	reposView       *ui.Cores
	statusView      *ui.Cores
	commitsView     *ui.Cores
	branchesView    *ui.Cores
	remotesView     *ui.Cores
	stashView       *ui.Cores
	tagsView        *ui.Cores
	gitClient       *GitClient
	repositories    []GitRepository
	currentRepoPath string
	currentViewName string
	refreshTimer    *time.Timer
	refreshInterval time.Duration
}

// NewGitView creates a new Git view
func NewGitView(app *tview.Application, pages *tview.Pages) *GitView {
	gv := &GitView{
		app:             app,
		pages:           pages,
		viewPages:       tview.NewPages(),
		repositories:    []GitRepository{},
		refreshInterval: 30 * time.Second,
	}

	// Initialize Git client
	gv.gitClient = NewGitClient()

	// Create all views
	gv.reposView = gv.newReposView()
	gv.statusView = gv.newStatusView()
	gv.commitsView = gv.newCommitsView()
	gv.branchesView = gv.newBranchesView()
	gv.remotesView = gv.newRemotesView()
	gv.stashView = gv.newStashView()
	gv.tagsView = gv.newTagsView()

	// Set modal pages for all views
	views := []*ui.Cores{
		gv.reposView,
		gv.statusView,
		gv.commitsView,
		gv.branchesView,
		gv.remotesView,
		gv.stashView,
		gv.tagsView,
	}
	for _, view := range views {
		if view != nil {
			view.SetModalPages(gv.pages)
		}
	}

	// Add all view pages
	gv.viewPages.AddPage("git-repos", gv.reposView.GetLayout(), true, true)
	gv.viewPages.AddPage("git-status", gv.statusView.GetLayout(), true, false)
	gv.viewPages.AddPage("git-commits", gv.commitsView.GetLayout(), true, false)
	gv.viewPages.AddPage("git-branches", gv.branchesView.GetLayout(), true, false)
	gv.viewPages.AddPage("git-remotes", gv.remotesView.GetLayout(), true, false)
	gv.viewPages.AddPage("git-stash", gv.stashView.GetLayout(), true, false)
	gv.viewPages.AddPage("git-tags", gv.tagsView.GetLayout(), true, false)

	// Set current view
	gv.currentViewName = viewRepos
	gv.setViewStack(gv.reposView, viewRepos)
	gv.setViewStack(gv.statusView, viewStatus)
	gv.setViewStack(gv.commitsView, viewCommits)
	gv.setViewStack(gv.branchesView, viewBranches)
	gv.setViewStack(gv.remotesView, viewRemotes)
	gv.setViewStack(gv.stashView, viewStash)
	gv.setViewStack(gv.tagsView, viewTags)

	// Set initial status
	gv.reposView.SetInfoText("[yellow]Git Manager[white]\nRepositories: 0\nPress [green]D[white] to add directory")

	// Start auto-refresh timer
	gv.startAutoRefresh()

	return gv
}

// GetMainUI returns the main UI component
func (gv *GitView) GetMainUI() tview.Primitive {
	return gv.viewPages
}

// Stop cleans up resources when the view is no longer used
func (gv *GitView) Stop() {
	if gv.refreshTimer != nil {
		gv.refreshTimer.Stop()
	}

	views := []*ui.Cores{
		gv.reposView,
		gv.statusView,
		gv.commitsView,
		gv.branchesView,
		gv.remotesView,
		gv.stashView,
		gv.tagsView,
	}
	for _, view := range views {
		if view != nil {
			view.StopAutoRefresh()
			view.UnregisterHandlers()
		}
	}
}

// refresh refreshes the current view
func (gv *GitView) refresh() {
	currentView := gv.currentCores()
	if currentView != nil {
		currentView.RefreshData()
	}
}

// handleAction handles actions triggered by the UI
func (gv *GitView) handleAction(action string, payload map[string]interface{}) error {
	switch action {
	case "refresh":
		gv.refresh()
		return nil
	case "keypress":
		if key, ok := payload["key"].(string); ok {
			// Navigation keys (available in all views)
			switch key {
			case "G":
				gv.showRepos()
				return nil
			case "S":
				gv.showStatus()
				return nil
			case "L":
				gv.showCommits()
				return nil
			case "B":
				gv.showBranches()
				return nil
			case "M":
				gv.showRemotes()
				return nil
			case "T":
				gv.showTags()
				return nil
			case "H":
				gv.showStash()
				return nil
			case "?":
				gv.showHelp()
				return nil
			}

			// View-specific action keys
			switch gv.currentViewName {
			case viewRepos:
				return gv.handleReposKeypress(key)
			case viewStatus:
				return gv.handleStatusKeypress(key)
			case viewCommits:
				return gv.handleCommitsKeypress(key)
			case viewBranches:
				return gv.handleBranchesKeypress(key)
			case viewRemotes:
				return gv.handleRemotesKeypress(key)
			case viewStash:
				return gv.handleStashKeypress(key)
			case viewTags:
				return gv.handleTagsKeypress(key)
			}
		}
	case "navigate_back":
		if view, ok := payload["current_view"].(string); ok {
			if view == viewRoot {
				gv.switchToView(viewRepos)
				return nil
			}
			gv.switchToView(view)
			return nil
		}
	}
	return fmt.Errorf("unhandled")
}

func (gv *GitView) handleReposKeypress(key string) error {
	switch key {
	case "D":
		gv.showDirectorySelector()
		return nil
	case "R":
		gv.showFuzzyRepoSearch()
		return nil
	case "F":
		gv.fetchSelectedRepo()
		return nil
	case "P":
		gv.pullSelectedRepo()
		return nil
	case "U":
		gv.pushSelectedRepo()
		return nil
	case "C":
		gv.checkoutBranch()
		return nil
	case "N":
		gv.createNewBranch()
		return nil
	}
	return fmt.Errorf("unhandled")
}

func (gv *GitView) handleStatusKeypress(key string) error {
	switch key {
	case "A":
		gv.stageSelectedFile()
		return nil
	case "U":
		gv.unstageSelectedFile()
		return nil
	case "D":
		gv.showFileDiff()
		return nil
	case "R":
		gv.restoreSelectedFile()
		return nil
	case "C":
		gv.showCommitDialog()
		return nil
	}
	return fmt.Errorf("unhandled")
}

func (gv *GitView) handleCommitsKeypress(key string) error {
	switch key {
	case "D":
		gv.showCommitDiff()
		return nil
	case "C":
		gv.checkoutCommit()
		return nil
	case "R":
		gv.revertCommit()
		return nil
	case "P":
		gv.cherryPickCommit()
		return nil
	}
	return fmt.Errorf("unhandled")
}

func (gv *GitView) handleBranchesKeypress(key string) error {
	switch key {
	case "C":
		gv.checkoutSelectedBranch()
		return nil
	case "N":
		gv.createBranchFromView()
		return nil
	case "D":
		gv.deleteSelectedBranch()
		return nil
	case "R":
		gv.renameSelectedBranch()
		return nil
	case "E":
		gv.mergeSelectedBranch()
		return nil
	}
	return fmt.Errorf("unhandled")
}

func (gv *GitView) handleRemotesKeypress(key string) error {
	switch key {
	case "A":
		gv.addRemote()
		return nil
	case "D":
		gv.removeRemote()
		return nil
	case "F":
		gv.fetchRemote()
		return nil
	case "P":
		gv.pruneRemote()
		return nil
	}
	return fmt.Errorf("unhandled")
}

func (gv *GitView) handleStashKeypress(key string) error {
	switch key {
	case "N":
		gv.createStash()
		return nil
	case "A":
		gv.applyStash()
		return nil
	case "P":
		gv.popStash()
		return nil
	case "D":
		gv.dropStash()
		return nil
	case "V":
		gv.viewStash()
		return nil
	}
	return fmt.Errorf("unhandled")
}

func (gv *GitView) handleTagsKeypress(key string) error {
	switch key {
	case "N":
		gv.createTag()
		return nil
	case "D":
		gv.deleteTag()
		return nil
	case "C":
		gv.checkoutTag()
		return nil
	case "P":
		gv.pushTag()
		return nil
	}
	return fmt.Errorf("unhandled")
}

// showHelp displays Git plugin help
func (gv *GitView) showHelp() {
	helpText := `
[yellow]Git Manager Help[white]

[green]Navigation Views:[white]
G       - Repositories view (main)
S       - Status view
L       - Commits view
B       - Branches view
M       - Remotes view
T       - Tags view
H       - Stash view

[green]Repository Actions (G view):[white]
D       - Add directory to search
R       - Fuzzy search repositories
F       - Fetch from remote
P       - Pull changes
U       - Push changes
C       - Checkout branch
N       - Create new branch
Enter   - View repository details

[green]Status Actions (S view):[white]
A       - Stage file
U       - Unstage file
D       - View diff
R       - Restore file
C       - Commit changes

[green]Commit Actions (L view):[white]
D       - View commit diff
C       - Checkout commit
R       - Revert commit
P       - Cherry-pick commit

[green]Branch Actions (B view):[white]
C       - Checkout branch
N       - Create branch
D       - Delete branch
R       - Rename branch
E       - Merge branch

[green]Remote Actions (M view):[white]
A       - Add remote
D       - Remove remote
F       - Fetch from remote
P       - Prune stale branches

[green]Stash Actions (H view):[white]
N       - Create stash
A       - Apply stash
P       - Pop stash
D       - Drop stash
V       - View stash content

[green]Tag Actions (T view):[white]
N       - Create tag
D       - Delete tag
C       - Checkout tag
P       - Push tag to remote

[green]General:[white]
?       - Show this help
Ctrl+R  - Refresh current view
/       - Filter table
Esc     - Close modal / Cancel
`

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		"Git Help",
		helpText,
		func() {
			current := gv.currentCores()
			if current != nil {
				gv.app.SetFocus(current.GetTable())
			}
		},
	)
}

// startAutoRefresh sets up and starts the auto-refresh timer
func (gv *GitView) startAutoRefresh() {
	// Load the refresh interval from config
	if uiConfig, err := GetGitUIConfig(); err == nil {
		gv.refreshInterval = time.Duration(uiConfig.RefreshInterval) * time.Second
	}

	// Cancel any existing timer
	if gv.refreshTimer != nil {
		gv.refreshTimer.Stop()
	}

	// Create a new timer
	gv.refreshTimer = time.AfterFunc(gv.refreshInterval, func() {
		gv.app.QueueUpdate(func() {
			gv.refresh()
			gv.startAutoRefresh()
		})
	})
}

// AutoDiscoverRepositories finds Git repositories from configured paths
func (gv *GitView) AutoDiscoverRepositories() {
	paths, err := GetSearchPaths()
	if err != nil || len(paths) == 0 {
		gv.reposView.Log("[yellow]No search paths configured. Press D to add a directory.")
		return
	}

	gv.reposView.Log("[blue]Searching for Git repositories...")

	for _, path := range paths {
		go gv.discoverRepositories(path)
	}
}
