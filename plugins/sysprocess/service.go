package sysprocess

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"

	"github.com/shirou/gopsutil/v3/process"
)

// Service is the RPC-facing sysprocess backend (no tview).
type Service struct {
	mu             sync.Mutex
	currentUser    string
	processes      []*UserProcess
	detailsProcess *UserProcess
	cpuCache       map[int32]float64
	portCache      map[int32][]string
	sortBy         string // "cpu" or "mem"
	currentView    string
	diskPath       string
	diskEntries    []diskEntry
}

// NewService creates a sysprocess RPC service.
func NewService() *Service {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/"
	}
	return &Service{
		currentUser: getCurrentUsername(),
		cpuCache:    make(map[int32]float64),
		portCache:   make(map[int32][]string),
		sortBy:      "cpu",
		currentView: viewProcesses,
		diskPath:    home,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "sysprocess",
		Version:     "2.0.0",
		Description: "User-space process monitor — why is this running?",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"process", "monitoring", "user", "witr"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "",
	}, nil
}

func (s *Service) Configure(_ pluginrpc.ConfigureRequest) error {
	pluginrpc.RPCLog("Service.Configure (noop — local plugin)")
	return nil
}

func (s *Service) GetView(req pluginrpc.ViewRequest) (pluginrpc.ViewData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	viewID := req.View
	if viewID == "" {
		viewID = s.currentView
	}
	if viewID == "" {
		viewID = viewProcesses
	}
	pluginrpc.RPCLog("Service.GetView begin view=%s", viewID)
	start := time.Now()
	view, err := s.buildViewLocked(viewID)
	if err != nil {
		pluginrpc.RPCLog("Service.GetView err=%v", err)
		return pluginrpc.ViewData{}, err
	}
	pluginrpc.RPCLog("Service.GetView OK view=%s rows=%d dur=%s", view.View, len(view.Rows), time.Since(start))
	return view, nil
}

func (s *Service) DoAction(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	action := req.Action
	if strings.HasPrefix(action, "goto_") {
		viewID := strings.TrimPrefix(action, "goto_")
		if viewID == viewDetails {
			if pid := req.Payload["key"]; pid != "" {
				s.setDetailsFromPIDLocked(pid)
			}
		}
		view, err := s.buildViewLocked(viewID)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "switched to " + viewID, Next: &view}, nil
	}

	switch action {
	case "refresh", "":
		s.loadProcessDataLocked()
		if s.currentView == viewDisk {
			s.scanDiskLocked(s.diskPath)
		}
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "refreshed", Next: &view}, nil

	case "kill":
		pidStr := req.Payload["key"]
		if pidStr == "" || pidStr == "?" {
			return pluginrpc.ActionResult{OK: false, Message: "no process selected"}, nil
		}
		pid64, err := strconv.ParseInt(pidStr, 10, 32)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: "invalid pid"}, nil
		}
		if err := syscall.Kill(int(pid64), syscall.SIGKILL); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		s.loadProcessDataLocked()
		view, _ := s.buildViewLocked(s.currentView)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("killed pid %d", pid64), Next: &view}, nil

	case "sort_cpu":
		s.sortBy = "cpu"
		s.sortProcessesLocked()
		view, _ := s.buildViewLocked(viewProcesses)
		return pluginrpc.ActionResult{OK: true, Message: "sorted by CPU", Next: &view}, nil

	case "sort_mem":
		s.sortBy = "mem"
		s.sortProcessesLocked()
		view, _ := s.buildViewLocked(viewProcesses)
		return pluginrpc.ActionResult{OK: true, Message: "sorted by memory", Next: &view}, nil

	case "details":
		pidStr := req.Payload["key"]
		if pidStr == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no process selected"}, nil
		}
		if !s.setDetailsFromPIDLocked(pidStr) {
			return pluginrpc.ActionResult{OK: false, Message: "process not found"}, nil
		}
		view, _ := s.buildViewLocked(viewDetails)
		return pluginrpc.ActionResult{OK: true, Message: "details", Next: &view}, nil

	case "disk_open":
		name := req.Payload["col1"]
		if name == "" {
			name = req.Payload["key"]
		}
		name = strings.TrimSuffix(strings.ReplaceAll(name, "[yellow]", ""), "[white]")
		name = strings.TrimSuffix(name, "/")
		if name == ".." {
			s.diskGoUpLocked()
		} else if name != "" && name != "-" {
			next := filepath.Join(s.diskPath, name)
			info, err := os.Stat(next)
			if err != nil || !info.IsDir() {
				return pluginrpc.ActionResult{OK: false, Message: "not a directory"}, nil
			}
			s.scanDiskLocked(next)
		}
		view, _ := s.buildViewLocked(viewDisk)
		return pluginrpc.ActionResult{OK: true, Message: "opened " + s.diskPath, Next: &view}, nil

	case "disk_up":
		s.diskGoUpLocked()
		view, _ := s.buildViewLocked(viewDisk)
		return pluginrpc.ActionResult{OK: true, Message: "up to " + s.diskPath, Next: &view}, nil

	case "jump_to_process":
		pidStr := req.Payload["key"]
		if pidStr == "" || pidStr == "?" {
			return pluginrpc.ActionResult{OK: false, Message: "no pid"}, nil
		}
		view, _ := s.buildViewLocked(viewProcesses)
		return pluginrpc.ActionResult{OK: true, Message: "jumped to process list (select PID " + pidStr + ")", Next: &view}, nil

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) Stop() error {
	return nil
}

func (s *Service) setDetailsFromPIDLocked(pidStr string) bool {
	pid64, err := strconv.ParseInt(pidStr, 10, 32)
	if err != nil {
		return false
	}
	pid := int32(pid64)
	for _, p := range s.processes {
		if p.PID == pid {
			s.enrichProcessLocked(p)
			s.detailsProcess = p
			return true
		}
	}
	proc, err := process.NewProcess(pid)
	if err != nil {
		return false
	}
	ports := getAllListeningPorts()
	up := buildUserProcess(proc, s.cpuCache, ports)
	if up == nil {
		return false
	}
	s.enrichProcessLocked(up)
	s.detailsProcess = up
	return true
}

func (s *Service) enrichProcessLocked(p *UserProcess) {
	if p == nil {
		return
	}
	s.portCache = getAllListeningPorts()
	p.Ports = s.portCache[p.PID]
	if p.Cwd != "" {
		p.GitRepo, p.GitBranch = findGitRepo(p.Cwd)
	}
	p.Warnings = getProcessWarnings(p)
}

func (s *Service) loadProcessDataLocked() {
	allProcs, err := process.Processes()
	if err != nil {
		pluginrpc.RPCLog("loadProcessData err=%v", err)
		return
	}
	var userProcs []*process.Process
	for _, proc := range allProcs {
		if isCurrentUserProcess(proc, s.currentUser) {
			userProcs = append(userProcs, proc)
		}
	}
	for _, proc := range userProcs {
		if percent, err := proc.CPUPercent(); err == nil {
			s.cpuCache[proc.Pid] = percent
		}
	}
	s.portCache = getAllListeningPorts()
	var processes []*UserProcess
	for _, proc := range userProcs {
		up := buildUserProcess(proc, s.cpuCache, s.portCache)
		if up != nil {
			processes = append(processes, up)
		}
	}
	s.processes = processes
	s.sortProcessesLocked()
}

func (s *Service) sortProcessesLocked() {
	if s.sortBy == "mem" {
		sort.Slice(s.processes, func(i, j int) bool {
			return s.processes[i].MemPercent > s.processes[j].MemPercent
		})
		return
	}
	sort.Slice(s.processes, func(i, j int) bool {
		return s.processes[i].CPUPercent > s.processes[j].CPUPercent
	})
}

func (s *Service) diskGoUpLocked() {
	if s.diskPath == "/" {
		return
	}
	s.scanDiskLocked(filepath.Dir(s.diskPath))
}

func (s *Service) scanDiskLocked(path string) {
	s.diskPath = path
	s.diskEntries = scanDirectoryEntries(path)
}
