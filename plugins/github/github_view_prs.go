package main

import (
	"fmt"
	"strings"

	"omo/pkg/ui"
)

func (gv *GitHubView) newPRsView() *ui.CoreView {
	cores := ui.NewCoreView(gv.app, "Pull Requests")
	cores.SetTableHeaders([]string{"#", "Title", "State", "Author", "Branch", "Base", "Changes", "Labels", "Age"})
	cores.SetRefreshCallback(gv.refreshPRsData)
	cores.SetSelectionKey("#")

	cores.AddKeyBinding("L", "Repos", gv.showRepos)
	cores.AddKeyBinding("P", "PRs", gv.showPRs)
	cores.AddKeyBinding("W", "Workflows", gv.showWorkflows)
	cores.AddKeyBinding("A", "Runs", gv.showRuns)
	cores.AddKeyBinding("E", "Env Vars", gv.showEnvVars)
	cores.AddKeyBinding("S", "Secrets", gv.showSecrets)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("F", "Releases", gv.showReleases)

	cores.AddKeyBinding("M", "Merge", gv.mergePR)
	cores.AddKeyBinding("C", "Close", gv.closePR)
	cores.AddKeyBinding("O", "Reopen", gv.reopenPR)
	cores.AddKeyBinding("V", "Approve", gv.approvePR)
	cores.AddKeyBinding("K", "Checks", gv.viewPRChecks)
	cores.AddKeyBinding("I", "Reviews", gv.viewPRReviews)
	cores.AddKeyBinding("T", "Toggle State", gv.togglePRState)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	cores.SetRowSelectedCallback(func(row int) {
		tableData := cores.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			cores.Log(fmt.Sprintf("[blue]Selected PR: %s - %s", tableData[row][0], tableData[row][1]))
		}
	})

	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		gv.showPRDetails()
	})

	cores.RegisterHandlers()
	return cores
}

func (gv *GitHubView) refreshPRsData() ([][]string, error) {
	if !gv.githubClient.HasActiveRepo() {
		return [][]string{}, fmt.Errorf("no repository selected")
	}

	prs, err := gv.githubClient.ListPullRequests(gv.prState)
	if err != nil {
		return [][]string{}, err
	}

	rows := make([][]string, len(prs))
	for i, pr := range prs {
		rows[i] = pr.GetTableRow()
	}

	if gv.activeRepo != nil {
		gv.prsView.SetInfoMap(map[string]string{
			"Repo":   gv.activeRepo.FullName,
			"Filter": gv.prState,
			"Count":  fmt.Sprintf("%d", len(prs)),
			"Status": "Connected",
		})
	}

	return rows, nil
}

func (gv *GitHubView) getSelectedPRNumber() (int, bool) {
	row := gv.prsView.GetSelectedRowData()
	if len(row) == 0 {
		return 0, false
	}
	var num int
	fmt.Sscanf(stripColorTags(row[0]), "#%d", &num)
	return num, num > 0
}

func (gv *GitHubView) mergePR() {
	num, ok := gv.getSelectedPRNumber()
	if !ok {
		gv.prsView.Log("[yellow]No PR selected")
		return
	}

	items := [][]string{
		{"merge", "Create a merge commit"},
		{"squash", "Squash and merge"},
		{"rebase", "Rebase and merge"},
	}

	ui.ShowStandardListSelectorModal(
		gv.pages,
		gv.app,
		"Select Merge Method",
		items,
		func(index int, name string, cancelled bool) {
			if cancelled {
				gv.app.SetFocus(gv.prsView.GetTable())
				return
			}

			methods := []string{"merge", "squash", "rebase"}
			method := methods[index]

			ui.ShowStandardConfirmationModal(
				gv.pages,
				gv.app,
				"Merge Pull Request",
				fmt.Sprintf("Merge PR [yellow]#%d[white] using [green]%s[white] method?", num, method),
				func(confirmed bool) {
					if confirmed {
						gv.prsView.Log(fmt.Sprintf("[yellow]Merging PR #%d...", num))
						go func() {
							if err := gv.githubClient.MergePullRequest(num, method); err != nil {
								gv.app.QueueUpdateDraw(func() {
									gv.prsView.Log(fmt.Sprintf("[red]Failed to merge: %v", err))
								})
							} else {
								gv.app.QueueUpdateDraw(func() {
									gv.prsView.Log(fmt.Sprintf("[green]PR #%d merged successfully", num))
									gv.refresh()
								})
							}
						}()
					}
					gv.app.SetFocus(gv.prsView.GetTable())
				},
			)
		},
	)
}

func (gv *GitHubView) closePR() {
	num, ok := gv.getSelectedPRNumber()
	if !ok {
		gv.prsView.Log("[yellow]No PR selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Close Pull Request",
		fmt.Sprintf("Close PR [red]#%d[white]?", num),
		func(confirmed bool) {
			if confirmed {
				gv.prsView.Log(fmt.Sprintf("[yellow]Closing PR #%d...", num))
				go func() {
					if err := gv.githubClient.ClosePullRequest(num); err != nil {
						gv.app.QueueUpdateDraw(func() {
							gv.prsView.Log(fmt.Sprintf("[red]Failed to close: %v", err))
						})
					} else {
						gv.app.QueueUpdateDraw(func() {
							gv.prsView.Log(fmt.Sprintf("[green]PR #%d closed", num))
							gv.refresh()
						})
					}
				}()
			}
			gv.app.SetFocus(gv.prsView.GetTable())
		},
	)
}

func (gv *GitHubView) reopenPR() {
	num, ok := gv.getSelectedPRNumber()
	if !ok {
		gv.prsView.Log("[yellow]No PR selected")
		return
	}

	gv.prsView.Log(fmt.Sprintf("[yellow]Reopening PR #%d...", num))
	go func() {
		if err := gv.githubClient.ReopenPullRequest(num); err != nil {
			gv.app.QueueUpdateDraw(func() {
				gv.prsView.Log(fmt.Sprintf("[red]Failed to reopen: %v", err))
			})
		} else {
			gv.app.QueueUpdateDraw(func() {
				gv.prsView.Log(fmt.Sprintf("[green]PR #%d reopened", num))
				gv.refresh()
			})
		}
	}()
}

func (gv *GitHubView) approvePR() {
	num, ok := gv.getSelectedPRNumber()
	if !ok {
		gv.prsView.Log("[yellow]No PR selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Approve Pull Request",
		fmt.Sprintf("Approve PR [green]#%d[white]?", num),
		func(confirmed bool) {
			if confirmed {
				gv.prsView.Log(fmt.Sprintf("[yellow]Approving PR #%d...", num))
				go func() {
					if err := gv.githubClient.ApprovePullRequest(num); err != nil {
						gv.app.QueueUpdateDraw(func() {
							gv.prsView.Log(fmt.Sprintf("[red]Failed to approve: %v", err))
						})
					} else {
						gv.app.QueueUpdateDraw(func() {
							gv.prsView.Log(fmt.Sprintf("[green]PR #%d approved", num))
						})
					}
				}()
			}
			gv.app.SetFocus(gv.prsView.GetTable())
		},
	)
}

func (gv *GitHubView) viewPRChecks() {
	num, ok := gv.getSelectedPRNumber()
	if !ok {
		gv.prsView.Log("[yellow]No PR selected")
		return
	}

	gv.prsView.Log(fmt.Sprintf("[yellow]Loading checks for PR #%d...", num))

	go func() {
		checks, err := gv.githubClient.GetPRChecks(num)
		gv.app.QueueUpdateDraw(func() {
			if err != nil {
				gv.prsView.Log(fmt.Sprintf("[red]Failed to load checks: %v", err))
				return
			}

			var details strings.Builder
			details.WriteString(fmt.Sprintf("[yellow]Checks for PR #%d[white]\n\n", num))

			if len(checks) == 0 {
				details.WriteString("No checks found\n")
			}

			for _, check := range checks {
				status := check.Status
				if check.Conclusion != "" {
					status = check.Conclusion
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
				}
				details.WriteString(fmt.Sprintf("%s [white]%s - %s\n", icon, check.Name, status))
			}

			ui.ShowInfoModal(
				gv.pages,
				gv.app,
				fmt.Sprintf("PR #%d Checks", num),
				details.String(),
				func() {
					gv.app.SetFocus(gv.prsView.GetTable())
				},
			)
		})
	}()
}

func (gv *GitHubView) viewPRReviews() {
	num, ok := gv.getSelectedPRNumber()
	if !ok {
		gv.prsView.Log("[yellow]No PR selected")
		return
	}

	gv.prsView.Log(fmt.Sprintf("[yellow]Loading reviews for PR #%d...", num))

	go func() {
		reviews, err := gv.githubClient.GetPRReviews(num)
		gv.app.QueueUpdateDraw(func() {
			if err != nil {
				gv.prsView.Log(fmt.Sprintf("[red]Failed to load reviews: %v", err))
				return
			}

			var details strings.Builder
			details.WriteString(fmt.Sprintf("[yellow]Reviews for PR #%d[white]\n\n", num))

			if len(reviews) == 0 {
				details.WriteString("No reviews yet\n")
			}

			for _, review := range reviews {
				icon := "[gray]\u2022"
				switch review.State {
				case "APPROVED":
					icon = "[green]\u2714"
				case "CHANGES_REQUESTED":
					icon = "[red]\u2718"
				case "COMMENTED":
					icon = "[blue]\u2022"
				case "DISMISSED":
					icon = "[gray]\u2718"
				}
				details.WriteString(fmt.Sprintf("%s [aqua]%s[white] - %s", icon, review.User, review.State))
				if review.Body != "" {
					body := review.Body
					if len(body) > 100 {
						body = body[:100] + "..."
					}
					details.WriteString(fmt.Sprintf("\n    %s", body))
				}
				details.WriteString(fmt.Sprintf("\n    [gray]%s[white]\n\n", formatAge(review.CreatedAt)))
			}

			ui.ShowInfoModal(
				gv.pages,
				gv.app,
				fmt.Sprintf("PR #%d Reviews", num),
				details.String(),
				func() {
					gv.app.SetFocus(gv.prsView.GetTable())
				},
			)
		})
	}()
}

func (gv *GitHubView) togglePRState() {
	switch gv.prState {
	case "open":
		gv.prState = "closed"
	case "closed":
		gv.prState = "all"
	default:
		gv.prState = "open"
	}
	gv.prsView.Log(fmt.Sprintf("[blue]Showing %s PRs", gv.prState))
	gv.refresh()
}

func (gv *GitHubView) showPRDetails() {
	num, ok := gv.getSelectedPRNumber()
	if !ok {
		return
	}

	gv.prsView.Log(fmt.Sprintf("[yellow]Loading PR #%d details...", num))

	go func() {
		pr, err := gv.githubClient.GetPullRequest(num)
		gv.app.QueueUpdateDraw(func() {
			if err != nil {
				gv.prsView.Log(fmt.Sprintf("[red]Failed to load PR: %v", err))
				return
			}

			var details strings.Builder
			details.WriteString(fmt.Sprintf("[yellow]PR #%d: %s[white]\n\n", pr.Number, pr.Title))
			details.WriteString(fmt.Sprintf("[green]State:[white]     %s\n", pr.State))
			if pr.Draft {
				details.WriteString("[green]Draft:[white]     Yes\n")
			}
			details.WriteString(fmt.Sprintf("[green]Author:[white]    %s\n", pr.Author))
			details.WriteString(fmt.Sprintf("[green]Branch:[white]    %s → %s\n", pr.Branch, pr.Base))
			details.WriteString(fmt.Sprintf("[green]Mergeable:[white] %s\n", pr.Mergeable))
			details.WriteString(fmt.Sprintf("[green]Changes:[white]   +%d / -%d\n", pr.Additions, pr.Deletions))
			details.WriteString(fmt.Sprintf("[green]Commits:[white]   %d\n", pr.Commits))
			details.WriteString(fmt.Sprintf("[green]Comments:[white]  %d\n", pr.Comments))
			details.WriteString(fmt.Sprintf("[green]Created:[white]   %s\n", formatAge(pr.CreatedAt)))
			details.WriteString(fmt.Sprintf("[green]Updated:[white]   %s\n", formatAge(pr.UpdatedAt)))

			if len(pr.Labels) > 0 {
				details.WriteString(fmt.Sprintf("\n[yellow]Labels:[white]\n"))
				for _, label := range pr.Labels {
					details.WriteString(fmt.Sprintf("  \u2022 %s\n", label))
				}
			}

			if len(pr.Reviewers) > 0 {
				details.WriteString(fmt.Sprintf("\n[yellow]Requested Reviewers:[white]\n"))
				for _, reviewer := range pr.Reviewers {
					details.WriteString(fmt.Sprintf("  \u2022 %s\n", reviewer))
				}
			}

			details.WriteString(fmt.Sprintf("\n[green]URL:[white] %s\n", pr.URL))

			ui.ShowInfoModal(
				gv.pages,
				gv.app,
				fmt.Sprintf("PR #%d", pr.Number),
				details.String(),
				func() {
					gv.app.SetFocus(gv.prsView.GetTable())
				},
			)
		})
	}()
}
