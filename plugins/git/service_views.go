package git

import (
	"fmt"

	"omo/pkg/pluginrpc"
)

func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Repos", Action: "goto_repos"},
		{Key: "1", Label: "Status", Action: "goto_status"},
		{Key: "2", Label: "Commits", Action: "goto_commits"},
		{Key: "3", Label: "Branches", Action: "goto_branches"},
		{Key: "4", Label: "Remotes", Action: "goto_remotes"},
		{Key: "5", Label: "Stash", Action: "goto_stash"},
		{Key: "6", Label: "Tags", Action: "goto_tags"},
	}
}

func reposActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "F", Label: "Fetch", Action: "fetch"},
		{Key: "P", Label: "Pull", Action: "pull"},
		{Key: "U", Label: "Push", Action: "push"},
		{Key: "E", Label: "Select", Action: "select_repo"},
	}
}

func statusActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "A", Label: "Stage", Action: "stage"},
		{Key: "U", Label: "Unstage", Action: "unstage"},
		{Key: "D", Label: "Diff", Action: "diff"},
		{Key: "X", Label: "Restore", Action: "restore"},
	}
}

func commitsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "D", Label: "Diff", Action: "diff"},
		{Key: "E", Label: "Details", Action: "view_details"},
		{Key: "C", Label: "Checkout", Action: "checkout"},
		{Key: "X", Label: "Revert", Action: "revert"},
		{Key: "P", Label: "Cherry-pick", Action: "cherry_pick"},
	}
}

func branchesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "C", Label: "Checkout", Action: "checkout"},
		{Key: "D", Label: "Delete", Action: "delete"},
		{Key: "E", Label: "Merge", Action: "merge"},
	}
}

func remotesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "D", Label: "Remove", Action: "delete"},
		{Key: "F", Label: "Fetch", Action: "fetch_remote"},
		{Key: "P", Label: "Prune", Action: "prune_remote"},
	}
}

func stashActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "A", Label: "Apply", Action: "apply_stash"},
		{Key: "P", Label: "Pop", Action: "pop_stash"},
		{Key: "D", Label: "Drop", Action: "delete"},
		{Key: "V", Label: "View", Action: "view_stash"},
	}
}

func tagsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "D", Label: "Delete", Action: "delete"},
		{Key: "C", Label: "Checkout", Action: "checkout"},
		{Key: "P", Label: "Push", Action: "push_tag"},
	}
}

func helpSections() []pluginrpc.HelpSection {
	return pluginrpc.HelpWithGlobal([]pluginrpc.HelpSection{
		{Title: "Views (0-6)", Bindings: viewNavBindings()},
		{Title: "Repos", Bindings: reposActions()},
		{Title: "Status", Bindings: statusActions()},
		{Title: "Commits", Bindings: commitsActions()},
		{Title: "Branches", Bindings: branchesActions()},
		{Title: "Remotes", Bindings: remotesActions()},
		{Title: "Stash", Bindings: stashActions()},
		{Title: "Tags", Bindings: tagsActions()},
	}...)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	Help:  helpSections,
}

func (s *Service) baseInfo(extra string) string {
	path := s.currentPath
	if path == "" {
		path = "(none)"
	}
	msg := fmt.Sprintf("[green]Git Manager[white]\nRepo: %s\nView: %s", path, s.currentView)
	return pluginrpc.FormatInfo(msg, extra)
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewRepos
	}
	s.currentView = viewID

	if s.currentPath == "" && viewID != viewRepos {
		return ui.Decorate(pluginrpc.StatusErrorView(
			viewID, "Git Manager",
			"[yellow]Git Manager[white]\nNo repository configured",
			"not configured", "Configure with a KeePass path attribute",
		)), nil
	}

	switch viewID {
	case viewStatus:
		return s.viewStatusLocked()
	case viewCommits:
		return s.viewCommitsLocked()
	case viewBranches:
		return s.viewBranchesLocked()
	case viewRemotes:
		return s.viewRemotesLocked()
	case viewStash:
		return s.viewStashLocked()
	case viewTags:
		return s.viewTagsLocked()
	default:
		return s.viewReposLocked()
	}
}

func (s *Service) viewReposLocked() (pluginrpc.ViewData, error) {
	for i := range s.repos {
		s.refreshRepoLocked(&s.repos[i])
	}
	rows := make([][]string, 0, len(s.repos))
	for _, repo := range s.repos {
		statusDisplay := repo.Status
		switch statusDisplay {
		case "clean":
			statusDisplay = "[green]Clean[white]"
		case "dirty":
			statusDisplay = "[yellow]Modified[white]"
		case "ahead":
			statusDisplay = "[cyan]Ahead[white]"
		case "behind":
			statusDisplay = "[red]Behind[white]"
		case "":
			statusDisplay = "[gray]...[white]"
		}
		rows = append(rows, []string{
			repo.Name,
			repo.Branch,
			statusDisplay,
			fmt.Sprintf("%d", repo.Modified),
			fmt.Sprintf("%d", repo.Staged),
			fmt.Sprintf("%d", repo.Untracked),
			repo.LastCommit,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "0", "0", "0", "No repositories configured"})
	}
	return ui.Decorate(pluginrpc.ViewData{
		View:         viewRepos,
		Title:        "Git Repositories",
		Info:         s.baseInfo(fmt.Sprintf("Repos: %d", len(s.repos))),
		Status:       "ok",
		Headers:      []string{"Repository", "Branch", "Status", "Modified", "Staged", "Untracked", "Last Commit"},
		Rows:         rows,
		SelectionKey: "Repository",
	}, reposActions()...), nil
}

func (s *Service) viewStatusLocked() (pluginrpc.ViewData, error) {
	files, err := s.client.GetStatusFiles(s.currentPath)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(files))
	for _, f := range files {
		// File first so host selectionPayload key=row[0] is the path.
		rows = append(rows, []string{f.Path, f.Status, f.Type})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"Working tree clean", "-", "-"})
	}
	return ui.Decorate(pluginrpc.ViewData{
		View:         viewStatus,
		Title:        "Git Status",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      []string{"File", "Status", "Type"},
		Rows:         rows,
		SelectionKey: "File",
	}, statusActions()...), nil
}

func (s *Service) viewCommitsLocked() (pluginrpc.ViewData, error) {
	commits, err := s.client.GetCommits(s.currentPath, 50)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(commits))
	for _, c := range commits {
		rows = append(rows, []string{c.Hash, c.Author, c.Date, c.Message})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "No commits"})
	}
	return ui.Decorate(pluginrpc.ViewData{
		View:         viewCommits,
		Title:        "Git Commits",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      []string{"Hash", "Author", "Date", "Message"},
		Rows:         rows,
		SelectionKey: "Hash",
	}, commitsActions()...), nil
}

func (s *Service) viewBranchesLocked() (pluginrpc.ViewData, error) {
	branches, err := s.client.GetBranchesInfo(s.currentPath)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(branches))
	for _, b := range branches {
		cur := ""
		if b.IsCurrent {
			cur = "*"
		}
		rows = append(rows, []string{
			b.Name,
			cur,
			b.Tracking,
			fmt.Sprintf("%d", b.Ahead),
			fmt.Sprintf("%d", b.Behind),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "", "-", "0", "0"})
	}
	return ui.Decorate(pluginrpc.ViewData{
		View:         viewBranches,
		Title:        "Git Branches",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      []string{"Branch", "Current", "Tracking", "Ahead", "Behind"},
		Rows:         rows,
		SelectionKey: "Branch",
	}, branchesActions()...), nil
}

func (s *Service) viewRemotesLocked() (pluginrpc.ViewData, error) {
	remotes, err := s.client.GetRemotesInfo(s.currentPath)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(remotes))
	for _, r := range remotes {
		rows = append(rows, []string{r.Name, r.FetchURL, r.PushURL})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "No remotes"})
	}
	return ui.Decorate(pluginrpc.ViewData{
		View:         viewRemotes,
		Title:        "Git Remotes",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      []string{"Remote", "Fetch URL", "Push URL"},
		Rows:         rows,
		SelectionKey: "Remote",
	}, remotesActions()...), nil
}

func (s *Service) viewStashLocked() (pluginrpc.ViewData, error) {
	entries, err := s.client.GetStashList(s.currentPath)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, []string{e.Index, e.Branch, e.Message})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "No stash entries"})
	}
	return ui.Decorate(pluginrpc.ViewData{
		View:         viewStash,
		Title:        "Git Stash",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      []string{"Index", "Branch", "Message"},
		Rows:         rows,
		SelectionKey: "Index",
	}, stashActions()...), nil
}

func (s *Service) viewTagsLocked() (pluginrpc.ViewData, error) {
	tags, err := s.client.GetTagsInfo(s.currentPath)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(tags))
	for _, t := range tags {
		typ := "lightweight"
		if t.IsAnnotated {
			typ = "annotated"
		}
		rows = append(rows, []string{t.Name, typ, t.Date, t.Message})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "No tags"})
	}
	return ui.Decorate(pluginrpc.ViewData{
		View:         viewTags,
		Title:        "Git Tags",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      []string{"Tag", "Type", "Date", "Message"},
		Rows:         rows,
		SelectionKey: "Tag",
	}, tagsActions()...), nil
}
