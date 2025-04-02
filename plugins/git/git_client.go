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
		statusSymbol := ""

		if fileStatus.Staging != git.Unmodified {
			switch fileStatus.Staging {
			case git.Added:
				statusSymbol += "A"
			case git.Modified:
				statusSymbol += "M"
			case git.Deleted:
				statusSymbol += "D"
			case git.Renamed:
				statusSymbol += "R"
			case git.Copied:
				statusSymbol += "C"
			}
		} else {
			statusSymbol += " "
		}

		if fileStatus.Worktree != git.Unmodified {
			switch fileStatus.Worktree {
			case git.Added, git.Untracked:
				statusSymbol += "?"
			case git.Modified:
				statusSymbol += "M"
			case git.Deleted:
				statusSymbol += "D"
			case git.Renamed:
				statusSymbol += "R"
			case git.Copied:
				statusSymbol += "C"
			}
		} else {
			statusSymbol += " "
		}

		result.WriteString(fmt.Sprintf("%s %s\n", statusSymbol, file))
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
