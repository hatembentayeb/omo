package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"omo/pkg/ui"
)

// S3ProfileInfo stores AWS profile information including region
type S3ProfileInfo struct {
	Name   string
	Region string
}

// ShowProfileSelector shows the profile selector modal with profiles and regions
func (bv *BucketsView) ShowProfileSelector() {
	profileInfos := bv.getAWSProfiles()

	// Format profiles for the modal, showing regions too
	items := make([][]string, 0, len(profileInfos))
	for _, info := range profileInfos {
		displayName := fmt.Sprintf("%s - Region: %s", info.Name, info.Region)
		items = append(items, []string{displayName, ""})
	}

	ui.ShowStandardListSelectorModal(
		bv.pages,
		bv.app,
		"Select AWS Profile",
		items,
		func(index int, text string, cancelled bool) {
			if cancelled || index < 0 {
				bv.cores.Log("[blue]Profile selection cancelled")
				bv.app.SetFocus(bv.currentCores().GetTable())
				return
			}

			if index < len(profileInfos) {
				selected := profileInfos[index]
				bv.cores.Log(fmt.Sprintf("[blue]Switching to profile: %s (region: %s)", selected.Name, selected.Region))
				bv.configureAWSSession(selected.Name, selected.Region)
				bv.refreshBuckets()
			}

			bv.app.SetFocus(bv.currentCores().GetTable())
		},
	)
}

func parseAWSConfigRegions(configPath string) map[string]string {
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

func parseAWSCredentialProfiles(credsPath string, regionMap map[string]string, existing map[string]bool) []S3ProfileInfo {
	credsData, err := os.ReadFile(credsPath)
	if err != nil {
		return nil
	}
	var profiles []S3ProfileInfo
	for _, line := range strings.Split(string(credsData), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			profile := line[1 : len(line)-1]
			if existing[profile] {
				continue
			}
			region := regionMap[profile]
			if region == "" {
				region = "us-east-1"
			}
			profiles = append(profiles, S3ProfileInfo{Name: profile, Region: region})
		}
	}
	return profiles
}

// getAWSProfiles retrieves available AWS profiles from credentials and config files.
// It merges profiles from ~/.aws/credentials with region info from ~/.aws/config,
// and also includes any profiles defined in the S3 plugin YAML config.
func (bv *BucketsView) getAWSProfiles() []S3ProfileInfo {
	var profileInfos []S3ProfileInfo

	s3Profiles, err := GetAvailableS3Profiles()
	if err == nil && len(s3Profiles) > 0 {
		for _, p := range s3Profiles {
			region := p.Region
			if region == "" {
				region = "us-east-1"
			}
			profileInfos = append(profileInfos, S3ProfileInfo{Name: p.Name, Region: region})
		}
		bv.cores.Log(fmt.Sprintf("[green]Found %d profiles from S3 config", len(s3Profiles)))
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		bv.cores.Log(fmt.Sprintf("[red]Error getting home directory: %v", err))
		if len(profileInfos) == 0 {
			return []S3ProfileInfo{{Name: "default", Region: "us-east-1"}}
		}
		return profileInfos
	}

	credsPath := filepath.Join(homeDir, ".aws", "credentials")
	configPath := filepath.Join(homeDir, ".aws", "config")

	regionMap := parseAWSConfigRegions(configPath)

	existing := make(map[string]bool)
	for _, p := range profileInfos {
		existing[p.Name] = true
	}

	credProfiles := parseAWSCredentialProfiles(credsPath, regionMap, existing)
	if credProfiles == nil && len(profileInfos) == 0 {
		bv.cores.Log("[yellow]Could not read AWS credentials file")
		return []S3ProfileInfo{{Name: "default", Region: "us-east-1"}}
	}
	profileInfos = append(profileInfos, credProfiles...)

	if len(profileInfos) == 0 {
		bv.cores.Log("[yellow]No profiles found, using default")
		return []S3ProfileInfo{{Name: "default", Region: "us-east-1"}}
	}

	bv.cores.Log(fmt.Sprintf("[green]Found %d AWS profiles total", len(profileInfos)))
	return profileInfos
}
