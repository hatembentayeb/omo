package ui

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ProgressModal holds the state for a progress modal
type ProgressModal struct {
	pages         *tview.Pages
	app           *tview.Application
	modal         *tview.Flex
	progressBar   *tview.TextView
	statusText    *tview.TextView
	progress      int
	maxProgress   int
	pageName      string
	onCancel      func()
	isCancellable bool
	autoClose     bool
	done          bool
}

// NewProgressModal creates a new progress modal with the given configuration
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
	contentFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(pm.progressBar, 1, 0, false).
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
	form.SetBackgroundColor(tcell.ColorBlack)
	form.SetButtonBackgroundColor(tcell.ColorBlack)

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
	frame.SetBackgroundColor(tcell.ColorBlack)
	frame.SetBorderPadding(1, 1, 2, 2)
	frame.AddText(" "+title+" ", true, tview.AlignCenter, tcell.ColorYellow)

	// Create a centered flex
	width := 60
	height := 10

	pm.modal = tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(frame, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	// Update the initial progress bar
	pm.updateProgressBar()

	return pm
}

// SetCancellable sets whether the progress modal can be cancelled
func (pm *ProgressModal) SetCancellable(cancellable bool) *ProgressModal {
	pm.isCancellable = cancellable
	return pm
}

// SetOnCancel sets a callback to be called when the cancel button is clicked
func (pm *ProgressModal) SetOnCancel(callback func()) *ProgressModal {
	pm.onCancel = callback
	return pm
}

// SetAutoClose sets whether the progress modal should automatically close when progress reaches 100%
func (pm *ProgressModal) SetAutoClose(autoClose bool) *ProgressModal {
	pm.autoClose = autoClose
	return pm
}

// Show displays the progress modal
func (pm *ProgressModal) Show() *ProgressModal {
	pm.pages.AddPage(pm.pageName, pm.modal, true, true)
	return pm
}

// Close removes the progress modal
func (pm *ProgressModal) Close() {
	pm.pages.RemovePage(pm.pageName)
}

// UpdateProgress updates the progress bar
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

// updateProgressBar updates the visual progress bar
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

// ShowProgressModal creates, configures, and shows a progress modal in one operation
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
