package k8suser

import (
	"fmt"
	"os/exec"
	"strings"

	"omo/pkg/pluginrpc"
)

const titleK8sUsers = "K8s Users"

func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Users", Action: "goto_users"},
		{Key: "1", Label: "Roles", Action: "goto_roles"},
	}
}

func usersActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "C", Label: "Create User", Action: "create_user"},
		{Key: "D", Label: "Delete", Action: "delete"},
		{Key: "A", Label: "Assign Role", Action: "assign_role"},
		{Key: "V", Label: "Details", Action: "view_details"},
		{Key: "T", Label: "Test Access", Action: "test_access"},
		{Key: "K", Label: "Connection Cmd", Action: "connection_command"},
		{Key: "X", Label: "Set Context", Action: "set_context"},
	}
}

func rolesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "C", Label: "Create Role", Action: "create_role"},
		{Key: "D", Label: "Delete", Action: "delete"},
		{Key: "V", Label: "Details", Action: "view_details"},
	}
}

func helpSections() []pluginrpc.HelpSection {
	return pluginrpc.HelpNav(viewNavBindings(), nil,
		pluginrpc.HelpSection{Title: "Users", Bindings: usersActions()},
		pluginrpc.HelpSection{Title: "Roles", Bindings: rolesActions()},
	)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	Help:  helpSections,
}

func (s *Service) baseInfo(extra string) string {
	msg := fmt.Sprintf("[green]K8s User Manager[white]\nContext: %s\nKubeconfig: %s\nView: %s",
		s.client.CurrentContext, s.client.KubeConfig, s.currentView)
	return pluginrpc.FormatInfo(msg, extra)
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = k8sViewUsers
	}
	s.currentView = viewID

	if s.client.CurrentContext == "" && s.client.KubeConfig == "" {
		_, _ = s.client.GetKubeConfig()
	}

	switch viewID {
	case k8sViewRoles:
		return s.k8sViewRolesLocked()
	default:
		return s.k8sViewUsersLocked()
	}
}

func (s *Service) k8sViewUsersLocked() (pluginrpc.ViewData, error) {
	if s.client.CurrentContext == "" {
		return ui.StatusError(k8sViewUsers, titleK8sUsers, s.baseInfo("No context configured"),
			"not configured", "Configure kubeconfig/context via host secrets", usersActions()...), nil
	}

	users, err := s.client.GetUsers()
	if err != nil {
		return ui.StatusError(k8sViewUsers, titleK8sUsers, s.baseInfo(err.Error()),
			"error", err.Error(), usersActions()...), nil
	}

	rows := make([][]string, 0, len(users))
	for _, u := range users {
		rows = append(rows, []string{u.Username, u.CertExpiry, u.Namespace, u.Roles})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"No certificate-based users found", "Use create_user", "", ""})

	return ui.OK(k8sViewUsers, titleK8sUsers, s.baseInfo(fmt.Sprintf("Users: %d", len(users))),
		[]string{"Username", "Certificate Expiry", "Namespaces", "Roles"}, rows, "Username", usersActions()...), nil
}

func (s *Service) k8sViewRolesLocked() (pluginrpc.ViewData, error) {
	rows, err := s.fetchRolesLocked()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	return ui.OK(k8sViewRoles, "K8s Roles", s.baseInfo(fmt.Sprintf("Roles: %d", len(rows))),
		[]string{"Name", "Namespace", "Resources"}, rows, "Name", rolesActions()...), nil
}

func (s *Service) fetchRolesLocked() ([][]string, error) {
	namespaces, err := s.client.GetNamespaces()
	if err != nil {
		return [][]string{{"Error fetching namespaces", err.Error(), ""}}, nil
	}

	var allRoles [][]string
	for _, namespace := range namespaces {
		if namespace == "cluster-wide" {
			cmd := exec.Command("kubectl", "get", "clusterroles", "-o",
				`jsonpath={range .items[*]}{.metadata.name}{"\t"}{"cluster-wide"}{"\t"}{.rules[0].resources}{"\n"}{end}`)
			output, err := cmd.CombinedOutput()
			if err != nil {
				continue
			}
			for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
				if line == "" {
					continue
				}
				parts := strings.Split(line, "\t")
				if len(parts) < 3 {
					continue
				}
				if strings.HasPrefix(parts[0], "system:") {
					continue
				}
				allRoles = append(allRoles, []string{parts[0], "cluster-wide", parts[2]})
			}
			continue
		}

		cmd := exec.Command("kubectl", "get", "roles", "-n", namespace, "-o",
			`jsonpath={range .items[*]}{.metadata.name}{"\t"}{.metadata.namespace}{"\t"}{.rules[0].resources}{"\n"}{end}`)
		output, err := cmd.CombinedOutput()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if line == "" {
				continue
			}
			parts := strings.Split(line, "\t")
			if len(parts) < 3 {
				continue
			}
			allRoles = append(allRoles, []string{parts[0], parts[1], parts[2]})
		}
	}

	if len(allRoles) == 0 {
		return [][]string{{"No custom roles found", "Use create_role", ""}}, nil
	}
	return allRoles, nil
}
