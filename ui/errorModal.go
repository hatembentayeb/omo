package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowErrorModal displays a modal with an error message and OK button
func ShowErrorModal(
	pages *tview.Pages,
	app *tview.Application,
	title string,
	errorText string,
	callback func(),
) {
	// Create text view for the error message
	textView := tview.NewTextView()
	textView.SetText(errorText)
	textView.SetTextColor(tcell.ColorWhite)
	textView.SetTextAlign(tview.AlignCenter)
	textView.SetDynamicColors(true)
	textView.SetBorder(true)
	textView.SetBorderColor(tcell.ColorRed) // Red for errors
	textView.SetTitle(" " + title + " ")
	textView.SetTitleColor(tcell.ColorOrange)
	textView.SetTitleAlign(tview.AlignCenter)
	textView.SetBorderPadding(1, 1, 2, 2)

	// Create form for the OK button
	form := tview.NewForm()
	form.SetItemPadding(0)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetBackgroundColor(tcell.ColorBlack)
	form.SetButtonBackgroundColor(tcell.ColorBlack)
	form.SetButtonTextColor(tcell.ColorWhite)

	// Add OK button
	form.AddButton("OK", func() {
		pages.RemovePage("error-modal")
		if callback != nil {
			callback()
		}
	})

	// Style the button with focus colors
	if b := form.GetButton(0); b != nil {
		b.SetBackgroundColor(tcell.ColorBlack)
		b.SetLabelColor(tcell.ColorWhite)
		b.SetBackgroundColorActivated(tcell.ColorWhite)
		b.SetLabelColorActivated(tcell.ColorBlack)
	}

	// Set a width for the modal
	width := 50
	height := 8 // Adjust based on content

	// Create a flexbox container to center the components
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(textView, height-3, 1, false).
			AddItem(form, 3, 0, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	RemovePage(pages, app, "error-modal", callback)

	// Show the modal
	pages.AddPage("error-modal", flex, true, true)
	app.SetFocus(form)
}
