// Package ui provides terminal UI components for building consistent
// terminal applications with a unified interface.
package ui

import (
	"fmt"
	"log"
	"time"

	"github.com/rivo/tview"
)

// ErrorLevel defines the severity level of an error.
// The levels are arranged in increasing order of severity,
// allowing for appropriate handling and display based on importance.
type ErrorLevel int

const (
	// ErrorLevelInfo represents an informational message.
	// Information messages are typically non-critical notifications.
	ErrorLevelInfo ErrorLevel = iota

	// ErrorLevelWarning represents a warning message.
	// Warnings indicate potential issues that don't prevent operation.
	ErrorLevelWarning

	// ErrorLevelError represents an error message.
	// Errors indicate issues that may disrupt normal operation.
	ErrorLevelError

	// ErrorLevelFatal represents a fatal error message.
	// Fatal errors indicate critical issues that prevent continued operation.
	ErrorLevelFatal
)

// ErrorHandler provides a centralized way to handle and display errors
// across the application. It coordinates error logging, user notifications,
// and appropriate UI responses based on error severity.
type ErrorHandler struct {
	app         *tview.Application   // Reference to the main application
	pages       *tview.Pages         // Pages component for showing modals
	logFunc     func(message string) // Function to log messages to the UI log panel
	errorLogger *log.Logger          // Optional external logger
}

// NewErrorHandler creates a new error handler.
// This factory function initializes an ErrorHandler with the necessary
// components for displaying and logging errors.
//
// Parameters:
//   - app: The tview application instance
//   - pages: The pages component for displaying modal dialogs
//   - logFunc: A function that logs messages to the UI log panel
//
// Returns:
//   - A new ErrorHandler instance
func NewErrorHandler(app *tview.Application, pages *tview.Pages, logFunc func(message string)) *ErrorHandler {
	return &ErrorHandler{
		app:     app,
		pages:   pages,
		logFunc: logFunc,
	}
}

// HandleError processes an error and displays it appropriately based on its level.
// This method performs the following actions based on error severity:
// - Logs the error to the UI log panel with appropriate formatting
// - Logs to an external logger if configured
// - Displays an error modal for errors of level Error or Fatal
//
// Parameters:
//   - err: The error to handle
//   - level: The severity level of the error
//   - title: The title for the error modal (uses "Error" if empty)
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

// levelToString converts an error level to its string representation.
// This helper function provides standardized text labels for each error level.
//
// Parameters:
//   - level: The ErrorLevel to convert
//
// Returns:
//   - The string representation of the error level
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

// HandleErrorWithCallback processes an error and calls the provided callback if needed.
// This method extends HandleError by adding a callback function that will be called:
// - Immediately if there is no error
// - After handling the error for non-fatal errors
// - Not called at all for fatal errors
//
// Parameters:
//   - err: The error to handle
//   - level: The severity level of the error
//   - title: The title for the error modal
//   - callback: Function to call after handling non-fatal errors
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

// SetErrorLogger sets an external logger for error logging.
// This allows integration with application-wide logging systems.
//
// Parameters:
//   - logger: The log.Logger instance to use for logging
func (h *ErrorHandler) SetErrorLogger(logger *log.Logger) {
	h.errorLogger = logger
}
