package git

import "time"

// GitRepository represents a Git repository tracked by the RPC service.
type GitRepository struct {
	Name       string
	Path       string
	Branch     string
	Status     string
	LastCommit string
	Modified   int
	Staged     int
	Untracked  int
	lastUpdated time.Time
}
