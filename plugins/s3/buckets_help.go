package main

import (
	"omo/pkg/ui"
)

// showHelp displays the help modal with bucket view hints
func (bv *BucketsView) showHelp() {
	// Create the help content with sections
	content := `[yellow]S3 Bucket Manager Help[white]

[green]Navigation[white]
  [aqua]↑/↓[white] - Navigate between buckets
  [aqua]Enter[white] - Select a bucket

[green]Actions[white]
  [aqua]R[white] - Refresh bucket list manually
  [aqua]S[white] - Sort buckets by column
  [aqua]O[white] - Browse objects in selected bucket
  [aqua]M[white] - Calculate metrics for selected bucket
  [aqua]Ctrl+T[white] - Select AWS profile
  [aqua]?[white] - Show this help screen
  [aqua]ESC[white] - Cancel/Clear selection

[green]AWS Profiles[white]
  AWS profiles are loaded from your ~/.aws/credentials file.
  Use Ctrl+T to switch between profiles if you have multiple configured.`

	// Show the info modal with a callback to return focus to the table
	ui.ShowInfoModal(
		bv.pages,
		bv.app,
		"S3 Plugin Help",
		content,
		func() {
			// Return focus to the table when modal is closed
			bv.app.SetFocus(bv.cores.GetTable())
		},
	)
}
