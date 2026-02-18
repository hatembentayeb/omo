package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// configureAWSSession configures the AWS session
func (bv *BucketsView) configureAWSSession(profile, region string) error {
	// Create session
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: profile,
		Config: aws.Config{
			Region: aws.String(region),
		},
		SharedConfigState: session.SharedConfigEnable,
	})
	if err != nil {
		bv.cores.Log(fmt.Sprintf("[red]Error creating AWS session: %v", err))
		return err
	}

	// Create S3 client
	bv.s3Client = s3.New(sess)
	bv.currentProfile = profile
	bv.currentRegion = region
	bv.cores.Log(fmt.Sprintf("[green]Connected to AWS with profile: %s, region: %s", profile, region))

	return nil
}

// createS3ClientForRegion creates a new S3 client for a specific region
func (bv *BucketsView) createS3ClientForRegion(profile, region string) *s3.S3 {
	// Create session options
	options := session.Options{
		Config: aws.Config{
			Region: aws.String(region),
		},
	}

	// Add profile if specified
	if profile != "" {
		options.Profile = profile
		options.SharedConfigState = session.SharedConfigEnable
	}

	// Create session
	sess, err := session.NewSessionWithOptions(options)
	if err != nil {
		bv.cores.Log(fmt.Sprintf("[red]Error creating AWS session for region %s: %v", region, err))
		return nil
	}

	// Create S3 client
	client := s3.New(sess)
	return client
}

// getBucketRegion gets the region for a bucket
func (bv *BucketsView) getBucketRegion(bucketName string) (string, error) {
	// NOTE: Create a new client configured with us-east-1 (required for the GetBucketLocation call)
	usEastClient := bv.createS3ClientForRegion(bv.currentProfile, "us-east-1")
	if usEastClient == nil {
		return bv.currentRegion, fmt.Errorf("failed to create us-east-1 client for region lookup")
	}

	// Get bucket location using the us-east-1 client (AWS API requirement)
	result, err := usEastClient.GetBucketLocation(&s3.GetBucketLocationInput{
		Bucket: aws.String(bucketName),
	})

	if err != nil {
		return "unknown", err
	}

	// If location constraint is nil or empty, it's us-east-1
	if result.LocationConstraint == nil || *result.LocationConstraint == "" {
		return "us-east-1", nil
	}

	return *result.LocationConstraint, nil
}
