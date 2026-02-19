package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"omo/pkg/ui"
)

const (
	rbacSubAccounts = "accounts"
	rbacSubPolicies = "policies"
	rbacSubGroups   = "groups"
)

// RBACView manages the RBAC management TUI.
type RBACView struct {
	app       *tview.Application
	pages     *tview.Pages
	cores     *ui.CoreView
	k8sClient *K8sClient
	apiClient *ArgoAPIClient
	adminPass string // admin password for setting other accounts' passwords

	subView  string // current sub-view
	rbacData RBACConfig
}

// NewRBACView creates a new RBACView.
func NewRBACView(app *tview.Application, pages *tview.Pages, cores *ui.CoreView, k8sClient *K8sClient, apiClient *ArgoAPIClient, adminPass string) *RBACView {
	return &RBACView{
		app:       app,
		pages:     pages,
		cores:     cores,
		k8sClient: k8sClient,
		apiClient: apiClient,
		adminPass: adminPass,
		subView:   rbacSubAccounts,
	}
}

// fetchRBACData reads both ConfigMaps and populates the cached data.
func (v *RBACView) fetchRBACData() ([][]string, error) {
	if v.k8sClient == nil {
		return [][]string{{"No kubeconfig configured for this instance", "", "", ""}}, nil
	}

	argoCM, err := v.k8sClient.GetConfigMap("argocd-cm")
	if err != nil {
		return nil, fmt.Errorf("failed to read argocd-cm: %w", err)
	}
	rbacCM, err := v.k8sClient.GetConfigMap("argocd-rbac-cm")
	if err != nil {
		return nil, fmt.Errorf("failed to read argocd-rbac-cm: %w", err)
	}

	v.rbacData.Accounts = ParseArgoCM(argoCM)
	v.rbacData.Policies, v.rbacData.Groups, v.rbacData.DefaultPolicy = ParseRBACCM(rbacCM)

	return v.formatCurrentSubView(), nil
}

func (v *RBACView) formatCurrentSubView() [][]string {
	switch v.subView {
	case rbacSubPolicies:
		return v.formatPolicies()
	case rbacSubGroups:
		return v.formatGroups()
	default:
		return v.formatAccounts()
	}
}

func (v *RBACView) formatAccounts() [][]string {
	if len(v.rbacData.Accounts) == 0 {
		return [][]string{{"No accounts found in argocd-cm", "", ""}}
	}
	var rows [][]string
	for _, a := range v.rbacData.Accounts {
		caps := strings.Join(a.Capabilities, ", ")
		if caps == "" {
			caps = "None"
		}
		enabled := "Yes"
		if !a.Enabled {
			enabled = "No"
		}
		rows = append(rows, []string{a.Name, caps, enabled})
	}
	return rows
}

func (v *RBACView) formatPolicies() [][]string {
	if len(v.rbacData.Policies) == 0 {
		return [][]string{{"No policy rules found", "", "", "", ""}}
	}
	var rows [][]string
	for _, p := range v.rbacData.Policies {
		rows = append(rows, []string{p.Subject, p.Resource, p.Action, p.Object, p.Effect})
	}
	return rows
}

func (v *RBACView) formatGroups() [][]string {
	if len(v.rbacData.Groups) == 0 {
		return [][]string{{"No group bindings found", ""}}
	}
	var rows [][]string
	for _, g := range v.rbacData.Groups {
		rows = append(rows, []string{g.User, g.Role})
	}
	return rows
}

// SetupTableHandlers configures row selection handling.
func (v *RBACView) SetupTableHandlers() {
	if v.cores == nil {
		return
	}
	v.cores.SetRowSelectedCallback(func(row int) {
		Debug("RBAC row selected: %d (subView=%s)", row, v.subView)
	})
}

// HandleKey processes a keypress in the RBAC view. Returns true if handled.
func (v *RBACView) HandleKey(key string) bool {
	switch key {
	case "1":
		v.switchSubView(rbacSubAccounts)
	case "2":
		v.switchSubView(rbacSubPolicies)
	case "3":
		v.switchSubView(rbacSubGroups)
	case "R":
		v.cores.RefreshData()
	case "C":
		v.createItem()
	case "D":
		v.deleteItem()
	case "E":
		v.editItem()
	case "T":
		if v.subView == rbacSubAccounts {
			v.toggleAccountEnabled()
		}
	case "W":
		if v.subView == rbacSubAccounts {
			v.setAccountPassword()
		}
	case "V":
		v.viewDetails()
	default:
		return false
	}
	return true
}

func (v *RBACView) switchSubView(sub string) {
	v.subView = sub

	switch sub {
	case rbacSubAccounts:
		v.cores.SetTableHeaders([]string{"Name", "Capabilities", "Enabled"})
		v.cores.ClearKeyBindings()
		v.addCommonKeyBindings()
		v.cores.AddKeyBinding("E", "Edit Capabilities", nil)
		v.cores.AddKeyBinding("T", "Toggle Enabled", nil)
		v.cores.AddKeyBinding("W", "Set Password", nil)
	case rbacSubPolicies:
		v.cores.SetTableHeaders([]string{"Subject", "Resource", "Action", "Object", "Effect"})
		v.cores.ClearKeyBindings()
		v.addCommonKeyBindings()
	case rbacSubGroups:
		v.cores.SetTableHeaders([]string{"User", "Role"})
		v.cores.ClearKeyBindings()
		v.addCommonKeyBindings()
	}

	v.cores.RefreshData()
	v.cores.Log(fmt.Sprintf("[blue]Switched to RBAC %s sub-view", sub))
}

func (v *RBACView) addCommonKeyBindings() {
	v.cores.AddKeyBinding("1", "Accounts", nil)
	v.cores.AddKeyBinding("2", "Policies", nil)
	v.cores.AddKeyBinding("3", "Groups", nil)
	v.cores.AddKeyBinding("R", "Refresh", nil)
	v.cores.AddKeyBinding("C", "Create", nil)
	v.cores.AddKeyBinding("D", "Delete", nil)
	v.cores.AddKeyBinding("V", "View Details", nil)
	v.cores.AddKeyBinding("^B", "Back", nil)
	v.cores.AddKeyBinding("?", "Help", nil)
}

// --- Create ---

func (v *RBACView) createItem() {
	switch v.subView {
	case rbacSubAccounts:
		v.showCreateAccountModal()
	case rbacSubPolicies:
		v.showCreatePolicyModal()
	case rbacSubGroups:
		v.showCreateGroupModal()
	}
}

func (v *RBACView) showCreateAccountModal() {
	form := tview.NewForm()
	form.AddInputField("Account Name:", "", 30, nil, nil)
	form.AddInputField("Capabilities:", "apiKey, login", 40, nil, nil)
	form.AddCheckbox("Enabled:", true, nil)

	form.AddButton("Create", func() {
		name := form.GetFormItem(0).(*tview.InputField).GetText()
		capsStr := form.GetFormItem(1).(*tview.InputField).GetText()
		enabled := form.GetFormItem(2).(*tview.Checkbox).IsChecked()

		if name == "" {
			v.cores.Log("[red]Account name is required")
			return
		}

		caps := parseCaps(capsStr)

		v.pages.RemovePage("rbac-create-modal")
		v.applyNewAccount(RBACAccount{Name: name, Capabilities: caps, Enabled: enabled})
	})
	form.AddButton("Cancel", func() {
		v.pages.RemovePage("rbac-create-modal")
		v.app.SetFocus(v.cores.GetTable())
	})

	v.showFormModal("rbac-create-modal", " Create RBAC Account ", form, 60, 12)
}

func (v *RBACView) showCreatePolicyModal() {
	form := tview.NewForm()
	form.AddInputField("Subject (e.g. role:admin):", "", 40, nil, nil)
	form.AddInputField("Resource:", "applications", 30, nil, nil)
	form.AddInputField("Action:", "*", 20, nil, nil)
	form.AddInputField("Object:", "*", 20, nil, nil)
	form.AddDropDown("Effect:", []string{"allow", "deny"}, 0, nil)

	form.AddButton("Create", func() {
		subject := form.GetFormItem(0).(*tview.InputField).GetText()
		resource := form.GetFormItem(1).(*tview.InputField).GetText()
		action := form.GetFormItem(2).(*tview.InputField).GetText()
		object := form.GetFormItem(3).(*tview.InputField).GetText()
		_, effect := form.GetFormItem(4).(*tview.DropDown).GetCurrentOption()

		if subject == "" || resource == "" {
			v.cores.Log("[red]Subject and resource are required")
			return
		}

		v.pages.RemovePage("rbac-create-modal")
		v.applyNewPolicy(PolicyRule{
			Subject:  subject,
			Resource: resource,
			Action:   action,
			Object:   object,
			Effect:   effect,
		})
	})
	form.AddButton("Cancel", func() {
		v.pages.RemovePage("rbac-create-modal")
		v.app.SetFocus(v.cores.GetTable())
	})

	v.showFormModal("rbac-create-modal", " Create Policy Rule ", form, 60, 18)
}

func (v *RBACView) showCreateGroupModal() {
	form := tview.NewForm()
	form.AddInputField("User/Account:", "", 30, nil, nil)
	form.AddInputField("Role (e.g. role:admin):", "", 30, nil, nil)

	form.AddButton("Create", func() {
		user := form.GetFormItem(0).(*tview.InputField).GetText()
		role := form.GetFormItem(1).(*tview.InputField).GetText()

		if user == "" || role == "" {
			v.cores.Log("[red]User and role are required")
			return
		}

		v.pages.RemovePage("rbac-create-modal")
		v.applyNewGroup(GroupBinding{User: user, Role: role})
	})
	form.AddButton("Cancel", func() {
		v.pages.RemovePage("rbac-create-modal")
		v.app.SetFocus(v.cores.GetTable())
	})

	v.showFormModal("rbac-create-modal", " Create Group Binding ", form, 55, 11)
}

// --- Delete ---

func (v *RBACView) deleteItem() {
	row := v.cores.GetSelectedRow()

	switch v.subView {
	case rbacSubAccounts:
		if row < 0 || row >= len(v.rbacData.Accounts) {
			v.cores.Log("[red]Select an account to delete")
			return
		}
		acct := v.rbacData.Accounts[row]
		ui.ShowStandardConfirmationModal(v.pages, v.app,
			"Delete Account",
			fmt.Sprintf("Delete account '%s'?\nThis removes it from argocd-cm.", acct.Name),
			func(ok bool) {
				if ok {
					v.applyDeleteAccount(row)
				}
				v.app.SetFocus(v.cores.GetTable())
			},
		)

	case rbacSubPolicies:
		if row < 0 || row >= len(v.rbacData.Policies) {
			v.cores.Log("[red]Select a policy to delete")
			return
		}
		p := v.rbacData.Policies[row]
		ui.ShowStandardConfirmationModal(v.pages, v.app,
			"Delete Policy",
			fmt.Sprintf("Delete policy rule?\n%s", p.String()),
			func(ok bool) {
				if ok {
					v.applyDeletePolicy(row)
				}
				v.app.SetFocus(v.cores.GetTable())
			},
		)

	case rbacSubGroups:
		if row < 0 || row >= len(v.rbacData.Groups) {
			v.cores.Log("[red]Select a group binding to delete")
			return
		}
		g := v.rbacData.Groups[row]
		ui.ShowStandardConfirmationModal(v.pages, v.app,
			"Delete Group Binding",
			fmt.Sprintf("Remove '%s' from '%s'?", g.User, g.Role),
			func(ok bool) {
				if ok {
					v.applyDeleteGroup(row)
				}
				v.app.SetFocus(v.cores.GetTable())
			},
		)
	}
}

// --- Edit ---

func (v *RBACView) editItem() {
	if v.subView != rbacSubAccounts {
		return
	}
	row := v.cores.GetSelectedRow()
	if row < 0 || row >= len(v.rbacData.Accounts) {
		v.cores.Log("[red]Select an account to edit")
		return
	}

	acct := v.rbacData.Accounts[row]
	currentCaps := strings.Join(acct.Capabilities, ", ")

	ui.ShowCompactStyledInputModal(
		v.pages, v.app,
		fmt.Sprintf("Edit Capabilities: %s", acct.Name),
		"Capabilities:", currentCaps, 40, nil,
		func(text string, cancelled bool) {
			if cancelled {
				v.app.SetFocus(v.cores.GetTable())
				return
			}
			v.rbacData.Accounts[row].Capabilities = parseCaps(text)
			v.saveArgoCM("Updated capabilities for " + acct.Name)
		},
	)
}

// --- Toggle enabled ---

func (v *RBACView) toggleAccountEnabled() {
	row := v.cores.GetSelectedRow()
	if row < 0 || row >= len(v.rbacData.Accounts) {
		v.cores.Log("[red]Select an account to toggle")
		return
	}

	acct := &v.rbacData.Accounts[row]
	acct.Enabled = !acct.Enabled
	status := "enabled"
	if !acct.Enabled {
		status = "disabled"
	}
	v.saveArgoCM(fmt.Sprintf("Account '%s' %s", acct.Name, status))
}

// --- Set Password ---

func (v *RBACView) setAccountPassword() {
	row := v.cores.GetSelectedRow()
	if row < 0 || row >= len(v.rbacData.Accounts) {
		v.cores.Log("[red]Select an account to set password for")
		return
	}

	if v.apiClient == nil || !v.apiClient.IsConnected {
		v.cores.Log("[red]Not connected to ArgoCD API — cannot set password")
		return
	}

	acct := v.rbacData.Accounts[row]

	form := tview.NewForm()
	form.AddInputField("Account:", acct.Name, 30, nil, nil)
	form.GetFormItem(0).(*tview.InputField).SetDisabled(true)
	form.AddPasswordField("New Password:", "", 30, '*', nil)
	form.AddPasswordField("Confirm Password:", "", 30, '*', nil)

	form.AddButton("Set Password", func() {
		newPass := form.GetFormItem(1).(*tview.InputField).GetText()
		confirm := form.GetFormItem(2).(*tview.InputField).GetText()

		if newPass == "" {
			v.cores.Log("[red]Password cannot be empty")
			return
		}
		if newPass != confirm {
			v.cores.Log("[red]Passwords do not match")
			return
		}

		v.pages.RemovePage("rbac-password-modal")

		adminPass := v.adminPass
		if adminPass == "" && v.apiClient != nil {
			adminPass = v.apiClient.Password
		}

		pm := ui.ShowProgressModal(v.pages, v.app, "Setting password", 100, true, nil, true)
		accountName := acct.Name

		safeGo(func() {
			err := v.apiClient.UpdatePassword(accountName, newPass, adminPass)
			v.app.QueueUpdateDraw(func() {
				pm.Close()
				if err != nil {
					v.cores.Log(fmt.Sprintf("[red]Failed to set password: %v", err))
				} else {
					v.cores.Log(fmt.Sprintf("[green]Password set for account '%s'", accountName))
				}
				v.app.SetFocus(v.cores.GetTable())
			})
		})
	})

	form.AddButton("Cancel", func() {
		v.pages.RemovePage("rbac-password-modal")
		v.app.SetFocus(v.cores.GetTable())
	})

	v.showFormModal("rbac-password-modal", " Set Password: "+acct.Name+" ", form, 55, 14)
}

// --- View Details ---

func (v *RBACView) viewDetails() {
	row := v.cores.GetSelectedRow()

	switch v.subView {
	case rbacSubAccounts:
		if row < 0 || row >= len(v.rbacData.Accounts) {
			return
		}
		a := v.rbacData.Accounts[row]
		caps := strings.Join(a.Capabilities, ", ")
		if caps == "" {
			caps = "None"
		}
		enabled := "Yes"
		if !a.Enabled {
			enabled = "No"
		}

		// Find group bindings for this account
		var groupLines []string
		for _, g := range v.rbacData.Groups {
			if g.User == a.Name {
				groupLines = append(groupLines, "  "+g.Role)
			}
		}
		groupsText := "None"
		if len(groupLines) > 0 {
			groupsText = strings.Join(groupLines, "\n")
		}

		// Find policies for this account
		var policyLines []string
		for _, p := range v.rbacData.Policies {
			if p.Subject == a.Name {
				policyLines = append(policyLines, "  "+p.String())
			}
		}
		policiesText := "None (check role-based policies)"
		if len(policyLines) > 0 {
			policiesText = strings.Join(policyLines, "\n")
		}

		content := fmt.Sprintf(`[yellow]Account: %s[white]

[green]Enabled:[white] %s
[green]Capabilities:[white] %s

[green]Group Memberships:[white]
%s

[green]Direct Policies:[white]
%s`,
			a.Name, enabled, caps, groupsText, policiesText)

		ui.ShowInfoModal(v.pages, v.app, "Account: "+a.Name, content, func() {
			v.app.SetFocus(v.cores.GetTable())
		})

	case rbacSubPolicies:
		if row < 0 || row >= len(v.rbacData.Policies) {
			return
		}
		p := v.rbacData.Policies[row]
		content := fmt.Sprintf(`[yellow]Policy Rule[white]

[green]Subject:[white]  %s
[green]Resource:[white] %s
[green]Action:[white]   %s
[green]Object:[white]   %s
[green]Effect:[white]   %s

[green]Raw:[white] %s`,
			p.Subject, p.Resource, p.Action, p.Object, p.Effect, p.String())

		ui.ShowInfoModal(v.pages, v.app, "Policy Details", content, func() {
			v.app.SetFocus(v.cores.GetTable())
		})

	case rbacSubGroups:
		if row < 0 || row >= len(v.rbacData.Groups) {
			return
		}
		g := v.rbacData.Groups[row]

		// Find all policies for this role
		var policyLines []string
		for _, p := range v.rbacData.Policies {
			if p.Subject == g.Role {
				policyLines = append(policyLines, "  "+p.String())
			}
		}
		policiesText := "None"
		if len(policyLines) > 0 {
			policiesText = strings.Join(policyLines, "\n")
		}

		content := fmt.Sprintf(`[yellow]Group Binding[white]

[green]User:[white] %s
[green]Role:[white] %s

[green]Role Policies:[white]
%s`,
			g.User, g.Role, policiesText)

		ui.ShowInfoModal(v.pages, v.app, "Group Binding", content, func() {
			v.app.SetFocus(v.cores.GetTable())
		})
	}
}

// --- Apply mutations ---

func (v *RBACView) applyNewAccount(acct RBACAccount) {
	v.rbacData.Accounts = append(v.rbacData.Accounts, acct)
	v.saveArgoCM("Created account: " + acct.Name)
}

func (v *RBACView) applyDeleteAccount(idx int) {
	name := v.rbacData.Accounts[idx].Name
	v.rbacData.Accounts = append(v.rbacData.Accounts[:idx], v.rbacData.Accounts[idx+1:]...)
	v.saveArgoCM("Deleted account: " + name)
}

func (v *RBACView) applyNewPolicy(p PolicyRule) {
	v.rbacData.Policies = append(v.rbacData.Policies, p)
	v.saveRBACCM("Created policy rule for " + p.Subject)
}

func (v *RBACView) applyDeletePolicy(idx int) {
	subj := v.rbacData.Policies[idx].Subject
	v.rbacData.Policies = append(v.rbacData.Policies[:idx], v.rbacData.Policies[idx+1:]...)
	v.saveRBACCM("Deleted policy rule for " + subj)
}

func (v *RBACView) applyNewGroup(g GroupBinding) {
	v.rbacData.Groups = append(v.rbacData.Groups, g)
	v.saveRBACCM("Added " + g.User + " to " + g.Role)
}

func (v *RBACView) applyDeleteGroup(idx int) {
	user := v.rbacData.Groups[idx].User
	v.rbacData.Groups = append(v.rbacData.Groups[:idx], v.rbacData.Groups[idx+1:]...)
	v.saveRBACCM("Removed " + user + " from group")
}

// --- ConfigMap writers ---

func (v *RBACView) saveArgoCM(msg string) {
	pm := ui.ShowProgressModal(v.pages, v.app, "Updating argocd-cm", 100, true, nil, true)

	safeGo(func() {
		cm, err := v.k8sClient.GetConfigMap("argocd-cm")
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				pm.Close()
				v.cores.Log(fmt.Sprintf("[red]Failed to read argocd-cm: %v", err))
			})
			return
		}

		ApplyArgoCM(cm, v.rbacData.Accounts)
		err = v.k8sClient.UpdateConfigMap(cm)

		v.app.QueueUpdateDraw(func() {
			pm.Close()
			if err != nil {
				v.cores.Log(fmt.Sprintf("[red]Failed to update argocd-cm: %v", err))
			} else {
				v.cores.Log(fmt.Sprintf("[green]%s", msg))
				v.cores.SetTableData(v.formatCurrentSubView())
			}
			v.app.SetFocus(v.cores.GetTable())
		})
	})
}

func (v *RBACView) saveRBACCM(msg string) {
	pm := ui.ShowProgressModal(v.pages, v.app, "Updating argocd-rbac-cm", 100, true, nil, true)

	safeGo(func() {
		cm, err := v.k8sClient.GetConfigMap("argocd-rbac-cm")
		if err != nil {
			v.app.QueueUpdateDraw(func() {
				pm.Close()
				v.cores.Log(fmt.Sprintf("[red]Failed to read argocd-rbac-cm: %v", err))
			})
			return
		}

		ApplyRBACCM(cm, v.rbacData.Policies, v.rbacData.Groups, v.rbacData.DefaultPolicy)
		err = v.k8sClient.UpdateConfigMap(cm)

		v.app.QueueUpdateDraw(func() {
			pm.Close()
			if err != nil {
				v.cores.Log(fmt.Sprintf("[red]Failed to update argocd-rbac-cm: %v", err))
			} else {
				v.cores.Log(fmt.Sprintf("[green]%s", msg))
				v.cores.SetTableData(v.formatCurrentSubView())
			}
			v.app.SetFocus(v.cores.GetTable())
		})
	})
}

// --- Helpers ---

func (v *RBACView) showFormModal(pageID, title string, form *tview.Form, width, height int) {
	form.SetBorder(true)
	form.SetTitle(title)
	form.SetTitleAlign(tview.AlignCenter)
	form.SetBorderColor(tcell.ColorAqua)
	form.SetTitleColor(tcell.ColorOrange)
	form.SetBackgroundColor(tcell.ColorDefault)
	form.SetButtonBackgroundColor(tcell.ColorDefault)
	form.SetButtonTextColor(tcell.ColorWhite)
	form.SetFieldBackgroundColor(tcell.ColorDefault)
	form.SetFieldTextColor(tcell.ColorWhite)
	form.SetBorderPadding(1, 1, 2, 2)

	for i := 0; i < form.GetButtonCount(); i++ {
		if b := form.GetButton(i); b != nil {
			b.SetBackgroundColor(tcell.ColorDefault)
			b.SetLabelColor(tcell.ColorWhite)
			b.SetBackgroundColorActivated(tcell.ColorAqua)
			b.SetLabelColorActivated(tcell.ColorBlack)
		}
	}

	innerFlex := tview.NewFlex().SetDirection(tview.FlexRow)
	innerFlex.SetBackgroundColor(tcell.ColorDefault)
	innerFlex.AddItem(nil, 0, 1, false).
		AddItem(form, height, 1, true).
		AddItem(nil, 0, 1, false)

	flex := tview.NewFlex()
	flex.SetBackgroundColor(tcell.ColorDefault)
	flex.AddItem(nil, 0, 1, false).
		AddItem(innerFlex, width, 1, true).
		AddItem(nil, 0, 1, false)

	ui.RemovePage(v.pages, v.app, pageID, func() {
		v.app.SetFocus(v.cores.GetTable())
	})

	v.pages.AddPage(pageID, flex, true, true)
	v.app.SetFocus(form)
}
