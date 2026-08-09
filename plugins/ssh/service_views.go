package ssh

import (
	"fmt"
	"strings"
	"time"

	"omo/pkg/pluginrpc"
)

func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Servers", Action: "goto_servers"},
		{Key: "1", Label: "Overview", Action: "goto_overview"},
		{Key: "2", Label: "Processes", Action: "goto_processes"},
		{Key: "3", Label: "Disk", Action: "goto_disk"},
		{Key: "4", Label: "Network", Action: "goto_network"},
		{Key: "5", Label: "Docker", Action: "goto_docker"},
		{Key: "6", Label: "Services", Action: "goto_services"},
	}
}

func serversActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "C", Label: "Connect", Action: "connect"},
		{Key: "I", Label: "Server Info", Action: "server_info"},
		{Key: "S", Label: "Shell", Action: "shell"},
		{Key: "E", Label: "Execute", Action: "execute"},
	}
}

func overviewActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "S", Label: "Shell", Action: "shell"},
		{Key: "E", Label: "Execute", Action: "execute"},
		{Key: "I", Label: "Server Info", Action: "server_info"},
	}
}

func shellActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "S", Label: "Shell", Action: "shell"},
	}
}

func connectActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "C", Label: "Connect", Action: "connect"},
	}
}

func helpSections() []pluginrpc.HelpSection {
	return []pluginrpc.HelpSection{
		{Title: "Views (0-6)", Bindings: viewNavBindings()},
		{Title: "Servers", Bindings: serversActions()},
		{Title: "Overview", Bindings: overviewActions()},
		{Title: "Shell", Bindings: shellActions()},
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
	status := "Not Connected"
	if s.client != nil && s.client.IsConnected() {
		status = fmt.Sprintf("Connected (%s)", s.client.GetConnectedDuration().Truncate(time.Second))
	}
	msg := fmt.Sprintf("[green]SSH Manager[white]\nServer: %s@%s:%d\nStatus: %s\nView: %s",
		s.server.User, s.server.Host, s.server.Port, status, s.currentView)
	if extra != "" {
		msg += "\n" + extra
	}
	return msg
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewServers
	}
	s.currentView = viewID

	switch viewID {
	case viewOverview:
		return s.viewOverviewLocked()
	case viewProcesses:
		return s.viewProcessesLocked()
	case viewDisk:
		return s.viewDiskLocked()
	case viewNetwork:
		return s.viewNetworkLocked()
	case viewDocker:
		return s.viewDockerLocked()
	case viewServices:
		return s.viewServicesLocked()
	default:
		return s.viewServersLocked()
	}
}

func (s *Service) viewServersLocked() (pluginrpc.ViewData, error) {
	rows := [][]string{}
	if s.configured {
		proxy := "-"
		if s.server.ProxyCommand != "" {
			proxy = "proxy"
		} else if s.server.JumpHost != "" {
			proxy = "jump:" + s.server.JumpHost
		}
		tags := strings.Join(s.server.Tags, ",")
		if tags == "" {
			tags = "-"
		}
		rows = append(rows, []string{
			s.server.Name,
			s.server.Environment,
			s.server.Host,
			fmt.Sprintf("%d", s.server.Port),
			s.server.User,
			s.server.AuthMethod,
			proxy,
			tags,
		})
	}
	if len(rows) == 0 {
		rows = [][]string{{"No server configured", "", "", "", "", "", "", ""}}
	}

	return decorate(pluginrpc.ViewData{
		View:         viewServers,
		Title:        "SSH Servers",
		Info:         s.baseInfo("Configured via host secrets (multi-server discovery skipped in RPC)"),
		Status:       "ok",
		Headers:      []string{"Name", "Environment", "Host", "Port", "User", "Auth", "Proxy/Jump", "Tags"},
		Rows:         rows,
		SelectionKey: "Name",
	}, serversActions()...), nil
}

func (s *Service) viewOverviewLocked() (pluginrpc.ViewData, error) {
	if err := s.ensureConnectedLocked(); err != nil {
		return s.notConnectedView(viewOverview, err)
	}
	info, err := s.client.GetHostInfo()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := [][]string{
		{"Hostname", info.Hostname},
		{"OS", info.OS},
		{"Kernel", info.Kernel},
		{"Uptime", info.Uptime},
		{"CPUs", info.CPUCount},
		{"Memory Total", info.MemTotal},
		{"Memory Available", info.MemAvail},
		{"Disk /", info.DiskUsage},
		{"Load Avg", info.LoadAvg},
		{"IP Addresses", strings.Join(info.IPAddresses, ", ")},
		{"Last Login", info.LastLogin},
		{"Connected For", s.client.GetConnectedDuration().Truncate(time.Second).String()},
	}
	return decorate(pluginrpc.ViewData{
		View:         viewOverview,
		Title:        "Overview",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      []string{"Property", "Value"},
		Rows:         rows,
		SelectionKey: "Property",
	}, overviewActions()...), nil
}

func (s *Service) viewProcessesLocked() (pluginrpc.ViewData, error) {
	if err := s.ensureConnectedLocked(); err != nil {
		return s.notConnectedView(viewProcesses, err)
	}
	rows, err := s.client.GetProcesses()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	headers := []string{"USER", "PID", "%CPU", "%MEM", "VSZ", "RSS", "TTY", "STAT", "START", "TIME", "COMMAND"}
	if len(rows) > 0 && looksLikeHeader(rows[0]) {
		headers = rows[0]
		rows = rows[1:]
	}
	rows = padRows(rows, len(headers))
	return decorate(pluginrpc.ViewData{
		View:         viewProcesses,
		Title:        "Processes",
		Info:         s.baseInfo(fmt.Sprintf("Processes: %d", len(rows))),
		Status:       "ok",
		Headers:      headers,
		Rows:         rows,
		SelectionKey: "PID",
	}, shellActions()...), nil
}

func (s *Service) viewDiskLocked() (pluginrpc.ViewData, error) {
	if err := s.ensureConnectedLocked(); err != nil {
		return s.notConnectedView(viewDisk, err)
	}
	rows, err := s.client.GetDiskUsage()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	headers := []string{"Filesystem", "Size", "Used", "Avail", "Use%", "Mounted"}
	if len(rows) > 0 && looksLikeHeader(rows[0]) {
		headers = normalizeDiskHeaders(rows[0])
		rows = rows[1:]
	}
	rows = padRows(rows, len(headers))
	return decorate(pluginrpc.ViewData{
		View:         viewDisk,
		Title:        "Disk Usage",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      headers,
		Rows:         rows,
		SelectionKey: "Filesystem",
	}, shellActions()...), nil
}

func (s *Service) viewNetworkLocked() (pluginrpc.ViewData, error) {
	if err := s.ensureConnectedLocked(); err != nil {
		return s.notConnectedView(viewNetwork, err)
	}
	rows, err := s.client.GetNetworkConnections()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	headers := []string{"Proto", "Recv-Q", "Send-Q", "Local", "Remote", "State", "Process"}
	if len(rows) > 0 && looksLikeHeader(rows[0]) {
		headers = rows[0]
		rows = rows[1:]
	}
	rows = padRows(rows, len(headers))
	return decorate(pluginrpc.ViewData{
		View:         viewNetwork,
		Title:        "Network",
		Info:         s.baseInfo(""),
		Status:       "ok",
		Headers:      headers,
		Rows:         rows,
		SelectionKey: "Local",
	}, shellActions()...), nil
}

func (s *Service) viewDockerLocked() (pluginrpc.ViewData, error) {
	if err := s.ensureConnectedLocked(); err != nil {
		return s.notConnectedView(viewDocker, err)
	}
	rows, err := s.client.GetDockerContainers()
	if err != nil {
		return decorate(pluginrpc.ViewData{
			View:    viewDocker,
			Title:   "Docker",
			Info:    s.baseInfo(err.Error()),
			Status:  "unavailable",
			Headers: []string{"Status", "Detail"},
			Rows:    [][]string{{"unavailable", err.Error()}},
		}, shellActions()...), nil
	}
	headers := []string{"Names", "Image", "Status", "Ports"}
	if len(rows) > 0 && looksLikeHeader(rows[0]) {
		headers = rows[0]
		rows = rows[1:]
	}
	rows = padRows(rows, len(headers))
	return decorate(pluginrpc.ViewData{
		View:         viewDocker,
		Title:        "Docker",
		Info:         s.baseInfo(fmt.Sprintf("Containers: %d", len(rows))),
		Status:       "ok",
		Headers:      headers,
		Rows:         rows,
		SelectionKey: "Names",
	}, shellActions()...), nil
}

func (s *Service) viewServicesLocked() (pluginrpc.ViewData, error) {
	if err := s.ensureConnectedLocked(); err != nil {
		return s.notConnectedView(viewServices, err)
	}
	rows, err := s.client.GetSystemdServices()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	headers := []string{"Unit", "Load", "Active", "Sub", "Description"}
	rows = padRows(rows, len(headers))
	return decorate(pluginrpc.ViewData{
		View:         viewServices,
		Title:        "Services",
		Info:         s.baseInfo(fmt.Sprintf("Services: %d", len(rows))),
		Status:       "ok",
		Headers:      headers,
		Rows:         rows,
		SelectionKey: "Unit",
	}, shellActions()...), nil
}

func (s *Service) notConnectedView(viewID string, err error) (pluginrpc.ViewData, error) {
	return decorate(pluginrpc.ViewData{
		View:    viewID,
		Title:   "SSH Manager",
		Info:    s.baseInfo(err.Error()),
		Status:  "not connected",
		Headers: []string{"Status", "Detail"},
		Rows:    [][]string{{"error", err.Error()}},
	}, connectActions()...), nil
}

func looksLikeHeader(row []string) bool {
	if len(row) == 0 {
		return false
	}
	first := strings.ToUpper(row[0])
	return first == "USER" || first == "FILESYSTEM" || first == "PROTO" ||
		first == "NAMES" || first == "NETID" || first == "UNIT" ||
		strings.Contains(first, "FILESYSTEM")
}

func normalizeDiskHeaders(row []string) []string {
	if len(row) >= 6 {
		return []string{"Filesystem", "Size", "Used", "Avail", "Use%", "Mounted"}
	}
	return row
}

func padRows(rows [][]string, n int) [][]string {
	out := make([][]string, 0, len(rows))
	for _, r := range rows {
		if len(r) >= n {
			// Join trailing command fields for processes
			if len(r) > n {
				merged := make([]string, n)
				copy(merged, r[:n-1])
				merged[n-1] = strings.Join(r[n-1:], " ")
				out = append(out, merged)
			} else {
				out = append(out, r)
			}
			continue
		}
		padded := make([]string, n)
		copy(padded, r)
		for i := len(r); i < n; i++ {
			padded[i] = ""
		}
		out = append(out, padded)
	}
	return out
}
