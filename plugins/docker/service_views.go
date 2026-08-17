package docker

import (
	"fmt"
	"strings"

	"omo/pkg/pluginrpc"
)

func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Containers", Action: "goto_containers"},
		{Key: "1", Label: "Images", Action: "goto_images"},
		{Key: "2", Label: "Networks", Action: "goto_networks"},
		{Key: "3", Label: "Volumes", Action: "goto_volumes"},
		{Key: "4", Label: "Stats", Action: "goto_stats"},
		{Key: "5", Label: "Compose", Action: "goto_compose"},
		{Key: "6", Label: "System", Action: "goto_system"},
	}
}

func containersActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "S", Label: "Start", Action: "start"},
		{Key: "X", Label: "Stop", Action: "stop"},
		{Key: "D", Label: "Delete", Action: "delete"},
		{Key: "L", Label: "Logs", Action: "logs"},
		{Key: "E", Label: "Inspect", Action: "inspect"},
		{Key: "P", Label: "Pause", Action: "pause"},
		{Key: "U", Label: "Unpause", Action: "unpause"},
		{Key: "K", Label: "Kill", Action: "kill"},
		{Key: "Z", Label: "Restart", Action: "restart"},
	}
}

func imagesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "D", Label: "Delete", Action: "delete"},
		{Key: "P", Label: "Pull", Action: "pull"},
		{Key: "H", Label: "History", Action: "history"},
		{Key: "U", Label: "Run", Action: "run"},
		{Key: "E", Label: "Inspect", Action: "inspect"},
	}
}

func networksActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "D", Label: "Delete", Action: "delete"},
		{Key: "E", Label: "Inspect", Action: "inspect"},
	}
}

func volumesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "D", Label: "Delete", Action: "delete"},
		{Key: "P", Label: "Prune", Action: "prune"},
		{Key: "E", Label: "Inspect", Action: "inspect"},
	}
}

func composeActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "U", Label: "Up", Action: "compose_up"},
		{Key: "D", Label: "Down", Action: "delete"},
		{Key: "S", Label: "Stop", Action: "compose_stop"},
		{Key: "Z", Label: "Restart", Action: "compose_restart"},
		{Key: "L", Label: "Logs", Action: "logs"},
	}
}

func systemActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "P", Label: "Prune All", Action: "prune_system"},
		{Key: "D", Label: "Disk Usage", Action: "disk_usage"},
		{Key: "E", Label: "Events", Action: "events"},
	}
}

func logsActions() []pluginrpc.KeyBinding {
	// Host owns wrap / mark / find / copy in the Actions column for LogsBody views.
	// Plugin-side actions can be added here later (e.g. follow, tail size).
	return nil
}

func helpSections() []pluginrpc.HelpSection {
	return pluginrpc.HelpNav(viewNavBindings(), nil,
		pluginrpc.HelpSection{Title: "Containers", Bindings: containersActions()},
		pluginrpc.HelpSection{Title: "Images", Bindings: imagesActions()},
		pluginrpc.HelpSection{Title: "Networks", Bindings: networksActions()},
		pluginrpc.HelpSection{Title: "Volumes", Bindings: volumesActions()},
		pluginrpc.HelpSection{Title: "Compose", Bindings: composeActions()},
		pluginrpc.HelpSection{Title: "System", Bindings: systemActions()},
		pluginrpc.HelpSection{Title: "Logs", Bindings: []pluginrpc.KeyBinding{
			{Key: "L", Label: "Open from container/compose"},
			{Key: "w", Label: "Toggle wrap"},
			{Key: "a", Label: "Toggle autoscroll"},
			{Key: "m", Label: "Mark line"},
			{Key: "/", Label: "Find"},
			{Key: "Y", Label: "Copy all"},
			{Key: "C", Label: "Copy marked lines"},
			{Key: "ESC", Label: "Back"},
		}},
	)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	Help:  helpSections,
}

func (s *Service) baseInfo(extra string) string {
	name := s.hostName
	if name == "" {
		name = "docker"
	}
	host := s.hostURL
	if host == "" {
		host = "(default)"
	}
	msg := fmt.Sprintf("[green]Docker Manager[white]\nHost: %s\nURL: %s\nView: %s", name, host, s.currentView)
	return pluginrpc.FormatInfo(msg, extra)
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewContainers
	}
	s.currentView = viewID

	if s.client == nil || !s.client.IsConnected() {
		return ui.NotConnected(viewID, "Docker Manager", "not connected — Configure with a docker host"), nil
	}

	switch viewID {
	case viewImages:
		return s.viewImagesLocked()
	case viewNetworks:
		return s.viewNetworksLocked()
	case viewVolumes:
		return s.viewVolumesLocked()
	case viewStats:
		return s.viewStatsLocked()
	case viewCompose:
		return s.viewComposeLocked()
	case viewSystem:
		return s.viewSystemLocked()
	case viewLogs:
		return s.viewLogsLocked()
	default:
		return s.viewContainersLocked()
	}
}

func (s *Service) viewLogsLocked() (pluginrpc.ViewData, error) {
	s.currentView = viewLogs
	if s.logsTarget == "" {
		return ui.Logs(viewLogs, "Logs", s.baseInfo("No target — press L on a container/compose project"),
			"(no logs target selected)"), nil
	}
	var body string
	var err error
	if s.logsCompose {
		body, err = s.client.ComposeLogs(s.logsTarget)
	} else {
		body, err = s.client.GetContainerLogs(s.logsTarget)
	}
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	title := "Logs: " + s.logsTarget
	extra := "Target: " + s.logsTarget
	if s.logsCompose {
		extra = "Compose: " + s.logsTarget
	}
	return ui.Logs(viewLogs, title, s.baseInfo(extra), body, logsActions()...), nil
}

func (s *Service) viewContainersLocked() (pluginrpc.ViewData, error) {
	list, err := s.client.ListContainers()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := pluginrpc.EnsureRows(pluginrpc.MapRows(list, func(c DockerContainer) []string { return c.GetTableRow() }), []string{"-", "-", "-", "-", "No containers", "-"})
	return ui.Connected(viewContainers, "Docker Containers", s.baseInfo(fmt.Sprintf("Containers: %d", len(list))), []string{"ID", "Name", "Image", "State", "Status", "Ports"}, rows, "ID", containersActions()...), nil
}

func (s *Service) viewDashboardLocked() (pluginrpc.ViewData, error) {
	info, err := s.client.GetSystemInfo()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	host := strings.TrimSpace(s.hostName)
	if host == "" {
		host = strings.TrimSpace(s.hostURL)
	}
	if host == "" {
		host = "local"
	}
	return pluginrpc.Widget("Docker", "connected", host, [][2]string{
		{"Running", fmt.Sprintf("%d", info.ContainersRunning)},
		{"Containers", fmt.Sprintf("%d", info.Containers)},
		{"Images", fmt.Sprintf("%d", info.Images)},
		{"Version", pluginrpc.Truncate(info.ServerVersion, 24)},
	}), nil
}

func (s *Service) viewImagesLocked() (pluginrpc.ViewData, error) {
	list, err := s.client.ListImages()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := pluginrpc.EnsureRows(pluginrpc.MapRows(list, func(img DockerImage) []string { return img.GetTableRow() }), []string{"-", "-", "-", "-", "No images"})
	return ui.Connected(viewImages, "Docker Images", s.baseInfo(fmt.Sprintf("Images: %d", len(list))), []string{"ID", "Repository", "Tag", "Size", "Created"}, rows, "ID", imagesActions()...), nil
}

func (s *Service) viewNetworksLocked() (pluginrpc.ViewData, error) {
	list, err := s.client.ListNetworks()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(list))
	for _, n := range list {
		id := n.ID
		if len(id) > 12 {
			id = id[:12]
		}
		rows = append(rows, []string{id, n.Name, n.Driver, n.Scope, n.Subnet, n.Gateway})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "-", "No networks"})
	return ui.Connected(viewNetworks, "Docker Networks", s.baseInfo(fmt.Sprintf("Networks: %d", len(list))), []string{"ID", "Name", "Driver", "Scope", "Subnet", "Gateway"}, rows, "ID", networksActions()...), nil
}

func (s *Service) viewVolumesLocked() (pluginrpc.ViewData, error) {
	list, err := s.client.ListVolumes()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(list))
	for _, v := range list {
		created := v.CreatedAt
		if created == "" && !v.Created.IsZero() {
			created = v.Created.Format("2006-01-02")
		}
		rows = append(rows, []string{v.Name, v.Driver, v.Mountpoint, v.Scope, created})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "No volumes"})
	return ui.Connected(viewVolumes, "Docker Volumes", s.baseInfo(fmt.Sprintf("Volumes: %d", len(list))), []string{"Name", "Driver", "Mountpoint", "Scope", "Created"}, rows, "Name", volumesActions()...), nil
}

func (s *Service) viewStatsLocked() (pluginrpc.ViewData, error) {
	stats, err := s.client.GetContainerStats()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(stats))
	for _, st := range stats {
		rows = append(rows, []string{st.Name, st.CPUPercent, st.MemoryUsage, st.MemoryPercent, st.NetIO, st.BlockIO, st.PIDs})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "-", "-", "-", "No running containers"})
	return ui.Connected(viewStats, "Docker Stats", s.baseInfo(""), []string{"Container", "CPU %", "Memory", "Mem %", "Net I/O", "Block I/O", "PIDs"}, rows, "Container"), nil
}

func (s *Service) viewComposeLocked() (pluginrpc.ViewData, error) {
	projects, err := s.client.ListComposeProjects()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(projects))
	for _, p := range projects {
		rows = append(rows, []string{
			p.Name,
			p.Status,
			fmt.Sprintf("%d", p.ServiceCount),
			fmt.Sprintf("%d", p.RunningCount),
			p.ConfigFile,
		})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "0", "0", "No compose projects"})
	return ui.Connected(viewCompose, "Docker Compose", s.baseInfo(fmt.Sprintf("Projects: %d", len(projects))), []string{"Project", "Status", "Services", "Running", "Config"}, rows, "Project", composeActions()...), nil
}

func (s *Service) viewSystemLocked() (pluginrpc.ViewData, error) {
	info, err := s.client.GetSystemInfo()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := [][]string{
		{"Server Version", info.ServerVersion},
		{"API Version", info.APIVersion},
		{"OS", info.OperatingSystem},
		{"Architecture", info.Architecture},
		{"Kernel", info.KernelVersion},
		{"CPUs", fmt.Sprintf("%d", info.NCPU)},
		{"Memory", info.MemTotal},
		{"Containers", fmt.Sprintf("%d (running %d, paused %d, stopped %d)",
			info.Containers, info.ContainersRunning, info.ContainersPaused, info.ContainersStopped)},
		{"Images", fmt.Sprintf("%d", info.Images)},
		{"Driver", info.Driver},
		{"Logging", info.LoggingDriver},
		{"Cgroup", info.CgroupDriver + " / " + info.CgroupVersion},
		{"Root Dir", info.DockerRootDir},
		{"Swarm", info.SwarmStatus},
	}
	return ui.Connected(viewSystem, "Docker System", s.baseInfo(""), []string{"Property", "Value"}, rows, "Property", systemActions()...), nil
}
