package main

import (
	"fmt"
	"strings"

	"omo/pkg/ui"
)

func (gv *GitHubView) newEnvVarsView() *ui.CoreView {
	cores := ui.NewCoreView(gv.app, "Environment Variables")
	cores.SetTableHeaders([]string{"Name", "Value", "Updated"})
	cores.SetRefreshCallback(gv.refreshEnvVarsData)
	cores.SetSelectionKey("Name")

	cores.AddKeyBinding("L", "Repos", gv.showRepos)
	cores.AddKeyBinding("P", "PRs", gv.showPRs)
	cores.AddKeyBinding("W", "Workflows", gv.showWorkflows)
	cores.AddKeyBinding("A", "Runs", gv.showRuns)
	cores.AddKeyBinding("E", "Env Vars", gv.showEnvVars)
	cores.AddKeyBinding("S", "Secrets", gv.showSecrets)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("F", "Releases", gv.showReleases)

	cores.AddKeyBinding("N", "New Var", gv.createEnvVar)
	cores.AddKeyBinding("U", "Update", gv.updateEnvVar)
	cores.AddKeyBinding("D", "Delete", gv.deleteEnvVar)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	cores.SetRowSelectedCallback(func(row int) {
		tableData := cores.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			cores.Log(fmt.Sprintf("[blue]Selected: %s = %s", tableData[row][0], tableData[row][1]))
		}
	})

	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		gv.showEnvVarDetails()
	})

	cores.RegisterHandlers()
	return cores
}

func (gv *GitHubView) refreshEnvVarsData() ([][]string, error) {
	if !gv.githubClient.HasActiveRepo() {
		return [][]string{}, fmt.Errorf("no repository selected")
	}

	vars, err := gv.githubClient.ListRepoVariables()
	if err != nil {
		return [][]string{}, err
	}

	rows := make([][]string, len(vars))
	for i, v := range vars {
		rows[i] = v.GetTableRow()
	}

	if gv.activeRepo != nil {
		gv.envVarsView.SetInfoMap(map[string]string{
			"Repo":      gv.activeRepo.FullName,
			"Variables": fmt.Sprintf("%d", len(vars)),
			"Status":    "Connected",
		})
	}

	return rows, nil
}

func (gv *GitHubView) getSelectedVarName() (string, bool) {
	row := gv.envVarsView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	return stripColorTags(row[0]), true
}

func (gv *GitHubView) createEnvVar() {
	ui.ShowCompactStyledInputModal(
		gv.pages,
		gv.app,
		"New Variable",
		"Variable Name",
		"",
		40,
		nil,
		func(name string, cancelled bool) {
			if cancelled || name == "" {
				gv.app.SetFocus(gv.envVarsView.GetTable())
				return
			}

			name = strings.ToUpper(strings.TrimSpace(name))

			ui.ShowCompactStyledInputModal(
				gv.pages,
				gv.app,
				"Variable Value",
				"Value",
				"",
				60,
				nil,
				func(value string, cancelled2 bool) {
					if cancelled2 {
						gv.app.SetFocus(gv.envVarsView.GetTable())
						return
					}

					gv.envVarsView.Log(fmt.Sprintf("[yellow]Creating variable %s...", name))

					go func() {
						if err := gv.githubClient.CreateRepoVariable(name, value); err != nil {
							gv.app.QueueUpdateDraw(func() {
								gv.envVarsView.Log(fmt.Sprintf("[red]Failed to create: %v", err))
							})
						} else {
							gv.app.QueueUpdateDraw(func() {
								gv.envVarsView.Log(fmt.Sprintf("[green]Variable %s created", name))
								gv.refresh()
							})
						}
					}()

					gv.app.SetFocus(gv.envVarsView.GetTable())
				},
			)
		},
	)
}

func (gv *GitHubView) updateEnvVar() {
	name, ok := gv.getSelectedVarName()
	if !ok {
		gv.envVarsView.Log("[yellow]No variable selected")
		return
	}

	row := gv.envVarsView.GetSelectedRowData()
	currentValue := ""
	if len(row) > 1 {
		currentValue = row[1]
	}

	ui.ShowCompactStyledInputModal(
		gv.pages,
		gv.app,
		fmt.Sprintf("Update %s", name),
		"New Value",
		currentValue,
		60,
		nil,
		func(value string, cancelled bool) {
			if cancelled {
				gv.app.SetFocus(gv.envVarsView.GetTable())
				return
			}

			gv.envVarsView.Log(fmt.Sprintf("[yellow]Updating %s...", name))

			go func() {
				if err := gv.githubClient.UpdateRepoVariable(name, value); err != nil {
					gv.app.QueueUpdateDraw(func() {
						gv.envVarsView.Log(fmt.Sprintf("[red]Failed to update: %v", err))
					})
				} else {
					gv.app.QueueUpdateDraw(func() {
						gv.envVarsView.Log(fmt.Sprintf("[green]Variable %s updated", name))
						gv.refresh()
					})
				}
			}()

			gv.app.SetFocus(gv.envVarsView.GetTable())
		},
	)
}

func (gv *GitHubView) deleteEnvVar() {
	name, ok := gv.getSelectedVarName()
	if !ok {
		gv.envVarsView.Log("[yellow]No variable selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Delete Variable",
		fmt.Sprintf("Delete variable [red]%s[white]?\nThis action cannot be undone!", name),
		func(confirmed bool) {
			if confirmed {
				gv.envVarsView.Log(fmt.Sprintf("[yellow]Deleting %s...", name))
				go func() {
					if err := gv.githubClient.DeleteRepoVariable(name); err != nil {
						gv.app.QueueUpdateDraw(func() {
							gv.envVarsView.Log(fmt.Sprintf("[red]Failed to delete: %v", err))
						})
					} else {
						gv.app.QueueUpdateDraw(func() {
							gv.envVarsView.Log(fmt.Sprintf("[green]Variable %s deleted", name))
							gv.refresh()
						})
					}
				}()
			}
			gv.app.SetFocus(gv.envVarsView.GetTable())
		},
	)
}

func (gv *GitHubView) showEnvVarDetails() {
	row := gv.envVarsView.GetSelectedRowData()
	if len(row) < 3 {
		return
	}

	var details strings.Builder
	details.WriteString(fmt.Sprintf("[yellow]Variable: %s[white]\n\n", row[0]))
	details.WriteString(fmt.Sprintf("[green]Name:[white]    %s\n", row[0]))
	details.WriteString(fmt.Sprintf("[green]Value:[white]   %s\n", row[1]))
	details.WriteString(fmt.Sprintf("[green]Updated:[white] %s\n", row[2]))

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		fmt.Sprintf("Variable: %s", row[0]),
		details.String(),
		func() {
			gv.app.SetFocus(gv.envVarsView.GetTable())
		},
	)
}
