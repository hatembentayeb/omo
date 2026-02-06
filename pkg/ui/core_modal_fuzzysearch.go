package ui

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// FuzzySearchItem represents an item in the fuzzy search list
type FuzzySearchItem struct {
	Name        string
	Description string
	Data        interface{}
}

// ShowFuzzySearchModal displays a modal with real-time fuzzy search filtering
func ShowFuzzySearchModal(
	pages *tview.Pages,
	app *tview.Application,
	title string,
	items []FuzzySearchItem,
	callback func(index int, item *FuzzySearchItem, cancelled bool),
) {
	const pageID = "fuzzy-search-modal"

	// Create the main container
	container := tview.NewFlex()
	container.SetDirection(tview.FlexRow)
	container.SetBorder(true)
	container.SetTitle(" " + title + " ")
	container.SetTitleAlign(tview.AlignCenter)
	container.SetBorderColor(tcell.ColorAqua)
	container.SetTitleColor(tcell.ColorOrange)
	container.SetBackgroundColor(tcell.ColorDefault)
	container.SetBorderPadding(0, 0, 1, 1)

	// Create the search input field
	inputField := tview.NewInputField()
	inputField.SetLabel(" 🔍 ")
	inputField.SetLabelColor(tcell.ColorYellow)
	inputField.SetFieldBackgroundColor(tcell.ColorDefault)
	inputField.SetFieldTextColor(tcell.ColorWhite)
	inputField.SetPlaceholder("Type to search...")
	inputField.SetPlaceholderTextColor(tcell.ColorGray)

	// Create the results list
	list := tview.NewList()
	list.SetBackgroundColor(tcell.ColorDefault)
	list.SetMainTextColor(tcell.ColorWhite)
	list.SetSecondaryTextColor(tcell.ColorGray)
	list.SetSelectedTextColor(tcell.ColorBlack)
	list.SetSelectedBackgroundColor(tcell.ColorAqua)
	list.SetHighlightFullLine(true)
	list.ShowSecondaryText(true)

	// Track filtered items
	filteredItems := make([]int, len(items)) // Maps list index to original items index
	for i := range items {
		filteredItems[i] = i
	}

	// Function to update the list based on search query
	updateList := func(query string) {
		list.Clear()
		filteredItems = filteredItems[:0]

		query = strings.ToLower(strings.TrimSpace(query))

		for i, item := range items {
			if query == "" || fuzzyMatch(strings.ToLower(item.Name), query) || fuzzyMatch(strings.ToLower(item.Description), query) {
				list.AddItem(item.Name, item.Description, 0, nil)
				filteredItems = append(filteredItems, i)
			}
		}

		if list.GetItemCount() > 0 {
			list.SetCurrentItem(0)
		}
	}

	// Initialize with all items
	updateList("")

	// Handle input changes for real-time filtering
	inputField.SetChangedFunc(func(text string) {
		updateList(text)
	})

	// Track if selection is in progress to prevent double-trigger
	selecting := false

	// Handle selection
	selectItem := func() {
		if selecting {
			return
		}
		selecting = true

		if list.GetItemCount() == 0 {
			// No items to select, close modal
			pages.RemovePage(pageID)
			if callback != nil {
				callback(-1, nil, true)
			}
			return
		}

		currentIndex := list.GetCurrentItem()
		if currentIndex < 0 || currentIndex >= len(filteredItems) {
			// Invalid index, close modal
			pages.RemovePage(pageID)
			if callback != nil {
				callback(-1, nil, true)
			}
			return
		}

		originalIndex := filteredItems[currentIndex]
		if originalIndex < 0 || originalIndex >= len(items) {
			// Invalid original index, close modal
			pages.RemovePage(pageID)
			if callback != nil {
				callback(-1, nil, true)
			}
			return
		}

		selectedItem := items[originalIndex]
		// Remove page first, then call callback
		pages.RemovePage(pageID)
		if callback != nil {
			callback(originalIndex, &selectedItem, false)
		}
	}

	// Set up list selection
	list.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selectItem()
	})

	// Handle Enter in input field
	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			selectItem()
		} else if key == tcell.KeyEscape {
			pages.RemovePage(pageID)
			if callback != nil {
				callback(-1, nil, true)
			}
		} else if key == tcell.KeyDown || key == tcell.KeyTab {
			app.SetFocus(list)
		}
	})

	// Handle navigation in list
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			selectItem()
			return nil
		case tcell.KeyEscape:
			pages.RemovePage(pageID)
			if callback != nil {
				callback(-1, nil, true)
			}
			return nil
		case tcell.KeyRune:
			// If typing, go back to input field
			app.SetFocus(inputField)
			// Re-send the key to input field
			return event
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			app.SetFocus(inputField)
			return event
		}
		return event
	})

	// Container input capture for global ESC
	container.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			pages.RemovePage(pageID)
			if callback != nil {
				callback(-1, nil, true)
			}
			return nil
		}
		return event
	})

	// Add info text
	infoText := tview.NewTextView()
	infoText.SetText(" ↑↓ Navigate  Enter Select  Esc Cancel")
	infoText.SetTextColor(tcell.ColorGray)
	infoText.SetBackgroundColor(tcell.ColorDefault)
	infoText.SetTextAlign(tview.AlignCenter)

	// Build the layout
	container.AddItem(inputField, 1, 0, true)
	container.AddItem(list, 0, 1, false)
	container.AddItem(infoText, 1, 0, false)

	// Set modal size
	width := 70
	height := 20

	// Center the modal
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

	// Show the modal
	pages.AddPage(pageID, flex, true, true)
	app.SetFocus(inputField)
}

// fuzzyMatch performs a simple fuzzy match
func fuzzyMatch(text, pattern string) bool {
	if pattern == "" {
		return true
	}

	// Simple substring match
	if strings.Contains(text, pattern) {
		return true
	}

	// Fuzzy match - all pattern chars must appear in order
	patternIdx := 0
	for i := 0; i < len(text) && patternIdx < len(pattern); i++ {
		if text[i] == pattern[patternIdx] {
			patternIdx++
		}
	}
	return patternIdx == len(pattern)
}
