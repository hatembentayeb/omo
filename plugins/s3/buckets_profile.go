package main

import (
	"fmt"

	"omo/pkg/ui"
)

// ShowProfileSelector shows the profile selector modal
func (bv *BucketsView) ShowProfileSelector() {
	// Get list of profiles
	profiles, err := bv.loadAWSProfiles()
	if err != nil {
		bv.cores.Log(fmt.Sprintf("[red]Error loading AWS profiles: %v", err))
		return
	}

	// Format profiles for modal
	items := make([][]string, 0, len(profiles))
	for _, profile := range profiles {
		items = append(items, []string{profile, ""})
	}

	// Show modal - the callback will be called for both selection and cancellation
	ui.ShowStandardListSelectorModal(
		bv.pages,
		bv.app,
		"Select AWS Profile",
		items,
		func(index int, name string, cancelled bool) {
			// Always return focus to the table, whether cancelled or selected

			if !cancelled && index >= 0 {
				// Configure session with selected profile
				bv.configureAWSSession(name, bv.currentRegion)
				// Refresh buckets
				bv.refreshBuckets()
			} else {
				// Log that selection was cancelled
				bv.cores.Log("[blue]Profile selection cancelled")
				bv.app.SetFocus(bv.cores.GetTable())
			}

			// Return focus to the table
			bv.app.SetFocus(bv.cores.GetTable())
		},
	)
}

// loadAWSProfiles loads AWS profiles from credentials file
func (bv *BucketsView) loadAWSProfiles() ([]string, error) {
	return loadAWSProfilesFromCredentials()
}
