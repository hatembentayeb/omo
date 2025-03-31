package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowStandardListSelectorModal displays a modal with a list of items to select from
func ShowStandardListSelectorModal(
	pages *tview.Pages,
	app *tview.Application,
	title string,
	items [][]string, // Each item is [name, description]
	callback func(index int, name string, cancelled bool),
) {
	// Create list with items
	list := tview.NewList()
	list.SetHighlightFullLine(true)
	list.SetSelectedBackgroundColor(tcell.ColorAqua)
	list.SetSelectedTextColor(tcell.ColorBlack)
	list.SetBorder(true)
	list.SetBorderColor(tcell.ColorAqua)
	list.SetTitle(" " + title + " ")
	list.SetTitleColor(tcell.ColorOrange)
	list.SetTitleAlign(tview.AlignCenter)
	list.SetBorderPadding(1, 1, 2, 2)

	// Add items to the list
	for i, item := range items {
		name := item[0]
		description := ""
		if len(item) > 1 {
			description = item[1]
		}
		list.AddItem(name, description, rune('0'+i), nil)
	}

	// Create help text
	helpText := tview.NewTextView()
	helpText.SetText(" Enter: Select  •  Esc: Cancel ")
	helpText.SetTextAlign(tview.AlignCenter)
	helpText.SetTextColor(tcell.ColorYellow)
	helpText.SetDynamicColors(true)

	// Set a fixed width for the modal
	width := 50
	height := len(items) + 6 // Adjust based on item count plus border and help text

	// Create a flexbox container to center the components
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(list, height-2, 1, true).
			AddItem(helpText, 1, 0, false).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	// Set the callback for when an item is selected
	list.SetSelectedFunc(func(index int, mainText, secondaryText string, shortcut rune) {
		pages.RemovePage("list-selector-modal")
		if callback != nil {
			callback(index, mainText, false)
		}
	})

	const pageID = "list-selector-modal"
	RemovePage(pages, app, pageID, func() {
		if callback != nil {
			callback(-1, "", true)
		}
	})

	// Show the modal
	pages.AddPage(pageID, flex, true, true)
	app.SetFocus(list)
}
