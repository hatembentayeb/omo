package main

import (
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

	// Get from kubectl the CA cert paths
	caPaths, err := cm.getKubectlCAPaths()
	if err != nil {
		return nil, fmt.Errorf("failed to get CA paths: %v", err)
	}

	// Sign the CSR
	err = cm.signCSR(csrPath, certPath, caPaths.CAKey, caPaths.CACert, username, "365") // Valid for 1 year
	if err != nil {
		return nil, fmt.Errorf("failed to sign CSR: %v", err)
	}

	// Calculate expiry date (1 year from now)
	expiryDate := time.Now().AddDate(1, 0, 0)

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
	// Create a config file for the CSR
	configPath := filepath.Join(filepath.Dir(keyPath), "csr.conf")
	configContent := fmt.Sprintf(`[req]
default_bits = 4096
prompt = no
default_md = sha256
distinguished_name = dn

[dn]
CN = %s
O = system:masters
`, username)

	err := os.WriteFile(configPath, []byte(configContent), 0600)
	if err != nil {
		return fmt.Errorf("failed to write CSR config: %v", err)
	}

	// Generate CSR
	cmd := exec.Command("openssl", "req", "-new", "-key", keyPath, "-out", csrPath, "-config", configPath)
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
	// Create a config for the signing
	configPath := filepath.Join(filepath.Dir(csrPath), "signing.conf")
	configContent := fmt.Sprintf(`[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name

[req_distinguished_name]

[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = %s
`, username)

	err := os.WriteFile(configPath, []byte(configContent), 0600)
	if err != nil {
		return fmt.Errorf("failed to write signing config: %v", err)
	}

	// Sign the CSR
	cmd := exec.Command("openssl", "x509", "-req",
		"-in", csrPath,
		"-CA", caCertPath,
		"-CAkey", caKeyPath,
		"-CAcreateserial",
		"-out", certPath,
		"-days", days,
		"-extensions", "v3_req",
		"-extfile", configPath)

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

	// Create kubeconfig content
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
`, serverURL, certInfo.Username, certInfo.Username, certInfo.Username,
		encodeBase64(string(certData)), encodeBase64(string(keyData)))

	// Write the kubeconfig file
	err = os.WriteFile(kubeConfigPath, []byte(kubeConfigContent), 0600)
	if err != nil {
		return "", fmt.Errorf("failed to write kubeconfig: %v", err)
	}

	return kubeConfigPath, nil
}

// encodeBase64 encodes a string to base64
func encodeBase64(data string) string {
	cmd := exec.Command("base64")
	cmd.Stdin = strings.NewReader(data)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(output))
}
