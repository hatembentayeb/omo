package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"omo/ui"
)

// DockerEntity provides a common interface for different Docker entities
type DockerEntity interface {
	GetTableRow() []string
}

// DockerContainer represents a Docker container
type DockerContainer struct {
	ID            string
	Name          string
	Image         string
	Status        string
	Created       time.Time
	Ports         []string
	State         string
	NetworkMode   string
	RestartPolicy string
	Labels        map[string]string
}

// GetTableRow returns container data as a table row
func (c *DockerContainer) GetTableRow() []string {
	return []string{
		c.ID[:12],                   // Short ID
		c.Name,                      // Container name
		c.Image,                     // Image name
		c.State,                     // Running state
		c.Status,                    // Status with runtime
		strings.Join(c.Ports, ", "), // Ports
	}
}

// DockerImage represents a Docker image
type DockerImage struct {
	ID           string
	Repository   string
	Tag          string
	Size         string
	Created      time.Time
	CreatedSince string
	Digest       string
}

// GetTableRow returns image data as a table row
func (i *DockerImage) GetTableRow() []string {
	return []string{
		i.ID[:12],      // Short ID
		i.Repository,   // Repository name
		i.Tag,          // Tag
		i.Size,         // Size (formatted)
		i.CreatedSince, // Relative creation time
	}
}

// DockerNetwork represents a Docker network
type DockerNetwork struct {
	ID      string
	Name    string
	Driver  string
	Scope   string
	Created time.Time
	Subnet  string
	Gateway string
}

// GetTableRow returns network data as a table row
func (n *DockerNetwork) GetTableRow() []string {
	return []string{
		n.ID[:12], // Short ID
		n.Name,    // Network name
		n.Driver,  // Network driver
		n.Scope,   // Network scope
		n.Subnet,  // Subnet
	}
}

// DockerVolume represents a Docker volume
type DockerVolume struct {
	Name       string
	Driver     string
	Mountpoint string
	Created    time.Time
	Labels     map[string]string
	Size       string
}

// GetTableRow returns volume data as a table row
func (v *DockerVolume) GetTableRow() []string {
	return []string{
		v.Name,       // Volume name
		v.Driver,     // Volume driver
		v.Mountpoint, // Mount point
		v.Size,       // Size (if available)
	}
}

// ViewType represents which Docker resource is being viewed
type ViewType int

const (
	// ViewContainers shows Docker containers
	ViewContainers ViewType = iota
	// ViewImages shows Docker images
	ViewImages
	// ViewNetworks shows Docker networks
	ViewNetworks
	// ViewVolumes shows Docker volumes
	ViewVolumes
)

// HelpSection represents a section of help content
type HelpSection struct {
	Title string
	Items []HelpItem
}

// HelpItem represents a single help item with key binding and description
type HelpItem struct {
	Key         string
	Description string
}

// DockerView manages the UI for viewing Docker resources
type DockerView struct {
	app          *tview.Application
	pages        *tview.Pages
	cores        *ui.Cores
	errorHandler *ui.ErrorHandler
	refreshTimer *time.Timer
	currentView  ViewType
	dockerClient *DockerClient
	viewFactory  *ui.ViewFactory
}

// NewDockerView creates a new Docker view
func NewDockerView(app *tview.Application, pages *tview.Pages) *DockerView {
	dv := &DockerView{
		app:         app,
		pages:       pages,
		currentView: ViewContainers, // Default view is containers
	}

	// Create error handler
	dv.errorHandler = ui.NewErrorHandler(app, pages, func(message string) {
		if dv.cores != nil {
			dv.cores.Log(message)
		}
	})

	// Create view factory
	dv.viewFactory = ui.NewViewFactory(app, pages)

	// Define key handlers
	keyHandlers := map[string]string{
		"R": "Refresh",
		"C": "View Containers",
		"I": "View Images",
		"N": "View Networks",
		"V": "View Volumes",
		"S": "Start Container",
		"X": "Stop Container",
		"D": "Remove Container",
		"L": "View Logs",
		"E": "Execute Command",
		"P": "Prune System",
		"?": "Help",
	}

	// Create Cores UI component using the view factory
	dv.cores = dv.viewFactory.CreateTableView(ui.TableViewConfig{
		Title: "Docker Containers",
		TableHeaders: []string{
			"ID", "Name", "Image", "State", "Status", "Ports",
		},
		RefreshFunc:    dv.refreshContainers,
		KeyHandlers:    keyHandlers,
		SelectedFunc:   nil,
		AutoRefresh:    false,
		RefreshSeconds: 0,
	})

	// Get the table directly and set up selection handler
	table := dv.cores.GetTable()
	table.SetupSelection()

	// Initialize Docker client with logger function
	dv.dockerClient = NewDockerClient()
	dv.dockerClient.SetLogger(func(message string) {
		dv.cores.Log(message)
	})

	// Register additional key handlers
	dv.cores.SetActionCallback(func(action string, payload map[string]interface{}) error {
		if action == "keypress" {
			if key, ok := payload["key"].(string); ok {
				switch key {
				case "C":
					dv.switchView(ViewContainers)
					return nil
				case "I":
					dv.switchView(ViewImages)
					return nil
				case "N":
					dv.switchView(ViewNetworks)
					return nil
				case "V":
					dv.switchView(ViewVolumes)
					return nil
				case "S":
					dv.startContainer()
					return nil
				case "X":
					dv.stopContainer()
					return nil
				case "D":
					dv.removeContainer()
					return nil
				case "L":
					dv.viewContainerLogs()
					return nil
				case "E":
					dv.executeCommand()
					return nil
				case "P":
					dv.pruneSystem()
					return nil
				case "?":
					dv.showHelpModal()
					return nil
				}
			}
		}
		return nil
	})

	return dv
}

// GetMainUI returns the main UI component
func (dv *DockerView) GetMainUI() tview.Primitive {
	return dv.cores.GetLayout()
}

// showLoadingIndicator displays a loading indicator in the log panel
func (dv *DockerView) showLoadingIndicator(message string) {
	dv.cores.Log(fmt.Sprintf("[yellow]⏳ %s...", message))
}

// hideLoadingIndicator removes the loading indicator
func (dv *DockerView) hideLoadingIndicator(message string) {
	dv.cores.Log(fmt.Sprintf("[green]✓ %s", message))
}

// RefreshAll refreshes data for the current view
func (dv *DockerView) RefreshAll() {
	var rows [][]string
	var err error

	// Get the data by calling the appropriate refresh function
	switch dv.currentView {
	case ViewContainers:
		rows, err = dv.refreshContainers()
	case ViewImages:
		rows, err = dv.refreshImages()
	case ViewNetworks:
		rows, err = dv.refreshNetworks()
	case ViewVolumes:
		rows, err = dv.refreshVolumes()
	}

	// Handle results
	if err != nil {
		dv.cores.Log(fmt.Sprintf("[red]Error refreshing view: %v", err))
	} else if rows != nil {
		// Update the table directly
		dv.cores.SetTableData(rows)
	}
}

// switchView changes the current view type and refreshes data
func (dv *DockerView) switchView(viewType ViewType) {
	// Update the current view type
	dv.currentView = viewType

	// Update title and headers based on the view type
	switch viewType {
	case ViewContainers:
		dv.cores.GetTable().SetTitle(fmt.Sprintf(" [yellow]%s[white] ", "Docker Containers"))
		dv.cores.SetTableHeaders([]string{"ID", "Name", "Image", "State", "Status", "Ports"})
		dv.cores.SetRefreshCallback(dv.refreshContainers)
		dv.cores.Log("[blue]Switched to Containers view")
	case ViewImages:
		dv.cores.GetTable().SetTitle(fmt.Sprintf(" [yellow]%s[white] ", "Docker Images"))
		dv.cores.SetTableHeaders([]string{"ID", "Repository", "Tag", "Size", "Created"})
		dv.cores.SetRefreshCallback(dv.refreshImages)
		dv.cores.Log("[blue]Switched to Images view")
	case ViewNetworks:
		dv.cores.GetTable().SetTitle(fmt.Sprintf(" [yellow]%s[white] ", "Docker Networks"))
		dv.cores.SetTableHeaders([]string{"ID", "Name", "Driver", "Scope", "Subnet"})
		dv.cores.SetRefreshCallback(dv.refreshNetworks)
		dv.cores.Log("[blue]Switched to Networks view")
	case ViewVolumes:
		dv.cores.GetTable().SetTitle(fmt.Sprintf(" [yellow]%s[white] ", "Docker Volumes"))
		dv.cores.SetTableHeaders([]string{"Name", "Driver", "Mountpoint", "Size"})
		dv.cores.SetRefreshCallback(dv.refreshVolumes)
		dv.cores.Log("[blue]Switched to Volumes view")
	}

	// Force an immediate refresh of the data
	var rows [][]string
	var err error

	switch viewType {
	case ViewContainers:
		rows, err = dv.refreshContainers()
	case ViewImages:
		rows, err = dv.refreshImages()
	case ViewNetworks:
		rows, err = dv.refreshNetworks()
	case ViewVolumes:
		rows, err = dv.refreshVolumes()
	}

	if err != nil {
		dv.cores.Log(fmt.Sprintf("[red]Error refreshing data: %v", err))
	} else if rows != nil {
		// Use SetTableData from the Cores UI
		dv.cores.SetTableData(rows)
	}
}

// refreshContainers fetches and displays Docker containers
func (dv *DockerView) refreshContainers() ([][]string, error) {
	dv.showLoadingIndicator("Loading Docker containers")

	// Get containers directly first - this needs to be synchronous
	containers, err := dv.dockerClient.ListContainers()
	if err != nil {
		dv.hideLoadingIndicator("Error loading containers")
		dv.cores.Log(fmt.Sprintf("[red]Docker error: %v", err))
		return nil, err
	}

	// Start the progress indicator in a separate goroutine
	go func() {
		// Small delay to prevent too many log messages at once
		time.Sleep(500 * time.Millisecond)
		if len(containers) > 0 {
			dv.cores.Log(fmt.Sprintf("[yellow]⏳ Processing %d containers...", len(containers)))
		}
	}()

	// Convert to table rows immediately
	rows := make([][]string, len(containers))
	for i, container := range containers {
		rows[i] = container.GetTableRow()
	}

	if len(containers) == 0 {
		dv.cores.Log("[yellow]No containers found. Docker may not be running or no containers exist.")
	} else {
		dv.cores.Log(fmt.Sprintf("[green]✓ Loaded %d containers", len(containers)))
	}

	return rows, nil
}

// refreshImages fetches and displays Docker images
func (dv *DockerView) refreshImages() ([][]string, error) {
	dv.showLoadingIndicator("Loading Docker images")

	// Get images directly
	images, err := dv.dockerClient.ListImages()
	if err != nil {
		dv.hideLoadingIndicator("Error loading images")
		dv.cores.Log(fmt.Sprintf("[red]Docker error: %v", err))
		return nil, err
	}

	// Convert to table rows
	rows := make([][]string, len(images))
	for i, image := range images {
		rows[i] = image.GetTableRow()
	}

	if len(images) == 0 {
		dv.cores.Log("[yellow]No images found. Docker may not be running or no images exist.")
	} else {
		dv.cores.Log(fmt.Sprintf("[green]✓ Loaded %d images", len(images)))
	}

	return rows, nil
}

// refreshNetworks fetches and displays Docker networks
func (dv *DockerView) refreshNetworks() ([][]string, error) {
	dv.showLoadingIndicator("Loading Docker networks")

	// Get networks directly
	networks, err := dv.dockerClient.ListNetworks()
	if err != nil {
		dv.hideLoadingIndicator("Error loading networks")
		dv.cores.Log(fmt.Sprintf("[red]Docker error: %v", err))
		return nil, err
	}

	// Convert to table rows
	rows := make([][]string, len(networks))
	for i, network := range networks {
		rows[i] = network.GetTableRow()
	}

	if len(networks) == 0 {
		dv.cores.Log("[yellow]No networks found. Docker may not be running or no networks exist.")
	} else {
		dv.cores.Log(fmt.Sprintf("[green]✓ Loaded %d networks", len(networks)))
	}

	return rows, nil
}

// refreshVolumes fetches and displays Docker volumes
func (dv *DockerView) refreshVolumes() ([][]string, error) {
	dv.showLoadingIndicator("Loading Docker volumes")

	// Get volumes directly
	volumes, err := dv.dockerClient.ListVolumes()
	if err != nil {
		dv.hideLoadingIndicator("Error loading volumes")
		dv.cores.Log(fmt.Sprintf("[red]Docker error: %v", err))
		return nil, err
	}

	// Convert to table rows
	rows := make([][]string, len(volumes))
	for i, volume := range volumes {
		rows[i] = volume.GetTableRow()
	}

	if len(volumes) == 0 {
		dv.cores.Log("[yellow]No volumes found. Docker may not be running or no volumes exist.")
	} else {
		dv.cores.Log(fmt.Sprintf("[green]✓ Loaded %d volumes", len(volumes)))
	}

	return rows, nil
}

// getSelectedContainer gets the currently selected container
func (dv *DockerView) getSelectedContainer() (*DockerContainer, error) {
	if dv.currentView != ViewContainers {
		return nil, fmt.Errorf("not in container view")
	}

	// Get selected row index
	table := dv.cores.GetTable()
	row := table.GetSelectedRow()
	if row < 0 {
		return nil, fmt.Errorf("no container selected")
	}

	// Get containers from client
	containers, err := dv.dockerClient.ListContainers()
	if err != nil {
		return nil, err
	}

	// Check if the row is valid
	if row >= len(containers) {
		return nil, fmt.Errorf("invalid container selection")
	}

	return &containers[row], nil
}

// startContainer starts the selected container
func (dv *DockerView) startContainer() {
	container, err := dv.getSelectedContainer()
	if err != nil {
		dv.errorHandler.HandleError(err, ui.ErrorLevelWarning, "Container Selection")
		return
	}

	dv.showLoadingIndicator(fmt.Sprintf("Starting container %s", container.Name))

	err = dv.dockerClient.StartContainer(container.ID)
	if err != nil {
		dv.errorHandler.HandleError(err, ui.ErrorLevelError, "Start Container")
		return
	}

	dv.hideLoadingIndicator(fmt.Sprintf("Container %s started", container.Name))
	dv.RefreshAll()
}

// stopContainer stops the selected container
func (dv *DockerView) stopContainer() {
	container, err := dv.getSelectedContainer()
	if err != nil {
		dv.errorHandler.HandleError(err, ui.ErrorLevelWarning, "Container Selection")
		return
	}

	dv.showLoadingIndicator(fmt.Sprintf("Stopping container %s", container.Name))

	err = dv.dockerClient.StopContainer(container.ID)
	if err != nil {
		dv.errorHandler.HandleError(err, ui.ErrorLevelError, "Stop Container")
		return
	}

	dv.hideLoadingIndicator(fmt.Sprintf("Container %s stopped", container.Name))
	dv.RefreshAll()
}

// removeContainer removes the selected container
func (dv *DockerView) removeContainer() {
	container, err := dv.getSelectedContainer()
	if err != nil {
		dv.errorHandler.HandleError(err, ui.ErrorLevelWarning, "Container Selection")
		return
	}

	// Show confirmation modal
	dv.showConfirmationModal(
		"Remove Container",
		fmt.Sprintf("Are you sure you want to remove container %s (%s)?", container.Name, container.ID[:12]),
		func(confirmed bool) {
			if !confirmed {
				return
			}

			dv.showLoadingIndicator(fmt.Sprintf("Removing container %s", container.Name))

			err := dv.dockerClient.RemoveContainer(container.ID)
			if err != nil {
				dv.errorHandler.HandleError(err, ui.ErrorLevelError, "Remove Container")
				return
			}

			dv.hideLoadingIndicator(fmt.Sprintf("Container %s removed", container.Name))
			dv.RefreshAll()
		},
	)
}

// viewContainerLogs shows logs for the selected container
func (dv *DockerView) viewContainerLogs() {
	container, err := dv.getSelectedContainer()
	if err != nil {
		dv.errorHandler.HandleError(err, ui.ErrorLevelWarning, "Container Selection")
		return
	}

	dv.showLoadingIndicator(fmt.Sprintf("Fetching logs for container %s", container.Name))

	logs, err := dv.dockerClient.GetContainerLogs(container.ID)
	if err != nil {
		dv.errorHandler.HandleError(err, ui.ErrorLevelError, "Container Logs")
		return
	}

	dv.hideLoadingIndicator(fmt.Sprintf("Logs fetched for %s", container.Name))

	// Show logs in a modal
	dv.showLogsModal(container.Name, logs)
}

// executeCommand executes a command in the selected container
func (dv *DockerView) executeCommand() {
	container, err := dv.getSelectedContainer()
	if err != nil {
		dv.errorHandler.HandleError(err, ui.ErrorLevelWarning, "Container Selection")
		return
	}

	// Show command input modal
	dv.showCompactInputModal(
		"Execute Command",
		"Enter command to execute in container "+container.Name+":",
		"sh",
		func(command string, cancelled bool) {
			if cancelled || command == "" {
				return
			}

			dv.showLoadingIndicator(fmt.Sprintf("Executing '%s' in container %s", command, container.Name))

			output, err := dv.dockerClient.ExecInContainer(container.ID, command)
			if err != nil {
				dv.errorHandler.HandleError(err, ui.ErrorLevelError, "Command Execution")
				return
			}

			dv.hideLoadingIndicator(fmt.Sprintf("Command executed in %s", container.Name))

			// Show command output in a modal
			dv.showLogsModal(fmt.Sprintf("Command: %s", command), output)
		},
	)
}

// pruneSystem prunes unused Docker resources
func (dv *DockerView) pruneSystem() {
	// Show confirmation modal
	dv.showConfirmationModal(
		"Prune Docker System",
		"Are you sure you want to prune unused Docker resources?\nThis will remove all stopped containers, unused networks, dangling images, and build cache.",
		func(confirmed bool) {
			if !confirmed {
				return
			}

			dv.showLoadingIndicator("Pruning Docker system")

			output, err := dv.dockerClient.PruneSystem()
			if err != nil {
				dv.errorHandler.HandleError(err, ui.ErrorLevelError, "System Prune")
				return
			}

			dv.hideLoadingIndicator("Docker system pruned")

			// Show output in a modal
			dv.showInfoModal(
				"Prune Results",
				output,
				nil,
			)

			dv.RefreshAll()
		},
	)
}

// showLogsModal shows container logs in a modal
func (dv *DockerView) showLogsModal(title, content string) {
	textView := tview.NewTextView().
		SetScrollable(true).
		SetWrap(true).
		SetDynamicColors(true)

	textView.SetText(content)

	// Create a frame around the text view
	frame := tview.NewFrame(textView).
		SetBorders(0, 0, 0, 0, 0, 0).
		AddText(title, true, tview.AlignLeft, tcell.ColorBlue)

	// Add a close button message
	frame.AddText("Press ESC to close", false, tview.AlignRight, tcell.ColorYellow)

	// Create a page for the modal
	modalPage := "logs-modal"
	dv.pages.AddPage(modalPage, frame, true, true)

	// Set up a key handler for the frame
	frame.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			dv.pages.RemovePage(modalPage)
			return nil
		}
		return event
	})

	// Set focus on the text view for scrolling
	dv.app.SetFocus(textView)
}

// showHelpModal shows the help information for this plugin
func (dv *DockerView) showHelpModal() {
	helpText := `[yellow]Docker Plugin Help[white]

[green]Navigation:[white]
  [aqua]C[white]   - View Containers
  [aqua]I[white]   - View Images
  [aqua]N[white]   - View Networks
  [aqua]V[white]   - View Volumes
  [aqua]R[white]   - Refresh current view
  [aqua]ESC[white] - Close modal/Go back

[green]Container Actions:[white]
  [aqua]S[white] - Start selected container
  [aqua]X[white] - Stop selected container
  [aqua]D[white] - Remove selected container
  [aqua]L[white] - View logs of selected container
  [aqua]E[white] - Execute command in selected container

[green]System Actions:[white]
  [aqua]P[white]     - Prune unused Docker resources
  [aqua]Ctrl+D[white] - Refresh Docker data`

	dv.showInfoModal(
		"Docker Plugin Help",
		helpText,
		func() {
			dv.app.SetFocus(dv.cores.GetTable())
		},
	)
}

// Custom implementations of modal functions

// showInfoModal displays a modal with information and an OK button
func (dv *DockerView) showInfoModal(
	title string,
	text string,
	callback func(),
) {
	// Create text view for the information
	textView := tview.NewTextView()
	textView.SetText(text)
	textView.SetTextColor(tcell.ColorWhite)
	textView.SetDynamicColors(true)
	textView.SetScrollable(true)
	textView.SetWordWrap(true)
	textView.SetBorder(true)
	textView.SetBorderColor(tcell.ColorAqua)
	textView.SetTitle(" " + title + " ")
	textView.SetTitleColor(tcell.ColorOrange)
	textView.SetTitleAlign(tview.AlignCenter)
	textView.SetBorderPadding(1, 1, 2, 2)

	// Use larger width/height for readability
	width := 76  // Common width for modal
	height := 20 // Adjusted for content

	// Create a flexbox container to center the components
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(textView, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	// Set a key handler for the text view to close with ESC
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			dv.pages.RemovePage("info-modal")
			if callback != nil {
				callback()
			}
			return nil
		}
		return event
	})

	// Show the modal
	dv.pages.AddPage("info-modal", flex, true, true)
	dv.app.SetFocus(textView)
}

// showConfirmationModal displays a modal with Yes/No buttons for confirming actions
func (dv *DockerView) showConfirmationModal(
	title string,
	text string,
	callback func(confirmed bool),
) {
	// Create a form with buttons
	form := tview.NewForm()
	form.SetItemPadding(0)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetBackgroundColor(tcell.ColorDefault)
	form.SetButtonBackgroundColor(tcell.ColorDefault)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetBorder(true)
	form.SetTitle(" " + title + " ")
	form.SetTitleAlign(tview.AlignCenter)
	form.SetBorderColor(tcell.ColorAqua)
	form.SetTitleColor(tcell.ColorOrange)
	form.SetBorderPadding(1, 1, 2, 2)

	// Add text with minimal spacing
	form.AddTextView("", text, 0, 2, true, false)

	// Add buttons
	form.AddButton("Yes", func() {
		dv.pages.RemovePage("confirmation-modal")
		if callback != nil {
			callback(true)
		}
	})

	form.AddButton("No", func() {
		dv.pages.RemovePage("confirmation-modal")
		if callback != nil {
			callback(false)
		}
	})

	// Style the buttons with focus colors
	for i := 0; i < form.GetButtonCount(); i++ {
		if b := form.GetButton(i); b != nil {
			b.SetBackgroundColor(tcell.ColorDefault)
			b.SetLabelColor(tcell.ColorWhite)
			b.SetBackgroundColorActivated(tcell.ColorWhite)
			b.SetLabelColorActivated(tcell.ColorDefault)
		}
	}

	// Set a width for the modal
	width := 50
	height := 8 // Fixed height for confirmation dialog

	// Create a flexbox container to center the components
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	// Set up a key handler for the ESC key
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			dv.pages.RemovePage("confirmation-modal")
			if callback != nil {
				callback(false)
			}
			return nil
		}
		return event
	})

	// Show the modal
	dv.pages.AddPage("confirmation-modal", flex, true, true)
	dv.app.SetFocus(form)
}

// showCompactInputModal displays a compact modal with a text input field
func (dv *DockerView) showCompactInputModal(
	title string,
	inputLabel string,
	placeholder string,
	callback func(text string, cancelled bool),
) {
	// Create form with input field
	form := tview.NewForm()
	form.SetItemPadding(0)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetBackgroundColor(tcell.ColorDefault)
	form.SetButtonBackgroundColor(tcell.ColorDefault)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetFieldBackgroundColor(tcell.ColorDefault)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetBorder(true)
	form.SetTitle(" " + title + " ")
	form.SetTitleAlign(tview.AlignCenter)
	form.SetBorderColor(tcell.ColorAqua)
	form.SetTitleColor(tcell.ColorOrange)
	form.SetBorderPadding(1, 1, 2, 2)

	// Add the input field
	form.AddInputField(inputLabel, placeholder, 30, nil, nil)

	// Add buttons with minimal vertical spacing
	form.AddButton("OK", func() {
		value := form.GetFormItem(0).(*tview.InputField).GetText()
		dv.pages.RemovePage("compact-modal")

		if value == "" {
			if callback != nil {
				callback("", true) // Treat empty input as cancelled
			}
			return
		}

		if callback != nil {
			callback(value, false)
		}
	})

	form.AddButton("Cancel", func() {
		dv.pages.RemovePage("compact-modal")
		if callback != nil {
			callback("", true)
		}
	})

	// Style the buttons with focus colors
	for i := 0; i < form.GetButtonCount(); i++ {
		if b := form.GetButton(i); b != nil {
			b.SetBackgroundColor(tcell.ColorDefault)
			b.SetLabelColor(tcell.ColorWhite)
			b.SetBackgroundColorActivated(tcell.ColorWhite)
			b.SetLabelColorActivated(tcell.ColorDefault)
		}
	}

	// Set a width for the modal
	width := 50
	height := 8 // Compact height

	// Create a flexbox container to center the components
	flex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	// Set up a key handler for the ESC key
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			dv.pages.RemovePage("compact-modal")
			if callback != nil {
				callback("", true)
			}
			return nil
		}
		return event
	})

	// Show the modal
	dv.pages.AddPage("compact-modal", flex, true, true)
	dv.app.SetFocus(form)
}
