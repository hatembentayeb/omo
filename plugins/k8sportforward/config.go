package k8sportforward

import (
	"fmt"
	"sort"
	"strings"

	"omo/pkg/pluginapi"
)

// ClusterConfig is built from a KeePass entry at runtime.
//
// KeePass Entry Schema (path: k8sportforward/<environment>/<name>):
//
//	Title    → cluster display name
//	URL      → Kubernetes API server URL (optional when kubeconfig is set)
//	Notes    → description / notes
//
//	Custom Attributes:
//	  kubeconfig      → path to kubeconfig file (e.g. "~/.kube/config")
//	  kubeconfig_path → alias for kubeconfig
//	  context         → kubectl/client-go context name
//	  namespace       → optional default namespace filter
//	  tags            → comma-separated tags
type ClusterConfig struct {
	Name           string
	Description    string
	Environment    string
	Server         string
	Kubeconfig     string
	KubeconfigPath string
	Context        string
	Namespace      string
	Tags           []string
}

// DiscoverClusters reads KeePass groups under "k8sportforward/" (and falls
// back to "k8suser/" entries that already carry kubeconfig).
func DiscoverClusters() ([]ClusterConfig, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}
	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	paths, err := pluginapi.ListNonReferenceSecrets("k8sportforward")
	if err != nil || len(paths) == 0 {
		paths, err = pluginapi.ListNonReferenceSecrets("k8suser")
		if err != nil {
			return nil, fmt.Errorf("list k8sportforward/k8suser secrets: %w", err)
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no cluster entries (create under k8sportforward/<env>/<name> with kubeconfig)")
	}

	var clusters []ClusterConfig
	for _, path := range paths {
		entry, err := pluginapi.ResolveSecret(path)
		if err != nil || entry == nil {
			continue
		}
		env := extractEnvironment(path)
		c := ClusterConfig{
			Name:        entry.Title,
			Description: entry.Notes,
			Environment: env,
			Server:      entry.URL,
		}
		ca := entry.CustomAttributes
		if ca != nil {
			c.Kubeconfig = ca["kubeconfig"]
			c.KubeconfigPath = ca["kubeconfig_path"]
			c.Context = ca["context"]
			c.Namespace = ca["namespace"]
			c.Tags = splitTags(ca["tags"])
		}
		clusters = append(clusters, c)
	}
	sort.Slice(clusters, func(i, j int) bool {
		if clusters[i].Environment == clusters[j].Environment {
			return clusters[i].Name < clusters[j].Name
		}
		return clusters[i].Environment < clusters[j].Environment
	})
	return clusters, nil
}

func extractEnvironment(path string) string {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return "default"
	}
	return parts[1]
}

func splitTags(v string) []string {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	var out []string
	for _, t := range strings.Split(v, ",") {
		t = strings.TrimSpace(t)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
