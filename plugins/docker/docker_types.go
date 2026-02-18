package main

import (
	"strings"
	"time"
)

// DockerContainer represents a Docker container
type DockerContainer struct {
	ID            string
	Name          string
	Image         string
	Status        string
	Created       time.Time
	Ports         []string
	State         string
	NetworkMode   string
	RestartPolicy string
	Labels        map[string]string
}

// GetTableRow returns container data as a table row
func (c *DockerContainer) GetTableRow() []string {
	id := c.ID
	if len(id) > 12 {
		id = id[:12]
	}
	return []string{
		id,                          // Short ID
		c.Name,                      // Container name
		c.Image,                     // Image name
		c.State,                     // Running state
		c.Status,                    // Status with runtime
		strings.Join(c.Ports, ", "), // Ports
	}
}

// DockerImage represents a Docker image
type DockerImage struct {
	ID           string
	Repository   string
	Tag          string
	Size         string
	Created      time.Time
	CreatedSince string
	Digest       string
}

// GetTableRow returns image data as a table row
func (i *DockerImage) GetTableRow() []string {
	id := i.ID
	if len(id) > 12 {
		id = id[:12]
	}
	return []string{
		id,             // Short ID
		i.Repository,   // Repository name
		i.Tag,          // Tag
		i.Size,         // Size (formatted)
		i.CreatedSince, // Relative creation time
	}
}

// DockerNetwork represents a Docker network
type DockerNetwork struct {
	ID      string
	Name    string
	Driver  string
	Scope   string
	Created time.Time
	Subnet  string
	Gateway string
}

// GetTableRow returns network data as a table row
func (n *DockerNetwork) GetTableRow() []string {
	id := n.ID
	if len(id) > 12 {
		id = id[:12]
	}
	return []string{
		id,       // Short ID
		n.Name,   // Network name
		n.Driver, // Network driver
		n.Scope,  // Network scope
		n.Subnet, // Subnet
	}
}

// DockerVolume represents a Docker volume
type DockerVolume struct {
	Name       string
	Driver     string
	Mountpoint string
	Scope      string
	CreatedAt  string
	Created    time.Time
	Labels     map[string]string
	Size       string
}

// GetTableRow returns volume data as a table row
func (v *DockerVolume) GetTableRow() []string {
	return []string{
		v.Name,       // Volume name
		v.Driver,     // Volume driver
		v.Mountpoint, // Mount point
		v.Size,       // Size (if available)
	}
}
