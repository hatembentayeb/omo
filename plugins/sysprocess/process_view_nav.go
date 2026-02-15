package main

import (
	"os"

	"omo/pkg/ui"
)

// View name constants
const (
	viewRoot      = "sysprocess"
	viewProcesses = "processes"
	viewDetails   = "details"
	viewPorts     = "ports"
	viewWarnings  = "warnings"
	viewMetrics   = "metrics"
	viewDisk      = "disk"
)

// currentCores returns the CoreView for the currently active view
func (pv *ProcessView) currentCores() *ui.CoreView {
	switch pv.currentViewName {
	case viewDetails:
		return pv.detailsView
	case viewPorts:
		return pv.portsView
	case viewWarnings:
		return pv.warningsView
	case viewMetrics:
		return pv.metricsView
	case viewDisk:
		return pv.diskView
	default:
		return pv.processListView
	}
}

// setViewStack sets the navigation breadcrumb stack for a view
func (pv *ProcessView) setViewStack(cores *ui.CoreView, viewName string) {
	if cores == nil {
		return
	}
	stack := []string{viewRoot, viewProcesses}
	if viewName != viewProcesses {
		stack = append(stack, viewName)
	}
	cores.SetViewStack(stack)
}

// switchToView switches the active page and refreshes the view
func (pv *ProcessView) switchToView(viewName string) {
	pageName := "process-" + viewName
	pv.currentViewName = viewName
	pv.viewPages.SwitchToPage(pageName)

	pv.setViewStack(pv.currentCores(), viewName)
	pv.refresh()

	current := pv.currentCores()
	if current != nil {
		pv.app.SetFocus(current.GetTable())
	}
}

// showProcesses switches to the main process list view
func (pv *ProcessView) showProcesses() {
	pv.switchToView(viewProcesses)
}

// showDetails switches to the witr-style details view for the selected process
func (pv *ProcessView) showDetails() {
	proc, ok := pv.getSelectedProcess()
	if !ok {
		pv.processListView.Log("[yellow]No process selected — use arrow keys to select one")
		return
	}

	// Store for details view (like Kafka's selectedTopic when navigating to partitions)
	pv.detailsProcess = proc
	pv.enrichSelectedProcess(proc)
	pv.switchToView(viewDetails)
}

// showPorts switches to the listening ports view
func (pv *ProcessView) showPorts() {
	pv.switchToView(viewPorts)
}

// showWarnings switches to the warnings view
func (pv *ProcessView) showWarnings() {
	pv.switchToView(viewWarnings)
}

// showMetrics switches to the system metrics view
func (pv *ProcessView) showMetrics() {
	pv.switchToView(viewMetrics)
}

// showDisk switches to the disk usage (ncdu-like) view
func (pv *ProcessView) showDisk() {
	// Initialize with user home if not yet scanned
	path := "/"
	pv.diskMu.Lock()
	if pv.diskPath == "" {
		if home, err := os.UserHomeDir(); err == nil {
			pv.diskPath = home
			path = home
		} else {
			pv.diskPath = "/"
		}
	} else {
		path = pv.diskPath
	}
	pv.diskMu.Unlock()

	pv.startDiskScan(path) // Start before switch so refresh shows "Scanning..."
	pv.switchToView(viewDisk)
}
