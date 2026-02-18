package main

import (
	"fmt"
	"runtime"
	"time"

	"omo/pkg/ui"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

// newMetricsView creates the system metrics CoreView
func (pv *ProcessView) newMetricsView() *ui.CoreView {
	cv := ui.NewCoreView(pv.app, "System Metrics")
	cv.SetTableHeaders([]string{"Metric", "Value"})
	cv.SetRefreshCallback(pv.fetchMetricsData)
	cv.SetActionCallback(pv.handleAction)

	cv.AddKeyBinding("P", "Processes", nil)
	cv.AddKeyBinding("S", "Metrics", nil)
	cv.AddKeyBinding("D", "Disk", nil)
	cv.AddKeyBinding("L", "Ports", nil)
	cv.AddKeyBinding("G", "Warnings", nil)

	cv.RegisterHandlers()

	return cv
}

// fetchMetricsData returns system metrics (CPU, memory, disk, load, uptime)
func (pv *ProcessView) fetchMetricsData() ([][]string, error) {
	var data [][]string

	// CPU
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

	// Memory
	memInfo, err := mem.VirtualMemory()
	if err == nil {
		data = append(data, []string{"[yellow]Memory[white]", ""})
		data = append(data, []string{"Total", formatBytes(memInfo.Total)})
		data = append(data, []string{"Used", fmt.Sprintf("%s (%.1f%%)", formatBytes(memInfo.Used), memInfo.UsedPercent)})
		data = append(data, []string{"Available", formatBytes(memInfo.Available)})
		data = append(data, []string{"Usage", fmt.Sprintf("%.1f%%", memInfo.UsedPercent)})
		data = append(data, []string{"", createBarGraph(memInfo.UsedPercent, 100, 40)})
		data = append(data, []string{"", ""})
	}

	// Load average
	loadAvg, err := load.Avg()
	if err == nil {
		data = append(data, []string{"[yellow]Load Average[white]", ""})
		data = append(data, []string{"1 min", fmt.Sprintf("%.2f", loadAvg.Load1)})
		data = append(data, []string{"5 min", fmt.Sprintf("%.2f", loadAvg.Load5)})
		data = append(data, []string{"15 min", fmt.Sprintf("%.2f", loadAvg.Load15)})
		data = append(data, []string{"", ""})
	}

	// Host & Uptime
	hostInfo, err := host.Info()
	if err == nil {
		data = append(data, []string{"[yellow]Host[white]", ""})
		data = append(data, []string{"Hostname", hostInfo.Hostname})
		data = append(data, []string{"Platform", hostInfo.Platform})
		data = append(data, []string{"Uptime", formatDuration(time.Duration(hostInfo.Uptime) * time.Second)})
		data = append(data, []string{"", ""})
	}

	// Disk
	partitions, _ := disk.Partitions(false)
	data = append(data, []string{"[yellow]Disk[white]", ""})
	shown := 0
	for _, part := range partitions {
		if shown >= 5 {
			break
		}
		usage, err := disk.Usage(part.Mountpoint)
		if err != nil {
			continue
		}
		// Skip small/temp filesystems
		if usage.Total < 1024*1024*1024 { // < 1GB
			continue
		}
		shown++
		data = append(data, []string{part.Mountpoint, fmt.Sprintf("%s total", formatBytes(usage.Total))})
		data = append(data, []string{"", fmt.Sprintf("%.1f%% used", usage.UsedPercent)})
		data = append(data, []string{"", createBarGraph(usage.UsedPercent, 100, 40)})
		data = append(data, []string{"", ""})
	}
	if shown == 0 {
		data = append(data, []string{"", "[gray]No partitions[white]"})
	}

	// Process count
	pv.mu.Lock()
	procCount := len(pv.processes)
	pv.mu.Unlock()
	data = append(data, []string{"[yellow]Processes[white]", ""})
	data = append(data, []string{"Your processes", fmt.Sprintf("%d", procCount)})
	data = append(data, []string{"Updated", time.Now().Format("15:04:05")})

	return data, nil
}
