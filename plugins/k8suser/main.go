package main

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"omo/pkg/pluginapi"
	"omo/pkg/ui"
)

// OhmyopsPlugin is expected by the main application
var OhmyopsPlugin K8sUserPlugin

// K8sUserPlugin represents the Kubernetes user management plugin
type K8sUserPlugin struct {
	app            *tview.Application
	pages          *tview.Pages
	cores          *ui.Cores
	currentView    string
	userView       *UserView
	roleView       *RoleView
	certManager    *CertManager
	k8sClient      *K8sClient
	kubeconfig     string
	currentContext string
}

// Start initializes the plugin
func (p *K8sUserPlugin) Start(app *tview.Application) tview.Primitive {
	p.app = app
	p.pages = tview.NewPages()
	p.currentView = "main"

	// Initialize managers
	p.certManager = NewCertManager()
	p.k8sClient = NewK8sClient()

	// Try to get the default kubeconfig
	p.k8sClient.GetKubeConfig()

	// Initialize the user view - move this up before initializeMainView
	p.userView = NewUserView(p.app, p.pages, nil, p.certManager, p.k8sClient)

	// Initialize the role view
	p.roleView = NewRoleView(p.app, p.pages, nil, p.k8sClient)

	// Initialize views
	p.initializeMainView()

	// Update the cores reference in views with the initialized one
	p.userView.cores = p.cores
	p.roleView.cores = p.cores

	// Add keyboard handling for context selection (Ctrl+T)
	p.pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlT {
			p.showContextSelector()
			return nil
		}
		return event
	})

	// Show message to select a context
	p.cores.SetTableData([][]string{
		{"Please select a Kubernetes context", "", "", ""},
	})

	// Show context selector on startup
	p.cores.Log("[blue]Please select a Kubernetes context to continue...")

	// Queue the context selector to show after UI is drawn using a goroutine
	go func() {
		// Give the UI a moment to initialize
		time.Sleep(100 * time.Millisecond)
		p.app.QueueUpdateDraw(func() {
			p.showContextSelector()
		})
	}()

	return p.pages
}

// Stop cleans up resources when the plugin is unloaded.
func (p *K8sUserPlugin) Stop() {
	if p.cores != nil {
		p.cores.StopAutoRefresh()
		p.cores.UnregisterHandlers()
	}

	if p.pages != nil {
		pageIDs := []string{
			"main",
			"add-rule-modal",
			"create-role-modal",
			"assign-role-modal",
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

	p.userView = nil
	p.roleView = nil
	p.certManager = nil
	p.k8sClient = nil
	p.cores = nil
	p.pages = nil
	p.app = nil
}

// GetMetadata returns plugin metadata.
func (p *K8sUserPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "k8suser",
		Version:     "1.0.0",
		Description: "Kubernetes user and certificate management",
		Author:      "OhMyOps",
		License:     "MIT",
		Tags:        []string{"kubernetes", "security", "certificates", "users"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "",
	}
}

// initializeMainView creates the main view
func (p *K8sUserPlugin) initializeMainView() {
	// Create pattern for initializing the main view
	pattern := ui.ViewPattern{
		App:          p.app,
		Pages:        p.pages,
		Title:        "Kubernetes User Manager",
		HeaderText:   "Manage Kubernetes users with certificate-based authentication",
		TableHeaders: []string{"Username", "Certificate Expiry", "Namespaces", "Roles"},
		RefreshFunc:  p.fetchUsers,
		KeyHandlers: map[string]string{
			"R":  "Refresh",
			"C":  "Create User",
			"D":  "Delete User",
			"A":  "Assign Role",
			"V":  "View Details",
			"T":  "Test Access",
			"E":  "Export Config",
			"K":  "Connection Command",
			"M":  "Role Management",
			"^T": "Switch Context",
			"?":  "Help",
			"^B": "Back",
		},
		SelectedFunc: p.onUserSelected,
	}

	// Initialize the UI
	p.cores = ui.InitializeView(pattern)

	// Set the table selection handler with the right signature
	p.cores.GetTable().Select(0, 0).SetSelectedFunc(func(row, column int) {
		p.onUserSelected(row)
	})

	// Set up action handler
	p.setupActionHandler()

	// Add the core UI to the pages
	p.pages.AddPage("main", p.cores.GetLayout(), true, true)

	// Push initial view to navigation stack
	p.cores.PushView("K8s Users")

	// Set the current view to users
	p.currentView = "users"

	// Log initial state
	p.cores.Log("Plugin initialized")
}

// setupActionHandler configures the action handler for the plugin
func (p *K8sUserPlugin) setupActionHandler() {
	p.cores.SetActionCallback(func(action string, payload map[string]interface{}) error {
		if action == "keypress" {
			if key, ok := payload["key"].(string); ok {
				switch p.currentView {
				case "users":
					// User view actions
					switch key {
					case "R":
						p.refreshUsers()
					case "C":
						p.userView.showCreateUserModal()
					case "D":
						p.userView.showDeleteUserModal()
					case "A":
						p.userView.showAssignRoleModal()
					case "V":
						p.userView.showUserDetails()
					case "T":
						p.userView.showTestAccessModal()
					case "E":
						p.userView.exportUserConfig()
					case "K":
						p.userView.showConnectionCommand()
					case "M":
						p.switchToRolesView()
					case "?":
						p.showHelpModal()
					case "^B":
						p.returnToPreviousView()
					}
				case "roles":
					// Role view actions
					switch key {
					case "R":
						p.refreshRoles()
					case "C":
						p.roleView.showCreateRoleModal()
					case "D":
						p.roleView.showDeleteRoleModal()
					case "V":
						p.roleView.showRoleDetailsModal()
					case "U":
						p.switchToUsersView()
					case "?":
						p.showHelpModal()
					case "^B":
						p.returnToPreviousView()
					}
				default:
					// Default actions for any view
					switch key {
					case "R":
						p.refreshUsers()
					case "?":
						p.showHelpModal()
					}
				}
			}
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
		}
		return nil
	})
}

// fetchUsers retrieves Kubernetes users and formats them for display
func (p *K8sUserPlugin) fetchUsers() ([][]string, error) {
	// Debug log start
	p.cores.Log("[yellow]Debug: fetchUsers called")

	if p.k8sClient == nil || p.k8sClient.CurrentContext == "" {
		p.cores.Log("[yellow]Debug: No k8sClient or current context")
		return [][]string{{"Please select a Kubernetes context", "", "", ""}}, nil
	}

	users, err := p.k8sClient.GetUsers()
	if err != nil {
		p.cores.Log(fmt.Sprintf("[red]Error fetching users: %v", err))
		return [][]string{{"Error fetching users", err.Error(), "", ""}}, nil
	}

	// Debug log user count
	p.cores.Log(fmt.Sprintf("[yellow]Debug: GetUsers returned %d users", len(users)))

	// Store the users in the k8sClient for selection operations
	p.k8sClient.Users = users

	if len(users) == 0 {
		return [][]string{{"No certificate-based users found", "Use 'C' to create", "", ""}}, nil
	}

	result := make([][]string, 0, len(users))

	for i, user := range users {
		// Debug log each user
		p.cores.Log(fmt.Sprintf("[yellow]Debug: User[%d]: %s", i, user.Username))

		result = append(result, []string{
			user.Username,
			user.CertExpiry,
			user.Namespace,
			user.Roles,
		})
	}

	// Update the header text with current context
	p.cores.SetInfoText(fmt.Sprintf("Kubernetes User Manager | Context: %s", p.k8sClient.CurrentContext))

	return result, nil
}

// refreshUsers refreshes the user list
func (p *K8sUserPlugin) refreshUsers() {
	p.cores.RefreshData()
}

// showContextSelector displays a modal to select a Kubernetes context
func (p *K8sUserPlugin) showContextSelector() {
	contexts, err := p.k8sClient.GetContexts()
	if err != nil {
		p.cores.Log(fmt.Sprintf("[red]Error getting Kubernetes contexts: %v", err))
		return
	}

	if len(contexts) == 0 {
		p.cores.Log("[red]No Kubernetes contexts found in your kubeconfig")
		return
	}

	// Format contexts for the modal
	items := make([][]string, 0, len(contexts))
	for _, ctx := range contexts {
		// Highlight the current context
		displayName := ctx
		if ctx == p.k8sClient.CurrentContext {
			displayName += " (current)"
		}
		items = append(items, []string{displayName, ""})
	}

	ui.ShowStandardListSelectorModal(
		p.pages,
		p.app,
		"Select Kubernetes Context",
		items,
		func(index int, text string, cancelled bool) {
			if cancelled || index < 0 {
				// If no context is selected, show a message
				if p.k8sClient.CurrentContext == "" {
					p.cores.Log("[yellow]No context selected. Please select a context to continue.")
				}
				return
			}

			// Set the selected context
			selectedContext := contexts[index]

			// Skip if same context
			if selectedContext == p.k8sClient.CurrentContext {
				p.app.SetFocus(p.cores.GetTable())
				return
			}

			// Switch the context
			err := p.k8sClient.SetContext(selectedContext)
			if err != nil {
				p.cores.Log(fmt.Sprintf("[red]Error setting context: %v", err))
				return
			}

			p.cores.Log(fmt.Sprintf("[green]Switched to context: %s", selectedContext))

			// Refresh the user list
			p.refreshUsers()

			// Set focus to the table
			p.app.SetFocus(p.cores.GetTable())
		},
	)
}

// onUserSelected is called when a user is selected in the table
func (p *K8sUserPlugin) onUserSelected(row int) {
	// Debug log
	p.cores.Log(fmt.Sprintf("[yellow]Debug: onUserSelected called with row %d", row))

	// Debug state info
	p.cores.Log(fmt.Sprintf("[yellow]Debug: k8sClient is nil? %v", p.k8sClient == nil))

	if p.k8sClient == nil {
		p.cores.Log("[red]Error: k8sClient is nil")
		return
	}

	p.cores.Log(fmt.Sprintf("[yellow]Debug: Users length: %d", len(p.k8sClient.Users)))

	if row < 0 || row >= len(p.k8sClient.Users) {
		p.cores.Log(fmt.Sprintf("[red]Error: Invalid row %d (not in range 0-%d)", row, len(p.k8sClient.Users)-1))
		return
	}

	// Debug selected user
	user := p.k8sClient.Users[row]
	p.cores.Log(fmt.Sprintf("[yellow]Debug: Selected user: %s at row %d", user.Username, row))

	// Log the selected user
	p.cores.Log(fmt.Sprintf("[blue]Selected user: %s", user.Username))

	// We don't call showUserDetails here anymore since the user presses Enter to see details
}

// showHelpModal displays the help information
func (p *K8sUserPlugin) showHelpModal() {
	// Create the help content with sections
	content := `[yellow]Kubernetes User Manager Help[white]

[green]Navigation[white]
  [aqua]↑/↓[white] - Navigate between items
  [aqua]Enter[white] - Select an item

[green]User Management[white]
  [aqua]R[white] - Refresh list
  [aqua]C[white] - Create a new user with certificate
  [aqua]D[white] - Delete selected user
  [aqua]A[white] - Assign role to user
  [aqua]V[white] - View user details
  [aqua]T[white] - Test user access
  [aqua]E[white] - Export user kubeconfig
  [aqua]K[white] - Show kubectl connection command

[green]Role Management[white]
  [aqua]M[white] - Switch to role management view
  [aqua]U[white] - Switch to user management view
  [aqua]C[white] - Create a new custom role
  [aqua]D[white] - Delete selected role
  [aqua]V[white] - View role details

[green]Global Keys[white]
  [aqua]Ctrl+T[white] - Switch Kubernetes context
  [aqua]Esc[white] - Go back to previous view
  [aqua]Ctrl+B[white] - Go back to previous view`

	// Show the info modal with a callback to return focus to the table
	ui.ShowInfoModal(
		p.pages,
		p.app,
		"Kubernetes User Manager Help",
		content,
		func() {
			// Return focus to the table when modal is closed
			p.app.SetFocus(p.cores.GetTable())
		},
	)
}

// switchToView updates the current view and UI based on the view name
func (p *K8sUserPlugin) switchToView(viewName string) {
	// Set the current view based on the view name
	switch viewName {
	case "K8s Users":
		p.currentView = "users"
		p.cores.SetTableHeaders([]string{"Username", "Certificate Expiry", "Namespaces", "Roles"})
		p.cores.SetRefreshCallback(p.fetchUsers)
		p.cores.SetInfoText(fmt.Sprintf("Kubernetes User Manager | Context: %s", p.k8sClient.CurrentContext))

		// Clear existing key bindings and set new ones for users view
		p.cores.ClearKeyBindings()
		p.cores.AddKeyBinding("R", "Refresh", p.refreshUsers)
		p.cores.AddKeyBinding("C", "Create User", nil)
		p.cores.AddKeyBinding("D", "Delete User", nil)
		p.cores.AddKeyBinding("A", "Assign Role", nil)
		p.cores.AddKeyBinding("V", "View Details", nil)
		p.cores.AddKeyBinding("T", "Test Access", nil)
		p.cores.AddKeyBinding("E", "Export Config", nil)
		p.cores.AddKeyBinding("K", "Connection Command", nil)
		p.cores.AddKeyBinding("M", "Manage Roles", nil)
		p.cores.AddKeyBinding("?", "Help", nil)
		p.cores.AddKeyBinding("ESC", "Back", nil)

	case "K8s Roles":
		p.currentView = "roles"
		p.cores.SetTableHeaders([]string{"Name", "Namespace", "Resources"})
		p.cores.SetRefreshCallback(p.roleView.fetchRoles)
		p.cores.SetInfoText(fmt.Sprintf("Kubernetes Role Manager | Context: %s", p.k8sClient.CurrentContext))

		// Clear existing key bindings and set new ones for roles view
		p.cores.ClearKeyBindings()
		p.cores.AddKeyBinding("R", "Refresh", p.refreshRoles)
		p.cores.AddKeyBinding("C", "Create Role", nil)
		p.cores.AddKeyBinding("D", "Delete Role", nil)
		p.cores.AddKeyBinding("V", "View Details", nil)
		p.cores.AddKeyBinding("U", "Users View", nil)
		p.cores.AddKeyBinding("?", "Help", nil)
		p.cores.AddKeyBinding("ESC", "Back", nil)

	default:
		// If we don't recognize the view, default to user view
		p.currentView = "users"
		p.cores.SetTableHeaders([]string{"Username", "Certificate Expiry", "Namespaces", "Roles"})
		p.cores.SetRefreshCallback(p.fetchUsers)
		p.cores.SetInfoText(fmt.Sprintf("Kubernetes User Manager | Context: %s", p.k8sClient.CurrentContext))

		// Clear existing key bindings and set new ones for users view
		p.cores.ClearKeyBindings()
		p.cores.AddKeyBinding("R", "Refresh", p.refreshUsers)
		p.cores.AddKeyBinding("C", "Create User", nil)
		p.cores.AddKeyBinding("D", "Delete User", nil)
		p.cores.AddKeyBinding("A", "Assign Role", nil)
		p.cores.AddKeyBinding("V", "View Details", nil)
		p.cores.AddKeyBinding("T", "Test Access", nil)
		p.cores.AddKeyBinding("E", "Export Config", nil)
		p.cores.AddKeyBinding("K", "Connection Command", nil)
		p.cores.AddKeyBinding("M", "Manage Roles", nil)
		p.cores.AddKeyBinding("?", "Help", nil)
		p.cores.AddKeyBinding("ESC", "Back", nil)
	}

	// Refresh data to update the view
	p.cores.RefreshData()
}

// returnToPreviousView goes back one step in the view stack
func (p *K8sUserPlugin) returnToPreviousView() {
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
		// Default to users view if no previous view exists
		p.switchToUsersView()
	}
}

// safeGo runs a function in a goroutine with panic recovery
func safeGo(f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Don't use fmt.Printf as it causes UI issues
				// This could be logged to a file or handled differently if needed
				// If using in a UI context, ideally would use UI.cores.Log
			}
		}()
		f()
	}()
}

// switchToRolesView switches to the role management view
func (p *K8sUserPlugin) switchToRolesView() {
	// Update current view
	p.currentView = "roles"

	// Push view to navigation stack
	p.cores.PushView("K8s Roles")

	// Update UI
	p.cores.SetInfoText(fmt.Sprintf("Kubernetes Role Manager | Context: %s", p.k8sClient.CurrentContext))
	p.cores.SetTableHeaders([]string{"Name", "Namespace", "Resources"})

	// Clear existing key bindings and set new ones for roles view
	p.cores.ClearKeyBindings()
	p.cores.AddKeyBinding("R", "Refresh", p.refreshRoles)
	p.cores.AddKeyBinding("C", "Create Role", nil)
	p.cores.AddKeyBinding("D", "Delete Role", nil)
	p.cores.AddKeyBinding("V", "View Details", nil)
	p.cores.AddKeyBinding("U", "Users View", nil)
	p.cores.AddKeyBinding("?", "Help", nil)
	p.cores.AddKeyBinding("ESC", "Back", nil)

	// Set refresh callback for roles
	p.cores.SetRefreshCallback(p.roleView.fetchRoles)

	// Refresh data
	p.cores.RefreshData()
}

// switchToUsersView switches to the user management view
func (p *K8sUserPlugin) switchToUsersView() {
	// Update current view
	p.currentView = "users"

	// Push view to navigation stack
	p.cores.PushView("K8s Users")

	// Update UI
	p.cores.SetInfoText(fmt.Sprintf("Kubernetes User Manager | Context: %s", p.k8sClient.CurrentContext))
	p.cores.SetTableHeaders([]string{"Username", "Certificate Expiry", "Namespaces", "Roles"})

	// Clear existing key bindings and set new ones for users view
	p.cores.ClearKeyBindings()
	p.cores.AddKeyBinding("R", "Refresh", p.refreshUsers)
	p.cores.AddKeyBinding("C", "Create User", nil)
	p.cores.AddKeyBinding("D", "Delete User", nil)
	p.cores.AddKeyBinding("A", "Assign Role", nil)
	p.cores.AddKeyBinding("V", "View Details", nil)
	p.cores.AddKeyBinding("T", "Test Access", nil)
	p.cores.AddKeyBinding("E", "Export Config", nil)
	p.cores.AddKeyBinding("K", "Connection Command", nil)
	p.cores.AddKeyBinding("M", "Manage Roles", nil)
	p.cores.AddKeyBinding("?", "Help", nil)
	p.cores.AddKeyBinding("ESC", "Back", nil)

	// Set refresh callback for users
	p.cores.SetRefreshCallback(p.fetchUsers)

	// Refresh data
	p.cores.RefreshData()
}

// refreshRoles refreshes the role list
func (p *K8sUserPlugin) refreshRoles() {
	p.cores.RefreshData()
}
