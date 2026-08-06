package git

import (
	"fmt"

	"omo/pkg/pluginrpc"
)

func navBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "G", Label: "Repos", Action: "goto_repos"},
		{Key: "S", Label: "Status", Action: "goto_status"},
		{Key: "L", Label: "Commits", Action: "goto_commits"},
		{Key: "B", Label: "Branches", Action: "goto_branches"},
		{Key: "M", Label: "Remotes", Action: "goto_remotes"},
		{Key: "T", Label: "Tags", Action: "goto_tags"},
		{Key: "H", Label: "Stash", Action: "goto_stash"},
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
	path := s.currentPath
	if path == "" {
		path = "(none)"
	}
	msg := fmt.Sprintf("[green]Git Manager[white]\nRepo: %s\nView: %s", path, s.currentView)
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

	if s.currentPath == "" && viewID != viewRepos {
		return pluginrpc.ViewData{
			View:        viewID,
			Title:       "Git Manager",
			Info:        "[yellow]Git Manager[white]\nNo repository configured",
			Status:      "not configured",
			Headers:     []string{"Status", "Detail"},
			Rows:        [][]string{{"error", "Configure with a KeePass path attribute"}},
			KeyBindings: withNav(),
		}, nil
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
	return pluginrpc.ViewData{
		View:         viewRepos,
		Title:        "Git Repositories",
		Info:         s.baseInfo(fmt.Sprintf("Repos: %d", len(s.repos))),
		Status:       "ok",
		Headers:      []string{"Repository", "Branch", "Status", "Modified", "Staged", "Untracked", "Last Commit"},
		Rows:         rows,
		SelectionKey: "Repository",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "F", Label: "Fetch", Action: "fetch"},
			pluginrpc.KeyBinding{Key: "P", Label: "Pull", Action: "pull"},
			pluginrpc.KeyBinding{Key: "U", Label: "Push", Action: "push"},
			pluginrpc.KeyBinding{Key: "E", Label: "Select", Action: "select_repo"},
		),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewStatus,
		Title:        "Git Status",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      []string{"File", "Status", "Type"},
		Rows:         rows,
		SelectionKey: "File",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "A", Label: "Stage", Action: "stage"},
			pluginrpc.KeyBinding{Key: "U", Label: "Unstage", Action: "unstage"},
			pluginrpc.KeyBinding{Key: "D", Label: "Diff", Action: "diff"},
			pluginrpc.KeyBinding{Key: "X", Label: "Restore", Action: "restore"},
		),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewCommits,
		Title:        "Git Commits",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      []string{"Hash", "Author", "Date", "Message"},
		Rows:         rows,
		SelectionKey: "Hash",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "D", Label: "Diff", Action: "diff"},
			pluginrpc.KeyBinding{Key: "E", Label: "Details", Action: "view_details"},
			pluginrpc.KeyBinding{Key: "C", Label: "Checkout", Action: "checkout"},
			pluginrpc.KeyBinding{Key: "X", Label: "Revert", Action: "revert"},
			pluginrpc.KeyBinding{Key: "P", Label: "Cherry-pick", Action: "cherry_pick"},
		),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewBranches,
		Title:        "Git Branches",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      []string{"Branch", "Current", "Tracking", "Ahead", "Behind"},
		Rows:         rows,
		SelectionKey: "Branch",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "C", Label: "Checkout", Action: "checkout"},
			pluginrpc.KeyBinding{Key: "D", Label: "Delete", Action: "delete"},
			pluginrpc.KeyBinding{Key: "E", Label: "Merge", Action: "merge"},
		),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewRemotes,
		Title:        "Git Remotes",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      []string{"Remote", "Fetch URL", "Push URL"},
		Rows:         rows,
		SelectionKey: "Remote",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "D", Label: "Remove", Action: "delete"},
			pluginrpc.KeyBinding{Key: "F", Label: "Fetch", Action: "fetch_remote"},
			pluginrpc.KeyBinding{Key: "P", Label: "Prune", Action: "prune_remote"},
		),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewStash,
		Title:        "Git Stash",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      []string{"Index", "Branch", "Message"},
		Rows:         rows,
		SelectionKey: "Index",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "A", Label: "Apply", Action: "apply_stash"},
			pluginrpc.KeyBinding{Key: "P", Label: "Pop", Action: "pop_stash"},
			pluginrpc.KeyBinding{Key: "D", Label: "Drop", Action: "delete"},
			pluginrpc.KeyBinding{Key: "V", Label: "View", Action: "view_stash"},
		),
	}, nil
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
	return pluginrpc.ViewData{
		View:         viewTags,
		Title:        "Git Tags",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      []string{"Tag", "Type", "Date", "Message"},
		Rows:         rows,
		SelectionKey: "Tag",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "D", Label: "Delete", Action: "delete"},
			pluginrpc.KeyBinding{Key: "C", Label: "Checkout", Action: "checkout"},
			pluginrpc.KeyBinding{Key: "P", Label: "Push", Action: "push_tag"},
		),
	}, nil
}
