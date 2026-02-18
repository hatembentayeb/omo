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

func newFuzzyContainer(title string) *tview.Flex {
	container := tview.NewFlex()
	container.SetDirection(tview.FlexRow)
	container.SetBorder(true)
	container.SetTitle(" " + title + " ")
	container.SetTitleAlign(tview.AlignCenter)
	container.SetBorderColor(tcell.ColorAqua)
	container.SetTitleColor(tcell.ColorOrange)
	container.SetBackgroundColor(tcell.ColorDefault)
	container.SetBorderPadding(0, 0, 1, 1)
	return container
}

func newFuzzyInput() *tview.InputField {
	inputField := tview.NewInputField()
	inputField.SetLabel(" 🔍 ")
	inputField.SetLabelColor(tcell.ColorYellow)
	inputField.SetFieldBackgroundColor(tcell.ColorDefault)
	inputField.SetFieldTextColor(tcell.ColorWhite)
	inputField.SetPlaceholder("Type to search...")
	inputField.SetPlaceholderTextColor(tcell.ColorGray)
	return inputField
}

func newFuzzyList() *tview.List {
	list := tview.NewList()
	list.SetBackgroundColor(tcell.ColorDefault)
	list.SetMainTextColor(tcell.ColorWhite)
	list.SetSecondaryTextColor(tcell.ColorGray)
	list.SetSelectedTextColor(tcell.ColorBlack)
	list.SetSelectedBackgroundColor(tcell.ColorAqua)
	list.SetHighlightFullLine(true)
	list.ShowSecondaryText(true)
	return list
}

func centerModal(content tview.Primitive, width, height int) *tview.Flex {
	innerFlex := tview.NewFlex()
	innerFlex.SetDirection(tview.FlexRow)
	innerFlex.SetBackgroundColor(tcell.ColorDefault)
	innerFlex.AddItem(nil, 0, 1, false)
	innerFlex.AddItem(content, height, 0, true)
	innerFlex.AddItem(nil, 0, 1, false)

	flex := tview.NewFlex()
	flex.SetBackgroundColor(tcell.ColorDefault)
	flex.AddItem(nil, 0, 1, false)
	flex.AddItem(innerFlex, width, 0, true)
	flex.AddItem(nil, 0, 1, false)
	return flex
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

	container := newFuzzyContainer(title)
	inputField := newFuzzyInput()
	list := newFuzzyList()

	filteredItems := make([]int, len(items))
	for i := range items {
		filteredItems[i] = i
	}

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

	updateList("")

	inputField.SetChangedFunc(func(text string) {
		updateList(text)
	})

	cancelModal := func() {
		pages.RemovePage(pageID)
		if callback != nil {
			callback(-1, nil, true)
		}
	}

	selecting := false

	selectItem := func() {
		if selecting {
			return
		}
		selecting = true

		currentIndex := list.GetCurrentItem()
		if list.GetItemCount() == 0 || currentIndex < 0 || currentIndex >= len(filteredItems) {
			cancelModal()
			return
		}

		originalIndex := filteredItems[currentIndex]
		if originalIndex < 0 || originalIndex >= len(items) {
			cancelModal()
			return
		}

		selectedItem := items[originalIndex]
		pages.RemovePage(pageID)
		if callback != nil {
			callback(originalIndex, &selectedItem, false)
		}
	}

	list.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		selectItem()
	})

	inputField.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEnter:
			selectItem()
		case tcell.KeyEscape:
			cancelModal()
		case tcell.KeyDown, tcell.KeyTab:
			app.SetFocus(list)
		}
	})

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEnter:
			selectItem()
			return nil
		case tcell.KeyEscape:
			cancelModal()
			return nil
		case tcell.KeyRune:
			app.SetFocus(inputField)
			return event
		case tcell.KeyBackspace, tcell.KeyBackspace2:
			app.SetFocus(inputField)
			return event
		}
		return event
	})

	container.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			cancelModal()
			return nil
		}
		return event
	})

	infoText := tview.NewTextView()
	infoText.SetText(" ↑↓ Navigate  Enter Select  Esc Cancel")
	infoText.SetTextColor(tcell.ColorGray)
	infoText.SetBackgroundColor(tcell.ColorDefault)
	infoText.SetTextAlign(tview.AlignCenter)

	container.AddItem(inputField, 1, 0, true)
	container.AddItem(list, 0, 1, false)
	container.AddItem(infoText, 1, 0, false)

	pages.AddPage(pageID, centerModal(container, 70, 20), true, true)
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
