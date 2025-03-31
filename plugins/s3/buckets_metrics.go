package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"

	"omo/ui"
)

// showBucketMetricsConfirmation shows a confirmation modal before calculating metrics
func (bv *BucketsView) showBucketMetricsConfirmation() {
	// First check if a bucket is selected
	selectedRow := bv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(bv.buckets) {
		bv.cores.Log("[red]Error: No bucket selected - please select a bucket first")
		return
	}

	// Get the bucket name for the confirmation message
	bucketName := *bv.buckets[selectedRow].Name

	// Use the standard confirmation modal from ui package
	ui.ShowStandardConfirmationModal(
		bv.pages,
		bv.app,
		"Calculate Metrics",
		fmt.Sprintf("Calculate metrics for bucket '%s'?\n\nThis operation may take some time for large buckets.", bucketName),
		func(confirmed bool) {
			if confirmed {
				bv.calculateBucketMetrics()
			} else {
				bv.cores.Log("[blue]Metrics calculation cancelled")
			}
			// Return focus to the table
			bv.app.SetFocus(bv.cores.GetTable())
		},
	)
}

// calculateBucketMetrics calculates metrics for currently selected bucket
func (bv *BucketsView) calculateBucketMetrics() {
	selectedRow := bv.cores.GetSelectedRow()
	bv.cores.Log(fmt.Sprintf("[blue]Attempting to calculate metrics for row index: %d (total buckets: %d)", selectedRow, len(bv.buckets)))

	if selectedRow < 0 {
		bv.cores.Log("[red]Error: No bucket selected - please navigate to a bucket first")
		return
	}

	if selectedRow >= len(bv.buckets) {
		bv.cores.Log(fmt.Sprintf("[red]Error: Selected row (%d) is out of range (bucket count: %d)", selectedRow, len(bv.buckets)))
		return
	}

	bucket := bv.buckets[selectedRow]
	if bucket == nil || bucket.Name == nil {
		bv.cores.Log("[red]Error: Selected bucket data is invalid")
		return
	}

	bucketName := *bucket.Name

	// Get the correct region for the bucket
	bucketRegion, err := bv.getBucketRegion(bucketName)
	if err != nil {
		bv.cores.Log(fmt.Sprintf("[red]Error getting bucket region: %v", err))
		bucketRegion = bv.currentRegion // Fallback to current region
		bv.cores.Log(fmt.Sprintf("[yellow]Falling back to current region: %s", bucketRegion))
	}

	// Create a new S3 client in the bucket's region
	s3Client := bv.createS3ClientForRegion(bv.currentProfile, bucketRegion)
	if s3Client == nil {
		bv.cores.Log(fmt.Sprintf("[red]Error creating S3 client for region %s", bucketRegion))
		return
	}

	bv.cores.Log(fmt.Sprintf("[blue]Calculating metrics for bucket: %s in region: %s...", bucketName, bucketRegion))

	// Get bucket metrics
	var totalSize int64 = 0
	var objectCount int64 = 0

	// List objects in the bucket using the region-specific client
	err = s3Client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
	}, func(page *s3.ListObjectsV2Output, lastPage bool) bool {
		for _, obj := range page.Contents {
			totalSize += *obj.Size
			objectCount++
		}
		return true // continue pagination
	})

	if err != nil {
		bv.cores.Log(fmt.Sprintf("[red]Error calculating metrics: %v", err))
		return
	}

	// Format size in human-readable format
	sizeString := formatSize(totalSize)

	// Update info panel with metrics
	infoMap := map[string]string{
		"Bucket Size":  sizeString,
		"Object Count": fmt.Sprintf("%d", objectCount),
		"Buckets":      fmt.Sprintf("%d", len(bv.buckets)),
		"Region":       bucketRegion,
		"Profile":      bv.currentProfile,
	}
	bv.cores.SetInfoMap(infoMap)

	bv.cores.Log(fmt.Sprintf("[green]Bucket metrics: Size: %s, Objects: %d", sizeString, objectCount))
}

// formatSize converts bytes to human-readable format
func formatSize(bytes int64) string {
	const (
		_          = iota
		KB float64 = 1 << (10 * iota)
		MB
		GB
		TB
	)

	var size float64
	var unit string

	switch {
	case bytes >= int64(TB):
		size = float64(bytes) / TB
		unit = "TB"
	case bytes >= int64(GB):
		size = float64(bytes) / GB
		unit = "GB"
	case bytes >= int64(MB):
		size = float64(bytes) / MB
		unit = "MB"
	case bytes >= int64(KB):
		size = float64(bytes) / KB
		unit = "KB"
	default:
		size = float64(bytes)
		unit = "B"
	}

	return fmt.Sprintf("%.2f %s", size, unit)
}
