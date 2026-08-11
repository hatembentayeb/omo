package k8sportforward

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/util/httpstream"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"omo/pkg/pluginrpc"
)

// ActiveForward is a live local→pod port forward.
type ActiveForward struct {
	ID         string
	Kind       string
	Namespace  string
	Name       string
	Pod        string
	LocalPort  int
	RemotePort int
	Status     string // forwarding | starting | error | stopped
	Error      string
	StartedAt  time.Time

	stopChan  chan struct{}
	readyChan chan struct{}
	doneChan  chan struct{}
	mu        sync.Mutex
}

// ForwardManager tracks concurrent port-forwards for one cluster connection.
type ForwardManager struct {
	mu       sync.Mutex
	forwards map[string]*ActiveForward // keyed by ID
	byTarget map[string][]string       // resourceKey → forward IDs
}

func NewForwardManager() *ForwardManager {
	return &ForwardManager{
		forwards: map[string]*ActiveForward{},
		byTarget: map[string][]string{},
	}
}

func (m *ForwardManager) List() []*ActiveForward {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*ActiveForward, 0, len(m.forwards))
	for _, f := range m.forwards {
		out = append(out, f.snapshot())
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].LocalPort < out[j].LocalPort
	})
	return out
}

func (m *ForwardManager) Get(id string) *ActiveForward {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := m.forwards[id]
	if f == nil {
		return nil
	}
	return f.snapshot()
}

func (m *ForwardManager) ForTarget(kind, namespace, name string) []*ActiveForward {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := m.byTarget[resourceKey(kind, namespace, name)]
	out := make([]*ActiveForward, 0, len(ids))
	for _, id := range ids {
		if f := m.forwards[id]; f != nil {
			out = append(out, f.snapshot())
		}
	}
	return out
}

func (m *ForwardManager) IsForwarding(kind, namespace, name string) bool {
	return len(m.ForTarget(kind, namespace, name)) > 0
}

func (m *ForwardManager) LocalPortsFor(kind, namespace, name string) []int {
	fs := m.ForTarget(kind, namespace, name)
	ports := make([]int, 0, len(fs))
	for _, f := range fs {
		ports = append(ports, f.LocalPort)
	}
	sort.Ints(ports)
	return ports
}

func (m *ForwardManager) UsedLocalPorts() map[int]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[int]string{}
	for _, f := range m.forwards {
		out[f.LocalPort] = f.ID
	}
	return out
}

func (m *ForwardManager) Count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.forwards)
}

// Start resolves the target to a pod and starts a port-forward.
// localPort 0 means auto-pick (prefer remote, then remote+10000, then ephemeral).
func (m *ForwardManager) Start(client *K8sClient, kind, namespace, name string, localPort, remotePort int) (*ActiveForward, error) {
	if client == nil || !client.Connected() {
		return nil, fmt.Errorf("not connected to cluster")
	}
	if remotePort <= 0 || remotePort > 65535 {
		return nil, fmt.Errorf("invalid remote port %d", remotePort)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	podName, err := client.ResolvePod(ctx, kind, namespace, name)
	if err != nil {
		return nil, err
	}

	chosenLocal := localPort
	if chosenLocal <= 0 {
		chosenLocal, err = findFreePort(remotePort)
		if err != nil {
			return nil, err
		}
	} else if !portFree(chosenLocal) {
		alt, ferr := findFreePort(remotePort)
		if ferr != nil {
			return nil, fmt.Errorf("local port %d in use; could not suggest alternative: %w", chosenLocal, ferr)
		}
		return nil, fmt.Errorf("local port %d in use; suggested free port: %d", chosenLocal, alt)
	}

	id := forwardID(kind, namespace, name, chosenLocal, remotePort)

	m.mu.Lock()
	if existing := m.forwards[id]; existing != nil {
		m.mu.Unlock()
		return existing.snapshot(), nil
	}
	// Also reject duplicate local port
	for _, f := range m.forwards {
		if f.LocalPort == chosenLocal {
			m.mu.Unlock()
			return nil, fmt.Errorf("local port %d already used by %s", chosenLocal, f.ID)
		}
	}

	fwd := &ActiveForward{
		ID:         id,
		Kind:       kind,
		Namespace:  namespace,
		Name:       name,
		Pod:        podName,
		LocalPort:  chosenLocal,
		RemotePort: remotePort,
		Status:     "starting",
		StartedAt:  time.Now(),
		stopChan:   make(chan struct{}),
		readyChan:  make(chan struct{}),
		doneChan:   make(chan struct{}),
	}
	m.forwards[id] = fwd
	rk := resourceKey(kind, namespace, name)
	m.byTarget[rk] = append(m.byTarget[rk], id)
	m.mu.Unlock()

	go m.runForward(client, fwd)

	select {
	case <-fwd.readyChan:
		fwd.mu.Lock()
		fwd.Status = "forwarding"
		fwd.mu.Unlock()
		pluginrpc.RPCLog("forward ready id=%s local=%d remote=%d pod=%s/%s", id, chosenLocal, remotePort, namespace, podName)
		return fwd.snapshot(), nil
	case <-fwd.doneChan:
		fwd.mu.Lock()
		errMsg := fwd.Error
		fwd.mu.Unlock()
		m.remove(id)
		if errMsg == "" {
			errMsg = "port-forward exited before ready"
		}
		return nil, fmt.Errorf("%s", errMsg)
	case <-time.After(45 * time.Second):
		m.Stop(id)
		return nil, fmt.Errorf("timeout waiting for port-forward %s", id)
	}
}

func (m *ForwardManager) runForward(client *K8sClient, fwd *ActiveForward) {
	defer close(fwd.doneChan)
	// Do not auto-remove on exit: keep error/stopped rows visible until Stop().

	restConfig := client.RESTConfig()
	cs := client.Clientset()
	reqURL := cs.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(fwd.Namespace).
		Name(fwd.Pod).
		SubResource("portforward").
		URL()

	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		fwd.setError(fmt.Sprintf("spdy roundtripper: %v", err))
		return
	}

	spdyDialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, reqURL)
	dialer := httpstream.Dialer(spdyDialer)
	// Prefer websocket when available (newer clusters), fall back to SPDY on upgrade failure.
	if wsDialer, wsErr := portforward.NewSPDYOverWebsocketDialer(reqURL, restConfig); wsErr == nil {
		dialer = portforward.NewFallbackDialer(wsDialer, spdyDialer, func(err error) bool {
			return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
		})
	}

	var errBuf strings.Builder
	ports := []string{fmt.Sprintf("%d:%d", fwd.LocalPort, fwd.RemotePort)}
	pf, err := portforward.NewOnAddresses(dialer, []string{"127.0.0.1"}, ports, fwd.stopChan, fwd.readyChan, io.Discard, &errBuf)
	if err != nil {
		fwd.setError(err.Error())
		return
	}

	if err := pf.ForwardPorts(); err != nil {
		select {
		case <-fwd.stopChan:
			fwd.mu.Lock()
			fwd.Status = "stopped"
			fwd.mu.Unlock()
			pluginrpc.RPCLog("forward stopped id=%s", fwd.ID)
		default:
			msg := err.Error()
			if extra := strings.TrimSpace(errBuf.String()); extra != "" {
				msg = msg + ": " + extra
			}
			fwd.setError(msg)
		}
		return
	}
	fwd.mu.Lock()
	if fwd.Status == "forwarding" || fwd.Status == "starting" {
		fwd.Status = "stopped"
	}
	fwd.mu.Unlock()
}

func (m *ForwardManager) Stop(id string) error {
	m.mu.Lock()
	fwd := m.forwards[id]
	m.mu.Unlock()
	if fwd == nil {
		return fmt.Errorf("forward %q not found", id)
	}
	select {
	case <-fwd.stopChan:
	default:
		close(fwd.stopChan)
	}
	select {
	case <-fwd.doneChan:
	case <-time.After(5 * time.Second):
	}
	m.remove(id)
	return nil
}

func (m *ForwardManager) StopTarget(kind, namespace, name string) int {
	fs := m.ForTarget(kind, namespace, name)
	n := 0
	for _, f := range fs {
		if err := m.Stop(f.ID); err == nil {
			n++
		}
	}
	return n
}

func (m *ForwardManager) StopAll() int {
	m.mu.Lock()
	ids := make([]string, 0, len(m.forwards))
	for id := range m.forwards {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	n := 0
	for _, id := range ids {
		if err := m.Stop(id); err == nil {
			n++
		}
	}
	return n
}

func (m *ForwardManager) remove(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fwd := m.forwards[id]
	if fwd == nil {
		return
	}
	delete(m.forwards, id)
	rk := resourceKey(fwd.Kind, fwd.Namespace, fwd.Name)
	ids := m.byTarget[rk]
	filtered := ids[:0]
	for _, x := range ids {
		if x != id {
			filtered = append(filtered, x)
		}
	}
	if len(filtered) == 0 {
		delete(m.byTarget, rk)
	} else {
		m.byTarget[rk] = filtered
	}
}

func (f *ActiveForward) snapshot() *ActiveForward {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *f
	return &cp
}

func (f *ActiveForward) setError(msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Status = "error"
	f.Error = msg
	pluginrpc.RPCLog("forward error id=%s err=%s", f.ID, msg)
}

func (f *ActiveForward) Address() string {
	return fmt.Sprintf("127.0.0.1:%d", f.LocalPort)
}

func (f *ActiveForward) Age() string {
	return humanAge(time.Since(f.StartedAt))
}

// SuggestLocalPort picks a free local port for a remote target port.
func SuggestLocalPort(remote int, used map[int]string) (int, error) {
	prefer := remote
	if prefer <= 0 {
		prefer = 0
	}
	candidates := []int{}
	if prefer > 0 {
		candidates = append(candidates, prefer, prefer+10000)
		if prefer < 30000 {
			candidates = append(candidates, prefer+20000)
		}
	}
	for _, c := range candidates {
		if c <= 0 || c > 65535 {
			continue
		}
		if _, taken := used[c]; taken {
			continue
		}
		if portFree(c) {
			return c, nil
		}
	}
	return findFreePort(prefer)
}

func parseLocalPortInput(s string) (int, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "auto" || s == "0" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 65535 {
		return 0, fmt.Errorf("invalid local port %q", s)
	}
	return n, nil
}

func parseRemotePortInput(s string, fallbacks []int) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		if len(fallbacks) > 0 {
			return fallbacks[0], nil
		}
		return 0, fmt.Errorf("remote port required")
	}
	// Allow "6379" or "6379/TCP"
	s = strings.Split(s, "/")[0]
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 || n > 65535 {
		return 0, fmt.Errorf("invalid remote port %q", s)
	}
	return n, nil
}

// keepalive ping against API to ensure client still works (optional).
func (c *K8sClient) Ping(ctx context.Context) error {
	if c == nil || !c.Connected() {
		return fmt.Errorf("not connected")
	}
	_, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	return err
}
