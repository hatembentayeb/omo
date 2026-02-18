package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// getCurrentUsername returns the current OS user's username
func getCurrentUsername() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return os.Getenv("USER")
}

// isUserProcess checks if a process is a user-space process (not a kernel thread)
func isUserProcess(proc *process.Process) bool {
	// Skip init (PID 1) and kthreadd (PID 2)
	if proc.Pid <= 2 {
		return false
	}

	// Kernel threads have empty command lines
	cmdline, _ := proc.Cmdline()
	if cmdline == "" {
		return false
	}

	// Children of kthreadd (PID 2) are kernel threads
	ppid, err := proc.Ppid()
	if err == nil && ppid == 2 {
		return false
	}

	// Filter known kernel thread name prefixes
	name, _ := proc.Name()
	nameLower := strings.ToLower(name)

	kernelPrefixes := []string{
		"kthreadd", "ksoftirqd", "kworker", "migration", "watchdog",
		"cpuhp", "netns", "rcu_", "kdevtmpfs", "kauditd",
		"oom_reaper", "writeback", "kcompactd", "ksmd", "khugepaged",
		"kintegrityd", "kblockd", "blkcg", "devfreq", "kswapd",
		"ecryptfs", "kthrotld", "irq/", "scsi_", "dm-",
		"jbd2", "ext4", "xfs", "btrfs",
	}

	for _, prefix := range kernelPrefixes {
		if strings.HasPrefix(nameLower, prefix) {
			return false
		}
	}

	return true
}

// isCurrentUserProcess checks if a process belongs to the current user and is user-space
func isCurrentUserProcess(proc *process.Process, currentUser string) bool {
	if !isUserProcess(proc) {
		return false
	}

	username, err := proc.Username()
	if err != nil {
		return false
	}

	return username == currentUser
}

// getProcessAncestry builds the parent chain from root down to the given process
func getProcessAncestry(proc *process.Process) []AncestorProcess {
	var chain []AncestorProcess
	visited := make(map[int32]bool)

	current := proc
	for current != nil {
		if visited[current.Pid] {
			break
		}
		visited[current.Pid] = true

		name, _ := current.Name()
		chain = append([]AncestorProcess{{PID: current.Pid, Name: name}}, chain...)

		if current.Pid <= 1 {
			break
		}

		parent, err := current.Parent()
		if err != nil || parent == nil {
			// Fallback: try via PPID
			ppid, err := current.Ppid()
			if err != nil || ppid <= 0 || ppid == current.Pid {
				break
			}
			parent, err = process.NewProcess(ppid)
			if err != nil {
				break
			}
		}
		current = parent
	}

	return chain
}

type sourceRule struct {
	match    func(name string) bool
	source   string
	fallback bool
}

var sourceRules = []sourceRule{
	{match: func(n string) bool { return strings.Contains(n, "docker") || strings.Contains(n, "containerd-shim") }, source: "docker"},
	{match: func(n string) bool { return strings.Contains(n, "podman") }, source: "podman"},
	{match: func(n string) bool { return strings.HasPrefix(n, "pm2") }, source: "pm2"},
	{match: func(n string) bool { return n == "supervisord" || n == "supervisor" }, source: "supervisord"},
	{match: func(n string) bool { return n == "crond" || n == "cron" || n == "anacron" || n == "atd" }, source: "cron"},
	{match: func(n string) bool { return n == "sshd" }, source: "ssh"},
	{match: func(n string) bool { return strings.HasPrefix(n, "tmux") }, source: "tmux"},
	{match: func(n string) bool { return n == "screen" }, source: "screen"},
	{match: func(n string) bool { return n == "code" || strings.Contains(n, "cursor") || strings.Contains(n, "electron") }, source: "IDE"},
	{match: func(n string) bool { return n == "bash" || n == "zsh" || n == "fish" || n == "sh" || n == "dash" }, source: "shell", fallback: true},
	{match: func(n string) bool { return n == "systemd" || n == "init" }, source: "systemd", fallback: true},
}

// detectSource determines the primary supervisor/source that started a process
// by walking its ancestry chain. The target process (last element) is excluded.
func detectSource(ancestry []AncestorProcess) string {
	source := "unknown"

	limit := len(ancestry) - 1
	if limit <= 0 {
		return source
	}

	for i := 0; i < limit; i++ {
		nameLower := strings.ToLower(ancestry[i].Name)

		for _, rule := range sourceRules {
			if !rule.match(nameLower) {
				continue
			}
			if rule.fallback {
				if rule.source == "shell" && (source == "unknown" || source == "systemd") {
					source = "shell (" + ancestry[i].Name + ")"
				} else if rule.source == "systemd" && source == "unknown" {
					source = "systemd"
				}
			} else {
				source = rule.source
			}
			break
		}
	}

	return source
}

// getAllListeningPorts uses gopsutil net.Connections (pure Go, no exec)
// Returns a map of PID -> listening addresses (TCP + UDP)
func getAllListeningPorts() map[int32][]string {
	result := make(map[int32][]string)

	for _, kind := range []string{"tcp", "udp"} {
		conns, err := net.Connections(kind)
		if err != nil {
			continue
		}

		for _, c := range conns {
			// TCP: only LISTEN state; UDP: include bound sockets (local port set)
			if kind == "tcp" && c.Status != "LISTEN" {
				continue
			}

			addr := formatListenAddr(c.Laddr.IP, c.Laddr.Port)
			pid := c.Pid
			// Pid 0 = gopsutil couldn't resolve (no permission for other users' /proc)
			// Include anyway so user sees ALL ports — will show "?" for process info
			result[pid] = append(result[pid], addr)
		}
	}

	return result
}

// formatListenAddr formats Laddr as "ip:port" or "*:port" for 0.0.0.0/::/empty
func formatListenAddr(ip string, port uint32) string {
	switch {
	case ip == "" || ip == "0.0.0.0" || ip == "::" || ip == "*":
		return fmt.Sprintf("*:%d", port)
	default:
		return fmt.Sprintf("%s:%d", ip, port)
	}
}

// findGitRepo looks for a .git directory starting from dir and walking up
func findGitRepo(dir string) (repoName string, branch string) {
	if dir == "" {
		return "", ""
	}

	current := dir
	for i := 0; i < 10 && current != "/" && current != "" && current != "."; i++ {
		gitPath := filepath.Join(current, ".git")
		info, err := os.Stat(gitPath)
		if err != nil {
			current = filepath.Dir(current)
			continue
		}

		repoName = filepath.Base(current)

		if info.IsDir() {
			headFile := filepath.Join(gitPath, "HEAD")
			data, err := os.ReadFile(headFile)
			if err == nil {
				head := strings.TrimSpace(string(data))
				if strings.HasPrefix(head, "ref: refs/heads/") {
					branch = strings.TrimPrefix(head, "ref: refs/heads/")
				} else if len(head) >= 8 {
					branch = head[:8] + " (detached)"
				}
			}
		}

		return repoName, branch
	}

	return "", ""
}

// getProcessWarnings detects warning conditions for a process
func getProcessWarnings(p *UserProcess) []string {
	var warnings []string

	if p.Username == "root" {
		warnings = append(warnings, "Running as root")
	}

	if p.MemRSS > 1024*1024*1024 { // > 1GB RSS
		warnings = append(warnings, fmt.Sprintf("High memory: %s RSS", formatBytes(p.MemRSS)))
	}

	if p.CPUPercent > 50 {
		warnings = append(warnings, fmt.Sprintf("High CPU: %.1f%%", p.CPUPercent))
	}

	if p.CreateTime > 0 {
		created := time.Unix(p.CreateTime/1000, 0)
		uptime := time.Since(created)
		if uptime > 90*24*time.Hour {
			warnings = append(warnings, fmt.Sprintf("Running for %d days", int(uptime.Hours()/24)))
		}
	}

	for _, port := range p.Ports {
		if strings.HasPrefix(port, "0.0.0.0:") || strings.HasPrefix(port, "*:") {
			warnings = append(warnings, fmt.Sprintf("Public bind: %s", port))
			break
		}
	}

	return warnings
}

// formatBytes formats bytes into a human-readable string
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

// truncateString truncates a string to maxLen characters with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

// createBarGraph renders a horizontal bar for value/max (0-100 scale)
func createBarGraph(value, max float64, width int) string {
	if max <= 0 {
		return strings.Repeat("░", width)
	}
	filled := int((value / max) * float64(width))
	if filled > width {
		filled = width
	}
	color := "[green]"
	if value >= 80 {
		color = "[red]"
	} else if value >= 60 {
		color = "[yellow]"
	}
	bar := color + strings.Repeat("█", filled) + "[white]" + strings.Repeat("░", width-filled)
	return bar
}

// formatDuration formats a duration into a human-readable string like "2d 5h 30m"
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
