package ui

import (
	"fmt"
	"log"
	"time"

	"github.com/rivo/tview"
)

// ErrorLevel defines the severity level of an error
type ErrorLevel int

const (
	// ErrorLevelInfo represents an informational message
	ErrorLevelInfo ErrorLevel = iota
	// ErrorLevelWarning represents a warning message
	ErrorLevelWarning
	// ErrorLevelError represents an error message
	ErrorLevelError
	// ErrorLevelFatal represents a fatal error message
	ErrorLevelFatal
)

// ErrorHandler provides a centralized way to handle and display errors
type ErrorHandler struct {
	app         *tview.Application
	pages       *tview.Pages
	logFunc     func(message string)
	errorLogger *log.Logger
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(app *tview.Application, pages *tview.Pages, logFunc func(message string)) *ErrorHandler {
	return &ErrorHandler{
		app:     app,
		pages:   pages,
		logFunc: logFunc,
	}
}

// HandleError processes an error and displays it appropriately based on its level
func (h *ErrorHandler) HandleError(err error, level ErrorLevel, title string) {
	if err == nil {
		return
	}

	// Log the error to the log panel if a log function is provided
	if h.logFunc != nil {
		switch level {
		case ErrorLevelInfo:
			h.logFunc(fmt.Sprintf("[blue]INFO[white] %s", err.Error()))
		case ErrorLevelWarning:
			h.logFunc(fmt.Sprintf("[yellow]WARN[white] %s", err.Error()))
		case ErrorLevelError:
			h.logFunc(fmt.Sprintf("[red]ERROR[white] %s", err.Error()))
		case ErrorLevelFatal:
			h.logFunc(fmt.Sprintf("[red::b]FATAL[white::-] %s", err.Error()))
		}
	}

	// Log to the error logger if available
	if h.errorLogger != nil {
		h.errorLogger.Printf("[%s] %s", levelToString(level), err.Error())
	}

	// For errors and fatal errors, show a modal
	if level >= ErrorLevelError {
		if title == "" {
			title = "Error"
		}

		// Use the standard error modal
		ShowStandardErrorModal(
			h.pages,
			h.app,
			title,
			err.Error(),
			func() {
				// Auto-dismiss after a timeout for non-fatal errors
				if level < ErrorLevelFatal {
					time.AfterFunc(5*time.Second, func() {
						h.app.QueueUpdateDraw(func() {
							if h.pages.HasPage("error-modal") {
								h.pages.RemovePage("error-modal")
							}
						})
					})
				}
			},
		)
	}
}

// levelToString converts an error level to its string representation
func levelToString(level ErrorLevel) string {
	switch level {
	case ErrorLevelInfo:
		return "INFO"
	case ErrorLevelWarning:
		return "WARN"
	case ErrorLevelError:
		return "ERROR"
	case ErrorLevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// HandleErrorWithCallback processes an error and calls the provided callback if needed
func (h *ErrorHandler) HandleErrorWithCallback(
	err error,
	level ErrorLevel,
	title string,
	callback func(),
) {
	if err == nil {
		if callback != nil {
			callback()
		}
		return
	}

	// Handle the error first
	h.HandleError(err, level, title)

	// For non-fatal errors, still call the callback
	if level < ErrorLevelFatal && callback != nil {
		callback()
	}
}

// SetErrorLogger sets an external logger for error logging
func (h *ErrorHandler) SetErrorLogger(logger *log.Logger) {
	h.errorLogger = logger
}
