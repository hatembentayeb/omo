package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// K8sClient wraps a Kubernetes clientset scoped to an ArgoCD namespace.
type K8sClient struct {
	clientset *kubernetes.Clientset
	namespace string
}

// NewK8sClient builds a Kubernetes client from the instance's kubeconfig
// settings. Priority: inline kubeconfig > kubeconfig_path > ~/.kube/config.
func NewK8sClient(inst ArgocdInstance) (*K8sClient, error) {
	var config clientcmd.ClientConfig

	switch {
	case inst.Kubeconfig != "":
		cc, err := clientcmd.NewClientConfigFromBytes([]byte(inst.Kubeconfig))
		if err != nil {
			return nil, fmt.Errorf("parse inline kubeconfig: %w", err)
		}
		config = cc

	case inst.KubeconfigPath != "":
		path := expandPath(inst.KubeconfigPath)
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("kubeconfig file not found: %s", path)
		}
		config = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: path},
			&clientcmd.ConfigOverrides{},
		)

	default:
		home, _ := os.UserHomeDir()
		defaultPath := filepath.Join(home, ".kube", "config")
		config = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: defaultPath},
			&clientcmd.ConfigOverrides{},
		)
	}

	restConfig, err := config.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build rest config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}

	ns := inst.Namespace
	if ns == "" {
		ns = "argocd"
	}

	return &K8sClient{clientset: clientset, namespace: ns}, nil
}

// HasKubeconfig returns true if the instance has kubeconfig configured.
func HasKubeconfig(inst ArgocdInstance) bool {
	if inst.Kubeconfig != "" || inst.KubeconfigPath != "" {
		return true
	}
	home, _ := os.UserHomeDir()
	_, err := os.Stat(filepath.Join(home, ".kube", "config"))
	return err == nil
}

// GetConfigMap reads a ConfigMap by name from the ArgoCD namespace.
func (k *K8sClient) GetConfigMap(name string) (*corev1.ConfigMap, error) {
	return k.clientset.CoreV1().ConfigMaps(k.namespace).Get(
		context.Background(), name, metav1.GetOptions{},
	)
}

// UpdateConfigMap writes a ConfigMap back to the ArgoCD namespace.
func (k *K8sClient) UpdateConfigMap(cm *corev1.ConfigMap) error {
	_, err := k.clientset.CoreV1().ConfigMaps(k.namespace).Update(
		context.Background(), cm, metav1.UpdateOptions{},
	)
	return err
}

func expandPath(path string) string {
	if len(path) > 1 && path[:2] == "~/" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}
