package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"omo/pkg/ui"

	"github.com/rivo/tview"
)

type SSHView struct {
	app         *tview.Application
	pages       *tview.Pages
	viewPages   *tview.Pages
	currentView string

	serversView   *ui.CoreView
	overviewView  *ui.CoreView
	processesView *ui.CoreView
	diskView      *ui.CoreView
	networkView   *ui.CoreView
	dockerView    *ui.CoreView
	servicesView  *ui.CoreView
	execView      *ui.CoreView

	sshClient      *SSHClient
	servers        []SSHServer
	selectedServer *SSHServer
}

func NewSSHView(app *tview.Application, pages *tview.Pages) *SSHView {
	sv := &SSHView{
		app:         app,
		pages:       pages,
		viewPages:   tview.NewPages(),
		currentView: viewServers,
	}

	sv.serversView = sv.createServersView()
	sv.overviewView = sv.createOverviewView()
	sv.processesView = sv.createProcessesView()
	sv.diskView = sv.createDiskView()
	sv.networkView = sv.createNetworkView()
	sv.dockerView = sv.createDockerView()
	sv.servicesView = sv.createServicesView()
	sv.execView = sv.createExecView()

	sv.viewPages.AddPage("ssh-servers", sv.serversView.GetLayout(), true, true)
	sv.viewPages.AddPage("ssh-overview", sv.overviewView.GetLayout(), true, false)
	sv.viewPages.AddPage("ssh-processes", sv.processesView.GetLayout(), true, false)
	sv.viewPages.AddPage("ssh-disk", sv.diskView.GetLayout(), true, false)
	sv.viewPages.AddPage("ssh-network", sv.networkView.GetLayout(), true, false)
	sv.viewPages.AddPage("ssh-docker", sv.dockerView.GetLayout(), true, false)
	sv.viewPages.AddPage("ssh-services", sv.servicesView.GetLayout(), true, false)
	sv.viewPages.AddPage("ssh-exec", sv.execView.GetLayout(), true, false)

	return sv
}

func (sv *SSHView) GetMainUI() tview.Primitive { return sv.viewPages }

func (sv *SSHView) Stop() {
	if sv.sshClient != nil {
		sv.sshClient.Disconnect()
	}
	for _, v := range []*ui.CoreView{
		sv.serversView, sv.overviewView, sv.processesView,
		sv.diskView, sv.networkView, sv.dockerView,
		sv.servicesView, sv.execView,
	} {
		v.UnregisterHandlers()
		v.StopAutoRefresh()
	}
}

func (sv *SSHView) refresh() {
	if c := sv.currentCores(); c != nil {
		c.RefreshData()
	}
}

// --- View builders ---

func (sv *SSHView) createServersView() *ui.CoreView {
	core := ui.NewCoreView(sv.app, "Servers")
	core.SetModalPages(sv.pages)
	core.SetTableHeaders([]string{"Name", "Environment", "Host", "Port", "User", "Auth", "Proxy/Jump", "Tags"})
	core.SetSelectionKey("Name")

	core.AddKeyBinding("Enter", "SSH into server", nil)
	core.AddKeyBinding("S", "Quick SSH (shell)", nil)
	core.AddKeyBinding("C", "Connect (background)", nil)
	core.AddKeyBinding("I", "Server Info", nil)
	core.AddKeyBinding("O", "Overview", nil)
	core.AddKeyBinding("P", "Processes", nil)
	core.AddKeyBinding("D", "Disk", nil)
	core.AddKeyBinding("N", "Network", nil)
	core.AddKeyBinding("K", "Docker", nil)
	core.AddKeyBinding("V", "Services", nil)
	core.AddKeyBinding("E", "Execute", nil)
	core.AddKeyBinding("R", "Refresh", nil)
	core.AddKeyBinding("?", "Help", nil)

	core.SetRefreshCallback(func() ([][]string, error) {
		return sv.fetchServers()
	})
	core.SetActionCallback(func(action string, payload map[string]interface{}) error {
		return sv.handleAction(action, payload)
	})

	core.RegisterHandlers()
	core.RefreshData()
	return core
}

func (sv *SSHView) createOverviewView() *ui.CoreView {
	core := ui.NewCoreView(sv.app, "Overview")
	core.SetModalPages(sv.pages)
	core.SetTableHeaders([]string{"Property", "Value"})
	core.SetSelectionKey("Property")

	core.AddKeyBinding("S", "SSH into server", nil)
	core.AddKeyBinding("P", "Processes", nil)
	core.AddKeyBinding("D", "Disk", nil)
	core.AddKeyBinding("N", "Network", nil)
	core.AddKeyBinding("K", "Docker", nil)
	core.AddKeyBinding("V", "Services", nil)
	core.AddKeyBinding("E", "Execute", nil)
	core.AddKeyBinding("R", "Refresh", nil)
	core.AddKeyBinding("?", "Help", nil)

	core.SetRefreshCallback(func() ([][]string, error) {
		return sv.fetchOverview()
	})
	core.SetActionCallback(func(action string, payload map[string]interface{}) error {
		return sv.handleAction(action, payload)
	})

	core.RegisterHandlers()
	return core
}

func (sv *SSHView) createProcessesView() *ui.CoreView {
	core := ui.NewCoreView(sv.app, "Processes")
	core.SetModalPages(sv.pages)
	core.SetTableHeaders([]string{"USER", "PID", "%CPU", "%MEM", "VSZ", "RSS", "TTY", "STAT", "START", "TIME", "COMMAND"})
	core.SetSelectionKey("PID")

	core.AddKeyBinding("S", "SSH into server", nil)
	core.AddKeyBinding("O", "Overview", nil)
	core.AddKeyBinding("R", "Refresh", nil)

	core.SetRefreshCallback(func() ([][]string, error) { return sv.fetchProcesses() })
	core.SetActionCallback(func(action string, payload map[string]interface{}) error {
		return sv.handleAction(action, payload)
	})

	core.RegisterHandlers()
	return core
}

func (sv *SSHView) createDiskView() *ui.CoreView {
	core := ui.NewCoreView(sv.app, "Disk Usage")
	core.SetModalPages(sv.pages)
	core.SetTableHeaders([]string{"Filesystem", "Size", "Used", "Avail", "Use%", "Mounted"})
	core.SetSelectionKey("Filesystem")

	core.AddKeyBinding("S", "SSH into server", nil)
	core.AddKeyBinding("O", "Overview", nil)
	core.AddKeyBinding("R", "Refresh", nil)

	core.SetRefreshCallback(func() ([][]string, error) { return sv.fetchDiskUsage() })
	core.SetActionCallback(func(action string, payload map[string]interface{}) error {
		return sv.handleAction(action, payload)
	})

	core.RegisterHandlers()
	return core
}

func (sv *SSHView) createNetworkView() *ui.CoreView {
	core := ui.NewCoreView(sv.app, "Network")
	core.SetModalPages(sv.pages)
	core.SetTableHeaders([]string{"Proto", "Recv-Q", "Send-Q", "Local", "Remote", "State", "Process"})
	core.SetSelectionKey("Local")

	core.AddKeyBinding("S", "SSH into server", nil)
	core.AddKeyBinding("O", "Overview", nil)
	core.AddKeyBinding("R", "Refresh", nil)

	core.SetRefreshCallback(func() ([][]string, error) { return sv.fetchNetwork() })
	core.SetActionCallback(func(action string, payload map[string]interface{}) error {
		return sv.handleAction(action, payload)
	})

	core.RegisterHandlers()
	return core
}

func (sv *SSHView) createDockerView() *ui.CoreView {
	core := ui.NewCoreView(sv.app, "Docker")
	core.SetModalPages(sv.pages)
	core.SetTableHeaders([]string{"Name", "Image", "Status", "Ports"})
	core.SetSelectionKey("Name")

	core.AddKeyBinding("S", "SSH into server", nil)
	core.AddKeyBinding("O", "Overview", nil)
	core.AddKeyBinding("R", "Refresh", nil)

	core.SetRefreshCallback(func() ([][]string, error) { return sv.fetchDocker() })
	core.SetActionCallback(func(action string, payload map[string]interface{}) error {
		return sv.handleAction(action, payload)
	})

	core.RegisterHandlers()
	return core
}

func (sv *SSHView) createServicesView() *ui.CoreView {
	core := ui.NewCoreView(sv.app, "Services")
	core.SetModalPages(sv.pages)
	core.SetTableHeaders([]string{"Unit", "Load", "Active", "Sub", "Description"})
	core.SetSelectionKey("Unit")

	core.AddKeyBinding("S", "SSH into server", nil)
	core.AddKeyBinding("O", "Overview", nil)
	core.AddKeyBinding("R", "Refresh", nil)

	core.SetRefreshCallback(func() ([][]string, error) { return sv.fetchServices() })
	core.SetActionCallback(func(action string, payload map[string]interface{}) error {
		return sv.handleAction(action, payload)
	})

	core.RegisterHandlers()
	return core
}

func (sv *SSHView) createExecView() *ui.CoreView {
	core := ui.NewCoreView(sv.app, "Execute")
	core.SetModalPages(sv.pages)
	core.SetTableHeaders([]string{"Output"})
	core.SetSelectionKey("Output")

	core.AddKeyBinding("S", "SSH into server", nil)
	core.AddKeyBinding("O", "Overview", nil)
	core.AddKeyBinding("X", "Run Command", nil)
	core.AddKeyBinding("R", "Refresh", nil)

	core.SetRefreshCallback(func() ([][]string, error) {
		return [][]string{{"Type X to execute a command, or S to open a full SSH shell"}}, nil
	})
	core.SetActionCallback(func(action string, payload map[string]interface{}) error {
		return sv.handleAction(action, payload)
	})

	core.RegisterHandlers()
	return core
}

// --- Data fetchers ---

func (sv *SSHView) fetchServers() ([][]string, error) {
	servers, err := DiscoverServers()
	if err != nil {
		return [][]string{{err.Error(), "", "", "", "", "", "", ""}}, nil
	}

	sv.servers = servers
	if len(servers) == 0 {
		return [][]string{{"No SSH entries in KeePass (create under ssh/<env>/<name>)", "", "", "", "", "", "", ""}}, nil
	}

	var rows [][]string
	for _, srv := range servers {
		rows = append(rows, []string{
			srv.Name, srv.Environment, srv.Host,
			fmt.Sprintf("%d", srv.Port), srv.User,
			resolveAuthLabel(srv), resolveProxyLabel(srv),
			strings.Join(srv.Tags, ", "),
		})
	}
	return rows, nil
}

func resolveAuthLabel(srv SSHServer) string {
	if srv.PrivateKey != "" || srv.KeyPath != "" {
		return "key"
	}
	if srv.Password != "" && srv.AuthMethod != "key" {
		return "password"
	}
	return "auto"
}

func resolveProxyLabel(srv SSHServer) string {
	if srv.JumpHost != "" {
		return "jump:" + srv.JumpHost
	}
	if srv.ProxyCommand != "" {
		return "proxy"
	}
	return "-"
}

func (sv *SSHView) fetchOverview() ([][]string, error) {
	if sv.sshClient == nil || !sv.sshClient.IsConnected() {
		return [][]string{{"Not connected", "Press C to background-connect, or S for interactive SSH"}}, nil
	}

	info, err := sv.sshClient.GetHostInfo()
	if err != nil {
		return [][]string{{"Error", err.Error()}}, nil
	}

	rows := [][]string{
		{"Hostname", info.Hostname},
		{"OS", info.OS},
		{"Kernel", info.Kernel},
		{"Uptime", info.Uptime},
		{"CPU Cores", info.CPUCount},
		{"Memory Total", info.MemTotal},
		{"Memory Available", info.MemAvail},
		{"Disk Usage (/)", info.DiskUsage},
		{"Load Average", info.LoadAvg},
		{"IP Addresses", strings.Join(info.IPAddresses, ", ")},
		{"Last Login", info.LastLogin},
		{"Connected To", sv.sshClient.GetServerAddress()},
		{"Connected For", sv.sshClient.GetConnectedDuration().Round(time.Second).String()},
	}

	sv.overviewView.SetInfoText(
		"[aqua::b]Host:[white::b] " + info.Hostname + "\n" +
			"[aqua::b]OS:[white::b] " + info.OS + "\n" +
			"[aqua::b]Load:[white::b] " + info.LoadAvg + "\n" +
			"[aqua::b]Mem:[white::b] " + info.MemAvail + " / " + info.MemTotal + "\n" +
			"[aqua::b]Disk:[white::b] " + info.DiskUsage)

	return rows, nil
}

func (sv *SSHView) fetchProcesses() ([][]string, error) {
	if sv.sshClient == nil || !sv.sshClient.IsConnected() {
		return [][]string{{"Not connected", "", "", "", "", "", "", "", "", "", ""}}, nil
	}
	rows, err := sv.sshClient.GetProcesses()
	if err != nil {
		return nil, err
	}
	if len(rows) > 1 {
		return rows[1:], nil
	}
	return rows, nil
}

func (sv *SSHView) fetchDiskUsage() ([][]string, error) {
	if sv.sshClient == nil || !sv.sshClient.IsConnected() {
		return [][]string{{"Not connected", "", "", "", "", ""}}, nil
	}
	rows, err := sv.sshClient.GetDiskUsage()
	if err != nil {
		return nil, err
	}
	if len(rows) > 1 {
		return rows[1:], nil
	}
	return rows, nil
}

func (sv *SSHView) fetchNetwork() ([][]string, error) {
	if sv.sshClient == nil || !sv.sshClient.IsConnected() {
		return [][]string{{"Not connected", "", "", "", "", "", ""}}, nil
	}
	rows, err := sv.sshClient.GetNetworkConnections()
	if err != nil {
		return nil, err
	}
	if len(rows) > 1 {
		return rows[1:], nil
	}
	return rows, nil
}

func (sv *SSHView) fetchDocker() ([][]string, error) {
	if sv.sshClient == nil || !sv.sshClient.IsConnected() {
		return [][]string{{"Not connected", "", "", ""}}, nil
	}
	rows, err := sv.sshClient.GetDockerContainers()
	if err != nil {
		return [][]string{{"Docker not available or no containers", "", "", ""}}, nil
	}
	if len(rows) > 1 {
		return rows[1:], nil
	}
	return rows, nil
}

func (sv *SSHView) fetchServices() ([][]string, error) {
	if sv.sshClient == nil || !sv.sshClient.IsConnected() {
		return [][]string{{"Not connected", "", "", "", ""}}, nil
	}
	rows, err := sv.sshClient.GetSystemdServices()
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// --- Actions ---

func (sv *SSHView) handleAction(action string, payload map[string]interface{}) error {
	switch action {
	case "refresh":
		sv.refresh()
		return nil
	case "keypress":
		if key, ok := payload["key"].(string); ok {
			if sv.handleGlobalKeys(key) || sv.handleNavKeys(key) || sv.handleViewKeys(key) {
				return nil
			}
		}
	case "rowSelected":
		if sv.currentView == viewServers {
			sv.openInteractiveSSHForSelected()
			return nil
		}
	case "navigate_back":
		if sv.currentView != viewServers {
			sv.showServers()
			return nil
		}
	}
	return fmt.Errorf("unhandled")
}

func (sv *SSHView) handleGlobalKeys(key string) bool {
	switch key {
	case "S":
		sv.openInteractiveSSH()
		return true
	case "?":
		sv.showHelp()
		return true
	case "R":
		sv.refresh()
		return true
	}
	return false
}

func (sv *SSHView) handleNavKeys(key string) bool {
	switch key {
	case "O":
		sv.showOverview()
		return true
	case "P":
		sv.showProcesses()
		return true
	case "D":
		sv.showDisk()
		return true
	case "N":
		sv.showNetwork()
		return true
	case "K":
		sv.showDocker()
		return true
	case "V":
		sv.showServices()
		return true
	case "E":
		sv.showExec()
		return true
	case "L":
		sv.showServers()
		return true
	}
	return false
}

func (sv *SSHView) handleViewKeys(key string) bool {
	switch sv.currentView {
	case viewServers:
		switch key {
		case "Enter":
			sv.openInteractiveSSHForSelected()
			return true
		case "C":
			sv.backgroundConnect()
			return true
		case "I":
			sv.showServerDetails()
			return true
		}
	case viewExec:
		if key == "X" {
			sv.showExecCommandModal()
			return true
		}
	}
	return false
}

// --- Interactive SSH (real terminal session) ---

// openInteractiveSSHForSelected opens a real SSH session for the server
// currently highlighted in the server list.
func (sv *SSHView) openInteractiveSSHForSelected() {
	row := sv.serversView.GetSelectedRow()
	if row < 0 || row >= len(sv.servers) {
		sv.serversView.Log("[red]No server selected")
		return
	}
	srv := sv.servers[row]
	sv.launchSSHSession(srv)
}

// openInteractiveSSH opens a real SSH session for the currently connected
// server, or prompts the user to pick one.
func (sv *SSHView) openInteractiveSSH() {
	if sv.selectedServer != nil {
		sv.launchSSHSession(*sv.selectedServer)
		return
	}

	if sv.currentView == viewServers {
		sv.openInteractiveSSHForSelected()
		return
	}

	sv.ShowConnectionSelector()
}

// launchSSHSession suspends the TUI and runs a real ssh command.
// When the server has a password (and no key auth), it uses SSH_ASKPASS
// to feed the password automatically so the user isn't prompted.
func (sv *SSHView) launchSSHSession(srv SSHServer) {
	args := buildSSHArgs(srv)
	current := sv.currentCores()

	sv.app.Suspend(func() {
		fmt.Print("\033[H\033[2J")
		fmt.Printf("SSH to %s (%s@%s:%d)\n", srv.Name, srv.User, srv.Host, srv.Port)
		if srv.JumpHost != "" {
			fmt.Printf("via jump host: %s\n", srv.JumpHost)
		}
		if srv.ProxyCommand != "" {
			fmt.Printf("via proxy: %s\n", srv.ProxyCommand)
		}
		fmt.Println("Type 'exit' to return to omo.")
		fmt.Println()

		cmd := exec.Command("ssh", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if srv.Password != "" && srv.PrivateKey == "" && srv.KeyPath == "" {
			askpassScript, cleanup := writeAskpassHelper(srv.Password)
			if cleanup != nil {
				defer cleanup()
			}
			if askpassScript != "" {
				cmd.Env = append(os.Environ(),
					"SSH_ASKPASS="+askpassScript,
					"SSH_ASKPASS_REQUIRE=force",
					"DISPLAY=:0",
				)
			}
		}

		if err := cmd.Run(); err != nil {
			fmt.Printf("\nSSH exited: %v\n", err)
			fmt.Println("Press Enter to return to omo...")
			fmt.Scanln()
		}
	})

	if current != nil {
		sv.app.SetFocus(current.GetTable())
	}
	sv.serversView.RefreshData()
}

// writeAskpassHelper creates a temporary script that echoes the password.
// Returns the script path and a cleanup function. The script is chmod 0700
// and deleted on cleanup.
func writeAskpassHelper(password string) (string, func()) {
	f, err := os.CreateTemp("", "omo-askpass-*.sh")
	if err != nil {
		return "", nil
	}
	escaped := strings.ReplaceAll(password, "'", "'\\''")
	script := fmt.Sprintf("#!/bin/sh\necho '%s'\n", escaped)
	if _, err := f.WriteString(script); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil
	}
	f.Close()
	if err := os.Chmod(f.Name(), 0700); err != nil {
		os.Remove(f.Name())
		return "", nil
	}
	return f.Name(), func() { os.Remove(f.Name()) }
}

// buildSSHArgs constructs the ssh command arguments from server config.
func buildSSHArgs(srv SSHServer) []string {
	var args []string

	if srv.Port != 0 && srv.Port != 22 {
		args = append(args, "-p", fmt.Sprintf("%d", srv.Port))
	}

	if srv.KeyPath != "" {
		args = append(args, "-i", srv.KeyPath)
	}

	if srv.JumpHost != "" {
		args = append(args, "-J", srv.JumpHost)
	}

	if srv.ProxyCommand != "" {
		args = append(args, "-o", fmt.Sprintf("ProxyCommand=%s", srv.ProxyCommand))
	}

	if srv.KeepAlive > 0 {
		args = append(args, "-o", fmt.Sprintf("ServerAliveInterval=%d", srv.KeepAlive))
		args = append(args, "-o", "ServerAliveCountMax=3")
	}

	args = append(args, "-o", "StrictHostKeyChecking=no")

	if srv.Password != "" && srv.PrivateKey == "" && srv.KeyPath == "" {
		args = append(args, "-o", "PreferredAuthentications=password")
		args = append(args, "-o", "PubkeyAuthentication=no")
	}

	target := srv.Host
	if srv.User != "" {
		target = srv.User + "@" + srv.Host
	}
	args = append(args, target)

	if srv.StartupCmd != "" {
		args = append(args, "-t", srv.StartupCmd)
	}

	return args
}

// --- Background connect (for overview/processes/etc views) ---

func (sv *SSHView) backgroundConnect() {
	row := sv.serversView.GetSelectedRow()
	if row < 0 || row >= len(sv.servers) {
		sv.serversView.Log("[red]No server selected")
		return
	}

	srv := sv.servers[row]
	sv.selectedServer = &srv

	if sv.sshClient != nil {
		sv.sshClient.Disconnect()
	}

	sv.serversView.Log(fmt.Sprintf("[yellow]Connecting to %s@%s:%d...", srv.User, srv.Host, srv.Port))

	go func() {
		client := NewSSHClient(srv)
		if err := client.Connect(); err != nil {
			sv.app.QueueUpdateDraw(func() {
				sv.serversView.Log(fmt.Sprintf("[red]Failed: %v", err))
			})
			return
		}

		sv.sshClient = client
		sv.app.QueueUpdateDraw(func() {
			sv.serversView.Log(fmt.Sprintf("[green]Connected to %s (use O for overview, S for shell)", srv.Name))
			sv.serversView.RefreshData()
		})
	}()
}

func (sv *SSHView) showServerDetails() {
	row := sv.serversView.GetSelectedRow()
	if row < 0 || row >= len(sv.servers) {
		return
	}

	srv := sv.servers[row]
	details := fmt.Sprintf(
		"[yellow]Name:[white] %s\n"+
			"[yellow]Environment:[white] %s\n"+
			"[yellow]Host:[white] %s\n"+
			"[yellow]Port:[white] %d\n"+
			"[yellow]User:[white] %s\n"+
			"[yellow]Auth Method:[white] %s\n"+
			"[yellow]Key Path:[white] %s\n"+
			"[yellow]Proxy Command:[white] %s\n"+
			"[yellow]Jump Host:[white] %s\n"+
			"[yellow]Fingerprint:[white] %s\n"+
			"[yellow]Startup Cmd:[white] %s\n"+
			"[yellow]Keep Alive:[white] %ds\n"+
			"[yellow]Tags:[white] %s\n"+
			"[yellow]Description:[white] %s",
		srv.Name, srv.Environment, srv.Host, srv.Port,
		srv.User, srv.AuthMethod,
		dash(srv.KeyPath), dash(srv.ProxyCommand),
		dash(srv.JumpHost), dash(srv.Fingerprint),
		dash(srv.StartupCmd), srv.KeepAlive,
		dash(strings.Join(srv.Tags, ", ")),
		dash(srv.Description),
	)

	ui.ShowInfoModal(sv.pages, sv.app, "Server Details", details, func() {
		sv.app.SetFocus(sv.serversView.GetTable())
	})
}

func (sv *SSHView) showExecCommandModal() {
	if sv.sshClient == nil || !sv.sshClient.IsConnected() {
		sv.execView.Log("[red]Not connected. Press C on the server list to background-connect first.")
		return
	}

	ui.ShowCompactStyledInputModal(sv.pages, sv.app, "Execute Command", "Command: ", "", 60, nil, func(command string, cancelled bool) {
		if cancelled || command == "" {
			sv.app.SetFocus(sv.execView.GetTable())
			return
		}

		sv.execView.Log(fmt.Sprintf("[yellow]Running: %s", command))

		go func() {
			result, err := sv.sshClient.Execute(command)
			sv.app.QueueUpdateDraw(func() {
				if err != nil {
					sv.execView.Log(fmt.Sprintf("[red]Error: %v", err))
					return
				}

				var rows [][]string
				output := result.Stdout
				if result.Stderr != "" {
					output += "\n[stderr]\n" + result.Stderr
				}
				for _, line := range strings.Split(output, "\n") {
					if line != "" {
						rows = append(rows, []string{line})
					}
				}
				if len(rows) == 0 {
					rows = [][]string{{"(no output)"}}
				}

				sv.execView.SetRefreshCallback(func() ([][]string, error) { return rows, nil })
				sv.execView.RefreshData()
				sv.execView.Log(fmt.Sprintf("[green]Done in %s (exit %d)", result.Duration.Round(time.Millisecond), result.ExitCode))
			})
		}()

		sv.app.SetFocus(sv.execView.GetTable())
	})
}

func (sv *SSHView) showHelp() {
	helpText := `[yellow]SSH Plugin[white]

[green]Server List:[white]
Enter   Open interactive SSH session (real terminal)
S       Quick SSH to selected/connected server
C       Background connect (for overview data)
I       Server details (from KeePass)
R       Refresh

[green]After Background Connect:[white]
O       Overview (host info, OS, memory)
P       Processes (top by CPU)
D       Disk usage
N       Network connections
K       Docker containers
V       Systemd services
E       Execute command (non-interactive)

[green]Execute View:[white]
X       Run a command

[green]Available Everywhere:[white]
S       Open interactive SSH shell
/       Filter
Ctrl+T  Switch server
Esc     Back
?       This help

[green]All connection data is in KeePass:[white]
[green]  ssh/<environment>/<server-name>[white]`

	ui.ShowInfoModal(sv.pages, sv.app, "Help", helpText, func() {
		if c := sv.currentCores(); c != nil {
			sv.app.SetFocus(c.GetTable())
		}
	})
}

func (sv *SSHView) ShowConnectionSelector() {
	servers, err := DiscoverServers()
	if err != nil || len(servers) == 0 {
		sv.serversView.Log("[yellow]No SSH servers found in KeePass")
		return
	}

	var items [][]string
	for _, srv := range servers {
		items = append(items, []string{
			srv.Name,
			fmt.Sprintf("%s@%s:%d [%s]", srv.User, srv.Host, srv.Port, srv.Environment),
		})
	}

	ui.ShowStandardListSelectorModal(sv.pages, sv.app, "Select Server", items, func(index int, name string, cancelled bool) {
		if cancelled || index < 0 || index >= len(servers) {
			if c := sv.currentCores(); c != nil {
				sv.app.SetFocus(c.GetTable())
			}
			return
		}

		srv := servers[index]
		sv.launchSSHSession(srv)
	})
}

func (sv *SSHView) AutoConnectToDefaultInstance() {
	servers, err := DiscoverServers()
	if err != nil || len(servers) == 0 {
		return
	}
	sv.servers = servers
	sv.app.QueueUpdateDraw(func() {
		sv.serversView.RefreshData()
	})
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
