package jira

import (
	"fmt"
	"strings"

	"omo/pkg/pluginrpc"
)

func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Mine", Action: "goto_mine"},
		{Key: "1", Label: "Boards", Action: "goto_boards"},
		{Key: "2", Label: "Issues", Action: "goto_issues"},
		{Key: "3", Label: "Sprints", Action: "goto_sprints"},
		{Key: "4", Label: "Backlog", Action: "goto_backlog"},
		{Key: "5", Label: "Transitions", Action: "goto_transitions"},
		{Key: "6", Label: "Comments", Action: "goto_comments"},
		{Key: "7", Label: "Deploys", Action: "goto_deploys"},
		{Key: "8", Label: "Filters", Action: "goto_filters"},
		{Key: "9", Label: "Site", Action: "goto_site"},
	}
}

func issueListActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "Enter", Label: "Detail", Action: "issue_detail"},
		{Key: "C", Label: "Close", Action: "close"},
		{Key: "O", Label: "Reopen", Action: "reopen"},
		{Key: "A", Label: "Assign me", Action: "assign_me"},
		{Key: "U", Label: "Unassign", Action: "unassign"},
		{Key: "T", Label: "Transitions", Action: "goto_transitions"},
		{Key: "D", Label: "Deploys", Action: "goto_deploys"},
		{Key: "M", Label: "Comment", Action: "add_comment"},
		{Key: "N", Label: "New issue", Action: "create_issue"},
	}
}

func mineActions() []pluginrpc.KeyBinding {
	return append(issueListActions(), pluginrpc.KeyBinding{Key: "J", Label: "Run JQL", Action: "run_jql"})
}

func issuesActions() []pluginrpc.KeyBinding {
	return append(issueListActions(), pluginrpc.KeyBinding{Key: "X", Label: "Clear JQL", Action: "clear_jql"})
}

func backlogActions() []pluginrpc.KeyBinding {
	return append(issueListActions(), pluginrpc.KeyBinding{Key: "S", Label: "To sprint", Action: "move_to_sprint"})
}

func boardsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "Enter", Label: "Select", Action: "select_board"},
	}
}

func sprintsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "Enter", Label: "Select", Action: "select_sprint"},
	}
}

func transitionsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "Enter", Label: "Apply", Action: "apply_transition"},
	}
}

func commentsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "Enter", Label: "Full", Action: "comment_detail"},
		{Key: "N", Label: "Add", Action: "add_comment"},
	}
}

func deploysActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "Enter", Label: "Detail", Action: "deploy_detail"},
	}
}

func filtersActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "Enter", Label: "Run", Action: "run_filter"},
	}
}

func helpSections() []pluginrpc.HelpSection {
	return pluginrpc.HelpNav(viewNavBindings(), nil,
		pluginrpc.HelpSection{Title: "Mine", Bindings: mineActions()},
		pluginrpc.HelpSection{Title: "Boards", Bindings: boardsActions()},
		pluginrpc.HelpSection{Title: "Issues", Bindings: issuesActions()},
		pluginrpc.HelpSection{Title: "Sprints", Bindings: sprintsActions()},
		pluginrpc.HelpSection{Title: "Backlog", Bindings: backlogActions()},
		pluginrpc.HelpSection{Title: "Transitions", Bindings: transitionsActions()},
		pluginrpc.HelpSection{Title: "Comments", Bindings: commentsActions()},
		pluginrpc.HelpSection{Title: "Deploys", Bindings: deploysActions()},
		pluginrpc.HelpSection{Title: "Filters", Bindings: filtersActions()},
	)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	Help:  helpSections,
}

func (s *Service) baseInfo(extra string) string {
	site, user, board, issue := "-", "-", "-", "-"
	if s.client != nil {
		site = strings.TrimPrefix(s.client.Site(), "https://")
		if u := s.client.User(); u != nil {
			user = firstNonEmpty(u.DisplayName, u.EmailAddress, u.AccountID)
		}
	}
	if s.selectedBoard != nil {
		board = s.selectedBoard.Name
		if s.selectedBoard.Type != "" {
			board += " (" + s.selectedBoard.Type + ")"
		}
	}
	if s.selectedIssue != nil {
		issue = s.selectedIssue.Key
	}
	msg := fmt.Sprintf("%s\nSite: %s\nUser: %s\nBoard: %s\nIssue: %s\nView: %s",
		brandName, site, user, board, issue, s.currentView)
	return pluginrpc.FormatInfo(msg, extra)
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewMine
	}
	s.currentView = viewID
	if s.client == nil || !s.client.Connected() {
		return ui.NotConnected(viewID, brandName, "not configured"), nil
	}
	switch viewID {
	case viewBoards:
		return s.viewBoardsLocked()
	case viewIssues:
		return s.viewIssuesLocked()
	case viewSprints:
		return s.viewSprintsLocked()
	case viewBacklog:
		return s.viewBacklogLocked()
	case viewTransitions:
		return s.viewTransitionsLocked()
	case viewComments:
		return s.viewCommentsLocked()
	case viewDeploys:
		return s.viewDeploysLocked()
	case viewFilters:
		return s.viewFiltersLocked()
	case viewSite:
		return s.viewSiteLocked()
	default:
		return s.viewMineLocked()
	}
}

func (s *Service) viewMineLocked() (pluginrpc.ViewData, error) {
	issues, err := s.client.Search(jqlMine, maxList)
	if err != nil {
		return ui.NotConnectedErr(viewMine, brandName, err)
	}
	rows := issueRows(issues, true)
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "No issues assigned to you", "-"})
	return ui.Connected(viewMine, "Mine", s.baseInfo(fmt.Sprintf("Open: %d", len(issues))),
		[]string{"Key", "Type", "Status", "Priority", "Summary", "Updated"},
		rows, "Key", mineActions()...), nil
}

func (s *Service) viewBoardsLocked() (pluginrpc.ViewData, error) {
	boards, err := s.client.Boards()
	if err != nil {
		return ui.NotConnectedErr(viewBoards, brandName, err)
	}
	s.cachedBoards = boards
	rows := pluginrpc.MapRows(boards, func(b Board) []string {
		return []string{
			fmt.Sprintf("%d", b.ID),
			b.Name,
			dash(b.Type),
			dash(b.ProjectKey()),
		}
	})
	rows = pluginrpc.EnsureRows(rows, []string{"-", "No boards", "-", "-"})
	return ui.Connected(viewBoards, "Boards", s.baseInfo(fmt.Sprintf("Boards: %d · Enter selects", len(boards))),
		[]string{"ID", "Name", "Type", "Project"},
		rows, "ID", boardsActions()...), nil
}

func (s *Service) viewIssuesLocked() (pluginrpc.ViewData, error) {
	var issues []Issue
	var err error
	title := "Issues"
	extra := ""
	switch {
	case s.activeJQL != "":
		issues, err = s.client.Search(s.activeJQL, maxList)
		title = "Issues — JQL"
		extra = "JQL: " + pluginrpc.Truncate(s.activeJQL, 80)
	case s.selectedSprint != nil:
		issues, err = s.client.SprintIssues(s.selectedSprint.ID)
		label := s.selectedSprint.Name
		if label == "" {
			label = fmt.Sprintf("%d", s.selectedSprint.ID)
		}
		title = "Issues — " + label
		extra = "Sprint: " + label
	case s.selectedBoard != nil:
		issues, err = s.client.BoardIssues(s.selectedBoard.ID)
		title = "Issues — " + s.selectedBoard.Name
		extra = fmt.Sprintf("Board: %s", s.selectedBoard.Name)
	default:
		return needBoardView(viewIssues, "Issues")
	}
	if err != nil {
		return ui.NotConnectedErr(viewIssues, brandName, err)
	}
	rows := issueRows(issues, false)
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "No issues"})
	return ui.Connected(viewIssues, title, s.baseInfo(fmt.Sprintf("%s · %d issues", extra, len(issues))),
		[]string{"Key", "Status", "Assignee", "Priority", "Summary"},
		rows, "Key", issuesActions()...), nil
}

func (s *Service) viewSprintsLocked() (pluginrpc.ViewData, error) {
	if s.selectedBoard == nil {
		return needBoardView(viewSprints, "Sprints")
	}
	if strings.EqualFold(s.selectedBoard.Type, "kanban") {
		return ui.Connected(viewSprints, "Sprints — "+s.selectedBoard.Name, s.baseInfo(""),
			[]string{"Property", "Value"},
			[][]string{
				{"Status", "kanban — no sprints"},
				{"Hint", "Use view 2 (Issues) or 4 (Backlog)"},
			},
			"Property", sprintsActions()...), nil
	}
	sprints, err := s.client.Sprints(s.selectedBoard.ID)
	if err != nil {
		msg := err.Error()
		if strings.Contains(strings.ToLower(msg), "does not support sprint") || strings.Contains(strings.ToLower(msg), "kanban") {
			return ui.Connected(viewSprints, "Sprints — "+s.selectedBoard.Name, s.baseInfo(""),
				[]string{"Property", "Value"},
				[][]string{
					{"Status", "this board has no sprints"},
					{"Hint", "Kanban boards skip this view"},
				},
				"Property"), nil
		}
		return ui.NotConnectedErr(viewSprints, brandName, err)
	}
	rows := pluginrpc.MapRows(sprints, func(sp Sprint) []string {
		return []string{
			fmt.Sprintf("%d", sp.ID),
			sp.Name,
			dash(sp.State),
			formatWhen(sp.StartDate),
			formatWhen(sp.EndDate),
		}
	})
	rows = pluginrpc.EnsureRows(rows, []string{"-", "No sprints", "-", "-", "-"})
	return ui.Connected(viewSprints, "Sprints — "+s.selectedBoard.Name, s.baseInfo(fmt.Sprintf("Sprints: %d", len(sprints))),
		[]string{"ID", "Name", "State", "Start", "End"},
		rows, "ID", sprintsActions()...), nil
}

func (s *Service) viewBacklogLocked() (pluginrpc.ViewData, error) {
	if s.selectedBoard == nil {
		return needBoardView(viewBacklog, "Backlog")
	}
	issues, err := s.client.Backlog(s.selectedBoard.ID)
	if err != nil {
		return ui.NotConnectedErr(viewBacklog, brandName, err)
	}
	rows := issueRows(issues, false)
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "Backlog is empty"})
	return ui.Connected(viewBacklog, "Backlog — "+s.selectedBoard.Name, s.baseInfo(fmt.Sprintf("Backlog: %d", len(issues))),
		[]string{"Key", "Status", "Assignee", "Priority", "Summary"},
		rows, "Key", backlogActions()...), nil
}

func (s *Service) viewTransitionsLocked() (pluginrpc.ViewData, error) {
	if s.selectedIssue == nil || s.selectedIssue.Key == "" {
		return needIssueView(viewTransitions, "Transitions")
	}
	ts, err := s.client.Transitions(s.selectedIssue.Key)
	if err != nil {
		return ui.NotConnectedErr(viewTransitions, brandName, err)
	}
	rows := pluginrpc.MapRows(ts, func(t Transition) []string {
		return []string{
			t.ID,
			t.Name,
			dash(t.To.Name),
			dash(t.To.StatusCategory.Name),
		}
	})
	rows = pluginrpc.EnsureRows(rows, []string{"-", "No transitions", "-", "-"})
	return ui.Connected(viewTransitions, "Transitions — "+s.selectedIssue.Key,
		s.baseInfo(fmt.Sprintf("Available: %d", len(ts))),
		[]string{"ID", "Name", "To status", "Category"},
		rows, "ID", transitionsActions()...), nil
}

func (s *Service) viewCommentsLocked() (pluginrpc.ViewData, error) {
	if s.selectedIssue == nil || s.selectedIssue.Key == "" {
		return needIssueView(viewComments, "Comments")
	}
	comments, err := s.client.Comments(s.selectedIssue.Key)
	if err != nil {
		return ui.NotConnectedErr(viewComments, brandName, err)
	}
	s.cachedComments = comments
	rows := pluginrpc.MapRows(comments, func(c Comment) []string {
		return []string{
			c.ID,
			dash(c.Author.DisplayName),
			formatWhen(c.Created),
			pluginrpc.Truncate(strings.ReplaceAll(adfToText(c.Body), "\n", " "), 80),
		}
	})
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "No comments"})
	return ui.Connected(viewComments, "Comments — "+s.selectedIssue.Key,
		s.baseInfo(fmt.Sprintf("Comments: %d", len(comments))),
		[]string{"ID", "Author", "Created", "Body"},
		rows, "ID", commentsActions()...), nil
}

func (s *Service) viewDeploysLocked() (pluginrpc.ViewData, error) {
	if s.selectedIssue == nil || s.selectedIssue.Key == "" {
		return needIssueView(viewDeploys, "Deploys")
	}
	items, err := s.client.DevItems(s.selectedIssue.ID, s.selectedIssue.Key)
	if err != nil {
		return ui.NotConnectedErr(viewDeploys, brandName, err)
	}
	s.cachedDev = items
	if len(items) == 0 {
		return ui.Connected(viewDeploys, "Deploys — "+s.selectedIssue.Key, s.baseInfo(""),
			[]string{"Kind", "Name", "Env", "State", "When"},
			[][]string{{"-", "No development data — connect GitHub/Bitbucket in Jira", "-", "-", "-"}},
			"Kind", deploysActions()...), nil
	}
	rows := pluginrpc.MapRows(items, func(d DevItem) []string {
		return []string{d.Kind, d.Name, d.Env, d.State, d.When}
	})
	return ui.Connected(viewDeploys, "Deploys — "+s.selectedIssue.Key,
		s.baseInfo(fmt.Sprintf("Items: %d", len(items))),
		[]string{"Kind", "Name", "Env", "State", "When"},
		rows, "Kind", deploysActions()...), nil
}

func (s *Service) viewFiltersLocked() (pluginrpc.ViewData, error) {
	rows := pluginrpc.MapRows(builtinFilters(), func(f SavedFilter) []string {
		return []string{f.ID, f.Name, pluginrpc.Truncate(f.JQL, 60)}
	})
	saved, err := s.client.MyFilters()
	if err != nil {
		rows = append(rows, []string{"-", "(saved filters unavailable)", pluginrpc.Truncate(err.Error(), 60)})
	} else {
		for _, f := range saved {
			rows = append(rows, []string{f.ID, f.Name, pluginrpc.Truncate(f.JQL, 60)})
		}
	}
	return ui.Connected(viewFilters, "Filters", s.baseInfo(fmt.Sprintf("Rows: %d", len(rows))),
		[]string{"ID", "Name", "JQL"},
		rows, "ID", filtersActions()...), nil
}

func (s *Service) viewSiteLocked() (pluginrpc.ViewData, error) {
	u := s.client.User()
	display, account, email := "-", "-", "-"
	api := "ok"
	if u == nil {
		api = "unknown"
	} else {
		display = dash(u.DisplayName)
		account = dash(u.AccountID)
		email = dash(u.EmailAddress)
	}
	board, issue := "-", "-"
	if s.selectedBoard != nil {
		board = fmt.Sprintf("%s (%d)", s.selectedBoard.Name, s.selectedBoard.ID)
	}
	if s.selectedIssue != nil {
		issue = s.selectedIssue.Key
	}
	rows := [][]string{
		{"Display name", display},
		{"Account ID", account},
		{"Email", email},
		{"Site URL", s.client.Site()},
		{"Selected board", board},
		{"Selected issue", issue},
		{"API", api},
	}
	return ui.Connected(viewSite, "Site", s.baseInfo(""),
		[]string{"Property", "Value"}, rows, "Property"), nil
}

func issueRows(issues []Issue, withType bool) [][]string {
	return pluginrpc.MapRows(issues, func(issue Issue) []string {
		sum := pluginrpc.Truncate(issue.Fields.Summary, 80)
		if withType {
			return []string{
				issue.Key,
				dash(issue.TypeName()),
				colorStatus(issue.StatusName(), issue.Fields.Status.StatusCategory.Key),
				colorPriority(issue.PriorityName()),
				sum,
				formatWhen(issue.Fields.Updated),
			}
		}
		return []string{
			issue.Key,
			colorStatus(issue.StatusName(), issue.Fields.Status.StatusCategory.Key),
			issue.AssigneeName(),
			colorPriority(issue.PriorityName()),
			sum,
		}
	})
}

func needBoardView(viewID, title string) (pluginrpc.ViewData, error) {
	return ui.Connected(viewID, title, pluginrpc.FormatInfo(brandName, "select a board"),
		[]string{"Property", "Value"},
		[][]string{
			{"Status", "no board selected"},
			{"Hint", "Open view 1 and press Enter on a board"},
		},
		"Property"), nil
}

func needIssueView(viewID, title string) (pluginrpc.ViewData, error) {
	return ui.Connected(viewID, title, pluginrpc.FormatInfo(brandName, "select an issue"),
		[]string{"Property", "Value"},
		[][]string{
			{"Status", "no issue selected"},
			{"Hint", "Select an issue on Mine or Board first"},
		},
		"Property"), nil
}
