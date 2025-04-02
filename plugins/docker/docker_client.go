package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
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
