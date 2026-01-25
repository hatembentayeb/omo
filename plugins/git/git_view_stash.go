package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (gv *GitView) newStashView() *ui.Cores {
	cores := ui.NewCores(gv.app, "Git Stash")
	cores.SetTableHeaders([]string{"Index", "Branch", "Message"})
	cores.SetRefreshCallback(gv.refreshStashData)
	cores.SetSelectionKey("Index")

	// Navigation key bindings
	cores.AddKeyBinding("G", "Repos", gv.showRepos)
	cores.AddKeyBinding("S", "Status", gv.showStatus)
	cores.AddKeyBinding("L", "Commits", gv.showCommits)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("M", "Remotes", gv.showRemotes)
	cores.AddKeyBinding("T", "Tags", gv.showTags)
	cores.AddKeyBinding("H", "Stash", gv.showStash)

	// Stash action key bindings
	cores.AddKeyBinding("N", "New Stash", gv.createStash)
	cores.AddKeyBinding("A", "Apply", gv.applyStash)
	cores.AddKeyBinding("P", "Pop", gv.popStash)
	cores.AddKeyBinding("D", "Drop", gv.dropStash)
	cores.AddKeyBinding("V", "View", gv.viewStash)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	cores.RegisterHandlers()
	return cores
}

func (gv *GitView) refreshStashData() ([][]string, error) {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		return [][]string{{"", "", "No repository selected"}}, nil
	}

	stashes, err := gv.gitClient.GetStashList(repo.Path)
	if err != nil {
		return [][]string{{"", "", fmt.Sprintf("Error: %v", err)}}, err
	}

	if len(stashes) == 0 {
		return [][]string{{"", "", "No stashes"}}, nil
	}

	rows := make([][]string, len(stashes))
	for i, stash := range stashes {
		rows[i] = []string{
			fmt.Sprintf("[yellow]%s[white]", stash.Index),
			stash.Branch,
			stash.Message,
		}
	}

	if gv.stashView != nil {
		gv.stashView.SetInfoText(fmt.Sprintf("[green]Git Stash[white]\nRepo: %s\nStashes: %d",
			repo.Name, len(stashes)))
	}

	return rows, nil
}

func (gv *GitView) getSelectedStashIndex() (string, bool) {
	if gv.stashView == nil {
		return "", false
	}
	row := gv.stashView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	index := stripColorCodes(row[0])
	return index, index != ""
}

func (gv *GitView) createStash() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.stashView.Log("[yellow]No repository selected")
		return
	}

	ui.ShowCompactStyledInputModal(
		gv.pages,
		gv.app,
		"Create Stash",
		"Message (optional)",
		"",
		40,
		nil,
		func(message string, cancelled bool) {
			if cancelled {
				gv.app.SetFocus(gv.stashView.GetTable())
				return
			}

			if err := gv.gitClient.CreateStash(repo.Path, message); err != nil {
				gv.stashView.Log(fmt.Sprintf("[red]Failed to create stash: %v", err))
			} else {
				gv.stashView.Log("[green]Created new stash")
				gv.refresh()
			}
			gv.app.SetFocus(gv.stashView.GetTable())
		},
	)
}

func (gv *GitView) applyStash() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.stashView.Log("[yellow]No repository selected")
		return
	}

	index, ok := gv.getSelectedStashIndex()
	if !ok {
		gv.stashView.Log("[yellow]No stash selected")
		return
	}

	gv.stashView.Log(fmt.Sprintf("[yellow]Applying stash %s...", index))
	if err := gv.gitClient.ApplyStash(repo.Path, index); err != nil {
		gv.stashView.Log(fmt.Sprintf("[red]Apply failed: %v", err))
	} else {
		gv.stashView.Log(fmt.Sprintf("[green]Applied stash %s", index))
		gv.refresh()
	}
}

func (gv *GitView) popStash() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.stashView.Log("[yellow]No repository selected")
		return
	}

	index, ok := gv.getSelectedStashIndex()
	if !ok {
		gv.stashView.Log("[yellow]No stash selected")
		return
	}

	gv.stashView.Log(fmt.Sprintf("[yellow]Popping stash %s...", index))
	if err := gv.gitClient.PopStash(repo.Path, index); err != nil {
		gv.stashView.Log(fmt.Sprintf("[red]Pop failed: %v", err))
	} else {
		gv.stashView.Log(fmt.Sprintf("[green]Popped stash %s", index))
		gv.refresh()
	}
}

func (gv *GitView) dropStash() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.stashView.Log("[yellow]No repository selected")
		return
	}

	index, ok := gv.getSelectedStashIndex()
	if !ok {
		gv.stashView.Log("[yellow]No stash selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Drop Stash",
		fmt.Sprintf("Drop stash [red]%s[white]?\nThis cannot be undone!", index),
		func(confirmed bool) {
			if confirmed {
				if err := gv.gitClient.DropStash(repo.Path, index); err != nil {
					gv.stashView.Log(fmt.Sprintf("[red]Drop failed: %v", err))
				} else {
					gv.stashView.Log(fmt.Sprintf("[green]Dropped stash %s", index))
					gv.refresh()
				}
			}
			gv.app.SetFocus(gv.stashView.GetTable())
		},
	)
}

func (gv *GitView) viewStash() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.stashView.Log("[yellow]No repository selected")
		return
	}

	index, ok := gv.getSelectedStashIndex()
	if !ok {
		gv.stashView.Log("[yellow]No stash selected")
		return
	}

	content, err := gv.gitClient.ShowStash(repo.Path, index)
	if err != nil {
		gv.stashView.Log(fmt.Sprintf("[red]Failed to show stash: %v", err))
		return
	}

	coloredContent := colorizeDiff(content)

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		fmt.Sprintf("Stash: %s", index),
		coloredContent,
		func() {
			gv.app.SetFocus(gv.stashView.GetTable())
		},
	)
}
