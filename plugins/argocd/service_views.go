package argocd

import (
	"fmt"
	"strings"

	"omo/pkg/pluginrpc"
)

func argocdNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "A", Label: "Applications", Action: "goto_applications"},
		{Key: "P", Label: "Projects", Action: "goto_projects"},
		{Key: "U", Label: "Accounts", Action: "goto_accounts"},
		{Key: "G", Label: "RBAC", Action: "goto_rbac"},
	}
}

func withArgoNav(extra ...pluginrpc.KeyBinding) []pluginrpc.KeyBinding {
	out := make([]pluginrpc.KeyBinding, 0, len(extra)+len(argocdNavBindings())+1)
	out = append(out, pluginrpc.KeyBinding{Key: "R", Label: "Refresh", Action: "refresh"})
	out = append(out, extra...)
	out = append(out, argocdNavBindings()...)
	return out
}

func (s *Service) baseInfo(extra string) string {
	status := "Not Connected"
	if s.client != nil && s.client.IsConnected {
		status = "Connected"
	}
	msg := fmt.Sprintf("[green]ArgoCD Manager[white]\nServer: %s\nUser: %s\nStatus: %s\nView: %s",
		s.url, s.username, status, s.currentView)
	if extra != "" {
		msg += "\n" + extra
	}
	return msg
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewApplications
	}
	s.currentView = viewID

	needsAPI := viewID == viewApplications || viewID == viewProjects || viewID == viewAccounts
	if needsAPI {
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ViewData{
				View:    viewID,
				Title:   "ArgoCD Manager",
				Info:    "[yellow]ArgoCD Manager[white]\nStatus: Not Connected\n" + err.Error(),
				Status:  "not connected",
				Headers: []string{"Status", "Detail"},
				Rows:    [][]string{{"error", err.Error()}},
				KeyBindings: []pluginrpc.KeyBinding{
					{Key: "R", Label: "Refresh", Action: "refresh"},
				},
			}, nil
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
	if len(rows) == 0 {
		rows = [][]string{{"No applications found", "", "", ""}}
	}
	return pluginrpc.ViewData{
		View:         viewApplications,
		Title:        "Applications",
		Info:         s.baseInfo(fmt.Sprintf("Applications: %d", len(apps))),
		Status:       "ok",
		Headers:      []string{"Name", "Project", "Health", "Sync Status"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings: withArgoNav(
			pluginrpc.KeyBinding{Key: "S", Label: "Sync", Action: "sync"},
			pluginrpc.KeyBinding{Key: "F", Label: "Refresh App", Action: "refresh_app"},
			pluginrpc.KeyBinding{Key: "D", Label: "Delete", Action: "delete"},
			pluginrpc.KeyBinding{Key: "V", Label: "Details", Action: "view_details"},
		),
	}, nil
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
	if len(rows) == 0 {
		rows = [][]string{{"No projects found", "", "", ""}}
	}
	return pluginrpc.ViewData{
		View:         viewProjects,
		Title:        "Projects",
		Info:         s.baseInfo(fmt.Sprintf("Projects: %d", len(projects))),
		Status:       "ok",
		Headers:      []string{"Name", "Destinations", "Repositories", "Roles"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings: withArgoNav(
			pluginrpc.KeyBinding{Key: "C", Label: "Create", Action: "create_project"},
			pluginrpc.KeyBinding{Key: "D", Label: "Delete", Action: "delete"},
			pluginrpc.KeyBinding{Key: "V", Label: "Details", Action: "view_details"},
		),
	}, nil
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
	if len(rows) == 0 {
		rows = [][]string{{"No accounts found", "", "", ""}}
	}
	return pluginrpc.ViewData{
		View:         viewAccounts,
		Title:        "Accounts",
		Info:         s.baseInfo(fmt.Sprintf("Accounts: %d", len(accounts))),
		Status:       "ok",
		Headers:      []string{"Name", "Capabilities", "Enabled", "Tokens"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings: withArgoNav(
			pluginrpc.KeyBinding{Key: "C", Label: "Create", Action: "create_account"},
			pluginrpc.KeyBinding{Key: "T", Label: "Create Token", Action: "create_token"},
			pluginrpc.KeyBinding{Key: "D", Label: "Delete", Action: "delete"},
			pluginrpc.KeyBinding{Key: "V", Label: "Details", Action: "view_details"},
		),
	}, nil
}

func (s *Service) viewRBACLocked(viewID string) (pluginrpc.ViewData, error) {
	if err := s.ensureConnectedLocked(); err != nil {
		return pluginrpc.ViewData{
			View:    viewID,
			Title:   "RBAC",
			Info:    s.baseInfo(err.Error()),
			Status:  "error",
			Headers: []string{"Status", "Detail"},
			Rows:    [][]string{{"error", err.Error()}},
			KeyBindings: withArgoNav(
				pluginrpc.KeyBinding{Key: "1", Label: "Accounts", Action: "goto_rbac"},
				pluginrpc.KeyBinding{Key: "2", Label: "Policies", Action: "goto_rbac_policies"},
				pluginrpc.KeyBinding{Key: "3", Label: "Groups", Action: "goto_rbac_groups"},
			),
		}, nil
	}
	if s.k8sClient == nil {
		return pluginrpc.ViewData{
			View:    viewID,
			Title:   "RBAC",
			Info:    s.baseInfo("No kubeconfig configured"),
			Status:  "unavailable",
			Headers: []string{"Status", "Detail"},
			Rows:    [][]string{{"unavailable", "Add kubeconfig or kubeconfig_path to KeePass entry"}},
			KeyBindings: withArgoNav(
				pluginrpc.KeyBinding{Key: "1", Label: "Accounts", Action: "goto_rbac"},
				pluginrpc.KeyBinding{Key: "2", Label: "Policies", Action: "goto_rbac_policies"},
				pluginrpc.KeyBinding{Key: "3", Label: "Groups", Action: "goto_rbac_groups"},
			),
		}, nil
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

	nav := withArgoNav(
		pluginrpc.KeyBinding{Key: "1", Label: "Accounts", Action: "goto_rbac"},
		pluginrpc.KeyBinding{Key: "2", Label: "Policies", Action: "goto_rbac_policies"},
		pluginrpc.KeyBinding{Key: "3", Label: "Groups", Action: "goto_rbac_groups"},
		pluginrpc.KeyBinding{Key: "V", Label: "Details", Action: "view_details"},
	)

	switch viewID {
	case viewRBACPolicies:
		rows := make([][]string, 0, len(s.rbacData.Policies))
		for _, p := range s.rbacData.Policies {
			rows = append(rows, []string{p.Subject, p.Resource, p.Action, p.Object, p.Effect})
		}
		if len(rows) == 0 {
			rows = [][]string{{"No policy rules found", "", "", "", ""}}
		}
		return pluginrpc.ViewData{
			View:         viewRBACPolicies,
			Title:        "RBAC Policies",
			Info:         s.baseInfo(fmt.Sprintf("Policies: %d", len(s.rbacData.Policies))),
			Status:       "ok",
			Headers:      []string{"Subject", "Resource", "Action", "Object", "Effect"},
			Rows:         rows,
			SelectionKey: "Subject",
			KeyBindings:  nav,
		}, nil

	case viewRBACGroups:
		rows := make([][]string, 0, len(s.rbacData.Groups))
		for _, g := range s.rbacData.Groups {
			rows = append(rows, []string{g.User, g.Role})
		}
		if len(rows) == 0 {
			rows = [][]string{{"No group bindings found", ""}}
		}
		return pluginrpc.ViewData{
			View:         viewRBACGroups,
			Title:        "RBAC Groups",
			Info:         s.baseInfo(fmt.Sprintf("Groups: %d", len(s.rbacData.Groups))),
			Status:       "ok",
			Headers:      []string{"User", "Role"},
			Rows:         rows,
			SelectionKey: "User",
			KeyBindings:  nav,
		}, nil

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
		if len(rows) == 0 {
			rows = [][]string{{"No accounts found in argocd-cm", "", ""}}
		}
		return pluginrpc.ViewData{
			View:         viewRBAC,
			Title:        "RBAC Accounts",
			Info:         s.baseInfo(fmt.Sprintf("RBAC accounts: %d", len(s.rbacData.Accounts))),
			Status:       "ok",
			Headers:      []string{"Name", "Capabilities", "Enabled"},
			Rows:         rows,
			SelectionKey: "Name",
			KeyBindings:  nav,
		}, nil
	}
}
