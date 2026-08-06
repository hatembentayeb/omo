package awscosts

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/budgets"
	"github.com/aws/aws-sdk-go/service/costexplorer"
	"github.com/aws/aws-sdk-go/service/sts"
)

// getAWSAccountID retrieves the AWS account ID using STS
func getAWSAccountID(sess *session.Session) (string, error) {
	svc := sts.New(sess)
	result, err := svc.GetCallerIdentity(&sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get caller identity: %v", err)
	}
	return aws.StringValue(result.Account), nil
}

// AWSCostClient handles all AWS Cost Explorer API interactions
type AWSCostClient struct {
	profile      string
	region       string
	costExplorer *costexplorer.CostExplorer
	budgetsAPI   *budgets.Budgets
	sess         *session.Session
}

// NewAWSCostClient creates a new AWS cost client with the specified profile
func NewAWSCostClient(profile, region string) (*AWSCostClient, error) {
	return NewAWSCostClientWithCreds(profile, region, "", "")
}

// NewAWSCostClientWithCreds creates a cost client using a profile and/or static keys.
func NewAWSCostClientWithCreds(profile, region, accessKey, secretKey string) (*AWSCostClient, error) {
	if region == "" {
		region = "us-east-1"
	}
	opts := session.Options{
		Config: aws.Config{
			Region: aws.String(region),
		},
		SharedConfigState: session.SharedConfigEnable,
	}
	if profile != "" {
		opts.Profile = profile
	}
	if accessKey != "" && secretKey != "" {
		opts.Config.Credentials = credentials.NewStaticCredentials(accessKey, secretKey, "")
	}

	sess, err := session.NewSessionWithOptions(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %v", err)
	}

	return &AWSCostClient{
		profile:      profile,
		region:       region,
		costExplorer: costexplorer.New(sess),
		budgetsAPI:   budgets.New(sess),
		sess:         sess,
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
	accountID := "self"
	if c.sess != nil {
		if id, err := getAWSAccountID(c.sess); err == nil {
			accountID = id
		}
	}
	input := &budgets.DescribeBudgetsInput{
		AccountId:  aws.String(accountID),
		MaxResults: aws.Int64(100),
	}

	return c.budgetsAPI.DescribeBudgets(input)
}

// Session returns the underlying AWS session (for forecast helpers).
func (c *AWSCostClient) Session() *session.Session {
	return c.sess
}

// CostExplorer returns the Cost Explorer API client.
func (c *AWSCostClient) CostExplorer() *costexplorer.CostExplorer {
	return c.costExplorer
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
