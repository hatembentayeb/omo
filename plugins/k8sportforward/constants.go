package k8sportforward

const (
	viewWorkloads  = "workloads"
	viewServices   = "services"
	viewPods       = "pods"
	viewForwards   = "forwards"
	viewNamespaces = "namespaces"
	viewPorts      = "ports"
)

const (
	fwdMarkActive   = "●F"
	fwdMarkInactive = "○"
)

// systemNamespaces are skipped when scanning user workloads.
var systemNamespaces = map[string]struct{}{
	"kube-system":     {},
	"kube-public":     {},
	"kube-node-lease": {},
	"local-path-storage": {},
	"cattle-system":   {},
	"cattle-fleet-system": {},
	"ingress-nginx":   {},
}
