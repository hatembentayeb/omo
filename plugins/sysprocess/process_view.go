package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"omo/pkg/ui"

	"github.com/rivo/tview"
	"github.com/shirou/gopsutil/v3/process"
)

// ProcessView manages the UI for monitoring user-space processes
type ProcessView struct {
	app             *tview.Application
	pages           *tview.Pages
	viewPages       *tview.Pages
	processListView *ui.CoreView
	detailsView     *ui.CoreView
	portsView       *ui.CoreView
	warningsView    *ui.CoreView
	metricsView     *ui.CoreView
	diskView        *ui.CoreView
	currentViewName string
	currentUser     string

	// Data
	mu        sync.Mutex
	processes []*UserProcess

	// detailsProcess holds the process when navigating to details view (like Kafka's selectedTopic)
	detailsProcess *UserProcess

	cpuCache  map[int32]float64
	portCache map[int32][]string

	// Disk view (ncdu-like)
	diskMu       sync.Mutex
	diskPath     string
	diskEntries  []diskEntry
	diskScanning bool
}

// NewProcessView creates a new ProcessView with all sub-views
func NewProcessView(app *tview.Application, pages *tview.Pages) *ProcessView {
	pv := &ProcessView{
		app:         app,
		pages:       pages,
		viewPages:   tview.NewPages(),
		currentUser: getCurrentUsername(),
		cpuCache:    make(map[int32]float64),
		portCache:   make(map[int32][]string),
	}

	// Create all views
	pv.processListView = pv.newProcessListView()
	pv.detailsView = pv.newDetailsView()
	pv.portsView = pv.newPortsView()
	pv.warningsView = pv.newWarningsView()
	pv.metricsView = pv.newMetricsView()
	pv.diskView = pv.newDiskView()

	// Set modal pages for all views
	views := []*ui.CoreView{
		pv.processListView,
		pv.detailsView,
		pv.portsView,
		pv.warningsView,
		pv.metricsView,
		pv.diskView,
	}
	for _, view := range views {
		if view != nil {
			view.SetModalPages(pv.pages)
		}
	}

	// Add view pages
	pv.viewPages.AddPage("process-processes", pv.processListView.GetLayout(), true, true)
	pv.viewPages.AddPage("process-details", pv.detailsView.GetLayout(), true, false)
	pv.viewPages.AddPage("process-ports", pv.portsView.GetLayout(), true, false)
	pv.viewPages.AddPage("process-warnings", pv.warningsView.GetLayout(), true, false)
	pv.viewPages.AddPage("process-metrics", pv.metricsView.GetLayout(), true, false)
	pv.viewPages.AddPage("process-disk", pv.diskView.GetLayout(), true, false)

	// Set current view and navigation stacks
	pv.currentViewName = viewProcesses
	pv.setViewStack(pv.processListView, viewProcesses)
	pv.setViewStack(pv.detailsView, viewDetails)
	pv.setViewStack(pv.portsView, viewPorts)
	pv.setViewStack(pv.warningsView, viewWarnings)
	pv.setViewStack(pv.metricsView, viewMetrics)
	pv.setViewStack(pv.diskView, viewDisk)

	// Initial status
	pv.processListView.SetInfoText(fmt.Sprintf(
		"[yellow]Process Monitor[white]\nUser: %s\nStatus: Loading...",
		pv.currentUser,
	))

	return pv
}

// GetMainUI returns the main UI component to be embedded in the application
func (pv *ProcessView) GetMainUI() tview.Primitive {
	return pv.viewPages
}

// Stop cleans up resources when the view is no longer used
func (pv *ProcessView) Stop() {
	views := []*ui.CoreView{
		pv.processListView,
		pv.detailsView,
		pv.portsView,
		pv.warningsView,
		pv.metricsView,
		pv.diskView,
	}
	for _, view := range views {
		if view != nil {
			view.StopAutoRefresh()
			view.UnregisterHandlers()
		}
	}
}

// refresh triggers a data refresh on the current view
func (pv *ProcessView) refresh() {
	current := pv.currentCores()
	if current != nil {
		current.RefreshData()
	}
}

// loadProcessData fetches all user-space processes from the OS and enriches them
func (pv *ProcessView) loadProcessData() {
	current := pv.currentCores()
	if current != nil {
		current.Log("[yellow]Loading user processes...")
	}

	allProcs, err := process.Processes()
	if err != nil {
		if current != nil {
			current.Log(fmt.Sprintf("[red]Failed to load processes: %v", err))
		}
		return
	}

	// Filter to current user's user-space processes
	var userProcs []*process.Process
	for _, proc := range allProcs {
		if isCurrentUserProcess(proc, pv.currentUser) {
			userProcs = append(userProcs, proc)
		}
	}

	// Update CPU cache
	for _, proc := range userProcs {
		percent, err := proc.CPUPercent()
		if err == nil {
			pv.cpuCache[proc.Pid] = percent
		}
	}

	// Batch-fetch listening ports
	pv.portCache = getAllListeningPorts()

	// Build enriched process list
	var processes []*UserProcess
	for _, proc := range userProcs {
		up := buildUserProcess(proc, pv.cpuCache, pv.portCache)
		if up != nil {
			processes = append(processes, up)
		}
	}

	// Sort by CPU descending by default
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].CPUPercent > processes[j].CPUPercent
	})

	pv.mu.Lock()
	pv.processes = processes
	pv.mu.Unlock()

	// Update UI on main goroutine
	pv.app.QueueUpdateDraw(func() {
		pv.processListView.RefreshData()
		pv.processListView.SetInfoText(fmt.Sprintf(
			"[yellow]Process Monitor[white]\nUser: %s\nProcesses: %d\nUpdated: %s",
			pv.currentUser,
			len(processes),
			time.Now().Format("15:04:05"),
		))
		pv.processListView.Log(fmt.Sprintf("[green]Loaded %d user processes", len(processes)))
	})
}

// buildUserProcess creates a UserProcess from a gopsutil Process with ancestry and source
func buildUserProcess(proc *process.Process, cpuCache map[int32]float64, portCache map[int32][]string) *UserProcess {
	name, _ := proc.Name()
	username, _ := proc.Username()
	cmdline, _ := proc.Cmdline()
	cwd, _ := proc.Cwd()
	status, _ := proc.Status()
	statusStr := strings.Join(status, ", ")
	memPercent, _ := proc.MemoryPercent()
	memInfo, _ := proc.MemoryInfo()
	threads, _ := proc.NumThreads()
	createTime, _ := proc.CreateTime()
	ppid, _ := proc.Ppid()

	cpuPercent := cpuCache[proc.Pid]

	var memRSS, memVMS uint64
	if memInfo != nil {
		memRSS = memInfo.RSS
		memVMS = memInfo.VMS
	}

	// Build ancestry chain
	ancestry := getProcessAncestry(proc)

	// Detect what started this process
	source := detectSource(ancestry)

	// Get listening ports from cache
	ports := portCache[proc.Pid]

	up := &UserProcess{
		PID:        proc.Pid,
		Name:       name,
		Username:   username,
		Cmdline:    cmdline,
		Cwd:        cwd,
		Status:     statusStr,
		CPUPercent: cpuPercent,
		MemPercent: float64(memPercent),
		MemRSS:     memRSS,
		MemVMS:     memVMS,
		Threads:    threads,
		CreateTime: createTime,
		PPID:       ppid,
		Source:     source,
		Ancestry:   ancestry,
		Ports:      ports,
	}

	// Compute warnings
	up.Warnings = getProcessWarnings(up)

	return up
}

// killProcessByPID sends SIGKILL to the given PID (pure Go, no exec)
func (pv *ProcessView) killProcessByPID(pid int32) error {
	return syscall.Kill(int(pid), syscall.SIGKILL)
}

// getSelectedProcess returns the process at the currently selected row (Docker/Kafka pattern)
func (pv *ProcessView) getSelectedProcess() (*UserProcess, bool) {
	pv.mu.Lock()
	procs := pv.processes
	pv.mu.Unlock()

	row := pv.processListView.GetSelectedRow()
	if row < 0 || row >= len(procs) {
		return nil, false
	}
	return procs[row], true
}

// enrichSelectedProcess adds git repo info and refreshes ports/warnings for the given process
func (pv *ProcessView) enrichSelectedProcess(p *UserProcess) {
	if p == nil {
		return
	}

	// Refresh port cache
	pv.portCache = getAllListeningPorts()
	p.Ports = pv.portCache[p.PID]

	// Discover git repo from working directory
	if p.Cwd != "" {
		p.GitRepo, p.GitBranch = findGitRepo(p.Cwd)
	}

	// Refresh warnings with updated data
	p.Warnings = getProcessWarnings(p)
}

// handleAction dispatches key presses and actions across all views
func (pv *ProcessView) handleAction(action string, payload map[string]interface{}) error {
	switch action {
	case "refresh":
		pv.refresh()
		return nil
	case "rowSelected":
		if pv.currentViewName == viewDisk {
			if idx, ok := payload["rowIndex"].(int); ok {
				pv.onDiskRowActivated(idx)
			}
			return nil
		}
	case "keypress":
		if key, ok := payload["key"].(string); ok {
			if pv.handleProcessKeys(key) {
				return nil
			}
		}
	case "navigate_back":
		pv.switchToView(viewProcesses)
		return nil
	}

	return fmt.Errorf("unhandled")
}

func (pv *ProcessView) handleProcessKeys(key string) bool {
	if pv.currentViewName == viewProcesses {
		switch key {
		case "K":
			pv.killSelectedProcess()
			return true
		case "T":
			pv.sortByCPU()
			return true
		case "M":
			pv.sortByMemory()
			return true
		}
	}
	if pv.currentViewName == viewPorts {
		switch key {
		case "K":
			pv.killProcessOnPort()
			return true
		case "W":
			pv.showDetailsForPort()
			return true
		case "J":
			pv.jumpToProcessForPort()
			return true
		}
	}

	switch key {
	case "W":
		if pv.currentViewName != viewPorts {
			pv.showDetails()
		}
		return true
	case "L":
		pv.showPorts()
		return true
	case "G":
		pv.showWarnings()
		return true
	case "S":
		pv.showMetrics()
		return true
	case "D":
		pv.showDisk()
		return true
	case "U":
		if pv.currentViewName == viewDisk {
			pv.diskGoUp()
		}
		return true
	case "P":
		pv.showProcesses()
		return true
	case "?":
		pv.showHelp()
		return true
	case "R":
		go pv.loadProcessData()
		return true
	}
	return false
}

// showHelp displays the plugin help modal
func (pv *ProcessView) showHelp() {
	helpText := `
[yellow]Process Monitor Help[white]

[green]Navigation Views:[white]
P       - Process list (main view)
W       - Why Running? (witr-style details)
L       - Listening Ports view
G       - Warnings view
S       - System Metrics (CPU, memory, disk)
D       - Disk Usage (ncdu-like, biggest folders)
Ctrl+R  - Reload all process data

[green]Process Actions (P view):[white]
K       - Kill selected process
T       - Sort by CPU usage
M       - Sort by Memory usage
Enter   - Select process

[green]Port Actions (L view):[white]
Shows ALL listening ports (Docker, root, your processes)
Run with [yellow]sudo[white] to see PID/process for root-owned ports
K       - Kill process holding selected port
W       - Why Running? for that process
J       - Jump to process in list (if in your list)

[green]Disk Usage (D view):[white]
Enter   - Open folder (drill down)
U       - Go up to parent
Largest folders first (ncdu-style)

[green]General:[white]
?       - Show this help
R       - Refresh / Reload data
/       - Filter table
Esc     - Navigate back

[green]Inspired by:[white]
github.com/pranshuparmar/witr
"Why is this running?"
`

	ui.ShowInfoModal(
		pv.pages,
		pv.app,
		"Process Monitor Help",
		helpText,
		func() {
			current := pv.currentCores()
			if current != nil {
				pv.app.SetFocus(current.GetTable())
			}
		},
	)
}
