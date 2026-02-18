package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// K8sClient handles interactions with the Kubernetes API
type K8sClient struct {
	KubeConfig     string     // Path to kubeconfig file
	CurrentContext string     // Current kubernetes context
	Users          []*K8sUser // List of users
}

// K8sUser represents a Kubernetes user
type K8sUser struct {
	Username    string
	Namespace   string
	Roles       string
	RolesList   []string
	CertExpiry  string
	Certificate *CertificateInfo
}

// NewK8sClient creates a new Kubernetes client
func NewK8sClient() *K8sClient {
	return &K8sClient{
		Users: make([]*K8sUser, 0),
	}
}

// GetKubeConfig gets the path to the kubeconfig file
func (kc *K8sClient) GetKubeConfig() (string, error) {
	// Get KUBECONFIG environment variable
	kubeConfig := os.Getenv("KUBECONFIG")

	// If not set, use the default location
	if kubeConfig == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %v", err)
		}

		kubeConfig = filepath.Join(homeDir, ".kube", "config")
	}

	// Check if the file exists
	if _, err := os.Stat(kubeConfig); os.IsNotExist(err) {
		return "", fmt.Errorf("kubeconfig file not found at %s", kubeConfig)
	}

	kc.KubeConfig = kubeConfig

	// Get current context
	cmd := exec.Command("kubectl", "config", "current-context")
	output, err := cmd.CombinedOutput()
	if err == nil {
		kc.CurrentContext = strings.TrimSpace(string(output))
	}

	return kubeConfig, nil
}

// GetContexts gets the list of available contexts
func (kc *K8sClient) GetContexts() ([]string, error) {
	cmd := exec.Command("kubectl", "config", "get-contexts", "-o", "name")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl error: %v, output: %s", err, string(output))
	}

	contexts := strings.Split(strings.TrimSpace(string(output)), "\n")
	var result []string

	for _, ctx := range contexts {
		if ctx != "" {
			result = append(result, ctx)
		}
	}

	return result, nil
}

// SetContext sets the current kubernetes context
func (kc *K8sClient) SetContext(context string) error {
	cmd := exec.Command("kubectl", "config", "use-context", context)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl error: %v, output: %s", err, string(output))
	}

	kc.CurrentContext = context
	return nil
}

func formatCertExpiry(certInfo *CertificateInfo) string {
	daysUntilExpiry := int(certInfo.ExpiryDate.Sub(time.Now()).Hours() / 24)
	return fmt.Sprintf("%s (%d days)", certInfo.ExpiryDate.Format("2006-01-02"), daysUntilExpiry)
}

func (kc *K8sClient) hasUser(username string) bool {
	for _, user := range kc.Users {
		if user.Username == username {
			return true
		}
	}
	return false
}

func (kc *K8sClient) loadKubeconfigUsers() error {
	cmd := exec.Command("kubectl", "config", "view", "-o",
		"jsonpath={range .users[*]}{.name}{\"\\t\"}{.user.client-certificate}{\"\\n\"}{end}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl error: %v, output: %s", err, string(output))
	}

	certManager := NewCertManager()
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 2 || parts[1] == "" {
			continue
		}
		username := parts[0]
		namespace, roles := kc.getUserRoles(username)

		user := &K8sUser{
			Username:  username,
			Namespace: namespace,
			Roles:     roles,
		}

		certInfo, err := certManager.GetCertificateInfo(username)
		if err == nil {
			user.CertExpiry = formatCertExpiry(certInfo)
			user.Certificate = certInfo
		} else {
			user.CertExpiry = "Unknown"
		}

		kc.Users = append(kc.Users, user)
	}
	return nil
}

func (kc *K8sClient) loadCertDirUsers() {
	certManager := NewCertManager()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return
	}
	usersDir := filepath.Join(homeDir, ".k8s-users")
	entries, err := os.ReadDir(usersDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		username := entry.Name()
		if kc.hasUser(username) {
			continue
		}
		certInfo, err := certManager.GetCertificateInfo(username)
		if err != nil {
			continue
		}
		namespace, roles := kc.getUserRoles(username)
		kc.Users = append(kc.Users, &K8sUser{
			Username:    username,
			Namespace:   namespace,
			Roles:       roles,
			CertExpiry:  formatCertExpiry(certInfo),
			Certificate: certInfo,
		})
	}
}

func (kc *K8sClient) loadCSRUsers() {
	csrCmd := exec.Command("kubectl", "get", "csr", "-o",
		"jsonpath={range .items[*]}{.spec.username}{\",\"}{end}")
	csrOutput, err := csrCmd.CombinedOutput()
	if err != nil {
		return
	}
	for _, csrUsername := range strings.Split(strings.TrimSpace(string(csrOutput)), ",") {
		if csrUsername == "" || kc.hasUser(csrUsername) {
			continue
		}
		namespace, roles := kc.getUserRoles(csrUsername)
		kc.Users = append(kc.Users, &K8sUser{
			Username:   csrUsername,
			Namespace:  namespace,
			Roles:      roles,
			CertExpiry: "CSR Pending",
		})
	}
}

// GetUsers gets the list of certificate-based users
func (kc *K8sClient) GetUsers() ([]*K8sUser, error) {
	kc.Users = make([]*K8sUser, 0)

	if err := kc.loadKubeconfigUsers(); err != nil {
		return nil, err
	}

	kc.loadCertDirUsers()
	kc.loadCSRUsers()

	return kc.Users, nil
}

func collectRoleBindings(username string, namespaces map[string]bool, roleBindings map[string]bool) {
	cmd := exec.Command("kubectl", "get", "rolebindings", "--all-namespaces", "-o",
		"jsonpath={range .items[*]}{.metadata.namespace}{\"\\t\"}{.metadata.name}{\"\\t\"}{.subjects[*].name}{\"\\n\"}{end}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		for _, name := range strings.Split(parts[2], " ") {
			if name == username {
				namespaces[parts[0]] = true
				roleBindings[parts[1]] = true
			}
		}
	}
}

func collectClusterRoleBindings(username string, namespaces map[string]bool, roleBindings map[string]bool) {
	cmd := exec.Command("kubectl", "get", "clusterrolebindings", "-o",
		"jsonpath={range .items[*]}{.metadata.name}{\"\\t\"}{.subjects[*].name}{\"\\n\"}{end}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		for _, name := range strings.Split(parts[1], " ") {
			if name == username {
				namespaces["cluster-wide"] = true
				roleBindings[parts[0]] = true
			}
		}
	}
}

func mapKeysJoined(m map[string]bool, fallback string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return fallback
	}
	return strings.Join(keys, ", ")
}

// getUserRoles gets the roles assigned to a user in Kubernetes
func (kc *K8sClient) getUserRoles(username string) (string, string) {
	namespaces := make(map[string]bool)
	roleBindings := make(map[string]bool)

	collectRoleBindings(username, namespaces, roleBindings)
	collectClusterRoleBindings(username, namespaces, roleBindings)

	return mapKeysJoined(namespaces, "none"), mapKeysJoined(roleBindings, "none")
}

// CreateUser creates a new Kubernetes user with certificate
func (kc *K8sClient) CreateUser(username string) (*K8sUser, error) {
	// Create certificate for the user
	certManager := NewCertManager()
	certInfo, err := certManager.GenerateUserCert(username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate certificate: %v", err)
	}

	// Format the expiry date
	daysUntilExpiry := int(certInfo.ExpiryDate.Sub(time.Now()).Hours() / 24)
	expiryStr := fmt.Sprintf("%s (%d days)",
		certInfo.ExpiryDate.Format("2006-01-02"), daysUntilExpiry)

	// Return the new user
	user := &K8sUser{
		Username:    username,
		Namespace:   "none",
		Roles:       "none",
		CertExpiry:  expiryStr,
		Certificate: certInfo,
	}

	return user, nil
}

// GetNamespaces gets the list of available namespaces
func (kc *K8sClient) GetNamespaces() ([]string, error) {
	cmd := exec.Command("kubectl", "get", "namespaces", "-o", "jsonpath={.items[*].metadata.name}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl error: %v, output: %s", err, string(output))
	}

	namespaces := strings.Split(strings.TrimSpace(string(output)), " ")
	var result []string

	for _, ns := range namespaces {
		if ns != "" {
			result = append(result, ns)
		}
	}

	// Add cluster-wide option
	result = append(result, "cluster-wide")

	return result, nil
}

// GetRoles gets the list of available roles in a namespace
func (kc *K8sClient) GetRoles(namespace string) ([]string, error) {
	var result []string

	if namespace == "cluster-wide" {
		// Get cluster roles
		cmd := exec.Command("kubectl", "get", "clusterroles", "-o", "jsonpath={.items[*].metadata.name}")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("kubectl error: %v, output: %s", err, string(output))
		}

		roles := strings.Split(strings.TrimSpace(string(output)), " ")
		for _, role := range roles {
			if role != "" {
				result = append(result, role)
			}
		}
	} else {
		// Get namespace roles
		cmd := exec.Command("kubectl", "get", "roles", "-n", namespace, "-o", "jsonpath={.items[*].metadata.name}")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("kubectl error: %v, output: %s", err, string(output))
		}

		roles := strings.Split(strings.TrimSpace(string(output)), " ")
		for _, role := range roles {
			if role != "" {
				result = append(result, role)
			}
		}

		// Also get cluster roles since they can be used in RoleBindings
		cmd = exec.Command("kubectl", "get", "clusterroles", "-o", "jsonpath={.items[*].metadata.name}")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("kubectl error: %v, output: %s", err, string(output))
		}

		clusterRoles := strings.Split(strings.TrimSpace(string(output)), " ")
		for _, role := range clusterRoles {
			if role != "" {
				result = append(result, "clusterrole/"+role)
			}
		}
	}

	return result, nil
}

// AssignRoleToUser assigns a role to a user
func (kc *K8sClient) AssignRoleToUser(username, namespace, role string) error {
	var cmd *exec.Cmd
	var bindingName string

	// Log to the UI instead of using fmt.Printf
	// fmt.Printf("Assigning role %s to user %s in namespace %s\n", role, username, namespace)
	// We'll pass the message back to the caller to log properly

	if namespace == "cluster-wide" {
		// Create a ClusterRoleBinding for cluster-wide permissions
		bindingName = fmt.Sprintf("%s-%s-binding", username, role)

		// For cluster-wide roles, we use a ClusterRoleBinding
		cmd = exec.Command("kubectl", "create", "clusterrolebinding", bindingName,
			"--clusterrole="+role, "--user="+username)

		// Create special binding for cluster-admin if requested
		if role == "cluster-admin" {
			// Create and apply cluster-admin YAML directly
			clusterAdminYaml := fmt.Sprintf(`
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %s-cluster-admin
subjects:
- kind: User
  name: %s
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io
`, username, username)

			// Write to a temp file
			tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("%s-cluster-admin.yaml", username))
			err := os.WriteFile(tempFile, []byte(clusterAdminYaml), 0600)
			if err != nil {
				return fmt.Errorf("failed to write cluster-admin YAML: %v", err)
			}

			// Apply the YAML
			applyCmd := exec.Command("kubectl", "apply", "-f", tempFile)
			output, err := applyCmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("kubectl error applying cluster-admin: %v, output: %s", err, string(output))
			}

			// Clean up
			os.Remove(tempFile)
		}
	} else {
		// Create a RoleBinding for namespace-scoped permissions
		if strings.HasPrefix(role, "clusterrole/") {
			// Extract the actual role name for cluster roles used in namespaces
			roleName := strings.TrimPrefix(role, "clusterrole/")
			bindingName = fmt.Sprintf("%s-%s-binding", username, roleName)

			// Create a RoleBinding using a ClusterRole
			roleBindingYaml := fmt.Sprintf(`
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: %s
  namespace: %s
subjects:
- kind: User
  name: %s
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: %s
  apiGroup: rbac.authorization.k8s.io
`, bindingName, namespace, username, roleName)

			// Write to a temp file
			tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%s.yaml", username, roleName))
			err := os.WriteFile(tempFile, []byte(roleBindingYaml), 0600)
			if err != nil {
				return fmt.Errorf("failed to write role binding YAML: %v", err)
			}

			// Apply the YAML
			applyCmd := exec.Command("kubectl", "apply", "-f", tempFile)
			output, err := applyCmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("kubectl error: %v, output: %s", err, string(output))
			}

			// Clean up
			os.Remove(tempFile)
		} else {
			// Regular role
			bindingName = fmt.Sprintf("%s-%s-binding", username, role)

			// Create RoleBinding using kubectl command
			cmd = exec.Command("kubectl", "create", "rolebinding", bindingName,
				"--role="+role, "--user="+username, "-n", namespace)

			output, err := cmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("kubectl error: %v, output: %s", err, string(output))
			}
		}
	}

	// If cmd was set, execute it (for the simple cases)
	if cmd != nil {
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("kubectl error: %v, output: %s", err, string(output))
		}
	}

	return nil
}

func deleteUserRoleBindings(username string) {
	cmd := exec.Command("kubectl", "get", "rolebindings", "--all-namespaces", "-o",
		"jsonpath={range .items[*]}{.metadata.namespace}{\"\\t\"}{.metadata.name}{\"\\t\"}{.subjects[*].name}{\"\\n\"}{end}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		for _, name := range strings.Split(parts[2], " ") {
			if name == username {
				deleteCmd := exec.Command("kubectl", "delete", "rolebinding", parts[1], "-n", parts[0])
				deleteCmd.CombinedOutput()
			}
		}
	}
}

func deleteUserClusterRoleBindings(username string) {
	cmd := exec.Command("kubectl", "get", "clusterrolebindings", "-o",
		"jsonpath={range .items[*]}{.metadata.name}{\"\\t\"}{.subjects[*].name}{\"\\n\"}{end}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		for _, name := range strings.Split(parts[1], " ") {
			if name == username {
				deleteCmd := exec.Command("kubectl", "delete", "clusterrolebinding", parts[0])
				deleteCmd.CombinedOutput()
			}
		}
	}
}

func deleteUserCSRs(username string) {
	cmd := exec.Command("kubectl", "get", "csr", "-o", "jsonpath={.items[*].metadata.name}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}
	for _, csrName := range strings.Split(strings.TrimSpace(string(output)), " ") {
		if strings.Contains(csrName, username) {
			deleteCmd := exec.Command("kubectl", "delete", "csr", csrName)
			deleteCmd.CombinedOutput()
		}
	}
}

// DeleteUser deletes a user from Kubernetes
func (kc *K8sClient) DeleteUser(username string) error {
	certManager := NewCertManager()
	err := certManager.DeleteCertificate(username)
	if err != nil {
		return fmt.Errorf("failed to delete certificate files: %v", err)
	}

	deleteUserRoleBindings(username)
	deleteUserClusterRoleBindings(username)
	deleteUserCSRs(username)

	return nil
}

// TestAccess tests a user's access to a resource
func (kc *K8sClient) TestAccess(username, namespace, resource, verb string) (bool, string, error) {
	// Find the user's certificate
	user := &K8sUser{
		Username: username,
	}

	for _, u := range kc.Users {
		if u.Username == username && u.Certificate != nil {
			user = u
			break
		}
	}

	if user.Certificate == nil {
		certManager := NewCertManager()
		certInfo, err := certManager.GetCertificateInfo(username)
		if err != nil {
			return false, "", fmt.Errorf("failed to get certificate info: %v", err)
		}
		user.Certificate = certInfo
	}

	// Generate a temporary kubeconfig
	certManager := NewCertManager()
	kubeConfigPath, err := certManager.GenerateKubeConfig(user.Certificate, "")
	if err != nil {
		return false, "", fmt.Errorf("failed to generate kubeconfig: %v", err)
	}

	// Build the command to test access
	args := []string{"auth", "can-i", verb, resource, "--kubeconfig", kubeConfigPath}
	if namespace != "cluster-wide" && namespace != "" {
		args = append(args, "-n", namespace)
	}

	// Replace direct fmt.Printf with a message that will be handled by the UI
	// fmt.Printf("Testing access with command: kubectl %s\n", strings.Join(args, " "))

	cmd := exec.Command("kubectl", args...)
	output, err := cmd.CombinedOutput()
	result := strings.TrimSpace(string(output))

	// Clean up temporary kubeconfig
	os.Remove(kubeConfigPath)

	// Parse the result
	if result == "yes" {
		return true, result, nil
	}

	if strings.Contains(result, "no") {
		return false, result, nil
	}

	return false, result, fmt.Errorf("unexpected response: %s, error: %v", result, err)
}

// CreateCustomRole creates a custom role with the specified rules
func (kc *K8sClient) CreateCustomRole(name, namespace string, rules []map[string]interface{}) error {
	// Determine if this is a cluster role or namespace role
	isClusterRole := namespace == "cluster-wide"

	// Convert rules to YAML format
	rulesYaml := ""
	for _, rule := range rules {
		rulesYaml += "- apiGroups:\n"

		// Handle API groups
		apiGroups, ok := rule["apiGroups"].([]string)
		if !ok {
			apiGroups = []string{""}
		}
		for _, group := range apiGroups {
			rulesYaml += fmt.Sprintf("  - \"%s\"\n", group)
		}

		// Handle resources
		rulesYaml += "  resources:\n"
		resources, ok := rule["resources"].([]string)
		if !ok {
			return fmt.Errorf("resources not specified or invalid format")
		}
		for _, resource := range resources {
			rulesYaml += fmt.Sprintf("  - \"%s\"\n", resource)
		}

		// Handle verbs
		rulesYaml += "  verbs:\n"
		verbs, ok := rule["verbs"].([]string)
		if !ok {
			return fmt.Errorf("verbs not specified or invalid format")
		}
		for _, verb := range verbs {
			rulesYaml += fmt.Sprintf("  - \"%s\"\n", verb)
		}
	}

	// Create the role YAML content
	var roleYaml string
	if isClusterRole {
		roleYaml = fmt.Sprintf(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: %s
rules:
%s`, name, rulesYaml)
	} else {
		roleYaml = fmt.Sprintf(`apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: %s
  namespace: %s
rules:
%s`, name, namespace, rulesYaml)
	}

	// Write to a temp file
	tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("%s-role.yaml", name))
	err := os.WriteFile(tempFile, []byte(roleYaml), 0600)
	if err != nil {
		return fmt.Errorf("failed to write role YAML: %v", err)
	}

	// Apply the role
	applyCmd := exec.Command("kubectl", "apply", "-f", tempFile)
	output, err := applyCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl error applying role: %v, output: %s", err, string(output))
	}

	// Clean up the temp file
	os.Remove(tempFile)

	return nil
}

// DeleteCustomRole deletes a custom role
func (kc *K8sClient) DeleteCustomRole(name, namespace string) error {
	var cmd *exec.Cmd

	if namespace == "cluster-wide" {
		// Delete cluster role
		cmd = exec.Command("kubectl", "delete", "clusterrole", name)
	} else {
		// Delete namespace role
		cmd = exec.Command("kubectl", "delete", "role", name, "-n", namespace)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl error deleting role: %v, output: %s", err, string(output))
	}

	return nil
}
