package docker

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

// Service is the RPC-facing docker backend (no tview).
type Service struct {
	mu          sync.Mutex
	client      *DockerClient
	hostURL     string
	hostName    string
	tls         bool
	tlsVerify   bool
	certPath    string
	currentView string
}

// NewService creates a docker RPC service.
func NewService() *Service {
	return &Service{
		client:      NewDockerClient(),
		currentView: viewContainers,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "docker",
		Version:     "2.0.0",
		Description: "Docker container, image, network, volume, and compose management",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"containers", "docker", "devops", "infrastructure", "compose"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/docker",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("Service.Configure begin")

	if req.Settings == nil {
		return fmt.Errorf("missing settings")
	}
	s.hostName = req.Settings["name"]
	s.hostURL = req.Settings["host"]
	if s.hostURL == "" {
		s.hostURL = req.Settings["url"]
	}
	s.certPath = req.Settings["cert_path"]
	s.tls = isTruthy(req.Settings["tls"])
	s.tlsVerify = isTruthy(req.Settings["tls_verify"])

	pluginrpc.RPCLog("Service.Configure name=%s host=%s", s.hostName, s.hostURL)

	host := DockerHost{
		Name:      s.hostName,
		Host:      s.hostURL,
		TLS:       s.tls,
		TLSVerify: s.tlsVerify,
		CertPath:  s.certPath,
	}
	return s.client.ConnectToHost(host)
}

func isTruthy(v string) bool {
	return v == "true" || v == "1" || v == "yes"
}

func (s *Service) GetView(req pluginrpc.ViewRequest) (pluginrpc.ViewData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	viewID := req.View
	if viewID == "" {
		viewID = s.currentView
	}
	if viewID == "" {
		viewID = viewContainers
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
		view, err := s.buildViewLocked(viewID)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "switched to " + viewID, Next: &view}, nil
	}

	id := req.Payload["key"]
	switch action {
	case "refresh", "":
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "refreshed", Next: &view}, nil

	case "start":
		if id == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no container selected"}, nil
		}
		if err := s.client.StartContainer(id); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewContainers)
		return pluginrpc.ActionResult{OK: true, Message: "started " + id, Next: &view}, nil

	case "stop":
		if id == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no container selected"}, nil
		}
		if err := s.client.StopContainer(id); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewContainers)
		return pluginrpc.ActionResult{OK: true, Message: "stopped " + id, Next: &view}, nil

	case "restart":
		if id == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no container selected"}, nil
		}
		if err := s.client.RestartContainer(id); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(s.currentView)
		return pluginrpc.ActionResult{OK: true, Message: "restarted " + id, Next: &view}, nil

	case "pause":
		if id == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no container selected"}, nil
		}
		if err := s.client.PauseContainer(id); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewContainers)
		return pluginrpc.ActionResult{OK: true, Message: "paused " + id, Next: &view}, nil

	case "unpause":
		if id == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no container selected"}, nil
		}
		if err := s.client.UnpauseContainer(id); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewContainers)
		return pluginrpc.ActionResult{OK: true, Message: "unpaused " + id, Next: &view}, nil

	case "kill":
		if id == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no container selected"}, nil
		}
		if err := s.client.KillContainer(id); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewContainers)
		return pluginrpc.ActionResult{OK: true, Message: "killed " + id, Next: &view}, nil

	case "delete":
		if id == "" {
			return pluginrpc.ActionResult{OK: false, Message: "nothing selected"}, nil
		}
		var err error
		switch s.currentView {
		case viewImages:
			err = s.client.RemoveImage(id)
		case viewNetworks:
			err = s.client.RemoveNetwork(id)
		case viewVolumes:
			err = s.client.RemoveVolume(id)
		case viewCompose:
			err = s.client.ComposeDown(id)
		default:
			err = s.client.RemoveContainer(id)
		}
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(s.currentView)
		return pluginrpc.ActionResult{OK: true, Message: "deleted " + id, Next: &view}, nil

	case "logs":
		if id == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no selection"}, nil
		}
		var body string
		var err error
		if s.currentView == viewCompose {
			body, err = s.client.ComposeLogs(id)
		} else {
			body, err = s.client.GetContainerLogs(id)
		}
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Logs: " + id, ModalBody: body}, nil

	case "inspect":
		if id == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no selection"}, nil
		}
		body, title, err := s.inspectLocked(id)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: title, ModalBody: body}, nil

	case "history":
		if id == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no image selected"}, nil
		}
		body, err := s.client.GetImageHistory(id)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: "History: " + id, ModalBody: body}, nil

	case "pull":
		name := id
		if repo := req.Payload["col1"]; repo != "" && repo != "<none>" {
			tag := req.Payload["col2"]
			if tag == "" || tag == "<none>" {
				tag = "latest"
			}
			name = repo + ":" + tag
		}
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no image to pull"}, nil
		}
		if err := s.client.PullImage(name); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewImages)
		return pluginrpc.ActionResult{OK: true, Message: "pulled " + name, Next: &view}, nil

	case "run":
		if id == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no image selected"}, nil
		}
		imageName := id
		if repo := req.Payload["col1"]; repo != "" && repo != "<none>" {
			tag := req.Payload["col2"]
			if tag == "" || tag == "<none>" {
				tag = "latest"
			}
			imageName = repo + ":" + tag
		}
		cid, err := s.client.CreateContainer(imageName, "")
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.StartContainer(cid); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewContainers)
		return pluginrpc.ActionResult{OK: true, Message: "started container from " + imageName, Next: &view}, nil

	case "prune":
		body, err := s.client.PruneVolumes()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewVolumes)
		return pluginrpc.ActionResult{OK: true, Message: body, Next: &view}, nil

	case "prune_system":
		body, err := s.client.PruneSystem()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewSystem)
		return pluginrpc.ActionResult{OK: true, Message: body, Next: &view, ModalTitle: "Prune Result", ModalBody: body}, nil

	case "disk_usage":
		du, err := s.client.GetDiskUsage()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		body := fmt.Sprintf("Images: %d (%s, reclaimable %s)\nContainers: %d (%s, reclaimable %s)\nVolumes: %d (%s, reclaimable %s)\nBuild Cache: %d (%s, reclaimable %s)\nTotal reclaimable: %s",
			du.ImagesCount, du.ImagesSize, du.ImagesReclaimable,
			du.ContainersCount, du.ContainersSize, du.ContainersReclaimable,
			du.VolumesCount, du.VolumesSize, du.VolumesReclaimable,
			du.BuildCacheCount, du.BuildCacheSize, du.BuildCacheReclaimable,
			du.TotalReclaimable)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Disk Usage", ModalBody: body}, nil

	case "events":
		body, err := s.client.GetRecentEvents()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Recent Events", ModalBody: body}, nil

	case "compose_up":
		if id == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no project selected"}, nil
		}
		if err := s.client.ComposeUp(id); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewCompose)
		return pluginrpc.ActionResult{OK: true, Message: "compose up " + id, Next: &view}, nil

	case "compose_stop":
		if id == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no project selected"}, nil
		}
		if err := s.client.ComposeStop(id); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewCompose)
		return pluginrpc.ActionResult{OK: true, Message: "compose stop " + id, Next: &view}, nil

	case "compose_restart":
		if id == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no project selected"}, nil
		}
		if err := s.client.ComposeRestart(id); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewCompose)
		return pluginrpc.ActionResult{OK: true, Message: "compose restart " + id, Next: &view}, nil

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return nil
}

func (s *Service) inspectLocked(id string) (body, title string, err error) {
	switch s.currentView {
	case viewImages:
		info, e := s.client.InspectImage(id)
		if e != nil {
			return "", "", e
		}
		return fmt.Sprintf("%+v", info), "Image: " + id, nil
	case viewNetworks:
		info, e := s.client.InspectNetwork(id)
		if e != nil {
			return "", "", e
		}
		return fmt.Sprintf("%+v", info), "Network: " + id, nil
	case viewVolumes:
		info, e := s.client.InspectVolume(id)
		if e != nil {
			return "", "", e
		}
		return fmt.Sprintf("%+v", info), "Volume: " + id, nil
	default:
		info, e := s.client.InspectContainer(id)
		if e != nil {
			return "", "", e
		}
		return fmt.Sprintf("%+v", info), "Container: " + id, nil
	}
}
