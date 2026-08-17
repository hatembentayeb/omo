package jira

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

// Service is the Jira Cloud RPC plugin (no tview).
type Service struct {
	mu             sync.Mutex
	client         *Client
	name           string
	currentView    string
	selectedBoard  *Board
	selectedSprint *Sprint
	selectedIssue  *selectedIssue
	activeJQL      string
	cachedBoards   []Board
	cachedComments []Comment
	cachedDev      []DevItem
}

func NewService() *Service {
	return &Service{currentView: viewMine}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "jira",
		Version:     "1.0.0",
		Description: "Manage Jira Cloud boards, issues, sprints, comments, and deployments",
		Author:      "OhMyOps",
		License:     "MIT",
		Tags:        []string{"jira", "atlassian", "issues", "boards", "developer-tools"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/jira",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("jira.Configure begin")
	if req.Settings == nil {
		return fmt.Errorf("missing settings")
	}
	token := firstNonEmpty(req.Settings["password"], req.Settings["api_token"], req.Settings["token"])
	email := firstNonEmpty(req.Settings["username"], req.Settings["email"])
	site := normalizeSite(firstNonEmpty(req.Settings["url"], req.Settings["host"]))
	if token == "" {
		return fmt.Errorf("API token required — set KeePass Password")
	}
	if email == "" {
		return fmt.Errorf("email required — set KeePass UserName")
	}
	if site == "" {
		return fmt.Errorf("site URL required — set KeePass URL to https://your-site.atlassian.net")
	}
	s.name = firstNonEmpty(req.Settings["name"], req.Settings["title"], "jira")
	s.client = NewClient(site, email, token)
	s.selectedBoard = nil
	s.selectedSprint = nil
	s.selectedIssue = nil
	s.activeJQL = ""
	s.cachedBoards = nil
	s.cachedComments = nil
	s.cachedDev = nil
	if _, err := s.client.Myself(); err != nil {
		s.client = nil
		return fmt.Errorf("jira login failed: %w", err)
	}
	pluginrpc.RPCLog("jira.Configure ok site=%s user=%s", site, email)
	return nil
}

func (s *Service) GetView(req pluginrpc.ViewRequest) (pluginrpc.ViewData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.View == pluginrpc.DashboardView {
		return s.viewDashboardLocked()
	}
	viewID := req.View
	if viewID == "" {
		viewID = s.currentView
	}
	if viewID == "" {
		viewID = viewMine
	}
	return s.buildViewLocked(viewID)
}

func (s *Service) DoAction(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	action := req.Action
	from := req.View
	if from == "" {
		from = s.currentView
	}

	if strings.HasPrefix(action, "goto_") {
		viewID := strings.TrimPrefix(action, "goto_")
		s.captureContextLocked(from, viewID, req.Payload)
		view, err := s.buildViewLocked(viewID)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "switched to " + viewID, Next: &view}, nil
	}

	switch action {
	case "refresh", "":
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "refreshed", Next: &view, Reaction: "fresh"}, nil
	case "select_board":
		return s.actionSelectBoardLocked(req.Payload)
	case "select_sprint":
		return s.actionSelectSprintLocked(req.Payload)
	case "issue_detail":
		return s.actionIssueDetailLocked(req.Payload)
	case "close":
		return s.actionCloseReopenLocked(req.Payload, true)
	case "reopen":
		return s.actionCloseReopenLocked(req.Payload, false)
	case "assign_me":
		return s.actionAssignLocked(req.Payload, true)
	case "unassign":
		return s.actionAssignLocked(req.Payload, false)
	case "apply_transition":
		return s.actionApplyTransitionLocked(req.Payload)
	case "add_comment":
		return s.actionAddCommentLocked(req.Payload)
	case "comment_detail":
		return s.actionCommentDetailLocked(req.Payload)
	case "create_issue":
		return s.actionCreateIssueLocked(req.Payload)
	case "run_jql":
		return s.actionRunJQLLocked(req.Payload)
	case "run_filter":
		return s.actionRunFilterLocked(req.Payload)
	case "clear_jql":
		s.activeJQL = ""
		s.selectedSprint = nil
		view, err := s.buildViewLocked(viewIssues)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "cleared JQL", Next: &view, Reaction: "ok"}, nil
	case "move_to_sprint":
		return s.actionMoveToSprintLocked(req.Payload)
	case "deploy_detail":
		return s.actionDeployDetailLocked(req.Payload)
	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = nil
	s.selectedBoard = nil
	s.selectedSprint = nil
	s.selectedIssue = nil
	return nil
}

func (s *Service) ensureClientLocked() error {
	if s.client == nil || !s.client.Connected() {
		return fmt.Errorf("not configured — set KeePass URL, UserName (email), Password (API token)")
	}
	return nil
}

func (s *Service) payloadKey(payload map[string]string) string {
	return stripColorTags(firstNonEmpty(payload["key"], payload["col0"]))
}

func (s *Service) rememberIssueLocked(key string) {
	key = stripColorTags(key)
	if !looksLikeIssueKey(key) {
		return
	}
	if s.selectedIssue != nil && s.selectedIssue.Key == key {
		return
	}
	s.selectedIssue = &selectedIssue{Key: key}
	if s.client == nil {
		return
	}
	if issue, err := s.client.GetIssue(key); err == nil {
		s.selectedIssue = issueRef(issue)
	}
}

func issueRef(issue *Issue) *selectedIssue {
	if issue == nil {
		return nil
	}
	return &selectedIssue{
		Key:        issue.Key,
		ID:         issue.ID,
		ProjectKey: issue.ProjectKey(),
		Summary:    issue.Fields.Summary,
	}
}

func (s *Service) captureContextLocked(fromView, toView string, payload map[string]string) {
	key := s.payloadKey(payload)
	if isPlaceholderKey(key) {
		return
	}
	switch fromView {
	case viewBoards:
		if toView == viewIssues || toView == viewSprints || toView == viewBacklog {
			_ = s.selectBoardLocked(key, payload)
		}
	case viewMine, viewIssues, viewBacklog:
		if toView == viewTransitions || toView == viewComments || toView == viewDeploys {
			s.rememberIssueLocked(key)
		}
	case viewSprints:
		if toView == viewIssues {
			_ = s.selectSprintLocked(key)
		}
	}
}

func (s *Service) selectBoardLocked(idStr string, payload map[string]string) error {
	if err := s.ensureClientLocked(); err != nil {
		return err
	}
	idStr = stripColorTags(idStr)
	var id int64
	if _, err := fmt.Sscan(idStr, &id); err != nil || id == 0 {
		return fmt.Errorf("select a board")
	}
	name := stripColorTags(firstNonEmpty(payload["col1"], payload["name"]))
	typ := stripColorTags(firstNonEmpty(payload["col2"], payload["type"]))
	project := stripColorTags(firstNonEmpty(payload["col3"], payload["project"]))
	for i := range s.cachedBoards {
		if s.cachedBoards[i].ID == id {
			b := s.cachedBoards[i]
			s.selectedBoard = &b
			s.selectedSprint = nil
			s.activeJQL = ""
			return nil
		}
	}
	s.selectedBoard = &Board{ID: id, Name: name, Type: typ}
	s.selectedBoard.Location.ProjectKey = project
	s.selectedSprint = nil
	s.activeJQL = ""
	return nil
}

func (s *Service) selectSprintLocked(idStr string) error {
	idStr = stripColorTags(idStr)
	var id int64
	if _, err := fmt.Sscan(idStr, &id); err != nil || id == 0 {
		return fmt.Errorf("select a sprint")
	}
	s.selectedSprint = &Sprint{ID: id}
	s.activeJQL = ""
	return nil
}

func (s *Service) issueKeyFromPayload(payload map[string]string) string {
	key := s.payloadKey(payload)
	if looksLikeIssueKey(key) {
		return key
	}
	if s.selectedIssue != nil {
		return s.selectedIssue.Key
	}
	return ""
}

func (s *Service) projectKeyLocked() string {
	if s.selectedBoard != nil {
		if k := s.selectedBoard.ProjectKey(); k != "" && !strings.Contains(k, " ") {
			return k
		}
	}
	if s.selectedIssue != nil && s.selectedIssue.ProjectKey != "" {
		return s.selectedIssue.ProjectKey
	}
	return ""
}
