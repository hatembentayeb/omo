package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ShowCompactStyledInputModal displays a compact, perfectly centered modal with a text input field
func ShowCompactStyledInputModal(
	pages *tview.Pages,
	app *tview.Application,
	title string,
	inputLabel string,
	placeholder string,
	inputFieldWidth int,
	fieldValidator func(textToCheck string, lastChar rune) bool,
	callback func(text string, cancelled bool),
) {
	const pageID = "compact-modal"

	// Create form with input field - matching UI package styling
	form := tview.NewForm()
	form.SetItemPadding(0)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetBackgroundColor(tcell.ColorDefault)
	form.SetButtonBackgroundColor(tcell.ColorDefault)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetFieldBackgroundColor(tcell.ColorDefault)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetBorder(true)
	form.SetTitle(" " + title + " ")
	form.SetTitleAlign(tview.AlignCenter)
	form.SetBorderColor(tcell.ColorAqua)
	form.SetTitleColor(tcell.ColorOrange)
	form.SetBorderPadding(1, 1, 2, 2)

	// Add the input field with specified width
	form.AddInputField(inputLabel, placeholder, inputFieldWidth, fieldValidator, nil)
	inputField := form.GetFormItem(0).(*tview.InputField)

	// Shared cleanup and callback invocation
	closeModal := func(value string, cancelled bool) {
		pages.RemovePage(pageID)
		if callback != nil {
			callback(value, cancelled)
		}
	}

	// Add buttons with minimal vertical spacing
	form.AddButton("OK", func() {
		value := inputField.GetText()
		closeModal(value, value == "")
	})

	form.AddButton("Cancel", func() {
		closeModal("", true)
	})

	// Handle Enter key in input field
	inputField.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			value := inputField.GetText()
			closeModal(value, value == "")
		} else if key == tcell.KeyEscape {
			closeModal("", true)
		}
	})

	// Style the buttons with focus colors
	for i := 0; i < form.GetButtonCount(); i++ {
		if b := form.GetButton(i); b != nil {
			b.SetBackgroundColor(tcell.ColorDefault)
			b.SetLabelColor(tcell.ColorWhite)
			b.SetBackgroundColorActivated(tcell.ColorAqua)
			b.SetLabelColorActivated(tcell.ColorBlack)
		}
	}

	// Set a width for the modal
	width := 50
	height := 8 // Compact height

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

	// Show the modal
	pages.AddPage(pageID, flex, true, true)
	app.SetFocus(inputField)
}
