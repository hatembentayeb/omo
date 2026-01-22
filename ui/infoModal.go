package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowInfoModal displays a modal with information and an OK button
func ShowInfoModal(
	pages *tview.Pages,
	app *tview.Application,
	title string,
	text string,
	callback func(),
) {
	// Create text view for the information
	textView := tview.NewTextView()
	textView.SetText(text)
	textView.SetTextColor(tcell.ColorWhite)
	textView.SetDynamicColors(true)
	textView.SetScrollable(true) // Make scrollable like help modal
	textView.SetWordWrap(true)   // Add word wrap like help modal
	textView.SetBorder(true)
	textView.SetBorderColor(tcell.ColorAqua)
	textView.SetTitle(" " + title + " ")
	textView.SetTitleColor(tcell.ColorOrange)
	textView.SetTitleAlign(tview.AlignCenter)
	textView.SetBorderPadding(1, 1, 2, 2)

	// Use larger width/height like help modal
	width := 76  // Same as default help modal
	height := 26 // Adjusted for content

	// Create a flexbox container to center the components - similar to help modal layout
	innerFlex := tview.NewFlex()
	innerFlex.SetDirection(tview.FlexRow)
	innerFlex.SetBackgroundColor(tcell.ColorDefault)
	innerFlex.AddItem(nil, 0, 1, false).
		AddItem(textView, height, 1, true).
		AddItem(nil, 0, 1, false)
	
	flex := tview.NewFlex()
	flex.SetBackgroundColor(tcell.ColorDefault)
	flex.AddItem(nil, 0, 1, false).
		AddItem(innerFlex, width, 1, true).
		AddItem(nil, 0, 1, false)

	// Use the global RemovePage function for consistent ESC handling
	const pageID = "info-modal"
	RemovePage(pages, app, pageID, callback)

	// Show the modal
	pages.AddPage(pageID, flex, true, true)
	app.SetFocus(textView) // Focus on the text view directly
}
