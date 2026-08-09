package argocd

import (
	"fmt"
	"strings"

	"omo/pkg/pluginrpc"
)

func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Applications", Action: "goto_applications"},
		{Key: "1", Label: "Projects", Action: "goto_projects"},
		{Key: "2", Label: "Accounts", Action: "goto_accounts"},
		{Key: "3", Label: "RBAC", Action: "goto_rbac"},
	}
}

func moreViewBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "P", Label: "Policies", Action: "goto_rbac_policies"},
		{Key: "G", Label: "Groups", Action: "goto_rbac_groups"},
	}
}

func applicationsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "S", Label: "Sync", Action: "sync"},
		{Key: "F", Label: "Refresh App", Action: "refresh_app"},
		{Key: "D", Label: "Delete", Action: "delete"},
		{Key: "V", Label: "Details", Action: "view_details"},
	}
}

func projectsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "C", Label: "Create", Action: "create_project"},
		{Key: "D", Label: "Delete", Action: "delete"},
		{Key: "V", Label: "Details", Action: "view_details"},
	}
}

func accountsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "C", Label: "Create", Action: "create_account"},
		{Key: "T", Label: "Create Token", Action: "create_token"},
		{Key: "D", Label: "Delete", Action: "delete"},
		{Key: "V", Label: "Details", Action: "view_details"},
	}
}

func rbacActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "V", Label: "Details", Action: "view_details"},
	}
}

func helpSections() []pluginrpc.HelpSection {
	return pluginrpc.HelpNav(viewNavBindings(), moreViewBindings(),
		pluginrpc.HelpSection{Title: "Applications", Bindings: applicationsActions()},
		pluginrpc.HelpSection{Title: "Projects", Bindings: projectsActions()},
		pluginrpc.HelpSection{Title: "Accounts", Bindings: accountsActions()},
		pluginrpc.HelpSection{Title: "RBAC", Bindings: rbacActions()},
	)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	More:  moreViewBindings,
	Help:  helpSections,
}

func (s *Service) baseInfo(extra string) string {
	status := "Not Connected"
	if s.client != nil && s.client.IsConnected {
		status = "Connected"
	}
	msg := fmt.Sprintf("[green]ArgoCD Manager[white]\nServer: %s\nUser: %s\nStatus: %s\nView: %s",
		s.url, s.username, status, s.currentView)
	return pluginrpc.FormatInfo(msg, extra)
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewApplications
	}
	s.currentView = viewID

	needsAPI := viewID == viewApplications || viewID == viewProjects || viewID == viewAccounts
	if needsAPI {
		if err := s.ensureConnectedLocked(); err != nil {
			return ui.NotConnectedErr(viewID, "ArgoCD Manager", err)
		}
	}

	switch viewID {
	case viewProjects:
		return s.viewProjectsLocked()
	case viewAccounts:
		return s.viewAccountsLocked()
	case viewRBAC:
		return s.viewRBACLocked(viewRBAC)
	case viewRBACPolicies:
		return s.viewRBACLocked(viewRBACPolicies)
	case viewRBACGroups:
		return s.viewRBACLocked(viewRBACGroups)
	default:
		return s.viewApplicationsLocked()
	}
}

func (s *Service) viewApplicationsLocked() (pluginrpc.ViewData, error) {
	apps, err := s.client.GetApplications()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(apps))
	for _, a := range apps {
		name := a.Name
		if name == "" && a.Metadata != nil {
			if n, ok := a.Metadata["name"].(string); ok {
				name = n
			}
		}
		rows = append(rows, []string{
			name,
			a.Project,
			a.Health.Status,
			a.Sync.Status,
		})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"No applications found", "", "", ""})
	return ui.OK(viewApplications, "Applications", s.baseInfo(fmt.Sprintf("Applications: %d", len(apps))), []string{"Name", "Project", "Health", "Sync Status"}, rows, "Name", applicationsActions()...), nil
}

func (s *Service) viewProjectsLocked() (pluginrpc.ViewData, error) {
	projects, err := s.client.GetProjects()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(projects))
	for _, p := range projects {
		rows = append(rows, []string{
			p.Name,
			fmt.Sprintf("%d", len(p.Destinations)),
			fmt.Sprintf("%d", len(p.SourceRepos)),
			fmt.Sprintf("%d", len(p.Roles)),
		})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"No projects found", "", "", ""})
	return ui.OK(viewProjects, "Projects", s.baseInfo(fmt.Sprintf("Projects: %d", len(projects))), []string{"Name", "Destinations", "Repositories", "Roles"}, rows, "Name", projectsActions()...), nil
}

func (s *Service) viewAccountsLocked() (pluginrpc.ViewData, error) {
	accounts, err := s.client.GetAccounts()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(accounts))
	for _, a := range accounts {
		caps := strings.Join(a.Capabilities, ", ")
		if caps == "" {
			caps = "None"
		}
		enabled := "Yes"
		if !a.Enabled {
			enabled = "No"
		}
		rows = append(rows, []string{
			a.Name,
			caps,
			enabled,
			fmt.Sprintf("%d", len(a.Tokens)),
		})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"No accounts found", "", "", ""})
	return ui.OK(viewAccounts, "Accounts", s.baseInfo(fmt.Sprintf("Accounts: %d", len(accounts))), []string{"Name", "Capabilities", "Enabled", "Tokens"}, rows, "Name", accountsActions()...), nil
}

func (s *Service) viewRBACLocked(viewID string) (pluginrpc.ViewData, error) {
	if err := s.ensureConnectedLocked(); err != nil {
		return ui.StatusError(viewID, "RBAC", s.baseInfo(err.Error()), "error", err.Error()), nil
	}
	if s.k8sClient == nil {
		return ui.StatusError(viewID, "RBAC", s.baseInfo("No kubeconfig configured"), "unavailable",
			"Add kubeconfig or kubeconfig_path to KeePass entry"), nil
	}

	argoCM, err := s.k8sClient.GetConfigMap("argocd-cm")
	if err != nil {
		return pluginrpc.ViewData{}, fmt.Errorf("read argocd-cm: %w", err)
	}
	rbacCM, err := s.k8sClient.GetConfigMap("argocd-rbac-cm")
	if err != nil {
		return pluginrpc.ViewData{}, fmt.Errorf("read argocd-rbac-cm: %w", err)
	}
	s.rbacData.Accounts = ParseArgoCM(argoCM)
	s.rbacData.Policies, s.rbacData.Groups, s.rbacData.DefaultPolicy = ParseRBACCM(rbacCM)

	switch viewID {
	case viewRBACPolicies:
		rows := make([][]string, 0, len(s.rbacData.Policies))
		for _, p := range s.rbacData.Policies {
			rows = append(rows, []string{p.Subject, p.Resource, p.Action, p.Object, p.Effect})
		}
		rows = pluginrpc.EnsureRows(rows, []string{"No policy rules found", "", "", "", ""})
		return ui.OK(viewRBACPolicies, "RBAC Policies", s.baseInfo(fmt.Sprintf("Policies: %d", len(s.rbacData.Policies))), []string{"Subject", "Resource", "Action", "Object", "Effect"}, rows, "Subject", rbacActions()...), nil

	case viewRBACGroups:
		rows := make([][]string, 0, len(s.rbacData.Groups))
		for _, g := range s.rbacData.Groups {
			rows = append(rows, []string{g.User, g.Role})
		}
		rows = pluginrpc.EnsureRows(rows, []string{"No group bindings found", ""})
		return ui.OK(viewRBACGroups, "RBAC Groups", s.baseInfo(fmt.Sprintf("Groups: %d", len(s.rbacData.Groups))), []string{"User", "Role"}, rows, "User", rbacActions()...), nil

	default:
		rows := make([][]string, 0, len(s.rbacData.Accounts))
		for _, a := range s.rbacData.Accounts {
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
		rows = pluginrpc.EnsureRows(rows, []string{"No accounts found in argocd-cm", "", ""})
		return ui.OK(viewRBAC, "RBAC Accounts", s.baseInfo(fmt.Sprintf("RBAC accounts: %d", len(s.rbacData.Accounts))), []string{"Name", "Capabilities", "Enabled"}, rows, "Name", rbacActions()...), nil
	}
}
