package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (gv *GitView) newRemotesView() *ui.Cores {
	cores := ui.NewCores(gv.app, "Git Remotes")
	cores.SetTableHeaders([]string{"Remote", "Fetch URL", "Push URL"})
	cores.SetRefreshCallback(gv.refreshRemotesData)
	cores.SetSelectionKey("Remote")

	// Navigation key bindings
	cores.AddKeyBinding("G", "Repos", gv.showRepos)
	cores.AddKeyBinding("S", "Status", gv.showStatus)
	cores.AddKeyBinding("L", "Commits", gv.showCommits)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("M", "Remotes", gv.showRemotes)
	cores.AddKeyBinding("T", "Tags", gv.showTags)
	cores.AddKeyBinding("H", "Stash", gv.showStash)

	// Remote action key bindings
	cores.AddKeyBinding("A", "Add", gv.addRemote)
	cores.AddKeyBinding("D", "Remove", gv.removeRemote)
	cores.AddKeyBinding("F", "Fetch", gv.fetchRemote)
	cores.AddKeyBinding("P", "Prune", gv.pruneRemote)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	cores.RegisterHandlers()
	return cores
}

func (gv *GitView) refreshRemotesData() ([][]string, error) {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		return [][]string{{"No repository selected", "", ""}}, nil
	}

	remotes, err := gv.gitClient.GetRemotesInfo(repo.Path)
	if err != nil {
		return [][]string{{fmt.Sprintf("Error: %v", err), "", ""}}, err
	}

	if len(remotes) == 0 {
		return [][]string{{"No remotes configured", "", ""}}, nil
	}

	rows := make([][]string, len(remotes))
	for i, remote := range remotes {
		rows[i] = []string{
			fmt.Sprintf("[yellow]%s[white]", remote.Name),
			remote.FetchURL,
			remote.PushURL,
		}
	}

	if gv.remotesView != nil {
		gv.remotesView.SetInfoText(fmt.Sprintf("[green]Git Remotes[white]\nRepo: %s\nRemotes: %d",
			repo.Name, len(remotes)))
	}

	return rows, nil
}

func (gv *GitView) getSelectedRemoteName() (string, bool) {
	if gv.remotesView == nil {
		return "", false
	}
	row := gv.remotesView.GetSelectedRowData()
	if len(row) == 0 {
		return "", false
	}
	name := stripColorCodes(row[0])
	return name, name != "" && name != "No repository selected" && name != "No remotes configured"
}

func (gv *GitView) addRemote() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.remotesView.Log("[yellow]No repository selected")
		return
	}

	ui.ShowCompactStyledInputModal(
		gv.pages,
		gv.app,
		"Add Remote",
		"Remote Name",
		"",
		20,
		nil,
		func(remoteName string, cancelled bool) {
			if cancelled || remoteName == "" {
				gv.app.SetFocus(gv.remotesView.GetTable())
				return
			}

			ui.ShowCompactStyledInputModal(
				gv.pages,
				gv.app,
				"Add Remote",
				"Remote URL",
				"",
				50,
				nil,
				func(remoteURL string, cancelled bool) {
					if cancelled || remoteURL == "" {
						gv.app.SetFocus(gv.remotesView.GetTable())
						return
					}

					if err := gv.gitClient.AddRemote(repo.Path, remoteName, remoteURL); err != nil {
						gv.remotesView.Log(fmt.Sprintf("[red]Failed to add remote: %v", err))
					} else {
						gv.remotesView.Log(fmt.Sprintf("[green]Added remote %s", remoteName))
						gv.refresh()
					}
					gv.app.SetFocus(gv.remotesView.GetTable())
				},
			)
		},
	)
}

func (gv *GitView) removeRemote() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.remotesView.Log("[yellow]No repository selected")
		return
	}

	remoteName, ok := gv.getSelectedRemoteName()
	if !ok {
		gv.remotesView.Log("[yellow]No remote selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Remove Remote",
		fmt.Sprintf("Remove remote [red]%s[white]?", remoteName),
		func(confirmed bool) {
			if confirmed {
				if err := gv.gitClient.RemoveRemote(repo.Path, remoteName); err != nil {
					gv.remotesView.Log(fmt.Sprintf("[red]Failed to remove remote: %v", err))
				} else {
					gv.remotesView.Log(fmt.Sprintf("[green]Removed remote %s", remoteName))
					gv.refresh()
				}
			}
			gv.app.SetFocus(gv.remotesView.GetTable())
		},
	)
}

func (gv *GitView) fetchRemote() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.remotesView.Log("[yellow]No repository selected")
		return
	}

	remoteName, ok := gv.getSelectedRemoteName()
	if !ok {
		gv.remotesView.Log("[yellow]No remote selected")
		return
	}

	gv.remotesView.Log(fmt.Sprintf("[yellow]Fetching from %s...", remoteName))

	go func() {
		result, err := gv.gitClient.FetchRemote(repo.Path, remoteName)
		gv.app.QueueUpdateDraw(func() {
			if err != nil {
				gv.remotesView.Log(fmt.Sprintf("[red]Fetch failed: %v", err))
			} else {
				gv.remotesView.Log(fmt.Sprintf("[green]%s", result))
			}
		})
	}()
}

func (gv *GitView) pruneRemote() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.remotesView.Log("[yellow]No repository selected")
		return
	}

	remoteName, ok := gv.getSelectedRemoteName()
	if !ok {
		gv.remotesView.Log("[yellow]No remote selected")
		return
	}

	gv.remotesView.Log(fmt.Sprintf("[yellow]Pruning %s...", remoteName))

	if err := gv.gitClient.PruneRemote(repo.Path, remoteName); err != nil {
		gv.remotesView.Log(fmt.Sprintf("[red]Prune failed: %v", err))
	} else {
		gv.remotesView.Log(fmt.Sprintf("[green]Pruned stale branches from %s", remoteName))
		gv.refresh()
	}
}
