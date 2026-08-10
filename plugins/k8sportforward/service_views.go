package k8sportforward

import (
	"fmt"
	"strconv"
	"strings"

	"omo/pkg/pluginrpc"
)

func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Workloads", Action: "goto_workloads"},
		{Key: "1", Label: "Services", Action: "goto_services"},
		{Key: "2", Label: "Pods", Action: "goto_pods"},
		{Key: "3", Label: "Forwards", Action: "goto_forwards"},
		{Key: "4", Label: "Namespaces", Action: "goto_namespaces"},
		{Key: "5", Label: "Ports", Action: "goto_ports"},
	}
}

func workloadActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "F", Label: "Forward", Action: "start_forward"},
		{Key: "X", Label: "Stop Fwd", Action: "stop_forward"},
		{Key: "E", Label: "Details", Action: "view_details"},
		{Key: "C", Label: "Conn Info", Action: "connection_info"},
		{Key: "P", Label: "Suggest Port", Action: "suggest_port"},
	}
}

func serviceActions() []pluginrpc.KeyBinding {
	return workloadActions()
}

func podActions() []pluginrpc.KeyBinding {
	return workloadActions()
}

func forwardActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "X", Label: "Stop", Action: "stop_forward"},
		{Key: "A", Label: "Stop All", Action: "stop_all_forwards"},
		{Key: "E", Label: "Details", Action: "view_details"},
		{Key: "C", Label: "Conn Info", Action: "connection_info"},
	}
}

func namespaceActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "F", Label: "Filter", Action: "filter_namespace"},
		{Key: "C", Label: "Clear Filter", Action: "clear_namespace_filter"},
	}
}

func portActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "X", Label: "Stop", Action: "stop_forward"},
		{Key: "C", Label: "Conn Info", Action: "connection_info"},
		{Key: "E", Label: "Details", Action: "view_details"},
	}
}

func helpSections() []pluginrpc.HelpSection {
	return pluginrpc.HelpNav(viewNavBindings(), nil,
		pluginrpc.HelpSection{Title: "Workloads", Bindings: workloadActions()},
		pluginrpc.HelpSection{Title: "Services", Bindings: serviceActions()},
		pluginrpc.HelpSection{Title: "Pods", Bindings: podActions()},
		pluginrpc.HelpSection{Title: "Forwards", Bindings: forwardActions()},
		pluginrpc.HelpSection{Title: "Namespaces", Bindings: namespaceActions()},
		pluginrpc.HelpSection{Title: "Ports", Bindings: portActions()},
	)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	Help:  helpSections,
}

func (s *Service) baseInfo(extra string) string {
	name := s.name
	if name == "" {
		name = "cluster"
	}
	ctx := s.contextName
	if ctx == "" && s.client != nil {
		ctx = s.client.Context()
	}
	server := s.server
	if server == "" && s.client != nil {
		server = s.client.Server()
	}
	ns := s.namespaceFilter
	if ns == "" {
		ns = "all (non-system)"
	}
	active := 0
	if s.forwards != nil {
		active = s.forwards.Count()
	}
	msg := fmt.Sprintf("[green]K8s Port Forward[white]\nCluster: %s\nContext: %s\nServer: %s\nNamespace: %s\nActive ●F: %d\nView: %s",
		name, ctx, server, ns, active, s.currentView)
	return pluginrpc.FormatInfo(msg, extra)
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewWorkloads
	}
	s.currentView = viewID

	if err := s.ensureConnectedLocked(); err != nil {
		return ui.NotConnectedErr(viewID, "K8s Port Forward", err)
	}

	switch viewID {
	case viewWorkloads:
		return s.viewWorkloadsLocked()
	case viewServices:
		return s.viewServicesLocked()
	case viewPods:
		return s.viewPodsLocked()
	case viewForwards:
		return s.viewForwardsLocked()
	case viewNamespaces:
		return s.viewNamespacesLocked()
	case viewPorts:
		return s.viewPortsLocked()
	default:
		return s.viewWorkloadsLocked()
	}
}

func (s *Service) viewWorkloadsLocked() (pluginrpc.ViewData, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	list, err := s.client.ListWorkloads(ctx, s.namespaceFilter)
	if err != nil {
		return ui.NotConnectedErr(viewWorkloads, "K8s Port Forward", err)
	}
	headers := []string{"Fwd", "Kind", "Namespace", "Name", "Ready", "Type", "Ports", "Local"}
	rows := make([][]string, 0, len(list))
	for _, w := range list {
		fwd, local := s.fwdCells(w.Kind, w.Namespace, w.Name)
		rows = append(rows, []string{
			fwd,
			w.Kind,
			w.Namespace,
			w.Name,
			w.Ready,
			w.Type,
			formatPorts(w.Ports),
			local,
		})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"○", "-", "-", "No workloads", "-", "-", "-", "-"})
	extra := fmt.Sprintf("%d workloads (Deployments + StatefulSets)", len(list))
	return ui.Connected(viewWorkloads, "Workloads", s.baseInfo(extra), headers, rows, "Name", workloadActions()...), nil
}

func (s *Service) viewServicesLocked() (pluginrpc.ViewData, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	list, err := s.client.ListServices(ctx, s.namespaceFilter)
	if err != nil {
		return ui.NotConnectedErr(viewServices, "K8s Port Forward", err)
	}
	headers := []string{"Fwd", "Kind", "Namespace", "Name", "SvcType", "Type", "Ports", "Local"}
	rows := make([][]string, 0, len(list))
	for _, svc := range list {
		fwd, local := s.fwdCells("Service", svc.Namespace, svc.Name)
		ports := make([]int, 0, len(svc.Ports))
		for _, p := range svc.Ports {
			ports = append(ports, int(p.Port))
		}
		rows = append(rows, []string{
			fwd,
			"Service",
			svc.Namespace,
			svc.Name,
			svc.Type,
			svc.Category,
			formatPorts(ports),
			local,
		})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"○", "-", "-", "No services", "-", "-", "-", "-"})
	return ui.Connected(viewServices, "Services", s.baseInfo(fmt.Sprintf("%d services", len(list))), headers, rows, "Name", serviceActions()...), nil
}

func (s *Service) viewPodsLocked() (pluginrpc.ViewData, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	list, err := s.client.ListPods(ctx, s.namespaceFilter)
	if err != nil {
		return ui.NotConnectedErr(viewPods, "K8s Port Forward", err)
	}
	headers := []string{"Fwd", "Kind", "Namespace", "Name", "Phase", "Type", "Ports", "Local"}
	rows := make([][]string, 0, len(list))
	for _, p := range list {
		fwd, local := s.fwdCells("Pod", p.Namespace, p.Name)
		phase := p.Phase
		if p.Ready {
			phase = phase + " ✓"
		}
		rows = append(rows, []string{
			fwd,
			"Pod",
			p.Namespace,
			p.Name,
			phase,
			p.Type,
			formatPorts(p.Ports),
			local,
		})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"○", "-", "-", "No pods", "-", "-", "-", "-"})
	return ui.Connected(viewPods, "Pods", s.baseInfo(fmt.Sprintf("%d pods", len(list))), headers, rows, "Name", podActions()...), nil
}

func (s *Service) viewForwardsLocked() (pluginrpc.ViewData, error) {
	list := s.forwards.List()
	headers := []string{"ID", "Status", "Target", "Local", "Remote", "Age"}
	rows := make([][]string, 0, len(list))
	for _, f := range list {
		status := f.Status
		if status == "forwarding" {
			status = "●F " + status
		}
		rows = append(rows, []string{
			f.ID,
			status,
			fmt.Sprintf("%s %s/%s", f.Kind, f.Namespace, f.Name),
			strconv.Itoa(f.LocalPort),
			strconv.Itoa(f.RemotePort),
			f.Age(),
		})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"-", "idle", "No active forwards", "-", "-", "-"})
	extra := fmt.Sprintf("%d active forward(s)", len(list))
	return ui.Connected(viewForwards, "Active Forwards", s.baseInfo(extra), headers, rows, "ID", forwardActions()...), nil
}

func (s *Service) viewNamespacesLocked() (pluginrpc.ViewData, error) {
	ctx, cancel := ctxTimeout()
	defer cancel()
	list, err := s.client.ListUserNamespaces(ctx)
	if err != nil {
		return ui.NotConnectedErr(viewNamespaces, "K8s Port Forward", err)
	}
	headers := []string{"Namespace", "Status", "Age", "Filter"}
	rows := make([][]string, 0, len(list))
	for _, ns := range list {
		mark := ""
		if s.namespaceFilter == ns.Name {
			mark = "←"
		}
		rows = append(rows, []string{ns.Name, ns.Status, ns.Age, mark})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"No user namespaces", "-", "-", "-"})
	return ui.Connected(viewNamespaces, "Namespaces", s.baseInfo("non-system only"), headers, rows, "Namespace", namespaceActions()...), nil
}

func (s *Service) viewPortsLocked() (pluginrpc.ViewData, error) {
	list := s.forwards.List()
	headers := []string{"Local", "Remote", "Address", "Target", "Status", "ID"}
	rows := make([][]string, 0, len(list))
	for _, f := range list {
		rows = append(rows, []string{
			strconv.Itoa(f.LocalPort),
			strconv.Itoa(f.RemotePort),
			f.Address(),
			fmt.Sprintf("%s/%s", f.Namespace, f.Name),
			"●F " + f.Status,
			f.ID,
		})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "127.0.0.1", "No ports forwarded", "-", "-"})
	extra := "Bind redis/postgres KeePass host=127.0.0.1 to these local ports"
	return ui.Connected(viewPorts, "Port Registry", s.baseInfo(extra), headers, rows, "Local", portActions()...), nil
}

func (s *Service) fwdCells(kind, namespace, name string) (fwd, local string) {
	ports := s.forwards.LocalPortsFor(kind, namespace, name)
	if len(ports) == 0 {
		return fwdMarkInactive, "-"
	}
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = strconv.Itoa(p)
	}
	return fwdMarkActive, strings.Join(parts, ",")
}
