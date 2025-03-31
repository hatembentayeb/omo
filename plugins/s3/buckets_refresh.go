package main

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/service/s3"
)

// refreshBuckets refreshes the list of buckets
func (bv *BucketsView) refreshBuckets() {
	if bv.s3Client == nil {
		bv.cores.Log("[red]Error: S3 client not initialized")
		return
	}

	bv.cores.Log("[blue]Refreshing buckets...")

	// List buckets
	result, err := bv.s3Client.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		bv.cores.Log(fmt.Sprintf("[red]Error listing buckets: %v", err))
		return
	}

	// Store buckets for later use
	bv.buckets = result.Buckets

	// Convert to table data - IMPORTANT: order must match buckets array exactly
	tableData := make([][]string, 0, len(result.Buckets))
	for i, bucket := range result.Buckets {
		region, err := bv.getBucketRegion(*bucket.Name)
		if err != nil {
			region = "unknown"
			bv.cores.Log(fmt.Sprintf("[yellow]Could not determine region for bucket %s: %v", *bucket.Name, err))
		}

		tableData = append(tableData, []string{
			*bucket.Name,
			bucket.CreationDate.Format(time.RFC3339),
			region,
		})

		// Store bucket data in correct index
		bv.buckets[i] = bucket
	}

	// Update the table
	bv.cores.SetTableData(tableData)
	bv.cores.Log(fmt.Sprintf("[green]Found %d buckets", len(result.Buckets)))

	// Update info panel using map
	infoMap := map[string]string{
		"Profile": bv.currentProfile,
		"Region":  bv.currentRegion,
		"Buckets": fmt.Sprintf("%d", len(result.Buckets)),
	}
	bv.cores.SetInfoMap(infoMap)

	// Focus the table
	bv.app.SetFocus(bv.cores.GetTable())
}
