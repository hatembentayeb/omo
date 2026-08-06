package s3

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

// Service is the RPC-facing S3 backend (no tview).
type Service struct {
	mu            sync.Mutex
	client        *S3Client
	profile       string
	region        string
	accessKey     string
	secretKey     string
	endpoint      string
	currentBucket string
	currentPrefix string
	currentView   string
}

// NewService creates an S3 RPC service.
func NewService() *Service {
	return &Service{
		client:      NewS3Client(),
		region:      "us-east-1",
		currentView: s3ViewBuckets,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "s3",
		Version:     "1.2.0",
		Description: "Manage AWS S3 buckets",
		Author:      "HATMAN",
		License:     "Apache-2.0",
		Tags:        []string{"storage", "cloud", "aws"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/ohmyops-v2/plugins/s3",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("Service.Configure begin")

	if req.Settings == nil {
		return fmt.Errorf("missing settings")
	}
	s.profile = req.Settings["name"]
	s.accessKey = req.Settings["username"]
	s.secretKey = req.Settings["password"]
	s.endpoint = req.Settings["host"]
	if s.endpoint == "" {
		s.endpoint = req.Settings["url"]
	}
	// KeePass may store a real S3 endpoint in URL; region comes from custom attr.
	s.region = req.Settings["region"]
	if s.region == "" {
		s.region = "us-east-1"
	}
	// If host looks like a region (no scheme), treat it as region not endpoint.
	if s.endpoint != "" && !strings.Contains(s.endpoint, "://") && !strings.Contains(s.endpoint, ".") {
		s.region = s.endpoint
		s.endpoint = ""
	}
	if s.profile == "" && s.accessKey == "" {
		return fmt.Errorf("name (profile) or username (access key) required")
	}

	pluginrpc.RPCLog("Service.Configure profile=%s region=%s endpoint=%s", s.profile, s.region, s.endpoint)
	if s.client != nil && s.client.IsConnected() {
		s.client.Disconnect()
	}
	s.currentBucket = ""
	s.currentPrefix = ""
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
		viewID = s3ViewBuckets
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
	key := req.Payload["key"]

	if strings.HasPrefix(action, "goto_") {
		viewID := strings.TrimPrefix(action, "goto_")
		if viewID == s3ViewObjects {
			if key != "" && s.currentView == s3ViewBuckets {
				s.currentBucket = key
				s.currentPrefix = ""
			}
			if s.currentBucket == "" {
				return pluginrpc.ActionResult{OK: false, Message: "no bucket selected"}, nil
			}
		}
		if viewID == s3ViewBuckets {
			s.currentBucket = ""
			s.currentPrefix = ""
		}
		view, err := s.buildViewLocked(viewID)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "switched to " + viewID, Next: &view}, nil
	}

	switch action {
	case "refresh", "":
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "refreshed", Next: &view}, nil

	case "open_objects":
		if key == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no bucket selected"}, nil
		}
		s.currentBucket = key
		s.currentPrefix = ""
		view, err := s.buildViewLocked(s3ViewObjects)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "opened " + key, Next: &view}, nil

	case "navigate":
		name := key
		storage := req.Payload["col3"]
		if name == "../" {
			return s.navigateUpLocked()
		}
		if storage == "Directory" || strings.HasSuffix(name, "/") {
			s.currentPrefix = s.currentPrefix + name
			view, err := s.buildViewLocked(s3ViewObjects)
			if err != nil {
				return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
			}
			return pluginrpc.ActionResult{OK: true, Message: "cd " + s.currentPrefix, Next: &view}, nil
		}
		fullKey := s.currentPrefix + name
		body := fmt.Sprintf("Key: %s\nBucket: %s\nSize: %s\nLast Modified: %s\nStorage Class: %s\nProfile: %s",
			fullKey, s.currentBucket, req.Payload["col1"], req.Payload["col2"], storage, s.profile)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Object: " + name, ModalBody: body}, nil

	case "up":
		return s.navigateUpLocked()

	case "object_info":
		name := key
		if name == "" || name == "../" || req.Payload["col3"] == "Directory" {
			return pluginrpc.ActionResult{OK: false, Message: "no object selected"}, nil
		}
		fullKey := s.currentPrefix + name
		body := fmt.Sprintf("Key: %s\nBucket: %s\nSize: %s\nLast Modified: %s\nStorage Class: %s\nProfile: %s",
			fullKey, s.currentBucket, req.Payload["col1"], req.Payload["col2"], req.Payload["col3"], s.profile)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Object: " + name, ModalBody: body}, nil

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) navigateUpLocked() (pluginrpc.ActionResult, error) {
	if s.currentPrefix == "" {
		view, err := s.buildViewLocked(s3ViewBuckets)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "buckets", Next: &view}, nil
	}
	prefix := strings.TrimSuffix(s.currentPrefix, "/")
	lastSlash := strings.LastIndex(prefix, "/")
	if lastSlash >= 0 {
		s.currentPrefix = prefix[:lastSlash+1]
	} else {
		s.currentPrefix = ""
	}
	view, err := s.buildViewLocked(s3ViewObjects)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	return pluginrpc.ActionResult{OK: true, Message: "up", Next: &view}, nil
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		s.client.Disconnect()
	}
	return nil
}

func (s *Service) ensureConnectedLocked() error {
	if s.client != nil && s.client.IsConnected() {
		return nil
	}
	if s.profile == "" && s.accessKey == "" {
		return fmt.Errorf("not configured")
	}
	return s.client.Connect(s.profile, s.region, s.accessKey, s.secretKey, s.endpoint)
}
