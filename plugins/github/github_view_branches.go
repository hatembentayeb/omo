package main

import (
	"fmt"
	"strings"

	"omo/pkg/ui"
)

func (gv *GitHubView) newBranchesView() *ui.CoreView {
	cores := ui.NewCoreView(gv.app, "Branches")
	cores.SetTableHeaders([]string{"Name", "SHA", "Protected"})
	cores.SetRefreshCallback(gv.refreshBranchesData)
	cores.SetSelectionKey("Name")

	cores.AddKeyBinding("L", "Repos", gv.showRepos)
	cores.AddKeyBinding("P", "PRs", gv.showPRs)
	cores.AddKeyBinding("W", "Workflows", gv.showWorkflows)
	cores.AddKeyBinding("A", "Runs", gv.showRuns)
	cores.AddKeyBinding("E", "Env Vars", gv.showEnvVars)
	cores.AddKeyBinding("S", "Secrets", gv.showSecrets)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("F", "Releases", gv.showReleases)

	cores.AddKeyBinding("D", "Delete", gv.deleteBranch)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	cores.SetRowSelectedCallback(func(row int) {
		tableData := cores.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			cores.Log(fmt.Sprintf("[blue]Selected branch: %s", tableData[row][0]))
		}
	})

	cores.RegisterHandlers()
	return cores
}

func (gv *GitHubView) refreshBranchesData() ([][]string, error) {
	if !gv.githubClient.HasActiveRepo() {
		return [][]string{}, fmt.Errorf("no repository selected")
	}

	branches, err := gv.githubClient.ListBranches()
	if err != nil {
		return [][]string{}, err
	}

	rows := make([][]string, len(branches))
	for i, b := range branches {
		rows[i] = b.GetTableRow()
	}

	if gv.activeRepo != nil {
		gv.branchesView.SetInfoMap(map[string]string{
			"Repo":     gv.activeRepo.FullName,
			"Branches": fmt.Sprintf("%d", len(branches)),
			"Default":  gv.activeRepo.DefaultBranch,
			"Status":   "Connected",
		})
	}

	return rows, nil
}

func (gv *GitHubView) getSelectedBranchName() (string, bool) {
	row := gv.branchesView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	name := stripColorTags(row[0])
	name = strings.TrimRight(name, " *")
	return name, name != ""
}

func (gv *GitHubView) deleteBranch() {
	name, ok := gv.getSelectedBranchName()
	if !ok {
		gv.branchesView.Log("[yellow]No branch selected")
		return
	}

	if gv.activeRepo != nil && name == gv.activeRepo.DefaultBranch {
		gv.branchesView.Log("[red]Cannot delete the default branch")
		return
	}

	row := gv.branchesView.GetSelectedRowData()
	if len(row) > 2 && stripColorTags(row[2]) == "Yes" {
		gv.branchesView.Log("[red]Cannot delete a protected branch")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Delete Branch",
		fmt.Sprintf("Delete branch [red]%s[white]?\nThis action cannot be undone!", name),
		func(confirmed bool) {
			if confirmed {
				gv.branchesView.Log(fmt.Sprintf("[yellow]Deleting branch %s...", name))
				go func() {
					if err := gv.githubClient.DeleteBranch(name); err != nil {
						gv.app.QueueUpdateDraw(func() {
							gv.branchesView.Log(fmt.Sprintf("[red]Failed to delete: %v", err))
						})
					} else {
						gv.app.QueueUpdateDraw(func() {
							gv.branchesView.Log(fmt.Sprintf("[green]Branch %s deleted", name))
							gv.refresh()
						})
					}
				}()
			}
			gv.app.SetFocus(gv.branchesView.GetTable())
		},
	)
}
