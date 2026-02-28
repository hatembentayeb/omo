package main

import (
	"fmt"
	"strings"

	"omo/pkg/ui"
)

func (gv *GitHubView) newRunsView() *ui.CoreView {
	cores := ui.NewCoreView(gv.app, "Workflow Runs")
	cores.SetTableHeaders([]string{"ID", "Workflow", "Status", "Branch", "Event", "Actor", "Duration", "Age"})
	cores.SetRefreshCallback(gv.refreshRunsData)
	cores.SetSelectionKey("ID")

	cores.AddKeyBinding("L", "Repos", gv.showRepos)
	cores.AddKeyBinding("P", "PRs", gv.showPRs)
	cores.AddKeyBinding("W", "Workflows", gv.showWorkflows)
	cores.AddKeyBinding("A", "Runs", gv.showRuns)
	cores.AddKeyBinding("E", "Env Vars", gv.showEnvVars)
	cores.AddKeyBinding("S", "Secrets", gv.showSecrets)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("F", "Releases", gv.showReleases)

	cores.AddKeyBinding("X", "Cancel", gv.cancelRun)
	cores.AddKeyBinding("G", "Re-run", gv.rerunWorkflow)
	cores.AddKeyBinding("J", "Jobs", gv.viewRunJobs)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	cores.SetRowSelectedCallback(func(row int) {
		tableData := cores.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			cores.Log(fmt.Sprintf("[blue]Run %s: %s - %s", tableData[row][0], tableData[row][1], tableData[row][2]))
		}
	})

	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		gv.showRunDetails()
	})

	cores.RegisterHandlers()
	return cores
}

func (gv *GitHubView) refreshRunsData() ([][]string, error) {
	if !gv.githubClient.HasActiveRepo() {
		return [][]string{}, fmt.Errorf("no repository selected")
	}

	runs, err := gv.githubClient.ListWorkflowRuns("")
	if err != nil {
		return [][]string{}, err
	}

	rows := make([][]string, len(runs))
	for i, r := range runs {
		rows[i] = r.GetTableRow()
	}

	if gv.activeRepo != nil {
		gv.runsView.SetInfoMap(map[string]string{
			"Repo":   gv.activeRepo.FullName,
			"Runs":   fmt.Sprintf("%d", len(runs)),
			"Status": "Connected",
		})
	}

	return rows, nil
}

func (gv *GitHubView) getSelectedRunID() (int64, bool) {
	row := gv.runsView.GetSelectedRowData()
	if len(row) == 0 {
		return 0, false
	}
	var id int64
	fmt.Sscanf(stripColorTags(row[0]), "%d", &id)
	return id, id > 0
}

func (gv *GitHubView) cancelRun() {
	id, ok := gv.getSelectedRunID()
	if !ok {
		gv.runsView.Log("[yellow]No run selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Cancel Workflow Run",
		fmt.Sprintf("Cancel workflow run [red]%d[white]?", id),
		func(confirmed bool) {
			if confirmed {
				gv.runsView.Log(fmt.Sprintf("[yellow]Cancelling run %d...", id))
				go func() {
					if err := gv.githubClient.CancelWorkflowRun(id); err != nil {
						gv.app.QueueUpdateDraw(func() {
							gv.runsView.Log(fmt.Sprintf("[red]Failed to cancel: %v", err))
						})
					} else {
						gv.app.QueueUpdateDraw(func() {
							gv.runsView.Log(fmt.Sprintf("[green]Run %d cancelled", id))
							gv.refresh()
						})
					}
				}()
			}
			gv.app.SetFocus(gv.runsView.GetTable())
		},
	)
}

func (gv *GitHubView) rerunWorkflow() {
	id, ok := gv.getSelectedRunID()
	if !ok {
		gv.runsView.Log("[yellow]No run selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Re-run Workflow",
		fmt.Sprintf("Re-run workflow [yellow]%d[white]?", id),
		func(confirmed bool) {
			if confirmed {
				gv.runsView.Log(fmt.Sprintf("[yellow]Re-running workflow %d...", id))
				go func() {
					if err := gv.githubClient.RerunWorkflow(id); err != nil {
						gv.app.QueueUpdateDraw(func() {
							gv.runsView.Log(fmt.Sprintf("[red]Failed to re-run: %v", err))
						})
					} else {
						gv.app.QueueUpdateDraw(func() {
							gv.runsView.Log(fmt.Sprintf("[green]Workflow %d re-run triggered", id))
							gv.refresh()
						})
					}
				}()
			}
			gv.app.SetFocus(gv.runsView.GetTable())
		},
	)
}

func (gv *GitHubView) viewRunJobs() {
	id, ok := gv.getSelectedRunID()
	if !ok {
		gv.runsView.Log("[yellow]No run selected")
		return
	}

	gv.runsView.Log(fmt.Sprintf("[yellow]Loading jobs for run %d...", id))

	go func() {
		jobs, err := gv.githubClient.GetWorkflowRunJobs(id)
		gv.app.QueueUpdateDraw(func() {
			if err != nil {
				gv.runsView.Log(fmt.Sprintf("[red]Failed to load jobs: %v", err))
				return
			}

			var details strings.Builder
			details.WriteString(fmt.Sprintf("[yellow]Jobs for Run %d[white]\n\n", id))

			if len(jobs) == 0 {
				details.WriteString("No jobs found\n")
			}

			for _, job := range jobs {
				status := job.Status
				if job.Conclusion != "" {
					status = job.Conclusion
				}
				icon := "[gray]\u2022"
				switch status {
				case "success":
					icon = "[green]\u2714"
				case "failure":
					icon = "[red]\u2718"
				case "in_progress":
					icon = "[yellow]\u25cb"
				case "queued":
					icon = "[gray]\u25cb"
				case "cancelled":
					icon = "[gray]\u2718"
				}
				details.WriteString(fmt.Sprintf("%s [white]%s\n", icon, job.Name))
				details.WriteString(fmt.Sprintf("    Status: %s", status))
				if job.Duration != "" {
					details.WriteString(fmt.Sprintf("  |  Duration: %s", job.Duration))
				}
				if job.RunnerName != "" {
					details.WriteString(fmt.Sprintf("  |  Runner: %s", job.RunnerName))
				}
				details.WriteString("\n\n")
			}

			ui.ShowInfoModal(
				gv.pages,
				gv.app,
				fmt.Sprintf("Run %d Jobs", id),
				details.String(),
				func() {
					gv.app.SetFocus(gv.runsView.GetTable())
				},
			)
		})
	}()
}

func (gv *GitHubView) showRunDetails() {
	row := gv.runsView.GetSelectedRowData()
	if len(row) < 8 {
		return
	}

	var details strings.Builder
	details.WriteString(fmt.Sprintf("[yellow]Workflow Run[white]\n\n"))
	details.WriteString(fmt.Sprintf("[green]Run ID:[white]     %s\n", row[0]))
	details.WriteString(fmt.Sprintf("[green]Workflow:[white]   %s\n", row[1]))
	details.WriteString(fmt.Sprintf("[green]Status:[white]     %s\n", row[2]))
	details.WriteString(fmt.Sprintf("[green]Branch:[white]     %s\n", row[3]))
	details.WriteString(fmt.Sprintf("[green]Trigger:[white]    %s\n", row[4]))
	details.WriteString(fmt.Sprintf("[green]Actor:[white]      %s\n", row[5]))
	details.WriteString(fmt.Sprintf("[green]Duration:[white]   %s\n", row[6]))
	details.WriteString(fmt.Sprintf("[green]Started:[white]    %s\n", row[7]))

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		"Workflow Run Details",
		details.String(),
		func() {
			gv.app.SetFocus(gv.runsView.GetTable())
		},
	)
}
