package main

import (
	"omo/ui"

	"github.com/rivo/tview"
)

// CreateStandardFormModal creates a standard modal containing a form
// This is a wrapper around standard UI components to maintain API compatibility
// while delegating the actual implementation to the UI package.
func CreateStandardFormModal(form *tview.Form, title string, width, height int) tview.Primitive {
	// Style the form according to UI package standards
	form.SetItemPadding(0)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetBorder(true)
	form.SetTitle(" " + title + " ")
	form.SetTitleAlign(tview.AlignCenter)

	// Create a flex for the form with padding (similar to the UI package pattern)
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

	return flex
}

// ShowConfirmModal shows a confirmation dialog
// This is a wrapper that calls the UI package's standard confirmation modal
func ShowConfirmModal(pages *tview.Pages, app *tview.Application, title, message string, callback func(confirmed bool)) {
	// Use the UI package's standard confirmation modal
	ui.ShowStandardConfirmationModal(pages, app, title, message, callback)
}
