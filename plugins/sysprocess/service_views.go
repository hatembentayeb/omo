package sysprocess

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"omo/pkg/pluginrpc"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Processes", Action: "goto_processes"},
		{Key: "1", Label: "Ports", Action: "goto_ports"},
		{Key: "2", Label: "Warnings", Action: "goto_warnings"},
		{Key: "3", Label: "Metrics", Action: "goto_metrics"},
		{Key: "4", Label: "Disk", Action: "goto_disk"},
	}
}

const labelWhyRunning = "Why Running?"

func processesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "W", Label: labelWhyRunning, Action: "details"},
		{Key: "K", Label: "Kill", Action: "kill"},
		{Key: "T", Label: "Sort CPU", Action: "sort_cpu"},
		{Key: "M", Label: "Sort Mem", Action: "sort_mem"},
	}
}

func portsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "W", Label: labelWhyRunning, Action: "details"},
		{Key: "K", Label: "Kill", Action: "kill"},
		{Key: "J", Label: "Jump", Action: "jump_to_process"},
	}
}

func warningsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "W", Label: labelWhyRunning, Action: "details"},
	}
}

func diskActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Open", Action: "disk_open"},
		{Key: "U", Label: "Up", Action: "disk_up"},
	}
}

func helpSections() []pluginrpc.HelpSection {
	return pluginrpc.HelpNav(viewNavBindings(), nil,
		pluginrpc.HelpSection{Title: "Processes", Bindings: processesActions()},
		pluginrpc.HelpSection{Title: "Ports", Bindings: portsActions()},
		pluginrpc.HelpSection{Title: "Warnings", Bindings: warningsActions()},
		pluginrpc.HelpSection{Title: "Disk", Bindings: diskActions()},
	)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	Help:  helpSections,
}

func (s *Service) baseInfo(extra string) string {
	msg := fmt.Sprintf("[green]Process Monitor[white]\nUser: %s\nProcesses: %d\nView: %s",
		s.currentUser, len(s.processes), s.currentView)
	return pluginrpc.FormatInfo(msg, extra)
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewProcesses
	}
	s.currentView = viewID

	if len(s.processes) == 0 && viewID != viewDisk && viewID != viewMetrics {
		s.loadProcessDataLocked()
	}

	switch viewID {
	case viewDetails:
		return s.viewDetailsLocked()
	case viewPorts:
		return s.viewPortsLocked()
	case viewWarnings:
		return s.viewWarningsLocked()
	case viewMetrics:
		return s.viewMetricsLocked()
	case viewDisk:
		return s.viewDiskLocked()
	default:
		return s.viewProcessesLocked()
	}
}

func (s *Service) viewProcessesLocked() (pluginrpc.ViewData, error) {
	rows := pluginrpc.MapRows(s.processes, func(p *UserProcess) []string { return p.GetTableRow() })
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "0", "0", "-", "-", "No processes"})
	return ui.OK(viewProcesses, "User Processes", s.baseInfo(""), []string{"PID", "Name", "User", "CPU%", "Mem%", "Source", "Status", "Started"}, rows, "PID", processesActions()...), nil
}

func (s *Service) viewDetailsLocked() (pluginrpc.ViewData, error) {
	p := s.detailsProcess
	if p == nil {
		return ui.OK(viewDetails, "Why Is This Running?", s.baseInfo(""), []string{"Property", "Value"}, [][]string{{"", "[yellow]No process selected. Press 0 and select one, then W."}}, "Property"), nil
	}

	var data [][]string
	data = append(data, []string{"[yellow::b]Target", ""})
	data = append(data, []string{"Query", fmt.Sprintf("%s (PID %d)", p.Name, p.PID)})
	data = append(data, []string{"", ""})
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
	data = append(data, []string{"[yellow::b]Why It Exists", ""})
	data = append(data, []string{"Ancestry", p.GetAncestryString()})
	data = append(data, []string{"", ""})
	for _, line := range strings.Split(p.GetAncestryTree(), "\n") {
		if line != "" {
			data = append(data, []string{"", line})
		}
	}
	data = append(data, []string{"", ""})
	data = append(data, []string{"[yellow::b]Source", ""})
	data = append(data, []string{"Started By", p.Source})
	data = append(data, []string{"", ""})
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
	if len(p.Warnings) > 0 {
		data = append(data, []string{"", ""})
		data = append(data, []string{"[yellow::b]Warnings", ""})
		for _, w := range p.Warnings {
			data = append(data, []string{"!", w})
		}
	}

	return ui.OK(viewDetails, "Why Is This Running?", s.baseInfo(fmt.Sprintf("PID %d", p.PID)), []string{"Property", "Value"}, data, "Property"), nil
}

func (s *Service) viewPortsLocked() (pluginrpc.ViewData, error) {
	s.portCache = getAllListeningPorts()
	processesByPID := make(map[int32]*UserProcess)
	for _, p := range s.processes {
		processesByPID[p.PID] = p
	}

	var data [][]string
	for pid, addrs := range s.portCache {
		if pid == 0 {
			for _, addr := range addrs {
				bindWarning := ""
				if len(addr) > 0 && addr[0] == '*' {
					bindWarning = " [red](public)[white]"
				}
				data = append(data, []string{"?", "? [gray](run sudo)[white]", "?", addr + bindWarning, "-"})
			}
			continue
		}
		proc, err := process.NewProcess(pid)
		if err != nil {
			for _, addr := range addrs {
				data = append(data, []string{fmt.Sprintf("%d", pid), "?", "?", addr, "-"})
			}
			continue
		}
		if !isUserProcess(proc) {
			continue
		}
		name, _ := proc.Name()
		username, _ := proc.Username()
		source := "-"
		if up, ok := processesByPID[pid]; ok {
			source = up.Source
			name = up.Name
			username = up.Username
		} else {
			ancestry := getProcessAncestry(proc)
			source = detectSource(ancestry)
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
	sort.Slice(data, func(i, j int) bool {
		return data[i][0] < data[j][0]
	})
	if len(data) == 0 {
		data = append(data, []string{"-", "-", "-", "No listening ports", "-"})
	}
	return ui.OK(viewPorts, "Listening Ports", s.baseInfo(""), []string{"PID", "Name", "User", "Address", "Source"}, data, "PID", portsActions()...), nil
}

func (s *Service) viewWarningsLocked() (pluginrpc.ViewData, error) {
	var data [][]string
	for _, p := range s.processes {
		for _, w := range p.Warnings {
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
	return ui.OK(viewWarnings, "Process Warnings", s.baseInfo(""), []string{"PID", "Name", "Warning", "Details"}, data, "PID", warningsActions()...), nil
}

func (s *Service) viewMetricsLocked() (pluginrpc.ViewData, error) {
	var data [][]string

	cpuPercent, _ := cpu.Percent(0, false)
	cpuUsage := 0.0
	if len(cpuPercent) > 0 {
		cpuUsage = cpuPercent[0]
	}
	cpuInfo, _ := cpu.Info()
	cpuModel := "Unknown"
	if len(cpuInfo) > 0 {
		cpuModel = cpuInfo[0].ModelName
	}
	data = append(data, []string{"[yellow]CPU[white]", ""})
	data = append(data, []string{"Model", truncateString(cpuModel, 50)})
	data = append(data, []string{"Cores", fmt.Sprintf("%d", runtime.NumCPU())})
	data = append(data, []string{"Usage", fmt.Sprintf("%.1f%%", cpuUsage)})
	data = append(data, []string{"", createBarGraph(cpuUsage, 100, 40)})
	data = append(data, []string{"", ""})

	if memInfo, err := mem.VirtualMemory(); err == nil {
		data = append(data, []string{"[yellow]Memory[white]", ""})
		data = append(data, []string{"Total", formatBytes(memInfo.Total)})
		data = append(data, []string{"Used", fmt.Sprintf("%s (%.1f%%)", formatBytes(memInfo.Used), memInfo.UsedPercent)})
		data = append(data, []string{"Available", formatBytes(memInfo.Available)})
		data = append(data, []string{"", createBarGraph(memInfo.UsedPercent, 100, 40)})
		data = append(data, []string{"", ""})
	}

	if loadAvg, err := load.Avg(); err == nil {
		data = append(data, []string{"[yellow]Load Average[white]", ""})
		data = append(data, []string{"1 min", fmt.Sprintf("%.2f", loadAvg.Load1)})
		data = append(data, []string{"5 min", fmt.Sprintf("%.2f", loadAvg.Load5)})
		data = append(data, []string{"15 min", fmt.Sprintf("%.2f", loadAvg.Load15)})
		data = append(data, []string{"", ""})
	}

	if hostInfo, err := host.Info(); err == nil {
		data = append(data, []string{"[yellow]Host[white]", ""})
		data = append(data, []string{"Hostname", hostInfo.Hostname})
		data = append(data, []string{"Platform", hostInfo.Platform})
		data = append(data, []string{"Uptime", formatDuration(time.Duration(hostInfo.Uptime) * time.Second)})
		data = append(data, []string{"", ""})
	}

	partitions, _ := disk.Partitions(false)
	data = append(data, []string{"[yellow]Disk[white]", ""})
	shown := 0
	for _, part := range partitions {
		if shown >= 5 {
			break
		}
		usage, err := disk.Usage(part.Mountpoint)
		if err != nil || usage.Total < 1<<30 {
			continue
		}
		data = append(data, []string{part.Mountpoint, fmt.Sprintf("%s / %s (%.0f%%)",
			formatBytes(usage.Used), formatBytes(usage.Total), usage.UsedPercent)})
		shown++
	}
	data = append(data, []string{"", ""})
	data = append(data, []string{"User Processes", fmt.Sprintf("%d", len(s.processes))})

	return ui.OK(viewMetrics, "System Metrics", s.baseInfo(""), []string{"Metric", "Value"}, data, "Metric"), nil
}

func (s *Service) viewDiskLocked() (pluginrpc.ViewData, error) {
	if len(s.diskEntries) == 0 {
		s.scanDiskLocked(s.diskPath)
	}
	rows := make([][]string, 0, len(s.diskEntries))
	for _, e := range s.diskEntries {
		name := e.name
		if e.isParent {
			name = "[yellow]..[white]"
		} else if e.isDir {
			name = name + "/"
		}
		sizeStr := "-"
		if !e.isParent {
			sizeStr = formatBytes(uint64(e.size))
		}
		rows = append(rows, []string{sizeStr, name})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"", "[yellow]No entries[white]"})
	return ui.OK(viewDisk, "Disk Usage (ncdu)", s.baseInfo("Path: "+truncateString(s.diskPath, 50)), []string{"Size", "Name"}, rows, "Name", diskActions()...), nil
}

// scanDirectoryEntries lists path contents with sizes (sync; may be slow on large trees).
func scanDirectoryEntries(path string) []diskEntry {
	var entries []diskEntry
	if path != "/" {
		entries = append(entries, diskEntry{
			name:     "..",
			path:     filepath.Dir(path),
			isDir:    true,
			isParent: true,
		})
	}
	dir, err := os.Open(path)
	if err != nil {
		return entries
	}
	defer dir.Close()
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return entries
	}
	type sized struct {
		entry diskEntry
		size  int64
	}
	var sizedList []sized
	for _, name := range names {
		if name == "." || name == ".." {
			continue
		}
		fullPath := filepath.Join(path, name)
		info, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}
		var size int64
		if info.IsDir() {
			size = dirSize(fullPath)
		} else {
			size = info.Size()
		}
		sizedList = append(sizedList, sized{
			entry: diskEntry{name: name, path: fullPath, size: size, isDir: info.IsDir()},
			size:  size,
		})
	}
	sort.Slice(sizedList, func(i, j int) bool { return sizedList[i].size > sizedList[j].size })
	for _, item := range sizedList {
		entries = append(entries, item.entry)
	}
	return entries
}
