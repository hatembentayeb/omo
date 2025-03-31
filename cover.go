package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const logo = `[#FF6B00]
 ██████  ██   ██ ███    ███ ██    ██  ██████  ██████  ███████ 
██    ██ ██   ██ ████  ████  ██  ██  ██    ██ ██   ██ ██      
██    ██ ███████ ██ ████ ██   ████   ██    ██ ██████  ███████ 
██    ██ ██   ██ ██  ██  ██    ██    ██    ██ ██           ██ 
 ██████  ██   ██ ██      ██    ██     ██████  ██      ███████ [white]
`
const (
	subtitle    = `[#4CAF50]OhMyOps - Terminal Operations Dashboard[white]`
	welcome     = `[#FFD700]Welcome to OhMyOps![white]`
	description = `[#00BCD4]A powerful terminal-based operations toolkit for DevOps professionals[white]`
	features    = `[#E91E63]• Streamline your DevOps workflows efficiently
• Extend functionality with custom plugins
• Manage your infrastructure with ease[white]`
	quickStart = `[#FFD700]Quick Start Guide[white]`
	commands   = `[#03A9F4]• Browse available plugins in the left sidebar
• Press [#FF9900]'r'[white] to refresh the plugin list
• Press [#FF9900]'a'[white] to access settings
• Select a plugin to activate its features[white]`
)

// Cover returns the cover page
func Cover() tview.Primitive {
	// Create styled logo with glow effect
	logoBox := tview.NewTextView()
	logoBox.SetDynamicColors(true)
	logoBox.SetTextAlign(tview.AlignCenter)

	// Add subtle shadow effect to logo
	logoWithShadow := logo + "\n[#FF8C40]  OMO - Terminal Operations Dashboard  [white]"
	logoBox.SetText(logoWithShadow)

	// Create the info content
	infoContent := welcome + "\n\n" +
		"[#4CAF50]Version:[white] v1.0.0\n" +
		"[#4CAF50]Available Plugins:[white] " + "Discover in sidebar\n\n" +
		description

	// Create a framed info box
	infoBox := tview.NewTextView()
	infoBox.SetDynamicColors(true)
	infoBox.SetTextAlign(tview.AlignCenter)
	infoBox.SetBorder(true)
	infoBox.SetBorderColor(tcell.ColorDarkCyan)
	infoBox.SetTitle(" About ")
	infoBox.SetTitleColor(tcell.ColorOrange)
	infoBox.SetBorderPadding(1, 1, 2, 2)
	infoBox.SetText(infoContent)

	// Create features content
	enhancedFeatures := "[#FFD700]✨ Plugin System[white]\n" +
		"   Extend functionality with powerful plugins\n\n" +
		"[#FFD700]✨ Easy Installation[white]\n" +
		"   Simple command-line installation for plugins\n\n" +
		"[#FFD700]✨ Configuration[white]\n" +
		"   Flexible configuration for all your needs\n\n" +
		"[#FFD700]✨ Getting Started[white]\n" +
		"   " + commands

	// Create a features showcase
	featuresBox := tview.NewTextView()
	featuresBox.SetDynamicColors(true)
	featuresBox.SetTextAlign(tview.AlignLeft)
	featuresBox.SetBorder(true)
	featuresBox.SetBorderColor(tcell.ColorDarkCyan)
	featuresBox.SetTitle(" Features ")
	featuresBox.SetTitleColor(tcell.ColorOrange)
	featuresBox.SetBorderPadding(1, 1, 2, 2)
	featuresBox.SetText(enhancedFeatures)

	// Create a grid layout for better organization
	grid := tview.NewGrid()
	grid.SetColumns(0, 40, 40, 0)
	grid.SetRows(0, 12, 0, 15, 0)
	grid.SetBorders(false)

	// Add items to the grid with proper positioning
	grid.AddItem(logoBox, 1, 1, 1, 2, 0, 0, true)
	grid.AddItem(infoBox, 3, 1, 1, 1, 0, 0, false)
	grid.AddItem(featuresBox, 3, 2, 1, 1, 0, 0, false)

	// Create a frame around everything for a polished look
	frame := tview.NewFrame(grid)
	frame.SetBorders(0, 0, 0, 0, 0, 0)
	frame.AddText("Press Tab to navigate", true, tview.AlignCenter, tcell.ColorDimGray)

	app.SetFocus(logoBox)
	return frame
}

func Center(width, height int, p tview.Primitive) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}
