package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ProgressModal represents a modal with a progress indicator
type ProgressModal struct {
	pages         *tview.Pages
	app           *tview.Application
	modal         *tview.Flex
	progressBar   *tview.TextView
	messageText   *tview.TextView
	cancelButton  *tview.Button
	onCancel      func()
	isDeterminate bool
	progress      int
	maxProgress   int
	cancelled     bool
}

// NewProgressModal creates a new progress modal
func NewProgressModal(pages *tview.Pages, app *tview.Application, title string, isDeterminate bool) *ProgressModal {
	pm := &ProgressModal{
		pages:         pages,
		app:           app,
		isDeterminate: isDeterminate,
		progress:      0,
		maxProgress:   100,
		cancelled:     false,
	}

	// Create message text area
	pm.messageText = tview.NewTextView()
	pm.messageText.SetDynamicColors(true)
	pm.messageText.SetTextAlign(tview.AlignCenter)
	pm.messageText.SetText("Processing...")
	pm.messageText.SetWordWrap(true)
	pm.messageText.SetBackgroundColor(tcell.ColorBlack)

	// Create progress display
	pm.progressBar = tview.NewTextView()
	pm.progressBar.SetDynamicColors(true)
	pm.progressBar.SetTextAlign(tview.AlignCenter)
	pm.progressBar.SetBackgroundColor(tcell.ColorBlack)

	if isDeterminate {
		pm.progressBar.SetText("[yellow]0%[white]")
	} else {
		pm.progressBar.SetText("[yellow]⏳[white]")
	}

	// Create form for the button
	buttonForm := tview.NewForm().
		AddButton("Cancel", func() {
			pm.cancelled = true
			if pm.onCancel != nil {
				pm.onCancel()
			}
			pm.Close()
		})

	// Style the form to match other modals
	buttonForm.SetBackgroundColor(tcell.ColorBlack)
	buttonForm.SetButtonBackgroundColor(tcell.ColorBlack)
	buttonForm.SetButtonTextColor(tcell.ColorWhite)
	buttonForm.SetButtonsAlign(tview.AlignCenter)

	// Style the cancel button to match other modals
	if buttonForm.GetButtonCount() > 0 {
		button := buttonForm.GetButton(0)
		button.SetBackgroundColor(tcell.ColorBlack)
		button.SetLabelColor(tcell.ColorAqua)
		button.SetBackgroundColorActivated(tcell.ColorAqua)
		button.SetLabelColorActivated(tcell.ColorBlack)
		pm.cancelButton = button
	}

	// Create content with consistent styling
	content := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(pm.messageText, 2, 1, false).
		AddItem(pm.progressBar, 1, 1, false).
		AddItem(buttonForm, 1, 1, true)

	// Set up border and title with consistent styling
	content.SetBorder(true)
	content.SetTitle(" " + title + " ")
	content.SetTitleAlign(tview.AlignCenter)
	content.SetTitleColor(tcell.ColorAqua)
	content.SetBorderColor(tcell.ColorAqua)
	content.SetBackgroundColor(tcell.ColorBlack)
	content.SetBorderPadding(1, 1, 1, 1)

	// Center the modal
	pm.modal = tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().
				SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(content, 7, 1, true).
				AddItem(nil, 0, 1, false),
			50, 1, true).
		AddItem(nil, 0, 1, false)

	return pm
}

// Show displays the progress modal
func (pm *ProgressModal) Show() {
	pm.pages.AddPage("progress-modal", pm.modal, true, true)
	pm.app.SetFocus(pm.cancelButton)

	// Add ESC handler
	oldHandler := pm.app.GetInputCapture()
	pm.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			pm.cancelled = true
			if pm.onCancel != nil {
				pm.onCancel()
			}
			pm.Close()
			pm.app.SetInputCapture(oldHandler)
			return nil
		}
		return event
	})
}

// Close removes the progress modal
func (pm *ProgressModal) Close() {
	pm.pages.RemovePage("progress-modal")
}

// UpdateMessage updates the message displayed in the progress modal
func (pm *ProgressModal) UpdateMessage(message string) {
	pm.app.QueueUpdateDraw(func() {
		pm.messageText.SetText(message)
	})
}

// IncrementProgress increments the progress by the specified amount
func (pm *ProgressModal) IncrementProgress(amount int) {
	if !pm.isDeterminate {
		return
	}

	pm.progress += amount
	if pm.progress > pm.maxProgress {
		pm.progress = pm.maxProgress
	}

	pm.updateProgressDisplay()
}

// SetProgress sets the progress to a specific value
func (pm *ProgressModal) SetProgress(progress int) {
	if !pm.isDeterminate {
		return
	}

	pm.progress = progress
	if pm.progress > pm.maxProgress {
		pm.progress = pm.maxProgress
	}
	if pm.progress < 0 {
		pm.progress = 0
	}

	pm.updateProgressDisplay()
}

// SetMaxProgress sets the maximum progress value
func (pm *ProgressModal) SetMaxProgress(maxProgress int) {
	pm.maxProgress = maxProgress
	pm.updateProgressDisplay()
}

// IsCancelled returns whether the operation was cancelled
func (pm *ProgressModal) IsCancelled() bool {
	return pm.cancelled
}

// SetOnCancel sets the function to call when the operation is cancelled
func (pm *ProgressModal) SetOnCancel(onCancel func()) {
	pm.onCancel = onCancel
}

// PulseIndeterminate updates the indeterminate progress indicator
func (pm *ProgressModal) PulseIndeterminate() {
	if pm.isDeterminate {
		return
	}

	pm.app.QueueUpdateDraw(func() {
		currentText := pm.progressBar.GetText(false)
		if currentText == "[yellow]⏳[white]" {
			pm.progressBar.SetText("[yellow]⌛[white]")
		} else {
			pm.progressBar.SetText("[yellow]⏳[white]")
		}
	})
}

// updateProgressDisplay updates the progress bar display
func (pm *ProgressModal) updateProgressDisplay() {
	if !pm.isDeterminate {
		return
	}

	percentage := 0
	if pm.maxProgress > 0 {
		percentage = (pm.progress * 100) / pm.maxProgress
	}

	pm.app.QueueUpdateDraw(func() {
		pm.progressBar.SetText(fmt.Sprintf("[yellow]%d%%[white]", percentage))
	})
}

// ShowStandardProgressModal shows a standard progress modal with indeterminate progress
func ShowStandardProgressModal(
	pages *tview.Pages,
	app *tview.Application,
	title string,
	message string,
	onCancel func(),
) *ProgressModal {
	progressModal := NewProgressModal(pages, app, title, false)
	progressModal.UpdateMessage(message)
	progressModal.SetOnCancel(onCancel)
	progressModal.Show()
	return progressModal
}
