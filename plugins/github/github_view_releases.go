package main

import (
	"fmt"
	"strings"

	"omo/pkg/ui"
)

func (gv *GitHubView) newReleasesView() *ui.CoreView {
	cores := ui.NewCoreView(gv.app, "Releases")
	cores.SetTableHeaders([]string{"Tag", "Name", "Status", "Author", "Assets", "Published"})
	cores.SetRefreshCallback(gv.refreshReleasesData)
	cores.SetSelectionKey("Tag")

	cores.AddKeyBinding("L", "Repos", gv.showRepos)
	cores.AddKeyBinding("P", "PRs", gv.showPRs)
	cores.AddKeyBinding("W", "Workflows", gv.showWorkflows)
	cores.AddKeyBinding("A", "Runs", gv.showRuns)
	cores.AddKeyBinding("E", "Env Vars", gv.showEnvVars)
	cores.AddKeyBinding("S", "Secrets", gv.showSecrets)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("F", "Releases", gv.showReleases)

	cores.AddKeyBinding("D", "Delete", gv.deleteRelease)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	cores.SetRowSelectedCallback(func(row int) {
		tableData := cores.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			cores.Log(fmt.Sprintf("[blue]Selected release: %s - %s", tableData[row][0], tableData[row][1]))
		}
	})

	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		gv.showReleaseDetails()
	})

	cores.RegisterHandlers()
	return cores
}

func (gv *GitHubView) refreshReleasesData() ([][]string, error) {
	if !gv.githubClient.HasActiveRepo() {
		return [][]string{}, fmt.Errorf("no repository selected")
	}

	releases, err := gv.githubClient.ListReleases()
	if err != nil {
		return [][]string{}, err
	}

	gv.cachedReleases = releases

	rows := make([][]string, len(releases))
	for i, r := range releases {
		rows[i] = r.GetTableRow()
	}

	if gv.activeRepo != nil {
		gv.releasesView.SetInfoMap(map[string]string{
			"Repo":     gv.activeRepo.FullName,
			"Releases": fmt.Sprintf("%d", len(releases)),
			"Status":   "Connected",
		})
	}

	return rows, nil
}

func (gv *GitHubView) getSelectedRelease() (*Release, bool) {
	row := gv.releasesView.GetSelectedRowData()
	if len(row) == 0 {
		return nil, false
	}

	tag := stripColorTags(row[0])
	for _, r := range gv.cachedReleases {
		if r.TagName == tag {
			return &r, true
		}
	}
	return nil, false
}

func (gv *GitHubView) deleteRelease() {
	release, ok := gv.getSelectedRelease()
	if !ok {
		gv.releasesView.Log("[yellow]No release selected")
		return
	}

	ui.ShowStandardConfirmationModal(
		gv.pages,
		gv.app,
		"Delete Release",
		fmt.Sprintf("Delete release [red]%s (%s)[white]?\nThis action cannot be undone!", release.TagName, release.Name),
		func(confirmed bool) {
			if confirmed {
				gv.releasesView.Log(fmt.Sprintf("[yellow]Deleting release %s...", release.TagName))
				go func() {
					if err := gv.githubClient.DeleteRelease(release.ID); err != nil {
						gv.app.QueueUpdateDraw(func() {
							gv.releasesView.Log(fmt.Sprintf("[red]Failed to delete: %v", err))
						})
					} else {
						gv.app.QueueUpdateDraw(func() {
							gv.releasesView.Log(fmt.Sprintf("[green]Release %s deleted", release.TagName))
							gv.refresh()
						})
					}
				}()
			}
			gv.app.SetFocus(gv.releasesView.GetTable())
		},
	)
}

func (gv *GitHubView) showReleaseDetails() {
	release, ok := gv.getSelectedRelease()
	if !ok {
		return
	}

	var details strings.Builder
	details.WriteString(fmt.Sprintf("[yellow]Release: %s[white]\n\n", release.Name))
	details.WriteString(fmt.Sprintf("[green]Tag:[white]        %s\n", release.TagName))
	details.WriteString(fmt.Sprintf("[green]Name:[white]       %s\n", release.Name))
	details.WriteString(fmt.Sprintf("[green]Author:[white]     %s\n", release.Author))
	details.WriteString(fmt.Sprintf("[green]Published:[white]  %s\n", formatAge(release.PublishedAt)))
	details.WriteString(fmt.Sprintf("[green]Assets:[white]     %d\n", release.Assets))

	status := "published"
	if release.Draft {
		status = "draft"
	} else if release.Prerelease {
		status = "prerelease"
	}
	details.WriteString(fmt.Sprintf("[green]Status:[white]     %s\n", status))
	details.WriteString(fmt.Sprintf("[green]URL:[white]        %s\n", release.URL))

	if release.Body != "" {
		body := release.Body
		if len(body) > 500 {
			body = body[:500] + "\n\n... (truncated)"
		}
		details.WriteString(fmt.Sprintf("\n[yellow]Release Notes:[white]\n%s\n", body))
	}

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		fmt.Sprintf("Release: %s", release.TagName),
		details.String(),
		func() {
			gv.app.SetFocus(gv.releasesView.GetTable())
		},
	)
}
