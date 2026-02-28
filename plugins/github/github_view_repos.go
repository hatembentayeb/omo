package main

import (
	"fmt"
	"strings"

	"omo/pkg/ui"
)

func (gv *GitHubView) newReposView() *ui.CoreView {
	cores := ui.NewCoreView(gv.app, "Repositories")
	cores.SetTableHeaders([]string{"Name", "Description", "Language", "Stars", "Visibility", "Default Branch", "Updated"})
	cores.SetRefreshCallback(gv.refreshReposData)
	cores.SetSelectionKey("Name")

	cores.AddKeyBinding("L", "Repos", gv.showRepos)
	cores.AddKeyBinding("P", "PRs", gv.showPRs)
	cores.AddKeyBinding("W", "Workflows", gv.showWorkflows)
	cores.AddKeyBinding("A", "Runs", gv.showRuns)
	cores.AddKeyBinding("E", "Env Vars", gv.showEnvVars)
	cores.AddKeyBinding("S", "Secrets", gv.showSecrets)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("F", "Releases", gv.showReleases)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	cores.SetRowSelectedCallback(func(row int) {
		tableData := cores.GetTableData()
		if row >= 0 && row < len(tableData) && len(tableData[row]) > 0 {
			cores.Log(fmt.Sprintf("[blue]Selected: %s", tableData[row][0]))
		}
	})

	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		gv.selectRepo()
	})

	cores.RegisterHandlers()
	return cores
}

func (gv *GitHubView) refreshReposData() ([][]string, error) {
	if !gv.githubClient.HasAccount() {
		return [][]string{}, fmt.Errorf("no account connected")
	}

	repos, err := gv.githubClient.ListRepos()
	if err != nil {
		return [][]string{}, err
	}

	gv.cachedRepos = repos

	rows := make([][]string, len(repos))
	for i, r := range repos {
		visibility := "public"
		if r.Private {
			visibility = "private"
		}
		if r.Archived {
			visibility = "archived"
		}
		if r.Fork {
			visibility += "/fork"
		}
		desc := r.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		lang := r.Language
		if lang == "" {
			lang = "-"
		}
		updated := r.UpdatedAt
		if len(updated) > 10 {
			updated = updated[:10]
		}
		rows[i] = []string{
			r.FullName,
			desc,
			lang,
			fmt.Sprintf("%d", r.Stars),
			visibility,
			r.DefaultBranch,
			updated,
		}
	}

	if gv.currentAccount != nil {
		gv.reposView.SetInfoMap(map[string]string{
			"Account": gv.currentAccount.Owner,
			"Type":    gv.currentAccount.AccountType,
			"Repos":   fmt.Sprintf("%d", len(repos)),
			"Status":  "Connected",
		})
	}

	return rows, nil
}

func (gv *GitHubView) selectRepo() {
	row := gv.reposView.GetSelectedRowData()
	if len(row) == 0 {
		return
	}

	repoFullName := row[0]

	for _, repo := range gv.cachedRepos {
		if repo.FullName == repoFullName {
			gv.githubClient.SetActiveRepo(&repo)
			gv.activeRepo = &repo
			gv.reposView.Log(fmt.Sprintf("[green]Selected repo: %s", repo.FullName))
			gv.switchToView(viewPRs)
			return
		}
	}

	gv.reposView.Log(fmt.Sprintf("[red]Repo not found: %s", repoFullName))
}

func (gv *GitHubView) showRepoDetails() {
	row := gv.reposView.GetSelectedRowData()
	if len(row) < 7 {
		return
	}

	var details strings.Builder
	details.WriteString(fmt.Sprintf("[yellow]Repository: %s[white]\n\n", row[0]))
	details.WriteString(fmt.Sprintf("[green]Name:[white]       %s\n", row[0]))
	details.WriteString(fmt.Sprintf("[green]Description:[white] %s\n", row[1]))
	details.WriteString(fmt.Sprintf("[green]Language:[white]    %s\n", row[2]))
	details.WriteString(fmt.Sprintf("[green]Stars:[white]       %s\n", row[3]))
	details.WriteString(fmt.Sprintf("[green]Visibility:[white]  %s\n", row[4]))
	details.WriteString(fmt.Sprintf("[green]Branch:[white]      %s\n", row[5]))
	details.WriteString(fmt.Sprintf("[green]Updated:[white]     %s\n", row[6]))

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		fmt.Sprintf("Repo: %s", row[0]),
		details.String(),
		func() {
			gv.app.SetFocus(gv.reposView.GetTable())
		},
	)
}
