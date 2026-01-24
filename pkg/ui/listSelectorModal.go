// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowStandardListSelectorModal displays a modal with a list of items to select from.
// This function creates and displays a modal dialog with a selectable list of items,
// allowing the user to choose an option from the provided choices. The modal includes:
// - A titled border with aqua color
// - A list of items with name and optional description
// - Keyboard shortcuts for quick selection
// - Help text at the bottom showing available keys
// - Callback function invoked with the selected item or cancellation
//
// Parameters:
//   - pages: The tview.Pages instance to add the modal to
//   - app: The tview.Application instance for focus management
//   - title: The title to display at the top of the modal
//   - items: A slice of string slices, where each inner slice contains:
//   - [0]: The name/main text of the item
//   - [1]: Optional description/secondary text (can be empty)
//   - callback: Function called when an item is selected or the modal is cancelled
//     with the following parameters:
//   - index: The index of the selected item, or -1 if cancelled
//   - name: The name of the selected item, or empty string if cancelled
//   - cancelled: true if the selection was cancelled, false if an item was selected
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
	list.SetSelectedTextColor(tcell.ColorDefault)
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
	innerFlex := tview.NewFlex()
	innerFlex.SetDirection(tview.FlexRow)
	innerFlex.SetBackgroundColor(tcell.ColorDefault)
	innerFlex.AddItem(nil, 0, 1, false).
		AddItem(list, height-2, 1, true).
		AddItem(helpText, 1, 0, false).
		AddItem(nil, 0, 1, false)

	flex := tview.NewFlex()
	flex.SetBackgroundColor(tcell.ColorDefault)
	flex.AddItem(nil, 0, 1, false).
		AddItem(innerFlex, width, 1, true).
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
