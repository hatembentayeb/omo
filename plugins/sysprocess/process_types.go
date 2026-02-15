package main

import (
	"fmt"
	"strings"
	"time"
)

// UserProcess represents a user-space process with enriched context
type UserProcess struct {
	PID        int32
	Name       string
	Username   string
	Cmdline    string
	Cwd        string
	Status     string
	CPUPercent float64
	MemPercent float64
	MemRSS     uint64
	MemVMS     uint64
	Threads    int32
	CreateTime int64
	PPID       int32
	Source     string            // What started this process (shell, systemd, docker, etc.)
	Ancestry   []AncestorProcess // Parent chain from root to this process
	Ports      []string          // Listening ports
	GitRepo    string            // Git repo name if working dir is inside a repo
	GitBranch  string            // Git branch name
	Warnings   []string          // Warning messages
}

// AncestorProcess represents a single process in the ancestry chain
type AncestorProcess struct {
	PID  int32
	Name string
}

// GetTableRow returns process data formatted for the main list table
func (p *UserProcess) GetTableRow() []string {
	started := "unknown"
	if p.CreateTime > 0 {
		created := time.Unix(p.CreateTime/1000, 0)
		elapsed := time.Since(created)
		started = formatDuration(elapsed) + " ago"
	}

	return []string{
		fmt.Sprintf("%d", p.PID),
		p.Name,
		p.Username,
		fmt.Sprintf("%.1f", p.CPUPercent),
		fmt.Sprintf("%.1f", p.MemPercent),
		p.Source,
		p.Status,
		started,
	}
}

// GetAncestryString returns the ancestry chain as a single-line arrow format
// e.g. "systemd (pid 1) → bash (pid 1234) → node (pid 5678)"
func (p *UserProcess) GetAncestryString() string {
	if len(p.Ancestry) == 0 {
		return "unknown"
	}
	parts := make([]string, len(p.Ancestry))
	for i, a := range p.Ancestry {
		parts[i] = fmt.Sprintf("%s (pid %d)", a.Name, a.PID)
	}
	return strings.Join(parts, " → ")
}

// GetAncestryTree returns the ancestry as an indented tree
func (p *UserProcess) GetAncestryTree() string {
	if len(p.Ancestry) == 0 {
		return "unknown"
	}
	var sb strings.Builder
	for i, a := range p.Ancestry {
		if i == 0 {
			sb.WriteString(fmt.Sprintf("%s (pid %d)\n", a.Name, a.PID))
		} else {
			indent := strings.Repeat("  ", i-1)
			if i == len(p.Ancestry)-1 {
				// Highlight the target process
				sb.WriteString(fmt.Sprintf("%s└─ [yellow]%s[white] (pid %d)", indent, a.Name, a.PID))
			} else {
				sb.WriteString(fmt.Sprintf("%s└─ %s (pid %d)\n", indent, a.Name, a.PID))
			}
		}
	}
	return sb.String()
}

// GetPortsString returns listening ports as a comma-separated string
func (p *UserProcess) GetPortsString() string {
	if len(p.Ports) == 0 {
		return "none"
	}
	return strings.Join(p.Ports, ", ")
}
