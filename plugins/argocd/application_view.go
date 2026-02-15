package main

import (
	"fmt"
	"strings"

	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// ApplicationView handles the application management view
type ApplicationView struct {
	app          *tview.Application
	pages        *tview.Pages
	cores        *ui.CoreView
	apiClient    *ArgoAPIClient
	applications []Application
}

// NewApplicationView creates a new application view
func NewApplicationView(app *tview.Application, pages *tview.Pages, cores *ui.CoreView, apiClient *ArgoAPIClient) *ApplicationView {
	return &ApplicationView{
		app:          app,
		pages:        pages,
		cores:        cores,
		apiClient:    apiClient,
		applications: []Application{},
	}
}

// SetupTableHandlers sets up the handlers for table events
func (v *ApplicationView) SetupTableHandlers() {
	if v.cores == nil {
		Debug("Cannot set up table handlers: cores is nil")
		return
	}

	// Set up selected row handler
	v.cores.SetRowSelectedCallback(v.onApplicationSelected)

	Debug("Set up table handlers for application view")
}

// onApplicationSelected is called when an application is selected in the table
func (v *ApplicationView) onApplicationSelected(row int) {
	// Make sure the row is valid
	if v.cores == nil {
		Debug("Cores reference is nil in onApplicationSelected")
		return
	}

	Debug("onApplicationSelected called with row %d", row)
	Debug("Applications length: %d", len(v.applications))

	if row < 0 || row >= len(v.applications) {
		Debug("Invalid row %d (not in range 0-%d)", row, len(v.applications)-1)
		return
	}

	// Get the selected application
	app := v.applications[row]
	Debug("Selected application: %s at row %d", app.Name, row)

	// Update status bar or other UI elements with selected app info
	if app.Name != "" {
		v.cores.Log(fmt.Sprintf("[blue]Selected application: %s (Project: %s, Health: %s, Sync: %s)",
			app.Name, app.Project, app.Health.Status, app.Sync.Status))
	}
}

// fetchApplications gets the list of applications from ArgoCD
func (v *ApplicationView) fetchApplications() ([][]string, error) {
	Debug("fetchApplications called in ApplicationView")

	// Check if connected
	if !v.apiClient.IsConnected {
		Debug("API client not connected")
		return [][]string{
			{"Not connected to ArgoCD", "", "", ""},
		}, nil
	}

	Debug("API client is connected, calling GetApplications()")

	// Fetch applications
	applications, err := v.apiClient.GetApplications()
	if err != nil {
		Debug("Error fetching applications: %v", err)
		return [][]string{
			{fmt.Sprintf("Error: %v", err), "", "", ""},
		}, err
	}

	Debug("Fetched %d applications in ApplicationView", len(applications))

	// Debug print each application's complete structure
	for i, app := range applications {
		Debug("Application %d structure: %+v", i, app)
	}

	// Store for later use
	v.applications = applications

	// Format data for display
	result := [][]string{}

	if len(applications) == 0 {
		Debug("No applications found")
		return [][]string{
			{"No applications found", "", "", ""},
		}, nil
	}

	for i, app := range applications {
		Debug("Processing application %d: %s", i, app.Name)

		// Make sure application name is not empty
		if app.Name == "" {
			if app.Metadata != nil {
				if name, ok := app.Metadata["name"].(string); ok && name != "" {
					app.Name = name
				} else {
					app.Name = fmt.Sprintf("Unnamed App %d", i)
				}
			} else {
				app.Name = fmt.Sprintf("Unnamed App %d", i)
			}
		}

		// Extract project name with enhanced fallbacks
		projectName := app.Project
		if projectName == "" || projectName == "-" {
			// Try from spec
			if app.Spec != nil {
				if project, ok := app.Spec["project"].(string); ok && project != "" {
					projectName = project
				}
			}

			// Try from metadata
			if projectName == "" || projectName == "-" {
				if app.Metadata != nil {
					// Direct project field
					if project, ok := app.Metadata["project"].(string); ok && project != "" {
						projectName = project
					} else if labels, ok := app.Metadata["labels"].(map[string]interface{}); ok {
						// Labels
						if project, ok := labels["argocd.argoproj.io/project"].(string); ok && project != "" {
							projectName = project
						}
					}
				}
			}

			// Try from status
			if (projectName == "" || projectName == "-") && app.Status != nil {
				// Direct from status
				if project, ok := app.Status["project"].(string); ok && project != "" {
					projectName = project
				} else if spec, ok := app.Status["spec"].(map[string]interface{}); ok {
					// From status.spec
					if project, ok := spec["project"].(string); ok && project != "" {
						projectName = project
					}
				}
			}

			// If still empty after all attempts, try to get from the URL
			if projectName == "" || projectName == "-" {
				// Some ArgoCD instances use URL paths that include the project name
				// Example: /applications/[project]/[app-name]
				// We can try to extract it if available
				if app.Metadata != nil {
					if selfLink, ok := app.Metadata["selfLink"].(string); ok && selfLink != "" {
						parts := strings.Split(selfLink, "/")
						if len(parts) >= 3 {
							// URL format might be /applications/[project]/[name]
							for i, part := range parts {
								if part == "applications" && i+1 < len(parts) {
									potentialProject := parts[i+1]
									if potentialProject != "" && potentialProject != app.Name {
										projectName = potentialProject
										break
									}
								}
							}
						}
					}
				}
			}

			// If still empty, use "default" as a reasonable fallback
			if projectName == "" || projectName == "-" {
				projectName = "default"
			}
		}

		// Extract health status
		healthStatus := app.Health.Status
		if healthStatus == "" || healthStatus == "Unknown" || healthStatus == "-" {
			// Try to extract health from status field if not already set
			if app.Status != nil {
				if health, ok := app.Status["health"].(map[string]interface{}); ok {
					if status, ok := health["status"].(string); ok && status != "" {
						healthStatus = status
					}
				}
			}

			// If still not found, check direct properties
			if (healthStatus == "" || healthStatus == "Unknown" || healthStatus == "-") && app.Status != nil {
				if status, ok := app.Status["healthStatus"].(string); ok && status != "" {
					healthStatus = status
				}
			}
		}

		// Extract sync status
		syncStatus := app.Sync.Status
		if syncStatus == "" || syncStatus == "Unknown" || syncStatus == "-" {
			// Try to extract sync status from status field if not already set
			if app.Status != nil {
				if sync, ok := app.Status["sync"].(map[string]interface{}); ok {
					if status, ok := sync["status"].(string); ok && status != "" {
						syncStatus = status
					}
				}
			}

			// If still not found, check direct properties
			if (syncStatus == "" || syncStatus == "Unknown" || syncStatus == "-") && app.Status != nil {
				if status, ok := app.Status["syncStatus"].(string); ok && status != "" {
					syncStatus = status
				}
			}
		}

		Debug("  Adding app row: Name=%s, Project=%s, Health=%s, Sync=%s",
			app.Name, projectName, healthStatus, syncStatus)

		result = append(result, []string{
			app.Name,
			projectName,
			healthStatus,
			syncStatus,
		})
	}

	Debug("Returning %d application rows from ApplicationView", len(result))
	return result, nil
}

// getSelectedApplication returns the currently selected application
func (v *ApplicationView) getSelectedApplication() *Application {
	// Ensure the cores reference is set
	if v.cores == nil {
		Debug("ApplicationView cores reference is nil")
		return nil
	}

	// Get the selected row
	row := v.cores.GetSelectedRow()
	Debug("ApplicationView selected row: %d, applications length: %d", row, len(v.applications))

	// Check if row is valid
	if row < 0 || row >= len(v.applications) {
		return nil
	}

	// Return the selected application
	return &v.applications[row]
}

// showApplicationDetailsModal displays details of the selected application
func (v *ApplicationView) showApplicationDetailsModal() {
	// Get selected application
	app := v.getSelectedApplication()
	if app == nil {
		v.cores.Log("[red]Please select an application to view")
		return
	}

	// Get fresh application data
	var err error
	app, err = v.apiClient.GetApplication(app.Name)
	if err != nil {
		v.cores.Log(fmt.Sprintf("[red]Failed to get application details: %v", err))
		return
	}

	// Get status info
	resources := ""
	if statusResources, ok := app.Status["resources"].([]interface{}); ok {
		for i, resource := range statusResources {
			if i > 5 { // Limit to 5 resources to avoid large modal
				resources += "... and more\n"
				break
			}
			if res, ok := resource.(map[string]interface{}); ok {
				kind := res["kind"]
				name := res["name"]
				status := res["status"]
				resources += fmt.Sprintf("- %s/%s: %s\n", kind, name, status)
			}
		}
	}
	if resources == "" {
		resources = "No resources information available\n"
	}

	// Create content for modal
	content := fmt.Sprintf(`[yellow]Application: %s[white]

[green]Project:[white] %s

[green]Health:[white] %s
%s

[green]Sync Status:[white] %s
Revision: %s

[green]Resources:[white]
%s

[green]Deployment:[white]
Namespace: %s
Server: %s
`,
		app.Name,
		app.Project,
		app.Health.Status,
		app.Health.Message,
		app.Sync.Status,
		app.Sync.Revision,
		resources,
		app.Namespace,
		app.Server,
	)

	// Show modal
	ui.ShowInfoModal(
		v.pages,
		v.app,
		fmt.Sprintf("Application Details: %s", app.Name),
		content,
		func() {
			v.app.SetFocus(v.cores.GetTable())
		},
	)
}

// showCreateApplicationModal displays a modal to create a new application
func (v *ApplicationView) showCreateApplicationModal() {
	// For this simplified implementation, show a message that this feature requires more complex implementation
	v.cores.Log("[yellow]Creating an application requires a complex form with many options.")
	v.cores.Log("[yellow]This feature would typically use a multi-page form wizard in a real implementation.")

	ui.ShowInfoModal(
		v.pages,
		v.app,
		"Create Application",
		"Creating an ArgoCD application requires many configuration options including:\n\n"+
			"- Source repository details\n"+
			"- Path to manifests or Helm chart\n"+
			"- Destination server and namespace\n"+
			"- Sync policy and automation settings\n\n"+
			"This would be implemented as a multi-step wizard in a complete implementation.",
		func() {
			v.app.SetFocus(v.cores.GetTable())
		},
	)
}

// showDeleteApplicationModal displays a modal to delete an application
func (v *ApplicationView) showDeleteApplicationModal() {
	// Get selected application
	app := v.getSelectedApplication()
	if app == nil {
		v.cores.Log("[red]Please select an application to delete")
		return
	}

	// Show confirmation dialog
	ui.ShowStandardConfirmationModal(
		v.pages,
		v.app,
		"Delete Application",
		fmt.Sprintf("Are you sure you want to delete the application '%s'?", app.Name),
		func(confirmed bool) {
			if !confirmed {
				return
			}

			// Show progress
			pm := ui.ShowProgressModal(
				v.pages, v.app, "Deleting application", 100, true,
				nil, true,
			)

			// Delete application
			appName := app.Name
			safeGo(func() {
				err := v.apiClient.DeleteApplication(appName)
				if err != nil {
					v.app.QueueUpdateDraw(func() {
						pm.Close()
						v.cores.Log(fmt.Sprintf("[red]Failed to delete application: %v", err))
					})
					return
				}

				v.app.QueueUpdateDraw(func() {
					pm.Close()
					v.cores.Log(fmt.Sprintf("[green]Deleted application: %s", appName))
					v.cores.RefreshData()
				})
			})
		},
	)
}

// showSyncApplicationModal displays a modal to sync an application
func (v *ApplicationView) showSyncApplicationModal() {
	// Get selected application
	app := v.getSelectedApplication()
	if app == nil {
		v.cores.Log("[red]Please select an application to sync")
		return
	}

	// Show confirmation dialog
	ui.ShowStandardConfirmationModal(
		v.pages,
		v.app,
		"Sync Application",
		fmt.Sprintf("Are you sure you want to sync the application '%s'?", app.Name),
		func(confirmed bool) {
			if !confirmed {
				return
			}

			// Show progress
			pm := ui.ShowProgressModal(
				v.pages, v.app, "Syncing application", 100, true,
				nil, true,
			)

			// Sync application
			appName := app.Name
			safeGo(func() {
				err := v.apiClient.SyncApplication(appName)
				if err != nil {
					v.app.QueueUpdateDraw(func() {
						pm.Close()
						v.cores.Log(fmt.Sprintf("[red]Failed to sync application: %v", err))
					})
					return
				}

				v.app.QueueUpdateDraw(func() {
					pm.Close()
					v.cores.Log(fmt.Sprintf("[green]Triggered sync for application: %s", appName))
					v.cores.RefreshData()
				})
			})
		},
	)
}

// showRefreshApplicationModal refreshes the selected application's status
func (v *ApplicationView) showRefreshApplicationModal() {
	// Get selected application
	app := v.getSelectedApplication()
	if app == nil {
		v.cores.Log("[red]Please select an application to refresh")
		return
	}

	// Show progress
	pm := ui.ShowProgressModal(
		v.pages, v.app, "Refreshing application status", 100, true,
		nil, true,
	)

	// Refresh application
	appName := app.Name
	safeGo(func() {
		err := v.apiClient.RefreshApplication(appName)
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				pm.Close()
				v.cores.Log(fmt.Sprintf("[red]Failed to refresh application: %v", err))
			})
			return
		}

		v.app.QueueUpdateDraw(func() {
			pm.Close()
			v.cores.Log(fmt.Sprintf("[green]Refreshed application status: %s", appName))
			v.cores.RefreshData()
		})
	})
}
