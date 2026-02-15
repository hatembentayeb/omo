package main

import (
	"fmt"
	"strings"

	"omo/pkg/ui"
)

// newWarningsView creates the warnings CoreView
func (pv *ProcessView) newWarningsView() *ui.CoreView {
	cv := ui.NewCoreView(pv.app, "Process Warnings")
	cv.SetTableHeaders([]string{"PID", "Name", "Warning", "Details"})
	cv.SetRefreshCallback(pv.fetchWarningsList)
	cv.SetActionCallback(pv.handleAction)

	cv.AddKeyBinding("P", "Processes", nil)
	cv.AddKeyBinding("S", "Metrics", nil)
	cv.AddKeyBinding("D", "Disk", nil)
	cv.AddKeyBinding("W", "Why Running?", nil)
	cv.AddKeyBinding("L", "Ports", nil)

	cv.RegisterHandlers()

	return cv
}

// fetchWarningsList returns rows for all user processes that have warnings
func (pv *ProcessView) fetchWarningsList() ([][]string, error) {
	pv.mu.Lock()
	processes := pv.processes
	pv.mu.Unlock()

	var data [][]string
	for _, p := range processes {
		if len(p.Warnings) == 0 {
			continue
		}

		for _, w := range p.Warnings {
			// Color code by severity
			color := "[yellow]"
			if strings.Contains(w, "root") || strings.Contains(w, "Public") {
				color = "[red]"
			}

			data = append(data, []string{
				fmt.Sprintf("%d", p.PID),
				p.Name,
				color + w + "[white]",
				truncateString(p.Cmdline, 40),
			})
		}
	}

	if len(data) == 0 {
		data = append(data, []string{"", "", "[green]No warnings — all processes look healthy", ""})
	}

	return data, nil
}
