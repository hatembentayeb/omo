package main

import (
	"fmt"
	"os"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"omo/pkg/pluginapi"
	"omo/pkg/ui"
)

// OhmyopsPlugin is expected by the main application
var OhmyopsPlugin ArgocdPlugin

// ArgocdPlugin represents the ArgoCD management plugin
type ArgocdPlugin struct {
	app             *tview.Application
	pages           *tview.Pages
	cores           *ui.CoreView
	currentView     string
	accountView     *AccountView
	projectView     *ProjectView
	applicationView *ApplicationView
	apiClient       *ArgoAPIClient
	serverURL       string
	credentials     CredentialInfo
	config          *ArgocdConfig
}

// CredentialInfo holds connection credentials
type CredentialInfo struct {
	Username string
	Password string
	Token    string
}

// Start initializes the plugin
func (p *ArgocdPlugin) Start(app *tview.Application) tview.Primitive {
	// Initialize debug logger
	err := InitDebugLogger()
	if err != nil {
		// If we can't initialize the logger, log to console instead
		fmt.Printf("Failed to initialize debug logger: %v\n", err)
	} else {
		Debug("ArgoCD plugin starting...")
	}

	p.app = app
	p.pages = tview.NewPages()
	p.currentView = "main"

	// Load ArgoCD config
	config, err := LoadArgocdConfig()
	if err != nil {
		// If no config found, we'll create it on first connection
		p.config = nil
		Debug("No ArgoCD config found: %v", err)
	} else {
		p.config = config
		Debug("Loaded ArgoCD config with %d instances", len(config.Instances))
	}

	// Initialize API client with config
	p.apiClient = NewArgoAPIClient(p.config)
	Debug("API client initialized")

	// Initialize views
	p.accountView = NewAccountView(p.app, p.pages, nil, p.apiClient)
	p.projectView = NewProjectView(p.app, p.pages, nil, p.apiClient)
	p.applicationView = NewApplicationView(p.app, p.pages, nil, p.apiClient)
	Debug("Views initialized")

	// Initialize the main view
	p.initializeMainView()

	// Update the cores reference in views with the initialized one
	p.accountView.cores = p.cores
	p.projectView.cores = p.cores
	p.applicationView.cores = p.cores
	Debug("Core references updated in views")

	// Set up table handlers for all views
	p.applicationView.SetupTableHandlers()
	p.projectView.SetupTableHandlers()
	p.accountView.SetupTableHandlers()
	Debug("Table handlers set up for all views")

	// Add keyboard handling for connection (Ctrl+T)
	p.pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlT {
			p.showInstanceSelector()
			return nil
		}
		return event
	})

	// Show message to connect to ArgoCD server
	p.cores.SetTableData([][]string{
		{"Please connect to an ArgoCD server", "", "", ""},
	})

	// Show connection message
	p.cores.Log("[blue]Please select an ArgoCD instance to connect to...")

	// Use a single post-initialization task
	safeGo(func() {
		// Wait for UI to initialize
		time.Sleep(300 * time.Millisecond)

		// Show the instance selector
		p.app.QueueUpdateDraw(func() {
			Debug("Showing instance selector after initialization")
			p.showInstanceSelector()
		})

		// If we have a default instance configured, connect to it
		if p.config != nil && p.config.DefaultInstance != "" {
			Debug("Auto-connecting to default instance: %s", p.config.DefaultInstance)

			// Find the default instance
			var defaultInstance *ArgocdInstance
			for _, inst := range p.config.Instances {
				if inst.Name == p.config.DefaultInstance {
					defaultInstance = &inst
					break
				}
			}

			if defaultInstance != nil {
				// Wait a moment for UI to show
				time.Sleep(500 * time.Millisecond)

				// Connect to the default instance
				p.app.QueueUpdateDraw(func() {
					Debug("Connecting to default instance: %s (%s)",
						defaultInstance.Name, defaultInstance.URL)

					// Show connecting message
					p.cores.Log(fmt.Sprintf("[blue]Auto-connecting to default instance: %s (%s)...",
						defaultInstance.Name, defaultInstance.URL))

					// Connect in background to avoid blocking UI
					safeGo(func() {
						err := p.apiClient.Connect(defaultInstance.URL,
							defaultInstance.Username, defaultInstance.Password)

						if err != nil {
							p.app.QueueUpdateDraw(func() {
								p.cores.Log(fmt.Sprintf("[red]Failed to connect to default instance: %v", err))
							})
							return
						}

						// Store credentials
						p.serverURL = defaultInstance.URL
						p.credentials.Username = defaultInstance.Username
						p.credentials.Password = defaultInstance.Password

						p.app.QueueUpdateDraw(func() {
							p.cores.Log(fmt.Sprintf("[green]Connected to ArgoCD instance: %s",
								defaultInstance.Name))

							// Update UI with connection info
							p.cores.SetInfoText(fmt.Sprintf("ArgoCD Manager | Server: %s | User: %s | Instance: %s",
								defaultInstance.URL, defaultInstance.Username, defaultInstance.Name))

							// Refresh data to show applications
							p.cores.RefreshData()
						})
					})
				})
			}
		}
	})

	Debug("ArgoCD plugin initialized")
	return p.pages
}

// Stop cleans up resources when the plugin is unloaded.
func (p *ArgocdPlugin) Stop() {
	if p.cores != nil {
		p.cores.StopAutoRefresh()
		p.cores.UnregisterHandlers()
	}

	if debugLogger != nil {
		debugLogger.Close()
	}

	if p.pages != nil {
		pageIDs := []string{
			"main",
			"debug-logs-modal",
			"create-account-modal",
			"create-token-modal",
			"project-roles-modal",
			"create-project-modal",
			"confirm-modal",
			"confirmation-modal",
			"error-modal",
			"info-modal",
			"list-selector-modal",
			"progress-modal",
			"sort-modal",
			"compact-modal",
		}
		for _, pageID := range pageIDs {
			if p.pages.HasPage(pageID) {
				p.pages.RemovePage(pageID)
			}
		}
	}

	p.accountView = nil
	p.projectView = nil
	p.applicationView = nil
	p.apiClient = nil
	p.cores = nil
	p.pages = nil
	p.app = nil
}

// GetMetadata returns plugin metadata.
func (p *ArgocdPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "argocd",
		Version:     "1.0.0",
		Description: "ArgoCD management plugin",
		Author:      "OhMyOps",
		License:     "MIT",
		Tags:        []string{"argocd", "gitops", "kubernetes", "deployment"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "",
	}
}

// initializeMainView creates the main view
func (p *ArgocdPlugin) initializeMainView() {
	p.cores = ui.NewCoreView(p.app, "ArgoCD Manager")
	p.cores.SetModalPages(p.pages)
	p.cores.SetTableHeaders([]string{"Name", "Project", "Health", "Sync Status"})
	p.cores.SetRefreshCallback(p.fetchApplications)
	p.cores.SetInfoText("Manage your ArgoCD server")

	// Key bindings
	p.cores.AddKeyBinding("C", "Create Application", nil)
	p.cores.AddKeyBinding("D", "Delete Application", nil)
	p.cores.AddKeyBinding("S", "Sync Application", nil)
	p.cores.AddKeyBinding("V", "View Details", nil)
	p.cores.AddKeyBinding("F", "Refresh Status", nil)
	p.cores.AddKeyBinding("A", "Accounts", nil)
	p.cores.AddKeyBinding("P", "Projects", nil)
	p.cores.AddKeyBinding("^T", "Select Instance", nil)
	p.cores.AddKeyBinding("^D", "Debug Logs", nil)
	p.cores.AddKeyBinding("^B", "Back", nil)

	p.setupActionHandler()
	p.cores.RegisterHandlers()

	p.pages.AddPage("main", p.cores.GetLayout(), true, true)
	p.cores.PushView("Applications")
	p.currentView = "applications"
	p.cores.Log("Plugin initialized")
}

// setupActionHandler configures the action handler for the plugin
func (p *ArgocdPlugin) setupActionHandler() {
	p.cores.SetActionCallback(func(action string, payload map[string]interface{}) error {
		if action == "keypress" {
			if key, ok := payload["key"].(string); ok {
				switch p.currentView {
				case "applications":
					// Application view actions
					switch key {
					case "R":
						p.refreshApplications()
					case "C":
						p.applicationView.showCreateApplicationModal()
					case "D":
						p.applicationView.showDeleteApplicationModal()
					case "S":
						p.applicationView.showSyncApplicationModal()
					case "V":
						p.applicationView.showApplicationDetailsModal()
					case "F":
						p.applicationView.showRefreshApplicationModal()
					case "A":
						p.switchToAccountsView()
					case "P":
						p.switchToProjectsView()
					case "?":
						p.showHelpModal()
					case "^B":
						p.returnToPreviousView()
					case "^D":
						p.showDebugLogsModal()
					}
				case "projects":
					// Project view actions
					switch key {
					case "R":
						p.refreshProjects()
					case "C":
						p.projectView.showCreateProjectModal()
					case "D":
						p.projectView.showDeleteProjectModal()
					case "V":
						p.projectView.showProjectDetailsModal()
					case "O":
						p.projectView.showProjectRolesModal()
					case "A":
						p.switchToApplicationsView()
					case "U":
						p.switchToAccountsView()
					case "?":
						p.showHelpModal()
					case "^B":
						p.returnToPreviousView()
					case "^D":
						p.showDebugLogsModal()
					}
				case "accounts":
					// Account view actions
					switch key {
					case "R":
						p.refreshAccounts()
					case "C":
						p.accountView.showCreateAccountModal()
					case "D":
						p.accountView.showDeleteAccountModal()
					case "V":
						p.accountView.showAccountDetailsModal()
					case "T":
						p.accountView.showCreateTokenModal()
					case "A":
						p.switchToApplicationsView()
					case "P":
						p.switchToProjectsView()
					case "?":
						p.showHelpModal()
					case "^B":
						p.returnToPreviousView()
					case "^D":
						p.showDebugLogsModal()
					}
				}
			}
		} else if action == "enter_pressed" {
			// Handle Enter key based on current view
			switch p.currentView {
			case "applications":
				p.applicationView.showApplicationDetailsModal()
			case "projects":
				p.projectView.showProjectDetailsModal()
			case "accounts":
				p.accountView.showAccountDetailsModal()
			}
			return nil
		} else if action == "navigate_back" {
			// When ESC is pressed, the Core UI automatically pops the view,
			// and we just need to update our internal state to match
			currentView := p.cores.GetCurrentView()
			p.switchToView(currentView)
		} else if action == "back" {
			// This is triggered when ESC is pressed, before navigate_back
			if fromView, ok := payload["from"].(string); ok {
				p.cores.Log(fmt.Sprintf("[blue]Navigating back from %s view", fromView))
			}
		} else if action == "rowSelected" {
			// Handle row selection if needed at the global level
			Debug("Row selected: %v", payload)
		}
		return nil
	})
}

// fetchApplications retrieves ArgoCD applications and formats them for display
func (p *ArgocdPlugin) fetchApplications() ([][]string, error) {
	if p.apiClient == nil || !p.apiClient.IsConnected {
		return [][]string{
			{"Please connect to ArgoCD server", "", "", ""},
		}, nil
	}

	applications, err := p.applicationView.fetchApplications()
	if err != nil {
		Debug("Error fetching applications in main: %v", err)
		return [][]string{
			{fmt.Sprintf("Error: %v", err), "", "", ""},
		}, nil
	}

	// Ensure the application table gets updated even if no applications found
	if len(applications) == 0 || (len(applications) == 1 && applications[0][0] == "No applications found") {
		Debug("No applications found or received empty result")
		return [][]string{
			{"No applications found", "", "", ""},
		}, nil
	}

	Debug("Main returning %d application rows", len(applications))
	return applications, nil
}

// fetchProjects retrieves ArgoCD projects and formats them for display
func (p *ArgocdPlugin) fetchProjects() ([][]string, error) {
	if p.apiClient == nil || !p.apiClient.IsConnected {
		return [][]string{
			{"Please connect to ArgoCD server", "", "", ""},
		}, nil
	}

	projects, err := p.projectView.fetchProjects()
	if err != nil {
		Debug("Error fetching projects in main: %v", err)
		return [][]string{
			{fmt.Sprintf("Error: %v", err), "", "", ""},
		}, nil
	}

	// Ensure the project table gets updated even if no projects found
	if len(projects) == 0 || (len(projects) == 1 && projects[0][0] == "No projects found") {
		Debug("No projects found or received empty result")
		return [][]string{
			{"No projects found", "", "", ""},
		}, nil
	}

	Debug("Main returning %d project rows", len(projects))
	return projects, nil
}

// fetchAccounts retrieves ArgoCD accounts and formats them for display
func (p *ArgocdPlugin) fetchAccounts() ([][]string, error) {
	if p.apiClient == nil || !p.apiClient.IsConnected {
		return [][]string{
			{"Please connect to ArgoCD server", "", "", ""},
		}, nil
	}

	return p.accountView.fetchAccounts()
}

// refreshApplications refreshes the application list
func (p *ArgocdPlugin) refreshApplications() {
	Debug("Manual refresh of applications requested")
	p.cores.RefreshData()
	Debug("Application refresh complete")
}

// refreshProjects refreshes the project list
func (p *ArgocdPlugin) refreshProjects() {
	Debug("Manual refresh of projects requested")
	p.cores.RefreshData()
	Debug("Project refresh complete")
}

// refreshAccounts refreshes the account list
func (p *ArgocdPlugin) refreshAccounts() {
	Debug("Manual refresh of accounts requested")
	p.cores.RefreshData()
	Debug("Account refresh complete")
}

// showInstanceSelector displays a modal to select an ArgoCD instance
func (p *ArgocdPlugin) showInstanceSelector() {
	// Remove any existing instance selector modal first
	p.pages.RemovePage("list-selector-modal")

	// Check if we have instances configured
	if p.config == nil || len(p.config.Instances) == 0 {
		// No config file or no instances, show error message
		p.cores.Log("[red]No ArgoCD instances configured. Please add instances to ~/.omo/configs/argocd/argocd.yaml")
		return
	}

	// Format instances for the modal
	items := make([][]string, len(p.config.Instances))
	for i, instance := range p.config.Instances {
		// Highlight the current instance if there is one
		displayName := instance.Name
		if p.serverURL == instance.URL {
			displayName += " (current)"
		}
		items[i] = []string{displayName, instance.URL}
	}

	ui.ShowStandardListSelectorModal(
		p.pages,
		p.app,
		"Select ArgoCD Instance",
		items,
		func(index int, text string, cancelled bool) {
			if cancelled || index < 0 {
				// If no instance is selected, show a message
				p.cores.Log("[yellow]No instance selected. Please select an instance to continue.")
				return
			}

			// Get the selected instance
			instance := p.config.Instances[index]

			// Skip if same instance
			if p.serverURL == instance.URL && p.credentials.Username == instance.Username {
				p.app.SetFocus(p.cores.GetTable())
				return
			}

			// Show connecting message
			p.cores.Log(fmt.Sprintf("[blue]Connecting to %s (%s)...", instance.Name, instance.URL))

			// Show progress modal
			pm := ui.ShowProgressModal(
				p.pages, p.app, "Connecting to ArgoCD", 100, true,
				nil, true,
			)

			// Connect to the selected instance
			safeGo(func() {
				err := p.apiClient.Connect(instance.URL, instance.Username, instance.Password)
				if err != nil {
					p.app.QueueUpdateDraw(func() {
						pm.Close()
						p.cores.Log(fmt.Sprintf("[red]Failed to connect: %v", err))
					})
					return
				}

				// Store credentials
				p.serverURL = instance.URL
				p.credentials.Username = instance.Username
				p.credentials.Password = instance.Password

				p.app.QueueUpdateDraw(func() {
					pm.Close()
					p.cores.Log(fmt.Sprintf("[green]Connected to ArgoCD instance: %s", instance.Name))

					// Update UI with connection info
					p.cores.SetInfoText(fmt.Sprintf("ArgoCD Manager | Server: %s | User: %s | Instance: %s",
						instance.URL, instance.Username, instance.Name))

					// Refresh data to show applications
					p.cores.RefreshData()

					// Set focus back to the table
					p.app.SetFocus(p.cores.GetTable())

					// Queue a second refresh after a short delay to ensure all data is loaded
					go func() {
						time.Sleep(500 * time.Millisecond)
						p.app.QueueUpdateDraw(func() {
							Debug("Performing second refresh after connection")
							p.refreshApplications()
						})
					}()
				})
			})
		},
	)
}

// showHelpModal displays the help information
func (p *ArgocdPlugin) showHelpModal() {
	// Create the help content with sections
	content := `[yellow]ArgoCD Manager Help[white]

[green]Navigation[white]
  [aqua]↑/↓[white] - Navigate between items
  [aqua]Enter[white] - Select an item

[green]General[white]
  [aqua]R[white] - Refresh list
  [aqua]Ctrl+T[white] - Select ArgoCD instance from config
  [aqua]Ctrl+D[white] - View debug logs
  [aqua]?[white] - Show help
  [aqua]Esc/Ctrl+B[white] - Go back to previous view

[green]Applications[white]
  [aqua]C[white] - Create a new application
  [aqua]D[white] - Delete selected application
  [aqua]S[white] - Sync application
  [aqua]V[white] - View application details
  [aqua]F[white] - Refresh application status
  [aqua]A[white] - Switch to accounts view
  [aqua]P[white] - Switch to projects view

[green]Projects[white]
  [aqua]C[white] - Create a new project
  [aqua]D[white] - Delete selected project
  [aqua]V[white] - View project details
  [aqua]O[white] - View project roles
  [aqua]A[white] - Switch to applications view
  [aqua]U[white] - Switch to accounts view

[green]Accounts[white]
  [aqua]C[white] - Create a new account
  [aqua]D[white] - Delete selected account
  [aqua]V[white] - View account details
  [aqua]T[white] - Create token for account
  [aqua]A[white] - Switch to applications view
  [aqua]P[white] - Switch to projects view

[green]Configuration[white]
  ArgoCD instances are configured in ~/.omo/configs/argocd/argocd.yaml
  To add new instances, edit this file directly

[green]Troubleshooting[white]
  Debug logs are saved in logs/argocd-debug-*.log
  Press Ctrl+D to view the current debug log`

	// Show the info modal with a callback to return focus to the table
	ui.ShowInfoModal(
		p.pages,
		p.app,
		"ArgoCD Manager Help",
		content,
		func() {
			// Return focus to the table when modal is closed
			p.app.SetFocus(p.cores.GetTable())
		},
	)
}

// switchToView updates the current view and UI based on the view name
func (p *ArgocdPlugin) switchToView(viewName string) {
	// Set the current view based on the view name
	switch viewName {
	case "Applications":
		p.currentView = "applications"
		p.cores.SetTableHeaders([]string{"Name", "Project", "Health", "Sync Status"})
		p.cores.SetRefreshCallback(p.fetchApplications)
		p.cores.SetInfoText(fmt.Sprintf("ArgoCD Manager | Server: %s | User: %s", p.serverURL, p.credentials.Username))

		// Update key bindings for applications view
		p.cores.ClearKeyBindings()
		p.cores.AddKeyBinding("R", "Refresh", p.refreshApplications)
		p.cores.AddKeyBinding("C", "Create Application", nil)
		p.cores.AddKeyBinding("D", "Delete Application", nil)
		p.cores.AddKeyBinding("S", "Sync Application", nil)
		p.cores.AddKeyBinding("V", "View Details", nil)
		p.cores.AddKeyBinding("F", "Refresh Status", nil)
		p.cores.AddKeyBinding("A", "Accounts", nil)
		p.cores.AddKeyBinding("P", "Projects", nil)
		p.cores.AddKeyBinding("^T", "Instance", nil)
		p.cores.AddKeyBinding("?", "Help", nil)
		p.cores.AddKeyBinding("ESC", "Back", nil)

	case "Projects":
		p.currentView = "projects"
		p.cores.SetTableHeaders([]string{"Name", "Destinations", "Repositories", "Roles"})
		p.cores.SetRefreshCallback(p.fetchProjects)
		p.cores.SetInfoText(fmt.Sprintf("ArgoCD Project Manager | Server: %s | User: %s", p.serverURL, p.credentials.Username))

		// Update key bindings for projects view
		p.cores.ClearKeyBindings()
		p.cores.AddKeyBinding("R", "Refresh", p.refreshProjects)
		p.cores.AddKeyBinding("C", "Create Project", nil)
		p.cores.AddKeyBinding("D", "Delete Project", nil)
		p.cores.AddKeyBinding("V", "View Details", nil)
		p.cores.AddKeyBinding("O", "View Roles", nil)
		p.cores.AddKeyBinding("A", "Applications", nil)
		p.cores.AddKeyBinding("U", "Accounts", nil)
		p.cores.AddKeyBinding("^T", "Instance", nil)
		p.cores.AddKeyBinding("?", "Help", nil)
		p.cores.AddKeyBinding("ESC", "Back", nil)

	case "Accounts":
		p.currentView = "accounts"
		p.cores.SetTableHeaders([]string{"Name", "Capabilities", "Enabled", "Tokens"})
		p.cores.SetRefreshCallback(p.fetchAccounts)
		p.cores.SetInfoText(fmt.Sprintf("ArgoCD Account Manager | Server: %s | User: %s", p.serverURL, p.credentials.Username))

		// Update key bindings for accounts view
		p.cores.ClearKeyBindings()
		p.cores.AddKeyBinding("R", "Refresh", p.refreshAccounts)
		p.cores.AddKeyBinding("C", "Create Account", nil)
		p.cores.AddKeyBinding("D", "Delete Account", nil)
		p.cores.AddKeyBinding("V", "View Details", nil)
		p.cores.AddKeyBinding("T", "Create Token", nil)
		p.cores.AddKeyBinding("A", "Applications", nil)
		p.cores.AddKeyBinding("P", "Projects", nil)
		p.cores.AddKeyBinding("^T", "Instance", nil)
		p.cores.AddKeyBinding("?", "Help", nil)
		p.cores.AddKeyBinding("ESC", "Back", nil)

	default:
		// If we don't recognize the view, default to applications view
		p.currentView = "applications"
		p.cores.SetTableHeaders([]string{"Name", "Project", "Health", "Sync Status"})
		p.cores.SetRefreshCallback(p.fetchApplications)
		p.cores.SetInfoText(fmt.Sprintf("ArgoCD Manager | Server: %s | User: %s", p.serverURL, p.credentials.Username))

		// Update key bindings for applications view
		p.cores.ClearKeyBindings()
		p.cores.AddKeyBinding("R", "Refresh", p.refreshApplications)
		p.cores.AddKeyBinding("C", "Create Application", nil)
		p.cores.AddKeyBinding("D", "Delete Application", nil)
		p.cores.AddKeyBinding("S", "Sync Application", nil)
		p.cores.AddKeyBinding("V", "View Details", nil)
		p.cores.AddKeyBinding("F", "Refresh Status", nil)
		p.cores.AddKeyBinding("A", "Accounts", nil)
		p.cores.AddKeyBinding("P", "Projects", nil)
		p.cores.AddKeyBinding("^T", "Instance", nil)
		p.cores.AddKeyBinding("?", "Help", nil)
		p.cores.AddKeyBinding("ESC", "Back", nil)
	}

	// Refresh data to update the view
	p.cores.RefreshData()
}

// returnToPreviousView goes back one step in the view stack
func (p *ArgocdPlugin) returnToPreviousView() {
	// Only do something if we have more than one view in the stack
	currentViewName := p.cores.GetCurrentView()
	if currentViewName == "" {
		p.cores.Log("[yellow]Already at the root view")
		return
	}

	// Simulate ESC behavior by popping the view and updating
	popped := p.cores.PopView()
	if popped != "" {
		p.cores.Log(fmt.Sprintf("[blue]Popped view: %s", popped))
	}

	// Get the new current view
	newCurrentView := p.cores.GetCurrentView()
	if newCurrentView != "" {
		// Update the UI based on the previous view
		p.switchToView(newCurrentView)
		p.cores.Log(fmt.Sprintf("[blue]Navigated back to %s view (using Ctrl+B)", newCurrentView))
	} else {
		// Default to applications view if no previous view exists
		p.switchToApplicationsView()
	}
}

// switchToApplicationsView switches to the application view
func (p *ArgocdPlugin) switchToApplicationsView() {
	Debug("Switching to Applications View")

	// Make sure we're connected
	if p.apiClient == nil || !p.apiClient.IsConnected {
		p.cores.Log("[red]Not connected to ArgoCD. Please connect first.")
		return
	}

	// Update current view
	p.currentView = "applications"

	// Push view to navigation stack
	p.cores.PushView("Applications")

	// Set info text
	if p.apiClient.IsConnected {
		p.cores.SetInfoText(fmt.Sprintf("ArgoCD Manager | Server: %s | User: %s", p.serverURL, p.credentials.Username))
	} else {
		p.cores.SetInfoText("ArgoCD Manager | Not connected")
	}

	// Set up the table headers for applications
	p.cores.SetTableHeaders([]string{
		"Name",
		"Project",
		"Health Status",
		"Sync Status",
	})

	// Setup refresh callback to fetch applications
	p.cores.SetRefreshCallback(p.fetchApplications)

	// Refresh data
	go func() {
		// Refresh synchronously to avoid race condition
		applications, err := p.applicationView.fetchApplications()
		if err != nil {
			Debug("Error fetching applications in switchToApplicationsView: %v", err)
			// Handle error - use default message
			applications = [][]string{
				{"Error fetching applications", "", "", ""},
			}
		}

		p.app.QueueUpdateDraw(func() {
			// Update table data
			p.cores.SetTableData(applications)
			Debug("Updated applications table data with %d rows", len(applications))
		})
	}()

	// Make sure we unregister previous handlers and register application ones
	p.cores.UnregisterHandlers()
	p.applicationView.SetupTableHandlers()
	p.cores.RegisterHandlers()
}

// switchToProjectsView switches to the project management view
func (p *ArgocdPlugin) switchToProjectsView() {
	Debug("Switching to Projects view")

	// Make sure we're connected
	if p.apiClient == nil || !p.apiClient.IsConnected {
		p.cores.Log("[red]Not connected to ArgoCD. Please connect first.")
		return
	}

	// Update current view
	p.currentView = "projects"

	// Push view to navigation stack
	p.cores.PushView("Projects")

	// Update UI
	p.switchToView("Projects")

	// Clear any old data and set loading message
	p.cores.SetTableData([][]string{
		{"Loading projects...", "", "", ""},
	})

	// Force refresh the data with delayed execution to ensure UI is ready
	go func() {
		time.Sleep(200 * time.Millisecond)
		p.app.QueueUpdateDraw(func() {
			Debug("Performing explicit projects refresh")

			// First try to load the projects directly
			projects, err := p.apiClient.GetProjects()
			if err != nil {
				Debug("Error pre-loading projects: %v", err)
				p.cores.Log(fmt.Sprintf("[red]Error loading projects: %v", err))
			} else {
				Debug("Pre-loaded %d projects successfully", len(projects))
				// Update the projectView's projects slice
				p.projectView.projects = projects
			}

			// Now refresh the table through the normal flow
			p.cores.RefreshData()

			// Log completion
			Debug("Projects view refresh complete")
		})
	}()
}

// switchToAccountsView switches to the account management view
func (p *ArgocdPlugin) switchToAccountsView() {
	Debug("Switching to Accounts view")

	// Make sure we're connected
	if p.apiClient == nil || !p.apiClient.IsConnected {
		p.cores.Log("[red]Not connected to ArgoCD. Please connect first.")
		return
	}

	// Update current view
	p.currentView = "accounts"

	// Push view to navigation stack
	p.cores.PushView("Accounts")

	// Update UI
	p.switchToView("Accounts")

	// Clear any old data and set loading message
	p.cores.SetTableData([][]string{
		{"Loading accounts...", "", "", ""},
	})

	// Force refresh the data with delayed execution to ensure UI is ready
	go func() {
		time.Sleep(200 * time.Millisecond)
		p.app.QueueUpdateDraw(func() {
			Debug("Performing explicit accounts refresh")

			// First try to load the accounts directly
			accounts, err := p.apiClient.GetAccounts()
			if err != nil {
				Debug("Error pre-loading accounts: %v", err)
				p.cores.Log(fmt.Sprintf("[red]Error loading accounts: %v", err))
			} else {
				Debug("Pre-loaded %d accounts successfully", len(accounts))
				// Update the accountView's accounts slice
				p.accountView.accounts = accounts
			}

			// Now refresh the table through the normal flow
			p.cores.RefreshData()

			// Log completion
			Debug("Accounts view refresh complete")
		})
	}()
}

// showDebugLogsModal displays the debug logs in a modal
func (p *ArgocdPlugin) showDebugLogsModal() {
	// Get the log file path
	logger := GetLogger()
	if logger == nil || logger.file == nil {
		p.cores.Log("[red]Debug logger is not initialized")
		return
	}

	// Create a text view for the logs
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWrap(true).
		SetWordWrap(true)

	// Read the log file
	logFile := logger.file.Name()
	content, err := os.ReadFile(logFile)
	if err != nil {
		p.cores.Log(fmt.Sprintf("[red]Error reading debug logs: %v", err))
		return
	}

	// Set the content
	textView.SetText(string(content))
	textView.ScrollToEnd()

	// Create a frame for the textview
	frame := tview.NewFrame(textView).
		SetBorders(0, 0, 0, 0, 0, 0).
		AddText("ArgoCD Plugin Debug Logs", true, tview.AlignCenter, tcell.ColorGreen).
		AddText("Press ESC to close", false, tview.AlignCenter, tcell.ColorYellow)

	// Create the modal
	width := 80
	height := 30

	modal := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(frame, width, 1, true).
			AddItem(nil, 0, 1, false),
			height, 1, true).
		AddItem(nil, 0, 1, false)

	// Add the modal to pages
	p.pages.AddPage("debug-logs-modal", modal, true, true)

	// Set key handling
	textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			p.pages.RemovePage("debug-logs-modal")
			p.app.SetFocus(p.cores.GetTable())
			return nil
		}
		return event
	})

	// Set focus
	p.app.SetFocus(textView)
}
