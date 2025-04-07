package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"omo/ui"

	"github.com/rivo/tview"
)

// AWSProfileInfo stores AWS profile information including region
type AWSProfileInfo struct {
	Name   string
	Region string
}

// ProfileSelector provides functionality to select AWS profiles
type ProfileSelector struct {
	app      *tview.Application
	pages    *tview.Pages
	callback func(profile, region string)
	logger   func(string) // Optional logger function
}

// NewProfileSelector creates a new AWS profile selector
func NewProfileSelector(app *tview.Application, pages *tview.Pages, callback func(profile, region string)) *ProfileSelector {
	return &ProfileSelector{
		app:      app,
		pages:    pages,
		callback: callback,
	}
}

// SetLogger sets a logger function
func (p *ProfileSelector) SetLogger(logger func(string)) {
	p.logger = logger
}

// log logs a message if a logger is set
func (p *ProfileSelector) log(msg string) {
	if p.logger != nil {
		p.logger(msg)
	}
}

// Show displays the profile selector modal
func (p *ProfileSelector) Show() {
	profileInfos := p.getAWSProfiles()

	// Format profiles for the modal, showing regions too
	formattedProfiles := make([][]string, 0, len(profileInfos))
	for _, profileInfo := range profileInfos {
		displayName := fmt.Sprintf("%s - Region: %s", profileInfo.Name, profileInfo.Region)
		formattedProfiles = append(formattedProfiles, []string{displayName, ""})
	}

	ui.ShowStandardListSelectorModal(
		p.pages,
		p.app,
		"Select AWS Profile",
		formattedProfiles,
		func(index int, text string, cancelled bool) {
			if cancelled || index < 0 {
				// Return focus to the table when cancelled
				p.log("[blue]Profile selection cancelled")
				return
			}

			if p.callback != nil && index < len(profileInfos) {
				// Pass both profile name and region
				p.callback(profileInfos[index].Name, profileInfos[index].Region)
			}
		},
	)
}

// getAWSProfiles retrieves available AWS profiles from config files
func (p *ProfileSelector) getAWSProfiles() []AWSProfileInfo {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		p.log(fmt.Sprintf("[red]Error getting user home directory: %v", err))
		return []AWSProfileInfo{{Name: "default", Region: "us-east-1"}}
	}

	// Path to AWS credentials and config files
	credsPath := filepath.Join(homeDir, ".aws", "credentials")
	configPath := filepath.Join(homeDir, ".aws", "config")

	// Read credentials file
	credsData, err := os.ReadFile(credsPath)
	if err != nil {
		p.log(fmt.Sprintf("[yellow]Could not read AWS credentials file: %v", err))
		return []AWSProfileInfo{{Name: "default", Region: "us-east-1"}}
	}

	// Get regions from config file
	regionMap := make(map[string]string)
	configData, err := os.ReadFile(configPath)
	if err == nil {
		// Parse config file for regions
		configLines := strings.Split(string(configData), "\n")
		var currentProfile string

		for _, line := range configLines {
			line = strings.TrimSpace(line)

			// Look for profile sections: [profile name] or [default]
			if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
				profileLine := line[1 : len(line)-1]

				// Handle 'profile name' format
				if strings.HasPrefix(profileLine, "profile ") {
					currentProfile = strings.TrimPrefix(profileLine, "profile ")
					currentProfile = strings.TrimSpace(currentProfile)
				} else {
					// Direct profile name (like [default])
					currentProfile = profileLine
				}
			} else if strings.HasPrefix(line, "region") && currentProfile != "" {
				// Extract region value
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					region := strings.TrimSpace(parts[1])
					regionMap[currentProfile] = region
					p.log(fmt.Sprintf("[blue]Found region %s for profile %s", region, currentProfile))
				}
			}
		}
	} else {
		p.log(fmt.Sprintf("[yellow]Could not read AWS config file: %v", err))
	}

	// Parse profiles from credentials file
	// Profiles are in format [profile-name]
	profileInfos := []AWSProfileInfo{}
	lines := strings.Split(string(credsData), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			profile := line[1 : len(line)-1]

			// Get region for this profile, default to us-east-1 if not found
			region, exists := regionMap[profile]
			if !exists {
				region = "us-east-1"
				p.log(fmt.Sprintf("[yellow]No region found for profile %s, defaulting to us-east-1", profile))
			}

			profileInfos = append(profileInfos, AWSProfileInfo{
				Name:   profile,
				Region: region,
			})
		}
	}

	// If no profiles found, return default
	if len(profileInfos) == 0 {
		p.log("[yellow]No profiles found in AWS credentials file")
		return []AWSProfileInfo{{Name: "default", Region: "us-east-1"}}
	}

	p.log(fmt.Sprintf("[green]Found %d AWS profiles", len(profileInfos)))
	return profileInfos
}
