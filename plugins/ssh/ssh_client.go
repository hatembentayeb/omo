package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHClient struct {
	server      SSHServer
	client      *ssh.Client
	connected   bool
	connectedAt time.Time
}

type SSHExecResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

type SSHHostInfo struct {
	Hostname    string
	Uptime      string
	OS          string
	Kernel      string
	CPUCount    string
	MemTotal    string
	MemAvail    string
	DiskUsage   string
	LoadAvg     string
	IPAddresses []string
	LastLogin   string
}

func NewSSHClient(server SSHServer) *SSHClient {
	if server.Port == 0 {
		server.Port = 22
	}
	return &SSHClient{server: server}
}

func (c *SSHClient) Connect() error {
	config, err := c.buildSSHConfig()
	if err != nil {
		return fmt.Errorf("build config: %w", err)
	}

	addr := fmt.Sprintf("%s:%d", c.server.Host, c.server.Port)

	if c.server.ProxyCommand != "" {
		return c.connectViaProxy(config, addr)
	}
	if c.server.JumpHost != "" {
		return c.connectViaJumpHost(config, addr)
	}

	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}

	c.client = client
	c.connected = true
	c.connectedAt = time.Now()

	if c.server.StartupCmd != "" {
		go c.Execute(c.server.StartupCmd)
	}

	return nil
}

func (c *SSHClient) Disconnect() {
	if c.client != nil {
		c.client.Close()
	}
	c.connected = false
	c.client = nil
}

func (c *SSHClient) IsConnected() bool {
	if !c.connected || c.client == nil {
		return false
	}
	_, _, err := c.client.SendRequest("keepalive@omo", true, nil)
	if err != nil {
		c.connected = false
		return false
	}
	return true
}

func (c *SSHClient) Execute(command string) (*SSHExecResult, error) {
	if c.client == nil {
		return nil, fmt.Errorf("not connected")
	}

	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	defer session.Close()

	for k, v := range c.server.Env {
		session.Setenv(k, v)
	}

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	start := time.Now()
	exitCode := 0
	if err := session.Run(command); err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return nil, fmt.Errorf("run: %w", err)
		}
	}

	return &SSHExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Duration: time.Since(start),
	}, nil
}

func (c *SSHClient) GetHostInfo() (*SSHHostInfo, error) {
	info := &SSHHostInfo{}

	commands := map[string]*string{
		"hostname":     &info.Hostname,
		"uptime -p 2>/dev/null || uptime":                                                    &info.Uptime,
		"cat /etc/os-release 2>/dev/null | grep PRETTY_NAME | cut -d= -f2 | tr -d '\"'":     &info.OS,
		"uname -r":                                                                           &info.Kernel,
		"nproc 2>/dev/null || sysctl -n hw.ncpu 2>/dev/null || echo unknown":                 &info.CPUCount,
		"free -h 2>/dev/null | awk '/^Mem:/{print $2}' || echo unknown":                      &info.MemTotal,
		"free -h 2>/dev/null | awk '/^Mem:/{print $7}' || echo unknown":                      &info.MemAvail,
		"df -h / 2>/dev/null | awk 'NR==2{print $5}'":                                        &info.DiskUsage,
		"cat /proc/loadavg 2>/dev/null | awk '{print $1,$2,$3}' || echo unknown":             &info.LoadAvg,
		"last -1 2>/dev/null | head -1 || echo unknown":                                      &info.LastLogin,
	}

	for cmd, dest := range commands {
		result, err := c.Execute(cmd)
		if err == nil {
			*dest = strings.TrimSpace(result.Stdout)
		}
	}

	ipResult, err := c.Execute("hostname -I 2>/dev/null || ifconfig 2>/dev/null | grep 'inet ' | awk '{print $2}'")
	if err == nil {
		for _, ip := range strings.Fields(strings.TrimSpace(ipResult.Stdout)) {
			if ip != "" && ip != "127.0.0.1" {
				info.IPAddresses = append(info.IPAddresses, ip)
			}
		}
	}

	return info, nil
}

func (c *SSHClient) GetProcesses() ([][]string, error) {
	result, err := c.Execute("ps aux --sort=-%cpu 2>/dev/null | head -21 || ps aux | head -21")
	if err != nil {
		return nil, err
	}
	return parseTableOutput(result.Stdout), nil
}

func (c *SSHClient) GetDiskUsage() ([][]string, error) {
	result, err := c.Execute("df -h 2>/dev/null")
	if err != nil {
		return nil, err
	}
	return parseTableOutput(result.Stdout), nil
}

func (c *SSHClient) GetNetworkConnections() ([][]string, error) {
	result, err := c.Execute("ss -tunap 2>/dev/null | head -30 || netstat -tunap 2>/dev/null | head -30")
	if err != nil {
		return nil, err
	}
	return parseTableOutput(result.Stdout), nil
}

func (c *SSHClient) GetDockerContainers() ([][]string, error) {
	result, err := c.Execute("docker ps --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}\t{{.Ports}}' 2>/dev/null")
	if err != nil {
		return nil, err
	}
	if result.Stdout == "" {
		return nil, fmt.Errorf("docker not available")
	}
	return parseTableOutput(result.Stdout), nil
}

func (c *SSHClient) GetSystemdServices() ([][]string, error) {
	result, err := c.Execute("systemctl list-units --type=service --state=running --no-pager --no-legend 2>/dev/null | head -30")
	if err != nil {
		return nil, err
	}
	return parseTableOutput(result.Stdout), nil
}

func (c *SSHClient) GetServerAddress() string {
	return fmt.Sprintf("%s:%d", c.server.Host, c.server.Port)
}

func (c *SSHClient) GetConnectedDuration() time.Duration {
	if !c.connected {
		return 0
	}
	return time.Since(c.connectedAt)
}

// --- Auth & connection strategies ---

func (c *SSHClient) buildSSHConfig() (*ssh.ClientConfig, error) {
	authMethods := c.resolveAuthMethods()

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no auth method for %s@%s (set password, key_path, or private_key in KeePass)", c.server.User, c.server.Host)
	}

	return &ssh.ClientConfig{
		User:            c.server.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}, nil
}

func (c *SSHClient) resolveAuthMethods() []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	switch c.server.AuthMethod {
	case "key":
		methods = c.tryKeyAuth()
	case "password":
		if c.server.Password != "" {
			methods = append(methods, ssh.Password(c.server.Password))
		}
	default: // "auto"
		methods = c.tryKeyAuth()
		if c.server.Password != "" {
			methods = append(methods, ssh.Password(c.server.Password))
		}
	}

	return methods
}

func (c *SSHClient) tryKeyAuth() []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	if c.server.PrivateKey != "" {
		if signer, err := parsePrivateKey([]byte(c.server.PrivateKey), c.server.Passphrase); err == nil {
			methods = append(methods, ssh.PublicKeys(signer))
		}
	}

	if c.server.KeyPath != "" {
		if keyData, err := os.ReadFile(c.server.KeyPath); err == nil {
			if signer, err := parsePrivateKey(keyData, c.server.Passphrase); err == nil {
				methods = append(methods, ssh.PublicKeys(signer))
			}
		}
	}

	if len(methods) == 0 {
		for _, kp := range defaultKeyPaths() {
			keyData, err := os.ReadFile(kp)
			if err != nil {
				continue
			}
			signer, err := parsePrivateKey(keyData, "")
			if err != nil {
				continue
			}
			methods = append(methods, ssh.PublicKeys(signer))
			break
		}
	}

	return methods
}

func defaultKeyPaths() []string {
	return []string{
		os.ExpandEnv("$HOME/.ssh/id_ed25519"),
		os.ExpandEnv("$HOME/.ssh/id_rsa"),
		os.ExpandEnv("$HOME/.ssh/id_ecdsa"),
	}
}

func (c *SSHClient) connectViaProxy(config *ssh.ClientConfig, addr string) error {
	proxyCmd := strings.ReplaceAll(c.server.ProxyCommand, "%h", c.server.Host)
	proxyCmd = strings.ReplaceAll(proxyCmd, "%p", fmt.Sprintf("%d", c.server.Port))

	cmd := exec.Command("sh", "-c", proxyCmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("proxy stdin: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("proxy stdout: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start proxy: %w", err)
	}

	conn := &proxyConn{reader: stdout, writer: stdin, cmd: cmd}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		cmd.Process.Kill()
		return fmt.Errorf("ssh via proxy: %w", err)
	}

	c.client = ssh.NewClient(sshConn, chans, reqs)
	c.connected = true
	c.connectedAt = time.Now()
	return nil
}

func (c *SSHClient) connectViaJumpHost(config *ssh.ClientConfig, targetAddr string) error {
	jumpConfig := &ssh.ClientConfig{
		User:            c.server.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	parts := strings.SplitN(c.server.JumpHost, "@", 2)
	jumpAddr := c.server.JumpHost
	if len(parts) == 2 {
		jumpConfig.User = parts[0]
		jumpAddr = parts[1]
	}
	if !strings.Contains(jumpAddr, ":") {
		jumpAddr += ":22"
	}

	jumpConfig.Auth = c.resolveJumpAuth(config)

	jumpClient, err := ssh.Dial("tcp", jumpAddr, jumpConfig)
	if err != nil {
		return fmt.Errorf("dial jump %s: %w", jumpAddr, err)
	}

	conn, err := jumpClient.Dial("tcp", targetAddr)
	if err != nil {
		jumpClient.Close()
		return fmt.Errorf("dial target via jump: %w", err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, targetAddr, config)
	if err != nil {
		conn.Close()
		jumpClient.Close()
		return fmt.Errorf("ssh via jump: %w", err)
	}

	c.client = ssh.NewClient(sshConn, chans, reqs)
	c.connected = true
	c.connectedAt = time.Now()
	return nil
}

func (c *SSHClient) resolveJumpAuth(fallback *ssh.ClientConfig) []ssh.AuthMethod {
	if c.server.JumpKey != "" {
		if signer, err := parsePrivateKey([]byte(c.server.JumpKey), ""); err == nil {
			return []ssh.AuthMethod{ssh.PublicKeys(signer)}
		}
	}
	if c.server.JumpKeyPath != "" {
		if keyData, err := os.ReadFile(c.server.JumpKeyPath); err == nil {
			if signer, err := parsePrivateKey(keyData, ""); err == nil {
				return []ssh.AuthMethod{ssh.PublicKeys(signer)}
			}
		}
	}
	return fallback.Auth
}

func parsePrivateKey(keyData []byte, passphrase string) (ssh.Signer, error) {
	if passphrase != "" {
		return ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(passphrase))
	}
	return ssh.ParsePrivateKey(keyData)
}

func parseTableOutput(output string) [][]string {
	var rows [][]string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if fields := strings.Fields(line); len(fields) > 0 {
			rows = append(rows, fields)
		}
	}
	return rows
}

// proxyConn wraps a ProxyCommand process as a net.Conn.
type proxyConn struct {
	reader io.Reader
	writer io.WriteCloser
	cmd    *exec.Cmd
}

func (pc *proxyConn) Read(b []byte) (int, error)         { return pc.reader.Read(b) }
func (pc *proxyConn) Write(b []byte) (int, error)        { return pc.writer.Write(b) }
func (pc *proxyConn) Close() error                       { pc.writer.Close(); return pc.cmd.Process.Kill() }
func (pc *proxyConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (pc *proxyConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (pc *proxyConn) SetDeadline(t time.Time) error      { return nil }
func (pc *proxyConn) SetReadDeadline(t time.Time) error  { return nil }
func (pc *proxyConn) SetWriteDeadline(t time.Time) error { return nil }
