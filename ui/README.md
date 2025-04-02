# UI Framework Documentation

This document provides an overview of the UI framework and the recent improvements made to standardize the plugin development experience.

## Recent Improvements

We have implemented five key improvements to the UI framework:

### 1. Standardized Key Handling

A new `StandardKeyHandler` has been added in `keyhandler.go` to provide consistent key handling across all plugins. This ensures that key bindings like 'R' for refresh, '?' for help, and 'ESC' for back navigation work consistently in all plugins.

The new helper functions include:
- `RegisterStandardKeys()` - Adds common keybindings that should be consistent across plugins
- `StandardKeyHandler()` - Provides a standardized way to handle key events
- `ToggleHelpExpanded()` - Consistently toggles between basic and expanded help views

### 2. Reduced Duplication with Common View Patterns

The new `viewpatterns.go` file introduces utilities to reduce code duplication across plugins:
- `ViewPattern` struct for encapsulating common view configuration
- `InitializeView()` function for standardized view creation
- Helper functions for standardizing table formatting and row selection

### 3. Unified Error Handling

A new standardized error handling system has been added in `errorhandler.go`:
- `ErrorLevel` type with Info, Warning, Error, and Fatal severity levels
- `ErrorHandler` struct for centralized error management
- `ShowStandardErrorModal()` function for consistent error presentation
- Auto-dismissing errors with appropriate timeouts

### 4. Improved Documentation

New documentation has been added to help developers create consistent plugins:
- `PLUGIN_GUIDE.md` with detailed plugin development guidelines
- Examples of proper plugin structure
- Best practices for UI development
- Instructions for using the standardized components

### 5. View Component Factory

A new `ViewFactory` has been added in `viewfactory.go` to simplify creation of common view types:
- `CreateTableView()` - Creates standard table views
- `CreateSplitView()` - Creates master-detail split views
- `CreateDetailView()` - Creates detailed item views
- Configuration structs for each view type

### 6. Standard Modal Components

New standard modal components have been added for common UI interactions:
- `ShowStandardDirectorySelectorModal()` - For selecting directories
- `ShowStandardProgressModal()` - For displaying progress during long operations
- `ShowStandardListSelectorModal()` - For selecting from a list of items
- `ShowStandardErrorModal()` - For displaying errors
- `ShowStandardConfirmationModal()` - For confirming actions
- `ShowStandardInfoModal()` - For displaying information

## Directory Structure

```
ui/
  ├── cores.go            # Core UI component definition
  ├── directorySelector.go # Directory selector modal
  ├── errorhandler.go     # Unified error handling
  ├── errorModal.go       # Standard error modal components
  ├── handlers.go         # Input handlers
  ├── help.go             # Help text functionality
  ├── info.go             # Info panel functionality
  ├── init_ui.go          # UI initialization
  ├── keyhandler.go       # Standardized key handling
  ├── keyBindings.go      # Key binding management
  ├── listSelectorModal.go # List selector modal
  ├── navigation.go       # Navigation stack management
  ├── PLUGIN_GUIDE.md     # Detailed plugin development guide
  ├── progressModal.go    # Progress modal for long operations
  ├── refresh.go          # Data refresh functionality
  ├── README.md           # This documentation
  ├── table.go            # Table management
  ├── viewfactory.go      # View component factory
  └── viewpatterns.go     # Common view patterns
```

## How to Use

### Creating a Plugin with the View Factory

```go
// Import the UI package
import "omo/ui"

// Create a view factory
factory := ui.NewViewFactory(app, pages)

// Create a standard table view
view := factory.CreateTableView(ui.TableViewConfig{
    Title:          "My Plugin",
    TableHeaders:   []string{"ID", "Name", "Status"},
    RefreshFunc:    fetchData,
    KeyHandlers:    map[string]string{"C": "Connect", "D": "Delete"},
    SelectedFunc:   handleSelection,
    AutoRefresh:    true,
    RefreshSeconds: 30,
})
```

### Using the Error Handler

```go
// Create an error handler
errorHandler := ui.NewErrorHandler(app, pages, view.Log)

// Handle different error levels
errorHandler.HandleError(err, ui.ErrorLevelInfo, "Information")
errorHandler.HandleError(err, ui.ErrorLevelWarning, "Warning")
errorHandler.HandleError(err, ui.ErrorLevelError, "Error")
errorHandler.HandleError(err, ui.ErrorLevelFatal, "Fatal Error")
```

### Using Standard Modals

```go
// Directory Selector Modal
ui.ShowStandardDirectorySelectorModal(
    pages,
    app,
    "Select Directory",
    homeDir,
    func(directory string, cancelled bool) {
        if !cancelled {
            // Process the selected directory
        }
    }
)

// Progress Modal
progressModal := ui.ShowStandardProgressModal(
    pages,
    app,
    "Processing",
    "Working on task...",
    func() {
        // Handle cancel
    }
)

// Update the progress modal
progressModal.UpdateMessage("Almost done...")
progressModal.PulseIndeterminate()

// When complete
progressModal.Close()

// List Selector Modal
ui.ShowStandardListSelectorModal(
    pages,
    app,
    "Select Item",
    items,
    func(index int, value string, cancelled bool) {
        if !cancelled {
            // Process the selected item
        }
    }
)
```

See the `PLUGIN_GUIDE.md` for more detailed usage examples and best practices. 