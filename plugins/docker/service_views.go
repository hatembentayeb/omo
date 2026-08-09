package docker

import (
	"fmt"

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

func helpSections() []pluginrpc.HelpSection {
	return []pluginrpc.HelpSection{
		{Title: "Views (0-6)", Bindings: viewNavBindings()},
		{Title: "Containers", Bindings: containersActions()},
		{Title: "Images", Bindings: imagesActions()},
		{Title: "Networks", Bindings: networksActions()},
		{Title: "Volumes", Bindings: volumesActions()},
		{Title: "Compose", Bindings: composeActions()},
		{Title: "System", Bindings: systemActions()},
		{
			Title: "Global",
			Bindings: []pluginrpc.KeyBinding{
				{Key: "R", Label: "Refresh"},
				{Key: "?", Label: "Help (this screen)"},
				{Key: "/", Label: "Filter"},
				{Key: "^t", Label: "Switch target"},
				{Key: "ESC", Label: "Back / home"},
			},
		},
	}
}

// decorate: Views column = 0-6, Actions column (former logs) = this view only.
func decorate(view pluginrpc.ViewData, actions ...pluginrpc.KeyBinding) pluginrpc.ViewData {
	view.ViewBindings = viewNavBindings()
	view.KeyBindings = nil
	view.Actions = actions
	view.HelpSections = helpSections()
	return view
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
	if extra != "" {
		msg += "\n" + extra
	}
	return msg
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewContainers
	}
	s.currentView = viewID

	if s.client == nil || !s.client.IsConnected() {
		return pluginrpc.ViewData{
			View:    viewID,
			Title:   "Docker Manager",
			Info:    "[yellow]Docker Manager[white]\nStatus: Not Connected",
			Status:  "not connected",
			Headers: []string{"Status", "Detail"},
			Rows:    [][]string{{"error", "not connected — Configure with a docker host"}},
			KeyBindings: []pluginrpc.KeyBinding{
				{Key: "R", Label: "Refresh", Action: "refresh"},
			},
		}, nil
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
	default:
		return s.viewContainersLocked()
	}
}

func (s *Service) viewContainersLocked() (pluginrpc.ViewData, error) {
	list, err := s.client.ListContainers()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(list))
	for i := range list {
		rows = append(rows, list[i].GetTableRow())
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "-", "No containers", "-"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewContainers,
		Title:        "Docker Containers",
		Info:         s.baseInfo(fmt.Sprintf("Containers: %d", len(list))),
		Status:       "connected",
		Headers:      []string{"ID", "Name", "Image", "State", "Status", "Ports"},
		Rows:         rows,
		SelectionKey: "ID",
	}, containersActions()...), nil
}

func (s *Service) viewImagesLocked() (pluginrpc.ViewData, error) {
	list, err := s.client.ListImages()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(list))
	for i := range list {
		rows = append(rows, list[i].GetTableRow())
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "-", "No images"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewImages,
		Title:        "Docker Images",
		Info:         s.baseInfo(fmt.Sprintf("Images: %d", len(list))),
		Status:       "connected",
		Headers:      []string{"ID", "Repository", "Tag", "Size", "Created"},
		Rows:         rows,
		SelectionKey: "ID",
	}, imagesActions()...), nil
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
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "-", "-", "No networks"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewNetworks,
		Title:        "Docker Networks",
		Info:         s.baseInfo(fmt.Sprintf("Networks: %d", len(list))),
		Status:       "connected",
		Headers:      []string{"ID", "Name", "Driver", "Scope", "Subnet", "Gateway"},
		Rows:         rows,
		SelectionKey: "ID",
	}, networksActions()...), nil
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
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "-", "No volumes"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewVolumes,
		Title:        "Docker Volumes",
		Info:         s.baseInfo(fmt.Sprintf("Volumes: %d", len(list))),
		Status:       "connected",
		Headers:      []string{"Name", "Driver", "Mountpoint", "Scope", "Created"},
		Rows:         rows,
		SelectionKey: "Name",
	}, volumesActions()...), nil
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
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "-", "-", "-", "-", "No running containers"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewStats,
		Title:        "Docker Stats",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Container", "CPU %", "Memory", "Mem %", "Net I/O", "Block I/O", "PIDs"},
		Rows:         rows,
		SelectionKey: "Container",
	}), nil
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
	if len(rows) == 0 {
		rows = append(rows, []string{"-", "-", "0", "0", "No compose projects"})
	}
	return decorate(pluginrpc.ViewData{
		View:         viewCompose,
		Title:        "Docker Compose",
		Info:         s.baseInfo(fmt.Sprintf("Projects: %d", len(projects))),
		Status:       "connected",
		Headers:      []string{"Project", "Status", "Services", "Running", "Config"},
		Rows:         rows,
		SelectionKey: "Project",
	}, composeActions()...), nil
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
	return decorate(pluginrpc.ViewData{
		View:         viewSystem,
		Title:        "Docker System",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Property", "Value"},
		Rows:         rows,
		SelectionKey: "Property",
	}, systemActions()...), nil
}
