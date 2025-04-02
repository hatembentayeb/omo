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

// ShowStandardErrorModal displays a standardized error modal with consistent styling
// This is the preferred method for showing errors throughout the application
func ShowStandardErrorModal(
	pages *tview.Pages,
	app *tview.Application,
	title string,
	errorText string,
	callback func(),
) {
	// Default title if none provided
	if title == "" {
		title = "Error"
	}

	// Create text view for the error message with standardized styling
	textView := tview.NewTextView()
	textView.SetText(errorText)
	textView.SetTextColor(tcell.ColorWhite)
	textView.SetTextAlign(tview.AlignLeft)
	textView.SetDynamicColors(true)
	textView.SetScrollable(true)
	textView.SetWordWrap(true)
	textView.SetBorder(true)
	textView.SetBorderColor(tcell.ColorRed)
	textView.SetTitle(" " + title + " ")
	textView.SetTitleColor(tcell.ColorRed)
	textView.SetTitleAlign(tview.AlignCenter)
	textView.SetBorderPadding(1, 1, 2, 2)

	// Create form for buttons with standardized styling
	form := tview.NewForm()
	form.SetItemPadding(0)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetBackgroundColor(tcell.ColorBlack)
	form.SetButtonBackgroundColor(tcell.ColorBlack)
	form.SetButtonTextColor(tcell.ColorWhite)

	// Add OK button with standardized styling
	form.AddButton("OK", func() {
		pages.RemovePage("error-modal")
		if callback != nil {
			callback()
		}
	})

	// Style the button with standardized focus colors
	if b := form.GetButton(0); b != nil {
		b.SetBackgroundColor(tcell.ColorBlack)
		b.SetLabelColor(tcell.ColorRed)
		b.SetBackgroundColorActivated(tcell.ColorRed)
		b.SetLabelColorActivated(tcell.ColorBlack)
	}

	// Calculate appropriate dimensions based on content
	width := 60 // Wider modal for better readability

	// Estimate height based on error text length
	estimatedLines := len(errorText)/40 + 4 // Rough estimate: 40 chars per line + padding
	height := min(estimatedLines, 15)       // Cap at 15 lines to prevent overly large modals

	// Create a flexbox layout with standardized margins
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(textView, height-3, 1, true). // Make text view focused for scrolling
			AddItem(form, 3, 0, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	// Set a standardized ESC key handler
	RemovePage(pages, app, "error-modal", callback)

	// Show the modal with focus on the text view for scrolling
	pages.AddPage("error-modal", flex, true, true)
	app.SetFocus(textView) // Focus on text first to allow reading/scrolling
}
