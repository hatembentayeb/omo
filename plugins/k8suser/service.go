package k8suser

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

// Service is the RPC-facing k8suser backend (no tview).
// View IDs: k8sViewUsers / k8sViewRoles in k8suser_view_nav.go.
type Service struct {
	mu          sync.Mutex
	client      *K8sClient
	certManager *CertManager
	name        string
	kubeconfig  string
	context     string
	server      string
	token       string
	currentView string
}

// NewService creates a k8suser RPC service.
func NewService() *Service {
	return &Service{
		client:      NewK8sClient(),
		certManager: NewCertManager(),
		currentView: k8sViewUsers,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "k8suser",
		Version:     "1.0.0",
		Description: "Kubernetes user and certificate management",
		Author:      "OhMyOps",
		License:     "MIT",
		Tags:        []string{"kubernetes", "security", "certificates", "users"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/k8suser",
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
	s.server = firstNonEmpty(req.Settings["url"], req.Settings["host"])
	s.token = req.Settings["password"]
	s.kubeconfig = req.Settings["kubeconfig"]
	s.context = req.Settings["context"]

	if s.kubeconfig != "" {
		s.client.KubeConfig = expandHome(s.kubeconfig)
		_ = os.Setenv("KUBECONFIG", s.client.KubeConfig)
	} else {
		if _, err := s.client.GetKubeConfig(); err != nil {
			pluginrpc.RPCLog("GetKubeConfig: %v", err)
		}
	}

	if s.context != "" {
		if err := s.client.SetContext(s.context); err != nil {
			pluginrpc.RPCLog("SetContext %s: %v", s.context, err)
			// Still accept configure; context may already be current.
			s.client.CurrentContext = s.context
		}
	} else if s.client.CurrentContext == "" {
		_, _ = s.client.GetKubeConfig()
	}

	pluginrpc.RPCLog("Service.Configure name=%s kubeconfig=%s context=%s", s.name, s.client.KubeConfig, s.client.CurrentContext)
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
		viewID = k8sViewUsers
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

	case "create_user":
		username := firstNonEmpty(req.Payload["username"], req.Payload["name"], req.Payload["key"])
		if username == "" {
			return pluginrpc.ActionResult{OK: false, Message: "username required"}, nil
		}
		user, err := s.client.CreateUser(username)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(k8sViewUsers)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("created user %s (expires %s)", user.Username, user.CertExpiry), Next: &view}, nil

	case "delete":
		if s.currentView == k8sViewRoles || req.View == k8sViewRoles {
			name := firstNonEmpty(req.Payload["name"], req.Payload["key"])
			ns := firstNonEmpty(req.Payload["namespace"], "default")
			if name == "" {
				return pluginrpc.ActionResult{OK: false, Message: "role name required"}, nil
			}
			if err := s.client.DeleteCustomRole(name, ns); err != nil {
				return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
			}
			view, _ := s.buildViewLocked(k8sViewRoles)
			return pluginrpc.ActionResult{OK: true, Message: "deleted role " + name, Next: &view}, nil
		}
		username := firstNonEmpty(req.Payload["username"], req.Payload["name"], req.Payload["key"])
		if username == "" {
			return pluginrpc.ActionResult{OK: false, Message: "username required"}, nil
		}
		if err := s.client.DeleteUser(username); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(k8sViewUsers)
		return pluginrpc.ActionResult{OK: true, Message: "deleted user " + username, Next: &view}, nil

	case "assign_role":
		username := firstNonEmpty(req.Payload["username"], req.Payload["name"], req.Payload["key"])
		namespace := firstNonEmpty(req.Payload["namespace"], "default")
		role := req.Payload["role"]
		if username == "" || role == "" {
			return pluginrpc.ActionResult{OK: false, Message: "username and role required"}, nil
		}
		if err := s.client.AssignRoleToUser(username, namespace, role); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(k8sViewUsers)
		return pluginrpc.ActionResult{OK: true, Message: fmt.Sprintf("assigned %s to %s in %s", role, username, namespace), Next: &view}, nil

	case "create_role":
		name := firstNonEmpty(req.Payload["name"], req.Payload["key"])
		namespace := firstNonEmpty(req.Payload["namespace"], "default")
		resources := firstNonEmpty(req.Payload["resources"], "pods")
		verbs := firstNonEmpty(req.Payload["verbs"], "get,list,watch")
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "name required"}, nil
		}
		rules := []map[string]interface{}{
			{
				"apiGroups": []string{""},
				"resources": splitCSV(resources),
				"verbs":     splitCSV(verbs),
			},
		}
		if err := s.client.CreateCustomRole(name, namespace, rules); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(k8sViewRoles)
		return pluginrpc.ActionResult{OK: true, Message: "created role " + name, Next: &view}, nil

	case "view_details":
		return s.viewDetailsLocked(req)

	case "test_access":
		username := firstNonEmpty(req.Payload["username"], req.Payload["name"], req.Payload["key"])
		namespace := firstNonEmpty(req.Payload["namespace"], "default")
		resource := firstNonEmpty(req.Payload["resource"], "pods")
		verb := firstNonEmpty(req.Payload["verb"], "get")
		if username == "" {
			return pluginrpc.ActionResult{OK: false, Message: "username required"}, nil
		}
		ok, result, err := s.client.TestAccess(username, namespace, resource, verb)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{
			OK:         true,
			ModalTitle: "Access Test: " + username,
			ModalBody:  fmt.Sprintf("can-i %s %s -n %s → %v\n%s", verb, resource, namespace, ok, result),
		}, nil

	case "connection_command":
		username := firstNonEmpty(req.Payload["username"], req.Payload["name"], req.Payload["key"])
		if username == "" {
			return pluginrpc.ActionResult{OK: false, Message: "username required"}, nil
		}
		certInfo, err := s.certManager.GetCertificateInfo(username)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		body := fmt.Sprintf("User: %s\nCert: %s\nKey: %s\nExpiry: %s\n\nkubectl --client-certificate=%s --client-key=%s get pods",
			username, certInfo.Cert, certInfo.PrivateKey, certInfo.ExpiryDate.Format("2006-01-02"),
			certInfo.Cert, certInfo.PrivateKey)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Connection: " + username, ModalBody: body}, nil

	case "download_cert":
		return pluginrpc.ActionResult{
			OK:      true,
			Message: "cert download / KeePass UX not yet supported — use create_user",
		}, nil

	case "set_context":
		ctx := firstNonEmpty(req.Payload["context"], req.Payload["key"], req.Payload["name"])
		if ctx == "" {
			return pluginrpc.ActionResult{OK: false, Message: "context required"}, nil
		}
		if err := s.client.SetContext(ctx); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		s.context = ctx
		view, _ := s.buildViewLocked(k8sViewUsers)
		return pluginrpc.ActionResult{OK: true, Message: "switched to context " + ctx, Next: &view}, nil

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) viewDetailsLocked(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	if s.currentView == k8sViewRoles || req.View == k8sViewRoles {
		name := firstNonEmpty(req.Payload["name"], req.Payload["key"])
		ns := firstNonEmpty(req.Payload["namespace"], "cluster-wide")
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "role name required"}, nil
		}
		var cmd *exec.Cmd
		if ns == "cluster-wide" {
			cmd = exec.Command("kubectl", "get", "clusterrole", name, "-o", "yaml")
		} else {
			cmd = exec.Command("kubectl", "get", "role", name, "-n", ns, "-o", "yaml")
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: fmt.Sprintf("%v: %s", err, out)}, nil
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Role: " + name, ModalBody: string(out)}, nil
	}

	username := firstNonEmpty(req.Payload["username"], req.Payload["name"], req.Payload["key"])
	if username == "" {
		return pluginrpc.ActionResult{OK: false, Message: "username required"}, nil
	}
	users, err := s.client.GetUsers()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	for _, u := range users {
		if u.Username == username {
			body := fmt.Sprintf("Username: %s\nNamespaces: %s\nRoles: %s\nCert Expiry: %s",
				u.Username, u.Namespace, u.Roles, u.CertExpiry)
			return pluginrpc.ActionResult{OK: true, ModalTitle: "User: " + username, ModalBody: body}, nil
		}
	}
	return pluginrpc.ActionResult{OK: false, Message: "user not found: " + username}, nil
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return home + path[1:]
	}
	return path
}
