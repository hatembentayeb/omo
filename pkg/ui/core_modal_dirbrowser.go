package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// DirBrowserAction represents what the user wants to do with the selected directory
type DirBrowserAction int

const (
	DirBrowserAddRepo   DirBrowserAction = iota // Add the selected path as a single git repo
	DirBrowserScanRepos                         // Recursively scan the directory for repos
	DirBrowserCancel                            // User cancelled
)

// DirBrowserResult contains the result of the directory browser interaction
type DirBrowserResult struct {
	Path   string
	Action DirBrowserAction
}

// ShowDirectoryBrowserModal displays a dedicated directory browser for navigating
// the filesystem and selecting directories to add as git repositories.
//
// Unlike the fuzzy search modal, this browser:
//   - Navigates in-place (no modal stacking)
//   - Has a dedicated path display
//   - Separates browsing (Enter) from adding (Ctrl+A) and scanning (Ctrl+S)
//   - Supports going up with Backspace when the filter is empty
//   - Supports typing a full path (starting with / or ~) and pressing Enter to jump there
func ShowDirectoryBrowserModal(
	pages *tview.Pages,
	app *tview.Application,
	startPath string,
	isGitRepo func(path string) bool,
	callback func(result DirBrowserResult),
) {
	const pageID = "dir-browser-modal"

	currentPath := startPath

	// -- UI Components --

	container := tview.NewFlex()
	container.SetDirection(tview.FlexRow)
	container.SetBorder(true)
	container.SetTitle(" Add Directory ")
	container.SetTitleAlign(tview.AlignCenter)
	container.SetBorderColor(tcell.ColorAqua)
	container.SetTitleColor(tcell.ColorOrange)
	container.SetBackgroundColor(tcell.ColorDefault)
	container.SetBorderPadding(0, 0, 1, 1)

	// Path display bar
	pathView := tview.NewTextView()
	pathView.SetDynamicColors(true)
	pathView.SetBackgroundColor(tcell.ColorDefault)
	pathView.SetTextColor(tcell.ColorWhite)

	// Hint bar (shows contextual tips, e.g. when inside a git repo)
	hintView := tview.NewTextView()
	hintView.SetDynamicColors(true)
	hintView.SetBackgroundColor(tcell.ColorDefault)

	currentIsGitRepo := false

	updatePathDisplay := func(path string, isRepo bool) {
		display := path
		if len(display) > 60 {
			display = "..." + display[len(display)-57:]
		}
		if isRepo {
			pathView.SetText(fmt.Sprintf(" [green]>[white] %s [green][git repo][white]", display))
			hintView.SetText(" [green]>>> Press Enter to add this repo, or keep browsing below")
		} else {
			pathView.SetText(fmt.Sprintf(" [yellow]>[white] %s", display))
			hintView.SetText("")
		}
	}
	updatePathDisplay(currentPath, false)

	// Filter input
	filterInput := tview.NewInputField()
	filterInput.SetLabel(" / ")
	filterInput.SetLabelColor(tcell.ColorAqua)
	filterInput.SetFieldBackgroundColor(tcell.ColorDefault)
	filterInput.SetFieldTextColor(tcell.ColorWhite)
	filterInput.SetPlaceholder("Filter or type a path...")
	filterInput.SetPlaceholderTextColor(tcell.ColorGray)

	// Directory listing
	list := tview.NewList()
	list.SetBackgroundColor(tcell.ColorDefault)
	list.SetMainTextColor(tcell.ColorWhite)
	list.SetSecondaryTextColor(tcell.ColorGray)
	list.SetSelectedTextColor(tcell.ColorBlack)
	list.SetSelectedBackgroundColor(tcell.ColorAqua)
	list.SetHighlightFullLine(true)
	list.ShowSecondaryText(true)

	// -- Data --

	type dirEntry struct {
		name     string
		fullPath string
		isParent bool
		isGit    bool
	}

	var allEntries []dirEntry
	var filteredIndices []int

	// Load entries for a directory
	loadDirectory := func(path string) error {
		allEntries = allEntries[:0]

		// Check if the current directory itself is a git repo
		currentIsGitRepo = false
		if isGitRepo != nil {
			currentIsGitRepo = isGitRepo(path)
		}

		// Parent directory option
		if path != "/" {
			allEntries = append(allEntries, dirEntry{
				name:     "..",
				fullPath: filepath.Dir(path),
				isParent: true,
			})
		}

		// If this is a git repo, add a prominent "add this repo" entry at the top
		if currentIsGitRepo {
			allEntries = append(allEntries, dirEntry{
				name:     fmt.Sprintf("[bold]%s[white]", filepath.Base(path)),
				fullPath: path,
				isGit:    true,
				isParent: false,
			})
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if !entry.IsDir() {
				continue
			}
			fullPath := filepath.Join(path, entry.Name())
			isGit := false
			if isGitRepo != nil {
				isGit = isGitRepo(fullPath)
			}
			allEntries = append(allEntries, dirEntry{
				name:     entry.Name(),
				fullPath: fullPath,
				isGit:    isGit,
			})
		}

		currentPath = path
		updatePathDisplay(path, currentIsGitRepo)
		return nil
	}

	// Update the list widget from allEntries, applying the filter
	updateList := func(query string) {
		list.Clear()
		filteredIndices = filteredIndices[:0]

		query = strings.ToLower(strings.TrimSpace(query))

		for i, entry := range allEntries {
			// Always show ".." regardless of filter
			if !entry.isParent && query != "" {
				if !fuzzyMatch(strings.ToLower(entry.name), query) {
					continue
				}
			}

			var mainText, descText string
			if entry.isParent {
				mainText = "[yellow]..[white]"
				descText = fmt.Sprintf("  %s", entry.fullPath)
			} else if entry.isGit && entry.fullPath == currentPath {
				// This is the "add current repo" entry
				mainText = fmt.Sprintf("[green][bold]>>> Add %s[white]", filepath.Base(entry.fullPath))
				descText = fmt.Sprintf("  [green]git repository at %s[white]", entry.fullPath)
			} else if entry.isGit {
				mainText = fmt.Sprintf("[green]%s[white] [aqua][git][white]", entry.name)
				descText = fmt.Sprintf("  %s", entry.fullPath)
			} else {
				mainText = entry.name
				descText = fmt.Sprintf("  %s", entry.fullPath)
			}

			list.AddItem(mainText, descText, 0, nil)
			filteredIndices = append(filteredIndices, i)
		}

		if list.GetItemCount() > 0 {
			list.SetCurrentItem(0)
		}
	}

	// Navigate to a directory (updates everything in-place)
	navigateTo := func(path string) {
		if err := loadDirectory(path); err != nil {
			// If we can't read the directory, stay where we are
			return
		}
		filterInput.SetText("")
		updateList("")
	}

	// Initialize with the start path
	navigateTo(startPath)

	// -- Helpers --

	closeModal := func(result DirBrowserResult) {
		pages.RemovePage(pageID)
		if callback != nil {
			callback(result)
		}
	}

	getSelectedEntry := func() *dirEntry {
		idx := list.GetCurrentItem()
		if idx < 0 || idx >= len(filteredIndices) {
			return nil
		}
		origIdx := filteredIndices[idx]
		if origIdx < 0 || origIdx >= len(allEntries) {
			return nil
		}
		return &allEntries[origIdx]
	}

	// Check if text looks like a filesystem path
	looksLikePath := func(text string) bool {
		return strings.HasPrefix(text, "/") || strings.HasPrefix(text, "~/")
	}

	// Resolve ~ in path
	resolvePath := func(path string) string {
		if strings.HasPrefix(path, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				return filepath.Join(home, path[2:])
			}
		}
		return path
	}

	// Navigate into the selected entry
	enterSelected := func() {
		// If the filter text looks like a path, navigate there instead
		filterText := filterInput.GetText()
		if looksLikePath(filterText) {
			resolved := resolvePath(filterText)
			if info, err := os.Stat(resolved); err == nil && info.IsDir() {
				navigateTo(resolved)
				return
			}
			// Try as partial path - find closest existing parent
			for resolved != "/" && resolved != "." {
				parent := filepath.Dir(resolved)
				if info, err := os.Stat(parent); err == nil && info.IsDir() {
					navigateTo(parent)
					return
				}
				resolved = parent
			}
			return
		}

		entry := getSelectedEntry()
		if entry == nil {
			return
		}

		// If this is the "add current repo" entry, add it directly
		if entry.isGit && entry.fullPath == currentPath {
			closeModal(DirBrowserResult{Path: entry.fullPath, Action: DirBrowserAddRepo})
			return
		}

		navigateTo(entry.fullPath)
	}

	addSelected := func() {
		entry := getSelectedEntry()
		if entry != nil && !entry.isParent {
			closeModal(DirBrowserResult{Path: entry.fullPath, Action: DirBrowserAddRepo})
		} else {
			// No entry or parent selected: add the current directory itself
			closeModal(DirBrowserResult{Path: currentPath, Action: DirBrowserAddRepo})
		}
	}

	scanCurrent := func() {
		closeModal(DirBrowserResult{Path: currentPath, Action: DirBrowserScanRepos})
	}

	goUp := func() {
		if currentPath != "/" {
			navigateTo(filepath.Dir(currentPath))
		}
	}

	// -- Key Bindings --

	// Filter input: real-time filtering
	filterInput.SetChangedFunc(func(text string) {
		// Only filter if it doesn't look like a path being typed
		if !looksLikePath(text) {
			updateList(text)
		}
	})

	// Filter input: special keys
	filterInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if filterInput.GetText() == "" {
				goUp()
				return nil
			}
		case tcell.KeyCtrlA:
			addSelected()
			return nil
		case tcell.KeyCtrlS:
			scanCurrent()
			return nil
		case tcell.KeyDown:
			app.SetFocus(list)
			return nil
		case tcell.KeyUp:
			// Move list selection up while keeping focus on input
			current := list.GetCurrentItem()
			if current > 0 {
				list.SetCurrentItem(current - 1)
			}
			return nil
		}
		return event
	})

	filterInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			enterSelected()
		case tcell.KeyEscape:
			closeModal(DirBrowserResult{Action: DirBrowserCancel})
		case tcell.KeyTab:
			app.SetFocus(list)
		}
	})

	// List: navigation & actions
	list.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		enterSelected()
		app.SetFocus(filterInput)
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			closeModal(DirBrowserResult{Action: DirBrowserCancel})
			return nil
		case tcell.KeyCtrlA:
			addSelected()
			return nil
		case tcell.KeyCtrlS:
			scanCurrent()
			return nil
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			goUp()
			app.SetFocus(filterInput)
			return nil
		case tcell.KeyRune:
			// Typing while in list -> switch to filter
			app.SetFocus(filterInput)
			return event
		}
		return event
	})

	// -- Info Bar --

	infoText := tview.NewTextView()
	infoText.SetDynamicColors(true)
	infoText.SetBackgroundColor(tcell.ColorDefault)
	infoText.SetTextAlign(tview.AlignCenter)
	infoText.SetText("[yellow]Enter[gray] Open  [yellow]^A[gray] Add Repo  [yellow]^S[gray] Scan All  [yellow]Bksp[gray] Up  [yellow]Esc[gray] Close")

	// Separator line
	sep := tview.NewTextView()
	sep.SetBackgroundColor(tcell.ColorDefault)
	sep.SetTextColor(tcell.ColorDarkGray)
	sep.SetTextAlign(tview.AlignLeft)
	sep.SetText(strings.Repeat("─", 76))

	// -- Layout --

	container.AddItem(pathView, 1, 0, false)
	container.AddItem(hintView, 1, 0, false)
	container.AddItem(filterInput, 1, 0, true)
	container.AddItem(sep, 1, 0, false)
	container.AddItem(list, 0, 1, false)
	container.AddItem(infoText, 1, 0, false)

	width := 80
	height := 25

	innerFlex := tview.NewFlex()
	innerFlex.SetDirection(tview.FlexRow)
	innerFlex.SetBackgroundColor(tcell.ColorDefault)
	innerFlex.AddItem(nil, 0, 1, false)
	innerFlex.AddItem(container, height, 0, true)
	innerFlex.AddItem(nil, 0, 1, false)

	flex := tview.NewFlex()
	flex.SetBackgroundColor(tcell.ColorDefault)
	flex.AddItem(nil, 0, 1, false)
	flex.AddItem(innerFlex, width, 0, true)
	flex.AddItem(nil, 0, 1, false)

	pages.AddPage(pageID, flex, true, true)
	app.SetFocus(filterInput)
}
