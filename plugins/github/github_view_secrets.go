package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (gv *GitHubView) newSecretsView() *ui.CoreView {
	cores := ui.NewCoreView(gv.app, "Repository Secrets")
	cores.SetTableHeaders([]string{"Name", "Value", "Updated"})
	cores.SetRefreshCallback(gv.refreshSecretsData)
	cores.SetSelectionKey("Name")

	cores.AddKeyBinding("L", "Repos", gv.showRepos)
	cores.AddKeyBinding("P", "PRs", gv.showPRs)
	cores.AddKeyBinding("W", "Workflows", gv.showWorkflows)
	cores.AddKeyBinding("A", "Runs", gv.showRuns)
	cores.AddKeyBinding("E", "Env Vars", gv.showEnvVars)
	cores.AddKeyBinding("S", "Secrets", gv.showSecrets)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("F", "Releases", gv.showReleases)

	cores.AddKeyBinding("D", "Delete", gv.deleteSecret)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	cores.SetRowSelectedCallback(func(row int) {
		tableData := cores.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			cores.Log(fmt.Sprintf("[blue]Selected secret: %s", tableData[row][0]))
		}
	})

	cores.RegisterHandlers()
	return cores
}

func (gv *GitHubView) refreshSecretsData() ([][]string, error) {
	if !gv.githubClient.HasActiveRepo() {
		return [][]string{}, fmt.Errorf("no repository selected")
	}

	secrets, err := gv.githubClient.ListRepoSecrets()
	if err != nil {
		return [][]string{}, err
	}

	rows := make([][]string, len(secrets))
	for i, s := range secrets {
		rows[i] = s.GetTableRow()
	}

	if gv.activeRepo != nil {
		gv.secretsView.SetInfoMap(map[string]string{
			"Repo":    gv.activeRepo.FullName,
			"Secrets": fmt.Sprintf("%d", len(secrets)),
			"Status":  "Connected",
		})
	}

	return rows, nil
}

func (gv *GitHubView) getSelectedSecretName() (string, bool) {
	row := gv.secretsView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	return stripColorTags(row[0]), true
}

func (gv *GitHubView) deleteSecret() {
	name, ok := gv.getSelectedSecretName()
	if !ok {
		gv.secretsView.Log("[yellow]No secret selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Delete Secret",
		fmt.Sprintf("Delete secret [red]%s[white]?\nThis action cannot be undone!", name),
		func(confirmed bool) {
			if confirmed {
				gv.secretsView.Log(fmt.Sprintf("[yellow]Deleting secret %s...", name))
				go func() {
					if err := gv.githubClient.DeleteRepoSecret(name); err != nil {
						gv.app.QueueUpdateDraw(func() {
							gv.secretsView.Log(fmt.Sprintf("[red]Failed to delete: %v", err))
						})
					} else {
						gv.app.QueueUpdateDraw(func() {
							gv.secretsView.Log(fmt.Sprintf("[green]Secret %s deleted", name))
							gv.refresh()
						})
					}
				}()
			}
			gv.app.SetFocus(gv.secretsView.GetTable())
		},
	)
}
