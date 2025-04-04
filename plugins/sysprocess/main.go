package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"omo/ui"

	"github.com/rivo/tview"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// Logger for the plugin
var logger *log.Logger
var logFile *os.File

// Initialize logger
func init() {
	// Create logs directory if it doesn't exist
	logsDir := "./logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		fmt.Printf("Failed to create logs directory: %v\n", err)
	}

	// Create or open log file
	logFilePath := filepath.Join(logsDir, "sysprocess.log")
	var err error
	logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Failed to open log file: %v\n", err)
		return
	}

	// Set up the logger
	logger = log.New(logFile, "", log.LstdFlags)
	logger.Println("SysProcess plugin initialized")
}

// cleanup closes resources when the program exits
func cleanup() {
	if logFile != nil {
		logger.Println("Closing SysProcess plugin log file")
		logFile.Close()
	}
}

// Register cleanup function for when the plugin unloads
func init() {
	// This will ensure cleanup happens even if the plugin panics
	runtime.SetFinalizer(&OhmyopsPlugin, func(_ *Plugin) {
		cleanup()
	})
}

// logPanic logs panic information to file
func logPanic() {
	if r := recover(); r != nil {
		// Get stack trace
		buf := make([]byte, 4096)
		n := runtime.Stack(buf, false)
		stackTrace := string(buf[:n])

		// Log the panic with stack trace
		logger.Printf("PANIC RECOVERED: %v\n%s", r, stackTrace)

		// Also print to stderr
		fmt.Fprintf(os.Stderr, "SysProcess plugin panic: %v\n%s", r, stackTrace)
	}
}

// safeGo runs a function in a goroutine with panic recovery
func safeGo(f func()) {
	go func() {
		defer logPanic()
		f()
	}()
}

// Plugin is the main plugin struct
type Plugin struct {
	app         *tview.Application
	pages       *tview.Pages
	cores       *ui.Cores
	processes   []*process.Process
	lastCPU     map[int32]float64
	selectedPID int32
	currentView string
}

// Start initializes the plugin
func (p *Plugin) Start(app *tview.Application) tview.Primitive {
	logger.Println("Starting SysProcess plugin")
	p.app = app
	p.pages = tview.NewPages()
	p.lastCPU = make(map[int32]float64)
	p.currentView = "main"

	// Initialize main view
	p.initializeMainView()

	// Initial data load
	safeGo(p.refreshProcessData)

	logger.Println("SysProcess plugin started successfully")
	return p.pages
}

// GetMetadata returns plugin metadata
func (p *Plugin) GetMetadata() interface{} {
	logger.Println("GetMetadata called")
	return map[string]interface{}{
		"Name":        "sysprocess",
		"Version":     "1.0.0",
		"Description": "System process and resource monitor",
		"Author":      "OhMyOps",
		"License":     "MIT",
		"Tags":        []string{"system", "monitoring", "process"},
		"LastUpdated": time.Now().Format("Jan 2006"),
	}
}

// initializeMainView creates the main process list view
func (p *Plugin) initializeMainView() {
	// Create pattern for initializing the main view
	pattern := ui.ViewPattern{
		App:          p.app,
		Pages:        p.pages,
		Title:        "System Process Monitor",
		HeaderText:   "Monitor and analyze system processes",
		TableHeaders: []string{"PID", "Name", "CPU%", "Memory%", "Status", "Threads", "Created"},
		RefreshFunc:  p.fetchProcessData,
		KeyHandlers: map[string]string{
			"R": "Refresh",
			"K": "Kill Process",
			"I": "Process Info",
			"S": "System Info",
			"D": "Resource Dashboard",
			"N": "Kernel Info",
			"T": "Sort by CPU",
			"M": "Sort by Memory",
			"?": "Help",
		},
		SelectedFunc: p.onProcessSelected,
	}

	// Initialize the UI
	p.cores = ui.InitializeView(pattern)

	// Set up action handler
	p.setupActionHandler()

	// Add the core UI to the pages
	p.pages.AddPage("main", p.cores.GetLayout(), true, true)

	// Push initial view to navigation stack
	p.cores.PushView("Process List")

	// Log initial state
	p.cores.Log("Plugin initialized")
	p.updateSystemInfo()
}

// setupActionHandler configures the action handler for the plugin
func (p *Plugin) setupActionHandler() {
	p.cores.SetActionCallback(func(action string, payload map[string]interface{}) error {
		if action == "keypress" {
			if key, ok := payload["key"].(string); ok {
				switch key {
				case "R":
					p.refreshProcessData()
				case "K":
					p.killSelectedProcess()
				case "I":
					p.showProcessDetails()
				case "S":
					p.showSystemInfo()
				case "D":
					p.showResourceDashboard()
				case "N":
					p.showKernelInfo()
				case "T":
					p.sortByCPU()
				case "M":
					p.sortByMemory()
				case "?":
					p.showHelpModal()
				case "ESC":
					// Handle ESC key to return to the process list
					if p.currentView != "main" {
						p.returnToProcessList()
					}
				}
			}
		}
		return nil
	})
}

// returnToProcessList returns to the main process list view
func (p *Plugin) returnToProcessList() {
	logger.Println("Returning to process list")
	p.cores.PopView() // Remove current view from navigation stack
	p.currentView = "main"

	// Update UI back to process list
	p.cores.SetTableHeaders([]string{"PID", "Name", "CPU%", "Memory%", "Status", "Threads", "Created"})
	p.cores.Log("[blue]Returned to process list")
	p.cores.RefreshData()
}

// showHelpModal displays a help modal with keyboard shortcuts
func (p *Plugin) showHelpModal() {
	logger.Println("Showing help modal")

	// Create help content in a format consistent with other plugins
	helpText := `[yellow]System Process Monitor Help[white]

[aqua]Key Bindings:[white]
[green]R[white] - Refresh process list
[green]K[white] - Kill selected process
[green]I[white] - Show detailed process information
[green]S[white] - Show system information
[green]D[white] - Show resource dashboard
[green]N[white] - Show kernel information
[green]T[white] - Sort processes by CPU usage
[green]M[white] - Sort processes by memory usage
[green]?[white] - Show this help dialog
[green]ESC[white] - Close modal or return to process list view

[aqua]Navigation:[white]
- Use arrow keys to navigate the process list
- Select a process by clicking on it or using arrow keys
- Press ESC to close modals or return to the main process list
- When viewing system info, resource dashboard, or kernel info, press ESC to return to the process list
- The breadcrumb at the bottom shows your current location
`

	// Show help modal using the InfoModal which is consistent with other plugins
	ui.ShowInfoModal(
		p.pages, p.app,
		"Help",
		helpText,
		func() {
			// Ensure table regains focus after modal is closed
			p.app.SetFocus(p.cores.GetTable())
			p.cores.Log("Closed help dialog")
		},
	)
}

// refreshProcessData refreshes process data
func (p *Plugin) refreshProcessData() {
	logger.Println("Refreshing process data")
	p.cores.Log("Refreshing process data...")

	// Show progress
	pm := ui.ShowProgressModal(
		p.pages, p.app, "Loading Process Data", 100, true,
		nil, true,
	)

	safeGo(func() {
		// Get processes
		processes, err := process.Processes()
		if err != nil {
			errMsg := fmt.Sprintf("Failed to get processes: %v", err)
			logger.Println(errMsg)

			p.app.QueueUpdateDraw(func() {
				pm.Close()
				ui.ShowStandardErrorModal(
					p.pages, p.app, "Error",
					errMsg,
					nil,
				)
			})
			return
		}

		logger.Printf("Retrieved %d processes", len(processes))
		pm.UpdateProgress(30, "Retrieved process list...")

		// Calculate CPU percentages for each process
		for i, proc := range processes {
			if i%10 == 0 {
				progress := 30 + int(float64(i)/float64(len(processes))*50)
				pm.UpdateProgress(progress, fmt.Sprintf("Processing... %d/%d", i, len(processes)))
			}

			// Cache CPU percentages
			pid := proc.Pid
			percent, err := proc.CPUPercent()
			if err == nil {
				p.lastCPU[pid] = percent
			} else {
				logger.Printf("Failed to get CPU percent for PID %d: %v", pid, err)
			}
		}

		pm.UpdateProgress(90, "Finalizing data...")

		p.processes = processes

		// Update the table
		p.app.QueueUpdateDraw(func() {
			p.cores.RefreshData()
			pm.Close()
			p.cores.Log(fmt.Sprintf("[green]Found %d processes", len(processes)))
			p.updateSystemInfo()
		})

		logger.Printf("Process data refresh completed. %d processes found", len(processes))
	})
}

// fetchProcessData retrieves data for the table
func (p *Plugin) fetchProcessData() ([][]string, error) {
	// Return different data based on current view
	switch p.currentView {
	case "main":
		return p.fetchProcessListData()
	case "sysinfo":
		return p.fetchSystemInfoData()
	case "resource":
		return p.fetchResourceData()
	case "kernel":
		return p.fetchKernelInfoData()
	default:
		return p.fetchProcessListData()
	}
}

// fetchProcessListData retrieves process list data for the main table
func (p *Plugin) fetchProcessListData() ([][]string, error) {
	data := make([][]string, 0, len(p.processes))

	for _, proc := range p.processes {
		// Get process info
		pid := proc.Pid
		name, _ := proc.Name()
		status, _ := proc.Status()

		// Convert status from []string to string by joining with comma
		statusStr := strings.Join(status, ", ")

		// Get memory usage
		memPercent, _ := proc.MemoryPercent()

		// Get CPU percentage from cache
		cpuPercent := p.lastCPU[pid]

		// Get thread count
		threads, _ := proc.NumThreads()

		// Get creation time
		createTime, _ := proc.CreateTime()
		createTimeFormatted := time.Unix(createTime/1000, 0).Format("Jan 02 15:04")

		// Add to data
		data = append(data, []string{
			fmt.Sprintf("%d", pid),
			name,
			fmt.Sprintf("%.1f", cpuPercent),
			fmt.Sprintf("%.1f", memPercent),
			statusStr,
			fmt.Sprintf("%d", threads),
			createTimeFormatted,
		})
	}

	return data, nil
}

// fetchSystemInfoData retrieves system information for the system info view
func (p *Plugin) fetchSystemInfoData() ([][]string, error) {
	var data [][]string

	// Get host info
	hostInfo, err := host.Info()
	if err != nil {
		return data, err
	}

	// Get uptime
	uptime := time.Duration(hostInfo.Uptime) * time.Second
	days := int(uptime.Hours() / 24)
	hours := int(uptime.Hours()) % 24
	minutes := int(uptime.Minutes()) % 60
	uptimeStr := fmt.Sprintf("%d days, %d hours, %d minutes", days, hours, minutes)

	// Get platform info
	platform := fmt.Sprintf("%s %s (%s)", hostInfo.Platform, hostInfo.PlatformVersion, hostInfo.PlatformFamily)

	// Get CPU info
	cpuInfo, _ := cpu.Info()
	cpuCount := runtime.NumCPU()
	var cpuModel string
	if len(cpuInfo) > 0 {
		cpuModel = cpuInfo[0].ModelName
	} else {
		cpuModel = "Unknown"
	}

	// Get memory info
	memInfo, _ := mem.VirtualMemory()

	// Host information
	data = append(data, []string{"[yellow]Host Information[white]", ""})
	data = append(data, []string{"Hostname", hostInfo.Hostname})
	data = append(data, []string{"Platform", platform})
	data = append(data, []string{"Kernel", hostInfo.KernelVersion})
	data = append(data, []string{"Uptime", uptimeStr})
	data = append(data, []string{"", ""})

	// Hardware information
	data = append(data, []string{"[yellow]Hardware Information[white]", ""})
	data = append(data, []string{"CPU Model", cpuModel})
	data = append(data, []string{"CPU Cores", fmt.Sprintf("%d", cpuCount)})
	data = append(data, []string{"", ""})

	// Memory information
	data = append(data, []string{"[yellow]Memory Information[white]", ""})
	data = append(data, []string{"Total Memory", formatBytes(memInfo.Total)})
	data = append(data, []string{"Used Memory", fmt.Sprintf("%s (%.1f%%)", formatBytes(memInfo.Used), memInfo.UsedPercent)})
	data = append(data, []string{"Free Memory", formatBytes(memInfo.Free)})
	data = append(data, []string{"", ""})

	// Process information
	data = append(data, []string{"[yellow]Process Information[white]", ""})
	data = append(data, []string{"Process Count", fmt.Sprintf("%d", len(p.processes))})
	data = append(data, []string{"Last Updated", time.Now().Format("2006-01-02 15:04:05")})

	return data, nil
}

// fetchResourceData retrieves resource usage data for the resource dashboard
func (p *Plugin) fetchResourceData() ([][]string, error) {
	var data [][]string

	// Get CPU usage
	cpuPercent, _ := cpu.Percent(0, false)
	var cpuUsage float64
	if len(cpuPercent) > 0 {
		cpuUsage = cpuPercent[0]
	}

	// Get memory usage
	memInfo, _ := mem.VirtualMemory()

	// CPU Usage information
	data = append(data, []string{"[yellow]CPU Usage[white]", ""})
	cpuBar := createBarGraph(cpuUsage, 100, 30, "")
	data = append(data, []string{"CPU", fmt.Sprintf("%.1f%%", cpuUsage)})
	data = append(data, []string{"", cpuBar})
	data = append(data, []string{"", ""})

	// Memory Usage information
	data = append(data, []string{"[yellow]Memory Usage[white]", ""})
	memBar := createBarGraph(memInfo.UsedPercent, 100, 30, "")
	data = append(data, []string{"Memory", fmt.Sprintf("%.1f%%", memInfo.UsedPercent)})
	data = append(data, []string{"", memBar})
	data = append(data, []string{"", ""})

	// Disk Usage information
	data = append(data, []string{"[yellow]Disk Usage[white]", ""})
	partitions, _ := disk.Partitions(false)
	for _, part := range partitions {
		usage, err := disk.Usage(part.Mountpoint)
		if err != nil {
			continue
		}

		data = append(data, []string{part.Mountpoint, fmt.Sprintf("%s (%s)", part.Fstype, formatBytes(usage.Total))})
		diskBar := createBarGraph(usage.UsedPercent, 100, 30, "")
		data = append(data, []string{"", fmt.Sprintf("%.1f%% - %s used", usage.UsedPercent, formatBytes(usage.Used))})
		data = append(data, []string{"", diskBar})
		data = append(data, []string{"", ""})
	}

	// Top CPU Processes
	data = append(data, []string{"[yellow]Top CPU Processes[white]", ""})
	topCPUProcs := make([]*process.Process, 0, len(p.processes))
	for _, proc := range p.processes {
		topCPUProcs = append(topCPUProcs, proc)
	}
	sortProcessesByCPU(topCPUProcs, p.lastCPU)

	for i, proc := range topCPUProcs {
		if i >= 5 {
			break
		}

		name, _ := proc.Name()
		cpuPercent := p.lastCPU[proc.Pid]

		// Add color based on CPU usage
		var cpuColor string
		if cpuPercent > 50 {
			cpuColor = "[red]"
		} else if cpuPercent > 20 {
			cpuColor = "[yellow]"
		} else {
			cpuColor = "[green]"
		}

		data = append(data, []string{fmt.Sprintf("%s (PID: %d)", name, proc.Pid),
			fmt.Sprintf("%s%.1f%%[white]", cpuColor, cpuPercent)})
	}
	data = append(data, []string{"", ""})

	// Top Memory Processes
	data = append(data, []string{"[yellow]Top Memory Processes[white]", ""})
	topMemProcs := make([]*process.Process, 0, len(p.processes))
	for _, proc := range p.processes {
		topMemProcs = append(topMemProcs, proc)
	}
	sortProcessesByMemory(topMemProcs)

	for i, proc := range topMemProcs {
		if i >= 5 {
			break
		}

		name, _ := proc.Name()
		memPercent, _ := proc.MemoryPercent()
		memInfo, _ := proc.MemoryInfo()

		// Add color based on memory usage
		var memColor string
		if memPercent > 5 {
			memColor = "[red]"
		} else if memPercent > 1 {
			memColor = "[yellow]"
		} else {
			memColor = "[green]"
		}

		data = append(data, []string{fmt.Sprintf("%s (PID: %d)", name, proc.Pid),
			fmt.Sprintf("%s%.1f%%[white] (%s)", memColor, memPercent, formatBytes(memInfo.RSS))})
	}

	return data, nil
}

// fetchKernelInfoData retrieves kernel information data
func (p *Plugin) fetchKernelInfoData() ([][]string, error) {
	var data [][]string

	// Get host info for kernel version
	hostInfo, err := host.Info()
	if err != nil {
		return data, err
	}

	// Kernel Information
	data = append(data, []string{"[yellow]Kernel Information[white]", ""})
	data = append(data, []string{"Kernel Version", hostInfo.KernelVersion})
	data = append(data, []string{"Kernel Architecture", hostInfo.KernelArch})
	data = append(data, []string{"Operating System", hostInfo.OS})
	data = append(data, []string{"", ""})

	// Run uname command
	unameCmd := exec.Command("uname", "-a")
	unameOutput, err := unameCmd.Output()
	if err == nil {
		data = append(data, []string{"[yellow]Full uname Output[white]", ""})
		data = append(data, []string{"", string(unameOutput)})
		data = append(data, []string{"", ""})
	}

	// Get kernel modules
	modulesCmd := exec.Command("lsmod")
	modulesOutput, err := modulesCmd.Output()
	if err == nil {
		data = append(data, []string{"[yellow]Loaded Kernel Modules[white]", ""})
		modules := strings.Split(string(modulesOutput), "\n")
		for i, line := range modules {
			if i == 0 || line == "" {
				continue // Skip header and empty lines
			}
			fields := strings.Fields(line)
			if len(fields) >= 1 {
				data = append(data, []string{fields[0], ""})
			}
		}
		data = append(data, []string{"", ""})
	}

	// Get kernel parameters
	sysctlCmd := exec.Command("sysctl", "-a")
	sysctlOutput, err := sysctlCmd.Output()
	if err == nil {
		data = append(data, []string{"[yellow]Kernel Parameters (partial list)[white]", ""})
		sysctlLines := strings.Split(string(sysctlOutput), "\n")
		importantParams := []string{
			"vm.swappiness",
			"vm.dirty_ratio",
			"vm.dirty_background_ratio",
			"kernel.hostname",
			"kernel.ostype",
			"kernel.osrelease",
			"kernel.pid_max",
			"kernel.threads-max",
			"fs.file-max",
			"net.core.somaxconn",
		}

		for _, line := range sysctlLines {
			for _, param := range importantParams {
				if strings.HasPrefix(line, param) {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						data = append(data, []string{strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])})
					} else {
						data = append(data, []string{line, ""})
					}
					break
				}
			}
		}
	}

	return data, nil
}

// onProcessSelected handles process selection
func (p *Plugin) onProcessSelected(row int) {
	if row >= 0 && row < len(p.processes) {
		proc := p.processes[row]
		p.selectedPID = proc.Pid
		name, _ := proc.Name()
		p.cores.Log(fmt.Sprintf("[blue]Selected: %s (PID: %d)", name, proc.Pid))

		// Update info panel with basic process info
		cmdline, _ := proc.Cmdline()
		username, _ := proc.Username()
		p.cores.SetInfoMap(map[string]string{
			"PID":      fmt.Sprintf("%d", proc.Pid),
			"Name":     name,
			"User":     username,
			"Command":  truncateString(cmdline, 30),
			"Selected": time.Now().Format("15:04:05"),
		})
	}
}

// killSelectedProcess kills the selected process
func (p *Plugin) killSelectedProcess() {
	if p.selectedPID <= 0 {
		logger.Println("Attempted to kill process but no process selected")
		ui.ShowStandardErrorModal(
			p.pages, p.app, "Error",
			"Please select a process to kill",
			nil,
		)
		return
	}

	// Get process name
	var name string
	var cmdline string
	var username string
	for _, proc := range p.processes {
		if proc.Pid == p.selectedPID {
			name, _ = proc.Name()
			cmdline, _ = proc.Cmdline()
			username, _ = proc.Username()
			break
		}
	}

	logger.Printf("Attempting to kill process: %s (PID: %d)", name, p.selectedPID)

	// Prepare a more detailed confirmation message
	confirmationMsg := fmt.Sprintf(
		"[yellow]Are you sure you want to kill this process?[white]\n\n"+
			"[aqua]Process:[white] %s\n"+
			"[aqua]PID:[white] %d\n"+
			"[aqua]User:[white] %s\n"+
			"[aqua]Command:[white] %s\n\n"+
			"This will send SIGKILL (signal 9) to forcefully terminate the process.\n"+
			"Any unsaved data in this process will be lost.",
		name, p.selectedPID, username, truncateString(cmdline, 50),
	)

	// Show confirmation dialog
	ui.ShowStandardConfirmationModal(
		p.pages, p.app,
		"Confirm Kill Process",
		confirmationMsg,
		func(confirmed bool) {
			if confirmed {
				// Perform the kill
				cmd := exec.Command("kill", "-9", fmt.Sprintf("%d", p.selectedPID))
				err := cmd.Run()

				if err != nil {
					errMsg := fmt.Sprintf("Failed to kill process: %v", err)
					logger.Println(errMsg)

					ui.ShowStandardErrorModal(
						p.pages, p.app, "Error",
						errMsg,
						func() {
							// Return focus to the table
							p.app.SetFocus(p.cores.GetTable())
						},
					)
					return
				}

				logger.Printf("Successfully killed process: %s (PID: %d)", name, p.selectedPID)

				// Refresh process list
				p.cores.Log(fmt.Sprintf("[yellow]Killed process: %s (PID: %d)", name, p.selectedPID))
				p.refreshProcessData()
			} else {
				logger.Printf("Kill process cancelled for: %s (PID: %d)", name, p.selectedPID)
				p.cores.Log(fmt.Sprintf("[blue]Kill operation cancelled for: %s", name))
			}

			// Return focus to the table
			p.app.SetFocus(p.cores.GetTable())
		},
	)
}

// showProcessDetails shows detailed info about the selected process
func (p *Plugin) showProcessDetails() {
	if p.selectedPID <= 0 {
		ui.ShowStandardErrorModal(
			p.pages, p.app, "Error",
			"Please select a process to view",
			func() {
				// Return focus to the table
				p.app.SetFocus(p.cores.GetTable())
			},
		)
		return
	}

	// Find process
	var selectedProc *process.Process
	for _, proc := range p.processes {
		if proc.Pid == p.selectedPID {
			selectedProc = proc
			break
		}
	}

	if selectedProc == nil {
		ui.ShowStandardErrorModal(
			p.pages, p.app, "Error",
			fmt.Sprintf("Process with PID %d not found", p.selectedPID),
			func() {
				// Return focus to the table
				p.app.SetFocus(p.cores.GetTable())
			},
		)
		return
	}

	// Get process details
	name, _ := selectedProc.Name()
	cmdline, _ := selectedProc.Cmdline()
	cwd, _ := selectedProc.Cwd()
	createTime, _ := selectedProc.CreateTime()
	createTimeFormatted := time.Unix(createTime/1000, 0).Format("2006-01-02 15:04:05")
	status, _ := selectedProc.Status()
	statusStr := strings.Join(status, ", ")
	username, _ := selectedProc.Username()
	memInfo, _ := selectedProc.MemoryInfo()
	cpuPercent := p.lastCPU[p.selectedPID]
	memPercent, _ := selectedProc.MemoryPercent()
	numThreads, _ := selectedProc.NumThreads()

	// Format process information
	detailsText := fmt.Sprintf(
		"[yellow]Process Information[white]\n\n"+
			"[aqua]PID:[white] %d\n"+
			"[aqua]Name:[white] %s\n"+
			"[aqua]Status:[white] %s\n"+
			"[aqua]User:[white] %s\n"+
			"[aqua]Created:[white] %s\n\n"+

			"[yellow]Resource Usage[white]\n\n"+
			"[aqua]CPU Usage:[white] %.2f%%\n"+
			"[aqua]Memory Usage:[white] %.2f%%\n"+
			"[aqua]RSS Memory:[white] %s\n"+
			"[aqua]Virtual Memory:[white] %s\n"+
			"[aqua]Threads:[white] %d\n\n"+

			"[yellow]File System[white]\n\n"+
			"[aqua]Working Directory:[white] %s\n\n"+

			"[yellow]Command Line[white]\n\n%s\n",
		p.selectedPID,
		name,
		statusStr,
		username,
		createTimeFormatted,

		cpuPercent,
		memPercent,
		formatBytes(memInfo.RSS),
		formatBytes(memInfo.VMS),
		numThreads,

		cwd,
		cmdline,
	)

	// Add open files if available
	openFiles, err := selectedProc.OpenFiles()
	if err == nil && len(openFiles) > 0 {
		filesText := "[yellow]Open Files[white]\n\n"

		// Limit to 15 files to avoid overwhelming the display
		maxFiles := 15
		if len(openFiles) > maxFiles {
			filesText += fmt.Sprintf("(Showing %d of %d files)\n\n", maxFiles, len(openFiles))
		}

		for i, file := range openFiles {
			if i >= maxFiles {
				break
			}
			filesText += fmt.Sprintf("%s\n", file.Path)
		}

		detailsText += "\n\n" + filesText
	}

	logger.Printf("Showing process details for: %s (PID: %d)", name, p.selectedPID)

	// Show info modal
	ui.ShowInfoModal(
		p.pages, p.app,
		fmt.Sprintf("Process Details: %s (PID: %d)", name, p.selectedPID),
		detailsText,
		func() {
			// Return focus to the table
			p.app.SetFocus(p.cores.GetTable())
			p.cores.Log(fmt.Sprintf("[blue]Closed details for %s (PID: %d)", name, p.selectedPID))
		},
	)
}

// showSystemInfo shows system information
func (p *Plugin) showSystemInfo() {
	// Push view to navigation stack
	p.cores.PushView("System Info")
	p.currentView = "sysinfo"

	// Update UI
	p.cores.SetTableHeaders([]string{"Property", "Value"})
	p.cores.RefreshData()

	// Update system info periodically
	p.updateSystemInfo()
}

// updateSystemInfo updates system information
func (p *Plugin) updateSystemInfo() {
	logger.Println("Updating system info")
	safeGo(func() {
		// Get host info
		hostInfo, err := host.Info()
		if err != nil {
			errMsg := fmt.Sprintf("Failed to get host info: %v", err)
			logger.Println(errMsg)

			p.app.QueueUpdateDraw(func() {
				ui.ShowStandardErrorModal(
					p.pages, p.app, "Error",
					errMsg,
					nil,
				)
			})
			return
		}

		// Get uptime
		uptime := time.Duration(hostInfo.Uptime) * time.Second
		days := int(uptime.Hours() / 24)
		hours := int(uptime.Hours()) % 24
		minutes := int(uptime.Minutes()) % 60

		// Get CPU usage
		cpuPercent, _ := cpu.Percent(0, false)
		var cpuUsage float64
		if len(cpuPercent) > 0 {
			cpuUsage = cpuPercent[0]
		}

		// Get memory info
		memInfo, _ := mem.VirtualMemory()

		// Update info panel on main view
		p.app.QueueUpdateDraw(func() {
			p.cores.SetInfoMap(map[string]string{
				"Hostname":  hostInfo.Hostname,
				"CPU Usage": fmt.Sprintf("%.1f%%", cpuUsage),
				"Memory":    fmt.Sprintf("%.1f%%", memInfo.UsedPercent),
				"Processes": fmt.Sprintf("%d", len(p.processes)),
				"Uptime":    fmt.Sprintf("%dd %dh %dm", days, hours, minutes),
			})

			// Refresh the table if we're in system info view
			if p.currentView == "sysinfo" {
				p.cores.RefreshData()
			}
		})
	})
}

// showResourceDashboard shows the resource usage dashboard
func (p *Plugin) showResourceDashboard() {
	// Push view to navigation stack
	p.cores.PushView("Resource Dashboard")
	p.currentView = "resource"

	// Update UI
	p.cores.SetTableHeaders([]string{"Resource", "Usage"})
	p.cores.RefreshData()

	// Start periodic updates
	safeGo(func() {
		for {
			select {
			case <-time.After(2 * time.Second):
				if p.currentView == "resource" {
					p.app.QueueUpdateDraw(func() {
						p.cores.RefreshData()
					})
				} else {
					return
				}
			}
		}
	})
}

// showKernelInfo shows kernel information
func (p *Plugin) showKernelInfo() {
	// Push view to navigation stack
	p.cores.PushView("Kernel Info")
	p.currentView = "kernel"

	// Update UI
	p.cores.SetTableHeaders([]string{"Parameter", "Value"})
	p.cores.RefreshData()
}

// sortByCPU sorts processes by CPU usage
func (p *Plugin) sortByCPU() {
	p.cores.Log("Sorting by CPU usage...")
	sortProcessesByCPU(p.processes, p.lastCPU)

	p.cores.RefreshData()

}

// sortByMemory sorts processes by memory usage
func (p *Plugin) sortByMemory() {
	p.cores.Log("Sorting by memory usage...")
	sortProcessesByMemory(p.processes)

	p.cores.RefreshData()

}

// Helper functions
func sortProcessesByCPU(processes []*process.Process, cpuMap map[int32]float64) {
	// Sort processes by CPU usage
	sort.Slice(processes, func(i, j int) bool {
		return cpuMap[processes[i].Pid] > cpuMap[processes[j].Pid]
	})
}

func sortProcessesByMemory(processes []*process.Process) {
	// Sort processes by memory usage
	sort.Slice(processes, func(i, j int) bool {
		memI, _ := processes[i].MemoryPercent()
		memJ, _ := processes[j].MemoryPercent()
		return memI > memJ
	})
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

func createBarGraph(value, max float64, width int, label string) string {
	// Calculate number of filled blocks
	filledWidth := int((value / max) * float64(width))
	if filledWidth > width {
		filledWidth = width
	}

	// Choose color based on percentage
	var color string
	if value >= 80 {
		color = "[red]"
	} else if value >= 60 {
		color = "[yellow]"
	} else {
		color = "[green]"
	}

	// Create the bar
	bar := color
	for i := 0; i < filledWidth; i++ {
		bar += "█"
	}
	bar += "[white]"
	for i := filledWidth; i < width; i++ {
		bar += "░"
	}

	return bar
}

// Required export for plugin loading
var OhmyopsPlugin Plugin
