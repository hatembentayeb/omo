package main

import (
	"fmt"
	"strings"

	"omo/pkg/ui"
)

func (gv *GitHubView) newWorkflowsView() *ui.CoreView {
	cores := ui.NewCoreView(gv.app, "Workflows")
	cores.SetTableHeaders([]string{"ID", "Name", "Path", "State", "Updated"})
	cores.SetRefreshCallback(gv.refreshWorkflowsData)
	cores.SetSelectionKey("ID")

	cores.AddKeyBinding("L", "Repos", gv.showRepos)
	cores.AddKeyBinding("P", "PRs", gv.showPRs)
	cores.AddKeyBinding("W", "Workflows", gv.showWorkflows)
	cores.AddKeyBinding("A", "Runs", gv.showRuns)
	cores.AddKeyBinding("E", "Env Vars", gv.showEnvVars)
	cores.AddKeyBinding("S", "Secrets", gv.showSecrets)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("F", "Releases", gv.showReleases)

	cores.AddKeyBinding("D", "Dispatch", gv.dispatchWorkflow)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	cores.SetRowSelectedCallback(func(row int) {
		tableData := cores.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			cores.Log(fmt.Sprintf("[blue]Selected workflow: %s", tableData[row][1]))
		}
	})

	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		gv.showWorkflowDetails()
	})

	cores.RegisterHandlers()
	return cores
}

func (gv *GitHubView) refreshWorkflowsData() ([][]string, error) {
	if !gv.githubClient.HasActiveRepo() {
		return [][]string{}, fmt.Errorf("no repository selected")
	}

	workflows, err := gv.githubClient.ListWorkflows()
	if err != nil {
		return [][]string{}, err
	}

	rows := make([][]string, len(workflows))
	for i, w := range workflows {
		rows[i] = w.GetTableRow()
	}

	if gv.activeRepo != nil {
		gv.workflowsView.SetInfoMap(map[string]string{
			"Repo":      gv.activeRepo.FullName,
			"Workflows": fmt.Sprintf("%d", len(workflows)),
			"Status":    "Connected",
		})
	}

	return rows, nil
}

func (gv *GitHubView) getSelectedWorkflowID() (int64, bool) {
	row := gv.workflowsView.GetSelectedRowData()
	if len(row) == 0 {
		return 0, false
	}
	var id int64
	fmt.Sscanf(row[0], "%d", &id)
	return id, id > 0
}

func (gv *GitHubView) getSelectedWorkflowName() string {
	row := gv.workflowsView.GetSelectedRowData()
	if len(row) < 2 {
		return ""
	}
	return row[1]
}

func (gv *GitHubView) dispatchWorkflow() {
	id, ok := gv.getSelectedWorkflowID()
	if !ok {
		gv.workflowsView.Log("[yellow]No workflow selected")
		return
	}

	row := gv.workflowsView.GetSelectedRowData()
	if len(row) >= 4 && row[3] != "active" {
		gv.workflowsView.Log(fmt.Sprintf("[red]Cannot dispatch: workflow is %s", row[3]))
		return
	}

	name := gv.getSelectedWorkflowName()
	defaultRef := "main"
	if gv.activeRepo != nil {
		defaultRef = gv.activeRepo.DefaultBranch
	}

	ui.ShowCompactStyledInputModal(
		gv.pages,
		gv.app,
		"Dispatch Workflow",
		"Branch/Tag ref",
		defaultRef,
		40,
		nil,
		func(ref string, cancelled bool) {
			if cancelled || ref == "" {
				gv.app.SetFocus(gv.workflowsView.GetTable())
				return
			}

			gv.workflowsView.Log(fmt.Sprintf("[yellow]Dispatching %s on %s...", name, ref))

			go func() {
				if err := gv.githubClient.TriggerWorkflowDispatch(id, ref); err != nil {
					gv.app.QueueUpdateDraw(func() {
						gv.workflowsView.Log(fmt.Sprintf("[red]Failed to dispatch: %v", err))
					})
				} else {
					gv.app.QueueUpdateDraw(func() {
						gv.workflowsView.Log(fmt.Sprintf("[green]Workflow %s dispatched on %s", name, ref))
					})
				}
			}()

			gv.app.SetFocus(gv.workflowsView.GetTable())
		},
	)
}

func (gv *GitHubView) showWorkflowDetails() {
	row := gv.workflowsView.GetSelectedRowData()
	if len(row) < 4 {
		return
	}

	var details strings.Builder
	details.WriteString(fmt.Sprintf("[yellow]Workflow: %s[white]\n\n", row[1]))
	details.WriteString(fmt.Sprintf("[green]ID:[white]      %s\n", row[0]))
	details.WriteString(fmt.Sprintf("[green]Path:[white]    %s\n", row[2]))
	details.WriteString(fmt.Sprintf("[green]State:[white]   %s\n", row[3]))
	details.WriteString(fmt.Sprintf("[green]Updated:[white] %s\n", row[4]))

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		fmt.Sprintf("Workflow: %s", row[1]),
		details.String(),
		func() {
			gv.app.SetFocus(gv.workflowsView.GetTable())
		},
	)
}
