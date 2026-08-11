package k8sportforward

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

// Service is the RPC-facing Kubernetes port-forward backend (no tview).
type Service struct {
	mu sync.Mutex

	client   *K8sClient
	forwards *ForwardManager

	name           string
	kubeconfig     string
	kubeconfigData string // inline
	contextName    string
	namespaceFilter string
	server         string

	currentView string
}

// NewService creates the port-forward RPC service.
func NewService() *Service {
	return &Service{
		client:      &K8sClient{},
		forwards:    NewForwardManager(),
		currentView: viewWorkloads,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "k8sportforward",
		Version:     "1.0.0",
		Description: "Port-forward Kubernetes Deployments, StatefulSets, Services, and Pods with multi-tunnel management",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"kubernetes", "networking", "port-forward", "devops"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/k8sportforward",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("Service.Configure begin")

	if req.Settings == nil {
		req.Settings = map[string]string{}
	}

	name := req.Settings["name"]
	server := firstNonEmpty(req.Settings["url"], req.Settings["host"], req.Settings["server"])
	contextName := req.Settings["context"]
	namespaceFilter := req.Settings["namespace"]
	kubeconfig := firstNonEmpty(req.Settings["kubeconfig"], req.Settings["kubeconfig_path"])
	kubeconfigData := ""
	if strings.Contains(kubeconfig, "apiVersion:") || strings.Contains(kubeconfig, "clusters:") {
		kubeconfigData = kubeconfig
		kubeconfig = ""
	}
	if raw := req.Settings["kubeconfig_data"]; raw != "" {
		kubeconfigData = raw
	}

	// Warm re-Configure with the same cluster must NOT tear down live tunnels.
	// The host re-calls Configure every time you switch back to this plugin.
	sameCluster := s.client != nil && s.client.Connected() &&
		s.kubeconfig == kubeconfig &&
		s.kubeconfigData == kubeconfigData &&
		s.contextName == contextName
	if sameCluster {
		s.name = name
		if server != "" {
			s.server = server
		}
		s.namespaceFilter = namespaceFilter
		pluginrpc.RPCLog("Service.Configure skip reconnect (same cluster); keeping %d forward(s)", s.forwards.Count())
		return nil
	}

	if s.forwards != nil {
		n := s.forwards.StopAll()
		pluginrpc.RPCLog("Service.Configure stopped %d forward(s) before reconnect", n)
	}
	s.forwards = NewForwardManager()
	s.client = &K8sClient{}

	s.name = name
	s.server = server
	s.contextName = contextName
	s.namespaceFilter = namespaceFilter
	s.kubeconfig = kubeconfig
	s.kubeconfigData = kubeconfigData

	pluginrpc.RPCLog("Service.Configure name=%s kubeconfig=%s context=%s ns=%s",
		s.name, s.kubeconfig, s.contextName, s.namespaceFilter)

	if err := s.client.Connect(s.kubeconfigData, s.kubeconfig, s.contextName); err != nil {
		pluginrpc.RPCLog("Service.Configure connect err=%v", err)
		return err
	}
	s.contextName = s.client.Context()
	if s.server == "" {
		s.server = s.client.Server()
	}
	if s.name == "" {
		s.name = s.contextName
	}
	pluginrpc.RPCLog("Service.Configure OK context=%s server=%s", s.contextName, s.server)
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
		viewID = viewWorkloads
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

	case "start_forward":
		payload := withView(req.Payload, req.View, s.currentView)
		return s.actionStartForwardLocked(payload)

	case "stop_forward":
		payload := withView(req.Payload, req.View, s.currentView)
		return s.actionStopForwardLocked(payload)

	case "stop_all_forwards":
		n := s.forwards.StopAll()
		view, _ := s.buildViewLocked(viewForwards)
		return pluginrpc.ActionResult{
			OK:      true,
			Message: fmt.Sprintf("stopped %d forward(s)", n),
			Next:    &view,
		}, nil

	case "filter_namespace":
		ns := strings.TrimSpace(firstNonEmpty(req.Payload["namespace"], req.Payload["key"], req.Payload["col0"]))
		if ns == "" || ns == "*" || strings.EqualFold(ns, "all") {
			s.namespaceFilter = ""
		} else {
			s.namespaceFilter = ns
		}
		view, err := s.buildViewLocked(viewWorkloads)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		msg := "showing all user namespaces"
		if s.namespaceFilter != "" {
			msg = "filtered to namespace " + s.namespaceFilter
		}
		return pluginrpc.ActionResult{OK: true, Message: msg, Next: &view}, nil

	case "clear_namespace_filter":
		s.namespaceFilter = ""
		view, _ := s.buildViewLocked(s.currentView)
		return pluginrpc.ActionResult{OK: true, Message: "cleared namespace filter", Next: &view}, nil

	case "view_details":
		payload := withView(req.Payload, req.View, s.currentView)
		return s.actionViewDetailsLocked(payload)

	case "connection_info":
		payload := withView(req.Payload, req.View, s.currentView)
		return s.actionConnectionInfoLocked(payload)

	case "suggest_port":
		payload := withView(req.Payload, req.View, s.currentView)
		return s.actionSuggestPortLocked(payload)

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.forwards != nil {
		s.forwards.StopAll()
	}
	pluginrpc.RPCLog("Service.Stop")
	return nil
}

func (s *Service) ensureConnectedLocked() error {
	if s.client != nil && s.client.Connected() {
		return nil
	}
	// Lazy connect using defaults (useful when activated without KeePass).
	s.client = &K8sClient{}
	if err := s.client.Connect(s.kubeconfigData, s.kubeconfig, s.contextName); err != nil {
		return err
	}
	s.contextName = s.client.Context()
	if s.server == "" {
		s.server = s.client.Server()
	}
	return nil
}

func (s *Service) actionStartForwardLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	if err := s.ensureConnectedLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}

	kind, namespace, name := resolveTarget(payload)
	if kind == "" || namespace == "" || name == "" {
		return pluginrpc.ActionResult{OK: false, Message: "select a workload, service, or pod first"}, nil
	}

	ports := parsePortList(firstNonEmpty(payload["ports"], payload["col6"], payload["col5"], payload["col4"]))
	remote, err := parseRemotePortInput(payload["remote_port"], ports)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}

	local, err := parseLocalPortInput(payload["local_port"])
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}

	fwd, err := s.forwards.Start(s.client, kind, namespace, name, local, remote)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}

	view, _ := s.buildViewLocked(s.currentView)
	return pluginrpc.ActionResult{
		OK: true,
		Message: fmt.Sprintf("forwarding %s/%s %d → 127.0.0.1:%d (use in redis/postgres as host=127.0.0.1 port=%d)",
			namespace, name, remote, fwd.LocalPort, fwd.LocalPort),
		Next: &view,
		ModalTitle: "Port Forward Active",
		ModalBody: fmt.Sprintf(
			"Target:  %s %s/%s\nPod:     %s\nRemote:  %d\nLocal:   127.0.0.1:%d\n\nKeePass tip for redis/postgres plugins:\n  URL  = 127.0.0.1\n  port = %d\n",
			kind, namespace, name, fwd.Pod, remote, fwd.LocalPort, fwd.LocalPort,
		),
	}, nil
}

func (s *Service) actionStopForwardLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	view := payload["view"]
	id := firstNonEmpty(payload["id"], payload["key"], payload["col0"])
	if view == viewPorts {
		id = firstNonEmpty(payload["id"], payload["col5"])
	}
	if view == viewForwards {
		id = firstNonEmpty(payload["id"], payload["key"], payload["col0"])
	}

	// From workloads/services/pods: stop by target.
	kind, namespace, name := resolveTarget(payload)
	if id == "" || id == fwdMarkActive || id == fwdMarkInactive || id == "○" {
		if kind != "" && namespace != "" && name != "" {
			n := s.forwards.StopTarget(kind, namespace, name)
			viewData, _ := s.buildViewLocked(s.currentView)
			return pluginrpc.ActionResult{
				OK:      true,
				Message: fmt.Sprintf("stopped %d forward(s) for %s/%s", n, namespace, name),
				Next:    &viewData,
			}, nil
		}
		return pluginrpc.ActionResult{OK: false, Message: "no forward selected"}, nil
	}

	if err := s.forwards.Stop(id); err != nil {
		if kind != "" && namespace != "" && name != "" {
			n := s.forwards.StopTarget(kind, namespace, name)
			viewData, _ := s.buildViewLocked(s.currentView)
			return pluginrpc.ActionResult{
				OK:      true,
				Message: fmt.Sprintf("stopped %d forward(s)", n),
				Next:    &viewData,
			}, nil
		}
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	viewData, _ := s.buildViewLocked(s.currentView)
	return pluginrpc.ActionResult{OK: true, Message: "stopped " + id, Next: &viewData}, nil
}

func (s *Service) actionViewDetailsLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	kind, namespace, name := resolveTarget(payload)
	if namespace == "" || name == "" {
		id := firstNonEmpty(payload["id"], payload["key"], payload["col0"])
		if f := s.forwards.Get(id); f != nil {
			body := fmt.Sprintf("ID:      %s\nStatus:  %s\nTarget:  %s %s/%s\nPod:     %s\nLocal:   127.0.0.1:%d\nRemote:  %d\nAge:     %s\nError:   %s\n",
				f.ID, f.Status, f.Kind, f.Namespace, f.Name, f.Pod, f.LocalPort, f.RemotePort, f.Age(), dash(f.Error))
			return pluginrpc.ActionResult{OK: true, ModalTitle: "Forward Details", ModalBody: body}, nil
		}
		return pluginrpc.ActionResult{OK: false, Message: "nothing selected"}, nil
	}

	fs := s.forwards.ForTarget(kind, namespace, name)
	var b strings.Builder
	fmt.Fprintf(&b, "Kind:      %s\nNamespace: %s\nName:      %s\n", kind, namespace, name)
	if ports := firstNonEmpty(payload["ports"], payload["col6"], payload["col5"]); ports != "" {
		fmt.Fprintf(&b, "Ports:     %s\n", ports)
	}
	if len(fs) == 0 {
		b.WriteString("\nNo active forwards.\nPress F to start one.\n")
	} else {
		b.WriteString("\nActive forwards:\n")
		for _, f := range fs {
			fmt.Fprintf(&b, "  %s  127.0.0.1:%d → %d  (%s)  pod=%s\n", fwdMarkActive, f.LocalPort, f.RemotePort, f.Status, f.Pod)
		}
		b.WriteString("\nPoint redis/postgres KeePass entries at 127.0.0.1 and the local port above.\n")
	}
	return pluginrpc.ActionResult{OK: true, ModalTitle: name, ModalBody: b.String()}, nil
}

func (s *Service) actionConnectionInfoLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	kind, namespace, name := resolveTarget(payload)
	fs := s.forwards.ForTarget(kind, namespace, name)
	if len(fs) == 0 {
		id := firstNonEmpty(payload["id"], payload["key"], payload["col0"])
		if f := s.forwards.Get(id); f != nil {
			fs = []*ActiveForward{f}
		}
	}
	if len(fs) == 0 {
		return pluginrpc.ActionResult{OK: false, Message: "no active forward for selection"}, nil
	}
	var b strings.Builder
	b.WriteString("Use these localhost ports with other OMO plugins (redis, postgres, …):\n\n")
	for _, f := range fs {
		fmt.Fprintf(&b, "127.0.0.1:%d  →  %s/%s:%d  [%s]\n", f.LocalPort, f.Namespace, f.Name, f.RemotePort, f.Status)
		fmt.Fprintf(&b, "  KeePass: URL=127.0.0.1  port=%d\n\n", f.LocalPort)
	}
	return pluginrpc.ActionResult{OK: true, ModalTitle: "Connection Info", ModalBody: b.String()}, nil
}

func (s *Service) actionSuggestPortLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	ports := parsePortList(firstNonEmpty(payload["ports"], payload["remote_port"], payload["col6"], payload["col5"]))
	remote := 0
	if len(ports) > 0 {
		remote = ports[0]
	}
	if r, err := strconv.Atoi(strings.TrimSpace(payload["remote_port"])); err == nil {
		remote = r
	}
	suggested, err := SuggestLocalPort(remote, s.forwards.UsedLocalPorts())
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	body := fmt.Sprintf("Remote port: %d\nSuggested local port: %d\n\nAddress: 127.0.0.1:%d\nFree: yes\n", remote, suggested, suggested)
	return pluginrpc.ActionResult{OK: true, ModalTitle: "Suggested Port", ModalBody: body}, nil
}

func withView(payload map[string]string, views ...string) map[string]string {
	if payload == nil {
		payload = map[string]string{}
	} else {
		cp := make(map[string]string, len(payload)+1)
		for k, v := range payload {
			cp[k] = v
		}
		payload = cp
	}
	if payload["view"] == "" {
		for _, v := range views {
			if strings.TrimSpace(v) != "" {
				payload["view"] = v
				break
			}
		}
	}
	return payload
}

func resolveTarget(payload map[string]string) (kind, namespace, name string) {
	viewHint := payload["view"]

	// Explicit fields win.
	kind = payload["kind"]
	namespace = payload["namespace"]
	name = payload["name"]

	switch viewHint {
	case viewForwards:
		if target := firstNonEmpty(payload["target"], payload["col2"]); target != "" {
			kind, namespace, name = parseTargetCell(target)
		}
	case viewPorts:
		// Local, Remote, Address, Target, Status, ID
		if target := payload["col3"]; target != "" {
			nsName := strings.SplitN(target, "/", 2)
			if len(nsName) == 2 {
				namespace, name = nsName[0], nsName[1]
			}
		}
		if id := firstNonEmpty(payload["id"], payload["col5"], payload["key"]); id != "" {
			if k, ns, n := parseForwardID(id); n != "" {
				kind, namespace, name = k, ns, n
			}
		}
	case viewNamespaces:
		namespace = firstNonEmpty(payload["key"], payload["col0"], namespace)
	default:
		// Workloads / Services / Pods share: Fwd, Kind, Namespace, Name, …
		kind = firstNonEmpty(kind, payload["col1"])
		namespace = firstNonEmpty(namespace, payload["col2"])
		name = firstNonEmpty(name, payload["col3"])
	}

	if kind == "" && namespace != "" && name != "" {
		kind = "Pod"
	}
	return kind, namespace, name
}

func parseTargetCell(target string) (kind, namespace, name string) {
	parts := strings.Fields(strings.TrimSpace(target))
	switch len(parts) {
	case 2:
		kind = parts[0]
		nsName := strings.SplitN(parts[1], "/", 2)
		if len(nsName) == 2 {
			return kind, nsName[0], nsName[1]
		}
	case 1:
		nsName := strings.SplitN(parts[0], "/", 2)
		if len(nsName) == 2 {
			return "", nsName[0], nsName[1]
		}
	}
	return "", "", ""
}

// parseForwardID parses "kind/ns/name:local->remote".
func parseForwardID(id string) (kind, namespace, name string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", "", ""
	}
	main := id
	if i := strings.IndexByte(id, ':'); i >= 0 {
		main = id[:i]
	}
	parts := strings.Split(main, "/")
	if len(parts) >= 3 {
		return parts[0], parts[1], parts[2]
	}
	return "", "", ""
}

func isKind(s string) bool {
	switch strings.ToLower(s) {
	case "deployment", "statefulset", "service", "pod", "deploy", "sts", "svc":
		return true
	default:
		return false
	}
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func ctxTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 45*time.Second)
}
