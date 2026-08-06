package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

// Service is the RPC-facing git backend (no tview).
type Service struct {
	mu          sync.Mutex
	client      *GitClient
	repos       []GitRepository
	currentPath string
	currentView string
	name        string
	username    string
	token       string
	remoteURL   string
}

// NewService creates a git RPC service.
func NewService() *Service {
	return &Service{
		client:      NewGitClient(),
		currentView: viewRepos,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "git",
		Version:     "2.0.0",
		Description: "Git repository management with status, commits, branches, remotes, stash, and tags",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"version-control", "git", "development", "vcs"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/git",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("Service.Configure begin")

	if req.Settings == nil {
		return fmt.Errorf("missing settings")
	}
	s.name = req.Settings["name"]
	s.username = req.Settings["username"]
	s.token = req.Settings["password"]
	s.remoteURL = req.Settings["url"]
	if s.remoteURL == "" {
		s.remoteURL = req.Settings["host"]
	}
	path := req.Settings["path"]
	if path == "" {
		return fmt.Errorf("path is required (KeePass custom attribute)")
	}
	if !filepath.IsAbs(path) {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path)
		}
	}
	s.currentPath = path
	name := s.name
	if name == "" {
		name = filepath.Base(path)
	}
	s.repos = []GitRepository{{Name: name, Path: path}}
	s.refreshRepoLocked(&s.repos[0])
	pluginrpc.RPCLog("Service.Configure name=%s path=%s", name, path)
	return nil
}

func (s *Service) GetView(req pluginrpc.ViewRequest) (pluginrpc.ViewData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	viewID := req.View
	if viewID == "" {
		viewID = s.currentView
	}
	if viewID == "" {
		viewID = viewRepos
	}
	pluginrpc.RPCLog("Service.GetView begin view=%s path=%s", viewID, s.currentPath)
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
	if strings.HasPrefix(action, "goto_") {
		viewID := strings.TrimPrefix(action, "goto_")
		// Selecting a repo row updates current path when on repos view
		if s.currentView == viewRepos {
			if key := req.Payload["key"]; key != "" {
				s.selectRepoByNameLocked(key)
			}
		}
		view, err := s.buildViewLocked(viewID)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "switched to " + viewID, Next: &view}, nil
	}

	key := req.Payload["key"]
	path := s.currentPath

	switch action {
	case "refresh", "":
		if s.currentView == viewRepos {
			for i := range s.repos {
				s.refreshRepoLocked(&s.repos[i])
			}
		}
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "refreshed", Next: &view}, nil

	case "select_repo":
		if key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no repo selected"}, nil
		}
		if !s.selectRepoByNameLocked(key) {
			return pluginrpc.ActionResult{OK: false, Message: "repo not found"}, nil
		}
		view, _ := s.buildViewLocked(viewRepos)
		return pluginrpc.ActionResult{OK: true, Message: "selected " + key, Next: &view}, nil

	case "fetch":
		if path == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no repo selected"}, nil
		}
		msg, err := s.client.Fetch(path)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		s.refreshCurrentLocked()
		view, _ := s.buildViewLocked(s.currentView)
		return pluginrpc.ActionResult{OK: true, Message: msg, Next: &view}, nil

	case "pull":
		if path == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no repo selected"}, nil
		}
		msg, err := s.client.Pull(path)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		s.refreshCurrentLocked()
		view, _ := s.buildViewLocked(s.currentView)
		return pluginrpc.ActionResult{OK: true, Message: msg, Next: &view}, nil

	case "push":
		if path == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no repo selected"}, nil
		}
		msg, err := s.client.Push(path)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(s.currentView)
		return pluginrpc.ActionResult{OK: true, Message: msg, Next: &view}, nil

	case "stage":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no file selected"}, nil
		}
		if err := s.client.StageFile(path, key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewStatus)
		return pluginrpc.ActionResult{OK: true, Message: "staged " + key, Next: &view}, nil

	case "unstage":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no file selected"}, nil
		}
		if err := s.client.UnstageFile(path, key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewStatus)
		return pluginrpc.ActionResult{OK: true, Message: "unstaged " + key, Next: &view}, nil

	case "diff":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no selection"}, nil
		}
		var body string
		var err error
		title := "Diff: " + key
		switch s.currentView {
		case viewCommits:
			body, err = s.client.GetCommitDiff(path, key)
			title = "Commit Diff: " + key
		default:
			body, err = s.client.GetFileDiff(path, key)
		}
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: title, ModalBody: body}, nil

	case "view_details":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no selection"}, nil
		}
		body, err := s.client.GetCommitDetails(path, key)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Commit: " + key, ModalBody: body}, nil

	case "checkout":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no ref selected"}, nil
		}
		if err := s.client.Checkout(path, key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		s.refreshCurrentLocked()
		view, _ := s.buildViewLocked(s.currentView)
		return pluginrpc.ActionResult{OK: true, Message: "checked out " + key, Next: &view}, nil

	case "delete":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no selection"}, nil
		}
		var err error
		switch s.currentView {
		case viewBranches:
			err = s.client.DeleteBranch(path, key)
		case viewRemotes:
			err = s.client.RemoveRemote(path, key)
		case viewStash:
			err = s.client.DropStash(path, key)
		case viewTags:
			err = s.client.DeleteTag(path, key)
		default:
			return pluginrpc.ActionResult{OK: false, Message: "delete not supported in this view"}, nil
		}
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(s.currentView)
		return pluginrpc.ActionResult{OK: true, Message: "deleted " + key, Next: &view}, nil

	case "restore":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no file selected"}, nil
		}
		if err := s.client.RestoreFile(path, key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewStatus)
		return pluginrpc.ActionResult{OK: true, Message: "restored " + key, Next: &view}, nil

	case "revert":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no commit selected"}, nil
		}
		if err := s.client.RevertCommit(path, key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewCommits)
		return pluginrpc.ActionResult{OK: true, Message: "reverted " + key, Next: &view}, nil

	case "cherry_pick":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no commit selected"}, nil
		}
		if err := s.client.CherryPick(path, key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewCommits)
		return pluginrpc.ActionResult{OK: true, Message: "cherry-picked " + key, Next: &view}, nil

	case "merge":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no branch selected"}, nil
		}
		if err := s.client.MergeBranch(path, key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewBranches)
		return pluginrpc.ActionResult{OK: true, Message: "merged " + key, Next: &view}, nil

	case "fetch_remote":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no remote selected"}, nil
		}
		msg, err := s.client.FetchRemote(path, key)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewRemotes)
		return pluginrpc.ActionResult{OK: true, Message: msg, Next: &view}, nil

	case "prune_remote":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no remote selected"}, nil
		}
		if err := s.client.PruneRemote(path, key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewRemotes)
		return pluginrpc.ActionResult{OK: true, Message: "pruned " + key, Next: &view}, nil

	case "apply_stash":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no stash selected"}, nil
		}
		if err := s.client.ApplyStash(path, key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewStash)
		return pluginrpc.ActionResult{OK: true, Message: "applied stash " + key, Next: &view}, nil

	case "pop_stash":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no stash selected"}, nil
		}
		if err := s.client.PopStash(path, key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewStash)
		return pluginrpc.ActionResult{OK: true, Message: "popped stash " + key, Next: &view}, nil

	case "view_stash":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no stash selected"}, nil
		}
		body, err := s.client.ShowStash(path, key)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Stash " + key, ModalBody: body}, nil

	case "push_tag":
		if path == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no tag selected"}, nil
		}
		if err := s.client.PushTag(path, key); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewTags)
		return pluginrpc.ActionResult{OK: true, Message: "pushed tag " + key, Next: &view}, nil

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) Stop() error {
	return nil
}

func (s *Service) selectRepoByNameLocked(name string) bool {
	for _, r := range s.repos {
		if r.Name == name || r.Path == name {
			s.currentPath = r.Path
			return true
		}
	}
	return false
}

func (s *Service) refreshCurrentLocked() {
	for i := range s.repos {
		if s.repos[i].Path == s.currentPath {
			s.refreshRepoLocked(&s.repos[i])
			return
		}
	}
}

func (s *Service) refreshRepoLocked(repo *GitRepository) {
	if _, err := os.Stat(repo.Path); os.IsNotExist(err) {
		repo.Status = "not found"
		return
	}
	if branch, err := s.client.GetCurrentBranch(repo.Path); err == nil {
		repo.Branch = branch
	}
	if modified, staged, untracked, err := s.client.GetStatus(repo.Path); err == nil {
		repo.Modified = modified
		repo.Staged = staged
		repo.Untracked = untracked
		if modified > 0 || staged > 0 || untracked > 0 {
			repo.Status = "dirty"
		} else {
			repo.Status = "clean"
		}
	}
	if lastCommit, err := s.client.GetLastCommit(repo.Path); err == nil {
		repo.LastCommit = lastCommit
	}
	repo.lastUpdated = time.Now()
}
