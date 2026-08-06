package s3

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// S3Client wraps AWS S3 API access without UI dependencies.
type S3Client struct {
	client  *s3.S3
	profile string
	region  string
	access  string
	secret  string
	endpoint string
}

// NewS3Client creates an empty S3 client; call Connect before use.
func NewS3Client() *S3Client {
	return &S3Client{}
}

// Connect establishes an S3 session using profile and/or static credentials.
func (c *S3Client) Connect(profile, region, accessKey, secretKey, endpoint string) error {
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
	if endpoint != "" {
		opts.Config.Endpoint = aws.String(endpoint)
		opts.Config.S3ForcePathStyle = aws.Bool(true)
	}
	sess, err := session.NewSessionWithOptions(opts)
	if err != nil {
		return err
	}
	c.client = s3.New(sess)
	c.profile = profile
	c.region = region
	c.access = accessKey
	c.secret = secretKey
	c.endpoint = endpoint
	return nil
}

// IsConnected reports whether a session exists.
func (c *S3Client) IsConnected() bool {
	return c != nil && c.client != nil
}

// Disconnect clears the current session.
func (c *S3Client) Disconnect() {
	c.client = nil
}

// createClientForRegion builds an S3 client for a specific region.
func (c *S3Client) createClientForRegion(region string) *s3.S3 {
	return CreateS3ClientForRegion(c.profile, region, c.access, c.secret, c.endpoint)
}

// CreateS3ClientForRegion creates a new S3 client for a specific region.
func CreateS3ClientForRegion(profile, region, accessKey, secretKey, endpoint string) *s3.S3 {
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
	if endpoint != "" {
		opts.Config.Endpoint = aws.String(endpoint)
		opts.Config.S3ForcePathStyle = aws.Bool(true)
	}
	sess, err := session.NewSessionWithOptions(opts)
	if err != nil {
		return nil
	}
	return s3.New(sess)
}

// GetBucketRegion looks up the region for a bucket (requires us-east-1 client).
func GetBucketRegion(profile, fallbackRegion, accessKey, secretKey, endpoint, bucketName string) (string, error) {
	usEastClient := CreateS3ClientForRegion(profile, "us-east-1", accessKey, secretKey, endpoint)
	if usEastClient == nil {
		return fallbackRegion, fmt.Errorf("failed to create us-east-1 client for region lookup")
	}
	result, err := usEastClient.GetBucketLocation(&s3.GetBucketLocationInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return "unknown", err
	}
	if result.LocationConstraint == nil || *result.LocationConstraint == "" {
		return "us-east-1", nil
	}
	return *result.LocationConstraint, nil
}

// BucketRow is a serializable bucket listing row.
type BucketRow struct {
	Name         string
	CreationDate string
	Region       string
}

// ListBuckets returns bucket rows with regions.
func (c *S3Client) ListBuckets() ([]BucketRow, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("s3 client not connected")
	}
	result, err := c.client.ListBuckets(&s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}
	rows := make([]BucketRow, 0, len(result.Buckets))
	for _, bucket := range result.Buckets {
		name := aws.StringValue(bucket.Name)
		region, err := GetBucketRegion(c.profile, c.region, c.access, c.secret, c.endpoint, name)
		if err != nil {
			region = "unknown"
		}
		created := ""
		if bucket.CreationDate != nil {
			created = bucket.CreationDate.Format(time.RFC3339)
		}
		rows = append(rows, BucketRow{Name: name, CreationDate: created, Region: region})
	}
	return rows, nil
}

// ObjectRow is a serializable object listing row.
type ObjectRow struct {
	Name         string
	Size         string
	LastModified string
	StorageClass string
	IsDir        bool
}

// ListObjects lists objects under bucket/prefix (delimiter=/).
func (c *S3Client) ListObjects(bucket, prefix string) ([]ObjectRow, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("s3 client not connected")
	}
	region, err := GetBucketRegion(c.profile, c.region, c.access, c.secret, c.endpoint, bucket)
	if err != nil {
		region = c.region
	}
	client := c.createClientForRegion(region)
	if client == nil {
		return nil, fmt.Errorf("error creating S3 client for region %s", region)
	}

	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int64(1000),
	}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}

	result, err := client.ListObjectsV2(input)
	if err != nil {
		return nil, err
	}

	var rows []ObjectRow
	if prefix != "" {
		rows = append(rows, ObjectRow{Name: "../", StorageClass: ""})
	}
	for _, p := range result.CommonPrefixes {
		if p.Prefix == nil {
			continue
		}
		name := *p.Prefix
		if prefix != "" {
			name = strings.TrimPrefix(name, prefix)
		}
		rows = append(rows, ObjectRow{Name: name, Size: "-", LastModified: "-", StorageClass: "Directory", IsDir: true})
	}
	for _, obj := range result.Contents {
		if obj.Key == nil {
			continue
		}
		row := formatS3ObjectRow(obj, prefix)
		if row != nil {
			rows = append(rows, *row)
		}
	}
	return rows, nil
}

func formatS3ObjectRow(obj *s3.Object, prefix string) *ObjectRow {
	key := *obj.Key
	if key == prefix {
		return nil
	}
	name := key
	if prefix != "" {
		name = strings.TrimPrefix(key, prefix)
	}
	size := ""
	if obj.Size != nil {
		size = formatSize(*obj.Size)
	}
	lastModified := ""
	if obj.LastModified != nil {
		lastModified = obj.LastModified.Format(time.RFC3339)
	}
	storageClass := "STANDARD"
	if obj.StorageClass != nil {
		storageClass = *obj.StorageClass
	}
	return &ObjectRow{Name: name, Size: size, LastModified: lastModified, StorageClass: storageClass}
}
