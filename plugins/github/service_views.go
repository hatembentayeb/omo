package github

import (
	"fmt"

	"omo/pkg/pluginrpc"
)

func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Repos", Action: "goto_repositories"},
		{Key: "1", Label: "PRs", Action: "goto_pull-requests"},
		{Key: "2", Label: "Workflows", Action: "goto_workflows"},
		{Key: "3", Label: "Runs", Action: "goto_runs"},
		{Key: "4", Label: "Env Vars", Action: "goto_variables"},
		{Key: "5", Label: "Secrets", Action: "goto_secrets"},
		{Key: "6", Label: "Branches", Action: "goto_branches"},
		{Key: "7", Label: "Releases", Action: "goto_releases"},
	}
}

func repositoriesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "Enter", Label: "Select", Action: "select_repo"},
	}
}

func prsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "M", Label: "Merge", Action: "merge"},
		{Key: "C", Label: "Close", Action: "close"},
		{Key: "O", Label: "Reopen", Action: "reopen"},
		{Key: "V", Label: "Approve", Action: "approve"},
		{Key: "K", Label: "Checks", Action: "view_checks"},
		{Key: "I", Label: "Reviews", Action: "view_reviews"},
		{Key: "T", Label: "Toggle State", Action: "toggle_pr_state"},
	}
}

func workflowsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "D", Label: "Dispatch", Action: "dispatch"},
	}
}

func runsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "X", Label: "Cancel", Action: "cancel_run"},
		{Key: "G", Label: "Re-run", Action: "rerun"},
		{Key: "J", Label: "Jobs", Action: "view_jobs"},
	}
}

func variablesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "N", Label: "New Var", Action: "create_variable"},
		{Key: "U", Label: "Update", Action: "update_variable"},
		{Key: "D", Label: "Delete", Action: "delete_variable"},
	}
}

func secretsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "D", Label: "Delete", Action: "delete_secret"},
	}
}

func branchesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "D", Label: "Delete", Action: "delete_branch"},
	}
}

func releasesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "D", Label: "Delete", Action: "delete_release"},
	}
}

func helpSections() []pluginrpc.HelpSection {
	return pluginrpc.HelpWithGlobal([]pluginrpc.HelpSection{
		{Title: "Views (0-7)", Bindings: viewNavBindings()},
		{Title: "Repositories", Bindings: repositoriesActions()},
		{Title: "Pull Requests", Bindings: prsActions()},
		{Title: "Workflows", Bindings: workflowsActions()},
		{Title: "Runs", Bindings: runsActions()},
		{Title: "Variables", Bindings: variablesActions()},
		{Title: "Secrets", Bindings: secretsActions()},
		{Title: "Branches", Bindings: branchesActions()},
		{Title: "Releases", Bindings: releasesActions()},
	}...)
}

func decorate(view pluginrpc.ViewData, actions ...pluginrpc.KeyBinding) pluginrpc.ViewData {
	return pluginrpc.Decorate(view, viewNavBindings(), nil, helpSections(), actions...)
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
	return decorate(pluginrpc.ViewData{
		View:         viewRepos,
		Title:        "Repositories",
		Info:         s.baseInfo(fmt.Sprintf("Repos: %d · Enter/P selects repo", len(repos))),
		Status:       "connected",
		Headers:      []string{"Name", "Description", "Language", "Stars", "Visibility", "Default Branch", "Updated"},
		Rows:         rows,
		SelectionKey: "Name",
	}, repositoriesActions()...), nil
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
	return decorate(pluginrpc.ViewData{
		View:         viewPRs,
		Title:        "Pull Requests",
		Info:         s.baseInfo(fmt.Sprintf("Filter: %s · Count: %d", s.prState, len(prs))),
		Status:       "connected",
		Headers:      []string{"#", "Title", "State", "Author", "Branch", "Base", "Changes", "Labels", "Age"},
		Rows:         rows,
		SelectionKey: "#",
	}, prsActions()...), nil
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
	return decorate(pluginrpc.ViewData{
		View:         viewWorkflows,
		Title:        "Workflows",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"ID", "Name", "Path", "State", "Updated"},
		Rows:         rows,
		SelectionKey: "ID",
	}, workflowsActions()...), nil
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
	return decorate(pluginrpc.ViewData{
		View:         viewRuns,
		Title:        "Workflow Runs",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"ID", "Workflow", "Status", "Branch", "Event", "Actor", "Duration", "Age"},
		Rows:         rows,
		SelectionKey: "ID",
	}, runsActions()...), nil
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
	return decorate(pluginrpc.ViewData{
		View:         viewEnvVars,
		Title:        "Variables",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Name", "Value", "Updated"},
		Rows:         rows,
		SelectionKey: "Name",
	}, variablesActions()...), nil
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
	return decorate(pluginrpc.ViewData{
		View:         viewSecrets,
		Title:        "Secrets",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Name", "Value", "Updated"},
		Rows:         rows,
		SelectionKey: "Name",
	}, secretsActions()...), nil
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
	return decorate(pluginrpc.ViewData{
		View:         viewBranches,
		Title:        "Branches",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Name", "SHA", "Protected"},
		Rows:         rows,
		SelectionKey: "Name",
	}, branchesActions()...), nil
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
	return decorate(pluginrpc.ViewData{
		View:         viewReleases,
		Title:        "Releases",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Tag", "Name", "Status", "Author", "Assets", "Published"},
		Rows:         rows,
		SelectionKey: "Tag",
	}, releasesActions()...), nil
}
