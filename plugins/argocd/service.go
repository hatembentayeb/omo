package argocd

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

const (
	viewApplications = "applications"
	viewProjects     = "projects"
	viewAccounts     = "accounts"
	viewRBAC         = "rbac"
	viewRBACPolicies = "rbac_policies"
	viewRBACGroups   = "rbac_groups"
)

// Service is the RPC-facing ArgoCD backend (no tview).
type Service struct {
	mu          sync.Mutex
	client      *ArgoAPIClient
	k8sClient   *K8sClient
	url         string
	username    string
	password    string
	token       string
	name        string
	namespace   string
	kubeconfig  string
	kubePath    string
	currentView string
	rbacData    RBACConfig
}

// NewService creates an ArgoCD RPC service.
func NewService() *Service {
	return &Service{
		client:      NewArgoAPIClient(DefaultArgocdConfig()),
		namespace:   "argocd",
		currentView: viewApplications,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "argocd",
		Version:     "1.0.0",
		Description: "ArgoCD management plugin",
		Author:      "OhMyOps",
		License:     "MIT",
		Tags:        []string{"argocd", "gitops", "kubernetes", "deployment"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/argocd",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("Service.Configure begin")

	if req.Settings == nil {
		return fmt.Errorf("missing settings")
	}

	s.name = req.Settings["name"]
	s.url = firstNonEmpty(req.Settings["url"], req.Settings["host"])
	s.username = req.Settings["username"]
	s.password = req.Settings["password"]
	s.token = firstNonEmpty(req.Settings["auth_token"], req.Settings["token"])
	s.kubeconfig = req.Settings["kubeconfig"]
	s.kubePath = req.Settings["kubeconfig_path"]
	if ns := req.Settings["namespace"]; ns != "" {
		s.namespace = ns
	}
	if s.url == "" {
		return fmt.Errorf("url/host is required")
	}

	pluginrpc.RPCLog("Service.Configure url=%s user=%s token=%v", s.url, s.username, s.token != "")

	s.client = NewArgoAPIClient(DefaultArgocdConfig())
	s.k8sClient = nil
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
		viewID = viewApplications
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

	switch action {
	case "refresh", "":
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "refreshed", Next: &view}, nil

	case "sync":
		name := payloadName(req.Payload)
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no application selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.SyncApplication(name); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewApplications)
		return pluginrpc.ActionResult{OK: true, Message: "synced " + name, Next: &view}, nil

	case "refresh_app":
		name := payloadName(req.Payload)
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no application selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.RefreshApplication(name); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewApplications)
		return pluginrpc.ActionResult{OK: true, Message: "refreshed " + name, Next: &view}, nil

	case "delete":
		return s.deleteLocked(req)

	case "view_details":
		return s.viewDetailsLocked(req)

	case "create_account":
		name := req.Payload["name"]
		if name == "" {
			name = req.Payload["key"]
		}
		password := req.Payload["password"]
		if name == "" || password == "" {
			return pluginrpc.ActionResult{OK: false, Message: "name and password required"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		caps := []string{"login", "apiKey"}
		if c := req.Payload["capabilities"]; c != "" {
			caps = strings.Split(c, ",")
			for i := range caps {
				caps[i] = strings.TrimSpace(caps[i])
			}
		}
		if err := s.client.CreateAccount(Account{Name: name, Enabled: true, Capabilities: caps}, password); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewAccounts)
		return pluginrpc.ActionResult{OK: true, Message: "created account " + name, Next: &view}, nil

	case "create_token":
		name := payloadName(req.Payload)
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no account selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		hours := 24
		if h := req.Payload["expires_hours"]; h != "" {
			if n, err := strconv.Atoi(h); err == nil && n > 0 {
				hours = n
			}
		}
		tok, err := s.client.CreateToken(name, hours)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{
			OK:         true,
			ModalTitle: "Token for " + name,
			ModalBody:  fmt.Sprintf("Token: %s\nIssued: %s\nExpires: %s", tok.Token, tok.FormatIssuedAt(), tok.FormatExpiresAt()),
		}, nil

	case "create_project":
		name := req.Payload["name"]
		if name == "" {
			name = req.Payload["key"]
		}
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "name required"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		proj := &Project{
			Name:        name,
			Description: req.Payload["description"],
			SourceRepos: []string{"*"},
			Destinations: []Destination{{
				Server:    "*",
				Namespace: "*",
			}},
		}
		if err := s.client.CreateProject(proj); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(viewProjects)
		return pluginrpc.ActionResult{OK: true, Message: "created project " + name, Next: &view}, nil

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) deleteLocked(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	name := payloadName(req.Payload)
	if name == "" {
		return pluginrpc.ActionResult{OK: false, Message: "nothing selected"}, nil
	}
	if err := s.ensureConnectedLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}

	viewID := s.currentView
	if req.View != "" {
		viewID = req.View
	}

	var err error
	switch viewID {
	case viewProjects:
		err = s.client.DeleteProject(name)
	case viewAccounts:
		err = s.client.DeleteAccount(name)
	default:
		err = s.client.DeleteApplication(name)
		viewID = viewApplications
	}
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	view, _ := s.buildViewLocked(viewID)
	return pluginrpc.ActionResult{OK: true, Message: "deleted " + name, Next: &view}, nil
}

func (s *Service) viewDetailsLocked(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	name := payloadName(req.Payload)
	if name == "" {
		return pluginrpc.ActionResult{OK: false, Message: "nothing selected"}, nil
	}
	if err := s.ensureConnectedLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}

	viewID := s.currentView
	if req.View != "" {
		viewID = req.View
	}

	switch viewID {
	case viewProjects:
		proj, err := s.client.GetProject(name)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		body := fmt.Sprintf("Name: %s\nDescription: %s\nSourceRepos: %d\nDestinations: %d\nRoles: %d",
			proj.Name, proj.Description, len(proj.SourceRepos), len(proj.Destinations), len(proj.Roles))
		for _, r := range proj.Roles {
			body += fmt.Sprintf("\n  role %s: %s", r.Name, r.Description)
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Project: " + name, ModalBody: body}, nil

	case viewAccounts:
		acct, err := s.client.GetAccount(name)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		body := fmt.Sprintf("Name: %s\nEnabled: %v\nCapabilities: %s\nTokens: %d",
			acct.Name, acct.Enabled, strings.Join(acct.Capabilities, ", "), len(acct.Tokens))
		for _, t := range acct.Tokens {
			body += fmt.Sprintf("\n  token %s issued=%s expires=%s", t.ID, t.FormatIssuedAt(), t.FormatExpiresAt())
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Account: " + name, ModalBody: body}, nil

	case viewRBAC, viewRBACPolicies, viewRBACGroups:
		return pluginrpc.ActionResult{
			OK:         true,
			ModalTitle: "RBAC: " + name,
			ModalBody:  "Selected: " + name + "\n(RBAC mutations requiring multi-field forms are skipped in RPC; use native plugin or edit ConfigMaps)",
		}, nil

	default:
		app, err := s.client.GetApplication(name)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		body := fmt.Sprintf("Name: %s\nProject: %s\nHealth: %s\nSync: %s\nNamespace: %s\nServer: %s",
			app.Name, app.Project, app.Health.Status, app.Sync.Status, app.Namespace, app.Server)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Application: " + name, ModalBody: body}, nil
	}
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		s.client.IsConnected = false
		s.client.Token = ""
	}
	s.k8sClient = nil
	return nil
}

func (s *Service) ensureConnectedLocked() error {
	if s.client != nil && s.client.IsConnected && s.client.Token != "" {
		return nil
	}
	if s.url == "" {
		return fmt.Errorf("not configured (host did not call Configure)")
	}
	pluginrpc.RPCLog("ensureConnected: dialing %s …", s.url)

	base := s.url
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	s.client.BaseURL = base

	if s.token != "" {
		s.client.Token = s.token
		s.client.Username = s.username
		s.client.IsConnected = true
	} else {
		if err := s.client.Connect(s.url, s.username, s.password); err != nil {
			return err
		}
	}

	inst := ArgocdInstance{
		Name:           s.name,
		URL:            s.url,
		Username:       s.username,
		Password:       s.password,
		AuthToken:      s.token,
		Kubeconfig:     s.kubeconfig,
		KubeconfigPath: s.kubePath,
		Namespace:      s.namespace,
	}
	if HasKubeconfig(inst) {
		k8s, err := NewK8sClient(inst)
		if err != nil {
			pluginrpc.RPCLog("k8s client init failed: %v", err)
			s.k8sClient = nil
		} else {
			s.k8sClient = k8s
		}
	}
	return nil
}

func payloadName(p map[string]string) string {
	if p == nil {
		return ""
	}
	for _, k := range []string{"name", "key", "Name", "Username"} {
		if v := p[k]; v != "" {
			return v
		}
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
