# Docker Plugin for OMO

This plugin provides Docker container, image, network, and volume management capabilities for the OMO (Oh My Ops) terminal interface.

## Features

- **Container Management**
  - View all Docker containers
  - Start, stop, and remove containers
  - View container logs
  - Execute commands inside containers

- **Image Management**
  - View all Docker images
  - See image details like size, tags, and creation date

- **Network Management**
  - View all Docker networks
  - See network details like driver, scope, and subnet

- **Volume Management**
  - View all Docker volumes
  - See volume details like driver and mountpoint

- **System Management**
  - Prune unused Docker resources (containers, images, networks, and volumes)

## Usage

### Navigation

| Key | Function |
|-----|----------|
| C | View Containers |
| I | View Images |
| N | View Networks |
| V | View Volumes |
| R | Refresh current view |
| ? | Show help |
| ESC | Close modal/Go back |

### Container Actions

| Key | Function |
|-----|----------|
| S | Start selected container |
| X | Stop selected container |
| D | Remove selected container |
| L | View logs of selected container |
| E | Execute command in selected container |

### System Actions

| Key | Function |
|-----|----------|
| P | Prune unused Docker resources |
| Ctrl+D | Refresh Docker data |

## Requirements

- Docker CLI must be installed and accessible in the system path
- User must have permissions to execute Docker commands

## Implementation

This plugin uses the Docker CLI command-line interface to interact with Docker. It parses the output of Docker commands to display information in a user-friendly interface.

The plugin consists of three main components:

1. **main.go**: Entry point for the plugin
2. **docker_view.go**: UI components and view handling
3. **docker_client.go**: Docker command execution and data parsing

## Troubleshooting

If you encounter issues:

1. Ensure the Docker daemon is running
2. Check that your user has permissions to run Docker commands
3. Try running the Docker commands manually to verify they work outside the plugin

## License

MIT 