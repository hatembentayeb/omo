package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowStandardConfirmationModal displays a modal with Yes/No buttons for confirming actions
func ShowStandardConfirmationModal(
	pages *tview.Pages,
	app *tview.Application,
	title string,
	text string,
	callback func(confirmed bool),
) {
	// Create a form with buttons
	form := tview.NewForm()
	form.SetItemPadding(0)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetBackgroundColor(tcell.ColorDefault)
	form.SetButtonBackgroundColor(tcell.ColorDefault)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetBorder(true)
	form.SetTitle(" " + title + " ")
	form.SetTitleAlign(tview.AlignCenter)
	form.SetBorderColor(tcell.ColorAqua)
	form.SetTitleColor(tcell.ColorOrange)
	form.SetBorderPadding(1, 1, 2, 2)

	// Add text with minimal spacing
	form.AddTextView("", text, 0, 2, true, false)

	// Add buttons
	form.AddButton("Yes", func() {
		pages.RemovePage("confirmation-modal")
		if callback != nil {
			callback(true)
		}
	})

	form.AddButton("No", func() {
		pages.RemovePage("confirmation-modal")
		if callback != nil {
			callback(false)
		}
	})

	// Style the buttons with focus colors
	for i := 0; i < form.GetButtonCount(); i++ {
		if b := form.GetButton(i); b != nil {
			b.SetBackgroundColor(tcell.ColorDefault)
			b.SetLabelColor(tcell.ColorWhite)
			b.SetBackgroundColorActivated(tcell.ColorWhite)
			b.SetLabelColorActivated(tcell.ColorDefault)
		}
	}

	// Set a width for the modal
	width := 50
	height := 8 // Fixed height for confirmation dialog

	// Create a flexbox container to center the components
	innerFlex := tview.NewFlex()
	innerFlex.SetDirection(tview.FlexRow)
	innerFlex.SetBackgroundColor(tcell.ColorDefault)
	innerFlex.AddItem(nil, 0, 1, false).
		AddItem(form, height, 1, true).
		AddItem(nil, 0, 1, false)
	
	flex := tview.NewFlex()
	flex.SetBackgroundColor(tcell.ColorDefault)
	flex.AddItem(nil, 0, 1, false).
		AddItem(innerFlex, width, 1, true).
		AddItem(nil, 0, 1, false)

	const pageID = "confirmation-modal"
	RemovePage(pages, app, pageID, func() {
		if callback != nil {
			callback(false)
		}
	})

	// Show the modal
	pages.AddPage(pageID, flex, true, true)
	app.SetFocus(form)
}
