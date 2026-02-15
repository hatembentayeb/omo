package main

import (
	"fmt"
	"sort"
	"time"

	"omo/pkg/ui"
)

// newProcessListView creates the main process list CoreView
func (pv *ProcessView) newProcessListView() *ui.CoreView {
	cv := ui.NewCoreView(pv.app, "User Processes")
	cv.SetTableHeaders([]string{"PID", "Name", "User", "CPU%", "Mem%", "Source", "Status", "Started"})
	cv.SetRefreshCallback(pv.fetchProcessList)
	cv.SetRowSelectedCallback(pv.onProcessSelected)
	cv.SetActionCallback(pv.handleAction)

	cv.AddKeyBinding("W", "Why Running?", nil)
	cv.AddKeyBinding("K", "Kill", nil)
	cv.AddKeyBinding("L", "Ports", nil)
	cv.AddKeyBinding("G", "Warnings", nil)
	cv.AddKeyBinding("S", "Metrics", nil)
	cv.AddKeyBinding("D", "Disk", nil)
	cv.AddKeyBinding("T", "Sort CPU", nil)
	cv.AddKeyBinding("M", "Sort Mem", nil)

	cv.RegisterHandlers()

	return cv
}

// fetchProcessList returns table rows from the current process data
func (pv *ProcessView) fetchProcessList() ([][]string, error) {
	pv.mu.Lock()
	defer pv.mu.Unlock()

	data := make([][]string, 0, len(pv.processes))
	for _, p := range pv.processes {
		data = append(data, p.GetTableRow())
	}

	return data, nil
}

// onProcessSelected handles row selection — updates info panel (Docker/Kafka pattern: selection comes from GetSelectedRow)
func (pv *ProcessView) onProcessSelected(row int) {
	proc, ok := pv.getSelectedProcess()
	if !ok {
		return
	}

	pv.processListView.Log(fmt.Sprintf("[blue]Selected: %s (PID %d)", proc.Name, proc.PID))

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
}

// killSelectedProcess sends SIGKILL to the selected process after confirmation
func (pv *ProcessView) killSelectedProcess() {
	proc, ok := pv.getSelectedProcess()
	if !ok {
		pv.processListView.Log("[yellow]No process selected — use arrow keys to select one")
		return
	}

	confirmMsg := fmt.Sprintf(
		"[yellow]Kill this process?[white]\n\n"+
			"[aqua]Process:[white] %s\n"+
			"[aqua]PID:[white] %d\n"+
			"[aqua]User:[white] %s\n"+
			"[aqua]Command:[white] %s\n\n"+
			"This sends SIGKILL (signal 9).\nUnsaved data will be lost.",
		proc.Name, proc.PID, proc.Username, truncateString(proc.Cmdline, 50),
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
						func() { pv.app.SetFocus(pv.processListView.GetTable()) },
					)
					return
				}
				pv.processListView.Log(fmt.Sprintf("[yellow]Killed %s (PID %d)", proc.Name, proc.PID))
				go func() {
					time.Sleep(500 * time.Millisecond)
					pv.loadProcessData()
				}()
			} else {
				pv.processListView.Log("[blue]Kill cancelled")
			}
			pv.app.SetFocus(pv.processListView.GetTable())
		},
	)
}

// sortByCPU sorts the process list by CPU usage descending
func (pv *ProcessView) sortByCPU() {
	pv.mu.Lock()
	sort.Slice(pv.processes, func(i, j int) bool {
		return pv.processes[i].CPUPercent > pv.processes[j].CPUPercent
	})
	pv.mu.Unlock()

	pv.processListView.Log("[blue]Sorted by CPU usage")
	pv.processListView.RefreshData()
}

// sortByMemory sorts the process list by memory usage descending
func (pv *ProcessView) sortByMemory() {
	pv.mu.Lock()
	sort.Slice(pv.processes, func(i, j int) bool {
		return pv.processes[i].MemPercent > pv.processes[j].MemPercent
	})
	pv.mu.Unlock()

	pv.processListView.Log("[blue]Sorted by memory usage")
	pv.processListView.RefreshData()
}
