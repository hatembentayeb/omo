package main

import (
	"fmt"
	"strings"

	"omo/pkg/ui"
)

func (gv *GitView) newStatusView() *ui.Cores {
	cores := ui.NewCores(gv.app, "Git Status")
	cores.SetTableHeaders([]string{"Status", "File", "Type"})
	cores.SetRefreshCallback(gv.refreshStatusData)
	cores.SetSelectionKey("File")

	// Navigation key bindings
	cores.AddKeyBinding("G", "Repos", gv.showRepos)
	cores.AddKeyBinding("S", "Status", gv.showStatus)
	cores.AddKeyBinding("L", "Commits", gv.showCommits)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("M", "Remotes", gv.showRemotes)
	cores.AddKeyBinding("T", "Tags", gv.showTags)
	cores.AddKeyBinding("H", "Stash", gv.showStash)

	// Status action key bindings
	cores.AddKeyBinding("A", "Stage", gv.stageSelectedFile)
	cores.AddKeyBinding("U", "Unstage", gv.unstageSelectedFile)
	cores.AddKeyBinding("D", "Diff", gv.showFileDiff)
	cores.AddKeyBinding("R", "Restore", gv.restoreSelectedFile)
	cores.AddKeyBinding("C", "Commit", gv.showCommitDialog)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	cores.RegisterHandlers()
	return cores
}

func (gv *GitView) refreshStatusData() ([][]string, error) {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		return [][]string{{"", "No repository selected", ""}}, nil
	}

	files, err := gv.gitClient.GetStatusFiles(repo.Path)
	if err != nil {
		return [][]string{{"", fmt.Sprintf("Error: %v", err), ""}}, err
	}

	if len(files) == 0 {
		return [][]string{{"[green]✓[white]", "Working tree clean", ""}}, nil
	}

	rows := make([][]string, len(files))
	for i, file := range files {
		statusColor := "white"
		switch file.Status {
		case "M", "MM":
			statusColor = "yellow"
		case "A":
			statusColor = "green"
		case "D":
			statusColor = "red"
		case "?":
			statusColor = "gray"
		case "R":
			statusColor = "cyan"
		}

		rows[i] = []string{
			fmt.Sprintf("[%s]%s[white]", statusColor, file.Status),
			file.Path,
			file.Type,
		}
	}

	if gv.statusView != nil {
		gv.statusView.SetInfoText(fmt.Sprintf("[green]Git Status[white]\nRepo: %s\nBranch: %s\nChanges: %d",
			repo.Name, repo.Branch, len(files)))
	}

	return rows, nil
}

func (gv *GitView) getSelectedFile() (string, bool) {
	if gv.statusView == nil {
		return "", false
	}
	row := gv.statusView.GetSelectedRowData()
	if len(row) < 2 {
		return "", false
	}
	return row[1], true
}

func (gv *GitView) stageSelectedFile() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.statusView.Log("[yellow]No repository selected")
		return
	}

	file, ok := gv.getSelectedFile()
	if !ok || file == "Working tree clean" || file == "No repository selected" {
		gv.statusView.Log("[yellow]No file selected")
		return
	}

	if err := gv.gitClient.StageFile(repo.Path, file); err != nil {
		gv.statusView.Log(fmt.Sprintf("[red]Failed to stage: %v", err))
		return
	}

	gv.statusView.Log(fmt.Sprintf("[green]Staged: %s", file))
	gv.refresh()
}

func (gv *GitView) unstageSelectedFile() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.statusView.Log("[yellow]No repository selected")
		return
	}

	file, ok := gv.getSelectedFile()
	if !ok {
		gv.statusView.Log("[yellow]No file selected")
		return
	}

	if err := gv.gitClient.UnstageFile(repo.Path, file); err != nil {
		gv.statusView.Log(fmt.Sprintf("[red]Failed to unstage: %v", err))
		return
	}

	gv.statusView.Log(fmt.Sprintf("[green]Unstaged: %s", file))
	gv.refresh()
}

func (gv *GitView) showFileDiff() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.statusView.Log("[yellow]No repository selected")
		return
	}

	file, ok := gv.getSelectedFile()
	if !ok {
		gv.statusView.Log("[yellow]No file selected")
		return
	}

	diff, err := gv.gitClient.GetFileDiff(repo.Path, file)
	if err != nil {
		gv.statusView.Log(fmt.Sprintf("[red]Failed to get diff: %v", err))
		return
	}

	// Color the diff output
	coloredDiff := colorizeDiff(diff)

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		fmt.Sprintf("Diff: %s", file),
		coloredDiff,
		func() {
			gv.app.SetFocus(gv.statusView.GetTable())
		},
	)
}

func (gv *GitView) restoreSelectedFile() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.statusView.Log("[yellow]No repository selected")
		return
	}

	file, ok := gv.getSelectedFile()
	if !ok {
		gv.statusView.Log("[yellow]No file selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Restore File",
		fmt.Sprintf("Discard changes to [red]%s[white]?\nThis cannot be undone!", file),
		func(confirmed bool) {
			if confirmed {
				if err := gv.gitClient.RestoreFile(repo.Path, file); err != nil {
					gv.statusView.Log(fmt.Sprintf("[red]Failed to restore: %v", err))
				} else {
					gv.statusView.Log(fmt.Sprintf("[green]Restored: %s", file))
					gv.refresh()
				}
			}
			gv.app.SetFocus(gv.statusView.GetTable())
		},
	)
}

func (gv *GitView) showCommitDialog() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.statusView.Log("[yellow]No repository selected")
		return
	}

	ui.ShowCompactStyledInputModal(
		gv.pages,
		gv.app,
		"Commit Changes",
		"Message",
		"",
		50,
		nil,
		func(message string, cancelled bool) {
			if cancelled || message == "" {
				gv.app.SetFocus(gv.statusView.GetTable())
				return
			}

			gv.statusView.Log("[yellow]Committing...")
			if err := gv.gitClient.Commit(repo.Path, message); err != nil {
				gv.statusView.Log(fmt.Sprintf("[red]Commit failed: %v", err))
			} else {
				gv.statusView.Log(fmt.Sprintf("[green]Committed: %s", message))
				gv.refresh()
			}
			gv.app.SetFocus(gv.statusView.GetTable())
		},
	)
}

func colorizeDiff(diff string) string {
	var result strings.Builder
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			result.WriteString(fmt.Sprintf("[green]%s[white]\n", line))
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			result.WriteString(fmt.Sprintf("[red]%s[white]\n", line))
		} else if strings.HasPrefix(line, "@@") {
			result.WriteString(fmt.Sprintf("[cyan]%s[white]\n", line))
		} else {
			result.WriteString(line + "\n")
		}
	}
	return result.String()
}
