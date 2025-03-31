package ui

import (
	"fmt"
	"reflect"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// PluginHeaderView is a custom component for displaying plugin information
type PluginHeaderView struct {
	*tview.Box
	pluginName       string
	metadata         interface{} // Generic metadata interface
	hasMetadata      bool
	metadataProvider func(string) (interface{}, bool) // Function to get metadata
}

// NewPluginHeaderView creates a new custom plugin header view
func NewPluginHeaderView(metadataProvider func(string) (interface{}, bool)) *PluginHeaderView {
	return &PluginHeaderView{
		Box:              tview.NewBox().SetBorder(false),
		hasMetadata:      false,
		metadataProvider: metadataProvider,
	}
}

// SetPluginInfo sets the plugin information to display
func (v *PluginHeaderView) SetPluginInfo(pluginName string) *PluginHeaderView {
	v.pluginName = pluginName

	// Check if we have metadata for this plugin
	if v.metadataProvider != nil {
		if metadata, exists := v.metadataProvider(pluginName); exists {
			v.metadata = metadata
			v.hasMetadata = true
		} else {
			v.hasMetadata = false
		}
	}

	return v
}

// Draw draws this primitive onto the screen.
func (v *PluginHeaderView) Draw(screen tcell.Screen) {
	v.Box.Draw(screen)

	x, y, width, height := v.GetInnerRect()

	// Background styling
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {
			screen.SetContent(x+col, y+row, ' ', nil, tcell.StyleDefault.Background(tcell.ColorBlack))
		}
	}

	// Default app values
	appName := "OhMyOps"
	appVersion := "v1.0.0"
	appAuthor := "HATMAN"
	appStatus := "Up to date"
	appUpdated := time.Now().Format("Jan 2006")

	// Prepare display values
	name := appName
	version := appVersion
	displayAuthor := appAuthor
	status := appStatus
	updated := appUpdated

	// Determine what to display based on plugin selection
	if v.pluginName != "" {
		if v.hasMetadata {
			// Extract values from metadata - use type assertions
			// This is a more flexible approach that doesn't require tight coupling
			if m, ok := v.metadata.(map[string]interface{}); ok {
				// Try to extract from map
				if n, ok := m["Name"].(string); ok {
					name = n
				}
				if ver, ok := m["Version"].(string); ok {
					version = ver
				}
				if author, ok := m["Author"].(string); ok {
					displayAuthor = author
				}
				// Use string for updated date if available
				if updatedValue, ok := m["LastUpdated"].(string); ok {
					updated = updatedValue
				}
			} else {
				// Try to access using reflection
				name = getValueAsString(v.metadata, "Name", name)
				version = getValueAsString(v.metadata, "Version", version)
				displayAuthor = getValueAsString(v.metadata, "Author", displayAuthor)
				updated = getValueAsString(v.metadata, "LastUpdated", updated)
			}
		} else {
			// Show plugin name but default values
			name = v.pluginName
		}
	} else {
		// No plugin selected - show main app info
		// Values already set to app defaults
	}

	// Draw the header with custom styling
	labelStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite)
	valueStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow)
	statusStyle := tcell.StyleDefault.Foreground(tcell.ColorGreen)

	// Layout constants
	labelCol := x + 2
	valueCol := x + 12
	rowSpacing := 2

	// Row 1 - Name
	drawText(screen, labelCol, y+1, "Name:", labelStyle)
	drawText(screen, valueCol, y+1, name, valueStyle)

	// Row 2 - Version
	drawText(screen, labelCol, y+1+rowSpacing, "Version:", labelStyle)
	drawText(screen, valueCol, y+1+rowSpacing, version, valueStyle)

	// Row 3 - Author
	drawText(screen, labelCol, y+1+rowSpacing*2, "Author:", labelStyle)
	drawText(screen, valueCol, y+1+rowSpacing*2, displayAuthor, valueStyle)

	// Row 4 - Status
	drawText(screen, labelCol, y+1+rowSpacing*3, "Status:", labelStyle)
	drawText(screen, valueCol, y+1+rowSpacing*3, status, statusStyle)

	// Row 5 - Updated
	drawText(screen, labelCol, y+1+rowSpacing*4, "Updated:", labelStyle)
	drawText(screen, valueCol, y+1+rowSpacing*4, updated, valueStyle)
}

// Helper function to get string values from metadata
func getValueAsString(obj interface{}, fieldName string, defaultValue string) string {
	// Simple reflection-based field access
	value := reflect.ValueOf(obj)

	// Handle pointer types
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	// Only proceed if we have a struct
	if value.Kind() != reflect.Struct {
		return defaultValue
	}

	// Try to access the field
	field := value.FieldByName(fieldName)
	if !field.IsValid() {
		return defaultValue
	}

	// Handle different types of fields
	switch field.Kind() {
	case reflect.String:
		return field.String()
	case reflect.Struct:
		// Special case for time.Time
		if fieldName == "LastUpdated" {
			if t, ok := field.Interface().(time.Time); ok {
				return t.Format("Jan 2006")
			}
		}
		return defaultValue
	default:
		return fmt.Sprintf("%v", field.Interface())
	}
}

// Utility function to draw text on the screen
func drawText(screen tcell.Screen, x, y int, text string, style tcell.Style) {
	for i, r := range text {
		screen.SetContent(x+i, y, r, nil, style)
	}
}
