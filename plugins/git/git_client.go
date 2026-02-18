package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GitClient provides Git operations for repositories using native Go implementation
type GitClient struct {
}

// NewGitClient creates a new Git client
func NewGitClient() *GitClient {
	return &GitClient{}
}

// GetCurrentBranch gets the current branch name for a repository
func (g *GitClient) GetCurrentBranch(repoPath string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	// If it's a detached HEAD, return the short commit hash
	if head.Name().IsBranch() {
		return head.Name().Short(), nil
	}

	// It's a detached HEAD, return the short commit hash
	return head.Hash().String()[:7], nil
}

// GetStatus gets the status counts for a repository
func (g *GitClient) GetStatus(repoPath string) (modified, staged, untracked int, err error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to open repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get status: %w", err)
	}

	for _, fileStatus := range status {
		// File is unstaged/modified
		if fileStatus.Worktree != git.Unmodified {
			if fileStatus.Worktree == git.Untracked {
				untracked++
			} else {
				modified++
			}
		}

		// File is staged
		if fileStatus.Staging != git.Unmodified {
			staged++
		}
	}

	return modified, staged, untracked, nil
}

// GetLastCommit gets the last commit message for a repository
func (g *GitClient) GetLastCommit(repoPath string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return "", fmt.Errorf("failed to get commit: %w", err)
	}

	// Format the commit message
	when := commit.Author.When
	timeAgo := formatTimeAgo(when)

	return fmt.Sprintf("%s %s (%s)",
		head.Hash().String()[:7],
		strings.Split(commit.Message, "\n")[0],
		timeAgo), nil
}

// Fetch fetches updates from the remote repository
func (g *GitClient) Fetch(repoPath string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	err = repo.Fetch(&git.FetchOptions{
		RemoteName: "origin",
		Progress:   nil,
	})

	if err == git.NoErrAlreadyUpToDate {
		return "Already up to date", nil
	}

	if err != nil {
		return "", fmt.Errorf("fetch failed: %w", err)
	}

	return "Fetched latest changes", nil
}

// Pull pulls updates from the remote repository
func (g *GitClient) Pull(repoPath string) (string, error) {
	// Check if there are uncommitted changes
	modified, staged, _, err := g.GetStatus(repoPath)
	if err != nil {
		return "", err
	}

	if modified > 0 || staged > 0 {
		return "", errors.New("cannot pull with uncommitted changes")
	}

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	err = worktree.Pull(&git.PullOptions{
		RemoteName: "origin",
		Progress:   nil,
		Force:      false,
		Auth:       nil, // Explicitly set to nil for public repos
	})

	if err == git.NoErrAlreadyUpToDate {
		return "Already up to date", nil
	}

	if err != nil {
		return "", fmt.Errorf("pull failed: %w", err)
	}

	return "Pulled latest changes", nil
}

func stagingSymbol(code git.StatusCode) string {
	switch code {
	case git.Added:
		return "A"
	case git.Modified:
		return "M"
	case git.Deleted:
		return "D"
	case git.Renamed:
		return "R"
	case git.Copied:
		return "C"
	default:
		return " "
	}
}

func worktreeSymbol(code git.StatusCode) string {
	switch code {
	case git.Added, git.Untracked:
		return "?"
	case git.Modified:
		return "M"
	case git.Deleted:
		return "D"
	case git.Renamed:
		return "R"
	case git.Copied:
		return "C"
	default:
		return " "
	}
}

// GetDetailedStatus gets detailed status for a repository
func (g *GitClient) GetDetailedStatus(repoPath string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	worktree, err := repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := worktree.Status()
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	if len(status) == 0 {
		return "Working tree clean", nil
	}

	var result strings.Builder
	result.WriteString("Changes:\n\n")

	for file, fileStatus := range status {
		s := stagingSymbol(fileStatus.Staging) + worktreeSymbol(fileStatus.Worktree)
		result.WriteString(fmt.Sprintf("%s %s\n", s, file))
	}

	return result.String(), nil
}

// GetLog gets the commit log for a repository
func (g *GitClient) GetLog(repoPath string, count int) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	logOpts := git.LogOptions{
		Order: git.LogOrderCommitterTime,
	}

	commitIter, err := repo.Log(&logOpts)
	if err != nil {
		return "", fmt.Errorf("failed to get log: %w", err)
	}

	var result strings.Builder
	numCommits := 0

	err = commitIter.ForEach(func(commit *object.Commit) error {
		if numCommits >= count {
			return errors.New("reached limit")
		}

		when := commit.Author.When
		timeAgo := formatTimeAgo(when)

		result.WriteString(fmt.Sprintf("%s %s (%s)\n",
			commit.Hash.String()[:7],
			strings.Split(commit.Message, "\n")[0],
			timeAgo))

		numCommits++
		return nil
	})

	if err != nil && err.Error() != "reached limit" {
		return "", fmt.Errorf("error iterating commits: %w", err)
	}

	return result.String(), nil
}

// GetBranches gets the list of branches for a repository
func (g *GitClient) GetBranches(repoPath string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}

	branches, err := repo.Branches()
	if err != nil {
		return "", fmt.Errorf("failed to get branches: %w", err)
	}

	currentBranch, err := g.GetCurrentBranch(repoPath)
	if err != nil {
		currentBranch = ""
	}

	var result strings.Builder

	err = branches.ForEach(func(ref *plumbing.Reference) error {
		branchName := ref.Name().Short()
		prefix := "  "
		if branchName == currentBranch {
			prefix = "* "
		}
		result.WriteString(fmt.Sprintf("%s%s\n", prefix, branchName))
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error iterating branches: %w", err)
	}

	// Get remote branches
	remotes, err := repo.Remotes()
	if err != nil {
		return "", fmt.Errorf("failed to get remotes: %w", err)
	}

	if len(remotes) > 0 {
		result.WriteString("\nRemote branches:\n")

		for _, remote := range remotes {
			refs, err := remote.List(&git.ListOptions{})
			if err != nil {
				continue
			}

			for _, ref := range refs {
				if ref.Name().IsBranch() {
					result.WriteString(fmt.Sprintf("  %s/%s\n",
						remote.Config().Name,
						ref.Name().Short()))
				}
			}
		}
	}

	return result.String(), nil
}

// IsRepo checks if the given path is a Git repository
func (g *GitClient) IsRepo(path string) bool {
	// A more lightweight check that doesn't attempt to open the full repository
	gitDir := filepath.Join(path, ".git")

	// Check if .git directory exists
	if stat, err := os.Stat(gitDir); err == nil && stat.IsDir() {
		// Quick check for critical git files
		headFile := filepath.Join(gitDir, "HEAD")
		if _, err := os.Stat(headFile); err == nil {
			return true
		}
	}

	return false
}

// IsGitRepository checks if the given path is a Git repository
func (gc *GitClient) IsGitRepository(path string) bool {
	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return false
	}
	return true
}

// formatTimeAgo formats a time in the past as a human-readable string
func formatTimeAgo(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	seconds := int(diff.Seconds())
	minutes := seconds / 60
	hours := minutes / 60
	days := hours / 24
	months := days / 30
	years := months / 12

	if years > 0 {
		return fmt.Sprintf("%d years ago", years)
	} else if months > 0 {
		return fmt.Sprintf("%d months ago", months)
	} else if days > 0 {
		return fmt.Sprintf("%d days ago", days)
	} else if hours > 0 {
		return fmt.Sprintf("%d hours ago", hours)
	} else if minutes > 0 {
		return fmt.Sprintf("%d minutes ago", minutes)
	} else {
		return fmt.Sprintf("%d seconds ago", seconds)
	}
}

// Status returns the status of the repository
func (gc *GitClient) Status(path string) string {
	cmd := exec.Command("git", "-C", path, "status")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Error getting status: %v", err)
	}
	return string(output)
}

// Log returns the commit log of the repository
func (gc *GitClient) Log(path string) string {
	cmd := exec.Command("git", "-C", path, "log", "--oneline", "-n", "20")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Error getting log: %v", err)
	}
	return string(output)
}

// Branches returns the branches of the repository
func (gc *GitClient) Branches(path string) string {
	cmd := exec.Command("git", "-C", path, "branch", "-a")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("Error getting branches: %v", err)
	}
	return string(output)
}

// Push pushes changes to the remote repository
func (gc *GitClient) Push(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "push")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("push failed: %s", string(output))
	}
	return "Pushed successfully", nil
}

// GetBranchList returns a list of branch names
func (gc *GitClient) GetBranchList(path string) ([]string, error) {
	cmd := exec.Command("git", "-C", path, "branch", "--format=%(refname:short)")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var branches []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// Checkout checks out a branch or commit
func (gc *GitClient) Checkout(path, ref string) error {
	cmd := exec.Command("git", "-C", path, "checkout", ref)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("checkout failed: %s", string(output))
	}
	return nil
}

// CreateBranch creates and checks out a new branch
func (gc *GitClient) CreateBranch(path, branchName string) error {
	cmd := exec.Command("git", "-C", path, "checkout", "-b", branchName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create branch failed: %s", string(output))
	}
	return nil
}

// GetRemotes returns a list of remote names
func (gc *GitClient) GetRemotes(path string) ([]string, error) {
	cmd := exec.Command("git", "-C", path, "remote")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var remotes []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			remotes = append(remotes, line)
		}
	}
	return remotes, nil
}

// StatusFile represents a file in git status
type StatusFile struct {
	Status string
	Path   string
	Type   string
}

// GetStatusFiles returns the list of changed files
func (gc *GitClient) GetStatusFiles(path string) ([]StatusFile, error) {
	cmd := exec.Command("git", "-C", path, "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var files []StatusFile
	for _, line := range strings.Split(string(output), "\n") {
		if len(line) < 3 {
			continue
		}

		status := strings.TrimSpace(line[:2])
		filePath := strings.TrimSpace(line[3:])

		fileType := "file"
		if strings.HasSuffix(filePath, "/") {
			fileType = "directory"
		}

		files = append(files, StatusFile{
			Status: status,
			Path:   filePath,
			Type:   fileType,
		})
	}
	return files, nil
}

// StageFile stages a file
func (gc *GitClient) StageFile(path, file string) error {
	cmd := exec.Command("git", "-C", path, "add", file)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stage failed: %s", string(output))
	}
	return nil
}

// UnstageFile unstages a file
func (gc *GitClient) UnstageFile(path, file string) error {
	cmd := exec.Command("git", "-C", path, "reset", "HEAD", file)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unstage failed: %s", string(output))
	}
	return nil
}

// GetFileDiff returns the diff for a specific file
func (gc *GitClient) GetFileDiff(path, file string) (string, error) {
	cmd := exec.Command("git", "-C", path, "diff", "--", file)
	output, err := cmd.Output()
	if err != nil {
		// Try staged diff
		cmd = exec.Command("git", "-C", path, "diff", "--cached", "--", file)
		output, err = cmd.Output()
		if err != nil {
			return "", err
		}
	}
	return string(output), nil
}

// RestoreFile restores a file to its last committed state
func (gc *GitClient) RestoreFile(path, file string) error {
	cmd := exec.Command("git", "-C", path, "checkout", "--", file)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("restore failed: %s", string(output))
	}
	return nil
}

// Commit creates a commit with the given message
func (gc *GitClient) Commit(path, message string) error {
	cmd := exec.Command("git", "-C", path, "commit", "-m", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("commit failed: %s", string(output))
	}
	return nil
}

// CommitInfo represents commit information
type CommitInfo struct {
	Hash    string
	Author  string
	Date    string
	Message string
}

// GetCommits returns a list of commits
func (gc *GitClient) GetCommits(path string, count int) ([]CommitInfo, error) {
	cmd := exec.Command("git", "-C", path, "log", fmt.Sprintf("-n%d", count),
		"--format=%H|%an|%ar|%s")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var commits []CommitInfo
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) == 4 {
			commits = append(commits, CommitInfo{
				Hash:    parts[0][:7],
				Author:  parts[1],
				Date:    parts[2],
				Message: parts[3],
			})
		}
	}
	return commits, nil
}

// GetCommitDetails returns detailed info about a commit
func (gc *GitClient) GetCommitDetails(path, hash string) (string, error) {
	cmd := exec.Command("git", "-C", path, "show", "--stat", hash)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// GetCommitDiff returns the diff for a commit
func (gc *GitClient) GetCommitDiff(path, hash string) (string, error) {
	cmd := exec.Command("git", "-C", path, "show", hash)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// RevertCommit reverts a commit
func (gc *GitClient) RevertCommit(path, hash string) error {
	cmd := exec.Command("git", "-C", path, "revert", "--no-edit", hash)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("revert failed: %s", string(output))
	}
	return nil
}

// CherryPick cherry-picks a commit
func (gc *GitClient) CherryPick(path, hash string) error {
	cmd := exec.Command("git", "-C", path, "cherry-pick", hash)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cherry-pick failed: %s", string(output))
	}
	return nil
}

// BranchInfo represents branch information
type BranchInfo struct {
	Name      string
	IsCurrent bool
	Tracking  string
	Ahead     int
	Behind    int
}

// GetBranchesInfo returns detailed branch information
func (gc *GitClient) GetBranchesInfo(path string) ([]BranchInfo, error) {
	cmd := exec.Command("git", "-C", path, "branch", "-vv", "--format=%(HEAD)|%(refname:short)|%(upstream:short)|%(upstream:track)")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to simple branch list
		branches, err := gc.GetBranchList(path)
		if err != nil {
			return nil, err
		}
		var result []BranchInfo
		currentBranch, _ := gc.GetCurrentBranch(path)
		for _, b := range branches {
			result = append(result, BranchInfo{
				Name:      b,
				IsCurrent: b == currentBranch,
			})
		}
		return result, nil
	}

	var branches []BranchInfo
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		parts := strings.Split(line, "|")
		if len(parts) >= 2 {
			branch := BranchInfo{
				Name:      parts[1],
				IsCurrent: parts[0] == "*",
			}
			if len(parts) >= 3 {
				branch.Tracking = parts[2]
			}
			if len(parts) >= 4 {
				track := parts[3]
				if strings.Contains(track, "ahead") {
					fmt.Sscanf(track, "[ahead %d", &branch.Ahead)
				}
				if strings.Contains(track, "behind") {
					fmt.Sscanf(track, "[behind %d", &branch.Behind)
				}
			}
			branches = append(branches, branch)
		}
	}
	return branches, nil
}

// DeleteBranch deletes a branch
func (gc *GitClient) DeleteBranch(path, branch string) error {
	cmd := exec.Command("git", "-C", path, "branch", "-d", branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try force delete
		cmd = exec.Command("git", "-C", path, "branch", "-D", branch)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("delete failed: %s", string(output))
		}
	}
	return nil
}

// RenameBranch renames a branch
func (gc *GitClient) RenameBranch(path, oldName, newName string) error {
	cmd := exec.Command("git", "-C", path, "branch", "-m", oldName, newName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rename failed: %s", string(output))
	}
	return nil
}

// MergeBranch merges a branch into the current branch
func (gc *GitClient) MergeBranch(path, branch string) error {
	cmd := exec.Command("git", "-C", path, "merge", branch)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("merge failed: %s", string(output))
	}
	return nil
}

// RemoteInfo represents remote information
type RemoteInfo struct {
	Name     string
	FetchURL string
	PushURL  string
}

// GetRemotesInfo returns detailed remote information
func (gc *GitClient) GetRemotesInfo(path string) ([]RemoteInfo, error) {
	cmd := exec.Command("git", "-C", path, "remote", "-v")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	remoteMap := make(map[string]*RemoteInfo)
	for _, line := range strings.Split(string(output), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			name := parts[0]
			url := parts[1]
			isFetch := len(parts) >= 3 && strings.Contains(parts[2], "fetch")

			if _, exists := remoteMap[name]; !exists {
				remoteMap[name] = &RemoteInfo{Name: name}
			}
			if isFetch {
				remoteMap[name].FetchURL = url
			} else {
				remoteMap[name].PushURL = url
			}
		}
	}

	var remotes []RemoteInfo
	for _, r := range remoteMap {
		if r.PushURL == "" {
			r.PushURL = r.FetchURL
		}
		remotes = append(remotes, *r)
	}
	return remotes, nil
}

// AddRemote adds a new remote
func (gc *GitClient) AddRemote(path, name, url string) error {
	cmd := exec.Command("git", "-C", path, "remote", "add", name, url)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("add remote failed: %s", string(output))
	}
	return nil
}

// RemoveRemote removes a remote
func (gc *GitClient) RemoveRemote(path, name string) error {
	cmd := exec.Command("git", "-C", path, "remote", "remove", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("remove remote failed: %s", string(output))
	}
	return nil
}

// FetchRemote fetches from a specific remote
func (gc *GitClient) FetchRemote(path, remote string) (string, error) {
	cmd := exec.Command("git", "-C", path, "fetch", remote)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("fetch failed: %s", string(output))
	}
	return fmt.Sprintf("Fetched from %s", remote), nil
}

// PruneRemote prunes stale remote-tracking branches
func (gc *GitClient) PruneRemote(path, remote string) error {
	cmd := exec.Command("git", "-C", path, "remote", "prune", remote)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("prune failed: %s", string(output))
	}
	return nil
}

// StashEntry represents a stash entry
type StashEntry struct {
	Index   string
	Branch  string
	Message string
}

// GetStashList returns the list of stash entries
func (gc *GitClient) GetStashList(path string) ([]StashEntry, error) {
	cmd := exec.Command("git", "-C", path, "stash", "list")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var stashes []StashEntry
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		// Format: stash@{0}: WIP on main: abc1234 message
		parts := strings.SplitN(line, ":", 3)
		if len(parts) >= 2 {
			index := strings.TrimSpace(parts[0])
			branch := strings.TrimPrefix(strings.TrimSpace(parts[1]), "WIP on ")
			branch = strings.TrimPrefix(branch, "On ")
			message := ""
			if len(parts) >= 3 {
				message = strings.TrimSpace(parts[2])
			}
			stashes = append(stashes, StashEntry{
				Index:   index,
				Branch:  branch,
				Message: message,
			})
		}
	}
	return stashes, nil
}

// CreateStash creates a new stash
func (gc *GitClient) CreateStash(path, message string) error {
	var cmd *exec.Cmd
	if message != "" {
		cmd = exec.Command("git", "-C", path, "stash", "push", "-m", message)
	} else {
		cmd = exec.Command("git", "-C", path, "stash", "push")
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("stash failed: %s", string(output))
	}
	return nil
}

// ApplyStash applies a stash without removing it
func (gc *GitClient) ApplyStash(path, index string) error {
	cmd := exec.Command("git", "-C", path, "stash", "apply", index)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("apply failed: %s", string(output))
	}
	return nil
}

// PopStash applies and removes a stash
func (gc *GitClient) PopStash(path, index string) error {
	cmd := exec.Command("git", "-C", path, "stash", "pop", index)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pop failed: %s", string(output))
	}
	return nil
}

// DropStash removes a stash
func (gc *GitClient) DropStash(path, index string) error {
	cmd := exec.Command("git", "-C", path, "stash", "drop", index)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("drop failed: %s", string(output))
	}
	return nil
}

// ShowStash shows the content of a stash
func (gc *GitClient) ShowStash(path, index string) (string, error) {
	cmd := exec.Command("git", "-C", path, "stash", "show", "-p", index)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// TagInfo represents tag information
type TagInfo struct {
	Name        string
	IsAnnotated bool
	Date        string
	Message     string
}

// GetTagsInfo returns detailed tag information
func (gc *GitClient) GetTagsInfo(path string) ([]TagInfo, error) {
	cmd := exec.Command("git", "-C", path, "tag", "-l", "--format=%(refname:short)|%(objecttype)|%(creatordate:short)|%(subject)")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to simple tag list
		cmd = exec.Command("git", "-C", path, "tag", "-l")
		output, err = cmd.Output()
		if err != nil {
			return nil, err
		}
		var tags []TagInfo
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if line != "" {
				tags = append(tags, TagInfo{Name: line})
			}
		}
		return tags, nil
	}

	var tags []TagInfo
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "|", 4)
		if len(parts) >= 1 {
			tag := TagInfo{Name: parts[0]}
			if len(parts) >= 2 {
				tag.IsAnnotated = parts[1] == "tag"
			}
			if len(parts) >= 3 {
				tag.Date = parts[2]
			}
			if len(parts) >= 4 {
				tag.Message = parts[3]
			}
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

// CreateAnnotatedTag creates an annotated tag
func (gc *GitClient) CreateAnnotatedTag(path, name, message string) error {
	cmd := exec.Command("git", "-C", path, "tag", "-a", name, "-m", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create tag failed: %s", string(output))
	}
	return nil
}

// CreateLightweightTag creates a lightweight tag
func (gc *GitClient) CreateLightweightTag(path, name string) error {
	cmd := exec.Command("git", "-C", path, "tag", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create tag failed: %s", string(output))
	}
	return nil
}

// DeleteTag deletes a tag
func (gc *GitClient) DeleteTag(path, name string) error {
	cmd := exec.Command("git", "-C", path, "tag", "-d", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("delete tag failed: %s", string(output))
	}
	return nil
}

// PushTag pushes a tag to remote
func (gc *GitClient) PushTag(path, name string) error {
	cmd := exec.Command("git", "-C", path, "push", "origin", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("push tag failed: %s", string(output))
	}
	return nil
}
