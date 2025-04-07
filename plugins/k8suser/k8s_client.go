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

// GetUsers gets the list of certificate-based users
func (kc *K8sClient) GetUsers() ([]*K8sUser, error) {
	// First, let's try to get existing client certificate users from the kubeconfig
	cmd := exec.Command("kubectl", "config", "view", "-o",
		"jsonpath={range .users[*]}{.name}{\"\\t\"}{.user.client-certificate}{\"\\n\"}{end}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl error: %v, output: %s", err, string(output))
	}

	// Parse the output
	userLines := strings.Split(strings.TrimSpace(string(output)), "\n")

	kc.Users = make([]*K8sUser, 0)

	for _, line := range userLines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) != 2 || parts[1] == "" {
			// Skip users without client certificates
			continue
		}

		username := parts[0]

		// Get namespace bindings
		namespace, roles := kc.getUserRoles(username)

		// Create the user object
		user := &K8sUser{
			Username:  username,
			Namespace: namespace,
			Roles:     roles,
		}

		// Try to get certificate expiry
		certManager := NewCertManager()
		certInfo, err := certManager.GetCertificateInfo(username)
		if err == nil {
			// Format the expiry date
			daysUntilExpiry := int(certInfo.ExpiryDate.Sub(time.Now()).Hours() / 24)
			user.CertExpiry = fmt.Sprintf("%s (%d days)",
				certInfo.ExpiryDate.Format("2006-01-02"), daysUntilExpiry)
			user.Certificate = certInfo
		} else {
			user.CertExpiry = "Unknown"
		}

		kc.Users = append(kc.Users, user)
	}

	// Now, let's also add any users that have certificates in the .k8s-users directory
	// but might not be in the kubeconfig
	certManager := NewCertManager()
	homeDir, err := os.UserHomeDir()
	if err == nil {
		usersDir := filepath.Join(homeDir, ".k8s-users")
		entries, err := os.ReadDir(usersDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					username := entry.Name()

					// Check if this user is already in our list
					found := false
					for _, user := range kc.Users {
						if user.Username == username {
							found = true
							break
						}
					}

					if !found {
						// Get namespace bindings
						namespace, roles := kc.getUserRoles(username)

						// Try to get certificate info
						certInfo, err := certManager.GetCertificateInfo(username)
						if err == nil {
							// Format the expiry date
							daysUntilExpiry := int(certInfo.ExpiryDate.Sub(time.Now()).Hours() / 24)
							expiryStr := fmt.Sprintf("%s (%d days)",
								certInfo.ExpiryDate.Format("2006-01-02"), daysUntilExpiry)

							// Create and add the user
							kc.Users = append(kc.Users, &K8sUser{
								Username:    username,
								Namespace:   namespace,
								Roles:       roles,
								CertExpiry:  expiryStr,
								Certificate: certInfo,
							})
						}
					}
				}
			}
		}
	}

	// Also look for any users with CSRs
	csrCmd := exec.Command("kubectl", "get", "csr", "-o",
		"jsonpath={range .items[*]}{.spec.username}{\",\"}{end}")
	csrOutput, err := csrCmd.CombinedOutput()
	if err == nil {
		csrUsernames := strings.Split(strings.TrimSpace(string(csrOutput)), ",")
		for _, csrUsername := range csrUsernames {
			if csrUsername == "" {
				continue
			}

			// Skip usernames that are already in our list
			found := false
			for _, user := range kc.Users {
				if user.Username == csrUsername {
					found = true
					break
				}
			}

			if !found {
				// Get namespace bindings
				namespace, roles := kc.getUserRoles(csrUsername)

				// Add the user with CSR status
				kc.Users = append(kc.Users, &K8sUser{
					Username:   csrUsername,
					Namespace:  namespace,
					Roles:      roles,
					CertExpiry: "CSR Pending",
				})
			}
		}
	}

	return kc.Users, nil
}

// getUserRoles gets the roles assigned to a user in Kubernetes
func (kc *K8sClient) getUserRoles(username string) (string, string) {
	// Get the namespaces for role bindings associated with this user
	namespaces := make(map[string]bool)
	roleBindings := make(map[string]bool)

	// Check for RoleBindings in all namespaces
	cmd := exec.Command("kubectl", "get", "rolebindings", "--all-namespaces", "-o",
		"jsonpath={range .items[*]}{.metadata.namespace}{\"\\t\"}{.metadata.name}{\"\\t\"}{.subjects[*].name}{\"\\n\"}{end}")
	output, err := cmd.CombinedOutput()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")

		for _, line := range lines {
			if line == "" {
				continue
			}

			parts := strings.Split(line, "\t")
			if len(parts) < 3 {
				continue
			}

			namespace := parts[0]
			binding := parts[1]
			usernames := strings.Split(parts[2], " ")

			for _, name := range usernames {
				// Check both with and without system prefix
				if name == username {
					namespaces[namespace] = true
					roleBindings[binding] = true
				}
			}
		}
	}

	// Check for ClusterRoleBindings
	cmd = exec.Command("kubectl", "get", "clusterrolebindings", "-o",
		"jsonpath={range .items[*]}{.metadata.name}{\"\\t\"}{.subjects[*].name}{\"\\n\"}{end}")
	output, err = cmd.CombinedOutput()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")

		for _, line := range lines {
			if line == "" {
				continue
			}

			parts := strings.Split(line, "\t")
			if len(parts) < 2 {
				continue
			}

			binding := parts[0]
			usernames := strings.Split(parts[1], " ")

			for _, name := range usernames {
				// Check both with and without system prefix
				if name == username {
					namespaces["cluster-wide"] = true
					roleBindings[binding] = true
				}
			}
		}
	}

	// Build strings for namespaces and roles
	namespaceList := make([]string, 0, len(namespaces))
	for namespace := range namespaces {
		namespaceList = append(namespaceList, namespace)
	}

	rolesList := make([]string, 0, len(roleBindings))
	for role := range roleBindings {
		rolesList = append(rolesList, role)
	}

	namespaceStr := strings.Join(namespaceList, ", ")
	if namespaceStr == "" {
		namespaceStr = "none"
	}

	rolesStr := strings.Join(rolesList, ", ")
	if rolesStr == "" {
		rolesStr = "none"
	}

	return namespaceStr, rolesStr
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

// DeleteUser deletes a user from Kubernetes
func (kc *K8sClient) DeleteUser(username string) error {
	// Delete the user's certificate files
	certManager := NewCertManager()
	err := certManager.DeleteCertificate(username)
	if err != nil {
		return fmt.Errorf("failed to delete certificate files: %v", err)
	}

	// Delete any RoleBindings for this user
	cmd := exec.Command("kubectl", "get", "rolebindings", "--all-namespaces", "-o",
		"jsonpath={range .items[*]}{.metadata.namespace}{\"\\t\"}{.metadata.name}{\"\\t\"}{.subjects[*].name}{\"\\n\"}{end}")
	output, err := cmd.CombinedOutput()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")

		for _, line := range lines {
			if line == "" {
				continue
			}

			parts := strings.Split(line, "\t")
			if len(parts) < 3 {
				continue
			}

			namespace := parts[0]
			binding := parts[1]
			usernames := strings.Split(parts[2], " ")

			for _, name := range usernames {
				if name == username {
					// Delete this binding
					deleteCmd := exec.Command("kubectl", "delete", "rolebinding", binding, "-n", namespace)
					deleteCmd.CombinedOutput()
					// Ignore errors as we want to continue deleting other bindings
				}
			}
		}
	}

	// Delete any ClusterRoleBindings for this user
	cmd = exec.Command("kubectl", "get", "clusterrolebindings", "-o",
		"jsonpath={range .items[*]}{.metadata.name}{\"\\t\"}{.subjects[*].name}{\"\\n\"}{end}")
	output, err = cmd.CombinedOutput()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")

		for _, line := range lines {
			if line == "" {
				continue
			}

			parts := strings.Split(line, "\t")
			if len(parts) < 2 {
				continue
			}

			binding := parts[0]
			usernames := strings.Split(parts[1], " ")

			for _, name := range usernames {
				if name == username {
					// Delete this binding
					deleteCmd := exec.Command("kubectl", "delete", "clusterrolebinding", binding)
					deleteCmd.CombinedOutput()
					// Ignore errors as we want to continue deleting other bindings
				}
			}
		}
	}

	// Also check for and delete any pending CSRs for this user
	cmd = exec.Command("kubectl", "get", "csr", "-o", "jsonpath={.items[*].metadata.name}")
	output, err = cmd.CombinedOutput()
	if err == nil {
		csrNames := strings.Split(strings.TrimSpace(string(output)), " ")
		for _, csrName := range csrNames {
			if strings.Contains(csrName, username) {
				deleteCmd := exec.Command("kubectl", "delete", "csr", csrName)
				deleteCmd.CombinedOutput()
			}
		}
	}

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
