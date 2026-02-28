package main

import (
	"fmt"
	"time"

	"github.com/rivo/tview"

	"omo/pkg/ui"
)

type GitHubView struct {
	app             *tview.Application
	pages           *tview.Pages
	viewPages       *tview.Pages
	reposView       *ui.CoreView
	prsView         *ui.CoreView
	workflowsView   *ui.CoreView
	runsView        *ui.CoreView
	envVarsView     *ui.CoreView
	secretsView     *ui.CoreView
	branchesView    *ui.CoreView
	releasesView    *ui.CoreView
	githubClient    *GitHubClient
	currentAccount  *GitHubAccount
	activeRepo      *GitHubRepo
	accounts        []GitHubAccount
	cachedRepos     []GitHubRepo
	cachedReleases  []Release
	currentViewName string
	refreshTimer    *time.Timer
	refreshInterval time.Duration
	prState         string
}

func NewGitHubView(app *tview.Application, pages *tview.Pages) *GitHubView {
	gv := &GitHubView{
		app:             app,
		pages:           pages,
		viewPages:       tview.NewPages(),
		refreshInterval: 30 * time.Second,
		prState:         "open",
	}

	gv.githubClient = NewGitHubClient()
	gv.githubClient.SetLogger(func(message string) {
		current := gv.currentCores()
		if current != nil {
			current.Log(message)
		}
	})

	gv.reposView = gv.newReposView()
	gv.prsView = gv.newPRsView()
	gv.workflowsView = gv.newWorkflowsView()
	gv.runsView = gv.newRunsView()
	gv.envVarsView = gv.newEnvVarsView()
	gv.secretsView = gv.newSecretsView()
	gv.branchesView = gv.newBranchesView()
	gv.releasesView = gv.newReleasesView()

	views := []*ui.CoreView{
		gv.reposView,
		gv.prsView,
		gv.workflowsView,
		gv.runsView,
		gv.envVarsView,
		gv.secretsView,
		gv.branchesView,
		gv.releasesView,
	}
	for _, view := range views {
		if view != nil {
			view.SetModalPages(gv.pages)
		}
	}

	gv.viewPages.AddPage("github-repositories", gv.reposView.GetLayout(), true, true)
	gv.viewPages.AddPage("github-pull-requests", gv.prsView.GetLayout(), true, false)
	gv.viewPages.AddPage("github-workflows", gv.workflowsView.GetLayout(), true, false)
	gv.viewPages.AddPage("github-runs", gv.runsView.GetLayout(), true, false)
	gv.viewPages.AddPage("github-variables", gv.envVarsView.GetLayout(), true, false)
	gv.viewPages.AddPage("github-secrets", gv.secretsView.GetLayout(), true, false)
	gv.viewPages.AddPage("github-branches", gv.branchesView.GetLayout(), true, false)
	gv.viewPages.AddPage("github-releases", gv.releasesView.GetLayout(), true, false)

	gv.currentViewName = viewRepos
	gv.setViewStack(gv.reposView, viewRepos)
	gv.setViewStack(gv.prsView, viewPRs)
	gv.setViewStack(gv.workflowsView, viewWorkflows)
	gv.setViewStack(gv.runsView, viewRuns)
	gv.setViewStack(gv.envVarsView, viewEnvVars)
	gv.setViewStack(gv.secretsView, viewSecrets)
	gv.setViewStack(gv.branchesView, viewBranches)
	gv.setViewStack(gv.releasesView, viewReleases)

	gv.reposView.SetInfoText("[yellow]GitHub Manager[white]\nStatus: Not connected\nUse [green]Ctrl+G[white] to select account")

	gv.startAutoRefresh()

	return gv
}

func (gv *GitHubView) GetMainUI() tview.Primitive {
	return gv.viewPages
}

func (gv *GitHubView) Stop() {
	if gv.refreshTimer != nil {
		gv.refreshTimer.Stop()
	}

	views := []*ui.CoreView{
		gv.reposView,
		gv.prsView,
		gv.workflowsView,
		gv.runsView,
		gv.envVarsView,
		gv.secretsView,
		gv.branchesView,
		gv.releasesView,
	}
	for _, view := range views {
		if view != nil {
			view.StopAutoRefresh()
			view.UnregisterHandlers()
		}
	}
}

func (gv *GitHubView) ShowAccountSelector() {
	current := gv.currentCores()
	if current != nil {
		current.Log("[blue]Opening account selector...")
	}

	if len(gv.accounts) == 0 {
		accounts, err := GetAvailableAccounts()
		if err != nil {
			if current != nil {
				current.Log(fmt.Sprintf("[red]Failed to load accounts: %v", err))
			}
			return
		}
		gv.accounts = accounts
	}

	if len(gv.accounts) == 0 {
		if current != nil {
			current.Log("[yellow]No GitHub accounts configured. Add entries under github/<environment>/<name> in KeePass.")
		}
		return
	}

	items := make([][]string, len(gv.accounts))
	for i, acct := range gv.accounts {
		desc := acct.Description
		if desc == "" {
			desc = acct.AccountType
		}
		label := acct.Name
		if acct.Owner != "" {
			label = acct.Owner
		}
		items[i] = []string{
			fmt.Sprintf("%s/%s", acct.Environment, label),
			desc,
		}
	}

	accounts := gv.accounts
	ui.ShowStandardListSelectorModal(
		gv.pages,
		gv.app,
		"Select GitHub Account",
		items,
		func(index int, name string, cancelled bool) {
			if !cancelled && index >= 0 && index < len(accounts) {
				gv.connectToAccount(&accounts[index])
			} else {
				if current != nil {
					current.Log("[blue]Account selection cancelled")
				}
			}
			if current != nil {
				gv.app.SetFocus(current.GetTable())
			}
		},
	)
}

func (gv *GitHubView) connectToAccount(acct *GitHubAccount) {
	current := gv.currentCores()
	displayName := acct.Owner
	if displayName == "" {
		displayName = acct.Name
	}
	if current != nil {
		current.Log(fmt.Sprintf("[yellow]Connecting to %s (%s)...", displayName, acct.Environment))
	}

	gv.githubClient.SetAccount(acct)
	gv.currentAccount = acct
	gv.activeRepo = nil

	if current != nil {
		current.Log(fmt.Sprintf("[green]Connected as %s", acct.Owner))
		current.Log("[aqua]Fetching repositories...")
	}

	gv.switchToView(viewRepos)
}

func (gv *GitHubView) updateInfoPanel() {
	current := gv.currentCores()
	if current == nil || gv.currentAccount == nil {
		return
	}
	info := map[string]string{
		"Account": gv.currentAccount.Owner,
		"Env":     gv.currentAccount.Environment,
		"Status":  "Connected",
	}
	if gv.activeRepo != nil {
		info["Repo"] = gv.activeRepo.FullName
		info["Branch"] = gv.activeRepo.DefaultBranch
	}
	current.SetInfoMap(info)
}

func (gv *GitHubView) refresh() {
	currentView := gv.currentCores()
	if currentView != nil {
		currentView.RefreshData()
	}
}

func (gv *GitHubView) asyncRefresh() {
	viewName := gv.currentViewName
	currentView := gv.currentCores()
	if currentView == nil {
		return
	}

	refreshFn := gv.getRefreshFunc(viewName)
	if refreshFn == nil {
		return
	}

	gv.app.QueueUpdateDraw(func() {
		currentView.Log("Refreshing data...")
	})

	data, err := refreshFn()

	gv.app.QueueUpdateDraw(func() {
		if gv.currentViewName != viewName {
			return
		}
		if err != nil {
			currentView.Log(fmt.Sprintf("[red]Error refreshing data: %v", err))
			return
		}
		currentView.SetTableData(data)
		currentView.Log("[green]Data refreshed successfully")
		gv.updateInfoPanel()
	})
}

func (gv *GitHubView) getRefreshFunc(viewName string) func() ([][]string, error) {
	switch viewName {
	case viewRepos:
		return gv.refreshReposData
	case viewPRs:
		return gv.refreshPRsData
	case viewWorkflows:
		return gv.refreshWorkflowsData
	case viewRuns:
		return gv.refreshRunsData
	case viewEnvVars:
		return gv.refreshEnvVarsData
	case viewSecrets:
		return gv.refreshSecretsData
	case viewBranches:
		return gv.refreshBranchesData
	case viewReleases:
		return gv.refreshReleasesData
	default:
		return nil
	}
}

func (gv *GitHubView) handleAction(action string, payload map[string]interface{}) error {
	switch action {
	case "refresh":
		gv.refresh()
		return nil
	case "keypress":
		if key, ok := payload["key"].(string); ok {
			if gv.handleNavKeys(key) {
				return nil
			}
		}
	case "navigate_back":
		if view, ok := payload["current_view"].(string); ok {
			if view == viewRoot || view == viewRepos {
				gv.showRepos()
				return nil
			}
			gv.switchToView(view)
			return nil
		}
	}
	return fmt.Errorf("unhandled")
}

func (gv *GitHubView) handleNavKeys(key string) bool {
	switch key {
	case "L":
		gv.showRepos()
	case "P":
		gv.showPRs()
	case "W":
		gv.showWorkflows()
	case "A":
		gv.showRuns()
	case "E":
		gv.showEnvVars()
	case "S":
		gv.showSecrets()
	case "B":
		gv.showBranches()
	case "F":
		gv.showReleases()
	case "?":
		gv.showHelp()
	default:
		return false
	}
	return true
}

func (gv *GitHubView) showHelp() {
	helpText := `
[yellow]GitHub Manager Help[white]

[green]Navigation Views:[white]
L       - Repositories list (home)
P       - Pull Requests view
W       - Workflows view
A       - Workflow Runs (Actions)
E       - Environment Variables view
S       - Secrets view
B       - Branches view
F       - Releases view
R       - Refresh (standard)
Ctrl+G  - Switch GitHub account
Esc     - Back to repos list

[green]Repositories (L view):[white]
Enter   - Select repo (opens PRs)
/       - Filter repos

[green]Pull Request Actions (P view):[white]
M       - Merge PR (select merge method)
C       - Close PR
O       - Reopen PR
V       - Approve PR
K       - View PR checks
I       - View PR reviews
T       - Toggle PR state (open/closed/all)
Enter   - View PR details

[green]Workflow Actions (W view):[white]
D       - Dispatch workflow (trigger run)
Enter   - View workflow details

[green]Workflow Runs Actions (A view):[white]
X       - Cancel run
G       - Re-run workflow
J       - View run jobs
Enter   - View run details

[green]Environment Variables Actions (E view):[white]
N       - Create new variable
U       - Update variable value
D       - Delete variable
Enter   - View variable details

[green]Secrets Actions (S view):[white]
D       - Delete secret

[green]Branch Actions (B view):[white]
D       - Delete branch

[green]Release Actions (R view):[white]
D       - Delete release
Enter   - View release details

[green]General:[white]
?       - Show this help
/       - Filter table
Esc     - Back / Close modal
`

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		"GitHub Help",
		helpText,
		func() {
			current := gv.currentCores()
			if current != nil {
				gv.app.SetFocus(current.GetTable())
			}
		},
	)
}

func (gv *GitHubView) startAutoRefresh() {
	uiConfig := GetGitHubUIConfig()
	gv.refreshInterval = time.Duration(uiConfig.RefreshInterval) * time.Second

	if gv.refreshTimer != nil {
		gv.refreshTimer.Stop()
	}

	gv.refreshTimer = time.AfterFunc(gv.refreshInterval, func() {
		if gv.githubClient != nil && gv.githubClient.HasAccount() {
			go gv.asyncRefresh()
			gv.startAutoRefresh()
		} else {
			gv.startAutoRefresh()
		}
	})
}

func (gv *GitHubView) AutoDiscoverAccounts() {
	accounts, err := GetAvailableAccounts()
	if err != nil {
		gv.reposView.Log(fmt.Sprintf("[yellow]Failed to load accounts: %v", err))
		gv.reposView.Log("[yellow]Configure accounts in KeePass under github/<environment>/<name>")
		return
	}

	if len(accounts) == 0 {
		gv.reposView.Log("[yellow]No GitHub accounts configured")
		gv.reposView.Log("[yellow]Add entries under github/<environment>/<name> in KeePass")
		gv.reposView.Log("[yellow]Required: UserName (GitHub user/org), Password (PAT token)")
		return
	}

	gv.accounts = accounts
	gv.reposView.Log(fmt.Sprintf("[blue]Found %d account(s)", len(accounts)))

	gv.app.QueueUpdateDraw(func() {
		gv.ShowAccountSelector()
	})
}
