package main

import (
	"fmt"
	"os/exec"
	"strings"

	"omo/pkg/ui"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RoleView handles the user interface for managing custom roles
type RoleView struct {
	app       *tview.Application
	pages     *tview.Pages
	cores     *ui.CoreView
	k8sClient *K8sClient
}

// NewRoleView creates a new role management view
func NewRoleView(app *tview.Application, pages *tview.Pages, cores *ui.CoreView, k8sClient *K8sClient) *RoleView {
	return &RoleView{
		app:       app,
		pages:     pages,
		cores:     cores,
		k8sClient: k8sClient,
	}
}

func parseCSVFields(text string) []string {
	parts := strings.Split(text, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

func formatRulesDisplayText(rules []map[string]interface{}) string {
	if len(rules) == 0 {
		return "[yellow]No rules defined yet. Click 'Add Rule' to define rules.[white]"
	}

	text := "[green]Defined Rules:[white]\n\n"
	for i, rule := range rules {
		text += fmt.Sprintf("[aqua]Rule %d:[white]\n", i+1)

		if apiGroups, ok := rule["apiGroups"].([]string); ok {
			text += "  [yellow]API Groups:[white] "
			if len(apiGroups) == 0 {
				text += "core (\"\")"
			} else {
				text += strings.Join(apiGroups, ", ")
			}
			text += "\n"
		}

		if resources, ok := rule["resources"].([]string); ok {
			text += "  [yellow]Resources:[white] " + strings.Join(resources, ", ") + "\n"
		}

		if verbs, ok := rule["verbs"].([]string); ok {
			text += "  [yellow]Verbs:[white] " + strings.Join(verbs, ", ") + "\n"
		}

		text += "\n"
	}
	return text
}

func styleFormModal(form *tview.Form, title string) {
	form.SetBorder(true)
	form.SetTitle(title)
	form.SetTitleAlign(tview.AlignCenter)
	form.SetBorderColor(tcell.ColorBlue)
	form.SetTitleColor(tcell.ColorYellow)
	form.SetBackgroundColor(tcell.ColorDefault)
	form.SetButtonsAlign(tview.AlignCenter)
	form.SetButtonBackgroundColor(tcell.ColorDefault)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetFieldBackgroundColor(tcell.ColorDefault)
	form.SetFieldTextColor(tcell.ColorWhite)
}

// showCreateRoleModal shows a modal for creating a new custom role
func (rv *RoleView) showCreateRoleModal() {
	form := tview.NewForm()

	nameInput := tview.NewInputField().
		SetLabel("Role Name: ").
		SetFieldWidth(30)

	namespaceDropDown := tview.NewDropDown().
		SetLabel("Namespace: ")

	rules := make([]map[string]interface{}, 0)

	form.AddFormItem(nameInput)
	form.AddFormItem(namespaceDropDown)

	form.AddTextView("Rules:", "", 0, 10, true, false)
	form.GetFormItemByLabel("Rules:").(*tview.TextView).SetText("")
	form.GetFormItemByLabel("Rules:").(*tview.TextView).SetDynamicColors(true)
	rulesView := form.GetFormItemByLabel("Rules:").(*tview.TextView)
	rulesView.SetText("[yellow]No rules defined yet. Click 'Add Rule' to define rules.[white]")

	updateRulesDisplay := func() {
		rulesView.SetText(formatRulesDisplayText(rules))
	}

	showAddRuleModal := func() {
		ruleForm := tview.NewForm()

		apiGroupsInput := tview.NewInputField().SetLabel("API Groups: ").SetFieldWidth(30)
		resourcesInput := tview.NewInputField().SetLabel("Resources: ").SetFieldWidth(30)
		verbsInput := tview.NewInputField().SetLabel("Verbs: ").SetFieldWidth(30)

		ruleForm.AddFormItem(apiGroupsInput)
		ruleForm.AddFormItem(resourcesInput)
		ruleForm.AddFormItem(verbsInput)

		ruleForm.AddButton("OK", func() {
			resourcesText := resourcesInput.GetText()
			verbsText := verbsInput.GetText()
			if resourcesText == "" || verbsText == "" {
				rv.cores.Log("[red]Resources and verbs are required")
				return
			}

			apiGroupsText := apiGroupsInput.GetText()
			var apiGroups []string
			if apiGroupsText != "" {
				apiGroups = parseCSVFields(apiGroupsText)
			} else {
				apiGroups = []string{""}
			}

			rule := map[string]interface{}{
				"apiGroups": apiGroups,
				"resources": parseCSVFields(resourcesText),
				"verbs":     parseCSVFields(verbsText),
			}

			rules = append(rules, rule)
			updateRulesDisplay()
			rv.pages.RemovePage("add-rule-modal")
			rv.cores.Log(fmt.Sprintf("[green]Added rule for resources %s with verbs %s", resourcesText, verbsText))
		})
		ruleForm.AddButton("Cancel", func() {
			rv.pages.RemovePage("add-rule-modal")
		})

		styleFormModal(ruleForm, " Add Rule ")

		centerFlex := tview.NewFlex().
			AddItem(nil, 0, 1, false).
			AddItem(tview.NewFlex().
				SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(ruleForm, 10, 1, true).
				AddItem(nil, 0, 1, false), 60, 1, true).
			AddItem(nil, 0, 1, false)

		rv.pages.AddPage("add-rule-modal", centerFlex, true, true)
		rv.app.SetFocus(apiGroupsInput)
		ui.RemovePage(rv.pages, rv.app, "add-rule-modal", nil)
	}

	form.AddButton("Add Rule", showAddRuleModal)
	form.AddButton("Create Role", func() {
		name := nameInput.GetText()
		_, namespace := namespaceDropDown.GetCurrentOption()

		if name == "" {
			rv.cores.Log("[red]Role name is required")
			return
		}
		if namespace == "" {
			rv.cores.Log("[red]Namespace is required")
			return
		}
		if len(rules) == 0 {
			rv.cores.Log("[red]At least one rule is required")
			return
		}

		rv.cores.Log(fmt.Sprintf("[blue]Creating role with %d rules", len(rules)))
		rv.pages.RemovePage("create-role-modal")

		pm := ui.ShowProgressModal(rv.pages, rv.app, "Creating Role", 100, true, nil, true)

		safeGo(func() {
			err := rv.k8sClient.CreateCustomRole(name, namespace, rules)
			if err != nil {
				rv.app.QueueUpdateDraw(func() {
					pm.Close()
					ui.ShowStandardErrorModal(rv.pages, rv.app, "Role Creation Error",
						fmt.Sprintf("Failed to create role: %v", err), nil)
				})
				return
			}
			rv.app.QueueUpdateDraw(func() {
				pm.Close()
				rv.cores.Log(fmt.Sprintf("[green]Role %s created successfully in %s", name, namespace))
				rv.cores.RefreshData()
			})
		})
	})
	form.AddButton("Cancel", func() {
		rv.pages.RemovePage("create-role-modal")
	})

	styleFormModal(form, " Create Custom Role ")

	rv.cores.Log("[blue]Getting namespaces for role creation...")
	safeGo(func() {
		namespaces, err := rv.k8sClient.GetNamespaces()
		if err != nil {
			rv.app.QueueUpdateDraw(func() {
				rv.cores.Log(fmt.Sprintf("[red]Error getting namespaces: %v", err))
				namespaceDropDown.SetOptions([]string{"default", "cluster-wide"}, nil)
			})
			return
		}
		rv.app.QueueUpdateDraw(func() {
			namespaceDropDown.SetOptions(namespaces, nil)
			namespaceDropDown.SetCurrentOption(0)
		})
	})

	centerFlex := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().
			SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(form, 25, 1, true).
			AddItem(nil, 0, 1, false), 70, 1, true).
		AddItem(nil, 0, 1, false)

	rv.pages.AddPage("create-role-modal", centerFlex, true, true)
	rv.app.SetFocus(nameInput)
	ui.RemovePage(rv.pages, rv.app, "create-role-modal", nil)
}

// showDeleteRoleModal shows a modal for deleting a custom role
func (rv *RoleView) showDeleteRoleModal() {
	// Get currently selected row
	selectedRow := rv.cores.GetTable().GetSelectedRow() - 1

	// Debug log
	rv.cores.Log(fmt.Sprintf("[yellow]Debug: showDeleteRoleModal called, selectedRow = %d", selectedRow))

	// Extract role information
	tableData := rv.cores.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		rv.cores.Log("[red]No role selected")
		return
	}

	roleName := tableData[selectedRow][0]
	roleNamespace := tableData[selectedRow][1]

	// Show confirmation dialog
	ui.ShowStandardConfirmationModal(
		rv.pages, rv.app, "Delete Role",
		fmt.Sprintf("Are you sure you want to delete the role %s in %s?", roleName, roleNamespace),
		func(confirmed bool) {
			if !confirmed {
				return
			}

			// Show progress
			pm := ui.ShowProgressModal(
				rv.pages, rv.app, "Deleting Role", 100, true,
				nil, true,
			)

			// Delete role
			safeGo(func() {
				err := rv.k8sClient.DeleteCustomRole(roleName, roleNamespace)
				if err != nil {
					rv.app.QueueUpdateDraw(func() {
						pm.Close()
						ui.ShowStandardErrorModal(
							rv.pages, rv.app, "Role Deletion Error",
							fmt.Sprintf("Failed to delete role: %v", err),
							nil,
						)
					})
					return
				}

				// Success
				rv.app.QueueUpdateDraw(func() {
					pm.Close()
					rv.cores.Log(fmt.Sprintf("[green]Role %s deleted successfully from %s", roleName, roleNamespace))

					// Refresh roles list
					rv.cores.RefreshData()
				})
			})
		},
	)
}

// fetchRoles retrieves custom roles for the roles view
func (rv *RoleView) fetchRoles() ([][]string, error) {
	// Get roles in all namespaces
	var allRoles [][]string

	// Get namespaces
	namespaces, err := rv.k8sClient.GetNamespaces()
	if err != nil {
		return [][]string{{"Error fetching namespaces", err.Error(), ""}}, nil
	}

	// For each namespace, get roles
	for _, namespace := range namespaces {
		if namespace == "cluster-wide" {
			// Get cluster roles
			cmd := exec.Command("kubectl", "get", "clusterroles", "-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\t\"}{\"cluster-wide\"}{\"\\t\"}{.rules[0].resources}{\"\\n\"}{end}")
			output, err := cmd.CombinedOutput()
			if err != nil {
				rv.cores.Log(fmt.Sprintf("[red]Error fetching cluster roles: %v", err))
				continue
			}

			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}

				parts := strings.Split(line, "\t")
				if len(parts) < 3 {
					continue
				}

				name := parts[0]

				// Skip system roles
				if strings.HasPrefix(name, "system:") {
					continue
				}

				// Add to list
				allRoles = append(allRoles, []string{
					name,
					"cluster-wide",
					parts[2],
				})
			}
		} else {
			// Get namespace roles
			cmd := exec.Command("kubectl", "get", "roles", "-n", namespace, "-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\t\"}{.metadata.namespace}{\"\\t\"}{.rules[0].resources}{\"\\n\"}{end}")
			output, err := cmd.CombinedOutput()
			if err != nil {
				rv.cores.Log(fmt.Sprintf("[red]Error fetching roles in namespace %s: %v", namespace, err))
				continue
			}

			lines := strings.Split(strings.TrimSpace(string(output)), "\n")
			for _, line := range lines {
				if line == "" {
					continue
				}

				parts := strings.Split(line, "\t")
				if len(parts) < 3 {
					continue
				}

				// Add to list
				allRoles = append(allRoles, []string{
					parts[0],
					parts[1],
					parts[2],
				})
			}
		}
	}

	if len(allRoles) == 0 {
		return [][]string{{"No custom roles found", "Use 'C' to create", ""}}, nil
	}

	return allRoles, nil
}

// showRoleDetailsModal shows detailed information about a role
func (rv *RoleView) showRoleDetailsModal() {
	// Get currently selected row
	selectedRow := rv.cores.GetTable().GetSelectedRow() - 1

	// Debug log
	rv.cores.Log(fmt.Sprintf("[yellow]Debug: showRoleDetailsModal called, selectedRow = %d", selectedRow))

	// Extract role information
	tableData := rv.cores.GetTableData()
	if selectedRow < 0 || selectedRow >= len(tableData) {
		rv.cores.Log("[red]No role selected")
		return
	}

	roleName := tableData[selectedRow][0]
	roleNamespace := tableData[selectedRow][1]

	// Run kubectl describe to get detailed info
	var cmd *exec.Cmd
	if roleNamespace == "cluster-wide" {
		cmd = exec.Command("kubectl", "describe", "clusterrole", roleName)
	} else {
		cmd = exec.Command("kubectl", "describe", "role", roleName, "-n", roleNamespace)
	}

	// Show loading modal
	pm := ui.ShowProgressModal(
		rv.pages, rv.app, "Loading Role Details", 100, true,
		nil, true,
	)

	// Get role details
	safeGo(func() {
		output, err := cmd.CombinedOutput()

		rv.app.QueueUpdateDraw(func() {
			pm.Close()

			if err != nil {
				ui.ShowStandardErrorModal(
					rv.pages, rv.app, "Role Details Error",
					fmt.Sprintf("Failed to get role details: %v", err),
					nil,
				)
				return
			}

			// Show role details in modal
			ui.ShowInfoModal(
				rv.pages,
				rv.app,
				fmt.Sprintf(" Role Details: %s ", roleName),
				string(output),
				func() {
					// Return focus to table
					rv.app.SetFocus(rv.cores.GetTable())
				},
			)
		})
	})
}
