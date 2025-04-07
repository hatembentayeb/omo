package main

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/budgets"
	"github.com/aws/aws-sdk-go/service/costexplorer"
)

// AWSCostClient handles all AWS Cost Explorer API interactions
type AWSCostClient struct {
	profile      string
	region       string
	costExplorer *costexplorer.CostExplorer
	budgetsAPI   *budgets.Budgets
}

// NewAWSCostClient creates a new AWS cost client with the specified profile
func NewAWSCostClient(profile, region string) (*AWSCostClient, error) {
	// Create a session with the profile
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: profile,
		Config: aws.Config{
			Region: aws.String(region),
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %v", err)
	}

	// Create the client
	return &AWSCostClient{
		profile:      profile,
		region:       region,
		costExplorer: costexplorer.New(sess),
		budgetsAPI:   budgets.New(sess),
	}, nil
}

// GetCostsByService retrieves costs grouped by service for the given time period
func (c *AWSCostClient) GetCostsByService(startDate, endDate time.Time, granularity string) (*costexplorer.GetCostAndUsageOutput, error) {
	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &costexplorer.DateInterval{
			Start: aws.String(startDate.Format("2006-01-02")),
			End:   aws.String(endDate.Format("2006-01-02")),
		},
		Granularity: aws.String(granularity),
		Metrics:     []*string{aws.String("BlendedCost"), aws.String("UnblendedCost")},
		GroupBy: []*costexplorer.GroupDefinition{
			{
				Type: aws.String("DIMENSION"),
				Key:  aws.String("SERVICE"),
			},
		},
	}

	return c.costExplorer.GetCostAndUsage(input)
}

// GetCostsByAccount retrieves costs grouped by account
func (c *AWSCostClient) GetCostsByAccount(startDate, endDate time.Time, granularity string) (*costexplorer.GetCostAndUsageOutput, error) {
	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &costexplorer.DateInterval{
			Start: aws.String(startDate.Format("2006-01-02")),
			End:   aws.String(endDate.Format("2006-01-02")),
		},
		Granularity: aws.String(granularity),
		Metrics:     []*string{aws.String("BlendedCost")},
		GroupBy: []*costexplorer.GroupDefinition{
			{
				Type: aws.String("DIMENSION"),
				Key:  aws.String("LINKED_ACCOUNT"),
			},
		},
	}

	return c.costExplorer.GetCostAndUsage(input)
}

// GetCostsByRegion retrieves costs grouped by region
func (c *AWSCostClient) GetCostsByRegion(startDate, endDate time.Time, granularity string) (*costexplorer.GetCostAndUsageOutput, error) {
	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &costexplorer.DateInterval{
			Start: aws.String(startDate.Format("2006-01-02")),
			End:   aws.String(endDate.Format("2006-01-02")),
		},
		Granularity: aws.String(granularity),
		Metrics:     []*string{aws.String("BlendedCost")},
		GroupBy: []*costexplorer.GroupDefinition{
			{
				Type: aws.String("DIMENSION"),
				Key:  aws.String("REGION"),
			},
		},
	}

	return c.costExplorer.GetCostAndUsage(input)
}

// GetCostForecast retrieves cost forecast for the given time period
func (c *AWSCostClient) GetCostForecast(startDate, endDate time.Time, granularity string) (*costexplorer.GetCostForecastOutput, error) {
	input := &costexplorer.GetCostForecastInput{
		TimePeriod: &costexplorer.DateInterval{
			Start: aws.String(startDate.Format("2006-01-02")),
			End:   aws.String(endDate.Format("2006-01-02")),
		},
		Granularity: aws.String(granularity),
		Metric:      aws.String("BLENDED_COST"),
	}

	return c.costExplorer.GetCostForecast(input)
}

// GetBudgets retrieves all budgets for the account
func (c *AWSCostClient) GetBudgets() (*budgets.DescribeBudgetsOutput, error) {
	input := &budgets.DescribeBudgetsInput{
		AccountId:  aws.String("self"),
		MaxResults: aws.Int64(100),
	}

	return c.budgetsAPI.DescribeBudgets(input)
}

// GetServiceUsage retrieves detailed usage data for a specific service
func (c *AWSCostClient) GetServiceUsage(startDate, endDate time.Time, service string) (*costexplorer.GetCostAndUsageOutput, error) {
	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &costexplorer.DateInterval{
			Start: aws.String(startDate.Format("2006-01-02")),
			End:   aws.String(endDate.Format("2006-01-02")),
		},
		Granularity: aws.String("DAILY"),
		Metrics:     []*string{aws.String("BlendedCost"), aws.String("UsageQuantity")},
		Filter: &costexplorer.Expression{
			Dimensions: &costexplorer.DimensionValues{
				Key:    aws.String("SERVICE"),
				Values: []*string{aws.String(service)},
			},
		},
		GroupBy: []*costexplorer.GroupDefinition{
			{
				Type: aws.String("DIMENSION"),
				Key:  aws.String("USAGE_TYPE"),
			},
		},
	}

	return c.costExplorer.GetCostAndUsage(input)
}
