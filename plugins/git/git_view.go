package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rivo/tview"

	"omo/ui"
)

// GitRepository represents a Git repository
type GitRepository struct {
	Name        string
	Path        string
	Branch      string
	Status      string
	LastCommit  string
	Modified    int
	Staged      int
	Untracked   int
	lastUpdated time.Time
}

// GitView manages the UI for viewing Git repositories
type GitView struct {
	app             *tview.Application
	pages           *tview.Pages
	cores           *ui.Cores
	errorHandler    *ui.ErrorHandler
	refreshTimer    *time.Timer
	repositories    []GitRepository
	currentRepoPath string
	gitClient       *GitClient
	viewFactory     *ui.ViewFactory
}

// NewGitView creates a new Git view
func NewGitView(app *tview.Application, pages *tview.Pages) *GitView {
	gv := &GitView{
		app:          app,
		pages:        pages,
		repositories: []GitRepository{},
	}

	// Create error handler
	gv.errorHandler = ui.NewErrorHandler(app, pages, func(message string) {
		if gv.cores != nil {
			gv.cores.Log(message)
		}
	})

	// Create view factory
	gv.viewFactory = ui.NewViewFactory(app, pages)

	// Define key handlers using the simple Kafka approach
	keyHandlers := map[string]string{
		"R": "Refresh",
		"G": "Select Repository",
		"F": "Fetch",
		"P": "Pull",
		"S": "Status",
		"L": "Log",
		"B": "Branches",
		"D": "Add Directory",
		"?": "Help",
	}

	// Create Cores UI component using the view factory with just KeyHandlers
	gv.cores = gv.viewFactory.CreateTableView(ui.TableViewConfig{
		Title: "Git Repositories",
		TableHeaders: []string{
			"Repository", "Branch", "Status", "Modified", "Staged", "Untracked", "Last Commit",
		},
		RefreshFunc:    gv.refreshRepositories,
		KeyHandlers:    keyHandlers,
		SelectedFunc:   nil, // Remove the selection handler since we'll use table directly
		AutoRefresh:    false,
		RefreshSeconds: 0,
	})

	// Get the table directly and set up selection handler
	table := gv.cores.GetTable()
	table.SetupSelection()

	// Initialize Git client
	gv.gitClient = NewGitClient()

	// Register additional key handlers using the simple approach like Kafka
	gv.cores.SetActionCallback(func(action string, payload map[string]interface{}) error {
		if action == "keypress" {
			if key, ok := payload["key"].(string); ok {
				switch key {
				case "G":
					gv.ShowRepoSelector()
					return nil
				case "F":
					gv.fetchCurrentRepo()
					return nil
				case "P":
					gv.pullCurrentRepo()
					return nil
				case "S":
					gv.showStatusForCurrentRepo()
					return nil
				case "L":
					gv.showLogForCurrentRepo()
					return nil
				case "B":
					gv.showBranchesForCurrentRepo()
					return nil
				case "D":
					gv.ShowDirectorySelector()
					return nil
				case "?":
					gv.showHelpModal()
					return nil
				}
			}
		}
		return nil
	})

	return gv
}

// GetMainUI returns the main UI component
func (gv *GitView) GetMainUI() tview.Primitive {
	return gv.cores.GetLayout()
}

// showLoadingIndicator displays a loading indicator in the log panel
func (gv *GitView) showLoadingIndicator(message string) {
	gv.cores.Log(fmt.Sprintf("[yellow]⏳ %s...", message))
}

// hideLoadingIndicator removes the loading indicator
func (gv *GitView) hideLoadingIndicator(message string) {
	gv.cores.Log(fmt.Sprintf("[green]✓ %s", message))
}

// AutoDiscoverRepositories finds Git repositories in common directories
func (gv *GitView) AutoDiscoverRepositories() {
	gv.showLoadingIndicator("Searching for Git repositories")

	// Common paths where Git repositories might be found
	homedir, _ := os.UserHomeDir()
	searchPaths := []string{
		homedir + "/go/src",
		homedir + "/projects",
		homedir + "/work",
		homedir + "/Documents/projects",
	}

	repos := []GitRepository{}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			gv.showLoadingIndicator(fmt.Sprintf("Searching in %s", path))

			// Find all .git directories recursively (up to depth 3)
			err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil // Skip errors
				}

				// Check depth to avoid going too deep
				if strings.Count(path, string(os.PathSeparator)) > strings.Count(searchPaths[0], string(os.PathSeparator))+3 {
					if info.IsDir() && info.Name() != ".git" {
						return filepath.SkipDir
					}
					return nil
				}

				// Found a .git directory
				if info.IsDir() && info.Name() == ".git" {
					repoPath := filepath.Dir(path)
					repoName := filepath.Base(repoPath)

					// Only add if it's a valid repository
					if gv.gitClient.IsRepo(repoPath) {
						// Create repo entry
						repos = append(repos, GitRepository{
							Name: repoName,
							Path: repoPath,
						})
					}

					return filepath.SkipDir
				}
				return nil
			})

			// Sort repositories by name
			sort.Slice(repos, func(i, j int) bool {
				return repos[i].Name < repos[j].Name
			})

			// Handle error if any
			if err != nil {
				gv.errorHandler.HandleError(err, ui.ErrorLevelWarning, "Repository Search Error")
			}

			// Update the repositories list by appending the new ones
			gv.repositories = append(gv.repositories, repos...)

			// Remove duplicates by checking paths
			uniqueRepos := make([]GitRepository, 0, len(gv.repositories))
			paths := make(map[string]bool)

			for _, repo := range gv.repositories {
				if _, exists := paths[repo.Path]; !exists {
					paths[repo.Path] = true
					uniqueRepos = append(uniqueRepos, repo)
				}
			}

			gv.repositories = uniqueRepos

			// Show results message
			gv.cores.Log(fmt.Sprintf("[green]✓ Found %d Git repositories in %s", len(repos), path))

			// Refresh display to show the repository names with their information
			gv.cores.RefreshData()

			// Return focus to the table
			gv.app.SetFocus(gv.cores.GetTable())

			gv.hideLoadingIndicator(fmt.Sprintf("Found %d Git repositories", len(repos)))
		}
	}
}

// ShowRepoSelector shows a modal for selecting a repository
func (gv *GitView) ShowRepoSelector() {
	// Check if we have any repositories
	if len(gv.repositories) == 0 {
		// Show a message if no repositories are found
		gv.cores.Log("[yellow]No Git repositories found. Press 'D' to search for repositories in a directory.")
		return
	}

	// Remove any existing repo selector modal
	const modalName = "git-repo-selector"
	if gv.pages.HasPage(modalName) {
		gv.pages.RemovePage(modalName)
	}

	// Create data for the selection list
	items := make([][]string, len(gv.repositories))
	for i, repo := range gv.repositories {
		items[i] = []string{repo.Name, repo.Path}
	}

	// Show a list selector modal
	ui.ShowStandardListSelectorModal(
		gv.pages,
		gv.app,
		"Select Git Repository",
		items,
		func(index int, repoPath string, cancelled bool) {
			// Ensure table regains focus
			gv.app.SetFocus(gv.cores.GetTable())

			// Clean up modal
			if gv.pages.HasPage(modalName) {
				gv.pages.RemovePage(modalName)
			}

			if !cancelled && index >= 0 && index < len(gv.repositories) {
				// Set the current repository
				gv.currentRepoPath = gv.repositories[index].Path
				gv.cores.Log(fmt.Sprintf("[blue]Selected repository: %s", gv.repositories[index].Name))
				// Refresh data directly
				time.Sleep(50 * time.Millisecond)
				gv.cores.RefreshData()
			}
		},
	)
}

// refreshRepositories refreshes the Git repository data
func (gv *GitView) refreshRepositories() ([][]string, error) {
	// Create an initial result with existing data
	result := make([][]string, len(gv.repositories))
	for i, repo := range gv.repositories {
		// Format status for display in the table
		statusDisplay := repo.Status
		if statusDisplay == "clean" {
			statusDisplay = "[green]Clean[white]"
		} else if statusDisplay == "dirty" {
			statusDisplay = "[yellow]Modified[white]"
		} else if statusDisplay == "ahead" {
			statusDisplay = "[cyan]Ahead[white]"
		} else if statusDisplay == "behind" {
			statusDisplay = "[red]Behind[white]"
		} else if statusDisplay == "" {
			statusDisplay = "[gray]loading...[white]"
		}

		// Use existing data for display
		result[i] = []string{
			repo.Name,
			func() string {
				if repo.Branch == "" {
					return "[gray]loading...[white]"
				}
				return repo.Branch
			}(),
			statusDisplay,
			func() string {
				if repo.Modified > 0 {
					return fmt.Sprintf("[yellow]%d[white]", repo.Modified)
				}
				return "-"
			}(),
			func() string {
				if repo.Staged > 0 {
					return fmt.Sprintf("[green]%d[white]", repo.Staged)
				}
				return "-"
			}(),
			func() string {
				if repo.Untracked > 0 {
					return fmt.Sprintf("[red]%d[white]", repo.Untracked)
				}
				return "-"
			}(),
			func() string {
				if repo.LastCommit == "" {
					return "[gray]loading...[white]"
				}
				return repo.LastCommit
			}(),
		}
	}

	// Stop any existing refresh timer to prevent leaks
	if gv.refreshTimer != nil {
		gv.refreshTimer.Stop()
	}

	// Update repository information in the background with a timer
	// Using a timer instead of a direct goroutine gives us better control
	gv.refreshTimer = time.AfterFunc(100*time.Millisecond, func() {
		// Make a local copy of repositories to avoid race conditions
		reposCopy := make([]GitRepository, len(gv.repositories))
		copy(reposCopy, gv.repositories)

		// Process each repository
		for _, repo := range reposCopy {
			// Find the matching repository in the current list
			for j := range gv.repositories {
				if gv.repositories[j].Path == repo.Path {
					// Update repo in the actual list
					gv.updateRepoInfo(&gv.repositories[j])
					break
				}
			}

			// Add a small delay between updates to prevent CPU spikes
			time.Sleep(50 * time.Millisecond)
		}

		// Queue a UI refresh after all updates if we still have repositories
		if len(gv.repositories) > 0 {
			gv.app.QueueUpdateDraw(func() {
				if gv.cores != nil {
					gv.cores.Log("[green]Repository information updated")
				}
			})
		}
	})

	return result, nil
}

// updateRepoInfo updates the information for a Git repository
func (gv *GitView) updateRepoInfo(repo *GitRepository) {
	// Skip if repo doesn't exist
	if _, err := os.Stat(repo.Path); os.IsNotExist(err) {
		repo.Status = "not found"
		repo.lastUpdated = time.Now() // Update timestamp
		return
	}

	// Get branch
	branch, err := gv.gitClient.GetCurrentBranch(repo.Path)
	if err == nil {
		repo.Branch = branch
	} else {
		repo.Branch = "unknown"
	}

	// Get status - ensure we get the actual numbers
	modified, staged, untracked, err := gv.gitClient.GetStatus(repo.Path)
	if err == nil {
		repo.Modified = modified
		repo.Staged = staged
		repo.Untracked = untracked

		if modified > 0 || staged > 0 || untracked > 0 {
			repo.Status = "dirty"
		} else {
			repo.Status = "clean"
		}
	} else {
		repo.Status = "error"
		repo.Modified = 0
		repo.Staged = 0
		repo.Untracked = 0
	}

	// Get last commit
	lastCommit, err := gv.gitClient.GetLastCommit(repo.Path)
	if err == nil {
		repo.LastCommit = lastCommit
	} else {
		repo.LastCommit = "unknown"
	}

	// Update lastUpdated timestamp
	repo.lastUpdated = time.Now()
}

// getSelectedRepo returns the currently selected repository
func (gv *GitView) getSelectedRepo() (*GitRepository, error) {
	row := gv.cores.GetTable().GetSelectedRow()
	if row < 0 || row >= len(gv.repositories) {
		return nil, fmt.Errorf("no repository selected")
	}
	return &gv.repositories[row], nil
}

// checkRepoSelected checks if a repository is selected and exists
func (gv *GitView) checkRepoSelected() error {
	repo, err := gv.getSelectedRepo()
	if err != nil {
		return err
	}

	if !gv.gitClient.IsGitRepository(repo.Path) {
		return fmt.Errorf("selected path is not a git repository: %s", repo.Path)
	}
	return nil
}

// fetchCurrentRepo fetches the current repository
func (gv *GitView) fetchCurrentRepo() {
	if err := gv.checkRepoSelected(); err != nil {
		gv.cores.Log(fmt.Sprintf("[yellow]%v", err))
		return
	}
	repo, _ := gv.getSelectedRepo() // Error already checked in checkRepoSelected
	gv.gitClient.Fetch(repo.Path)
	gv.refreshRepositories()
}

// pullCurrentRepo pulls the current repository
func (gv *GitView) pullCurrentRepo() {
	if err := gv.checkRepoSelected(); err != nil {
		gv.cores.Log(fmt.Sprintf("[yellow]%v", err))
		return
	}
	repo, _ := gv.getSelectedRepo() // Error already checked in checkRepoSelected
	gv.gitClient.Pull(repo.Path)
	gv.refreshRepositories()
}

// showStatusForCurrentRepo shows the status of the current repository
func (gv *GitView) showStatusForCurrentRepo() {
	if err := gv.checkRepoSelected(); err != nil {
		gv.cores.Log(fmt.Sprintf("[yellow]%v", err))
		return
	}
	repo, _ := gv.getSelectedRepo() // Error already checked in checkRepoSelected
	status := gv.gitClient.Status(repo.Path)
	gv.showStatusModal(status)
}

// showLogForCurrentRepo shows the log of the current repository
func (gv *GitView) showLogForCurrentRepo() {
	if err := gv.checkRepoSelected(); err != nil {
		gv.cores.Log(fmt.Sprintf("[yellow]%v", err))
		return
	}
	repo, _ := gv.getSelectedRepo() // Error already checked in checkRepoSelected
	log := gv.gitClient.Log(repo.Path)
	gv.showLogModal(log)
}

// showBranchesForCurrentRepo shows the branches of the current repository
func (gv *GitView) showBranchesForCurrentRepo() {
	if err := gv.checkRepoSelected(); err != nil {
		gv.cores.Log(fmt.Sprintf("[yellow]%v", err))
		return
	}
	repo, _ := gv.getSelectedRepo() // Error already checked in checkRepoSelected
	branches := gv.gitClient.Branches(repo.Path)
	gv.showBranchesModal(branches)
}

// showHelpModal shows the help information
func (gv *GitView) showHelpModal() {
	helpText := `[yellow]Git Repository Manager Help[white]

[green]Key Bindings:[white]
  [yellow]D[white]   - Add repositories from a directory
  [yellow]R[white]   - Refresh repositories data
  [yellow]G[white]   - Select repository from list
  [yellow]F[white]   - Fetch updates from remote
  [yellow]P[white]   - Pull changes from remote
  [yellow]S[white]   - Show detailed status
  [yellow]L[white]   - Show commit log
  [yellow]B[white]   - Show branches
  [yellow]?[white]   - Show this help
  [yellow]ESC[white] - Close modals/Navigate back

[green]Tips:[white]
• Select a repository before using F, P, S, L, or B commands
• Use D to scan new directories for Git repositories
• Press R to refresh repository status
• Use G to quickly switch between repositories

[green]Status Colors:[white]
  [green]Clean[white]    - No changes
  [yellow]Modified[white]  - Has uncommitted changes
  [cyan]Ahead[white]     - Local commits not pushed
  [red]Behind[white]    - Remote changes not pulled`

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		"Help",
		helpText,
		func() {
			gv.app.SetFocus(gv.cores.GetTable())
		},
	)
}

// showStatusModal displays the repository status in a modal
func (gv *GitView) showStatusModal(status string) {
	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		"Repository Status",
		status,
		func() {
			gv.app.SetFocus(gv.cores.GetTable())
		},
	)
}

// showLogModal displays the commit log in a modal
func (gv *GitView) showLogModal(log string) {
	// Color the commit log output
	coloredLogs := strings.Builder{}
	for _, line := range strings.Split(log, "\n") {
		if line == "" {
			continue
		}
		// Extract commit hash (first 7 chars) and color it yellow
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			hash := parts[0]
			rest := parts[1]
			coloredLogs.WriteString(fmt.Sprintf("[yellow]%s[white] %s\n", hash, rest))
		} else {
			coloredLogs.WriteString(line + "\n")
		}
	}

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		"Commit Log",
		coloredLogs.String(),
		func() {
			gv.app.SetFocus(gv.cores.GetTable())
		},
	)
}

// showBranchesModal displays the branches in a modal
func (gv *GitView) showBranchesModal(branches string) {
	// Color the current branch
	coloredBranches := strings.Builder{}
	for _, line := range strings.Split(branches, "\n") {
		if strings.HasPrefix(line, "*") {
			coloredBranches.WriteString(fmt.Sprintf("[green]%s[white]\n", line))
		} else {
			coloredBranches.WriteString(line + "\n")
		}
	}

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		"Branches",
		coloredBranches.String(),
		func() {
			gv.app.SetFocus(gv.cores.GetTable())
		},
	)
}

// Manual repository discovery functions

// ManualDiscoverRepositories finds Git repositories in a specific directory
func (gv *GitView) ManualDiscoverRepositories(searchDir string) {
	// Validate directory exists
	if _, err := os.Stat(searchDir); err != nil {
		gv.errorHandler.HandleError(err, ui.ErrorLevelError, "Directory Not Found")
		return
	}

	// Show startup message in the log
	gv.cores.Log(fmt.Sprintf("[yellow]⏳ Searching for Git repositories in %s...", searchDir))

	// Return immediately to keep UI responsive
	go func() {
		repos := []GitRepository{}
		repoCount := 0

		// Set up a counter to periodically update the log with progress
		updateCounter := 0
		lastLogTime := time.Now()

		// Find all .git directories recursively (up to depth 3)
		initialDepth := strings.Count(searchDir, string(os.PathSeparator))
		err := filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			// Update progress periodically in log - but only if enough time has passed
			// to avoid flooding the UI with updates
			updateCounter++
			if updateCounter%1000 == 0 && time.Since(lastLogTime) > 500*time.Millisecond {
				// Use a simple direct log update instead of QueueUpdateDraw
				if repoCount > 0 {
					gv.cores.Log(fmt.Sprintf("[yellow]⏳ Still searching... Found %d repositories so far", repoCount))
				} else {
					gv.cores.Log(fmt.Sprintf("[yellow]⏳ Searching in %s...", searchDir))
				}
				lastLogTime = time.Now()
			}

			// Check depth to avoid going too deep
			currentDepth := strings.Count(path, string(os.PathSeparator))
			if currentDepth > initialDepth+3 {
				if info.IsDir() && info.Name() != ".git" {
					return filepath.SkipDir
				}
				return nil
			}

			// Found a .git directory
			if info.IsDir() && info.Name() == ".git" {
				repoPath := filepath.Dir(path)
				repoName := filepath.Base(repoPath)

				// Only add if it's a valid repository
				if gv.gitClient.IsRepo(repoPath) {
					// Create repo entry and immediately update its information
					repo := GitRepository{
						Name: repoName,
						Path: repoPath,
					}

					// Update repo info immediately
					gv.updateRepoInfo(&repo)

					// Add to repos list
					repos = append(repos, repo)
					repoCount++

					// Log when we find repositories but not too frequently to avoid UI blocking
					if repoCount%10 == 0 && time.Since(lastLogTime) > 1*time.Second {
						gv.cores.Log(fmt.Sprintf("[yellow]⏳ Found %d repositories so far", repoCount))
						lastLogTime = time.Now()
					}
				}

				return filepath.SkipDir
			}
			return nil
		})

		// Sort repositories by name
		sort.Slice(repos, func(i, j int) bool {
			return repos[i].Name < repos[j].Name
		})

		// Handle error if any
		if err != nil {
			gv.errorHandler.HandleError(err, ui.ErrorLevelWarning, "Repository Search Error")
		}

		// Update the repositories list by appending the new ones
		gv.repositories = append(gv.repositories, repos...)

		// Remove duplicates by checking paths
		uniqueRepos := make([]GitRepository, 0, len(gv.repositories))
		paths := make(map[string]bool)

		for _, repo := range gv.repositories {
			if _, exists := paths[repo.Path]; !exists {
				paths[repo.Path] = true
				uniqueRepos = append(uniqueRepos, repo)
			}
		}

		gv.repositories = uniqueRepos

		// Show results message
		gv.cores.Log(fmt.Sprintf("[green]✓ Found %d Git repositories in %s", len(repos), searchDir))

		// Refresh display to show the repository names with their information
		gv.cores.RefreshData()

		// Return focus to the table
		gv.app.SetFocus(gv.cores.GetTable())
	}()
}

// ShowDirectorySelector displays a modal for entering a directory to search
func (gv *GitView) ShowDirectorySelector() {
	// Get user's home directory as default
	homedir, _ := os.UserHomeDir()

	// Remove any existing modal pages to avoid stacking
	const modalName = "git-directory-selector"
	if gv.pages.HasPage(modalName) {
		gv.pages.RemovePage(modalName)
	}

	// Use the compact input modal instead of the standard directory selector
	ui.ShowCompactStyledInputModal(
		gv.pages,
		gv.app,
		"Search Git Repositories",
		"Directory: ",
		homedir,
		50,  // Input field width
		nil, // No validator
		func(directory string, cancelled bool) {
			// Ensure table regains focus
			gv.app.SetFocus(gv.cores.GetTable())

			// Make sure the modal is removed
			if gv.pages.HasPage(modalName) {
				gv.pages.RemovePage(modalName)
			}

			if !cancelled && directory != "" {
				// Expand tilde if present
				if strings.HasPrefix(directory, "~") {
					directory = strings.Replace(directory, "~", homedir, 1)
				}

				// Clean the path to normalize it
				directory = filepath.Clean(directory)

				// Check if directory exists before proceeding
				if stat, err := os.Stat(directory); err != nil || !stat.IsDir() {
					gv.cores.Log(fmt.Sprintf("[red]Error: %s is not a valid directory", directory))
					return
				}

				// Search for repositories in the selected directory
				gv.cores.Log(fmt.Sprintf("[blue]Searching for repositories in: %s", directory))
				gv.ManualDiscoverRepositories(directory)
			}
		},
	)
}
