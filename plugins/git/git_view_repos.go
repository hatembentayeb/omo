package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"omo/pkg/ui"
)

func (gv *GitView) newReposView() *ui.Cores {
	cores := ui.NewCores(gv.app, "Git Repositories")
	cores.SetTableHeaders([]string{"Repository", "Branch", "Status", "Modified", "Staged", "Untracked", "Last Commit"})
	cores.SetRefreshCallback(gv.refreshReposData)
	cores.SetSelectionKey("Repository")

	// Navigation key bindings
	cores.AddKeyBinding("G", "Repos", gv.showRepos)
	cores.AddKeyBinding("S", "Status", gv.showStatus)
	cores.AddKeyBinding("L", "Commits", gv.showCommits)
	cores.AddKeyBinding("B", "Branches", gv.showBranches)
	cores.AddKeyBinding("M", "Remotes", gv.showRemotes)
	cores.AddKeyBinding("T", "Tags", gv.showTags)
	cores.AddKeyBinding("H", "Stash", gv.showStash)

	// Repository action key bindings
	cores.AddKeyBinding("D", "Add Dir", gv.showDirectorySelector)
	cores.AddKeyBinding("R", "Search", gv.showFuzzyRepoSearch)
	cores.AddKeyBinding("F", "Fetch", gv.fetchSelectedRepo)
	cores.AddKeyBinding("P", "Pull", gv.pullSelectedRepo)
	cores.AddKeyBinding("U", "Push", gv.pushSelectedRepo)
	cores.AddKeyBinding("C", "Checkout", gv.checkoutBranch)
	cores.AddKeyBinding("N", "New Branch", gv.createNewBranch)
	cores.AddKeyBinding("?", "Help", gv.showHelp)

	cores.SetActionCallback(gv.handleAction)

	// Set row selection callback
	cores.SetRowSelectedCallback(func(row int) {
		if row >= 0 && row < len(gv.repositories) {
			repo := gv.repositories[row]
			gv.currentRepoPath = repo.Path
			cores.Log(fmt.Sprintf("[blue]Selected: %s (%s)", repo.Name, repo.Branch))
		}
	})

	// Set Enter key to show repo details
	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		gv.showRepoDetails()
	})

	cores.RegisterHandlers()
	return cores
}

func (gv *GitView) refreshReposData() ([][]string, error) {
	rows := make([][]string, len(gv.repositories))

	for i, repo := range gv.repositories {
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

		branch := repo.Branch
		if branch == "" {
			branch = "[gray]...[white]"
		}

		lastCommit := repo.LastCommit
		if lastCommit == "" {
			lastCommit = "[gray]...[white]"
		}

		rows[i] = []string{
			repo.Name,
			branch,
			statusDisplay,
			formatCount(repo.Modified, "yellow"),
			formatCount(repo.Staged, "green"),
			formatCount(repo.Untracked, "red"),
			lastCommit,
		}
	}

	if gv.reposView != nil {
		gv.reposView.SetInfoText(fmt.Sprintf("[green]Git Manager[white]\nRepositories: %d\nSelected: %s",
			len(gv.repositories), gv.getSelectedRepoName()))
	}

	// Update repo info in background
	go gv.updateAllRepoInfo()

	return rows, nil
}

func formatCount(count int, color string) string {
	if count > 0 {
		return fmt.Sprintf("[%s]%d[white]", color, count)
	}
	return "-"
}

func (gv *GitView) getSelectedRepoName() string {
	if gv.reposView == nil {
		return "none"
	}
	row := gv.reposView.GetSelectedRowData()
	if len(row) == 0 {
		return "none"
	}
	return row[0]
}

func (gv *GitView) getSelectedRepo() (*GitRepository, bool) {
	if gv.reposView == nil {
		return nil, false
	}
	row, _ := gv.reposView.GetTable().GetSelection()
	// Row 0 is header, data starts at row 1
	repoIndex := row - 1
	if repoIndex < 0 || repoIndex >= len(gv.repositories) {
		return nil, false
	}
	return &gv.repositories[repoIndex], true
}

func (gv *GitView) updateAllRepoInfo() {
	for i := range gv.repositories {
		gv.updateRepoInfo(&gv.repositories[i])
		time.Sleep(50 * time.Millisecond)
	}

	gv.app.QueueUpdateDraw(func() {
		if gv.reposView != nil {
			gv.reposView.RefreshData()
		}
	})
}

func (gv *GitView) updateRepoInfo(repo *GitRepository) {
	if _, err := os.Stat(repo.Path); os.IsNotExist(err) {
		repo.Status = "not found"
		return
	}

	if branch, err := gv.gitClient.GetCurrentBranch(repo.Path); err == nil {
		repo.Branch = branch
	}

	if modified, staged, untracked, err := gv.gitClient.GetStatus(repo.Path); err == nil {
		repo.Modified = modified
		repo.Staged = staged
		repo.Untracked = untracked
		if modified > 0 || staged > 0 || untracked > 0 {
			repo.Status = "dirty"
		} else {
			repo.Status = "clean"
		}
	}

	if lastCommit, err := gv.gitClient.GetLastCommit(repo.Path); err == nil {
		repo.LastCommit = lastCommit
	}

	repo.lastUpdated = time.Now()
}

func (gv *GitView) fetchSelectedRepo() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.reposView.Log("[yellow]No repository selected")
		return
	}

	gv.reposView.Log(fmt.Sprintf("[yellow]Fetching %s...", repo.Name))

	go func() {
		result, err := gv.gitClient.Fetch(repo.Path)
		gv.app.QueueUpdateDraw(func() {
			if err != nil {
				gv.reposView.Log(fmt.Sprintf("[red]Fetch failed: %v", err))
			} else {
				gv.reposView.Log(fmt.Sprintf("[green]%s: %s", repo.Name, result))
				gv.refresh()
			}
		})
	}()
}

func (gv *GitView) pullSelectedRepo() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.reposView.Log("[yellow]No repository selected")
		return
	}

	gv.reposView.Log(fmt.Sprintf("[yellow]Pulling %s...", repo.Name))

	go func() {
		result, err := gv.gitClient.Pull(repo.Path)
		gv.app.QueueUpdateDraw(func() {
			if err != nil {
				gv.reposView.Log(fmt.Sprintf("[red]Pull failed: %v", err))
			} else {
				gv.reposView.Log(fmt.Sprintf("[green]%s: %s", repo.Name, result))
				gv.refresh()
			}
		})
	}()
}

func (gv *GitView) pushSelectedRepo() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.reposView.Log("[yellow]No repository selected")
		return
	}

	gv.reposView.Log(fmt.Sprintf("[yellow]Pushing %s...", repo.Name))

	go func() {
		result, err := gv.gitClient.Push(repo.Path)
		gv.app.QueueUpdateDraw(func() {
			if err != nil {
				gv.reposView.Log(fmt.Sprintf("[red]Push failed: %v", err))
			} else {
				gv.reposView.Log(fmt.Sprintf("[green]%s: %s", repo.Name, result))
				gv.refresh()
			}
		})
	}()
}

func (gv *GitView) checkoutBranch() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.reposView.Log("[yellow]No repository selected")
		return
	}

	branches, err := gv.gitClient.GetBranchList(repo.Path)
	if err != nil {
		gv.reposView.Log(fmt.Sprintf("[red]Failed to get branches: %v", err))
		return
	}

	items := make([][]string, len(branches))
	for i, branch := range branches {
		items[i] = []string{branch, ""}
	}

	ui.ShowStandardListSelectorModal(
		gv.pages,
		gv.app,
		"Checkout Branch",
		items,
		func(index int, name string, cancelled bool) {
			if !cancelled && index >= 0 && index < len(branches) {
				gv.reposView.Log(fmt.Sprintf("[yellow]Checking out %s...", branches[index]))
				if err := gv.gitClient.Checkout(repo.Path, branches[index]); err != nil {
					gv.reposView.Log(fmt.Sprintf("[red]Checkout failed: %v", err))
				} else {
					gv.reposView.Log(fmt.Sprintf("[green]Checked out %s", branches[index]))
					gv.refresh()
				}
			}
			gv.app.SetFocus(gv.reposView.GetTable())
		},
	)
}

func (gv *GitView) createNewBranch() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		gv.reposView.Log("[yellow]No repository selected")
		return
	}

	ui.ShowCompactStyledInputModal(
		gv.pages,
		gv.app,
		"Create Branch",
		"Branch Name",
		"",
		30,
		nil,
		func(branchName string, cancelled bool) {
			if cancelled || branchName == "" {
				gv.app.SetFocus(gv.reposView.GetTable())
				return
			}

			gv.reposView.Log(fmt.Sprintf("[yellow]Creating branch %s...", branchName))
			if err := gv.gitClient.CreateBranch(repo.Path, branchName); err != nil {
				gv.reposView.Log(fmt.Sprintf("[red]Failed to create branch: %v", err))
			} else {
				gv.reposView.Log(fmt.Sprintf("[green]Created and checked out %s", branchName))
				gv.refresh()
			}
			gv.app.SetFocus(gv.reposView.GetTable())
		},
	)
}

func (gv *GitView) showRepoDetails() {
	repo, ok := gv.getSelectedRepo()
	if !ok {
		return
	}

	var details strings.Builder
	details.WriteString(fmt.Sprintf("[yellow]Repository: %s[white]\n\n", repo.Name))
	details.WriteString(fmt.Sprintf("[green]Path:[white] %s\n", repo.Path))
	details.WriteString(fmt.Sprintf("[green]Branch:[white] %s\n", repo.Branch))
	details.WriteString(fmt.Sprintf("[green]Status:[white] %s\n", repo.Status))
	details.WriteString(fmt.Sprintf("[green]Modified:[white] %d\n", repo.Modified))
	details.WriteString(fmt.Sprintf("[green]Staged:[white] %d\n", repo.Staged))
	details.WriteString(fmt.Sprintf("[green]Untracked:[white] %d\n", repo.Untracked))
	details.WriteString(fmt.Sprintf("[green]Last Commit:[white] %s\n", repo.LastCommit))

	// Get remote info
	if remotes, err := gv.gitClient.GetRemotes(repo.Path); err == nil && len(remotes) > 0 {
		details.WriteString(fmt.Sprintf("\n[yellow]Remotes:[white]\n"))
		for _, remote := range remotes {
			details.WriteString(fmt.Sprintf("  %s\n", remote))
		}
	}

	ui.ShowInfoModal(
		gv.pages,
		gv.app,
		fmt.Sprintf("Repository: %s", repo.Name),
		details.String(),
		func() {
			gv.app.SetFocus(gv.reposView.GetTable())
		},
	)
}

func (gv *GitView) showDirectorySelector() {
	if gv.pages == nil {
		gv.reposView.Log("[red]Pages not initialized")
		return
	}

	homedir, _ := os.UserHomeDir()
	gv.showDirectoryBrowser(homedir)
}

func (gv *GitView) showDirectoryBrowser(startPath string) {
	if gv.pages == nil {
		gv.reposView.Log("[red]Pages not initialized")
		return
	}

	// Get directories from the starting path
	dirs, err := gv.listDirectories(startPath)
	if err != nil {
		gv.reposView.Log(fmt.Sprintf("[red]Error reading directory: %v", err))
		gv.app.SetFocus(gv.reposView.GetTable())
		return
	}

	// If directory is empty (no subdirs), check if it's a git repo
	if len(dirs) == 0 {
		if gv.gitClient.IsRepo(startPath) {
			gv.addSingleRepository(startPath)
		} else {
			gv.reposView.Log(fmt.Sprintf("[yellow]No subdirectories in %s", startPath))
		}
		gv.app.SetFocus(gv.reposView.GetTable())
		return
	}

	// Build items for fuzzy search
	items := make([]ui.FuzzySearchItem, 0, len(dirs)+1)

	// Add parent directory option if not at root
	if startPath != "/" {
		parentDir := filepath.Dir(startPath)
		items = append(items, ui.FuzzySearchItem{
			Name:        "..",
			Description: fmt.Sprintf("Go up to %s", parentDir),
			Data:        map[string]interface{}{"path": parentDir, "action": "navigate"},
		})
	}

	// Add "Select this directory" option
	items = append(items, ui.FuzzySearchItem{
		Name:        fmt.Sprintf("[SELECT] %s", filepath.Base(startPath)),
		Description: fmt.Sprintf("Search for repos in: %s", startPath),
		Data:        map[string]interface{}{"path": startPath, "action": "select"},
	})

	// Add subdirectories
	for _, dir := range dirs {
		dirName := dir.Name()
		fullPath := filepath.Join(startPath, dirName)

		// Check if it's a git repo for display purposes
		isGit := gv.gitClient.IsRepo(fullPath)
		var desc string
		if isGit {
			desc = fmt.Sprintf("[green][git][white] %s", fullPath)
		} else {
			desc = fullPath
		}

		items = append(items, ui.FuzzySearchItem{
			Name:        dirName,
			Description: desc,
			Data:        map[string]interface{}{"path": fullPath, "action": "navigate"},
		})
	}

	title := fmt.Sprintf("Browse: %s", startPath)
	if len(title) > 60 {
		title = fmt.Sprintf("Browse: ...%s", startPath[len(startPath)-50:])
	}

	ui.ShowFuzzySearchModal(
		gv.pages,
		gv.app,
		title,
		items,
		func(index int, item *ui.FuzzySearchItem, cancelled bool) {
			if cancelled || item == nil {
				gv.app.SetFocus(gv.reposView.GetTable())
				return
			}

			// Safely extract data with type checking
			data, ok := item.Data.(map[string]interface{})
			if !ok {
				gv.reposView.Log("[red]Error: invalid item data")
				gv.app.SetFocus(gv.reposView.GetTable())
				return
			}

			path, _ := data["path"].(string)
			action, _ := data["action"].(string)

			if path == "" {
				gv.reposView.Log("[red]Error: no path in item")
				gv.app.SetFocus(gv.reposView.GetTable())
				return
			}

			switch action {
			case "select":
				// User selected to search this directory
				gv.reposView.Log(fmt.Sprintf("[yellow]Searching in %s...", path))
				go gv.discoverRepositories(path)
				gv.app.SetFocus(gv.reposView.GetTable())

			case "navigate":
				// Check if it's a git repo at selection time
				isRepo := gv.gitClient.IsRepo(path)
				if isRepo {
					// It's a git repo - add it and close
					gv.addSingleRepository(path)
					gv.app.SetFocus(gv.reposView.GetTable())
				} else {
					// Not a git repo - navigate deeper into the directory
					gv.showDirectoryBrowser(path)
				}

			default:
				// Unknown action, just return to table
				gv.app.SetFocus(gv.reposView.GetTable())
			}
		},
	)
}

func (gv *GitView) listDirectories(path string) ([]os.DirEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	dirs := make([]os.DirEntry, 0)
	for _, entry := range entries {
		// Skip hidden directories
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if entry.IsDir() {
			dirs = append(dirs, entry)
		}
	}

	return dirs, nil
}

func (gv *GitView) addSingleRepository(repoPath string) {
	repoName := filepath.Base(repoPath)

	// Check if already added
	for _, repo := range gv.repositories {
		if repo.Path == repoPath {
			gv.reposView.Log(fmt.Sprintf("[yellow]%s already in list", repoName))
			return
		}
	}

	// Add repo immediately with minimal info
	repo := GitRepository{
		Name:   repoName,
		Path:   repoPath,
		Status: "loading...",
	}
	gv.repositories = append(gv.repositories, repo)
	gv.reposView.Log(fmt.Sprintf("[green]Added repository: %s", repoName))
	gv.reposView.RefreshData()

	// Update repo info in background
	go func() {
		gv.updateRepoInfo(&gv.repositories[len(gv.repositories)-1])
		gv.app.QueueUpdateDraw(func() {
			gv.reposView.RefreshData()
		})
	}()
}

func (gv *GitView) showFuzzyRepoSearch() {
	if gv.pages == nil {
		gv.reposView.Log("[red]Pages not initialized")
		return
	}

	if len(gv.repositories) == 0 {
		gv.reposView.Log("[yellow]No repositories loaded. Press D to add a directory.")
		return
	}

	// Build items for fuzzy search
	items := make([]ui.FuzzySearchItem, len(gv.repositories))
	for i, repo := range gv.repositories {
		items[i] = ui.FuzzySearchItem{
			Name:        repo.Name,
			Description: fmt.Sprintf("%s | %s | %s", repo.Path, repo.Branch, repo.Status),
			Data:        i,
		}
	}

	ui.ShowFuzzySearchModal(
		gv.pages,
		gv.app,
		"Search Repositories",
		items,
		func(index int, item *ui.FuzzySearchItem, cancelled bool) {
			if cancelled || item == nil {
				gv.app.SetFocus(gv.reposView.GetTable())
				return
			}

			// Select the repository in the table
			repoIndex := item.Data.(int)
			if repoIndex >= 0 && repoIndex < len(gv.repositories) {
				gv.currentRepoPath = gv.repositories[repoIndex].Path
				// Move table selection to this row
				gv.reposView.GetTable().Select(repoIndex+1, 0) // +1 for header
				gv.reposView.Log(fmt.Sprintf("[green]Selected: %s", gv.repositories[repoIndex].Name))
			}
			gv.app.SetFocus(gv.reposView.GetTable())
		},
	)
}

func (gv *GitView) discoverRepositories(searchDir string) {
	repos := []GitRepository{}
	initialDepth := strings.Count(searchDir, string(os.PathSeparator))

	filepath.Walk(searchDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		currentDepth := strings.Count(path, string(os.PathSeparator))
		if currentDepth > initialDepth+3 {
			if info.IsDir() && info.Name() != ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() && info.Name() == ".git" {
			repoPath := filepath.Dir(path)
			repoName := filepath.Base(repoPath)

			if gv.gitClient.IsRepo(repoPath) {
				repo := GitRepository{
					Name: repoName,
					Path: repoPath,
				}
				gv.updateRepoInfo(&repo)
				repos = append(repos, repo)
			}
			return filepath.SkipDir
		}
		return nil
	})

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Name < repos[j].Name
	})

	// Add to existing repos, removing duplicates
	paths := make(map[string]bool)
	for _, repo := range gv.repositories {
		paths[repo.Path] = true
	}

	for _, repo := range repos {
		if !paths[repo.Path] {
			gv.repositories = append(gv.repositories, repo)
			paths[repo.Path] = true
		}
	}

	gv.app.QueueUpdateDraw(func() {
		gv.reposView.Log(fmt.Sprintf("[green]Found %d repositories in %s", len(repos), searchDir))
		gv.reposView.RefreshData()
	})
}
