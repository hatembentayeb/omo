package jira

import (
	"fmt"
	"strings"

	"omo/pkg/pluginrpc"
)

func (s *Service) actionSelectBoardLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	key := s.payloadKey(payload)
	if err := s.selectBoardLocked(key, payload); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "pick?"}, nil
	}
	view, err := s.buildViewLocked(viewIssues)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	name := s.selectedBoard.Name
	return pluginrpc.ActionResult{OK: true, Message: "board " + name, Next: &view, Reaction: "board"}, nil
}

func (s *Service) actionSelectSprintLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	key := s.payloadKey(payload)
	if err := s.selectSprintLocked(key); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "pick?"}, nil
	}
	if name := stripColorTags(firstNonEmpty(payload["col1"], payload["name"])); name != "" {
		s.selectedSprint.Name = name
		s.selectedSprint.State = stripColorTags(payload["col2"])
	}
	view, err := s.buildViewLocked(viewIssues)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	label := s.selectedSprint.Name
	if label == "" {
		label = key
	}
	return pluginrpc.ActionResult{OK: true, Message: "sprint " + label, Next: &view, Reaction: "sprint"}, nil
}

func (s *Service) actionIssueDetailLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	key := s.issueKeyFromPayload(payload)
	if key == "" {
		return pluginrpc.ActionResult{OK: false, Message: "select an issue", Reaction: "pick?"}, nil
	}
	issue, err := s.client.GetIssue(key)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	s.selectedIssue = issueRef(issue)
	desc := adfToText(issue.Fields.Description)
	if desc == "" {
		desc = "(no description)"
	}
	if len(desc) > 4000 {
		desc = desc[:4000] + "\n… truncated …"
	}
	body := fmt.Sprintf(
		"Key:       %s\nType:      %s\nStatus:    %s\nPriority:  %s\nAssignee:  %s\nProject:   %s\nUpdated:   %s\nSummary:   %s\n\n%s\n",
		issue.Key, issue.TypeName(), issue.StatusName(), issue.PriorityName(),
		issue.AssigneeName(), dash(issue.ProjectKey()), formatWhen(issue.Fields.Updated),
		issue.Fields.Summary, desc,
	)
	return pluginrpc.ActionResult{OK: true, ModalTitle: issue.Key, ModalBody: body}, nil
}

func (s *Service) actionCloseReopenLocked(payload map[string]string, closeIt bool) (pluginrpc.ActionResult, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	key := s.issueKeyFromPayload(payload)
	if key == "" {
		return pluginrpc.ActionResult{OK: false, Message: "select an issue", Reaction: "pick?"}, nil
	}
	s.rememberIssueLocked(key)
	ts, err := s.client.Transitions(key)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	var t *Transition
	word := "reopen"
	if closeIt {
		t = pickCloseTransition(ts)
		word = "close"
	} else {
		t = pickReopenTransition(ts)
	}
	if t == nil {
		view, _ := s.buildViewLocked(viewTransitions)
		return pluginrpc.ActionResult{
			OK: false, Message: "no " + word + " transition — pick one", Next: &view, Reaction: "pick?",
		}, nil
	}
	if err := s.client.DoTransition(key, t.ID); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	view, _ := s.buildViewLocked(s.currentView)
	return pluginrpc.ActionResult{
		OK: true, Message: fmt.Sprintf("%s %s → %s", word, key, t.To.Name), Next: &view, Reaction: word,
	}, nil
}

func (s *Service) actionAssignLocked(payload map[string]string, me bool) (pluginrpc.ActionResult, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	key := s.issueKeyFromPayload(payload)
	if key == "" {
		return pluginrpc.ActionResult{OK: false, Message: "select an issue", Reaction: "pick?"}, nil
	}
	s.rememberIssueLocked(key)
	accountID := ""
	msg := "unassigned " + key
	word := "free"
	if me {
		u := s.client.User()
		if u == nil || u.AccountID == "" {
			return pluginrpc.ActionResult{OK: false, Message: "current user unknown"}, nil
		}
		accountID = u.AccountID
		msg = "assigned " + key + " to me"
		word = "mine"
	}
	if err := s.client.Assign(key, accountID); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	view, _ := s.buildViewLocked(s.currentView)
	return pluginrpc.ActionResult{OK: true, Message: msg, Next: &view, Reaction: word}, nil
}

func (s *Service) actionApplyTransitionLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	if s.selectedIssue == nil || s.selectedIssue.Key == "" {
		return pluginrpc.ActionResult{OK: false, Message: "select an issue first"}, nil
	}
	id := s.payloadKey(payload)
	if isPlaceholderKey(id) {
		return pluginrpc.ActionResult{OK: false, Message: "select a transition", Reaction: "pick?"}, nil
	}
	if err := s.client.DoTransition(s.selectedIssue.Key, id); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	name := stripColorTags(firstNonEmpty(payload["col1"], payload["name"]))
	view, _ := s.buildViewLocked(viewTransitions)
	return pluginrpc.ActionResult{
		OK: true, Message: s.selectedIssue.Key + " → " + dash(name), Next: &view, Reaction: "moved",
	}, nil
}

func (s *Service) actionAddCommentLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	key := s.issueKeyFromPayload(payload)
	if key == "" {
		return pluginrpc.ActionResult{OK: false, Message: "select an issue first"}, nil
	}
	s.rememberIssueLocked(key)
	body := strings.TrimSpace(payload["body"])
	if body == "" {
		return pluginrpc.ActionResult{OK: false, Message: "comment required"}, nil
	}
	if err := s.client.AddComment(key, body); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	view, _ := s.buildViewLocked(viewComments)
	return pluginrpc.ActionResult{OK: true, Message: "commented on " + key, Next: &view, Reaction: "yay!"}, nil
}

func (s *Service) actionCommentDetailLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	id := s.payloadKey(payload)
	for _, c := range s.cachedComments {
		if c.ID == id {
			text := adfToText(c.Body)
			if text == "" {
				text = "(empty)"
			}
			body := fmt.Sprintf("Author:  %s\nCreated: %s\n\n%s\n", dash(c.Author.DisplayName), formatWhen(c.Created), text)
			return pluginrpc.ActionResult{OK: true, ModalTitle: "Comment", ModalBody: body}, nil
		}
	}
	return pluginrpc.ActionResult{OK: false, Message: "select a comment"}, nil
}

func (s *Service) actionCreateIssueLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	summary := strings.TrimSpace(firstNonEmpty(payload["summary"], payload["name"]))
	if summary == "" {
		return pluginrpc.ActionResult{OK: false, Message: "summary required"}, nil
	}
	project := s.projectKeyLocked()
	if project == "" {
		return pluginrpc.ActionResult{OK: false, Message: "select a board first (view 1) so the project is known"}, nil
	}
	issueType := "Task"
	if types, err := s.client.CreateMetaTypes(project); err == nil {
		issueType = pickCreateType(types).Name
	}
	created, err := s.client.CreateIssue(project, issueType, summary)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	s.selectedIssue = &selectedIssue{Key: created.Key, ID: created.ID, ProjectKey: project, Summary: summary}
	view, _ := s.buildViewLocked(s.currentView)
	if s.currentView != viewMine && s.currentView != viewIssues && s.currentView != viewBacklog {
		view, _ = s.buildViewLocked(viewMine)
	}
	return pluginrpc.ActionResult{
		OK: true, Message: "created " + created.Key, Next: &view, Reaction: "new",
		ModalTitle: "Issue Created", ModalBody: fmt.Sprintf("Key: %s\nType: %s\nProject: %s\nSummary: %s\n", created.Key, issueType, project, summary),
	}, nil
}

func (s *Service) actionRunJQLLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	jql := strings.TrimSpace(payload["jql"])
	if jql == "" {
		return pluginrpc.ActionResult{OK: false, Message: "JQL required"}, nil
	}
	s.activeJQL = jql
	s.selectedSprint = nil
	view, err := s.buildViewLocked(viewIssues)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	return pluginrpc.ActionResult{OK: true, Message: "ran JQL", Next: &view, Reaction: "jql"}, nil
}

func (s *Service) actionRunFilterLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	id := s.payloadKey(payload)
	if isPlaceholderKey(id) {
		return pluginrpc.ActionResult{OK: false, Message: "select a filter", Reaction: "pick?"}, nil
	}
	jql := stripColorTags(firstNonEmpty(payload["col2"], payload["jql"]))
	for _, f := range builtinFilters() {
		if f.ID == id {
			jql = f.JQL
			break
		}
	}
	if jql == "" || jql == "-" {
		saved, err := s.client.GetFilter(id)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
		}
		jql = saved.JQL
	}
	s.activeJQL = jql
	s.selectedSprint = nil
	view, err := s.buildViewLocked(viewIssues)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	return pluginrpc.ActionResult{OK: true, Message: "filter " + id, Next: &view, Reaction: "filter"}, nil
}

func (s *Service) actionMoveToSprintLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	if s.selectedBoard == nil {
		return pluginrpc.ActionResult{OK: false, Message: "select a board first"}, nil
	}
	key := s.issueKeyFromPayload(payload)
	if key == "" {
		return pluginrpc.ActionResult{OK: false, Message: "select an issue", Reaction: "pick?"}, nil
	}
	sprints, err := s.client.Sprints(s.selectedBoard.ID)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	var active []Sprint
	for _, sp := range sprints {
		if strings.EqualFold(sp.State, "active") {
			active = append(active, sp)
		}
	}
	if len(active) == 0 {
		view, _ := s.buildViewLocked(viewSprints)
		return pluginrpc.ActionResult{OK: false, Message: "no active sprint", Next: &view, Reaction: "pick?"}, nil
	}
	if len(active) > 1 {
		view, _ := s.buildViewLocked(viewSprints)
		return pluginrpc.ActionResult{OK: false, Message: "multiple active sprints — pick one", Next: &view, Reaction: "pick?"}, nil
	}
	if err := s.client.MoveToSprint(active[0].ID, []string{key}); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	s.selectedSprint = &active[0]
	view, _ := s.buildViewLocked(viewBacklog)
	return pluginrpc.ActionResult{
		OK: true, Message: key + " → " + active[0].Name, Next: &view, Reaction: "moved",
	}, nil
}

func (s *Service) actionDeployDetailLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	kind := stripColorTags(firstNonEmpty(payload["col0"], payload["kind"]))
	name := stripColorTags(firstNonEmpty(payload["col1"], payload["name"]))
	for _, d := range s.cachedDev {
		if d.Kind == kind && d.Name == name {
			body := fmt.Sprintf("Kind:  %s\nName:  %s\nEnv:   %s\nState: %s\nWhen:  %s\nURL:   %s\n",
				d.Kind, d.Name, d.Env, d.State, d.When, dash(d.URL))
			return pluginrpc.ActionResult{OK: true, ModalTitle: d.Kind + " " + d.Name, ModalBody: body}, nil
		}
	}
	return pluginrpc.ActionResult{OK: false, Message: "select a deployment row"}, nil
}
