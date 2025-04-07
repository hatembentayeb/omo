package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CertManager handles certificate operations using OpenSSL
type CertManager struct {
	BaseDir string // Base directory for storing certificates
}

// CertificateInfo contains information about a generated certificate
type CertificateInfo struct {
	Username   string
	PrivateKey string
	CSR        string
	Cert       string
	ExpiryDate time.Time
}

// NewCertManager creates a new certificate manager
func NewCertManager() *CertManager {
	// Create base directory in user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	baseDir := filepath.Join(homeDir, ".k8s-users")

	// Create the directory if it doesn't exist
	err = os.MkdirAll(baseDir, 0700)
	if err != nil {
		fmt.Printf("Warning: Failed to create certificate directory: %v\n", err)
	}

	return &CertManager{
		BaseDir: baseDir,
	}
}

// GenerateUserCert generates a certificate for a user
func (cm *CertManager) GenerateUserCert(username string) (*CertificateInfo, error) {
	// Create user directory
	userDir := filepath.Join(cm.BaseDir, username)
	err := os.MkdirAll(userDir, 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to create user directory: %v", err)
	}

	// Define paths for key, CSR, and certificate
	keyPath := filepath.Join(userDir, "key.pem")
	csrPath := filepath.Join(userDir, "csr.pem")
	certPath := filepath.Join(userDir, "cert.pem")

	// Generate the private key
	err = cm.generatePrivateKey(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	// Generate CSR
	err = cm.generateCSR(keyPath, csrPath, username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate CSR: %v", err)
	}

	// Read the CSR file
	csrData, err := os.ReadFile(csrPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read CSR file: %v", err)
	}

	// Base64 encode the CSR data
	encodedCSR := base64.StdEncoding.EncodeToString(csrData)

	// Create a unique name for the Kubernetes CSR resource
	csrResourceName := fmt.Sprintf("csr-%s-%d", username, time.Now().Unix())
	tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("%s.yaml", csrResourceName))

	// Create the CSR YAML content that includes the CSR data
	csrYaml := fmt.Sprintf(`apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: %s
spec:
  request: %s
  signerName: kubernetes.io/kube-apiserver-client
  expirationSeconds: 31536000  # 1 year
  usages:
  - client auth
`, csrResourceName, encodedCSR)

	// Write the CSR resource to a temp file
	err = os.WriteFile(tempFile, []byte(csrYaml), 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to write CSR resource YAML: %v", err)
	}

	// Submit the CSR to Kubernetes
	submitCmd := exec.Command("kubectl", "apply", "-f", tempFile)
	output, err := submitCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl error submitting CSR: %v, output: %s", err, string(output))
	}

	// Approve the CSR
	approveCmd := exec.Command("kubectl", "certificate", "approve", csrResourceName)
	output, err = approveCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl error approving CSR: %v, output: %s", err, string(output))
	}

	// Wait a moment for the approval to process
	time.Sleep(2 * time.Second)

	// Retrieve the signed certificate
	getCmd := exec.Command("kubectl", "get", "csr", csrResourceName, "-o", "jsonpath={.status.certificate}")
	output, err = getCmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl error getting signed certificate: %v, output: %s", err, string(output))
	}

	// Decode the base64 certificate
	certData, err := base64.StdEncoding.DecodeString(string(output))
	if err != nil {
		return nil, fmt.Errorf("failed to decode certificate: %v", err)
	}

	// Save the signed certificate
	err = os.WriteFile(certPath, certData, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to write certificate file: %v", err)
	}

	// Clean up the temporary files
	os.Remove(tempFile)

	// Calculate expiry date (1 year from now)
	expiryDate := time.Now().AddDate(1, 0, 0)

	// Return the certificate info
	return &CertificateInfo{
		Username:   username,
		PrivateKey: keyPath,
		CSR:        csrPath,
		Cert:       certPath,
		ExpiryDate: expiryDate,
	}, nil
}

// generatePrivateKey generates a private key using OpenSSL
func (cm *CertManager) generatePrivateKey(keyPath string) error {
	cmd := exec.Command("openssl", "genrsa", "-out", keyPath, "4096")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("openssl error: %v, output: %s", err, string(output))
	}

	return nil
}

// generateCSR generates a Certificate Signing Request using OpenSSL
func (cm *CertManager) generateCSR(keyPath, csrPath, username string) error {
	// Create the OpenSSL command with the correct format for Kubernetes user authentication
	// The CN (Common Name) is the username for Kubernetes RBAC
	// The O (Organization) field is used for group membership in Kubernetes
	cmd := exec.Command("openssl", "req", "-new", "-key", keyPath,
		"-out", csrPath,
		"-subj", fmt.Sprintf("/CN=%s/O=system:masters", username))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("openssl error: %v, output: %s", err, string(output))
	}

	return nil
}

// CAPaths contains paths to the CA key and certificate
type CAPaths struct {
	CAKey  string
	CACert string
}

// getKubectlCAPaths gets CA paths from kubectl
func (cm *CertManager) getKubectlCAPaths() (*CAPaths, error) {
	// Try to get the cluster info, including the location of certificates
	cmd := exec.Command("kubectl", "config", "view", "--raw", "-o", "jsonpath={.clusters[].cluster.certificate-authority-data}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl error: %v, output: %s", err, string(output))
	}

	// For this demo, we'll create temporary CA files for signing
	// In a real implementation, you'd properly extract and use the cluster's CA

	// Create a temporary CA certificate and key for demonstration
	// WARNING: In a real environment, you must use the actual cluster CA
	caCertPath := filepath.Join(cm.BaseDir, "temp-ca.crt")
	caKeyPath := filepath.Join(cm.BaseDir, "temp-ca.key")

	// Check if temp files already exist
	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		// Create a temporary CA cert and key
		err = cm.createTempCA(caCertPath, caKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create temp CA: %v", err)
		}
	}

	return &CAPaths{
		CAKey:  caKeyPath,
		CACert: caCertPath,
	}, nil
}

// createTempCA creates a temporary CA cert and key for demo purposes
// WARNING: In production, use the actual cluster CA
func (cm *CertManager) createTempCA(caCertPath, caKeyPath string) error {
	// Generate CA key
	keyCmd := exec.Command("openssl", "genrsa", "-out", caKeyPath, "4096")
	output, err := keyCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("openssl error generating CA key: %v, output: %s", err, string(output))
	}

	// Create CA cert config
	configPath := filepath.Join(cm.BaseDir, "ca.conf")
	configContent := `[req]
default_bits = 4096
prompt = no
default_md = sha256
x509_extensions = v3_ca
distinguished_name = dn

[dn]
CN = kubernetes-ca
O = kubernetes

[v3_ca]
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid:always,issuer:always
basicConstraints = critical, CA:true
`

	err = os.WriteFile(configPath, []byte(configContent), 0600)
	if err != nil {
		return fmt.Errorf("failed to write CA config: %v", err)
	}

	// Generate self-signed CA certificate
	certCmd := exec.Command("openssl", "req", "-x509", "-new", "-nodes",
		"-key", caKeyPath,
		"-out", caCertPath,
		"-config", configPath,
		"-days", "1825") // 5 years

	output, err = certCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("openssl error generating CA cert: %v, output: %s", err, string(output))
	}

	return nil
}

// signCSR signs a CSR with the given CA
func (cm *CertManager) signCSR(csrPath, certPath, caKeyPath, caCertPath, username, days string) error {
	// Use a simple signing command without complex configs
	cmd := exec.Command("openssl", "x509", "-req",
		"-in", csrPath,
		"-CA", caCertPath,
		"-CAkey", caKeyPath,
		"-CAcreateserial",
		"-out", certPath,
		"-days", days)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("openssl error: %v, output: %s", err, string(output))
	}

	return nil
}

// GetCertificateInfo retrieves information about an existing certificate
func (cm *CertManager) GetCertificateInfo(username string) (*CertificateInfo, error) {
	userDir := filepath.Join(cm.BaseDir, username)
	keyPath := filepath.Join(userDir, "key.pem")
	csrPath := filepath.Join(userDir, "csr.pem")
	certPath := filepath.Join(userDir, "cert.pem")

	// Check if all files exist
	for _, path := range []string{userDir, keyPath, csrPath, certPath} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return nil, fmt.Errorf("certificate files for %s not found", username)
		}
	}

	// Get certificate expiry
	cmd := exec.Command("openssl", "x509", "-in", certPath, "-noout", "-enddate")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("openssl error: %v, output: %s", err, string(output))
	}

	// Parse the expiry date
	expiryOutput := strings.TrimSpace(string(output))
	expiryStr := strings.TrimPrefix(expiryOutput, "notAfter=")
	expiryDate, err := time.Parse("Jan 2 15:04:05 2006 MST", expiryStr)
	if err != nil {
		// Default to one year from now if we can't parse the date
		expiryDate = time.Now().AddDate(1, 0, 0)
	}

	return &CertificateInfo{
		Username:   username,
		PrivateKey: keyPath,
		CSR:        csrPath,
		Cert:       certPath,
		ExpiryDate: expiryDate,
	}, nil
}

// DeleteCertificate deletes a user's certificate files
func (cm *CertManager) DeleteCertificate(username string) error {
	userDir := filepath.Join(cm.BaseDir, username)
	if _, err := os.Stat(userDir); os.IsNotExist(err) {
		return fmt.Errorf("certificate directory for %s not found", username)
	}

	err := os.RemoveAll(userDir)
	if err != nil {
		return fmt.Errorf("failed to delete certificate directory: %v", err)
	}

	return nil
}

// GenerateKubeConfig generates a kubeconfig file for the user
func (cm *CertManager) GenerateKubeConfig(certInfo *CertificateInfo, serverURL string) (string, error) {
	// Define the kubeconfig path
	kubeConfigPath := filepath.Join(cm.BaseDir, certInfo.Username+"-kubeconfig")

	// Read the certificate and key files
	certData, err := os.ReadFile(certInfo.Cert)
	if err != nil {
		return "", fmt.Errorf("failed to read certificate: %v", err)
	}

	keyData, err := os.ReadFile(certInfo.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to read private key: %v", err)
	}

	// Get the current kubeconfig for the server URL if not provided
	if serverURL == "" {
		cmd := exec.Command("kubectl", "config", "view", "--minify", "-o", "jsonpath={.clusters[].cluster.server}")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("kubectl error: %v, output: %s", err, string(output))
		}
		serverURL = string(output)
	}

	// Encode the cert and key data
	encodedCert := encodeBase64(string(certData))
	encodedKey := encodeBase64(string(keyData))

	// Create kubeconfig content using just the username
	// This follows the typical kubeconfig format used by kubectl
	kubeConfigContent := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- cluster:
    server: %s
    insecure-skip-tls-verify: true
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: %s
  name: %s@kubernetes
current-context: %s@kubernetes
users:
- name: %s
  user:
    client-certificate-data: %s
    client-key-data: %s
`,
		serverURL,
		certInfo.Username,
		certInfo.Username,
		certInfo.Username,
		certInfo.Username,
		encodedCert,
		encodedKey)

	// Write the kubeconfig file
	err = os.WriteFile(kubeConfigPath, []byte(kubeConfigContent), 0600)
	if err != nil {
		return "", fmt.Errorf("failed to write kubeconfig: %v", err)
	}

	return kubeConfigPath, nil
}

// encodeBase64 encodes a string to base64
func encodeBase64(data string) string {
	return base64.StdEncoding.EncodeToString([]byte(data))
}
