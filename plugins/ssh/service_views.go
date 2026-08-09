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
	return pluginrpc.HelpWithGlobal([]pluginrpc.HelpSection{
		{Title: "Views (0-6)", Bindings: viewNavBindings()},
		{Title: "Servers", Bindings: serversActions()},
		{Title: "Overview", Bindings: overviewActions()},
		{Title: "Shell", Bindings: shellActions()},
	}...)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	Help:  helpSections,
}

func (s *Service) baseInfo(extra string) string {
	status := "Not Connected"
	if s.client != nil && s.client.IsConnected() {
		status = fmt.Sprintf("Connected (%s)", s.client.GetConnectedDuration().Truncate(time.Second))
	}
	msg := fmt.Sprintf("[green]SSH Manager[white]\nServer: %s@%s:%d\nStatus: %s\nView: %s",
		s.server.User, s.server.Host, s.server.Port, status, s.currentView)
	return pluginrpc.FormatInfo(msg, extra)
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
		return s.shellTableLocked(viewProcesses, "Processes", "PID",
			[]string{"USER", "PID", "%CPU", "%MEM", "VSZ", "RSS", "TTY", "STAT", "START", "TIME", "COMMAND"},
			s.client.GetProcesses, func(n int) string { return fmt.Sprintf("Processes: %d", n) }, nil)
	case viewDisk:
		return s.shellTableLocked(viewDisk, "Disk Usage", "Filesystem",
			[]string{"Filesystem", "Size", "Used", "Avail", "Use%", "Mounted"},
			s.client.GetDiskUsage, nil, normalizeDiskHeaders)
	case viewNetwork:
		return s.shellTableLocked(viewNetwork, "Network", "Local",
			[]string{"Proto", "Recv-Q", "Send-Q", "Local", "Remote", "State", "Process"},
			s.client.GetNetworkConnections, nil, nil)
	case viewDocker:
		return s.viewDockerLocked()
	case viewServices:
		return s.shellTableLocked(viewServices, "Services", "Unit",
			[]string{"Unit", "Load", "Active", "Sub", "Description"},
			s.client.GetSystemdServices, func(n int) string { return fmt.Sprintf("Services: %d", n) }, nil)
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
	rows = pluginrpc.EnsureRows(rows, []string{"No server configured", "", "", "", "", "", "", ""})

	return ui.OK(viewServers, "SSH Servers",
		s.baseInfo("Configured via host secrets (multi-server discovery skipped in RPC)"),
		[]string{"Name", "Environment", "Host", "Port", "User", "Auth", "Proxy/Jump", "Tags"},
		rows, "Name", serversActions()...), nil
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
	return ui.OK(viewOverview, "Overview", s.baseInfo(""),
		[]string{"Property", "Value"}, rows, "Property", overviewActions()...), nil
}

// shellTableLocked builds a connected remote-command table (processes/disk/network/services).
func (s *Service) shellTableLocked(
	viewID, title, selectionKey string,
	defaultHeaders []string,
	fetch func() ([][]string, error),
	infoExtra func(n int) string,
	normalizeHeaders func([]string) []string,
) (pluginrpc.ViewData, error) {
	if err := s.ensureConnectedLocked(); err != nil {
		return s.notConnectedView(viewID, err)
	}
	rows, err := fetch()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	headers := defaultHeaders
	if len(rows) > 0 && looksLikeHeader(rows[0]) {
		if normalizeHeaders != nil {
			headers = normalizeHeaders(rows[0])
		} else {
			headers = rows[0]
		}
		rows = rows[1:]
	}
	rows = padRows(rows, len(headers))
	extra := ""
	if infoExtra != nil {
		extra = infoExtra(len(rows))
	}
	return ui.OK(viewID, title, s.baseInfo(extra), headers, rows, selectionKey, shellActions()...), nil
}

func (s *Service) viewDockerLocked() (pluginrpc.ViewData, error) {
	if err := s.ensureConnectedLocked(); err != nil {
		return s.notConnectedView(viewDocker, err)
	}
	rows, err := s.client.GetDockerContainers()
	if err != nil {
		return ui.StatusError(viewDocker, "Docker", s.baseInfo(err.Error()), "unavailable", err.Error(), shellActions()...), nil
	}
	headers := []string{"Names", "Image", "Status", "Ports"}
	if len(rows) > 0 && looksLikeHeader(rows[0]) {
		headers = rows[0]
		rows = rows[1:]
	}
	rows = padRows(rows, len(headers))
	return ui.OK(viewDocker, "Docker", s.baseInfo(fmt.Sprintf("Containers: %d", len(rows))),
		headers, rows, "Names", shellActions()...), nil
}

func (s *Service) notConnectedView(viewID string, err error) (pluginrpc.ViewData, error) {
	return ui.StatusError(viewID, "SSH Manager", s.baseInfo(err.Error()), "not connected", err.Error(), connectActions()...), nil
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
