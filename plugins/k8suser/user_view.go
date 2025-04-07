package main

import (
	"fmt"
	"os/exec"

	"omo/ui"

	"github.com/rivo/tview"
)

// UserView handles the user interface for managing Kubernetes users
type UserView struct {
	app         *tview.Application
	pages       *tview.Pages
	cores       *ui.Cores
	certManager *CertManager
	k8sClient   *K8sClient
	selectedRow int
}

// NewUserView creates a new user view
func NewUserView(app *tview.Application, pages *tview.Pages, cores *ui.Cores, certManager *CertManager, k8sClient *K8sClient) *UserView {
	return &UserView{
		app:         app,
		pages:       pages,
		cores:       cores,
		certManager: certManager,
		k8sClient:   k8sClient,
		selectedRow: -1,
	}
}

// showCreateUserModal shows a modal for creating a new user
func (uv *UserView) showCreateUserModal() {
	// Create an input field for the username
	inputField := tview.NewInputField().
		SetLabel("Username: ").
		SetFieldWidth(30).
		SetAcceptanceFunc(func(textToCheck string, lastChar rune) bool {
			// Allow only alphanumeric characters and dashes
			if lastChar == 0 {
				return len(textToCheck) > 0
			}
			return (lastChar >= 'a' && lastChar <= 'z') ||
				(lastChar >= '0' && lastChar <= '9') ||
				lastChar == '-'
		})

	// Create a form with buttons
	form := tview.NewForm().
		AddFormItem(inputField).
		AddButton("Create", func() {
			username := inputField.GetText()
			if username == "" {
				uv.cores.Log("[red]Username cannot be empty")
				return
			}

			// Close the modal
			uv.pages.RemovePage("create-user-modal")

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
					ShowConfirmModal(
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
		}).
		AddButton("Cancel", func() {
			uv.pages.RemovePage("create-user-modal")
		})

	// Create a modal for the form
	modal := CreateStandardFormModal(form, "Create New User", 60, 10)

	// Add the modal to the pages
	uv.pages.AddPage("create-user-modal", modal, true, true)
	uv.app.SetFocus(inputField)
}

// showDeleteUserModal shows a modal for deleting a user
func (uv *UserView) showDeleteUserModal() {
	// Get the selected user
	selectedRow := uv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(uv.k8sClient.Users) {
		uv.cores.Log("[red]No user selected")
		return
	}

	user := uv.k8sClient.Users[selectedRow]

	// Show a confirmation dialog
	ShowConfirmModal(
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
	// Get the selected user
	selectedRow := uv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(uv.k8sClient.Users) {
		uv.cores.Log("[red]No user selected")
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

	// Create a modal for the form
	modal := CreateStandardFormModal(form, "Assign Role to User", 60, 11)

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

	// Add the modal to the pages
	uv.pages.AddPage("assign-role-modal", modal, true, true)
	uv.app.SetFocus(namespaceDropDown)
}

// showUserDetails shows detailed information about a user
func (uv *UserView) showUserDetails() {
	// Get the selected user
	selectedRow := uv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(uv.k8sClient.Users) {
		uv.cores.Log("[red]No user selected")
		return
	}

	user := uv.k8sClient.Users[selectedRow]

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
	// Get the selected user
	selectedRow := uv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(uv.k8sClient.Users) {
		uv.cores.Log("[red]No user selected")
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
		SetTextAlign(tview.AlignLeft)

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

			if namespace == "" {
				resultsView.SetText("[red]Please select a namespace")
				return
			}

			if resource == "" {
				resultsView.SetText("[red]Please select a resource")
				return
			}

			if verb == "" {
				resultsView.SetText("[red]Please select a verb")
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

	// Create a flex layout for the form and results
	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(form, 0, 1, true).
		AddItem(resultsView, 0, 1, false)

	// Create a frame for the flex layout with a border and title
	frame := tview.NewFrame(flex).
		SetBorders(1, 1, 1, 1, 0, 0).
		SetBorder(true).
		SetTitle("Test Access - " + user.Username).
		SetTitleAlign(tview.AlignCenter).
		SetBackgroundColor(tview.Styles.PrimitiveBackgroundColor)

	// Get namespaces
	uv.cores.Log(fmt.Sprintf("[blue]Getting namespaces for testing %s's access...", user.Username))

	// Load namespaces
	safeGo(func() {
		namespaces, err := uv.k8sClient.GetNamespaces()
		if err != nil {
			uv.app.QueueUpdateDraw(func() {
				uv.cores.Log(fmt.Sprintf("[red]Error getting namespaces: %v", err))

				// Default to a single namespace
				namespaceDropDown.SetOptions([]string{"default", "cluster-wide"}, nil)
			})
			return
		}

		// Update the namespace dropdown
		uv.app.QueueUpdateDraw(func() {
			namespaceDropDown.SetOptions(namespaces, nil)
		})
	})

	// Add the modal to the pages with a done handler
	uv.pages.AddPage("test-access-modal", frame, true, true)
	uv.app.SetFocus(namespaceDropDown)
}

// exportUserConfig exports the kubeconfig for a user
func (uv *UserView) exportUserConfig() {
	// Get the selected user
	selectedRow := uv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(uv.k8sClient.Users) {
		uv.cores.Log("[red]No user selected")
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
