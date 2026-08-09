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
	return pluginrpc.HelpNav(viewNavBindings(), nil,
		pluginrpc.HelpSection{Title: "Repositories", Bindings: repositoriesActions()},
		pluginrpc.HelpSection{Title: "Pull Requests", Bindings: prsActions()},
		pluginrpc.HelpSection{Title: "Workflows", Bindings: workflowsActions()},
		pluginrpc.HelpSection{Title: "Runs", Bindings: runsActions()},
		pluginrpc.HelpSection{Title: "Variables", Bindings: variablesActions()},
		pluginrpc.HelpSection{Title: "Secrets", Bindings: secretsActions()},
		pluginrpc.HelpSection{Title: "Branches", Bindings: branchesActions()},
		pluginrpc.HelpSection{Title: "Releases", Bindings: releasesActions()},
	)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	Help:  helpSections,
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
	return pluginrpc.FormatInfo(msg, extra)
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewRepos
	}
	s.currentView = viewID

	if s.account == nil || !s.client.HasAccount() {
		return ui.NotConnected(viewID, "GitHub Manager", "not configured"), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"-", "No repositories", "-", "-", "-", "-", "-"})
	return ui.Connected(viewRepos, "Repositories", s.baseInfo(fmt.Sprintf("Repos: %d · Enter/P selects repo", len(repos))), []string{"Name", "Description", "Language", "Stars", "Visibility", "Default Branch", "Updated"}, rows, "Name", repositoriesActions()...), nil
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
	rows := pluginrpc.MapRows(prs, func(pr PullRequest) []string { return pr.GetTableRow() })
	rows = pluginrpc.EnsureRows(rows, []string{"-", "No pull requests", "-", "-", "-", "-", "-", "-", "-"})
	return ui.Connected(viewPRs, "Pull Requests", s.baseInfo(fmt.Sprintf("Filter: %s · Count: %d", s.prState, len(prs))), []string{"#", "Title", "State", "Author", "Branch", "Base", "Changes", "Labels", "Age"}, rows, "#", prsActions()...), nil
}

func (s *Service) viewWorkflowsLocked() (pluginrpc.ViewData, error) {
	if err := s.requireRepo(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	wfs, err := s.client.ListWorkflows()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := pluginrpc.MapRows(wfs, func(w Workflow) []string { return w.GetTableRow() })
	rows = pluginrpc.EnsureRows(rows, []string{"-", "No workflows", "-", "-", "-"})
	return ui.Connected(viewWorkflows, "Workflows", s.baseInfo(""), []string{"ID", "Name", "Path", "State", "Updated"}, rows, "ID", workflowsActions()...), nil
}

func (s *Service) viewRunsLocked() (pluginrpc.ViewData, error) {
	if err := s.requireRepo(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	runs, err := s.client.ListWorkflowRuns("")
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := pluginrpc.MapRows(runs, func(r WorkflowRun) []string { return r.GetTableRow() })
	rows = pluginrpc.EnsureRows(rows, []string{"-", "No runs", "-", "-", "-", "-", "-", "-"})
	return ui.Connected(viewRuns, "Workflow Runs", s.baseInfo(""), []string{"ID", "Workflow", "Status", "Branch", "Event", "Actor", "Duration", "Age"}, rows, "ID", runsActions()...), nil
}

func (s *Service) viewVariablesLocked() (pluginrpc.ViewData, error) {
	if err := s.requireRepo(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	vars, err := s.client.ListRepoVariables()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := pluginrpc.MapRows(vars, func(v EnvVariable) []string { return v.GetTableRow() })
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "No variables"})
	return ui.Connected(viewEnvVars, "Variables", s.baseInfo(""), []string{"Name", "Value", "Updated"}, rows, "Name", variablesActions()...), nil
}

func (s *Service) viewSecretsLocked() (pluginrpc.ViewData, error) {
	if err := s.requireRepo(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	secrets, err := s.client.ListRepoSecrets()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := pluginrpc.MapRows(secrets, func(sec RepoSecret) []string { return sec.GetTableRow() })
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "No secrets"})
	return ui.Connected(viewSecrets, "Secrets", s.baseInfo(""), []string{"Name", "Value", "Updated"}, rows, "Name", secretsActions()...), nil
}

func (s *Service) viewBranchesLocked() (pluginrpc.ViewData, error) {
	if err := s.requireRepo(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	branches, err := s.client.ListBranches()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := pluginrpc.MapRows(branches, func(b Branch) []string { return b.GetTableRow() })
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-"})
	return ui.Connected(viewBranches, "Branches", s.baseInfo(""), []string{"Name", "SHA", "Protected"}, rows, "Name", branchesActions()...), nil
}

func (s *Service) viewReleasesLocked() (pluginrpc.ViewData, error) {
	if err := s.requireRepo(); err != nil {
		return pluginrpc.ViewData{}, err
	}
	releases, err := s.client.ListReleases()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := pluginrpc.MapRows(releases, func(r Release) []string { return r.GetTableRow() })
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "-", "-"})
	return ui.Connected(viewReleases, "Releases", s.baseInfo(""), []string{"Tag", "Name", "Status", "Author", "Assets", "Published"}, rows, "Tag", releasesActions()...), nil
}
