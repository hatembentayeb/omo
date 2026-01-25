package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (gv *GitView) newBranchesView() *ui.Cores {
	cores := ui.NewCores(gv.app, "Git Branches")
	cores.SetTableHeaders([]string{"Branch", "Current", "Tracking", "Ahead", "Behind"})
	cores.SetRefreshCallback(gv.refreshBranchesData)
	cores.SetSelectionKey("Branch")

	// Navigation key bindings
	cores.AddKeyBinding("G", "Repos", gv.showRepos)
	cores.AddKeyBinding("S", "Status", gv.showStatus)
	cores.AddKeyBinding("L", "Commits", gv.showCommits)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("M", "Remotes", gv.showRemotes)
	cores.AddKeyBinding("T", "Tags", gv.showTags)
	cores.AddKeyBinding("H", "Stash", gv.showStash)

	// Branch action key bindings
	cores.AddKeyBinding("C", "Checkout", gv.checkoutSelectedBranch)
	cores.AddKeyBinding("N", "New", gv.createBranchFromView)
	cores.AddKeyBinding("D", "Delete", gv.deleteSelectedBranch)
	cores.AddKeyBinding("R", "Rename", gv.renameSelectedBranch)
	cores.AddKeyBinding("E", "Merge", gv.mergeSelectedBranch)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	cores.RegisterHandlers()
	return cores
}

func (gv *GitView) refreshBranchesData() ([][]string, error) {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		return [][]string{{"No repository selected", "", "", "", ""}}, nil
	}

	branches, err := gv.gitClient.GetBranchesInfo(repo.Path)
	if err != nil {
		return [][]string{{fmt.Sprintf("Error: %v", err), "", "", "", ""}}, err
	}

	rows := make([][]string, len(branches))
	for i, branch := range branches {
		current := ""
		branchName := branch.Name
		if branch.IsCurrent {
			current = "[green]✓[white]"
			branchName = fmt.Sprintf("[green]%s[white]", branch.Name)
		}

		rows[i] = []string{
			branchName,
			current,
			branch.Tracking,
			formatAheadBehind(branch.Ahead, "cyan"),
			formatAheadBehind(branch.Behind, "red"),
		}
	}

	if gv.branchesView != nil {
		gv.branchesView.SetInfoText(fmt.Sprintf("[green]Git Branches[white]\nRepo: %s\nBranches: %d",
			repo.Name, len(branches)))
	}

	return rows, nil
}

func formatAheadBehind(count int, color string) string {
	if count > 0 {
		return fmt.Sprintf("[%s]%d[white]", color, count)
	}
	return "-"
}

func (gv *GitView) getSelectedBranchName() (string, bool) {
	if gv.branchesView == nil {
		return "", false
	}
	row := gv.branchesView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	// Remove color codes if present
	name := row[0]
	// Strip [green] and [white] tags
	name = stripColorCodes(name)
	return name, name != "" && name != "No repository selected"
}

func stripColorCodes(s string) string {
	result := s
	colors := []string{"[green]", "[red]", "[yellow]", "[cyan]", "[white]", "[gray]", "[blue]"}
	for _, color := range colors {
		for {
			idx := indexOf(result, color)
			if idx == -1 {
				break
			}
			result = result[:idx] + result[idx+len(color):]
		}
	}
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func (gv *GitView) checkoutSelectedBranch() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.branchesView.Log("[yellow]No repository selected")
		return
	}

	branch, ok := gv.getSelectedBranchName()
	if !ok {
		gv.branchesView.Log("[yellow]No branch selected")
		return
	}

	gv.branchesView.Log(fmt.Sprintf("[yellow]Checking out %s...", branch))
	if err := gv.gitClient.Checkout(repo.Path, branch); err != nil {
		gv.branchesView.Log(fmt.Sprintf("[red]Checkout failed: %v", err))
	} else {
		gv.branchesView.Log(fmt.Sprintf("[green]Checked out %s", branch))
		gv.refresh()
	}
}

func (gv *GitView) createBranchFromView() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.branchesView.Log("[yellow]No repository selected")
		return
	}

	ui.ShowCompactStyledInputModal(
		gv.pages,
		gv.app,
		"Create Branch",
		"Branch Name",
		"",
		30,
		nil,
		func(branchName string, cancelled bool) {
			if cancelled || branchName == "" {
				gv.app.SetFocus(gv.branchesView.GetTable())
				return
			}

			if err := gv.gitClient.CreateBranch(repo.Path, branchName); err != nil {
				gv.branchesView.Log(fmt.Sprintf("[red]Failed to create branch: %v", err))
			} else {
				gv.branchesView.Log(fmt.Sprintf("[green]Created and checked out %s", branchName))
				gv.refresh()
			}
			gv.app.SetFocus(gv.branchesView.GetTable())
		},
	)
}

func (gv *GitView) deleteSelectedBranch() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.branchesView.Log("[yellow]No repository selected")
		return
	}

	branch, ok := gv.getSelectedBranchName()
	if !ok {
		gv.branchesView.Log("[yellow]No branch selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Delete Branch",
		fmt.Sprintf("Delete branch [red]%s[white]?\nThis cannot be undone!", branch),
		func(confirmed bool) {
			if confirmed {
				if err := gv.gitClient.DeleteBranch(repo.Path, branch); err != nil {
					gv.branchesView.Log(fmt.Sprintf("[red]Delete failed: %v", err))
				} else {
					gv.branchesView.Log(fmt.Sprintf("[green]Deleted branch %s", branch))
					gv.refresh()
				}
			}
			gv.app.SetFocus(gv.branchesView.GetTable())
		},
	)
}

func (gv *GitView) renameSelectedBranch() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.branchesView.Log("[yellow]No repository selected")
		return
	}

	oldName, ok := gv.getSelectedBranchName()
	if !ok {
		gv.branchesView.Log("[yellow]No branch selected")
		return
	}

	ui.ShowCompactStyledInputModal(
		gv.pages,
		gv.app,
		"Rename Branch",
		"New Name",
		oldName,
		30,
		nil,
		func(newName string, cancelled bool) {
			if cancelled || newName == "" || newName == oldName {
				gv.app.SetFocus(gv.branchesView.GetTable())
				return
			}

			if err := gv.gitClient.RenameBranch(repo.Path, oldName, newName); err != nil {
				gv.branchesView.Log(fmt.Sprintf("[red]Rename failed: %v", err))
			} else {
				gv.branchesView.Log(fmt.Sprintf("[green]Renamed %s to %s", oldName, newName))
				gv.refresh()
			}
			gv.app.SetFocus(gv.branchesView.GetTable())
		},
	)
}

func (gv *GitView) mergeSelectedBranch() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.branchesView.Log("[yellow]No repository selected")
		return
	}

	branch, ok := gv.getSelectedBranchName()
	if !ok {
		gv.branchesView.Log("[yellow]No branch selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Merge Branch",
		fmt.Sprintf("Merge [yellow]%s[white] into current branch?", branch),
		func(confirmed bool) {
			if confirmed {
				gv.branchesView.Log(fmt.Sprintf("[yellow]Merging %s...", branch))
				if err := gv.gitClient.MergeBranch(repo.Path, branch); err != nil {
					gv.branchesView.Log(fmt.Sprintf("[red]Merge failed: %v", err))
				} else {
					gv.branchesView.Log(fmt.Sprintf("[green]Merged %s", branch))
					gv.refresh()
				}
			}
			gv.app.SetFocus(gv.branchesView.GetTable())
		},
	)
}
