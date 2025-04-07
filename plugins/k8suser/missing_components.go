package main

import (
	"github.com/rivo/tview"
)

// CreateStandardFormModal creates a standard modal containing a form
func CreateStandardFormModal(form *tview.Form, title string, width, height int) tview.Primitive {
	// Create a flex for the form with padding
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().
				SetDirection(tview.FlexColumn).
				AddItem(nil, 0, 1, false).
				AddItem(form, width, 1, true).
				AddItem(nil, 0, 1, false),
			height, 1, true,
		).
		AddItem(nil, 0, 1, false)

	// Add border and title
	frame := tview.NewFrame(flex).
		SetBorders(1, 1, 1, 1, 0, 0).
		SetBorder(true).
		SetTitle(title).
		SetTitleAlign(tview.AlignCenter).
		SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	return frame
}

// ShowConfirmModal shows a confirmation dialog
func ShowConfirmModal(pages *tview.Pages, app *tview.Application, title, message string, callback func(confirmed bool)) {
	// Create a modal
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			pages.RemovePage("confirm-modal")
			callback(buttonIndex == 0) // Yes is index 0
		})

	modal.SetBorder(true)
	modal.SetTitle(title)
	modal.SetBorderColor(tview.Styles.BorderColor)
	modal.SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	// Add the modal to the pages
	pages.AddPage("confirm-modal", modal, true, true)
	app.SetFocus(modal)
}
