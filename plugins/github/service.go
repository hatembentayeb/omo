package github

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

// Service is the RPC-facing GitHub backend (no tview).
type Service struct {
	mu          sync.Mutex
	client      *GitHubClient
	account     *GitHubAccount
	activeRepo  *GitHubRepo
	cachedRepos []GitHubRepo
	prState     string
	currentView string
}

// NewService creates a GitHub RPC service.
func NewService() *Service {
	return &Service{
		client:      NewGitHubClient(),
		prState:     "open",
		currentView: viewRepos,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "github",
		Version:     "1.0.0",
		Description: "Manage GitHub PRs, Actions pipelines, environment variables, secrets, and releases",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"github", "ci-cd", "devops", "pull-requests", "actions"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/github",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("Service.Configure begin")

	if req.Settings == nil {
		return fmt.Errorf("missing settings")
	}
	token := req.Settings["password"]
	if token == "" {
		return fmt.Errorf("password (GitHub token) is required")
	}
	acctType := req.Settings["type"]
	if acctType == "" {
		if req.Settings["username"] != "" {
			acctType = "org"
		} else {
			acctType = "user"
		}
	}
	apiURL := req.Settings["host"]
	if apiURL == "" {
		apiURL = req.Settings["url"]
	}
	acct := &GitHubAccount{
		Name:        req.Settings["name"],
		Owner:       req.Settings["username"],
		Token:       token,
		APIURL:      apiURL,
		Description: req.Settings["notes"],
		AccountType: acctType,
	}
	s.account = acct
	s.activeRepo = nil
	s.cachedRepos = nil
	s.client.SetAccount(acct)
	pluginrpc.RPCLog("Service.Configure name=%s type=%s owner=%s", acct.Name, acct.AccountType, acct.Owner)
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
		viewID = viewRepos
	}
	pluginrpc.RPCLog("Service.GetView begin view=%s", viewID)
	start := time.Now()
	view, err := s.buildViewLocked(viewID)
	if err != nil {
		pluginrpc.RPCLog("Service.GetView err=%v", err)
		return pluginrpc.ViewData{}, err
	}
	pluginrpc.RPCLog("Service.GetView OK view=%s rows=%d dur=%s", view.View, len(view.Rows), time.Since(start))
	return view, nil
}

func (s *Service) DoAction(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	action := req.Action
	key := stripColorTags(req.Payload["key"])

	if strings.HasPrefix(action, "goto_") {
		viewID := strings.TrimPrefix(action, "goto_")
		if s.currentView == viewRepos && key != "" && viewID != viewRepos {
			if err := s.selectRepoLocked(key); err != nil {
				return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
			}
		}
		if viewID != viewRepos && !s.client.HasActiveRepo() {
			return pluginrpc.ActionResult{OK: false, Message: "select a repository first"}, nil
		}
		if viewID == viewRepos {
			s.activeRepo = nil
			s.client.SetActiveRepo(nil)
		}
		view, err := s.buildViewLocked(viewID)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "switched to " + viewID, Next: &view}, nil
	}

	switch action {
	case "refresh", "":
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "refreshed", Next: &view}, nil

	case "select_repo":
		if err := s.selectRepoLocked(key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewRepos)
		return pluginrpc.ActionResult{OK: true, Message: "selected " + key, Next: &view}, nil

	case "merge":
		num, ok := parsePRNumber(key)
		if !ok {
			return pluginrpc.ActionResult{OK: false, Message: "no PR selected"}, nil
		}
		method := req.Payload["method"]
		if method == "" {
			method = "merge"
		}
		if err := s.client.MergePullRequest(num, method); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewPRs)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("merged PR #%d", num), Next: &view}, nil

	case "close":
		num, ok := parsePRNumber(key)
		if !ok {
			return pluginrpc.ActionResult{OK: false, Message: "no PR selected"}, nil
		}
		if err := s.client.ClosePullRequest(num); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewPRs)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("closed PR #%d", num), Next: &view}, nil

	case "reopen":
		num, ok := parsePRNumber(key)
		if !ok {
			return pluginrpc.ActionResult{OK: false, Message: "no PR selected"}, nil
		}
		if err := s.client.ReopenPullRequest(num); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewPRs)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("reopened PR #%d", num), Next: &view}, nil

	case "approve":
		num, ok := parsePRNumber(key)
		if !ok {
			return pluginrpc.ActionResult{OK: false, Message: "no PR selected"}, nil
		}
		if err := s.client.ApprovePullRequest(num); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("approved PR #%d", num)}, nil

	case "toggle_pr_state":
		if s.prState == "open" {
			s.prState = "closed"
		} else if s.prState == "closed" {
			s.prState = "all"
		} else {
			s.prState = "open"
		}
		view, err := s.buildViewLocked(viewPRs)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "filter: " + s.prState, Next: &view}, nil

	case "view_checks":
		num, ok := parsePRNumber(key)
		if !ok {
			return pluginrpc.ActionResult{OK: false, Message: "no PR selected"}, nil
		}
		checks, err := s.client.GetPRChecks(num)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		var b strings.Builder
		for _, c := range checks {
			fmt.Fprintf(&b, "%s  %s/%s\n", c.Name, c.Status, c.Conclusion)
		}
		if b.Len() == 0 {
			b.WriteString("No checks found")
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: fmt.Sprintf("PR #%d Checks", num), ModalBody: b.String()}, nil

	case "view_reviews":
		num, ok := parsePRNumber(key)
		if !ok {
			return pluginrpc.ActionResult{OK: false, Message: "no PR selected"}, nil
		}
		reviews, err := s.client.GetPRReviews(num)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		var b strings.Builder
		for _, r := range reviews {
			fmt.Fprintf(&b, "%s [%s]\n%s\n\n", r.User, r.State, r.Body)
		}
		if b.Len() == 0 {
			b.WriteString("No reviews found")
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: fmt.Sprintf("PR #%d Reviews", num), ModalBody: b.String()}, nil

	case "dispatch":
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil || id == 0 {
			return pluginrpc.ActionResult{OK: false, Message: "no workflow selected"}, nil
		}
		ref := req.Payload["ref"]
		if ref == "" && s.activeRepo != nil {
			ref = s.activeRepo.DefaultBranch
		}
		if ref == "" {
			ref = "main"
		}
		if err := s.client.TriggerWorkflowDispatch(id, ref); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("dispatched workflow %d", id)}, nil

	case "cancel_run":
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil || id == 0 {
			return pluginrpc.ActionResult{OK: false, Message: "no run selected"}, nil
		}
		if err := s.client.CancelWorkflowRun(id); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewRuns)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("cancelled run %d", id), Next: &view}, nil

	case "rerun":
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil || id == 0 {
			return pluginrpc.ActionResult{OK: false, Message: "no run selected"}, nil
		}
		if err := s.client.RerunWorkflow(id); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewRuns)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("re-running %d", id), Next: &view}, nil

	case "view_jobs":
		id, err := strconv.ParseInt(key, 10, 64)
		if err != nil || id == 0 {
			return pluginrpc.ActionResult{OK: false, Message: "no run selected"}, nil
		}
		jobs, err := s.client.GetWorkflowRunJobs(id)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		var b strings.Builder
		for _, j := range jobs {
			fmt.Fprintf(&b, "%s  %s/%s  %s\n", j.Name, j.Status, j.Conclusion, j.Duration)
		}
		if b.Len() == 0 {
			b.WriteString("No jobs found")
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: fmt.Sprintf("Run %d Jobs", id), ModalBody: b.String()}, nil

	case "delete_secret":
		if key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no secret selected"}, nil
		}
		if err := s.client.DeleteRepoSecret(key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewSecrets)
		return pluginrpc.ActionResult{OK: true, Message: "deleted secret " + key, Next: &view}, nil

	case "delete_branch":
		if key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no branch selected"}, nil
		}
		name := strings.TrimSuffix(strings.TrimSpace(key), " *")
		if err := s.client.DeleteBranch(name); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewBranches)
		return pluginrpc.ActionResult{OK: true, Message: "deleted branch " + name, Next: &view}, nil

	case "delete_release":
		// key is tag name; find id from list
		releases, err := s.client.ListReleases()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		var id int64
		for _, r := range releases {
			if r.TagName == key {
				id = r.ID
				break
			}
		}
		if id == 0 {
			return pluginrpc.ActionResult{OK: false, Message: "release not found"}, nil
		}
		if err := s.client.DeleteRelease(id); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewReleases)
		return pluginrpc.ActionResult{OK: true, Message: "deleted release " + key, Next: &view}, nil

	case "delete_variable":
		if key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no variable selected"}, nil
		}
		if err := s.client.DeleteRepoVariable(key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewEnvVars)
		return pluginrpc.ActionResult{OK: true, Message: "deleted variable " + key, Next: &view}, nil

	case "create_variable":
		name := req.Payload["name"]
		if name == "" {
			name = key
		}
		value := req.Payload["value"]
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "name required"}, nil
		}
		if err := s.client.CreateRepoVariable(name, value); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewEnvVars)
		return pluginrpc.ActionResult{OK: true, Message: "created " + name, Next: &view}, nil

	case "update_variable":
		name := key
		value := req.Payload["value"]
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no variable selected"}, nil
		}
		if err := s.client.UpdateRepoVariable(name, value); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewEnvVars)
		return pluginrpc.ActionResult{OK: true, Message: "updated " + name, Next: &view}, nil

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeRepo = nil
	s.account = nil
	return nil
}

func (s *Service) selectRepoLocked(fullName string) error {
	fullName = stripColorTags(fullName)
	for i := range s.cachedRepos {
		if s.cachedRepos[i].FullName == fullName || s.cachedRepos[i].Name == fullName {
			repo := s.cachedRepos[i]
			s.activeRepo = &repo
			s.client.SetActiveRepo(&repo)
			return nil
		}
	}
	// Refresh cache once if miss
	repos, err := s.client.ListRepos()
	if err != nil {
		return err
	}
	s.cachedRepos = repos
	for i := range s.cachedRepos {
		if s.cachedRepos[i].FullName == fullName || s.cachedRepos[i].Name == fullName {
			repo := s.cachedRepos[i]
			s.activeRepo = &repo
			s.client.SetActiveRepo(&repo)
			return nil
		}
	}
	return fmt.Errorf("repo not found: %s", fullName)
}

func parsePRNumber(key string) (int, bool) {
	key = stripColorTags(key)
	key = strings.TrimPrefix(key, "#")
	n, err := strconv.Atoi(key)
	return n, err == nil && n > 0
}
