package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// ProjectView handles the project management view
type ProjectView struct {
	app       *tview.Application
	pages     *tview.Pages
	cores     *ui.CoreView
	apiClient *ArgoAPIClient
	projects  []Project
}

// NewProjectView creates a new project view
func NewProjectView(app *tview.Application, pages *tview.Pages, cores *ui.CoreView, apiClient *ArgoAPIClient) *ProjectView {
	return &ProjectView{
		app:       app,
		pages:     pages,
		cores:     cores,
		apiClient: apiClient,
		projects:  []Project{},
	}
}

// fetchProjects gets the list of projects from ArgoCD
func (v *ProjectView) fetchProjects() ([][]string, error) {
	Debug("fetchProjects called in ProjectView")

	// Check if connected
	if !v.apiClient.IsConnected {
		Debug("API client not connected")
		return [][]string{
			{"Not connected to ArgoCD", "", "", ""},
		}, nil
	}

	Debug("API client is connected, calling GetProjects()")

	// Fetch projects
	projects, err := v.apiClient.GetProjects()
	if err != nil {
		Debug("Error fetching projects: %v", err)
		return [][]string{
			{fmt.Sprintf("Error: %v", err), "", "", ""},
		}, err
	}

	Debug("Fetched %d projects in ProjectView", len(projects))

	// Store for later use
	v.projects = projects

	// Check if we have any projects first
	if len(projects) == 0 {
		Debug("No projects found")
		return [][]string{
			{"No projects found", "", "", ""},
		}, nil
	}

	// Debug print each project's complete structure
	for i, proj := range projects {
		Debug("Project %d structure: %+v", i, proj)
	}

	// Format data for display
	result := [][]string{}
	for i, project := range projects {
		Debug("Processing project %d: %s", i, project.Name)

		// Make sure project name is not empty
		if project.Name == "" {
			if project.Metadata != nil {
				if name, ok := project.Metadata["name"].(string); ok && name != "" {
					project.Name = name
				} else {
					project.Name = fmt.Sprintf("Unnamed Project %d", i)
				}
			} else {
				project.Name = fmt.Sprintf("Unnamed Project %d", i)
			}
		}

		// Format destinations - check if Destinations is nil to avoid panic
		dests := "None"
		if project.Destinations != nil && len(project.Destinations) > 0 {
			destNames := []string{}
			for j, dest := range project.Destinations {
				Debug("  Processing destination %d for project %s", j, project.Name)
				name := dest.Name
				if name == "" {
					if dest.Server != "" {
						name = dest.Server
					} else if dest.Namespace != "" {
						name = dest.Namespace
					} else {
						name = "unnamed"
					}
				}
				destNames = append(destNames, name)
			}
			dests = strings.Join(destNames, ", ")
		} else if project.Spec != nil {
			// Try to get destinations from spec
			if destinations, ok := project.Spec["destinations"].([]interface{}); ok && len(destinations) > 0 {
				destNames := []string{}
				for j, destItem := range destinations {
					Debug("  Processing spec destination %d for project %s", j, project.Name)
					if destMap, ok := destItem.(map[string]interface{}); ok {
						var destName string
						if name, ok := destMap["name"].(string); ok && name != "" {
							destName = name
						} else if server, ok := destMap["server"].(string); ok && server != "" {
							destName = server
						} else if namespace, ok := destMap["namespace"].(string); ok && namespace != "" {
							destName = namespace
						} else {
							destName = "unnamed"
						}
						destNames = append(destNames, destName)
					}
				}
				if len(destNames) > 0 {
					dests = strings.Join(destNames, ", ")
				}
			}
		}

		// Format repos - check if SourceRepos is nil to avoid panic
		repos := "None"
		if project.SourceRepos != nil && len(project.SourceRepos) > 0 {
			repos = fmt.Sprintf("%d repos", len(project.SourceRepos))
		} else if project.Spec != nil {
			// Try to get source repos from spec
			if sourceRepos, ok := project.Spec["sourceRepos"].([]interface{}); ok {
				repos = fmt.Sprintf("%d repos", len(sourceRepos))
			}
		}

		// Count roles - check if Roles is nil to avoid panic
		roleCount := "0"
		if project.Roles != nil && len(project.Roles) > 0 {
			roleCount = fmt.Sprintf("%d", len(project.Roles))
		} else if project.Spec != nil {
			// Try to get roles from spec
			if roles, ok := project.Spec["roles"].([]interface{}); ok {
				roleCount = fmt.Sprintf("%d", len(roles))
			}
		}

		// Add row to result
		Debug("  Adding project %s to results with %s destinations, %s, %s roles",
			project.Name, dests, repos, roleCount)

		result = append(result, []string{
			project.Name,
			dests,
			repos,
			roleCount,
		})
	}

	Debug("Returning %d project rows from ProjectView", len(result))
	return result, nil
}

// getSelectedProject returns the currently selected project
func (v *ProjectView) getSelectedProject() *Project {
	// Ensure the cores reference is set
	if v.cores == nil {
		Debug("ProjectView cores reference is nil")
		return nil
	}

	// Get the selected row
	row := v.cores.GetSelectedRow()
	Debug("ProjectView selected row: %d, projects length: %d", row, len(v.projects))

	// Check if row is valid
	if row < 0 || row >= len(v.projects) {
		return nil
	}

	// Return the selected project
	return &v.projects[row]
}

// showProjectDetailsModal displays details of the selected project
func (v *ProjectView) showProjectDetailsModal() {
	// Get selected project
	project := v.getSelectedProject()
	if project == nil {
		v.cores.Log("[red]Please select a project to view")
		return
	}

	// Get fresh project data
	var err error
	project, err = v.apiClient.GetProject(project.Name)
	if err != nil {
		v.cores.Log(fmt.Sprintf("[red]Failed to get project details: %v", err))
		return
	}

	// Format destination clusters
	destClusters := "None"
	if len(project.Destinations) > 0 {
		destList := []string{}
		for _, dest := range project.Destinations {
			destStr := "namespace: " + dest.Namespace
			if dest.Name != "" {
				destStr = "cluster: " + dest.Name + ", " + destStr
			} else if dest.Server != "" {
				destStr = "server: " + dest.Server + ", " + destStr
			}
			destList = append(destList, "- "+destStr)
		}
		destClusters = strings.Join(destList, "\n")
	}

	// Format source repositories
	sourceRepos := "None"
	if len(project.SourceRepos) > 0 {
		repoList := []string{}
		for _, repo := range project.SourceRepos {
			repoList = append(repoList, "- "+repo)
		}
		sourceRepos = strings.Join(repoList, "\n")
	}

	// Format roles
	rolesText := "None"
	if len(project.Roles) > 0 {
		roleList := []string{}
		for _, role := range project.Roles {
			roleStr := fmt.Sprintf("- %s", role.Name)
			if role.Description != "" {
				roleStr += fmt.Sprintf(" (%s)", role.Description)
			}
			roleList = append(roleList, roleStr)
		}
		rolesText = strings.Join(roleList, "\n")
	}

	// Format cluster resource whitelist
	resourcesText := "None"
	if len(project.ClusterResourceWhitelist) > 0 {
		resourceList := []string{}
		for _, resource := range project.ClusterResourceWhitelist {
			resourceList = append(resourceList, fmt.Sprintf("- %s/%s", resource.Group, resource.Kind))
		}
		resourcesText = strings.Join(resourceList, "\n")
	}

	// Create content for modal
	content := fmt.Sprintf(`[yellow]Project: %s[white]

[green]Description:[white] %s

[green]Destinations:[white]
%s

[green]Source Repositories:[white]
%s

[green]Cluster Resource Whitelist:[white]
%s

[green]Roles:[white]
%s
`,
		project.Name,
		project.Description,
		destClusters,
		sourceRepos,
		resourcesText,
		rolesText,
	)

	// Show modal
	ui.ShowInfoModal(
		v.pages,
		v.app,
		fmt.Sprintf("Project Details: %s", project.Name),
		content,
		func() {
			v.app.SetFocus(v.cores.GetTable())
		},
	)
}

// showProjectRolesModal displays the roles for the selected project
func (v *ProjectView) showProjectRolesModal() {
	// Get selected project
	project := v.getSelectedProject()
	if project == nil {
		v.cores.Log("[red]Please select a project to view roles")
		return
	}

	// Get fresh project data
	var err error
	project, err = v.apiClient.GetProject(project.Name)
	if err != nil {
		v.cores.Log(fmt.Sprintf("[red]Failed to get project details: %v", err))
		return
	}

	// If no roles, show message and return
	if len(project.Roles) == 0 {
		v.cores.Log("[yellow]This project has no roles defined")
		return
	}

	// Format roles data for table
	roleData := [][]string{}
	for _, role := range project.Roles {
		// Format policies
		policies := strings.Join(role.Policies, ", ")
		if policies == "" {
			policies = "None"
		}

		// Format groups
		groups := strings.Join(role.Groups, ", ")
		if groups == "" {
			groups = "None"
		}

		roleData = append(roleData, []string{
			role.Name,
			policies,
			groups,
		})
	}

	// Create table
	rolesTable := tview.NewTable().
		SetBorders(true)

	// Add headers
	headers := []string{"Role Name", "Policies", "Groups"}
	for i, header := range headers {
		rolesTable.SetCell(0, i,
			tview.NewTableCell(header).
				SetTextColor(tcell.ColorYellow).
				SetAlign(tview.AlignCenter).
				SetSelectable(false))
	}

	// Add data
	for i, row := range roleData {
		for j, cell := range row {
			rolesTable.SetCell(i+1, j,
				tview.NewTableCell(cell).
					SetTextColor(tcell.ColorWhite))
		}
	}

	// Create a frame for the table
	frame := tview.NewFrame(rolesTable).
		SetBorders(0, 0, 0, 0, 0, 0).
		AddText(fmt.Sprintf("Project: %s - Roles", project.Name), true, tview.AlignCenter, tcell.ColorGreen)

	// Create the modal
	width := 70
	height := 20

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
	v.pages.AddPage("project-roles-modal", modal, true, true)

	// Set key handling
	rolesTable.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			v.pages.RemovePage("project-roles-modal")
			v.app.SetFocus(v.cores.GetTable())
			return nil
		}
		return event
	})

	// Set focus
	v.app.SetFocus(rolesTable)
}

// showCreateProjectModal displays a modal to create a new project
func (v *ProjectView) showCreateProjectModal() {
	// Create form
	form := tview.NewForm()
	form.AddInputField("Project Name:", "", 20, nil, nil)
	form.AddInputField("Description:", "", 40, nil, nil)
	form.AddInputField("Source Repositories (comma-separated):", "", 40, nil, nil)
	form.AddInputField("Destination Clusters (comma-separated):", "", 40, nil, nil)

	// Add buttons
	form.AddButton("Create", func() {
		// Get form values
		name := form.GetFormItem(0).(*tview.InputField).GetText()
		description := form.GetFormItem(1).(*tview.InputField).GetText()
		reposInput := form.GetFormItem(2).(*tview.InputField).GetText()
		clustersInput := form.GetFormItem(3).(*tview.InputField).GetText()

		// Validate inputs
		if name == "" {
			v.cores.Log("[red]Project name is required")
			return
		}

		// Parse repositories
		repos := []string{}
		if reposInput != "" {
			repos = strings.Split(reposInput, ",")
			for i, repo := range repos {
				repos[i] = strings.TrimSpace(repo)
			}
		}

		// Parse destination clusters
		destinations := []Destination{}
		if clustersInput != "" {
			clusters := strings.Split(clustersInput, ",")
			for _, cluster := range clusters {
				cluster = strings.TrimSpace(cluster)
				if cluster != "" {
					destinations = append(destinations, Destination{
						Name:      cluster,
						Namespace: "*", // Allow all namespaces by default
					})
				}
			}
		}

		// Create project object
		project := &Project{
			Name:         name,
			Description:  description,
			SourceRepos:  repos,
			Destinations: destinations,
		}

		// Show progress
		pm := ui.ShowProgressModal(
			v.pages, v.app, "Creating project", 100, true,
			nil, true,
		)

		// Create project
		safeGo(func() {
			err := v.apiClient.CreateProject(project)
			if err != nil {
				v.app.QueueUpdateDraw(func() {
					pm.Close()
					v.cores.Log(fmt.Sprintf("[red]Failed to create project: %v", err))
				})
				return
			}

			v.app.QueueUpdateDraw(func() {
				pm.Close()
				v.pages.RemovePage("create-project-modal")
				v.cores.Log(fmt.Sprintf("[green]Created project: %s", name))
				v.cores.RefreshData()
			})
		})
	})

	form.AddButton("Cancel", func() {
		v.pages.RemovePage("create-project-modal")
	})

	// Style the form
	form.SetBorder(true)
	form.SetTitle(" Create Project ")
	form.SetTitleAlign(tview.AlignCenter)
	form.SetBorderColor(tcell.ColorBlue)
	form.SetTitleColor(tcell.ColorYellow)
	form.SetBackgroundColor(tcell.ColorDefault)

	// Create centered modal
	width := 60
	height := 14

	centerFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)

	// Add to pages
	v.pages.AddPage("create-project-modal", centerFlex, true, true)

	// Set focus
	v.app.SetFocus(form.GetFormItem(0))

	// Add ESC handler
	ui.RemovePage(v.pages, v.app, "create-project-modal", nil)
}

// showDeleteProjectModal displays a modal to delete a project
func (v *ProjectView) showDeleteProjectModal() {
	// Get selected project
	project := v.getSelectedProject()
	if project == nil {
		v.cores.Log("[red]Please select a project to delete")
		return
	}

	// Show confirmation dialog
	ui.ShowStandardConfirmationModal(
		v.pages,
		v.app,
		"Delete Project",
		fmt.Sprintf("Are you sure you want to delete the project '%s'?\nThis will also delete all applications in this project.", project.Name),
		func(confirmed bool) {
			if !confirmed {
				return
			}

			// Show progress
			pm := ui.ShowProgressModal(
				v.pages, v.app, "Deleting project", 100, true,
				nil, true,
			)

			// Delete project
			projectName := project.Name
			safeGo(func() {
				err := v.apiClient.DeleteProject(projectName)
				if err != nil {
					v.app.QueueUpdateDraw(func() {
						pm.Close()
						v.cores.Log(fmt.Sprintf("[red]Failed to delete project: %v", err))
					})
					return
				}

				v.app.QueueUpdateDraw(func() {
					pm.Close()
					v.cores.Log(fmt.Sprintf("[green]Deleted project: %s", projectName))
					v.cores.RefreshData()
				})
			})
		},
	)
}

// SetupTableHandlers sets up the handlers for table events
func (v *ProjectView) SetupTableHandlers() {
	if v.cores == nil {
		Debug("Cannot set up table handlers: cores is nil")
		return
	}

	// Set up selected row handler
	v.cores.SetRowSelectedCallback(v.onProjectSelected)

	Debug("Set up table handlers for project view")
}

// onProjectSelected is called when a project is selected in the table
func (v *ProjectView) onProjectSelected(row int) {
	// Make sure the row is valid
	if v.cores == nil {
		Debug("Cores reference is nil in onProjectSelected")
		return
	}

	Debug("onProjectSelected called with row %d", row)
	Debug("Projects length: %d", len(v.projects))

	if row < 0 || row >= len(v.projects) {
		Debug("Invalid row %d (not in range 0-%d)", row, len(v.projects)-1)
		return
	}

	// Get the selected project
	project := v.projects[row]
	Debug("Selected project: %s at row %d", project.Name, row)

	// Update status bar or other UI elements with selected project info
	if project.Name != "" {
		// Format destination count
		destCount := len(project.Destinations)
		destText := fmt.Sprintf("%d destination%s", destCount,
			map[bool]string{true: "s", false: ""}[destCount != 1])

		// Format repository count
		repoCount := len(project.SourceRepos)
		repoText := fmt.Sprintf("%d repo%s", repoCount,
			map[bool]string{true: "s", false: ""}[repoCount != 1])

		// Format role count
		roleCount := len(project.Roles)
		roleText := fmt.Sprintf("%d role%s", roleCount,
			map[bool]string{true: "s", false: ""}[roleCount != 1])

		v.cores.Log(fmt.Sprintf("[blue]Selected project: %s (%s, %s, %s)",
			project.Name, destText, repoText, roleText))
	}
}
