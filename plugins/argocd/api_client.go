package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ArgoAPIClient handles communication with the ArgoCD API
type ArgoAPIClient struct {
	BaseURL     string
	Username    string
	Password    string
	Token       string
	HTTPClient  *http.Client
	IsConnected bool
	Debug       struct {
		Enabled      bool
		LogAPICalls  bool
		LogResponses bool
	}
}

// Account represents an ArgoCD user account
type Account struct {
	Name         string   `json:"name"`
	Enabled      bool     `json:"enabled"`
	Capabilities []string `json:"capabilities"`
	Tokens       []Token  `json:"tokens"`
}

// Token represents an ArgoCD auth token
type Token struct {
	ID        string `json:"id"`
	IssuedAt  int64  `json:"issuedAt"`
	ExpiresAt int64  `json:"expiresAt"`
}

// FormatIssuedAt returns a human-readable issued time.
func (t Token) FormatIssuedAt() string {
	if t.IssuedAt == 0 {
		return "N/A"
	}
	return time.Unix(t.IssuedAt, 0).Format("2006-01-02 15:04:05")
}

// FormatExpiresAt returns a human-readable expiry time.
func (t Token) FormatExpiresAt() string {
	if t.ExpiresAt == 0 {
		return "Never"
	}
	return time.Unix(t.ExpiresAt, 0).Format("2006-01-02 15:04:05")
}

// CreateTokenResponse is the response from creating a token
type CreateTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expiresAt"`
	IssuedAt  int64  `json:"issuedAt"`
}

// FormatIssuedAt returns a human-readable issued time.
func (r CreateTokenResponse) FormatIssuedAt() string {
	if r.IssuedAt == 0 {
		return "N/A"
	}
	return time.Unix(r.IssuedAt, 0).Format("2006-01-02 15:04:05")
}

// FormatExpiresAt returns a human-readable expiry time.
func (r CreateTokenResponse) FormatExpiresAt() string {
	if r.ExpiresAt == 0 {
		return "Never"
	}
	return time.Unix(r.ExpiresAt, 0).Format("2006-01-02 15:04:05")
}

// Project represents an ArgoCD project
type Project struct {
	Name                     string                 `json:"name"`
	Description              string                 `json:"description"`
	ClusterResourceWhitelist []Resource             `json:"clusterResourceWhitelist"`
	Destinations             []Destination          `json:"destinations"`
	SourceRepos              []string               `json:"sourceRepos"`
	Roles                    []ProjectRole          `json:"roles"`
	Metadata                 map[string]interface{} `json:"metadata"`
	Spec                     map[string]interface{} `json:"spec"`
}

// Resource represents a Kubernetes resource
type Resource struct {
	Group string `json:"group"`
	Kind  string `json:"kind"`
}

// Destination represents a deployment destination
type Destination struct {
	Server    string `json:"server"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// ProjectRole represents a role within a project
type ProjectRole struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Policies    []string `json:"policies"`
	Groups      []string `json:"groups"`
}

// Application represents an ArgoCD application.
type Application struct {
	Name      string                 `json:"name"`
	Project   string                 `json:"project"`
	Status    map[string]interface{} `json:"status"`
	Health    Health                 `json:"health"`
	Sync      Sync                   `json:"sync"`
	Namespace string                 `json:"namespace"`
	Server    string                 `json:"server"`
	Metadata  map[string]interface{} `json:"metadata"`
	Spec      map[string]interface{} `json:"spec"`
}

func extractStringFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func extractNestedString(m map[string]interface{}, outerKey, innerKey string) string {
	if m == nil {
		return ""
	}
	if inner, ok := m[outerKey].(map[string]interface{}); ok {
		return extractStringFromMap(inner, innerKey)
	}
	return ""
}

func extractStringSliceFromMap(m map[string]interface{}, key string) []string {
	if m == nil {
		return nil
	}
	items, ok := m[key].([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

func resolveStatusFromInterface(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	if m, ok := v.(map[string]interface{}); ok {
		return extractStringFromMap(m, "status")
	}
	return ""
}

func (a *Application) resolveProjectFromJSON(spec map[string]interface{}, projectName interface{}) {
	if a.Project != "" {
		return
	}

	if s := extractStringFromMap(spec, "project"); s != "" {
		a.Project = s
		return
	}

	if s := resolveStatusFromInterface(projectName); s != "" {
		a.Project = s
		return
	}

	if a.Metadata != nil {
		if s := extractStringFromMap(a.Metadata, "project"); s != "" {
			a.Project = s
			return
		}
		if s := extractNestedString(a.Metadata, "labels", "argocd.argoproj.io/project"); s != "" {
			a.Project = s
			return
		}
	}

	if s := extractNestedString(a.Status, "spec", "project"); s != "" {
		a.Project = s
		return
	}

	if a.Project == "" && spec != nil {
		a.Spec = spec
	}
}

func (a *Application) resolveHealthFromJSON(healthStatus interface{}) {
	if a.Health.Status != "" {
		return
	}

	if s := resolveStatusFromInterface(healthStatus); s != "" {
		a.Health.Status = s
		return
	}

	if s := extractNestedString(a.Status, "health", "status"); s != "" {
		a.Health.Status = s
	}
}

func (a *Application) resolveSyncFromJSON(syncStatus interface{}) {
	if a.Sync.Status != "" {
		return
	}

	if s := resolveStatusFromInterface(syncStatus); s != "" {
		a.Sync.Status = s
		return
	}

	if s := extractNestedString(a.Status, "sync", "status"); s != "" {
		a.Sync.Status = s
	}
}

// UnmarshalJSON implements custom unmarshaling for Application
func (a *Application) UnmarshalJSON(data []byte) error {
	type Alias Application
	aux := &struct {
		*Alias
		HealthStatus interface{}            `json:"healthStatus"`
		SyncStatus   interface{}            `json:"syncStatus"`
		ProjectName  interface{}            `json:"projectName"`
		Spec         map[string]interface{} `json:"spec"`
	}{
		Alias: (*Alias)(a),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	a.resolveProjectFromJSON(aux.Spec, aux.ProjectName)
	a.resolveHealthFromJSON(aux.HealthStatus)
	a.resolveSyncFromJSON(aux.SyncStatus)

	return nil
}

// Health represents the health status of an application
type Health struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// UnmarshalJSON is a custom unmarshaler for Health
func (h *Health) UnmarshalJSON(data []byte) error {
	type Alias Health
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(h),
	}

	// Try standard unmarshal first
	if err := json.Unmarshal(data, &aux); err != nil {
		// If it's a string, assume it's just the status
		var status string
		if err := json.Unmarshal(data, &status); err == nil {
			h.Status = status
			h.Message = ""
			return nil
		}
		return err
	}

	// Provide default values if fields are empty
	if h.Status == "" {
		h.Status = "Unknown"
	}

	return nil
}

// Sync represents the sync status of an application
type Sync struct {
	Status   string `json:"status"`
	Revision string `json:"revision"`
}

// UnmarshalJSON is a custom unmarshaler for Sync
func (s *Sync) UnmarshalJSON(data []byte) error {
	type Alias Sync
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(s),
	}

	// Try standard unmarshal first
	if err := json.Unmarshal(data, &aux); err != nil {
		// If it's a string, assume it's just the status
		var status string
		if err := json.Unmarshal(data, &status); err == nil {
			s.Status = status
			s.Revision = ""
			return nil
		}
		return err
	}

	// Provide default values if fields are empty
	if s.Status == "" {
		s.Status = "Unknown"
	}

	return nil
}

// NewArgoAPIClient creates a new ArgoCD API client
func NewArgoAPIClient(config *ArgocdConfig) *ArgoAPIClient {
	// Default timeout
	timeout := time.Second * 30

	// Create client
	client := &ArgoAPIClient{
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
		IsConnected: false,
	}

	// Configure from config if available
	if config != nil {
		// Set timeout
		if config.Debug.RequestTimeoutSeconds > 0 {
			timeout = time.Second * time.Duration(config.Debug.RequestTimeoutSeconds)
			Debug("Using custom timeout: %v", timeout)
			client.HTTPClient.Timeout = timeout
		}

		// Set debug flags
		client.Debug.Enabled = config.Debug.Enabled
		client.Debug.LogAPICalls = config.Debug.LogAPICalls
		client.Debug.LogResponses = config.Debug.LogResponses

		Debug("Debug settings: enabled=%v, logAPICalls=%v, logResponses=%v",
			client.Debug.Enabled, client.Debug.LogAPICalls, client.Debug.LogResponses)
	}

	return client
}

// Authenticate authenticates to ArgoCD API with username and password
func (c *ArgoAPIClient) Authenticate(username, password string) error {
	Debug("Authenticating to ArgoCD API: %s", c.BaseURL)

	// Store credentials
	c.Username = username
	c.Password = password

	return c.authenticateWithJSON()
}

// authenticateWithJSON attempts to authenticate using JSON payloads
func (c *ArgoAPIClient) authenticateWithJSON() error {
	Debug("Trying JSON-based authentication")

	// Try standard path first
	err := c.tryAuthenticateWithJSON("api/v1/session")
	if err == nil {
		return nil
	}

	Debug("First JSON auth attempt failed: %v", err)

	// Try alternate paths
	for _, path := range []string{
		"api/v2/session",
		"auth/login",
		"api/login",
	} {
		Debug("Trying alternate JSON auth path: %s", path)
		err = c.tryAuthenticateWithJSON(path)
		if err == nil {
			return nil
		}
		Debug("JSON auth attempt failed with path %s: %v", path, err)
	}

	return fmt.Errorf("JSON authentication failed with all known endpoints")
}

// tryAuthenticateWithJSON attempts authentication using JSON payload
func (c *ArgoAPIClient) tryAuthenticateWithJSON(path string) error {
	// Create JSON payload for authentication
	payload := map[string]string{
		"username": c.Username,
		"password": c.Password,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		Debug("Error marshaling JSON payload: %v", err)
		return err
	}

	fullURL := c.BaseURL + path
	Debug("JSON authenticating with username: %s to URL: %s", c.Username, fullURL)

	req, err := http.NewRequest("POST", fullURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		Debug("Error creating JSON request: %v", err)
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	Debug("JSON request headers: %v", req.Header)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		Debug("JSON authentication request error: %v", err)
		return err
	}
	defer resp.Body.Close()

	Debug("JSON auth response status: %s", resp.Status)
	Debug("JSON auth response headers: %v", resp.Header)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		Debug("Failed to read JSON response body: %v", err)
		return fmt.Errorf("failed to read JSON response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		Debug("JSON auth failed with status %d, response body: %s", resp.StatusCode, string(bodyBytes))
		return fmt.Errorf("JSON auth failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	Debug("JSON auth response body: %s", string(bodyBytes))

	// Try multiple ways to extract the token from the response
	token, err := c.extractToken(bodyBytes)
	if err != nil {
		Debug("Failed to extract token from JSON auth: %v", err)
		return err
	}

	Debug("Got token from JSON auth (length: %d)", len(token))
	c.Token = token
	return nil
}

// extractToken attempts to extract the auth token from response bytes using multiple approaches
func (c *ArgoAPIClient) extractToken(bodyBytes []byte) (string, error) {
	Debug("Attempting to extract token from response")

	// First try the standard format
	var tokenResp struct {
		Token string `json:"token"`
	}

	if err := json.Unmarshal(bodyBytes, &tokenResp); err == nil && tokenResp.Token != "" {
		Debug("Found token in standard format")
		return tokenResp.Token, nil
	}

	// Try alternate format with nested token
	var nestedResp struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &nestedResp); err == nil && nestedResp.Data.Token != "" {
		Debug("Found token in nested format")
		return nestedResp.Data.Token, nil
	}

	// Try JWT format where the response might be the token itself
	respString := string(bodyBytes)
	respString = strings.TrimSpace(respString)

	// Check if response is just a plain token (JWT typically starts with "ey")
	if strings.HasPrefix(respString, "\"ey") && strings.HasSuffix(respString, "\"") {
		// Strip quotes
		token := respString[1 : len(respString)-1]
		Debug("Response appears to be a raw JWT token")
		return token, nil
	}

	// If the response is unquoted JWT
	if strings.HasPrefix(respString, "ey") {
		Debug("Response appears to be an unquoted JWT token")
		return respString, nil
	}

	// Try to parse as a generic JSON and look for common token field names
	var genericResp map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &genericResp); err == nil {
		tokenFieldNames := []string{"token", "access_token", "jwt", "id_token", "auth_token"}

		for _, fieldName := range tokenFieldNames {
			if tokenValue, ok := genericResp[fieldName].(string); ok && tokenValue != "" {
				Debug("Found token in field: %s", fieldName)
				return tokenValue, nil
			}
		}

		// Check nested under "data"
		if data, ok := genericResp["data"].(map[string]interface{}); ok {
			for _, fieldName := range tokenFieldNames {
				if tokenValue, ok := data[fieldName].(string); ok && tokenValue != "" {
					Debug("Found token in nested data.%s", fieldName)
					return tokenValue, nil
				}
			}
		}
	}

	Debug("Could not extract token from response")
	return "", fmt.Errorf("could not find token in response: %s", string(bodyBytes))
}

// Login attempts to authenticate with ArgoCD using available auth methods
func (c *ArgoAPIClient) Login() error {
	if c.Username == "" || c.Password == "" {
		return fmt.Errorf("username and password must be set")
	}

	// Try different authentication methods
	methods := []struct {
		name string
		fn   func() error
	}{
		{"JSON", c.authenticateWithJSON},
		{"form", c.authenticateWithForm},
	}

	var lastErr error
	for _, method := range methods {
		Debug("Trying authentication method: %s", method.name)
		lastErr = method.fn()
		if lastErr == nil {
			Debug("Authentication successful with method: %s", method.name)
			return nil
		}
		Debug("Authentication failed with method %s: %v", method.name, lastErr)
	}

	return fmt.Errorf("all authentication methods failed, last error: %v", lastErr)
}

// Connect establishes a connection to the ArgoCD server with the new authentication method
func (c *ArgoAPIClient) Connect(baseURL, username, password string) error {
	// Normalize base URL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL = baseURL + "/"
	}

	Debug("Connecting to ArgoCD server at %s", baseURL)
	c.BaseURL = baseURL
	c.Username = username
	c.Password = password

	// Try to detect the ArgoCD version to adjust our approach
	version, err := c.detectVersion()
	if err != nil {
		Debug("Failed to detect ArgoCD version: %v", err)
		// We'll continue with authentication anyway
	} else {
		Debug("Detected ArgoCD version: %s", version)
	}

	// Probe the API to discover endpoints
	err = c.probeAPI()
	if err != nil {
		Debug("API probe failed: %v", err)
		// Continue anyway, we'll try authentication with known paths
	}

	// Try to login with all available methods
	err = c.Login()
	if err != nil {
		Debug("All authentication methods failed: %v", err)
		return fmt.Errorf("failed to authenticate: %v", err)
	}

	c.IsConnected = true
	Debug("Successfully connected to ArgoCD")
	return nil
}

// detectVersion attempts to determine the ArgoCD version
func (c *ArgoAPIClient) detectVersion() (string, error) {
	Debug("Detecting ArgoCD version")

	versionURL := c.BaseURL + "api/version"

	req, err := http.NewRequest("GET", versionURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get version, status: %s", resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var versionInfo map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &versionInfo); err == nil {
		if version, ok := versionInfo["version"].(string); ok {
			return version, nil
		}
	}

	return "", fmt.Errorf("version info not found in response")
}

// authenticateWithForm attempts to authenticate using form-encoded data
func (c *ArgoAPIClient) authenticateWithForm() error {
	Debug("Trying form-based authentication")

	// Try standard path first
	err := c.tryAuthenticateWithForm("api/v1/session")
	if err == nil {
		return nil
	}

	Debug("First form auth attempt failed: %v", err)

	// Try alternate paths
	for _, path := range []string{
		"api/v2/session",
		"auth/login",
		"api/login",
	} {
		Debug("Trying alternate form auth path: %s", path)
		err = c.tryAuthenticateWithForm(path)
		if err == nil {
			return nil
		}
		Debug("Form auth attempt failed with path %s: %v", path, err)
	}

	return fmt.Errorf("form authentication failed with all known endpoints")
}

// tryAuthenticateWithForm attempts authentication using form URL-encoded data
func (c *ArgoAPIClient) tryAuthenticateWithForm(path string) error {
	// Create form data
	formData := fmt.Sprintf("username=%s&password=%s", c.Username, c.Password)

	fullURL := c.BaseURL + path
	Debug("Form authenticating with username: %s to URL: %s", c.Username, fullURL)

	req, err := http.NewRequest("POST", fullURL, strings.NewReader(formData))
	if err != nil {
		Debug("Error creating form request: %v", err)
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	Debug("Form request headers: %v", req.Header)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		Debug("Form authentication request error: %v", err)
		return err
	}
	defer resp.Body.Close()

	Debug("Form auth response status: %s", resp.Status)
	Debug("Form auth response headers: %v", resp.Header)

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		Debug("Failed to read form response body: %v", err)
		return fmt.Errorf("failed to read form response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		Debug("Form auth failed with status %d, response body: %s", resp.StatusCode, string(bodyBytes))
		return fmt.Errorf("form auth failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	Debug("Form auth response body: %s", string(bodyBytes))

	// Try multiple ways to extract the token from the response
	token, err := c.extractToken(bodyBytes)
	if err != nil {
		Debug("Failed to extract token from form auth: %v", err)
		return err
	}

	Debug("Got token from form auth (length: %d)", len(token))
	c.Token = token
	return nil
}

// probeAPI attempts to determine the ArgoCD API version and structure
func (c *ArgoAPIClient) probeAPI() error {
	Debug("Probing ArgoCD API to discover endpoints")

	// First check if we can access the server at all
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 5 redirects
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	// Try a simple GET request to the server root
	Debug("Trying base URL: %s", c.BaseURL)
	resp, err := client.Get(c.BaseURL)
	if err != nil {
		Debug("Failed to access base URL: %v", err)
		return err
	}
	defer resp.Body.Close()

	Debug("Base URL accessible, status: %s", resp.Status)

	// Try to access /api/version endpoint which is common in many ArgoCD installations
	versionURL := c.BaseURL + "api/version"
	Debug("Checking API version endpoint: %s", versionURL)
	resp, err = client.Get(versionURL)
	if err != nil {
		Debug("Failed to access version endpoint: %v", err)
	} else {
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			// Try to read version info
			bodyBytes, err := io.ReadAll(resp.Body)
			if err == nil {
				Debug("Version endpoint response: %s", string(bodyBytes))

				// Try to parse version info
				var versionInfo map[string]interface{}
				if err := json.Unmarshal(bodyBytes, &versionInfo); err == nil {
					if version, ok := versionInfo["version"].(string); ok {
						Debug("ArgoCD version: %s", version)
					}
				}
			}
		} else {
			Debug("Version endpoint returned status: %s", resp.Status)
		}
	}

	// Check common API endpoints to see which one(s) might be available
	endpoints := []string{
		"api/v1/applications",
		"api/v1/projects",
		"api/v1/clusters",
		"api/v1/repositories",
	}

	for _, endpoint := range endpoints {
		endpointURL := c.BaseURL + endpoint
		Debug("Probing endpoint: %s", endpointURL)
		resp, err = client.Get(endpointURL)
		if err != nil {
			Debug("Failed to access %s: %v", endpoint, err)
			continue
		}
		defer resp.Body.Close()

		// Even a 401 Unauthorized is a good sign - it means the endpoint exists
		// but requires authentication
		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusUnauthorized {
			Debug("Endpoint %s exists (status: %s)", endpoint, resp.Status)
		} else {
			Debug("Endpoint %s returned status: %s", endpoint, resp.Status)
		}
	}

	Debug("API probe completed")
	return nil
}

// makeRequest makes an HTTP request to the ArgoCD API
func (c *ArgoAPIClient) makeRequest(method, path string, body io.Reader) (*http.Response, error) {
	if !c.IsConnected {
		Debug("API client not connected")
		return nil, fmt.Errorf("not connected to ArgoCD server")
	}

	requestURL := c.BaseURL + path
	if c.Debug.LogAPICalls {
		Debug("Making %s request to %s", method, requestURL)
		if body != nil {
			// Copy the body to log it
			var buf bytes.Buffer
			tee := io.TeeReader(body, &buf)
			bodyBytes, _ := io.ReadAll(tee)
			body = &buf // Reset body for the actual request
			Debug("Request body: %s", string(bodyBytes))
		}
	}

	req, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		Debug("Error creating request: %v", err)
		return nil, err
	}

	// Add auth token
	if c.Token != "" {
		req.Header.Add("Authorization", "Bearer "+c.Token)
		if c.Debug.LogAPICalls {
			Debug("Added auth token (length: %d)", len(c.Token))
		}
	} else {
		Debug("No auth token available")
	}

	// Add content type for POST/PUT
	if method == "POST" || method == "PUT" {
		req.Header.Add("Content-Type", "application/json")
		if c.Debug.LogAPICalls {
			Debug("Added Content-Type: application/json header")
		}
	}

	// Log all request headers for debugging
	if c.Debug.LogAPICalls {
		Debug("Request headers:")
		for name, values := range req.Header {
			Debug("  %s: %s", name, strings.Join(values, ", "))
		}
	}

	// Make the request
	if c.Debug.LogAPICalls {
		Debug("Sending request...")
	}
	start := time.Now()
	resp, err := c.HTTPClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		Debug("Request error: %v", err)
		return nil, err
	}

	Debug("Response received in %v: %s", duration, resp.Status)

	// Log response headers
	if c.Debug.LogResponses {
		Debug("Response headers:")
		for name, values := range resp.Header {
			Debug("  %s: %s", name, strings.Join(values, ", "))
		}

		// Keep a copy of the response body for logging if needed
		if resp.Body != nil {
			bodyBytes, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			// Create a new ReadCloser from the bytes we read
			resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			Debug("Response body: %s", truncateBodyForLogging(bodyBytes))
		}
	}

	return resp, nil
}

// GetAccounts retrieves all accounts from ArgoCD
func (c *ArgoAPIClient) GetAccounts() ([]Account, error) {
	resp, err := c.makeRequest("GET", "api/v1/account", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get accounts with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Items []Account `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result.Items, nil
}

// GetAccount retrieves a specific account by name
func (c *ArgoAPIClient) GetAccount(name string) (*Account, error) {
	resp, err := c.makeRequest("GET", "api/v1/account/"+name, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get account with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var account Account
	if err := json.NewDecoder(resp.Body).Decode(&account); err != nil {
		return nil, err
	}

	return &account, nil
}

// CreateAccount creates a new account in ArgoCD
func (c *ArgoAPIClient) CreateAccount(account Account, password string) error {
	// Prepare the request body
	accountData := map[string]interface{}{
		"name":         account.Name,
		"enabled":      account.Enabled,
		"capabilities": account.Capabilities,
		"password":     password,
	}

	accountJSON, err := json.Marshal(accountData)
	if err != nil {
		return err
	}

	resp, err := c.makeRequest("POST", "api/v1/account", strings.NewReader(string(accountJSON)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create account with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeleteAccount deletes an account from ArgoCD
func (c *ArgoAPIClient) DeleteAccount(name string) error {
	resp, err := c.makeRequest("DELETE", "api/v1/account/"+name, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete account with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// UpdatePassword sets a new password for an account.
// The admin authenticates with their own current password.
func (c *ArgoAPIClient) UpdatePassword(accountName, newPassword, currentAdminPassword string) error {
	payload := map[string]string{
		"name":            accountName,
		"currentPassword": currentAdminPassword,
		"newPassword":     newPassword,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := c.makeRequest("PUT", "api/v1/account/password", strings.NewReader(string(payloadBytes)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set password with status %d: %s", resp.StatusCode, string(bodyBytes))
	}
	return nil
}

// CreateToken creates a new token for an account
func (c *ArgoAPIClient) CreateToken(accountName string, expiresInHours int) (*CreateTokenResponse, error) {
	// Prepare the request body
	tokenData := map[string]interface{}{
		"expiresIn": fmt.Sprintf("%dh", expiresInHours),
	}

	tokenJSON, err := json.Marshal(tokenData)
	if err != nil {
		return nil, err
	}

	resp, err := c.makeRequest("POST", "api/v1/account/"+accountName+"/token", strings.NewReader(string(tokenJSON)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create token with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result CreateTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// GetProjects retrieves all projects from ArgoCD
func (c *ArgoAPIClient) GetProjects() ([]Project, error) {
	// Try standard endpoint first
	projects, err := c.tryGetProjects("api/v1/projects")
	if err == nil {
		return projects, nil
	}

	Debug("Standard projects endpoint failed: %v", err)

	// Try alternate endpoints
	for _, path := range []string{
		"api/projects",
		"application-controller/api/v1/projects",
		"argocd/api/v1/projects",
		"api/v1/repos",    // Some versions might expose projects as repos
		"api/v1/settings", // Some might include project data in settings
	} {
		Debug("Trying alternate projects endpoint: %s", path)
		projects, err = c.tryGetProjects(path)
		if err == nil {
			return projects, nil
		}
		Debug("Projects endpoint %s failed: %v", path, err)
	}

	// Last resort: Try to extract projects from applications
	Debug("Attempting to extract projects from applications")
	apps, err := c.GetApplications()
	if err == nil && len(apps) > 0 {
		projectMap := make(map[string]bool)
		projects := []Project{}

		for _, app := range apps {
			if app.Project != "" && !projectMap[app.Project] {
				projectMap[app.Project] = true
				projects = append(projects, Project{
					Name:        app.Project,
					Description: fmt.Sprintf("Project extracted from application %s", app.Name),
				})
			}
		}

		if len(projects) > 0 {
			Debug("Extracted %d projects from applications", len(projects))
			return projects, nil
		}
	}

	return nil, fmt.Errorf("failed to get projects from any known endpoint")
}

func tryParseProjectsSpecFormat(data []byte) ([]Project, bool) {
	var result struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec Project `json:"spec"`
		} `json:"items"`
	}
	if err := json.Unmarshal(data, &result); err != nil || len(result.Items) == 0 {
		return nil, false
	}
	Debug("Found %d projects in spec format", len(result.Items))
	projects := make([]Project, 0, len(result.Items))
	for _, item := range result.Items {
		project := item.Spec
		project.Name = item.Metadata.Name
		projects = append(projects, project)
	}
	return projects, true
}

func tryParseProjectsItemsArray(data []byte) ([]Project, bool) {
	var result struct {
		Items []Project `json:"items"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		Debug("Failed to unmarshal projects as items array: %v", err)
		return nil, false
	}
	if len(result.Items) == 0 {
		return nil, false
	}
	Debug("Found %d projects in items array", len(result.Items))
	for i, proj := range result.Items {
		Debug("Project %d: Name=%s, Description=%s, Destinations=%d, SourceRepos=%d, Roles=%d",
			i, proj.Name, proj.Description, len(proj.Destinations), len(proj.SourceRepos), len(proj.Roles))
	}
	return result.Items, true
}

func tryParseProjectsDirectArray(data []byte) ([]Project, bool) {
	var result []Project
	if err := json.Unmarshal(data, &result); err != nil {
		Debug("Failed to unmarshal projects as direct array: %v", err)
		return nil, false
	}
	if len(result) == 0 {
		return nil, false
	}
	Debug("Found %d projects in direct array", len(result))
	for i, proj := range result {
		Debug("Project %d: Name=%s, Description=%s, Destinations=%d, SourceRepos=%d, Roles=%d",
			i, proj.Name, proj.Description, len(proj.Destinations), len(proj.SourceRepos), len(proj.Roles))
	}
	return result, true
}

func tryParseProjectsFromNestedKey(data []byte) ([]Project, bool) {
	var projArray []Project
	if err := json.Unmarshal(data, &projArray); err == nil && len(projArray) > 0 {
		return projArray, true
	}

	var projWithItems struct {
		Items []Project `json:"items"`
	}
	if err := json.Unmarshal(data, &projWithItems); err == nil && len(projWithItems.Items) > 0 {
		return projWithItems.Items, true
	}

	var projMap map[string]Project
	if err := json.Unmarshal(data, &projMap); err == nil && len(projMap) > 0 {
		projects := make([]Project, 0, len(projMap))
		for name, project := range projMap {
			if project.Name == "" {
				project.Name = name
			}
			projects = append(projects, project)
		}
		return projects, true
	}

	return nil, false
}

func tryParseProjectsNested(data []byte) ([]Project, bool) {
	var nestedResult map[string]interface{}
	if err := json.Unmarshal(data, &nestedResult); err != nil {
		return nil, false
	}
	Debug("Parsed response as map, looking for projects array")

	for _, key := range []string{"projects", "project", "resources", "data", "result", "appProjects"} {
		projData, ok := nestedResult[key]
		if !ok {
			continue
		}
		Debug("Found potential projects array in key: %s", key)

		projBytes, err := json.Marshal(projData)
		if err != nil {
			Debug("Error marshaling projects data: %v", err)
			continue
		}

		if projects, ok := tryParseProjectsFromNestedKey(projBytes); ok {
			Debug("Found %d projects in nested structure (key: %s)", len(projects), key)
			return projects, true
		}
	}
	return nil, false
}

func tryExtractProjectFromMap(m map[string]interface{}) (Project, bool) {
	if _, hasName := m["name"]; !hasName {
		return Project{}, false
	}
	project := Project{
		Name: fmt.Sprintf("%v", m["name"]),
	}
	if desc, hasDesc := m["description"]; hasDesc {
		project.Description = fmt.Sprintf("%v", desc)
	}
	return project, true
}

func tryParseProjectsMalformed(data []byte) ([]Project, bool) {
	if len(data) <= 10 || !strings.Contains(string(data), "\"name\"") {
		return nil, false
	}
	Debug("Trying to extract projects from potentially malformed response")

	var anyMap map[string]interface{}
	if err := json.Unmarshal(data, &anyMap); err != nil {
		return nil, false
	}

	var extracted []Project
	for _, value := range anyMap {
		if mapValue, isMap := value.(map[string]interface{}); isMap {
			if p, ok := tryExtractProjectFromMap(mapValue); ok {
				extracted = append(extracted, p)
			}
		} else if arrayValue, isArray := value.([]interface{}); isArray {
			for _, item := range arrayValue {
				if mapItem, isMap := item.(map[string]interface{}); isMap {
					if p, ok := tryExtractProjectFromMap(mapItem); ok {
						extracted = append(extracted, p)
					}
				}
			}
		}
	}

	if len(extracted) > 0 {
		Debug("Extracted %d projects from malformed response", len(extracted))
		return extracted, true
	}
	return nil, false
}

// tryGetProjects attempts to get projects from a specific endpoint
func (c *ArgoAPIClient) tryGetProjects(path string) ([]Project, error) {
	requestURL := path
	Debug("Making request to %s%s", c.BaseURL, requestURL)

	resp, err := c.makeRequest("GET", requestURL, nil)
	if err != nil {
		Debug("Error making request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorMsg := fmt.Sprintf("failed to get projects with status %d: %s", resp.StatusCode, string(bodyBytes))
		Debug("%s", errorMsg)
		return nil, fmt.Errorf(errorMsg)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	Debug("Response body: %s", string(bodyBytes))

	if projects, ok := tryParseProjectsSpecFormat(bodyBytes); ok {
		return projects, nil
	}
	if projects, ok := tryParseProjectsItemsArray(bodyBytes); ok {
		return projects, nil
	}
	if projects, ok := tryParseProjectsDirectArray(bodyBytes); ok {
		return projects, nil
	}
	if projects, ok := tryParseProjectsNested(bodyBytes); ok {
		return projects, nil
	}
	if projects, ok := tryParseProjectsMalformed(bodyBytes); ok {
		return projects, nil
	}

	if len(bodyBytes) > 10 {
		Debug("Received data but couldn't parse project list - likely format mismatch")
		return nil, fmt.Errorf("received data but couldn't parse project list: %s", string(bodyBytes[:100]))
	}

	Debug("No projects found in response")
	return nil, fmt.Errorf("no projects found in response")
}

// GetProject retrieves a specific project by name
func (c *ArgoAPIClient) GetProject(name string) (*Project, error) {
	resp, err := c.makeRequest("GET", "api/v1/projects/"+name, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get project with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var project Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, err
	}

	return &project, nil
}

// GetApplications retrieves all applications from ArgoCD
func (c *ArgoAPIClient) GetApplications() ([]Application, error) {
	// Try standard endpoint first
	apps, err := c.tryGetApplications("api/v1/applications")
	if err == nil {
		return apps, nil
	}

	Debug("Standard applications endpoint failed: %v", err)

	// Try alternate endpoints
	for _, path := range []string{
		"api/applications",
		"application-controller/api/v1/applications",
		"argocd/api/v1/applications",
	} {
		Debug("Trying alternate applications endpoint: %s", path)
		apps, err = c.tryGetApplications(path)
		if err == nil {
			return apps, nil
		}
		Debug("Applications endpoint %s failed: %v", path, err)
	}

	return nil, fmt.Errorf("failed to get applications from any known endpoint")
}

// tryGetApplications attempts to get applications from a specific endpoint
func (c *ArgoAPIClient) tryGetApplications(path string) ([]Application, error) {
	requestURL := path
	Debug("Making request to %s%s", c.BaseURL, requestURL)

	resp, err := c.makeRequest("GET", requestURL, nil)
	if err != nil {
		Debug("Error making request: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		errorMsg := fmt.Sprintf("failed to get applications with status %d: %s", resp.StatusCode, string(bodyBytes))
		Debug("%s", errorMsg)
		return nil, fmt.Errorf(errorMsg)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	Debug("Response body (partial): %s", truncateBodyForLogging(bodyBytes))

	// Try to parse as items array first
	var itemsResult struct {
		Items []Application `json:"items"`
	}
	if err := json.Unmarshal(bodyBytes, &itemsResult); err != nil {
		Debug("Failed to unmarshal as items array: %v", err)
	} else if len(itemsResult.Items) > 0 {
		Debug("Found %d applications in items array", len(itemsResult.Items))
		for i, app := range itemsResult.Items {
			Debug("Application %d: Name=%s, Project=%s, Health=%s, Sync=%s",
				i, app.Name, app.Project, app.Health.Status, app.Sync.Status)
		}
		return itemsResult.Items, nil
	}

	// Try direct array
	var directArray []Application
	if err := json.Unmarshal(bodyBytes, &directArray); err != nil {
		Debug("Failed to unmarshal as direct array: %v", err)
	} else if len(directArray) > 0 {
		Debug("Found %d applications in direct array", len(directArray))
		for i, app := range directArray {
			Debug("Application %d: Name=%s, Project=%s, Health=%s, Sync=%s",
				i, app.Name, app.Project, app.Health.Status, app.Sync.Status)
		}
		return directArray, nil
	}

	// Try to parse as nested structure
	var nestedResult map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &nestedResult); err == nil {
		Debug("Parsed response as map, looking for applications array")

		// Look for applications in different locations
		for _, key := range []string{"applications", "app", "apps", "resources"} {
			if appsData, ok := nestedResult[key]; ok {
				Debug("Found potential applications array in key: %s", key)

				// Marshal and unmarshal this section
				appsBytes, err := json.Marshal(appsData)
				if err != nil {
					Debug("Error marshaling applications data: %v", err)
					continue
				}

				// Try as array
				var appsArray []Application
				if err := json.Unmarshal(appsBytes, &appsArray); err != nil {
					Debug("Error unmarshaling as array: %v", err)
				} else if len(appsArray) > 0 {
					Debug("Found %d applications in nested structure", len(appsArray))
					return appsArray, nil
				}

				// Try as object with items
				var appsWithItems struct {
					Items []Application `json:"items"`
				}
				if err := json.Unmarshal(appsBytes, &appsWithItems); err != nil {
					Debug("Error unmarshaling as items container: %v", err)
				} else if len(appsWithItems.Items) > 0 {
					Debug("Found %d applications in nested items structure", len(appsWithItems.Items))
					return appsWithItems.Items, nil
				}
			}
		}
	}

	// If response has length but we couldn't parse it into any expected format
	if len(bodyBytes) > 10 {
		Debug("Received data but couldn't parse application list - likely format mismatch")
		return nil, fmt.Errorf("received data but couldn't parse application list: %s", truncateBodyForLogging(bodyBytes))
	}

	Debug("No applications found in response")
	return nil, fmt.Errorf("no applications found in response")
}

// truncateBodyForLogging returns a truncated version of the response body for logging
func truncateBodyForLogging(body []byte) string {
	maxLen := 500 // Maximum characters to log
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "..."
}

// GetApplication retrieves a specific application by name
func (c *ArgoAPIClient) GetApplication(name string) (*Application, error) {
	resp, err := c.makeRequest("GET", "api/v1/applications/"+name, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get application with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var application Application
	if err := json.NewDecoder(resp.Body).Decode(&application); err != nil {
		return nil, err
	}

	return &application, nil
}

// safeGo runs a function in a goroutine with panic recovery
func safeGo(f func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// Don't use fmt.Printf as it causes UI issues
				// This could be logged to a file or handled differently if needed
			}
		}()
		f()
	}()
}

// CreateProject creates a new project in ArgoCD
func (c *ArgoAPIClient) CreateProject(project *Project) error {
	projectJSON, err := json.Marshal(project)
	if err != nil {
		return err
	}

	resp, err := c.makeRequest("POST", "api/v1/projects", strings.NewReader(string(projectJSON)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create project with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeleteProject deletes a project from ArgoCD
func (c *ArgoAPIClient) DeleteProject(name string) error {
	resp, err := c.makeRequest("DELETE", "api/v1/projects/"+name, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete project with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// SyncApplication triggers a sync operation for an application
func (c *ArgoAPIClient) SyncApplication(name string) error {
	syncRequest := map[string]interface{}{
		"name":   name,
		"prune":  true,
		"dryRun": false,
		"strategy": map[string]interface{}{
			"hook": map[string]interface{}{
				"force": true,
			},
		},
	}

	syncJSON, err := json.Marshal(syncRequest)
	if err != nil {
		return err
	}

	resp, err := c.makeRequest("POST", "api/v1/applications/"+name+"/sync", strings.NewReader(string(syncJSON)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to sync application with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// RefreshApplication refreshes an application's state
func (c *ArgoAPIClient) RefreshApplication(name string) error {
	resp, err := c.makeRequest("GET", "api/v1/applications/"+name+"?refresh=normal", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to refresh application with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// DeleteApplication deletes an application from ArgoCD
func (c *ArgoAPIClient) DeleteApplication(name string) error {
	resp, err := c.makeRequest("DELETE", "api/v1/applications/"+name+"?cascade=true", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete application with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// CreateApplication creates a new application in ArgoCD
func (c *ArgoAPIClient) CreateApplication(app interface{}) error {
	appJSON, err := json.Marshal(app)
	if err != nil {
		return err
	}

	resp, err := c.makeRequest("POST", "api/v1/applications", strings.NewReader(string(appJSON)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create application with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func parseDestination(raw interface{}) (Destination, bool) {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return Destination{}, false
	}
	return Destination{
		Server:    extractStringFromMap(m, "server"),
		Name:      extractStringFromMap(m, "name"),
		Namespace: extractStringFromMap(m, "namespace"),
	}, true
}

func parseDestinations(spec map[string]interface{}) []Destination {
	items, ok := spec["destinations"].([]interface{})
	if !ok {
		return nil
	}
	var result []Destination
	for _, item := range items {
		if d, ok := parseDestination(item); ok {
			result = append(result, d)
		}
	}
	return result
}

func parseProjectRole(raw interface{}) (ProjectRole, bool) {
	m, ok := raw.(map[string]interface{})
	if !ok {
		return ProjectRole{}, false
	}
	return ProjectRole{
		Name:        extractStringFromMap(m, "name"),
		Description: extractStringFromMap(m, "description"),
		Policies:    extractStringSliceFromMap(m, "policies"),
		Groups:      extractStringSliceFromMap(m, "groups"),
	}, true
}

func parseProjectRoles(spec map[string]interface{}) []ProjectRole {
	items, ok := spec["roles"].([]interface{})
	if !ok {
		return nil
	}
	var result []ProjectRole
	for _, item := range items {
		if r, ok := parseProjectRole(item); ok {
			result = append(result, r)
		}
	}
	return result
}

func (p *Project) resolveFieldsFromSpec(spec map[string]interface{}) {
	if spec == nil {
		return
	}
	if len(p.Destinations) == 0 {
		if dests := parseDestinations(spec); len(dests) > 0 {
			p.Destinations = dests
		}
	}
	if len(p.SourceRepos) == 0 {
		if repos := extractStringSliceFromMap(spec, "sourceRepos"); len(repos) > 0 {
			p.SourceRepos = repos
		}
	}
	if p.Description == "" {
		p.Description = extractStringFromMap(spec, "description")
	}
	if len(p.Roles) == 0 {
		if roles := parseProjectRoles(spec); len(roles) > 0 {
			p.Roles = roles
		}
	}
}

// UnmarshalJSON custom unmarshaler for Project to handle potential missing fields
func (p *Project) UnmarshalJSON(data []byte) error {
	type Alias Project
	aux := &struct {
		*Alias
		Spec     map[string]interface{} `json:"spec"`
		Metadata map[string]interface{} `json:"metadata"`
	}{
		Alias: (*Alias)(p),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		Debug("Error unmarshaling Project: %v", err)
		return err
	}

	if p.Name == "" {
		p.Name = extractStringFromMap(aux.Metadata, "name")
	}

	p.resolveFieldsFromSpec(aux.Spec)

	return nil
}
