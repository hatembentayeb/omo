package main

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"omo/ui"
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

	// Initialize views
	p.initializeMainView()

	// Initialize the user view
	p.userView = NewUserView(p.app, p.pages, p.cores, p.certManager, p.k8sClient)

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

// GetMetadata returns plugin metadata
func (p *K8sUserPlugin) GetMetadata() interface{} {
	return map[string]interface{}{
		"Name":        "k8suser",
		"Version":     "1.0.0",
		"Description": "Kubernetes user and certificate management",
		"Author":      "OhMyOps",
		"License":     "MIT",
		"Tags":        []string{"kubernetes", "security", "certificates", "users"},
		"LastUpdated": time.Now().Format("Jan 2006"),
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
		TableHeaders: []string{"Username", "Namespace", "Roles", "Certificate Expiry"},
		RefreshFunc:  p.fetchUsers,
		KeyHandlers: map[string]string{
			"R":  "Refresh",
			"C":  "Create User",
			"D":  "Delete User",
			"A":  "Assign Role",
			"V":  "View Details",
			"T":  "Test Access",
			"E":  "Export Config",
			"^T": "Switch Context",
			"?":  "Help",
			"^B": "Back",
		},
		SelectedFunc: p.onUserSelected,
	}

	// Initialize the UI
	p.cores = ui.InitializeView(pattern)

	// Set up action handler
	p.setupActionHandler()

	// Add the core UI to the pages
	p.pages.AddPage("main", p.cores.GetLayout(), true, true)

	// Push initial view to navigation stack
	p.cores.PushView("K8s Users")

	// Log initial state
	p.cores.Log("Plugin initialized")
}

// setupActionHandler configures the action handler for the plugin
func (p *K8sUserPlugin) setupActionHandler() {
	p.cores.SetActionCallback(func(action string, payload map[string]interface{}) error {
		if action == "keypress" {
			if key, ok := payload["key"].(string); ok {
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
				case "?":
					p.showHelpModal()
				case "^B":
					p.returnToPreviousView()
				}
			}
		} else if action == "navigate_back" {
			currentView := p.cores.GetCurrentView()
			p.switchToView(currentView)
		}
		return nil
	})
}

// fetchUsers retrieves Kubernetes users and formats them for display
func (p *K8sUserPlugin) fetchUsers() ([][]string, error) {
	if p.k8sClient == nil || p.k8sClient.CurrentContext == "" {
		return [][]string{{"Please select a Kubernetes context", "", "", ""}}, nil
	}

	users, err := p.k8sClient.GetUsers()
	if err != nil {
		return [][]string{{"Error fetching users", err.Error(), "", ""}}, nil
	}

	if len(users) == 0 {
		return [][]string{{"No certificate-based users found", "Use 'C' to create", "", ""}}, nil
	}

	result := make([][]string, 0, len(users))

	for _, user := range users {
		result = append(result, []string{
			user.Username,
			user.Namespace,
			user.Roles,
			user.CertExpiry,
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

// onUserSelected handles when a user is selected in the table
func (p *K8sUserPlugin) onUserSelected(row int) {
	// Show user details when a row is selected
	p.userView.showUserDetails()
}

// showHelpModal displays the help information
func (p *K8sUserPlugin) showHelpModal() {
	// Create the help content with sections
	content := `[yellow]Kubernetes User Manager Help[white]

[green]Navigation[white]
  [aqua]↑/↓[white] - Navigate between users
  [aqua]Enter[white] - Select a user

[green]Actions[white]
  [aqua]R[white] - Refresh user list
  [aqua]C[white] - Create a new user with certificate
  [aqua]D[white] - Delete selected user
  [aqua]A[white] - Assign role to user
  [aqua]V[white] - View user details
  [aqua]T[white] - Test user access
  [aqua]E[white] - Export user kubeconfig
  [aqua]Ctrl+T[white] - Switch Kubernetes context
  [aqua]Esc[white] - Go back to previous view
  [aqua]Ctrl+B[white] - Go back to previous view

[green]Certificates[white]
  User certificates are generated using OpenSSL and stored in ~/.k8s-users/
  Each user gets a private key, CSR, and signed certificate.
  Certificates are valid for 1 year by default.`

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

// switchToView updates the current view
func (p *K8sUserPlugin) switchToView(viewName string) {
	p.currentView = viewName
}

// returnToPreviousView returns to the previous view
func (p *K8sUserPlugin) returnToPreviousView() {
	lastView := p.cores.PopView()
	if lastView != "" {
		currentView := p.cores.GetCurrentView()
		p.switchToView(currentView)
	}
}

// safeGo runs a function in a goroutine with panic recovery
func safeGo(f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovered from panic: %v\n", r)
			}
		}()
		f()
	}()
}
