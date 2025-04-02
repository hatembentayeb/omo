# Git Plugin for OMO

A Git repository management plugin for OMO (Oh My Ops) that leverages the native Go Git implementation without requiring the external Git command.

## Features

- **Manual Repository Discovery**: Specify directories to scan for Git repositories
- **Repository Browser**: View and manage multiple repositories from a central interface
- **Status Monitoring**: See the status of each repository at a glance
- **Git Operations**: Perform common Git operations like fetch and pull
- **Branch Management**: View local and remote branches
- **Commit History**: Browse commit logs

## Key Bindings

| Key | Function |
|-----|----------|
| D   | Add repositories from a directory |
| R   | Refresh repositories data |
| G   | Select repository from list |
| F   | Fetch updates from remote |
| P   | Pull changes from remote |
| S   | Show detailed status |
| L   | Show commit log |
| B   | Show branches |
| ?   | Show help |
| ESC | Navigate back or close modals |

## Usage

### Adding Repositories

1. Press `D` to open the directory selector
2. Enter the path to a directory containing Git repositories
3. Click "Search" or press Enter to begin scanning
4. Git repositories will be added to the list

You can add repositories from multiple directories by repeating these steps.

### Working with Repositories

1. Select a repository from the list by clicking on it
2. Use the key bindings to perform operations on the selected repository:
   - `F` to fetch updates
   - `P` to pull changes
   - `S` to view detailed status
   - `L` to view commit history
   - `B` to view branches

## Implementation Details

This plugin follows the OMO UI framework patterns:

- Uses the `ui.Cores` component for consistent UI
- Leverages the `ui.ViewFactory` to create standardized views
- Uses the `ui.ErrorHandler` for unified error handling
- Implements standard navigation patterns
- Follows key binding conventions

## Dependencies

- `github.com/go-git/go-git/v5`: Native Go implementation of Git

## Architecture

The plugin is structured with clean separation of concerns:

- `main.go`: Plugin entry point and metadata
- `git_view.go`: UI implementation 
- `git_client.go`: Git operations implementation using native Go Git 