package main

import (
	"fmt"
	"os/exec"

	"omo/ui"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// UserView handles the user interface for managing Kubernetes users
type UserView struct {
	app         *tview.Application
	pages       *tview.Pages
	cores       *ui.Cores
	certManager *CertManager
	k8sClient   *K8sClient
}

// NewUserView creates a new user view
func NewUserView(app *tview.Application, pages *tview.Pages, cores *ui.Cores, certManager *CertManager, k8sClient *K8sClient) *UserView {
	return &UserView{
		app:         app,
		pages:       pages,
		cores:       cores,
		certManager: certManager,
		k8sClient:   k8sClient,
	}
}

// showCreateUserModal shows a modal for creating a new user
func (uv *UserView) showCreateUserModal() {
	// Use UI's compact input modal
	ui.ShowCompactStyledInputModal(
		uv.pages,
		uv.app,
		"Create New User",
		"Username",
		"",
		30,
		func(textToCheck string, lastChar rune) bool {
			// Allow only alphanumeric characters and dashes
			if lastChar == 0 {
				return len(textToCheck) > 0
			}
			return (lastChar >= 'a' && lastChar <= 'z') ||
				(lastChar >= '0' && lastChar <= '9') ||
				lastChar == '-'
		},
		func(username string, cancelled bool) {
			if cancelled || username == "" {
				return
			}

			// Show a progress modal
			pm := ui.ShowProgressModal(
				uv.pages, uv.app, "Creating User", 100, true,
				nil, true,
			)

			// Create the user in a goroutine
			safeGo(func() {
				user, err := uv.k8sClient.CreateUser(username)
				if err != nil {
					uv.app.QueueUpdateDraw(func() {
						pm.Close()
						ui.ShowStandardErrorModal(
							uv.pages, uv.app, "User Creation Error",
							fmt.Sprintf("Failed to create user: %v", err),
							nil,
						)
					})
					return
				}

				// Show success message
				uv.app.QueueUpdateDraw(func() {
					pm.Close()
					uv.cores.Log(fmt.Sprintf("[green]User %s created successfully", username))

					// Show a popup to ask if the user wants to assign a role
					ui.ShowStandardConfirmationModal(
						uv.pages, uv.app, "Assign Role",
						fmt.Sprintf("User %s created successfully. Do you want to assign a role?", username),
						func(confirmed bool) {
							if confirmed {
								uv.showAssignRoleModalForUser(user)
							} else {
								// Refresh the user list
								uv.app.QueueUpdateDraw(func() {
									uv.cores.RefreshData()
								})
							}
						},
					)
				})
			})
		},
	)
}

// showDeleteUserModal shows a modal for deleting a user
func (uv *UserView) showDeleteUserModal() {
	// Get currently selected row from the table, accounting for header row
	selectedRow := uv.cores.GetTable().GetSelectedRow() - 1

	// Debug log
	uv.cores.Log(fmt.Sprintf("[yellow]Debug: showDeleteUserModal called, raw selectedRow = %d, adjusted = %d",
		uv.cores.GetTable().GetSelectedRow(), selectedRow))

	if uv.k8sClient == nil {
		uv.cores.Log("[red]Error: k8sClient is nil in UserView")
		return
	}

	uv.cores.Log(fmt.Sprintf("[yellow]Debug: UserView k8sClient user count: %d", len(uv.k8sClient.Users)))

	// Make sure we have a valid selected row
	if selectedRow < 0 || selectedRow >= len(uv.k8sClient.Users) {
		uv.cores.Log(fmt.Sprintf("[red]No user selected (invalid row %d, user count: %d)",
			selectedRow, len(uv.k8sClient.Users)))
		return
	}

	user := uv.k8sClient.Users[selectedRow]

	// Show a confirmation dialog
	ui.ShowStandardConfirmationModal(
		uv.pages, uv.app, "Delete User",
		fmt.Sprintf("Are you sure you want to delete the user %s? This will remove all role bindings for this user.", user.Username),
		func(confirmed bool) {
			if !confirmed {
				return
			}

			// Show a progress modal
			pm := ui.ShowProgressModal(
				uv.pages, uv.app, "Deleting User", 100, true,
				nil, true,
			)

			// Delete the user in a goroutine
			safeGo(func() {
				err := uv.k8sClient.DeleteUser(user.Username)
				if err != nil {
					uv.app.QueueUpdateDraw(func() {
						pm.Close()
						ui.ShowStandardErrorModal(
							uv.pages, uv.app, "User Deletion Error",
							fmt.Sprintf("Failed to delete user: %v", err),
							nil,
						)
					})
					return
				}

				// Show success message
				uv.app.QueueUpdateDraw(func() {
					pm.Close()
					uv.cores.Log(fmt.Sprintf("[green]User %s deleted successfully", user.Username))

					// Refresh the user list
					uv.cores.RefreshData()
				})
			})
		},
	)
}

// showAssignRoleModal shows a modal for assigning a role to a user
func (uv *UserView) showAssignRoleModal() {
	// Get currently selected row from the table, accounting for header row
	selectedRow := uv.cores.GetTable().GetSelectedRow() - 1

	// Debug log
	uv.cores.Log(fmt.Sprintf("[yellow]Debug: showAssignRoleModal called, raw selectedRow = %d, adjusted = %d",
		uv.cores.GetTable().GetSelectedRow(), selectedRow))

	if uv.k8sClient == nil {
		uv.cores.Log("[red]Error: k8sClient is nil in UserView")
		return
	}

	uv.cores.Log(fmt.Sprintf("[yellow]Debug: UserView k8sClient user count: %d", len(uv.k8sClient.Users)))

	// Make sure we have a valid selected row
	if selectedRow < 0 || selectedRow >= len(uv.k8sClient.Users) {
		uv.cores.Log(fmt.Sprintf("[red]No user selected (invalid row %d, user count: %d)",
			selectedRow, len(uv.k8sClient.Users)))
		return
	}

	user := uv.k8sClient.Users[selectedRow]
	uv.showAssignRoleModalForUser(user)
}

// showAssignRoleModalForUser shows a modal for assigning a role to a specific user
func (uv *UserView) showAssignRoleModalForUser(user *K8sUser) {
	// Create dropdown for namespace
	namespaceDropDown := tview.NewDropDown().
		SetLabel("Namespace: ")

	// Create dropdown for role
	roleDropDown := tview.NewDropDown().
		SetLabel("Role:      ")

	// Create a form with dropdowns and buttons
	form := tview.NewForm().
		AddFormItem(namespaceDropDown).
		AddFormItem(roleDropDown).
		AddButton("Assign", func() {
			// Get the selected namespace and role
			_, namespace := namespaceDropDown.GetCurrentOption()
			_, role := roleDropDown.GetCurrentOption()

			if namespace == "" {
				uv.cores.Log("[red]Please select a namespace")
				return
			}

			if role == "" {
				uv.cores.Log("[red]Please select a role")
				return
			}

			// Close the modal
			uv.pages.RemovePage("assign-role-modal")

			// Show a progress modal
			pm := ui.ShowProgressModal(
				uv.pages, uv.app, "Assigning Role", 100, true,
				nil, true,
			)

			// Assign the role in a goroutine
			safeGo(func() {
				err := uv.k8sClient.AssignRoleToUser(user.Username, namespace, role)
				if err != nil {
					uv.app.QueueUpdateDraw(func() {
						pm.Close()
						ui.ShowStandardErrorModal(
							uv.pages, uv.app, "Role Assignment Error",
							fmt.Sprintf("Failed to assign role: %v", err),
							nil,
						)
					})
					return
				}

				// Show success message
				uv.app.QueueUpdateDraw(func() {
					pm.Close()
					uv.cores.Log(fmt.Sprintf("[green]Role %s assigned to user %s in namespace %s", role, user.Username, namespace))

					// Refresh the user list
					uv.cores.RefreshData()
				})
			})
		}).
		AddButton("Cancel", func() {
			uv.pages.RemovePage("assign-role-modal")
		})

	// Style the form according to UI package standards
	form.SetItemPadding(0)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetBackgroundColor(tcell.ColorBlack)
	form.SetButtonBackgroundColor(tcell.ColorBlack)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetFieldBackgroundColor(tcell.ColorBlack)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetBorder(true)
	form.SetTitle(" Assign Role to User ")
	form.SetTitleAlign(tview.AlignCenter)
	form.SetBorderColor(tcell.ColorAqua)
	form.SetTitleColor(tcell.ColorOrange)
	form.SetBorderPadding(1, 1, 2, 2)

	// Style the buttons with focus colors
	for i := 0; i < form.GetButtonCount(); i++ {
		if b := form.GetButton(i); b != nil {
			b.SetBackgroundColor(tcell.ColorBlack)
			b.SetLabelColor(tcell.ColorWhite)
			b.SetBackgroundColorActivated(tcell.ColorWhite)
			b.SetLabelColorActivated(tcell.ColorBlack)
		}
	}

	// Get namespaces
	uv.cores.Log(fmt.Sprintf("[blue]Getting namespaces for user %s...", user.Username))

	// Load namespaces and roles
	safeGo(func() {
		namespaces, err := uv.k8sClient.GetNamespaces()
		if err != nil {
			uv.app.QueueUpdateDraw(func() {
				uv.cores.Log(fmt.Sprintf("[red]Error getting namespaces: %v", err))

				// Default to a single namespace
				namespaceDropDown.SetOptions([]string{"default", "cluster-wide"}, nil)
				roleDropDown.SetOptions([]string{"view", "edit", "admin", "cluster-admin"}, nil)
			})
			return
		}

		// Update the namespace dropdown
		uv.app.QueueUpdateDraw(func() {
			namespaceDropDown.SetOptions(namespaces, func(text string, index int) {
				// When a namespace is selected, update the role dropdown
				uv.cores.Log(fmt.Sprintf("[blue]Getting roles for namespace %s...", text))

				// Show a temporary loading message
				roleDropDown.SetOptions([]string{"Loading..."}, nil)

				safeGo(func() {
					roles, err := uv.k8sClient.GetRoles(text)
					if err != nil {
						uv.app.QueueUpdateDraw(func() {
							uv.cores.Log(fmt.Sprintf("[red]Error getting roles: %v", err))

							// Default to some common roles
							roleDropDown.SetOptions([]string{"view", "edit", "admin", "cluster-admin"}, nil)
						})
						return
					}

					// Update the role dropdown
					uv.app.QueueUpdateDraw(func() {
						roleDropDown.SetOptions(roles, nil)
					})
				})
			})

			// Trigger selection of the first namespace to populate roles
			namespaceDropDown.SetCurrentOption(0)
		})
	})

	// Create a centered flex container
	width := 60
	height := 11

	centerFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	// Add the modal to the pages
	uv.pages.AddPage("assign-role-modal", centerFlex, true, true)

	// Add ESC handler
	ui.RemovePage(uv.pages, uv.app, "assign-role-modal", nil)

	uv.app.SetFocus(namespaceDropDown)
}

// showUserDetails shows detailed information about a user
func (uv *UserView) showUserDetails() {
	// Get currently selected row from the table, accounting for header row
	selectedRow := uv.cores.GetTable().GetSelectedRow() - 1

	// Debug log
	uv.cores.Log(fmt.Sprintf("[yellow]Debug: showUserDetails called, raw selectedRow = %d, adjusted = %d",
		uv.cores.GetTable().GetSelectedRow(), selectedRow))

	if uv.k8sClient == nil {
		uv.cores.Log("[red]Error: k8sClient is nil in UserView")
		return
	}

	uv.cores.Log(fmt.Sprintf("[yellow]Debug: UserView k8sClient user count: %d", len(uv.k8sClient.Users)))

	// Make sure we have a valid selected row
	if selectedRow < 0 || selectedRow >= len(uv.k8sClient.Users) {
		uv.cores.Log(fmt.Sprintf("[red]No user selected (invalid row %d, user count: %d)",
			selectedRow, len(uv.k8sClient.Users)))
		return
	}

	user := uv.k8sClient.Users[selectedRow]
	uv.cores.Log(fmt.Sprintf("[yellow]Debug: Found user at selectedRow %d: %s", selectedRow, user.Username))

	// Push view to navigation stack
	uv.cores.PushView(fmt.Sprintf("User: %s", user.Username))

	// Update UI
	uv.cores.SetTableHeaders([]string{"Property", "Value"})
	uv.cores.SetRefreshCallback(func() ([][]string, error) {
		// Create detailed view for the selected user
		data := [][]string{
			{"Username", user.Username},
			{"Certificate Expiry", user.CertExpiry},
			{"Namespaces", user.Namespace},
			{"Roles", user.Roles},
		}

		// Add certificate file paths if available
		if user.Certificate != nil {
			data = append(data, []string{"Certificate", user.Certificate.Cert})
			data = append(data, []string{"Private Key", user.Certificate.PrivateKey})
			data = append(data, []string{"CSR", user.Certificate.CSR})
		}

		return data, nil
	})

	uv.cores.RefreshData()
}

// showTestAccessModal shows a modal for testing a user's access to resources
func (uv *UserView) showTestAccessModal() {
	// Get currently selected row from the table, accounting for header row
	selectedRow := uv.cores.GetTable().GetSelectedRow() - 1

	// Debug log
	uv.cores.Log(fmt.Sprintf("[yellow]Debug: showTestAccessModal called, raw selectedRow = %d, adjusted = %d",
		uv.cores.GetTable().GetSelectedRow(), selectedRow))

	if uv.k8sClient == nil {
		uv.cores.Log("[red]Error: k8sClient is nil in UserView")
		return
	}

	uv.cores.Log(fmt.Sprintf("[yellow]Debug: UserView k8sClient user count: %d", len(uv.k8sClient.Users)))

	// Make sure we have a valid selected row
	if selectedRow < 0 || selectedRow >= len(uv.k8sClient.Users) {
		uv.cores.Log(fmt.Sprintf("[red]No user selected (invalid row %d, user count: %d)",
			selectedRow, len(uv.k8sClient.Users)))
		return
	}

	user := uv.k8sClient.Users[selectedRow]

	// Common resources to test
	resources := []string{
		"pods", "deployments", "services", "configmaps", "secrets",
		"nodes", "namespaces", "persistentvolumes", "clusterroles",
		"customresourcedefinitions",
	}

	// Verbs to test
	verbs := []string{"get", "list", "watch", "create", "update", "patch", "delete"}

	// Create dropdown for namespace
	namespaceDropDown := tview.NewDropDown().
		SetLabel("Namespace: ")

	// Create dropdown for resource
	resourceDropDown := tview.NewDropDown().
		SetLabel("Resource:  ").
		SetOptions(resources, nil)

	// Create dropdown for verb
	verbDropDown := tview.NewDropDown().
		SetLabel("Verb:      ").
		SetOptions(verbs, nil)

	// Results text view
	resultsView := tview.NewTextView().
		SetDynamicColors(true).
		SetText("Test results will appear here").
		SetTextAlign(tview.AlignLeft).
		SetScrollable(true).
		SetWordWrap(true)

	// Style the results view
	resultsView.SetBorder(true)
	resultsView.SetBorderColor(tcell.ColorAqua)
	resultsView.SetTitle(" Results ")
	resultsView.SetTitleAlign(tview.AlignCenter)
	resultsView.SetTitleColor(tcell.ColorOrange)
	resultsView.SetBackgroundColor(tcell.ColorBlack)
	resultsView.SetTextColor(tcell.ColorWhite)
	resultsView.SetBorderPadding(1, 1, 2, 2)

	// Create a form with dropdowns and buttons
	form := tview.NewForm().
		AddFormItem(namespaceDropDown).
		AddFormItem(resourceDropDown).
		AddFormItem(verbDropDown).
		AddButton("Test", func() {
			// Get the selected namespace, resource, and verb
			_, namespace := namespaceDropDown.GetCurrentOption()
			_, resource := resourceDropDown.GetCurrentOption()
			_, verb := verbDropDown.GetCurrentOption()

			if namespace == "" || namespace == "Loading..." {
				resultsView.SetText("[red]Please select a namespace")
				return
			}

			// Show a testing message
			resultsView.SetText(fmt.Sprintf("[yellow]Testing if user %s can %s %s in namespace %s...",
				user.Username, verb, resource, namespace))

			// Test the access in a goroutine
			safeGo(func() {
				allowed, response, err := uv.k8sClient.TestAccess(user.Username, namespace, resource, verb)
				if err != nil {
					uv.app.QueueUpdateDraw(func() {
						resultsView.SetText(fmt.Sprintf("[red]Error testing access: %v", err))
					})
					return
				}

				// Show the result
				uv.app.QueueUpdateDraw(func() {
					if allowed {
						resultsView.SetText(fmt.Sprintf("[green]User %s CAN %s %s in namespace %s",
							user.Username, verb, resource, namespace))
					} else {
						resultsView.SetText(fmt.Sprintf("[red]User %s CANNOT %s %s in namespace %s\n[yellow]Response: %s",
							user.Username, verb, resource, namespace, response))
					}
				})
			})
		}).
		AddButton("Close", func() {
			uv.pages.RemovePage("test-access-modal")
		})

	// Style the form according to UI package standards
	form.SetItemPadding(1)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetBackgroundColor(tcell.ColorBlack)
	form.SetButtonBackgroundColor(tcell.ColorBlack)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetFieldBackgroundColor(tcell.ColorBlack)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetBorder(true)
	form.SetTitle(" Test Access ")
	form.SetTitleAlign(tview.AlignCenter)
	form.SetBorderColor(tcell.ColorAqua)
	form.SetTitleColor(tcell.ColorOrange)
	form.SetBorderPadding(1, 1, 2, 2)

	// Style the buttons with focus colors
	for i := 0; i < form.GetButtonCount(); i++ {
		if b := form.GetButton(i); b != nil {
			b.SetBackgroundColor(tcell.ColorBlack)
			b.SetLabelColor(tcell.ColorWhite)
			b.SetBackgroundColorActivated(tcell.ColorWhite)
			b.SetLabelColorActivated(tcell.ColorBlack)
		}
	}

	// Create a flex layout for form and results
	mainFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(form, 12, 0, true).
		AddItem(resultsView, 8, 0, false)

	// Create a centered flex container like UI package uses
	width := 70
	height := 22

	centerFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(mainFlex, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	// Get namespaces
	uv.cores.Log(fmt.Sprintf("[blue]Getting namespaces for testing %s's access...", user.Username))

	// Load namespaces
	safeGo(func() {
		namespaces, err := uv.k8sClient.GetNamespaces()
		if err != nil {
			uv.app.QueueUpdateDraw(func() {
				uv.cores.Log(fmt.Sprintf("[red]Error getting namespaces: %v", err))
				namespaceDropDown.SetOptions([]string{"default", "cluster-wide"}, nil)
			})
			return
		}

		// Update the namespace dropdown and select the first option
		uv.app.QueueUpdateDraw(func() {
			namespaceDropDown.SetOptions(namespaces, nil)
			if len(namespaces) > 0 {
				namespaceDropDown.SetCurrentOption(0)
			}
		})
	})

	// Add the modal to the pages
	pageID := "test-access-modal"
	uv.pages.AddPage(pageID, centerFlex, true, true)

	// Add ESC handler
	ui.RemovePage(uv.pages, uv.app, pageID, nil)

	uv.app.SetFocus(namespaceDropDown)
}

// exportUserConfig exports the kubeconfig for a user
func (uv *UserView) exportUserConfig() {
	// Get currently selected row from the table, accounting for header row
	selectedRow := uv.cores.GetTable().GetSelectedRow() - 1

	// Debug log
	uv.cores.Log(fmt.Sprintf("[yellow]Debug: exportUserConfig called, raw selectedRow = %d, adjusted = %d",
		uv.cores.GetTable().GetSelectedRow(), selectedRow))

	if uv.k8sClient == nil {
		uv.cores.Log("[red]Error: k8sClient is nil in UserView")
		return
	}

	uv.cores.Log(fmt.Sprintf("[yellow]Debug: UserView k8sClient user count: %d", len(uv.k8sClient.Users)))

	// Make sure we have a valid selected row
	if selectedRow < 0 || selectedRow >= len(uv.k8sClient.Users) {
		uv.cores.Log(fmt.Sprintf("[red]No user selected (invalid row %d, user count: %d)",
			selectedRow, len(uv.k8sClient.Users)))
		return
	}

	user := uv.k8sClient.Users[selectedRow]

	// Check if the user has a certificate
	if user.Certificate == nil {
		uv.cores.Log(fmt.Sprintf("[red]No certificate found for user %s", user.Username))
		return
	}

	// Show a progress modal
	pm := ui.ShowProgressModal(
		uv.pages, uv.app, "Exporting User Config", 100, true,
		nil, true,
	)

	// Export the config in a goroutine
	safeGo(func() {
		// Get the current server URL from kubectl
		cmd := exec.Command("kubectl", "config", "view", "--minify", "-o", "jsonpath={.clusters[].cluster.server}")
		output, err := cmd.CombinedOutput()
		if err != nil {
			uv.app.QueueUpdateDraw(func() {
				pm.Close()
				ui.ShowStandardErrorModal(
					uv.pages, uv.app, "Config Export Error",
					fmt.Sprintf("Failed to get server URL: %v", err),
					nil,
				)
			})
			return
		}
		serverURL := string(output)

		kubeConfigPath, err := uv.certManager.GenerateKubeConfig(user.Certificate, serverURL)
		if err != nil {
			uv.app.QueueUpdateDraw(func() {
				pm.Close()
				ui.ShowStandardErrorModal(
					uv.pages, uv.app, "Config Export Error",
					fmt.Sprintf("Failed to export config: %v", err),
					nil,
				)
			})
			return
		}

		// Show success message
		uv.app.QueueUpdateDraw(func() {
			pm.Close()

			// Show info modal with the path to the config
			content := fmt.Sprintf(`[yellow]Kubeconfig for %s created successfully[white]

The kubeconfig file has been saved to:
[green]%s[white]

To use this config file, run commands like:
[blue]kubectl --kubeconfig=%s get pods[white]

Or export it as your KUBECONFIG:
[blue]export KUBECONFIG=%s[white]`, user.Username, kubeConfigPath, kubeConfigPath, kubeConfigPath)

			ui.ShowInfoModal(
				uv.pages, uv.app, "Kubeconfig Exported",
				content,
				func() {
					// Return focus to the table
					uv.app.SetFocus(uv.cores.GetTable())
				},
			)
		})
	})
}

// showConnectionCommand shows the kubectl command to connect as the selected user
func (uv *UserView) showConnectionCommand() {
	// Get currently selected row from the table, accounting for header row
	selectedRow := uv.cores.GetTable().GetSelectedRow() - 1

	// Debug log
	uv.cores.Log(fmt.Sprintf("[yellow]Debug: showConnectionCommand called, raw selectedRow = %d, adjusted = %d",
		uv.cores.GetTable().GetSelectedRow(), selectedRow))

	if uv.k8sClient == nil {
		uv.cores.Log("[red]Error: k8sClient is nil in UserView")
		return
	}

	uv.cores.Log(fmt.Sprintf("[yellow]Debug: UserView k8sClient user count: %d", len(uv.k8sClient.Users)))

	// Make sure we have a valid selected row
	if selectedRow < 0 || selectedRow >= len(uv.k8sClient.Users) {
		uv.cores.Log(fmt.Sprintf("[red]No user selected (invalid row %d, user count: %d)",
			selectedRow, len(uv.k8sClient.Users)))
		return
	}

	user := uv.k8sClient.Users[selectedRow]

	// Check if the user has a certificate
	if user.Certificate == nil {
		uv.cores.Log(fmt.Sprintf("[red]No certificate found for user %s", user.Username))
		return
	}

	// Generate a kubeconfig for the user
	kubeConfigPath, err := uv.certManager.GenerateKubeConfig(user.Certificate, "")
	if err != nil {
		uv.cores.Log(fmt.Sprintf("[red]Failed to generate kubeconfig: %v", err))
		return
	}

	// Generate the kubeconfig command
	kubeConfigCommand := fmt.Sprintf("kubectl --kubeconfig=%s", kubeConfigPath)

	// Also generate the direct certificate command as fallback
	cmd := exec.Command("kubectl", "config", "view", "--minify", "-o", "jsonpath={.clusters[].cluster.server}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		uv.cores.Log(fmt.Sprintf("[red]Failed to get server URL: %v", err))
		return
	}
	serverURL := string(output)

	certPath := user.Certificate.Cert
	keyPath := user.Certificate.PrivateKey
	directCommand := fmt.Sprintf("kubectl --server=%s --client-certificate=%s --client-key=%s",
		serverURL, certPath, keyPath)

	// Show info modal with the connection commands
	content := fmt.Sprintf(`[yellow]Connection Commands for %s[white]

[green]Method 1 (Recommended): Use the generated kubeconfig file[white]
Use this command to run kubectl as %s:

[green]%s[white]

You can add additional kubectl commands after this, for example:

[green]%s get pods -n default[white]

[yellow]Method 2 (Alternative): Use certificate paths directly[white]
You can also use the certificate paths directly:

[green]%s[white]

For more convenient usage, consider exporting the kubeconfig using the 'E' key.`,
		user.Username, user.Username,
		kubeConfigCommand, kubeConfigCommand,
		directCommand)

	ui.ShowInfoModal(
		uv.pages,
		uv.app,
		"Kubectl Connection Command",
		content,
		func() {
			// Return focus to the table when modal is closed
			uv.app.SetFocus(uv.cores.GetTable())
		},
	)
}
