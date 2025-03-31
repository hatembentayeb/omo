package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// SortOptions defines the configuration for sort operations
type SortOptions struct {
	Column    string // Column to sort by
	SortType  string // Type of sort: "alphabet", "date", "number"
	Direction string // "asc" or "desc"
}

// ShowSortModal displays a modal with sort options and returns the selected options
func ShowSortModal(
	pages *tview.Pages,
	app *tview.Application,
	columns []string,
	callback func(options *SortOptions, cancelled bool),
) {
	// Create form for inputs
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" Sort Options ")
	form.SetTitleColor(tcell.ColorOrange)
	form.SetBorderColor(tcell.ColorAqua)
	form.SetFieldBackgroundColor(tcell.ColorBlack)
	form.SetButtonBackgroundColor(tcell.ColorDefault)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetLabelColor(tcell.ColorAqua)
	form.SetItemPadding(0) // Remove padding between form items

	// Default options
	options := &SortOptions{
		Column:    columns[0],
		SortType:  "alphabet",
		Direction: "asc",
	}

	// Column dropdown
	columnOptions := make([]string, len(columns))
	for i, col := range columns {
		columnOptions[i] = col
	}
	form.AddDropDown("Column", columnOptions, 0, func(option string, optionIndex int) {
		options.Column = option
	})

	// Sort type dropdown
	sortTypeOptions := []string{"alphabet", "date", "number"}
	form.AddDropDown("Sort Type", sortTypeOptions, 0, func(option string, optionIndex int) {
		options.SortType = option
	})

	// Direction dropdown
	directionOptions := []string{"ascending", "descending"}
	form.AddDropDown("Direction", directionOptions, 0, func(option string, optionIndex int) {
		if option == "ascending" {
			options.Direction = "asc"
		} else {
			options.Direction = "desc"
		}
	})

	// Create centered buttons with a cleaner layout
	form.AddButton("Apply", func() {
		pages.RemovePage("sort-modal")
		if callback != nil {
			callback(options, false)
		}
	})
	form.AddButton("Cancel", func() {
		pages.RemovePage("sort-modal")
		if callback != nil {
			callback(nil, true)
		}
	})

	// Set button alignment to center
	form.SetButtonsAlign(tview.AlignCenter)

	// Create a flexbox container for the modal, with reduced margins
	width := 45
	height := 12
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	// Show the modal
	pageID := "sort-modal"
	pages.AddPage(pageID, flex, true, true)

	// Set up ESC key handling to close the modal
	RemovePage(pages, app, pageID, func() {
		if callback != nil {
			callback(nil, true)
		}
	})

	app.SetFocus(form)
}
