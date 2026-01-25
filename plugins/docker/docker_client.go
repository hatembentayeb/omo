package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
)

// LogFunc is a function type for logging messages
type LogFunc func(message string)

// DockerClient provides Docker operations using the Docker API client
type DockerClient struct {
	client   *client.Client
	ctx      context.Context
	timeout  time.Duration
	logFunc  LogFunc  // Function to log messages to the UI
	initLogs []string // Store initialization logs until logger is set
}

// NewDockerClient creates a new Docker client with context
func NewDockerClient() *DockerClient {
	ctx := context.Background()
	initLogs := []string{}

	// Get Docker host environment
	dockerHost := os.Getenv("DOCKER_HOST")
	// Store log instead of printing to console
	initLogs = append(initLogs, fmt.Sprintf("Docker host from environment: %s", dockerHost))

	// Try with explicit host first if provided
	var cli *client.Client
	var err error

	if dockerHost != "" {
		initLogs = append(initLogs, "Creating Docker client with explicit host...")
		cli, err = client.NewClientWithOpts(
			client.WithHost(dockerHost),
			client.WithAPIVersionNegotiation(),
		)
	} else {
		initLogs = append(initLogs, "Creating Docker client from environment...")
		cli, err = client.NewClientWithOpts(
			client.FromEnv,
			client.WithAPIVersionNegotiation(),
		)
	}

	if err != nil {
		initLogs = append(initLogs, fmt.Sprintf("Warning: Failed to initialize Docker client: %v", err))
		initLogs = append(initLogs, "Trying with default Docker socket...")
		cli, err = client.NewClientWithOpts(client.WithAPIVersionNegotiation())
		if err != nil {
			initLogs = append(initLogs, fmt.Sprintf("Still couldn't connect to Docker: %v", err))
		}
	}

	// Verify connection
	if cli != nil {
		// Ping with short timeout
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		_, err := cli.Ping(pingCtx)
		if err != nil {
			initLogs = append(initLogs, fmt.Sprintf("Warning: Docker connection test failed: %v", err))
		} else {
			initLogs = append(initLogs, "Successfully connected to Docker daemon!")
		}
	}

	return &DockerClient{
		client:   cli,
		ctx:      ctx,
		timeout:  10 * time.Second,    // Default timeout for operations
		logFunc:  func(msg string) {}, // Default no-op logger
		initLogs: initLogs,            // Store initialization logs
	}
}

// SetLogger sets the logging function for the Docker client
func (d *DockerClient) SetLogger(logFunc LogFunc) {
	d.logFunc = logFunc

	// Log initial connection status
	if logFunc != nil {
		// First output any initialization logs that were captured
		for _, msg := range d.initLogs {
			d.logFunc(fmt.Sprintf("[dim]%s[white]", msg))
		}

		// Clear init logs after displaying them
		d.initLogs = nil

		if d.client != nil {
			d.logFunc("[blue]Docker client initialized")

			if dockerHost := os.Getenv("DOCKER_HOST"); dockerHost != "" {
				d.logFunc(fmt.Sprintf("[yellow]Using custom Docker host: %s", dockerHost))
			}
		} else {
			d.logFunc("[red]Failed to initialize Docker client")
		}
	}
}

// ListContainers returns a list of Docker containers
func (d *DockerClient) ListContainers() ([]DockerContainer, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	// Log to UI that we're fetching containers
	if d.logFunc != nil {
		d.logFunc("[yellow]Fetching containers...")
	}

	// Check if client is initialized
	if d.client == nil {
		if d.logFunc != nil {
			d.logFunc("[red]Docker client not initialized")
		}
		return nil, fmt.Errorf("docker client not initialized")
	}

	// List containers
	containers, err := d.client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error fetching containers: %v", err))
		}
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	// Add debug logging to UI only, not console
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[green]Found %d containers", len(containers)))
	}

	// Convert to our container type
	result := make([]DockerContainer, 0, len(containers))
	for _, c := range containers {
		// Convert ports to strings
		portsStr := []string{}
		for _, p := range c.Ports {
			if p.PublicPort > 0 {
				portsStr = append(portsStr, fmt.Sprintf("%d:%d/%s", p.PublicPort, p.PrivatePort, p.Type))
			} else {
				portsStr = append(portsStr, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
			}
		}

		// Get a nice container name without leading slash
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		result = append(result, DockerContainer{
			ID:          c.ID,
			Name:        name,
			Image:       c.Image,
			Status:      c.Status,
			State:       c.State,
			Created:     time.Unix(c.Created, 0),
			Ports:       portsStr,
			NetworkMode: "", // Would need additional inspection for this
		})
	}

	return result, nil
}

// ListImages returns a list of Docker images
func (d *DockerClient) ListImages() ([]DockerImage, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	// Log to UI that we're fetching images
	if d.logFunc != nil {
		d.logFunc("[yellow]Fetching images...")
	}

	// Check if client is initialized
	if d.client == nil {
		if d.logFunc != nil {
			d.logFunc("[red]Docker client not initialized")
		}
		return nil, fmt.Errorf("docker client not initialized")
	}

	// List images
	images, err := d.client.ImageList(ctx, image.ListOptions{All: true})
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error fetching images: %v", err))
		}
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	// Add debug logging to UI
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[green]Found %d images", len(images)))
	}

	// Convert to our image type
	result := make([]DockerImage, 0, len(images))
	for _, img := range images {
		// Format size
		size := formatSize(float64(img.Size))

		// Extract repository and tag from RepoTags
		repo, tag := "<none>", "<none>"
		if len(img.RepoTags) > 0 && img.RepoTags[0] != "<none>:<none>" {
			parts := strings.Split(img.RepoTags[0], ":")
			if len(parts) >= 2 {
				repo = parts[0]
				tag = parts[1]
			}
		}

		// Get the creation time
		created := time.Unix(img.Created, 0)
		createdSince := formatTimeAgo(created)

		// Get the short ID
		shortID := img.ID
		if len(shortID) > 12 {
			shortID = shortID[7:19] // Remove "sha256:" prefix and get first 12 chars
		}

		result = append(result, DockerImage{
			ID:           shortID,
			Repository:   repo,
			Tag:          tag,
			Size:         size,
			Created:      created,
			CreatedSince: createdSince,
			Digest:       getDigestFromID(img.ID),
		})
	}

	return result, nil
}

// ListNetworks returns a list of Docker networks
func (d *DockerClient) ListNetworks() ([]DockerNetwork, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	// Log to UI that we're fetching networks
	if d.logFunc != nil {
		d.logFunc("[yellow]Fetching networks...")
	}

	// Check if client is initialized
	if d.client == nil {
		if d.logFunc != nil {
			d.logFunc("[red]Docker client not initialized")
		}
		return nil, fmt.Errorf("docker client not initialized")
	}

	// List networks
	networks, err := d.client.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error fetching networks: %v", err))
		}
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	// Add debug logging to UI
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[green]Found %d networks", len(networks)))
	}

	// Convert to our network type
	result := make([]DockerNetwork, 0, len(networks))
	for _, n := range networks {
		network := DockerNetwork{
			ID:      n.ID,
			Name:    n.Name,
			Driver:  n.Driver,
			Scope:   n.Scope,
			Created: time.Time{}, // Creation time not available in NetworkResource
		}

		// Try to get subnet and gateway
		if len(n.IPAM.Config) > 0 {
			network.Subnet = n.IPAM.Config[0].Subnet
			network.Gateway = n.IPAM.Config[0].Gateway
		}

		result = append(result, network)
	}

	return result, nil
}

// ListVolumes returns a list of Docker volumes
func (d *DockerClient) ListVolumes() ([]DockerVolume, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	// Log to UI that we're fetching volumes
	if d.logFunc != nil {
		d.logFunc("[yellow]Fetching volumes...")
	}

	// Check if client is initialized
	if d.client == nil {
		if d.logFunc != nil {
			d.logFunc("[red]Docker client not initialized")
		}
		return nil, fmt.Errorf("docker client not initialized")
	}

	// List volumes
	volumes, err := d.client.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error fetching volumes: %v", err))
		}
		return nil, fmt.Errorf("failed to list volumes: %w", err)
	}

	// Add debug logging to UI
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[green]Found %d volumes", len(volumes.Volumes)))
	}

	// Convert to our volume type
	result := make([]DockerVolume, 0, len(volumes.Volumes))
	for _, vol := range volumes.Volumes {
		if vol == nil {
			continue
		}

		// Convert labels
		labels := make(map[string]string)
		if vol.Labels != nil {
			for k, v := range vol.Labels {
				labels[k] = v
			}
		}

		// Create volume record
		result = append(result, DockerVolume{
			Name:       vol.Name,
			Driver:     vol.Driver,
			Mountpoint: vol.Mountpoint,
			Scope:      vol.Scope,
			CreatedAt:  vol.CreatedAt,
			Created:    time.Time{}, // Creation time not provided by API
			Labels:     labels,
			Size:       "", // Size not provided by API
		})
	}

	return result, nil
}

// StartContainer starts a Docker container
func (d *DockerClient) StartContainer(containerID string) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	// Log to UI that we're starting container
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[yellow]Starting container %s...", containerID[:12]))
	}

	// Start the container
	err := d.client.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error starting container: %v", err))
		}
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Log success to UI
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[green]Container %s started successfully", containerID[:12]))
	}

	return nil
}

// StopContainer stops a Docker container
func (d *DockerClient) StopContainer(containerID string) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(d.ctx, 30*time.Second) // Longer timeout for stop
	defer cancel()

	// Log to UI that we're stopping container
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[yellow]Stopping container %s...", containerID[:12]))
	}

	// Use default 10s timeout for stop
	timeout := 10

	// Stop the container
	err := d.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error stopping container: %v", err))
		}
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Log success to UI
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[green]Container %s stopped successfully", containerID[:12]))
	}

	return nil
}

// RemoveContainer removes a Docker container
func (d *DockerClient) RemoveContainer(containerID string) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	// Log to UI that we're removing container
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[yellow]Removing container %s...", containerID[:12]))
	}

	// Remove the container
	err := d.client.ContainerRemove(ctx, containerID, container.RemoveOptions{})
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error removing container: %v", err))
		}
		return fmt.Errorf("failed to remove container: %w", err)
	}

	// Log success to UI
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[green]Container %s removed successfully", containerID[:12]))
	}

	return nil
}

// GetContainerLogs gets logs for a Docker container
func (d *DockerClient) GetContainerLogs(containerID string) (string, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	// Log to UI that we're fetching logs
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[yellow]Fetching logs for container %s...", containerID[:12]))
	}

	// Get container logs
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Tail:       "500", // Last 500 lines
	}

	// Get logs reader
	logsReader, err := d.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error fetching container logs: %v", err))
		}
		return "", fmt.Errorf("failed to get container logs: %w", err)
	}
	defer logsReader.Close()

	// Read all logs
	logs, err := io.ReadAll(logsReader)
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error reading container logs: %v", err))
		}
		return "", fmt.Errorf("failed to read container logs: %w", err)
	}

	// Log success to UI
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[green]Retrieved %d bytes of logs from container %s", len(logs), containerID[:12]))
	}

	return string(logs), nil
}

// ExecInContainer executes a command in a running container
func (d *DockerClient) ExecInContainer(containerID, command string) (string, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	// Log to UI that we're executing command
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[yellow]Executing command in container %s: %s", containerID[:12], command))
	}

	// Create exec configuration
	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          []string{"sh", "-c", command},
	}

	// Create exec instance
	execID, err := d.client.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error creating exec instance: %v", err))
		}
		return "", fmt.Errorf("failed to create exec instance: %w", err)
	}

	// Attach to exec instance
	resp, err := d.client.ContainerExecAttach(ctx, execID.ID, container.ExecStartOptions{})
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error attaching to exec instance: %v", err))
		}
		return "", fmt.Errorf("failed to attach to exec instance: %w", err)
	}
	defer resp.Close()

	// Read output
	output, err := io.ReadAll(resp.Reader)
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error reading exec output: %v", err))
		}
		return "", fmt.Errorf("failed to read exec output: %w", err)
	}

	// Get exec status to check if command succeeded
	inspectResp, err := d.client.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[yellow]Warning: failed to inspect exec instance: %v", err))
		}
		return string(output), fmt.Errorf("failed to inspect exec instance: %w", err)
	}

	// Log success or failure based on exit code
	if d.logFunc != nil {
		if inspectResp.ExitCode == 0 {
			d.logFunc(fmt.Sprintf("[green]Command executed successfully in container %s", containerID[:12]))
		} else {
			d.logFunc(fmt.Sprintf("[red]Command in container %s failed with exit code %d", containerID[:12], inspectResp.ExitCode))
		}
	}

	if inspectResp.ExitCode != 0 {
		return string(output), fmt.Errorf("command failed with exit code %d", inspectResp.ExitCode)
	}

	return string(output), nil
}

// PruneSystem prunes unused Docker resources
func (d *DockerClient) PruneSystem() (string, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(d.ctx, 2*time.Minute) // Longer timeout for prune
	defer cancel()

	// Log to UI that we're pruning system
	if d.logFunc != nil {
		d.logFunc("[yellow]Pruning Docker system (containers, networks, images)...")
	}

	// Prune containers
	containersReport, err := d.client.ContainersPrune(ctx, filters.Args{})
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error pruning containers: %v", err))
		}
		return "", fmt.Errorf("failed to prune containers: %w", err)
	}

	// Log containers pruned
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[blue]Pruned %d containers, reclaimed %d bytes", len(containersReport.ContainersDeleted), containersReport.SpaceReclaimed))
	}

	// Prune networks
	networksReport, err := d.client.NetworksPrune(ctx, filters.Args{})
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error pruning networks: %v", err))
		}
		return "", fmt.Errorf("failed to prune networks: %w", err)
	}

	// Log networks pruned
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[blue]Pruned %d networks", len(networksReport.NetworksDeleted)))
	}

	// Prune images
	imagesReport, err := d.client.ImagesPrune(ctx, filters.Args{})
	if err != nil {
		if d.logFunc != nil {
			d.logFunc(fmt.Sprintf("[red]Error pruning images: %v", err))
		}
		return "", fmt.Errorf("failed to prune images: %w", err)
	}

	// Log images pruned
	if d.logFunc != nil {
		d.logFunc(fmt.Sprintf("[blue]Pruned %d images, reclaimed %d bytes", len(imagesReport.ImagesDeleted), imagesReport.SpaceReclaimed))
	}

	// Log success to UI
	if d.logFunc != nil {
		d.logFunc("[green]Docker system pruned successfully")
	}

	// Create a formatted summary
	summary := fmt.Sprintf("Prune Summary:\n\n"+
		"Containers: %d pruned, %d bytes reclaimed\n"+
		"Networks: %d pruned\n"+
		"Images: %d pruned, %d bytes reclaimed\n\n"+
		"Total space reclaimed: %d bytes",
		len(containersReport.ContainersDeleted), containersReport.SpaceReclaimed,
		len(networksReport.NetworksDeleted),
		len(imagesReport.ImagesDeleted), imagesReport.SpaceReclaimed,
		containersReport.SpaceReclaimed+imagesReport.SpaceReclaimed)

	return summary, nil
}

// Helper functions

// formatSize converts bytes to human-readable size format
func formatSize(size float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	unitIndex := 0

	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}

	return fmt.Sprintf("%.2f %s", size, units[unitIndex])
}

// formatTimeAgo formats time as a human-readable "time ago" string
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "Just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%d minute%s ago", minutes, plural(minutes))
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		return fmt.Sprintf("%d hour%s ago", hours, plural(hours))
	} else if duration < 30*24*time.Hour {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d day%s ago", days, plural(days))
	} else if duration < 365*24*time.Hour {
		months := int(duration.Hours() / 24 / 30)
		return fmt.Sprintf("%d month%s ago", months, plural(months))
	}

	years := int(duration.Hours() / 24 / 365)
	return fmt.Sprintf("%d year%s ago", years, plural(years))
}

// plural returns "s" if count is not 1
func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

// getDigestFromID extracts digest from image ID
func getDigestFromID(id string) string {
	// Image ID is usually in the format "sha256:1234567890abcdef..."
	parts := strings.Split(id, ":")
	if len(parts) == 2 {
		return parts[1]
	}
	return id
}

// ConnectToHost connects to a specific Docker host
func (d *DockerClient) ConnectToHost(host DockerHost) error {
	var cli *client.Client
	var err error

	if host.Host != "" {
		cli, err = client.NewClientWithOpts(
			client.WithHost(host.Host),
			client.WithAPIVersionNegotiation(),
		)
	} else {
		cli, err = client.NewClientWithOpts(
			client.FromEnv,
			client.WithAPIVersionNegotiation(),
		)
	}

	if err != nil {
		return fmt.Errorf("failed to create Docker client: %w", err)
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("failed to connect to Docker: %w", err)
	}

	d.client = cli
	return nil
}

// RestartContainer restarts a Docker container
func (d *DockerClient) RestartContainer(containerID string) error {
	ctx, cancel := context.WithTimeout(d.ctx, 30*time.Second)
	defer cancel()

	timeout := 10
	return d.client.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: &timeout})
}

// PauseContainer pauses a Docker container
func (d *DockerClient) PauseContainer(containerID string) error {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	return d.client.ContainerPause(ctx, containerID)
}

// UnpauseContainer unpauses a Docker container
func (d *DockerClient) UnpauseContainer(containerID string) error {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	return d.client.ContainerUnpause(ctx, containerID)
}

// KillContainer kills a Docker container
func (d *DockerClient) KillContainer(containerID string) error {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	return d.client.ContainerKill(ctx, containerID, "SIGKILL")
}

// ContainerInspectInfo holds detailed container information
type ContainerInspectInfo struct {
	ID           string
	Name         string
	Image        string
	Created      string
	State        string
	Status       string
	Platform     string
	RestartCount int
	Ports        []string
	Mounts       []string
	Networks     []string
	Env          []string
}

// InspectContainer returns detailed information about a container
func (d *DockerClient) InspectContainer(containerID string) (*ContainerInspectInfo, error) {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	inspect, err := d.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}

	info := &ContainerInspectInfo{
		ID:           inspect.ID,
		Name:         strings.TrimPrefix(inspect.Name, "/"),
		Image:        inspect.Config.Image,
		Created:      inspect.Created,
		State:        inspect.State.Status,
		Status:       fmt.Sprintf("%s (ExitCode: %d)", inspect.State.Status, inspect.State.ExitCode),
		Platform:     inspect.Platform,
		RestartCount: inspect.RestartCount,
		Env:          inspect.Config.Env,
	}

	// Extract ports
	for port, bindings := range inspect.NetworkSettings.Ports {
		for _, binding := range bindings {
			info.Ports = append(info.Ports, fmt.Sprintf("%s:%s->%s", binding.HostIP, binding.HostPort, port))
		}
	}

	// Extract mounts
	for _, mount := range inspect.Mounts {
		info.Mounts = append(info.Mounts, fmt.Sprintf("%s -> %s (%s)", mount.Source, mount.Destination, mount.Type))
	}

	// Extract networks
	for name, network := range inspect.NetworkSettings.Networks {
		info.Networks = append(info.Networks, fmt.Sprintf("%s: %s", name, network.IPAddress))
	}

	return info, nil
}

// ImageInspectInfo holds detailed image information
type ImageInspectInfo struct {
	ID            string
	RepoTags      []string
	RepoDigests   []string
	Created       string
	Size          string
	Architecture  string
	OS            string
	DockerVersion string
	ExposedPorts  []string
	Env           []string
}

// InspectImage returns detailed information about an image
func (d *DockerClient) InspectImage(imageID string) (*ImageInspectInfo, error) {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	inspect, _, err := d.client.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		return nil, err
	}

	info := &ImageInspectInfo{
		ID:            inspect.ID,
		RepoTags:      inspect.RepoTags,
		RepoDigests:   inspect.RepoDigests,
		Created:       inspect.Created,
		Size:          formatSize(float64(inspect.Size)),
		Architecture:  inspect.Architecture,
		OS:            inspect.Os,
		DockerVersion: inspect.DockerVersion,
	}

	if inspect.Config != nil {
		for port := range inspect.Config.ExposedPorts {
			info.ExposedPorts = append(info.ExposedPorts, string(port))
		}
		info.Env = inspect.Config.Env
	}

	return info, nil
}

// RemoveImage removes a Docker image
func (d *DockerClient) RemoveImage(imageID string) error {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	_, err := d.client.ImageRemove(ctx, imageID, image.RemoveOptions{Force: false, PruneChildren: true})
	return err
}

// PullImage pulls a Docker image
func (d *DockerClient) PullImage(imageName string) error {
	ctx, cancel := context.WithTimeout(d.ctx, 5*time.Minute)
	defer cancel()

	reader, err := d.client.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()

	// Read the output to completion
	_, err = io.Copy(io.Discard, reader)
	return err
}

// GetImageHistory returns the history of an image
func (d *DockerClient) GetImageHistory(imageID string) (string, error) {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	history, err := d.client.ImageHistory(ctx, imageID)
	if err != nil {
		return "", err
	}

	var result strings.Builder
	for _, h := range history {
		created := formatTimeAgo(time.Unix(h.Created, 0))
		size := formatSize(float64(h.Size))
		createdBy := h.CreatedBy
		if len(createdBy) > 60 {
			createdBy = createdBy[:60] + "..."
		}
		result.WriteString(fmt.Sprintf("[green]%s[white] (%s)\n  %s\n\n", created, size, createdBy))
	}

	return result.String(), nil
}

// CreateContainer creates a new container from an image
func (d *DockerClient) CreateContainer(imageName, containerName string) (string, error) {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	config := &container.Config{
		Image: imageName,
	}

	resp, err := d.client.ContainerCreate(ctx, config, nil, nil, nil, containerName)
	if err != nil {
		return "", err
	}

	return resp.ID, nil
}

// NetworkInspectInfo holds detailed network information
type NetworkInspectInfo struct {
	ID         string
	Name       string
	Driver     string
	Scope      string
	Internal   bool
	Attachable bool
	EnableIPv6 bool
	Subnet     string
	Gateway    string
	Containers []string
	Labels     map[string]string
}

// InspectNetwork returns detailed information about a network
func (d *DockerClient) InspectNetwork(networkID string) (*NetworkInspectInfo, error) {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	inspect, err := d.client.NetworkInspect(ctx, networkID, network.InspectOptions{})
	if err != nil {
		return nil, err
	}

	info := &NetworkInspectInfo{
		ID:         inspect.ID,
		Name:       inspect.Name,
		Driver:     inspect.Driver,
		Scope:      inspect.Scope,
		Internal:   inspect.Internal,
		Attachable: inspect.Attachable,
		EnableIPv6: inspect.EnableIPv6,
		Labels:     inspect.Labels,
	}

	if len(inspect.IPAM.Config) > 0 {
		info.Subnet = inspect.IPAM.Config[0].Subnet
		info.Gateway = inspect.IPAM.Config[0].Gateway
	}

	for _, container := range inspect.Containers {
		info.Containers = append(info.Containers, fmt.Sprintf("%s (%s)", container.Name, container.IPv4Address))
	}

	return info, nil
}

// RemoveNetwork removes a Docker network
func (d *DockerClient) RemoveNetwork(networkID string) error {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	return d.client.NetworkRemove(ctx, networkID)
}

// CreateNetwork creates a new Docker network
func (d *DockerClient) CreateNetwork(name, driver string) error {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	_, err := d.client.NetworkCreate(ctx, name, network.CreateOptions{
		Driver: driver,
	})
	return err
}

// VolumeInspectInfo holds detailed volume information
type VolumeInspectInfo struct {
	Name       string
	Driver     string
	Mountpoint string
	Scope      string
	CreatedAt  string
	Labels     map[string]string
	Options    map[string]string
	UsageData  *VolumeUsageData
}

// VolumeUsageData holds volume usage information
type VolumeUsageData struct {
	Size     string
	RefCount int64
}

// InspectVolume returns detailed information about a volume
func (d *DockerClient) InspectVolume(name string) (*VolumeInspectInfo, error) {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	inspect, err := d.client.VolumeInspect(ctx, name)
	if err != nil {
		return nil, err
	}

	info := &VolumeInspectInfo{
		Name:       inspect.Name,
		Driver:     inspect.Driver,
		Mountpoint: inspect.Mountpoint,
		Scope:      inspect.Scope,
		CreatedAt:  inspect.CreatedAt,
		Labels:     inspect.Labels,
		Options:    inspect.Options,
	}

	if inspect.UsageData != nil {
		info.UsageData = &VolumeUsageData{
			Size:     formatSize(float64(inspect.UsageData.Size)),
			RefCount: inspect.UsageData.RefCount,
		}
	}

	return info, nil
}

// RemoveVolume removes a Docker volume
func (d *DockerClient) RemoveVolume(name string) error {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	return d.client.VolumeRemove(ctx, name, false)
}

// CreateVolume creates a new Docker volume
func (d *DockerClient) CreateVolume(name string) error {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	_, err := d.client.VolumeCreate(ctx, volume.CreateOptions{
		Name: name,
	})
	return err
}

// PruneVolumes removes all unused volumes
func (d *DockerClient) PruneVolumes() (string, error) {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	report, err := d.client.VolumesPrune(ctx, filters.Args{})
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("Removed %d volumes, reclaimed %s",
		len(report.VolumesDeleted), formatSize(float64(report.SpaceReclaimed))), nil
}

// ContainerStats holds container resource statistics
type ContainerStats struct {
	Name          string
	CPUPercent    string
	MemoryUsage   string
	MemoryPercent string
	NetIO         string
	BlockIO       string
	PIDs          string
}

// GetContainerStats returns resource usage statistics for all running containers
func (d *DockerClient) GetContainerStats() ([]ContainerStats, error) {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	containers, err := d.client.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("status", "running")),
	})
	if err != nil {
		return nil, err
	}

	var stats []ContainerStats
	for _, c := range containers {
		statsResp, err := d.client.ContainerStats(ctx, c.ID, false)
		if err != nil {
			continue
		}

		var statsJSON container.StatsResponse
		if err := json.NewDecoder(statsResp.Body).Decode(&statsJSON); err != nil {
			statsResp.Body.Close()
			continue
		}
		statsResp.Body.Close()

		// Calculate CPU percentage
		cpuPercent := calculateCPUPercent(&statsJSON)

		// Calculate memory
		memUsage := formatSize(float64(statsJSON.MemoryStats.Usage))
		memLimit := float64(statsJSON.MemoryStats.Limit)
		memPercent := float64(0)
		if memLimit > 0 {
			memPercent = float64(statsJSON.MemoryStats.Usage) / memLimit * 100
		}

		// Network I/O
		var netRx, netTx uint64
		for _, v := range statsJSON.Networks {
			netRx += v.RxBytes
			netTx += v.TxBytes
		}

		// Block I/O
		var blkRead, blkWrite uint64
		for _, v := range statsJSON.BlkioStats.IoServiceBytesRecursive {
			if v.Op == "Read" {
				blkRead += v.Value
			} else if v.Op == "Write" {
				blkWrite += v.Value
			}
		}

		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		stats = append(stats, ContainerStats{
			Name:          name,
			CPUPercent:    fmt.Sprintf("%.2f%%", cpuPercent),
			MemoryUsage:   memUsage,
			MemoryPercent: fmt.Sprintf("%.2f%%", memPercent),
			NetIO:         fmt.Sprintf("%s / %s", formatSize(float64(netRx)), formatSize(float64(netTx))),
			BlockIO:       fmt.Sprintf("%s / %s", formatSize(float64(blkRead)), formatSize(float64(blkWrite))),
			PIDs:          fmt.Sprintf("%d", statsJSON.PidsStats.Current),
		})
	}

	return stats, nil
}

func calculateCPUPercent(stats *container.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)

	if systemDelta > 0 && cpuDelta > 0 {
		return (cpuDelta / systemDelta) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}
	return 0.0
}

// SystemInfo holds Docker system information
type SystemInfo struct {
	ServerVersion     string
	APIVersion        string
	OperatingSystem   string
	Architecture      string
	KernelVersion     string
	MemTotal          string
	NCPU              int
	Containers        int
	ContainersRunning int
	ContainersPaused  int
	ContainersStopped int
	Images            int
	Driver            string
	LoggingDriver     string
	CgroupDriver      string
	CgroupVersion     string
	DockerRootDir     string
	SwarmStatus       string
}

// GetSystemInfo returns Docker system information
func (d *DockerClient) GetSystemInfo() (*SystemInfo, error) {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	info, err := d.client.Info(ctx)
	if err != nil {
		return nil, err
	}

	swarmStatus := "Inactive"
	if info.Swarm.LocalNodeState == "active" {
		swarmStatus = "Active"
	}

	return &SystemInfo{
		ServerVersion:     info.ServerVersion,
		APIVersion:        d.client.ClientVersion(),
		OperatingSystem:   info.OperatingSystem,
		Architecture:      info.Architecture,
		KernelVersion:     info.KernelVersion,
		MemTotal:          formatSize(float64(info.MemTotal)),
		NCPU:              info.NCPU,
		Containers:        info.Containers,
		ContainersRunning: info.ContainersRunning,
		ContainersPaused:  info.ContainersPaused,
		ContainersStopped: info.ContainersStopped,
		Images:            info.Images,
		Driver:            info.Driver,
		LoggingDriver:     info.LoggingDriver,
		CgroupDriver:      info.CgroupDriver,
		CgroupVersion:     info.CgroupVersion,
		DockerRootDir:     info.DockerRootDir,
		SwarmStatus:       swarmStatus,
	}, nil
}

// DiskUsage holds Docker disk usage information
type DiskUsage struct {
	ImagesCount           int
	ImagesSize            string
	ImagesReclaimable     string
	ContainersCount       int
	ContainersSize        string
	ContainersReclaimable string
	VolumesCount          int
	VolumesSize           string
	VolumesReclaimable    string
	BuildCacheCount       int
	BuildCacheSize        string
	BuildCacheReclaimable string
	TotalReclaimable      string
}

// GetDiskUsage returns Docker disk usage information
func (d *DockerClient) GetDiskUsage() (*DiskUsage, error) {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	du, err := d.client.DiskUsage(ctx, types.DiskUsageOptions{})
	if err != nil {
		return nil, err
	}

	var imagesSize, imagesReclaimable int64
	for _, img := range du.Images {
		imagesSize += img.Size
		if img.Containers == 0 {
			imagesReclaimable += img.Size
		}
	}

	var containersSize, containersReclaimable int64
	for _, c := range du.Containers {
		containersSize += c.SizeRw
		if c.State != "running" {
			containersReclaimable += c.SizeRw
		}
	}

	var volumesSize, volumesReclaimable int64
	for _, v := range du.Volumes {
		if v.UsageData.Size > 0 {
			volumesSize += v.UsageData.Size
			if v.UsageData.RefCount == 0 {
				volumesReclaimable += v.UsageData.Size
			}
		}
	}

	var buildCacheSize, buildCacheReclaimable int64
	for _, bc := range du.BuildCache {
		buildCacheSize += bc.Size
		if !bc.InUse {
			buildCacheReclaimable += bc.Size
		}
	}

	return &DiskUsage{
		ImagesCount:           len(du.Images),
		ImagesSize:            formatSize(float64(imagesSize)),
		ImagesReclaimable:     formatSize(float64(imagesReclaimable)),
		ContainersCount:       len(du.Containers),
		ContainersSize:        formatSize(float64(containersSize)),
		ContainersReclaimable: formatSize(float64(containersReclaimable)),
		VolumesCount:          len(du.Volumes),
		VolumesSize:           formatSize(float64(volumesSize)),
		VolumesReclaimable:    formatSize(float64(volumesReclaimable)),
		BuildCacheCount:       len(du.BuildCache),
		BuildCacheSize:        formatSize(float64(buildCacheSize)),
		BuildCacheReclaimable: formatSize(float64(buildCacheReclaimable)),
		TotalReclaimable:      formatSize(float64(imagesReclaimable + containersReclaimable + volumesReclaimable + buildCacheReclaimable)),
	}, nil
}

// GetRecentEvents returns recent Docker events
func (d *DockerClient) GetRecentEvents() (string, error) {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	since := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	eventsChan, errChan := d.client.Events(ctx, events.ListOptions{
		Since: since,
	})

	var result strings.Builder
	eventCount := 0
	maxEvents := 50

loop:
	for {
		select {
		case event := <-eventsChan:
			eventTime := time.Unix(event.Time, 0).Format("15:04:05")
			result.WriteString(fmt.Sprintf("[green]%s[white] [%s] %s: %s\n",
				eventTime, event.Type, event.Action, event.Actor.ID[:12]))
			eventCount++
			if eventCount >= maxEvents {
				break loop
			}
		case err := <-errChan:
			if err != nil && err != context.DeadlineExceeded {
				return result.String(), err
			}
			break loop
		case <-ctx.Done():
			break loop
		}
	}

	return result.String(), nil
}

// GetContainerLogsStream returns container logs with follow capability
func (d *DockerClient) GetContainerLogsStream(containerID string, tailLines int) (string, error) {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Tail:       fmt.Sprintf("%d", tailLines),
	}

	logsReader, err := d.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", err
	}
	defer logsReader.Close()

	logs, err := io.ReadAll(logsReader)
	if err != nil {
		return "", err
	}

	return string(logs), nil
}

// ComposeProject holds Docker Compose project information
type ComposeProject struct {
	Name         string
	Status       string
	ServiceCount int
	RunningCount int
	ConfigFile   string
}

// ListComposeProjects returns a list of Docker Compose projects
func (d *DockerClient) ListComposeProjects() ([]ComposeProject, error) {
	ctx, cancel := context.WithTimeout(d.ctx, d.timeout)
	defer cancel()

	// Get containers with compose labels
	containers, err := d.client.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, err
	}

	// Group by project
	projects := make(map[string]*ComposeProject)
	for _, c := range containers {
		projectName, ok := c.Labels["com.docker.compose.project"]
		if !ok {
			continue
		}

		if _, exists := projects[projectName]; !exists {
			configFile := c.Labels["com.docker.compose.project.config_files"]
			if len(configFile) > 40 {
				configFile = "..." + configFile[len(configFile)-37:]
			}

			projects[projectName] = &ComposeProject{
				Name:       projectName,
				ConfigFile: configFile,
			}
		}

		projects[projectName].ServiceCount++
		if c.State == "running" {
			projects[projectName].RunningCount++
		}
	}

	// Convert to slice and set status
	var result []ComposeProject
	for _, p := range projects {
		if p.RunningCount == p.ServiceCount {
			p.Status = "running"
		} else if p.RunningCount == 0 {
			p.Status = "stopped"
		} else {
			p.Status = "partial"
		}
		result = append(result, *p)
	}

	return result, nil
}

// ComposeUp starts a Docker Compose project
func (d *DockerClient) ComposeUp(projectName string) error {
	cmd := exec.Command("docker", "compose", "-p", projectName, "up", "-d")
	return cmd.Run()
}

// ComposeDown stops and removes a Docker Compose project
func (d *DockerClient) ComposeDown(projectName string) error {
	cmd := exec.Command("docker", "compose", "-p", projectName, "down")
	return cmd.Run()
}

// ComposeStop stops a Docker Compose project
func (d *DockerClient) ComposeStop(projectName string) error {
	cmd := exec.Command("docker", "compose", "-p", projectName, "stop")
	return cmd.Run()
}

// ComposeRestart restarts a Docker Compose project
func (d *DockerClient) ComposeRestart(projectName string) error {
	cmd := exec.Command("docker", "compose", "-p", projectName, "restart")
	return cmd.Run()
}

// ComposeLogs returns logs for a Docker Compose project
func (d *DockerClient) ComposeLogs(projectName string) (string, error) {
	cmd := exec.Command("docker", "compose", "-p", projectName, "logs", "--tail", "500")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// IsConnected checks if the Docker client is connected
func (d *DockerClient) IsConnected() bool {
	if d.client == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(d.ctx, 2*time.Second)
	defer cancel()

	_, err := d.client.Ping(ctx)
	return err == nil
}
