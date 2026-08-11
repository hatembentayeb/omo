package k8sportforward

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Workload is a Deployment or StatefulSet (or Service/Pod target).
type Workload struct {
	Kind      string
	Namespace string
	Name      string
	Ready     string
	Replicas  int32
	ReadyN    int32
	Type      string
	Ports     []int
	Labels    map[string]string
	Selector  map[string]string
}

// PodInfo is a runnable pod with container ports.
type PodInfo struct {
	Namespace string
	Name      string
	Phase     string
	Ready     bool
	Ports     []int
	Labels    map[string]string
	Type      string
}

// ServiceInfo is a Service with ports.
type ServiceInfo struct {
	Namespace string
	Name      string
	Type      string
	ClusterIP string
	Ports     []ServicePort
	Category  string
	Selector  map[string]string
}

// ServicePort pairs service port with optional target.
type ServicePort struct {
	Name       string
	Port       int32
	TargetPort int32
	Protocol   string
}

// NamespaceInfo is a non-system namespace summary.
type NamespaceInfo struct {
	Name   string
	Status string
	Age    string
}

// K8sClient wraps client-go access for listing and resolving pods.
type K8sClient struct {
	clientset  *kubernetes.Clientset
	restConfig *rest.Config
	context    string
	server     string
}

// Connect builds a client from kubeconfig settings.
// Priority: inline kubeconfig > kubeconfig path > KUBECONFIG / ~/.kube/config.
func (c *K8sClient) Connect(inline, path, contextName string) error {
	restConfig, ctxName, server, err := loadRESTConfig(inline, path, contextName)
	if err != nil {
		return err
	}
	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("create clientset: %w", err)
	}
	c.clientset = cs
	c.restConfig = restConfig
	c.context = ctxName
	c.server = server
	return nil
}

func (c *K8sClient) Connected() bool {
	return c != nil && c.clientset != nil && c.restConfig != nil
}

func (c *K8sClient) Context() string {
	if c == nil {
		return ""
	}
	return c.context
}

func (c *K8sClient) Server() string {
	if c == nil {
		return ""
	}
	return c.server
}

func (c *K8sClient) RESTConfig() *rest.Config {
	if c == nil {
		return nil
	}
	return c.restConfig
}

func (c *K8sClient) Clientset() *kubernetes.Clientset {
	if c == nil {
		return nil
	}
	return c.clientset
}

func loadRESTConfig(inline, path, contextName string) (*rest.Config, string, string, error) {
	var loader clientcmd.ClientConfig

	switch {
	case strings.TrimSpace(inline) != "" && (strings.Contains(inline, "apiVersion:") || strings.Contains(inline, "clusters:")):
		cc, err := clientcmd.NewClientConfigFromBytes([]byte(inline))
		if err != nil {
			return nil, "", "", fmt.Errorf("parse inline kubeconfig: %w", err)
		}
		if contextName != "" {
			raw, err := cc.RawConfig()
			if err != nil {
				return nil, "", "", err
			}
			loader = clientcmd.NewDefaultClientConfig(raw, &clientcmd.ConfigOverrides{
				CurrentContext: contextName,
			})
		} else {
			loader = cc
		}
	default:
		cfgPath := expandPath(firstNonEmpty(path, inline))
		if cfgPath == "" {
			cfgPath = defaultKubeconfigPath()
		}
		if _, err := os.Stat(cfgPath); err != nil {
			return nil, "", "", fmt.Errorf("kubeconfig not found at %s (set KUBECONFIG or KeePass kubeconfig)", cfgPath)
		}
		overrides := &clientcmd.ConfigOverrides{}
		if contextName != "" {
			overrides.CurrentContext = contextName
		}
		loader = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: cfgPath},
			overrides,
		)
	}

	restConfig, err := loader.ClientConfig()
	if err != nil {
		return nil, "", "", fmt.Errorf("build rest config: %w", err)
	}
	raw, err := loader.RawConfig()
	if err != nil {
		return nil, "", "", err
	}
	ctxName := contextName
	if ctxName == "" {
		ctxName = raw.CurrentContext
	}
	server := restConfig.Host
	if ctx, ok := raw.Contexts[ctxName]; ok && ctx != nil {
		if cl, ok := raw.Clusters[ctx.Cluster]; ok && cl != nil && cl.Server != "" {
			server = cl.Server
		}
	}
	return restConfig, ctxName, server, nil
}

// ListContexts returns available kubeconfig context names for the current file.
func ListContexts(path string) ([]string, string, error) {
	cfgPath := expandPath(path)
	if cfgPath == "" {
		cfgPath = defaultKubeconfigPath()
	}
	raw, err := clientcmd.LoadFromFile(cfgPath)
	if err != nil {
		return nil, "", err
	}
	names := make([]string, 0, len(raw.Contexts))
	for name := range raw.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, raw.CurrentContext, nil
}

func (c *K8sClient) ListUserNamespaces(ctx context.Context) ([]NamespaceInfo, error) {
	list, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var out []NamespaceInfo
	for _, ns := range list.Items {
		if isSystemNamespace(ns.Name) {
			continue
		}
		age := ""
		if !ns.CreationTimestamp.IsZero() {
			age = humanAge(time.Since(ns.CreationTimestamp.Time))
		}
		out = append(out, NamespaceInfo{
			Name:   ns.Name,
			Status: string(ns.Status.Phase),
			Age:    age,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (c *K8sClient) ListWorkloads(ctx context.Context, namespaceFilter string) ([]Workload, error) {
	namespaces, err := c.targetNamespaces(ctx, namespaceFilter)
	if err != nil {
		return nil, err
	}
	var out []Workload
	for _, ns := range namespaces {
		deps, err := c.clientset.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list deployments in %s: %w", ns, err)
		}
		for _, d := range deps.Items {
			out = append(out, workloadFromDeployment(d))
		}
		sts, err := c.clientset.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list statefulsets in %s: %w", ns, err)
		}
		for _, s := range sts.Items {
			out = append(out, workloadFromStatefulSet(s))
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (c *K8sClient) ListServices(ctx context.Context, namespaceFilter string) ([]ServiceInfo, error) {
	namespaces, err := c.targetNamespaces(ctx, namespaceFilter)
	if err != nil {
		return nil, err
	}
	var out []ServiceInfo
	for _, ns := range namespaces {
		svcs, err := c.clientset.CoreV1().Services(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list services in %s: %w", ns, err)
		}
		for _, svc := range svcs.Items {
			if svc.Spec.ClusterIP == corev1.ClusterIPNone && len(svc.Spec.Ports) == 0 {
				continue
			}
			info := ServiceInfo{
				Namespace: svc.Namespace,
				Name:      svc.Name,
				Type:      string(svc.Spec.Type),
				ClusterIP: svc.Spec.ClusterIP,
				Selector:  svc.Spec.Selector,
				Category:  classifyWorkload(svc.Name, svc.Labels),
			}
			for _, p := range svc.Spec.Ports {
				tp := p.TargetPort.IntVal
				if tp == 0 {
					tp = p.Port
				}
				info.Ports = append(info.Ports, ServicePort{
					Name:       p.Name,
					Port:       p.Port,
					TargetPort: tp,
					Protocol:   string(p.Protocol),
				})
			}
			out = append(out, info)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (c *K8sClient) ListPods(ctx context.Context, namespaceFilter string) ([]PodInfo, error) {
	namespaces, err := c.targetNamespaces(ctx, namespaceFilter)
	if err != nil {
		return nil, err
	}
	var out []PodInfo
	for _, ns := range namespaces {
		pods, err := c.clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("list pods in %s: %w", ns, err)
		}
		for _, p := range pods.Items {
			info := PodInfo{
				Namespace: p.Namespace,
				Name:      p.Name,
				Phase:     string(p.Status.Phase),
				Ready:     podReady(p),
				Labels:    p.Labels,
				Type:      classifyWorkload(p.Name, p.Labels),
				Ports:     containerPorts(p.Spec.Containers),
			}
			out = append(out, info)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// ResolvePod finds a ready pod for a workload / service / pod target.
func (c *K8sClient) ResolvePod(ctx context.Context, kind, namespace, name string) (string, error) {
	kind = strings.ToLower(kind)
	switch kind {
	case "pod", "pods", "po":
		pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		if pod.Status.Phase != corev1.PodRunning {
			return "", fmt.Errorf("pod %s/%s is %s", namespace, name, pod.Status.Phase)
		}
		return pod.Name, nil
	case "service", "services", "svc":
		svc, err := c.clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		if len(svc.Spec.Selector) == 0 {
			return "", fmt.Errorf("service %s/%s has no selector", namespace, name)
		}
		return c.pickReadyPod(ctx, namespace, svc.Spec.Selector)
	case "deployment", "deployments", "deploy":
		d, err := c.clientset.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return c.pickReadyPod(ctx, namespace, d.Spec.Selector.MatchLabels)
	case "statefulset", "statefulsets", "sts":
		s, err := c.clientset.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return "", err
		}
		return c.pickReadyPod(ctx, namespace, s.Spec.Selector.MatchLabels)
	default:
		return "", fmt.Errorf("unsupported kind %q", kind)
	}
}

func (c *K8sClient) pickReadyPod(ctx context.Context, namespace string, selector map[string]string) (string, error) {
	if len(selector) == 0 {
		return "", fmt.Errorf("empty selector in %s", namespace)
	}
	sel := labels.SelectorFromSet(selector).String()
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: sel})
	if err != nil {
		return "", err
	}
	var fallback string
	for _, p := range pods.Items {
		if p.DeletionTimestamp != nil {
			continue
		}
		if p.Status.Phase == corev1.PodRunning && podReady(p) {
			return p.Name, nil
		}
		if p.Status.Phase == corev1.PodRunning && fallback == "" {
			fallback = p.Name
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("no running pod matching selector in %s", namespace)
}

func (c *K8sClient) targetNamespaces(ctx context.Context, filter string) ([]string, error) {
	filter = strings.TrimSpace(filter)
	if filter != "" && filter != "*" && filter != "all" {
		if isSystemNamespace(filter) {
			return nil, fmt.Errorf("namespace %s is a system namespace", filter)
		}
		return []string{filter}, nil
	}
	nss, err := c.ListUserNamespaces(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(nss))
	for _, ns := range nss {
		out = append(out, ns.Name)
	}
	return out, nil
}

func workloadFromDeployment(d appsv1.Deployment) Workload {
	var replicas int32
	if d.Spec.Replicas != nil {
		replicas = *d.Spec.Replicas
	}
	ports := containerPortsFromPodTemplate(d.Spec.Template.Spec.Containers)
	sel := map[string]string{}
	if d.Spec.Selector != nil {
		sel = d.Spec.Selector.MatchLabels
	}
	return Workload{
		Kind:      "Deployment",
		Namespace: d.Namespace,
		Name:      d.Name,
		Ready:     fmt.Sprintf("%d/%d", d.Status.ReadyReplicas, replicas),
		Replicas:  replicas,
		ReadyN:    d.Status.ReadyReplicas,
		Type:      classifyWorkload(d.Name, d.Labels),
		Ports:     ports,
		Labels:    d.Labels,
		Selector:  sel,
	}
}

func workloadFromStatefulSet(s appsv1.StatefulSet) Workload {
	var replicas int32
	if s.Spec.Replicas != nil {
		replicas = *s.Spec.Replicas
	}
	ports := containerPortsFromPodTemplate(s.Spec.Template.Spec.Containers)
	sel := map[string]string{}
	if s.Spec.Selector != nil {
		sel = s.Spec.Selector.MatchLabels
	}
	return Workload{
		Kind:      "StatefulSet",
		Namespace: s.Namespace,
		Name:      s.Name,
		Ready:     fmt.Sprintf("%d/%d", s.Status.ReadyReplicas, replicas),
		Replicas:  replicas,
		ReadyN:    s.Status.ReadyReplicas,
		Type:      classifyWorkload(s.Name, s.Labels),
		Ports:     ports,
		Labels:    s.Labels,
		Selector:  sel,
	}
}

func containerPortsFromPodTemplate(containers []corev1.Container) []int {
	return containerPorts(containers)
}

func containerPorts(containers []corev1.Container) []int {
	seen := map[int]struct{}{}
	var out []int
	for _, c := range containers {
		for _, p := range c.Ports {
			port := int(p.ContainerPort)
			if port <= 0 {
				continue
			}
			if _, ok := seen[port]; ok {
				continue
			}
			seen[port] = struct{}{}
			out = append(out, port)
		}
	}
	sort.Ints(out)
	return out
}

func podReady(p corev1.Pod) bool {
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

func humanAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
