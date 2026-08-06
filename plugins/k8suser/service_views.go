package k8suser

import (
	"fmt"
	"os/exec"
	"strings"

	"omo/pkg/pluginrpc"
)

func k8sNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "U", Label: "Users", Action: "goto_users"},
		{Key: "M", Label: "Roles", Action: "goto_roles"},
	}
}

func withK8sNav(extra ...pluginrpc.KeyBinding) []pluginrpc.KeyBinding {
	out := make([]pluginrpc.KeyBinding, 0, len(extra)+len(k8sNavBindings())+1)
	out = append(out, pluginrpc.KeyBinding{Key: "R", Label: "Refresh", Action: "refresh"})
	out = append(out, extra...)
	out = append(out, k8sNavBindings()...)
	return out
}

func (s *Service) baseInfo(extra string) string {
	msg := fmt.Sprintf("[green]K8s User Manager[white]\nContext: %s\nKubeconfig: %s\nView: %s",
		s.client.CurrentContext, s.client.KubeConfig, s.currentView)
	if extra != "" {
		msg += "\n" + extra
	}
	return msg
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
		return pluginrpc.ViewData{
			View:    k8sViewUsers,
			Title:   "K8s Users",
			Info:    s.baseInfo("No context configured"),
			Status:  "not configured",
			Headers: []string{"Status", "Detail"},
			Rows:    [][]string{{"error", "Configure kubeconfig/context via host secrets"}},
			KeyBindings: withK8sNav(
				pluginrpc.KeyBinding{Key: "C", Label: "Create User", Action: "create_user"},
			),
		}, nil
	}

	users, err := s.client.GetUsers()
	if err != nil {
		return pluginrpc.ViewData{
			View:    k8sViewUsers,
			Title:   "K8s Users",
			Info:    s.baseInfo(err.Error()),
			Status:  "error",
			Headers: []string{"Status", "Detail"},
			Rows:    [][]string{{"error", err.Error()}},
			KeyBindings: withK8sNav(
				pluginrpc.KeyBinding{Key: "C", Label: "Create User", Action: "create_user"},
			),
		}, nil
	}

	rows := make([][]string, 0, len(users))
	for _, u := range users {
		rows = append(rows, []string{u.Username, u.CertExpiry, u.Namespace, u.Roles})
	}
	if len(rows) == 0 {
		rows = [][]string{{"No certificate-based users found", "Use create_user", "", ""}}
	}

	return pluginrpc.ViewData{
		View:         k8sViewUsers,
		Title:        "K8s Users",
		Info:         s.baseInfo(fmt.Sprintf("Users: %d", len(users))),
		Status:       "ok",
		Headers:      []string{"Username", "Certificate Expiry", "Namespaces", "Roles"},
		Rows:         rows,
		SelectionKey: "Username",
		KeyBindings: withK8sNav(
			pluginrpc.KeyBinding{Key: "C", Label: "Create User", Action: "create_user"},
			pluginrpc.KeyBinding{Key: "D", Label: "Delete", Action: "delete"},
			pluginrpc.KeyBinding{Key: "A", Label: "Assign Role", Action: "assign_role"},
			pluginrpc.KeyBinding{Key: "V", Label: "Details", Action: "view_details"},
			pluginrpc.KeyBinding{Key: "T", Label: "Test Access", Action: "test_access"},
			pluginrpc.KeyBinding{Key: "K", Label: "Connection Cmd", Action: "connection_command"},
		),
	}, nil
}

func (s *Service) k8sViewRolesLocked() (pluginrpc.ViewData, error) {
	rows, err := s.fetchRolesLocked()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	return pluginrpc.ViewData{
		View:         k8sViewRoles,
		Title:        "K8s Roles",
		Info:         s.baseInfo(fmt.Sprintf("Roles: %d", len(rows))),
		Status:       "ok",
		Headers:      []string{"Name", "Namespace", "Resources"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings: withK8sNav(
			pluginrpc.KeyBinding{Key: "C", Label: "Create Role", Action: "create_role"},
			pluginrpc.KeyBinding{Key: "D", Label: "Delete", Action: "delete"},
			pluginrpc.KeyBinding{Key: "V", Label: "Details", Action: "view_details"},
		),
	}, nil
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
