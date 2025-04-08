package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowConfirmModal displays a confirmation dialog with yes/no options
func ShowConfirmModal(
	pages *tview.Pages,
	app *tview.Application,
	title string,
	text string,
	yesLabel string,
	noLabel string,
	yesFunc func(),
	noFunc func(),
) {
	modal := tview.NewModal().
		SetText(text).
		AddButtons([]string{yesLabel, noLabel}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			pages.RemovePage("confirm-modal")
			if buttonIndex == 0 && yesFunc != nil {
				yesFunc()
			} else if buttonIndex == 1 && noFunc != nil {
				noFunc()
			}
		})

	modal.SetTitle(" " + title + " ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorRed).
		SetTitleColor(tcell.ColorYellow)

	pages.AddPage("confirm-modal", modal, true, true)
	app.SetFocus(modal)
}

// ShowInfoModal displays an information dialog with an OK button
func ShowInfoModal(
	pages *tview.Pages,
	app *tview.Application,
	title string,
	text string,
	doneFunc func(),
) {
	// Create a text view for scrollable content
	textView := tview.NewTextView().
		SetText(text).
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true)

	// Create a frame for the text view
	frame := tview.NewFrame(textView).
		SetBorders(0, 0, 0, 0, 0, 0)

	// Create the buttons
	buttons := tview.NewFlex().
		SetDirection(tview.FlexColumn)

	// Add OK button
	okButton := tview.NewButton("OK").
		SetSelectedFunc(func() {
			pages.RemovePage("info-modal")
			if doneFunc != nil {
				doneFunc()
			}
		})

	buttons.AddItem(nil, 0, 1, false).
		AddItem(okButton, 6, 1, true).
		AddItem(nil, 0, 1, false)

	// Build the modal
	modal := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(frame, 0, 1, true).
		AddItem(buttons, 1, 1, false)

	// Set up border and title
	modal.SetBorder(true).
		SetTitle(" " + title + " ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorBlue).
		SetTitleColor(tcell.ColorYellow).
		SetBackgroundColor(tcell.ColorBlack)

	// Handle ESC key
	modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			pages.RemovePage("info-modal")
			if doneFunc != nil {
				doneFunc()
			}
			return nil
		}
		return event
	})

	// Size and center the modal
	width := 70
	height := 20

	centeredModal := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(modal, height, 1, true).
				AddItem(nil, 0, 1, false),
			width, 1, true).
		AddItem(nil, 0, 1, false)

	// Add the modal to the pages and set focus
	pages.AddPage("info-modal", centeredModal, true, true)
	app.SetFocus(textView)
}

// RemovePage is a utility function to remove a page when ESC is pressed
func RemovePage(pages *tview.Pages, app *tview.Application, pageName string, focusAfter tview.Primitive) {
	if focusAfter == nil {
		// Create a generic handler that doesn't set focus
		pages.AddPage(pageName+"-esc-handler", tview.NewBox().SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				pages.RemovePage(pageName)
				pages.RemovePage(pageName + "-esc-handler")
				return nil
			}
			return event
		}), false, false)
	} else {
		// Create a handler that sets focus to the specified primitive
		pages.AddPage(pageName+"-esc-handler", tview.NewBox().SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyEscape {
				pages.RemovePage(pageName)
				pages.RemovePage(pageName + "-esc-handler")
				app.SetFocus(focusAfter)
				return nil
			}
			return event
		}), false, false)
	}
}
