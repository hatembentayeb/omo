// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
package ui

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ProgressModal holds the state for a progress modal.
// This component provides a progress bar dialog that can be used to show
// the status of long-running operations. It includes a progress bar,
// status text, and an optional cancel button.
type ProgressModal struct {
	pages         *tview.Pages       // Reference to the application pages
	app           *tview.Application // Reference to the main application
	modal         *tview.Flex        // The main container for the modal
	progressBar   *tview.TextView    // Visual progress bar component
	statusText    *tview.TextView    // Status text component
	progress      int                // Current progress value
	maxProgress   int                // Maximum progress value
	pageName      string             // Name of the page in the pages component
	onCancel      func()             // Callback when cancel is pressed
	isCancellable bool               // Whether the operation can be cancelled
	autoClose     bool               // Whether to auto-close when complete
	done          bool               // Whether the operation is complete
}

// NewProgressModal creates a new progress modal with the given configuration.
// This factory function initializes a ProgressModal with default settings
// and creates the UI components.
//
// Parameters:
//   - pages: The tview.Pages instance to add the modal to
//   - app: The tview.Application instance for UI updates
//   - title: The title to display at the top of the modal
//   - maxProgress: The maximum value for the progress bar (100%)
//
// Returns:
//   - A configured ProgressModal instance (not yet displayed)
func NewProgressModal(pages *tview.Pages, app *tview.Application, title string, maxProgress int) *ProgressModal {
	pm := &ProgressModal{
		pages:       pages,
		app:         app,
		progress:    0,
		maxProgress: maxProgress,
		pageName:    "progress-modal",
		autoClose:   true,
		done:        false,
	}

	// Create the progress bar text view
	pm.progressBar = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft).
		SetText("")

	// Create the status text
	pm.statusText = tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("")

	// Create a flex for the progress and status
	contentFlex := tview.NewFlex()
	contentFlex.SetDirection(tview.FlexRow)
	contentFlex.SetBackgroundColor(tcell.ColorDefault)
	contentFlex.AddItem(pm.progressBar, 1, 0, false).
		AddItem(pm.statusText, 1, 0, false)

	// Create a form for the cancel button
	form := tview.NewForm()
	form.SetButtonsAlign(tview.AlignCenter)
	form.AddButton("Cancel", func() {
		if pm.onCancel != nil {
			pm.onCancel()
		}
		pm.Close()
	})
	form.SetBackgroundColor(tcell.ColorDefault)
	form.SetButtonBackgroundColor(tcell.ColorDefault)
	form.SetButtonTextColor(tcell.ColorWhite)

	// Create the main flex layout
	innerFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(contentFlex, 3, 0, false).
		AddItem(nil, 1, 0, false).
		AddItem(form, 1, 0, false)

	// Add a border and title with more visible styling
	frame := tview.NewFrame(innerFlex)
	frame.SetBorders(2, 2, 2, 2, 4, 4)
	frame.SetBorderColor(tcell.ColorRed) // Try a more visible color
	frame.SetBackgroundColor(tcell.ColorDefault)
	frame.SetBorderPadding(1, 1, 2, 2)
	frame.AddText(" "+title+" ", true, tview.AlignCenter, tcell.ColorYellow)

	// Create a centered flex
	width := 60
	height := 10

	innerModalFlex := tview.NewFlex()
	innerModalFlex.SetDirection(tview.FlexRow)
	innerModalFlex.SetBackgroundColor(tcell.ColorDefault)
	innerModalFlex.AddItem(nil, 0, 1, false).
		AddItem(frame, height, 1, true).
		AddItem(nil, 0, 1, false)

	pm.modal = tview.NewFlex()
	pm.modal.SetBackgroundColor(tcell.ColorDefault)
	pm.modal.AddItem(nil, 0, 1, false).
		AddItem(innerModalFlex, width, 1, true).
		AddItem(nil, 0, 1, false)

	// Update the initial progress bar
	pm.updateProgressBar()

	return pm
}

// SetCancellable sets whether the progress modal can be cancelled.
// When cancellable, the modal will display a cancel button that,
// when clicked, will call the onCancel callback if set.
//
// Parameters:
//   - cancellable: Whether to enable cancellation
//
// Returns:
//   - The ProgressModal instance for method chaining
func (pm *ProgressModal) SetCancellable(cancellable bool) *ProgressModal {
	pm.isCancellable = cancellable
	return pm
}

// SetOnCancel sets a callback to be called when the cancel button is clicked.
// This function will be executed when the user clicks the cancel button.
//
// Parameters:
//   - callback: The function to call on cancellation
//
// Returns:
//   - The ProgressModal instance for method chaining
func (pm *ProgressModal) SetOnCancel(callback func()) *ProgressModal {
	pm.onCancel = callback
	return pm
}

// SetAutoClose sets whether the progress modal should automatically close when progress reaches 100%.
// When enabled, the modal will automatically close after a short delay when the operation completes.
//
// Parameters:
//   - autoClose: Whether to enable auto-closing
//
// Returns:
//   - The ProgressModal instance for method chaining
func (pm *ProgressModal) SetAutoClose(autoClose bool) *ProgressModal {
	pm.autoClose = autoClose
	return pm
}

// Show displays the progress modal.
// This adds the modal to the pages component and makes it visible.
//
// Returns:
//   - The ProgressModal instance for method chaining
func (pm *ProgressModal) Show() *ProgressModal {
	pm.pages.AddPage(pm.pageName, pm.modal, true, true)
	return pm
}

// Close removes the progress modal.
// This removes the modal from the pages component, hiding it from view.
func (pm *ProgressModal) Close() {
	pm.pages.RemovePage(pm.pageName)
}

// UpdateProgress updates the progress bar.
// This function updates both the visual progress bar and the status text,
// and handles auto-closing if enabled and progress is complete.
//
// Parameters:
//   - progress: The new progress value (0 to maxProgress)
//   - status: The status text to display
//
// Returns:
//   - The ProgressModal instance for method chaining
func (pm *ProgressModal) UpdateProgress(progress int, status string) *ProgressModal {
	if pm.done {
		return pm
	}

	pm.progress = progress
	if pm.progress > pm.maxProgress {
		pm.progress = pm.maxProgress
	}

	// Update the text
	pm.app.QueueUpdateDraw(func() {
		pm.updateProgressBar()
		pm.statusText.SetText(status)

		// Auto-close if we're done
		if pm.progress >= pm.maxProgress && pm.autoClose {
			pm.done = true
			time.AfterFunc(500*time.Millisecond, func() {
				pm.app.QueueUpdateDraw(func() {
					pm.Close()
				})
			})
		}
	})

	return pm
}

// updateProgressBar updates the visual progress bar.
// This internal function renders the progress bar with the current progress value,
// calculating the appropriate number of filled segments and the percentage text.
func (pm *ProgressModal) updateProgressBar() {
	percent := int(float64(pm.progress) / float64(pm.maxProgress) * 100)
	barWidth := 50
	progressChars := int(float64(barWidth) * float64(pm.progress) / float64(pm.maxProgress))

	bar := "[green]"
	for i := 0; i < barWidth; i++ {
		if i < progressChars {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "[white]"

	pm.progressBar.SetText(bar + " " + string(rune(percent)) + "%")
}

// ShowProgressModal creates, configures, and shows a progress modal in one operation.
// This convenience function combines the creation, configuration, and display
// of a progress modal into a single call.
//
// Parameters:
//   - pages: The tview.Pages instance to add the modal to
//   - app: The tview.Application instance for UI updates
//   - title: The title to display at the top of the modal
//   - maxProgress: The maximum value for the progress bar (100%)
//   - cancellable: Whether to enable cancellation
//   - onCancel: The function to call on cancellation
//   - autoClose: Whether to enable auto-closing
//
// Returns:
//   - A configured and displayed ProgressModal instance
func ShowProgressModal(
	pages *tview.Pages,
	app *tview.Application,
	title string,
	maxProgress int,
	cancellable bool,
	onCancel func(),
	autoClose bool,
) *ProgressModal {
	pm := NewProgressModal(pages, app, title, maxProgress)
	pm.SetCancellable(cancellable)
	pm.SetOnCancel(onCancel)
	pm.SetAutoClose(autoClose)
	pm.Show()
	return pm
}
