package main

import (
	"fmt"
	"strings"
	"time"

	"omo/pkg/ui"
)

// newDetailsView creates the witr-style "Why Is This Running?" CoreView
func (pv *ProcessView) newDetailsView() *ui.CoreView {
	cv := ui.NewCoreView(pv.app, "Why Is This Running?")
	cv.SetTableHeaders([]string{"Property", "Value"})
	cv.SetRefreshCallback(pv.fetchProcessDetails)
	cv.SetActionCallback(pv.handleAction)

	cv.AddKeyBinding("P", "Processes", nil)
	cv.AddKeyBinding("S", "Metrics", nil)
	cv.AddKeyBinding("D", "Disk", nil)
	cv.AddKeyBinding("L", "Ports", nil)
	cv.AddKeyBinding("G", "Warnings", nil)

	cv.RegisterHandlers()

	return cv
}

// fetchProcessDetails returns witr-style detail rows for the selected process
func (pv *ProcessView) fetchProcessDetails() ([][]string, error) {
	p := pv.detailsProcess

	if p == nil {
		return [][]string{
			{"", "[yellow]No process selected. Press P to go back and select one."},
		}, nil
	}

	var data [][]string

	// ── Target ──────────────────────────────────────────────
	data = append(data, []string{"[yellow::b]Target", ""})
	data = append(data, []string{"Query", fmt.Sprintf("%s (PID %d)", p.Name, p.PID)})
	data = append(data, []string{"", ""})

	// ── Process ─────────────────────────────────────────────
	data = append(data, []string{"[yellow::b]Process", ""})
	data = append(data, []string{"Name", p.Name})
	data = append(data, []string{"PID", fmt.Sprintf("%d", p.PID)})
	data = append(data, []string{"User", p.Username})
	data = append(data, []string{"Status", p.Status})
	data = append(data, []string{"Command", p.Cmdline})

	if p.CreateTime > 0 {
		created := time.Unix(p.CreateTime/1000, 0)
		elapsed := time.Since(created)
		data = append(data, []string{"Started", fmt.Sprintf(
			"%s (%s ago)", created.Format("Mon 2006-01-02 15:04:05"), formatDuration(elapsed),
		)})
	}

	data = append(data, []string{"CPU", fmt.Sprintf("%.1f%%", p.CPUPercent)})
	data = append(data, []string{"Memory", fmt.Sprintf("%.1f%% (%s RSS)", p.MemPercent, formatBytes(p.MemRSS))})
	data = append(data, []string{"Threads", fmt.Sprintf("%d", p.Threads)})
	data = append(data, []string{"", ""})

	// ── Why It Exists ───────────────────────────────────────
	data = append(data, []string{"[yellow::b]Why It Exists", ""})
	data = append(data, []string{"Ancestry", p.GetAncestryString()})
	data = append(data, []string{"", ""})

	// Show tree view of ancestry
	tree := p.GetAncestryTree()
	for _, line := range strings.Split(tree, "\n") {
		if line != "" {
			data = append(data, []string{"", line})
		}
	}
	data = append(data, []string{"", ""})

	// ── Source ───────────────────────────────────────────────
	data = append(data, []string{"[yellow::b]Source", ""})
	data = append(data, []string{"Started By", p.Source})
	data = append(data, []string{"", ""})

	// ── Context ─────────────────────────────────────────────
	data = append(data, []string{"[yellow::b]Context", ""})
	if p.Cwd != "" {
		data = append(data, []string{"Working Dir", p.Cwd})
	}
	if p.GitRepo != "" {
		gitInfo := p.GitRepo
		if p.GitBranch != "" {
			gitInfo += " (" + p.GitBranch + ")"
		}
		data = append(data, []string{"Git Repo", gitInfo})
	}
	if len(p.Ports) > 0 {
		data = append(data, []string{"Listening", p.GetPortsString()})
	}
	data = append(data, []string{"", ""})

	// ── Warnings ────────────────────────────────────────────
	if len(p.Warnings) > 0 {
		data = append(data, []string{"[red::b]Warnings", ""})
		for _, w := range p.Warnings {
			data = append(data, []string{"", "[red]" + w + "[white]"})
		}
	} else {
		data = append(data, []string{"[green::b]Warnings", ""})
		data = append(data, []string{"", "[green]No warnings[white]"})
	}

	return data, nil
}
