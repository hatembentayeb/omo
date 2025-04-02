# Plugin Development Guide

This document provides guidelines and best practices for developing plugins for OMO (Oh My Ops) that follow the standard UI patterns.

## Plugin Architecture

All plugins should follow this standard structure:

```
plugins/plugin-name/
  ├── main.go             # Plugin entry point and metadata
  ├── plugin_view.go      # Main plugin view implementation
  ├── plugin_client.go    # Backend client for external services
  └── specialized_views/  # Optional subdirectory for complex plugins
      ├── view1.go
      └── view2.go
```

## Required Plugin Interface

Your plugin must implement the `OhmyopsPlugin` interface:

```go
type OhmyopsPlugin interface {
  Start(*tview.Application) tview.Primitive
  GetMetadata() PluginMetadata
}
```

## Plugin Metadata

The `PluginMetadata` struct should be filled with information about your plugin:

```go
type PluginMetadata struct {
  Name        string    // Name of the plugin
  Version     string    // Version of the plugin
  Description string    // Short description of the plugin
  Author      string    // Author of the plugin
  License     string    // License of the plugin
  Tags        []string  // Tags for categorizing the plugin
  Arch        []string  // Supported architectures
  LastUpdated time.Time // Last update time
  URL         string    // URL to plugin repository or documentation
}
```

## UI Components

### Using the Cores Component

The `ui.Cores` struct provides a consistent UI layout for all plugins. Here's how to use it:

```go
import "omo/ui"

// Create a new Cores instance
cores := ui.NewCores(app, "My Plugin")

// Set table headers
cores.SetTableHeaders([]string{"ID", "Name", "Status"})

// Set refresh callback
cores.SetRefreshCallback(func() ([][]string, error) {
  // Fetch and return updated data
  return fetchData()
})

// Set up action callback to handle keypresses
cores.SetActionCallback(func(action string, payload map[string]interface{}) error {
  // Handle actions
  return nil
})

// Register key bindings
cores.RegisterStandardKeys() // Automatically registers R, ?, ESC
cores.AddKeyBinding("C", "Connect", connectFunction)

// Set row selection callback
cores.SetRowSelectedCallback(func(row int) {
  // Handle row selection
})

// Register handlers to capture key events
cores.RegisterHandlers()
```

### Navigation

Use the navigation system to manage views within your plugin:

```go
// Push a new view onto the stack
cores.PushView("detail-view")

// Pop back to the previous view
cores.PopView()

// Get the current view name
currentView := cores.GetCurrentView()
```

### Error Handling

Use the standardized error handling system:

```go
// Create an error handler
errorHandler := ui.NewErrorHandler(app, pages, cores.Log)

// Handle errors with different severity levels
errorHandler.HandleError(err, ui.ErrorLevelError, "Connection Failed")
```

### Using Standard Modal Components

The UI package provides several standard modal components that you should use for consistency across plugins:

#### Directory Selector Modal

Use this modal to prompt the user to select a directory:

```go
ui.ShowStandardDirectorySelectorModal(
    pages,
    app,
    "Select Directory",
    defaultDir, // can be empty for home directory
    func(directory string, cancelled bool) {
        if !cancelled && directory != "" {
            // Process the selected directory
        }
    }
)
```

#### Progress Modal

Use this modal to show progress during long-running operations:

```go
// Create and show the modal
progressModal := ui.ShowStandardProgressModal(
    pages,
    app,
    "Processing Task",
    "Starting operation...",
    func() {
        // This function is called when the user cancels
        // Perform any cleanup or cancellation logic here
    }
)

// Start your long-running operation in a goroutine
go func() {
    // Update the message as needed
    progressModal.UpdateMessage("Working on step 1...")
    
    // For indeterminate progress, pulse the indicator
    progressModal.PulseIndeterminate()
    
    // Close the modal when done
    defer progressModal.Close()
    
    // Check if cancelled
    if progressModal.IsCancelled() {
        return
    }
    
    // Continue with operation...
}()
```

#### List Selector Modal

Use this modal to let the user select from a list of items:

```go
items := [][]string{
    {"Option 1", "Description 1"},
    {"Option 2", "Description 2"},
    {"Option 3", "Description 3"},
}

ui.ShowStandardListSelectorModal(
    pages,
    app,
    "Select Option",
    items,
    func(index int, value string, cancelled bool) {
        if !cancelled && index >= 0 {
            // Process the selected item
        }
    }
)
```

#### Error Modal

Use this modal to display errors:

```go
ui.ShowStandardErrorModal(
    pages,
    app,
    "Error Title",
    "Error message details go here",
    func() {
        // This function is called when the modal is closed
    }
)
```

#### Info Modal

Use this modal to display information:

```go
ui.ShowStandardInfoModal(
    pages,
    app,
    "Information",
    "Information message goes here",
    func() {
        // This function is called when the modal is closed
    }
)
```

## Standard Key Bindings

All plugins should implement these standard key bindings:

| Key | Function |
|-----|----------|
| R   | Refresh data |
| ?   | Show/toggle help |
| ESC | Navigate back |

## Common View Patterns

Use the `ViewPattern` to create consistent views:

```go
view := ui.InitializeView(ui.ViewPattern{
  App:          app,
  Pages:        pages,
  Title:        "My View",
  TableHeaders: []string{"ID", "Name", "Status"},
  RefreshFunc:  fetchData,
  KeyHandlers:  map[string]string{"C": "Connect", "D": "Delete"},
  SelectedFunc: handleSelection,
})
```

## Testing Your Plugin

Before submitting your plugin:

1. Ensure it implements the required interface
2. Verify all UI elements follow the standard patterns
3. Test navigation between views
4. Verify error handling works correctly
5. Ensure key bindings are consistent with other plugins

## Example Plugin

See the `plugins/redis` or `plugins/kafka` directories for complete examples of well-structured plugins.

## Best Practices

1. Use `ui.Cores` for all main views
2. Follow the standard navigation patterns
3. Use consistent error handling
4. Use standard modal components for user interaction
5. Implement all standard key bindings
6. Keep business logic separate from UI code
7. Run long operations in background goroutines
8. Update the UI using QueueUpdateDraw 

// Example of using ViewFactory with unified key binding approach
// -----------------------------------------------------------
// This is the recommended way to create a view with key bindings:

// 1. First create key mapping and callbacks
keyHandlers := map[string]string{
    "R": "Refresh",
    "C": "Connect",
    "?": "Help",
    "I": "Info",
    "T": "Topics",
}

keyCallbacks := map[string]func(){
    "R": myView.refresh,
    "C": myView.showClusterSelector,
    "?": myView.showHelp,
    "I": myView.showBrokerInfo,
    "T": myView.showTopics,
}

// 2. Use the ViewFactory to create your view, passing both maps
myView.cores = myView.viewFactory.CreateTableView(ui.TableViewConfig{
    Title:          "My View",
    TableHeaders:   []string{"Column1", "Column2", "Column3"},
    RefreshFunc:    myView.refreshData,
    KeyHandlers:    keyHandlers,
    KeyCallbacks:   keyCallbacks,
    SelectedFunc:   myView.handleRowSelection,
    AutoRefresh:    true,
    RefreshSeconds: 30,
})

// The ViewFactory will use the standardized BindViewKeys function
// to ensure consistent key binding across all views 