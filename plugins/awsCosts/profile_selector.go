package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"omo/pkg/ui"

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

func parseAWSConfigRegionMap(configPath string) map[string]string {
	regionMap := make(map[string]string)
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return regionMap
	}
	var currentProfile string
	for _, line := range strings.Split(string(configData), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			profileLine := line[1 : len(line)-1]
			if strings.HasPrefix(profileLine, "profile ") {
				currentProfile = strings.TrimSpace(strings.TrimPrefix(profileLine, "profile "))
			} else {
				currentProfile = profileLine
			}
		} else if strings.HasPrefix(line, "region") && currentProfile != "" {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				regionMap[currentProfile] = strings.TrimSpace(parts[1])
			}
		}
	}
	return regionMap
}

func parseCredentialFileProfiles(credsPath string, regionMap map[string]string) []AWSProfileInfo {
	credsData, err := os.ReadFile(credsPath)
	if err != nil {
		return nil
	}
	var profiles []AWSProfileInfo
	for _, line := range strings.Split(string(credsData), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			profile := line[1 : len(line)-1]
			region := regionMap[profile]
			if region == "" {
				region = "us-east-1"
			}
			profiles = append(profiles, AWSProfileInfo{Name: profile, Region: region})
		}
	}
	return profiles
}

// getAWSProfiles retrieves available AWS profiles from config files
func (p *ProfileSelector) getAWSProfiles() []AWSProfileInfo {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		p.log(fmt.Sprintf("[red]Error getting user home directory: %v", err))
		return []AWSProfileInfo{{Name: "default", Region: "us-east-1"}}
	}

	credsPath := filepath.Join(homeDir, ".aws", "credentials")
	configPath := filepath.Join(homeDir, ".aws", "config")

	regionMap := parseAWSConfigRegionMap(configPath)
	profileInfos := parseCredentialFileProfiles(credsPath, regionMap)

	if len(profileInfos) == 0 {
		p.log("[yellow]No profiles found in AWS credentials file")
		return []AWSProfileInfo{{Name: "default", Region: "us-east-1"}}
	}

	p.log(fmt.Sprintf("[green]Found %d AWS profiles", len(profileInfos)))
	return profileInfos
}
