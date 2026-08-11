package k8sportforward

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func expandPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

func defaultKubeconfigPath() string {
	if v := os.Getenv("KUBECONFIG"); v != "" {
		// Use the first path if colon-separated.
		if i := strings.IndexByte(v, ':'); i >= 0 {
			return v[:i]
		}
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kube", "config")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func isSystemNamespace(name string) bool {
	if _, ok := systemNamespaces[name]; ok {
		return true
	}
	if strings.HasPrefix(name, "kube-") {
		return true
	}
	return false
}

func parsePortList(s string) []int {
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" || part == "-" {
			continue
		}
		// Accept "6379/TCP" or "6379:9121" → take first number.
		part = strings.Split(part, "/")[0]
		part = strings.Split(part, ":")[0]
		n, err := strconv.Atoi(part)
		if err == nil && n > 0 && n <= 65535 {
			out = append(out, n)
		}
	}
	return out
}

func formatPorts(ports []int) string {
	if len(ports) == 0 {
		return "-"
	}
	parts := make([]string, len(ports))
	for i, p := range ports {
		parts[i] = strconv.Itoa(p)
	}
	return strings.Join(parts, ",")
}

// findFreePort returns an available TCP port on localhost.
// Prefer prefer if free; otherwise allocate an ephemeral port.
func findFreePort(prefer int) (int, error) {
	if prefer > 0 && prefer <= 65535 && portFree(prefer) {
		return prefer, nil
	}
	// Prefer prefer+10000 style when prefer is a well-known service port.
	if prefer > 0 && prefer < 55535 {
		alt := prefer + 10000
		if portFree(alt) {
			return alt, nil
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("allocate free port: %w", err)
	}
	defer ln.Close()
	addr := ln.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

func portFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

func resourceKey(kind, namespace, name string) string {
	return strings.ToLower(kind) + "/" + namespace + "/" + name
}

func forwardID(kind, namespace, name string, local, remote int) string {
	return fmt.Sprintf("%s/%s/%s:%d->%d", strings.ToLower(kind), namespace, name, local, remote)
}
