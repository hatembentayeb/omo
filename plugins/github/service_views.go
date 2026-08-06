package github

import (
	"fmt"

	"omo/pkg/pluginrpc"
)

func navBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "L", Label: "Repos", Action: "goto_repositories"},
		{Key: "P", Label: "PRs", Action: "goto_pull-requests"},
		{Key: "W", Label: "Workflows", Action: "goto_workflows"},
		{Key: "A", Label: "Runs", Action: "goto_runs"},
		{Key: "E", Label: "Env Vars", Action: "goto_variables"},
		{Key: "S", Label: "Secrets", Action: "goto_secrets"},
		{Key: "B", Label: "Branches", Action: "goto_branches"},
		{Key: "F", Label: "Releases", Action: "goto_releases"},
	}
}

func withNav(extra ...pluginrpc.KeyBinding) []pluginrpc.KeyBinding {
	out := make([]pluginrpc.KeyBinding, 0, len(extra)+len(navBindings())+1)
	out = append(out, pluginrpc.KeyBinding{Key: "R", Label: "Refresh", Action: "refresh"})
	out = append(out, extra...)
	out = append(out, navBindings()...)
	return out
}

func (s *Service) baseInfo(extra string) string {
	owner := "-"
	acctType := "-"
	if s.account != nil {
		owner = s.account.Owner
		if owner == "" {
			owner = s.account.Name
		}
		acctType = s.account.AccountType
	}
	repo := "-"
	if s.activeRepo != nil {
		repo = s.activeRepo.FullName
	}
	msg := fmt.Sprintf("[green]GitHub Manager[white]\nAccount: %s (%s)\nRepo: %s\nView: %s",
		owner, acctType, repo, s.currentView)
	if extra != "" {
		msg += "\n" + extra
	}
	return msg
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewRepos
	}
	s.currentView = viewID

	if s.account == nil || !s.client.HasAccount() {
		return pluginrpc.ViewData{
			View:    viewID,
			Title:   "GitHub Manager",
			Info:    "[yellow]GitHub Manager[white]\nStatus: Not Connected\nHost must Configure with token",
			Status:  "not connected",
			Headers: []string{"Status", "Detail"},
			Rows:    [][]string{{"error", "not configured"}},
			KeyBindings: []pluginrpc.KeyBinding{
				{Key: "R", Label: "Refresh", Action: "refresh"},
			},
		}, nil
	}

	switch viewID {
	case viewPRs:
		return s.viewPRsLocked()
	case viewWorkflows:
		return s.viewWorkflowsLocked()
	case viewRuns:
		return s.viewRunsLocked()
	case viewEnvVars:
		return s.viewVariablesLocked()
	case viewSecrets:
		return s.viewSecretsLocked()
	case viewBranches:
		return s.viewBranchesLocked()
	case viewReleases:
		return s.viewReleasesLocked()
	default:
		return s.viewReposLocked()
	}
}

func (s *Service) viewReposLocked() (pluginrpc.ViewData, error) {
	repos, err := s.client.ListRepos()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	s.cachedRepos = repos
	rows := make([][]string, len(repos))
	for i, r := range repos {
		visibility := "[green]public"
		if r.Private {
			visibility = "[yellow]private"
		}
		if r.Archived {
			visibility = "[gray]archived"
		}
		if r.Fork {
			visibility += "[gray]/fork"
		}
		desc := r.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		lang := r.Language
		if lang == "" {
			lang = "[gray]-"
		} else {
			lang = "[yellow]" + lang
		}
		updated := r.UpdatedAt
		if len(updated) > 10 {
			updated = updated[:10]
		}
		rows[i] = []string{
			"[white]" + r.FullName,
			"[gray]" + desc,
			lang,
			"[white]" + fmt.Sprintf("%d", r.Stars),
			visibility,
			"[green]" + r.DefaultBranch,
			"[gray]" + updated,
		}
	}
	if len(rows) == 0 {
		rows = [][]string{{"-", "No repositories", "-", "-", "-", "-", "-"}}
	}
	return pluginrpc.ViewData{
		View:         viewRepos,
		Title:        "Repositories",
		Info:         s.baseInfo(fmt.Sprintf("Repos: %d · Enter/P selects repo", len(repos))),
		Status:       "connected",
		Headers:      []string{"Name", "Description", "Language", "Stars", "Visibility", "Default Branch", "Updated"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "Enter", Label: "Select", Action: "select_repo"},
		),
	}, nil
}

func (s *Service) requireRepo() error {
	if !s.client.HasActiveRepo() {
		return fmt.Errorf("no repository selected")
	}
	return nil
}

func (s *Service) viewPRsLocked() (pluginrpc.ViewData, error) {
	if err := s.requireRepo(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	prs, err := s.client.ListPullRequests(s.prState)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, len(prs))
	for i, pr := range prs {
		rows[i] = pr.GetTableRow()
	}
	if len(rows) == 0 {
		rows = [][]string{{"-", "No pull requests", "-", "-", "-", "-", "-", "-", "-"}}
	}
	return pluginrpc.ViewData{
		View:         viewPRs,
		Title:        "Pull Requests",
		Info:         s.baseInfo(fmt.Sprintf("Filter: %s · Count: %d", s.prState, len(prs))),
		Status:       "connected",
		Headers:      []string{"#", "Title", "State", "Author", "Branch", "Base", "Changes", "Labels", "Age"},
		Rows:         rows,
		SelectionKey: "#",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "M", Label: "Merge", Action: "merge"},
			pluginrpc.KeyBinding{Key: "C", Label: "Close", Action: "close"},
			pluginrpc.KeyBinding{Key: "O", Label: "Reopen", Action: "reopen"},
			pluginrpc.KeyBinding{Key: "V", Label: "Approve", Action: "approve"},
			pluginrpc.KeyBinding{Key: "K", Label: "Checks", Action: "view_checks"},
			pluginrpc.KeyBinding{Key: "I", Label: "Reviews", Action: "view_reviews"},
			pluginrpc.KeyBinding{Key: "T", Label: "Toggle State", Action: "toggle_pr_state"},
		),
	}, nil
}

func (s *Service) viewWorkflowsLocked() (pluginrpc.ViewData, error) {
	if err := s.requireRepo(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	wfs, err := s.client.ListWorkflows()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, len(wfs))
	for i, w := range wfs {
		rows[i] = w.GetTableRow()
	}
	if len(rows) == 0 {
		rows = [][]string{{"-", "No workflows", "-", "-", "-"}}
	}
	return pluginrpc.ViewData{
		View:         viewWorkflows,
		Title:        "Workflows",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"ID", "Name", "Path", "State", "Updated"},
		Rows:         rows,
		SelectionKey: "ID",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "D", Label: "Dispatch", Action: "dispatch"},
		),
	}, nil
}

func (s *Service) viewRunsLocked() (pluginrpc.ViewData, error) {
	if err := s.requireRepo(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	runs, err := s.client.ListWorkflowRuns("")
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, len(runs))
	for i, r := range runs {
		rows[i] = r.GetTableRow()
	}
	if len(rows) == 0 {
		rows = [][]string{{"-", "No runs", "-", "-", "-", "-", "-", "-"}}
	}
	return pluginrpc.ViewData{
		View:         viewRuns,
		Title:        "Workflow Runs",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"ID", "Workflow", "Status", "Branch", "Event", "Actor", "Duration", "Age"},
		Rows:         rows,
		SelectionKey: "ID",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "X", Label: "Cancel", Action: "cancel_run"},
			pluginrpc.KeyBinding{Key: "G", Label: "Re-run", Action: "rerun"},
			pluginrpc.KeyBinding{Key: "J", Label: "Jobs", Action: "view_jobs"},
		),
	}, nil
}

func (s *Service) viewVariablesLocked() (pluginrpc.ViewData, error) {
	if err := s.requireRepo(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	vars, err := s.client.ListRepoVariables()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, len(vars))
	for i, v := range vars {
		rows[i] = v.GetTableRow()
	}
	if len(rows) == 0 {
		rows = [][]string{{"-", "-", "No variables"}}
	}
	return pluginrpc.ViewData{
		View:         viewEnvVars,
		Title:        "Variables",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Name", "Value", "Updated"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "N", Label: "New Var", Action: "create_variable"},
			pluginrpc.KeyBinding{Key: "U", Label: "Update", Action: "update_variable"},
			pluginrpc.KeyBinding{Key: "D", Label: "Delete", Action: "delete_variable"},
		),
	}, nil
}

func (s *Service) viewSecretsLocked() (pluginrpc.ViewData, error) {
	if err := s.requireRepo(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	secrets, err := s.client.ListRepoSecrets()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, len(secrets))
	for i, sec := range secrets {
		rows[i] = sec.GetTableRow()
	}
	if len(rows) == 0 {
		rows = [][]string{{"-", "-", "No secrets"}}
	}
	return pluginrpc.ViewData{
		View:         viewSecrets,
		Title:        "Secrets",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Name", "Value", "Updated"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "D", Label: "Delete", Action: "delete_secret"},
		),
	}, nil
}

func (s *Service) viewBranchesLocked() (pluginrpc.ViewData, error) {
	if err := s.requireRepo(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	branches, err := s.client.ListBranches()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, len(branches))
	for i, b := range branches {
		rows[i] = b.GetTableRow()
	}
	if len(rows) == 0 {
		rows = [][]string{{"-", "-", "-"}}
	}
	return pluginrpc.ViewData{
		View:         viewBranches,
		Title:        "Branches",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Name", "SHA", "Protected"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "D", Label: "Delete", Action: "delete_branch"},
		),
	}, nil
}

func (s *Service) viewReleasesLocked() (pluginrpc.ViewData, error) {
	if err := s.requireRepo(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	releases, err := s.client.ListReleases()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, len(releases))
	for i, r := range releases {
		rows[i] = r.GetTableRow()
	}
	if len(rows) == 0 {
		rows = [][]string{{"-", "-", "-", "-", "-", "-"}}
	}
	return pluginrpc.ViewData{
		View:         viewReleases,
		Title:        "Releases",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Tag", "Name", "Status", "Author", "Assets", "Published"},
		Rows:         rows,
		SelectionKey: "Tag",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "D", Label: "Delete", Action: "delete_release"},
		),
	}, nil
}
