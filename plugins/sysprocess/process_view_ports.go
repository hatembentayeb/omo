package main

import (
	"fmt"
	"strconv"

	"omo/pkg/ui"

	"github.com/shirou/gopsutil/v3/process"
)

// newPortsView creates the listening ports CoreView
func (pv *ProcessView) newPortsView() *ui.CoreView {
	cv := ui.NewCoreView(pv.app, "Listening Ports")
	cv.SetTableHeaders([]string{"PID", "Name", "User", "Address", "Source"})
	cv.SetRefreshCallback(pv.fetchPortsList)
	cv.SetRowSelectedCallback(pv.onPortSelected)
	cv.SetActionCallback(pv.handleAction)

	cv.AddKeyBinding("P", "Processes", nil)
	cv.AddKeyBinding("S", "Metrics", nil)
	cv.AddKeyBinding("D", "Disk", nil)
	cv.AddKeyBinding("W", "Why Running?", nil)
	cv.AddKeyBinding("K", "Kill Process", nil)
	cv.AddKeyBinding("J", "Jump to Process", nil)
	cv.AddKeyBinding("G", "Warnings", nil)

	cv.RegisterHandlers()

	return cv
}

// getSelectedPortProcess returns the process for the currently selected port row.
// Builds UserProcess on the fly for PIDs not in process list (e.g. Docker, root).
func (pv *ProcessView) getSelectedPortProcess() (*UserProcess, bool) {
	row := pv.portsView.GetSelectedRowData()
	if len(row) < 4 {
		return nil, false
	}

	pid, err := strconv.ParseInt(row[0], 10, 32)
	if err != nil {
		return nil, false
	}
	pid32 := int32(pid)

	pv.mu.Lock()
	for _, p := range pv.processes {
		if p.PID == pid32 {
			pv.mu.Unlock()
			return p, true
		}
	}
	pv.mu.Unlock()

	// Not in our list (e.g. Docker, root) — build on the fly
	proc, err := process.NewProcess(pid32)
	if err != nil {
		return nil, false
	}
	// Merge port cache so we have fresh ports
	ports := getAllListeningPorts()
	up := buildUserProcess(proc, pv.cpuCache, ports)
	return up, true
}

// onPortSelected updates info panel when selecting a port row
func (pv *ProcessView) onPortSelected(row int) {
	proc, ok := pv.getSelectedPortProcess()
	if !ok {
		return
	}
	rowData := pv.portsView.GetSelectedRowData()
	addr := ""
	if len(rowData) >= 4 {
		addr = rowData[3]
	}
	pv.portsView.Log(fmt.Sprintf("[blue]Selected: %s (PID %d) — %s", proc.Name, proc.PID, proc.GetPortsString()))
	pv.portsView.SetInfoMap(map[string]string{
		"PID":     fmt.Sprintf("%d", proc.PID),
		"Name":    proc.Name,
		"Address": addr,
		"Source":  proc.Source,
	})
}

// killProcessOnPort kills the process holding the selected port
func (pv *ProcessView) killProcessOnPort() {
	proc, ok := pv.getSelectedPortProcess()
	if !ok {
		pv.portsView.Log("[yellow]No port selected")
		return
	}

	confirmMsg := fmt.Sprintf(
		"[yellow]Kill process holding this port?[white]\n\n"+
			"[aqua]Process:[white] %s\n"+
			"[aqua]PID:[white] %d\n"+
			"[aqua]Ports:[white] %s\n\n"+
			"This sends SIGKILL. Connections will be closed.",
		proc.Name, proc.PID, proc.GetPortsString(),
	)

	ui.ShowStandardConfirmationModal(
		pv.pages, pv.app,
		"Kill Process",
		confirmMsg,
		func(confirmed bool) {
			if confirmed {
				if err := pv.killProcessByPID(proc.PID); err != nil {
					ui.ShowStandardErrorModal(pv.pages, pv.app, "Error",
						fmt.Sprintf("Failed to kill: %v", err),
						func() { pv.app.SetFocus(pv.portsView.GetTable()) },
					)
					return
				}
				pv.portsView.Log(fmt.Sprintf("[yellow]Killed %s (PID %d)", proc.Name, proc.PID))
				go pv.loadProcessData()
			}
			pv.app.SetFocus(pv.portsView.GetTable())
		},
	)
}

// showDetailsForPort shows Why Running? for the process holding the selected port
func (pv *ProcessView) showDetailsForPort() {
	proc, ok := pv.getSelectedPortProcess()
	if !ok {
		pv.portsView.Log("[yellow]No port selected")
		return
	}
	pv.detailsProcess = proc
	pv.enrichSelectedProcess(proc)
	pv.switchToView(viewDetails)
}

// jumpToProcessForPort switches to process list and selects the process holding the port
func (pv *ProcessView) jumpToProcessForPort() {
	proc, ok := pv.getSelectedPortProcess()
	if !ok {
		pv.portsView.Log("[yellow]No port selected")
		return
	}

	pv.mu.Lock()
	var idx int = -1
	for i, p := range pv.processes {
		if p.PID == proc.PID {
			idx = i
			break
		}
	}
	pv.mu.Unlock()

	if idx < 0 {
		pv.portsView.Log("[yellow]Process runs as different user — not in your process list")
		return
	}

	pv.switchToView(viewProcesses)
	// Select the row in the process list (row 0 = header, so idx+1)
	pv.processListView.GetTable().Select(idx+1, 0)
	// Update info panel (programmatic Select may not trigger callback)
	infoMap := map[string]string{
		"PID":     fmt.Sprintf("%d", proc.PID),
		"Name":    proc.Name,
		"User":    proc.Username,
		"Source":  proc.Source,
		"Command": truncateString(proc.Cmdline, 40),
	}
	if len(proc.Ports) > 0 {
		infoMap["Ports"] = proc.GetPortsString()
	}
	pv.processListView.SetInfoMap(infoMap)
	pv.processListView.Log(fmt.Sprintf("[blue]Jumped to %s (PID %d)", proc.Name, proc.PID))
}

// fetchPortsList returns rows for ALL listening ports on the system (pure Go, gopsutil)
// Includes Docker, root processes, and any user-space process — not just current user
func (pv *ProcessView) fetchPortsList() ([][]string, error) {
	pv.portCache = getAllListeningPorts()

	pv.mu.Lock()
	processesByPID := make(map[int32]*UserProcess)
	for _, p := range pv.processes {
		processesByPID[p.PID] = p
	}
	pv.mu.Unlock()

	var data [][]string
	for pid, addrs := range pv.portCache {
		// Pid 0 = unknown (gopsutil couldn't resolve — run with sudo to see all)
		if pid == 0 {
			for _, addr := range addrs {
				bindWarning := ""
				if len(addr) > 0 && addr[0] == '*' {
					bindWarning = " [red](public)[white]"
				}
				data = append(data, []string{
					"?",
					"? [gray](run sudo)[white]",
					"?",
					addr + bindWarning,
					"-",
				})
			}
			continue
		}

		proc, err := process.NewProcess(pid)
		if err != nil {
			for _, addr := range addrs {
				data = append(data, []string{
					fmt.Sprintf("%d", pid),
					"?",
					"?",
					addr,
					"-",
				})
			}
			continue
		}

		// Skip kernel threads
		if !isUserProcess(proc) {
			continue
		}

		name, _ := proc.Name()
		username, _ := proc.Username()
		source := "?"
		if p, ok := processesByPID[pid]; ok {
			source = p.Source
		} else {
			source = detectSource(getProcessAncestry(proc))
		}

		for _, addr := range addrs {
			bindWarning := ""
			if len(addr) > 0 && addr[0] == '*' {
				bindWarning = " [red](public)[white]"
			}

			data = append(data, []string{
				fmt.Sprintf("%d", pid),
				name,
				username,
				addr + bindWarning,
				source,
			})
		}
	}

	if len(data) == 0 {
		data = append(data, []string{"", "", "", "[yellow]No listening ports found", ""})
	}

	return data, nil
}
