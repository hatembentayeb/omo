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

type dirEntry struct {
	name     string
	fullPath string
	isParent bool
	isGit    bool
}

type dirBrowser struct {
	pages           *tview.Pages
	app             *tview.Application
	isGitRepo       func(path string) bool
	callback        func(result DirBrowserResult)
	currentPath     string
	currentIsGitRepo bool
	allEntries      []dirEntry
	filteredIndices []int
	pathView        *tview.TextView
	hintView        *tview.TextView
	filterInput     *tview.InputField
	list            *tview.List
}

func (db *dirBrowser) updatePathDisplay() {
	display := db.currentPath
	if len(display) > 60 {
		display = "..." + display[len(display)-57:]
	}
	if db.currentIsGitRepo {
		db.pathView.SetText(fmt.Sprintf(" [green]>[white] %s [green][git repo][white]", display))
		db.hintView.SetText(" [green]>>> Press Enter to add this repo, or keep browsing below")
	} else {
		db.pathView.SetText(fmt.Sprintf(" [yellow]>[white] %s", display))
		db.hintView.SetText("")
	}
}

func (db *dirBrowser) loadDirectory(path string) error {
	db.allEntries = db.allEntries[:0]

	db.currentIsGitRepo = false
	if db.isGitRepo != nil {
		db.currentIsGitRepo = db.isGitRepo(path)
	}

	if path != "/" {
		db.allEntries = append(db.allEntries, dirEntry{
			name:     "..",
			fullPath: filepath.Dir(path),
			isParent: true,
		})
	}

	if db.currentIsGitRepo {
		db.allEntries = append(db.allEntries, dirEntry{
			name:     fmt.Sprintf("[bold]%s[white]", filepath.Base(path)),
			fullPath: path,
			isGit:    true,
		})
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") || !entry.IsDir() {
			continue
		}
		fullPath := filepath.Join(path, entry.Name())
		isGit := false
		if db.isGitRepo != nil {
			isGit = db.isGitRepo(fullPath)
		}
		db.allEntries = append(db.allEntries, dirEntry{
			name:     entry.Name(),
			fullPath: fullPath,
			isGit:    isGit,
		})
	}

	db.currentPath = path
	db.updatePathDisplay()
	return nil
}

func (db *dirBrowser) formatEntryText(entry dirEntry) (string, string) {
	if entry.isParent {
		return "[yellow]..[white]", fmt.Sprintf("  %s", entry.fullPath)
	}
	if entry.isGit && entry.fullPath == db.currentPath {
		return fmt.Sprintf("[green][bold]>>> Add %s[white]", filepath.Base(entry.fullPath)),
			fmt.Sprintf("  [green]git repository at %s[white]", entry.fullPath)
	}
	if entry.isGit {
		return fmt.Sprintf("[green]%s[white] [aqua][git][white]", entry.name),
			fmt.Sprintf("  %s", entry.fullPath)
	}
	return entry.name, fmt.Sprintf("  %s", entry.fullPath)
}

func (db *dirBrowser) updateList(query string) {
	db.list.Clear()
	db.filteredIndices = db.filteredIndices[:0]
	query = strings.ToLower(strings.TrimSpace(query))

	for i, entry := range db.allEntries {
		if !entry.isParent && query != "" {
			if !fuzzyMatch(strings.ToLower(entry.name), query) {
				continue
			}
		}
		mainText, descText := db.formatEntryText(entry)
		db.list.AddItem(mainText, descText, 0, nil)
		db.filteredIndices = append(db.filteredIndices, i)
	}

	if db.list.GetItemCount() > 0 {
		db.list.SetCurrentItem(0)
	}
}

func (db *dirBrowser) navigateTo(path string) {
	if err := db.loadDirectory(path); err != nil {
		return
	}
	db.filterInput.SetText("")
	db.updateList("")
}

func (db *dirBrowser) closeModal(result DirBrowserResult) {
	db.pages.RemovePage("dir-browser-modal")
	if db.callback != nil {
		db.callback(result)
	}
}

func (db *dirBrowser) getSelectedEntry() *dirEntry {
	idx := db.list.GetCurrentItem()
	if idx < 0 || idx >= len(db.filteredIndices) {
		return nil
	}
	origIdx := db.filteredIndices[idx]
	if origIdx < 0 || origIdx >= len(db.allEntries) {
		return nil
	}
	return &db.allEntries[origIdx]
}

func looksLikePath(text string) bool {
	return strings.HasPrefix(text, "/") || strings.HasPrefix(text, "~/")
}

func resolveUserPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func (db *dirBrowser) navigateToTypedPath(filterText string) {
	resolved := resolveUserPath(filterText)
	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		db.navigateTo(resolved)
		return
	}
	for resolved != "/" && resolved != "." {
		parent := filepath.Dir(resolved)
		if info, err := os.Stat(parent); err == nil && info.IsDir() {
			db.navigateTo(parent)
			return
		}
		resolved = parent
	}
}

func (db *dirBrowser) enterSelected() {
	filterText := db.filterInput.GetText()
	if looksLikePath(filterText) {
		db.navigateToTypedPath(filterText)
		return
	}

	entry := db.getSelectedEntry()
	if entry == nil {
		return
	}

	if entry.isGit && entry.fullPath == db.currentPath {
		db.closeModal(DirBrowserResult{Path: entry.fullPath, Action: DirBrowserAddRepo})
		return
	}

	db.navigateTo(entry.fullPath)
}

func (db *dirBrowser) addSelected() {
	entry := db.getSelectedEntry()
	if entry != nil && !entry.isParent {
		db.closeModal(DirBrowserResult{Path: entry.fullPath, Action: DirBrowserAddRepo})
	} else {
		db.closeModal(DirBrowserResult{Path: db.currentPath, Action: DirBrowserAddRepo})
	}
}

func (db *dirBrowser) goUp() {
	if db.currentPath != "/" {
		db.navigateTo(filepath.Dir(db.currentPath))
	}
}

func (db *dirBrowser) setupFilterInput() {
	db.filterInput.SetChangedFunc(func(text string) {
		if !looksLikePath(text) {
			db.updateList(text)
		}
	})

	db.filterInput.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			if db.filterInput.GetText() == "" {
				db.goUp()
				return nil
			}
		case tcell.KeyCtrlA:
			db.addSelected()
			return nil
		case tcell.KeyCtrlS:
			db.closeModal(DirBrowserResult{Path: db.currentPath, Action: DirBrowserScanRepos})
			return nil
		case tcell.KeyDown:
			db.app.SetFocus(db.list)
			return nil
		case tcell.KeyUp:
			current := db.list.GetCurrentItem()
			if current > 0 {
				db.list.SetCurrentItem(current - 1)
			}
			return nil
		}
		return event
	})

	db.filterInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			db.enterSelected()
		case tcell.KeyEscape:
			db.closeModal(DirBrowserResult{Action: DirBrowserCancel})
		case tcell.KeyTab:
			db.app.SetFocus(db.list)
		}
	})
}

func (db *dirBrowser) setupListBindings() {
	db.list.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		db.enterSelected()
		db.app.SetFocus(db.filterInput)
	})

	db.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			db.closeModal(DirBrowserResult{Action: DirBrowserCancel})
			return nil
		case tcell.KeyCtrlA:
			db.addSelected()
			return nil
		case tcell.KeyCtrlS:
			db.closeModal(DirBrowserResult{Path: db.currentPath, Action: DirBrowserScanRepos})
			return nil
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			db.goUp()
			db.app.SetFocus(db.filterInput)
			return nil
		case tcell.KeyRune:
			db.app.SetFocus(db.filterInput)
			return event
		}
		return event
	})
}

func (db *dirBrowser) buildLayout() *tview.Flex {
	container := tview.NewFlex()
	container.SetDirection(tview.FlexRow)
	container.SetBorder(true)
	container.SetTitle(" Add Directory ")
	container.SetTitleAlign(tview.AlignCenter)
	container.SetBorderColor(tcell.ColorAqua)
	container.SetTitleColor(tcell.ColorOrange)
	container.SetBackgroundColor(tcell.ColorDefault)
	container.SetBorderPadding(0, 0, 1, 1)

	infoText := tview.NewTextView()
	infoText.SetDynamicColors(true)
	infoText.SetBackgroundColor(tcell.ColorDefault)
	infoText.SetTextAlign(tview.AlignCenter)
	infoText.SetText("[yellow]Enter[gray] Open  [yellow]^A[gray] Add Repo  [yellow]^S[gray] Scan All  [yellow]Bksp[gray] Up  [yellow]Esc[gray] Close")

	sep := tview.NewTextView()
	sep.SetBackgroundColor(tcell.ColorDefault)
	sep.SetTextColor(tcell.ColorDarkGray)
	sep.SetTextAlign(tview.AlignLeft)
	sep.SetText(strings.Repeat("─", 76))

	container.AddItem(db.pathView, 1, 0, false)
	container.AddItem(db.hintView, 1, 0, false)
	container.AddItem(db.filterInput, 1, 0, true)
	container.AddItem(sep, 1, 0, false)
	container.AddItem(db.list, 0, 1, false)
	container.AddItem(infoText, 1, 0, false)

	return container
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
	pathView := tview.NewTextView()
	pathView.SetDynamicColors(true)
	pathView.SetBackgroundColor(tcell.ColorDefault)
	pathView.SetTextColor(tcell.ColorWhite)

	hintView := tview.NewTextView()
	hintView.SetDynamicColors(true)
	hintView.SetBackgroundColor(tcell.ColorDefault)

	filterInput := tview.NewInputField()
	filterInput.SetLabel(" / ")
	filterInput.SetLabelColor(tcell.ColorAqua)
	filterInput.SetFieldBackgroundColor(tcell.ColorDefault)
	filterInput.SetFieldTextColor(tcell.ColorWhite)
	filterInput.SetPlaceholder("Filter or type a path...")
	filterInput.SetPlaceholderTextColor(tcell.ColorGray)

	list := newFuzzyList()

	db := &dirBrowser{
		pages:       pages,
		app:         app,
		isGitRepo:   isGitRepo,
		callback:    callback,
		currentPath: startPath,
		pathView:    pathView,
		hintView:    hintView,
		filterInput: filterInput,
		list:        list,
	}

	db.navigateTo(startPath)
	db.setupFilterInput()
	db.setupListBindings()

	container := db.buildLayout()

	pages.AddPage("dir-browser-modal", centerModal(container, 80, 25), true, true)
	app.SetFocus(filterInput)
}
