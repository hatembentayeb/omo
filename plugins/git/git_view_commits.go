package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (gv *GitView) newCommitsView() *ui.Cores {
	cores := ui.NewCores(gv.app, "Git Commits")
	cores.SetTableHeaders([]string{"Hash", "Author", "Date", "Message"})
	cores.SetRefreshCallback(gv.refreshCommitsData)
	cores.SetSelectionKey("Hash")

	// Navigation key bindings
	cores.AddKeyBinding("G", "Repos", gv.showRepos)
	cores.AddKeyBinding("S", "Status", gv.showStatus)
	cores.AddKeyBinding("L", "Commits", gv.showCommits)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("M", "Remotes", gv.showRemotes)
	cores.AddKeyBinding("T", "Tags", gv.showTags)
	cores.AddKeyBinding("H", "Stash", gv.showStash)

	// Commits action key bindings
	cores.AddKeyBinding("D", "Diff", gv.showCommitDiff)
	cores.AddKeyBinding("C", "Checkout", gv.checkoutCommit)
	cores.AddKeyBinding("R", "Revert", gv.revertCommit)
	cores.AddKeyBinding("P", "Cherry-pick", gv.cherryPickCommit)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	// Set Enter key to show commit details
	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		gv.showCommitDetails()
	})

	cores.RegisterHandlers()
	return cores
}

func (gv *GitView) refreshCommitsData() ([][]string, error) {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		return [][]string{{"", "", "", "No repository selected"}}, nil
	}

	commits, err := gv.gitClient.GetCommits(repo.Path, 50)
	if err != nil {
		return [][]string{{"", "", "", fmt.Sprintf("Error: %v", err)}}, err
	}

	rows := make([][]string, len(commits))
	for i, commit := range commits {
		rows[i] = []string{
			fmt.Sprintf("[yellow]%s[white]", commit.Hash),
			commit.Author,
			commit.Date,
			truncateString(commit.Message, 50),
		}
	}

	if gv.commitsView != nil {
		gv.commitsView.SetInfoText(fmt.Sprintf("[green]Git Commits[white]\nRepo: %s\nBranch: %s\nCommits: %d",
			repo.Name, repo.Branch, len(commits)))
	}

	return rows, nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func (gv *GitView) getSelectedCommitHash() (string, bool) {
	if gv.commitsView == nil {
		return "", false
	}
	row := gv.commitsView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	// Remove color codes from hash
	hash := row[0]
	// Simple extraction - hash is 7 chars
	if len(hash) >= 7 {
		// Find the actual hash in the colored string
		for i := 0; i < len(hash)-6; i++ {
			if isHexChar(hash[i]) {
				end := i + 7
				if end <= len(hash) {
					return hash[i:end], true
				}
			}
		}
	}
	return "", false
}

func isHexChar(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func (gv *GitView) showCommitDetails() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		return
	}

	hash, ok := gv.getSelectedCommitHash()
	if !ok {
		return
	}

	details, err := gv.gitClient.GetCommitDetails(repo.Path, hash)
	if err != nil {
		gv.commitsView.Log(fmt.Sprintf("[red]Failed to get commit details: %v", err))
		return
	}

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		fmt.Sprintf("Commit: %s", hash),
		details,
		func() {
			gv.app.SetFocus(gv.commitsView.GetTable())
		},
	)
}

func (gv *GitView) showCommitDiff() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.commitsView.Log("[yellow]No repository selected")
		return
	}

	hash, ok := gv.getSelectedCommitHash()
	if !ok {
		gv.commitsView.Log("[yellow]No commit selected")
		return
	}

	diff, err := gv.gitClient.GetCommitDiff(repo.Path, hash)
	if err != nil {
		gv.commitsView.Log(fmt.Sprintf("[red]Failed to get diff: %v", err))
		return
	}

	coloredDiff := colorizeDiff(diff)

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		fmt.Sprintf("Diff: %s", hash),
		coloredDiff,
		func() {
			gv.app.SetFocus(gv.commitsView.GetTable())
		},
	)
}

func (gv *GitView) checkoutCommit() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.commitsView.Log("[yellow]No repository selected")
		return
	}

	hash, ok := gv.getSelectedCommitHash()
	if !ok {
		gv.commitsView.Log("[yellow]No commit selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Checkout Commit",
		fmt.Sprintf("Checkout commit [yellow]%s[white]?\nThis will put you in detached HEAD state.", hash),
		func(confirmed bool) {
			if confirmed {
				if err := gv.gitClient.Checkout(repo.Path, hash); err != nil {
					gv.commitsView.Log(fmt.Sprintf("[red]Checkout failed: %v", err))
				} else {
					gv.commitsView.Log(fmt.Sprintf("[green]Checked out %s", hash))
					gv.refresh()
				}
			}
			gv.app.SetFocus(gv.commitsView.GetTable())
		},
	)
}

func (gv *GitView) revertCommit() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.commitsView.Log("[yellow]No repository selected")
		return
	}

	hash, ok := gv.getSelectedCommitHash()
	if !ok {
		gv.commitsView.Log("[yellow]No commit selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Revert Commit",
		fmt.Sprintf("Revert commit [yellow]%s[white]?\nThis will create a new commit that undoes changes.", hash),
		func(confirmed bool) {
			if confirmed {
				if err := gv.gitClient.RevertCommit(repo.Path, hash); err != nil {
					gv.commitsView.Log(fmt.Sprintf("[red]Revert failed: %v", err))
				} else {
					gv.commitsView.Log(fmt.Sprintf("[green]Reverted %s", hash))
					gv.refresh()
				}
			}
			gv.app.SetFocus(gv.commitsView.GetTable())
		},
	)
}

func (gv *GitView) cherryPickCommit() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.commitsView.Log("[yellow]No repository selected")
		return
	}

	hash, ok := gv.getSelectedCommitHash()
	if !ok {
		gv.commitsView.Log("[yellow]No commit selected")
		return
	}

	if err := gv.gitClient.CherryPick(repo.Path, hash); err != nil {
		gv.commitsView.Log(fmt.Sprintf("[red]Cherry-pick failed: %v", err))
	} else {
		gv.commitsView.Log(fmt.Sprintf("[green]Cherry-picked %s", hash))
		gv.refresh()
	}
}
