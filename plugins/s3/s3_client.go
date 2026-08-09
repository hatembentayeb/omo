package s3

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// S3Client wraps AWS S3 API access without UI dependencies.
type S3Client struct {
	client      *s3.S3
	profile     string
	region      string
	access      string
	secret      string
	endpoint    string
	regionCache map[string]string
}

// NewS3Client creates an empty S3 client; call Connect before use.
func NewS3Client() *S3Client {
	return &S3Client{regionCache: map[string]string{}}
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
	c.regionCache = map[string]string{}
	return nil
}

func (c *S3Client) IsConnected() bool {
	return c != nil && c.client != nil
}

func (c *S3Client) Disconnect() {
	c.client = nil
	c.regionCache = map[string]string{}
}

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

func (c *S3Client) bucketClient(bucket string) (*s3.S3, string, error) {
	if !c.IsConnected() {
		return nil, "", fmt.Errorf("s3 client not connected")
	}
	region, err := c.BucketRegion(bucket)
	if err != nil {
		region = c.region
	}
	client := c.createClientForRegion(region)
	if client == nil {
		return nil, "", fmt.Errorf("error creating S3 client for region %s", region)
	}
	return client, region, nil
}

// BucketRegion looks up (and caches) the region for a bucket.
func (c *S3Client) BucketRegion(bucketName string) (string, error) {
	if r, ok := c.regionCache[bucketName]; ok {
		return r, nil
	}
	usEastClient := CreateS3ClientForRegion(c.profile, "us-east-1", c.access, c.secret, c.endpoint)
	if usEastClient == nil {
		return c.region, fmt.Errorf("failed to create us-east-1 client for region lookup")
	}
	result, err := usEastClient.GetBucketLocation(&s3.GetBucketLocationInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		return "unknown", err
	}
	region := "us-east-1"
	if result.LocationConstraint != nil && *result.LocationConstraint != "" {
		region = *result.LocationConstraint
	}
	c.regionCache[bucketName] = region
	return region, nil
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
		region, err := c.BucketRegion(name)
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
	client, _, err := c.bucketClient(bucket)
	if err != nil {
		return nil, err
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

// CreateBucket creates a bucket in the configured region.
func (c *S3Client) CreateBucket(name, region string) error {
	if !c.IsConnected() {
		return fmt.Errorf("s3 client not connected")
	}
	if region == "" {
		region = c.region
	}
	client := c.createClientForRegion(region)
	if client == nil {
		return fmt.Errorf("failed to create client for region %s", region)
	}
	input := &s3.CreateBucketInput{Bucket: aws.String(name)}
	if region != "" && region != "us-east-1" {
		input.CreateBucketConfiguration = &s3.CreateBucketConfiguration{
			LocationConstraint: aws.String(region),
		}
	}
	_, err := client.CreateBucket(input)
	if err == nil {
		c.regionCache[name] = region
	}
	return err
}

// DeleteBucket deletes an empty bucket.
func (c *S3Client) DeleteBucket(name string) error {
	client, _, err := c.bucketClient(name)
	if err != nil {
		return err
	}
	_, err = client.DeleteBucket(&s3.DeleteBucketInput{Bucket: aws.String(name)})
	delete(c.regionCache, name)
	return err
}

// DeleteObject deletes a single object key.
func (c *S3Client) DeleteObject(bucket, key string) error {
	client, _, err := c.bucketClient(bucket)
	if err != nil {
		return err
	}
	_, err = client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

// PutEmptyObject creates a zero-byte object (used for "folders").
func (c *S3Client) PutEmptyObject(bucket, key string) error {
	client, _, err := c.bucketClient(bucket)
	if err != nil {
		return err
	}
	_, err = client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader(""),
	})
	return err
}

// ObjectInfo holds HeadObject details.
type ObjectInfo struct {
	Key          string
	Size         string
	ContentType  string
	ETag         string
	LastModified string
	StorageClass string
	Metadata     map[string]string
}

// HeadObject returns metadata for an object.
func (c *S3Client) HeadObject(bucket, key string) (*ObjectInfo, error) {
	client, _, err := c.bucketClient(bucket)
	if err != nil {
		return nil, err
	}
	out, err := client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	info := &ObjectInfo{
		Key:          key,
		ContentType:  aws.StringValue(out.ContentType),
		ETag:         aws.StringValue(out.ETag),
		StorageClass: aws.StringValue(out.StorageClass),
		Metadata:     map[string]string{},
	}
	if out.ContentLength != nil {
		info.Size = formatSize(*out.ContentLength)
	}
	if out.LastModified != nil {
		info.LastModified = out.LastModified.Format(time.RFC3339)
	}
	if info.StorageClass == "" {
		info.StorageClass = "STANDARD"
	}
	for k, v := range out.Metadata {
		info.Metadata[k] = aws.StringValue(v)
	}
	return info, nil
}

// PresignGet returns a time-limited GET URL.
func (c *S3Client) PresignGet(bucket, key string, expiry time.Duration) (string, error) {
	client, _, err := c.bucketClient(bucket)
	if err != nil {
		return "", err
	}
	req, _ := client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return req.Presign(expiry)
}

// BucketOverview aggregates common bucket configuration.
type BucketOverview struct {
	Region            string
	Versioning        string
	Encryption        string
	PublicAccessBlock string
	ObjectCountApprox string
	TotalSizeApprox   string
	PrefixSampled     string
}

// GetBucketOverview fetches config + a quick listing sample for size/count.
func (c *S3Client) GetBucketOverview(bucket, prefix string) (*BucketOverview, error) {
	client, region, err := c.bucketClient(bucket)
	if err != nil {
		return nil, err
	}
	ov := &BucketOverview{Region: region, PrefixSampled: prefix}
	fillBucketVersioning(client, bucket, ov)
	fillBucketEncryption(client, bucket, ov)
	fillPublicAccessBlock(client, bucket, ov)
	sampleBucketObjects(client, bucket, prefix, ov)
	return ov, nil
}

func fillBucketVersioning(client *s3.S3, bucket string, ov *BucketOverview) {
	out, err := client.GetBucketVersioning(&s3.GetBucketVersioningInput{Bucket: aws.String(bucket)})
	if err != nil {
		ov.Versioning = "n/a: " + err.Error()
		return
	}
	status := aws.StringValue(out.Status)
	if status == "" {
		status = "Disabled"
	}
	ov.Versioning = status
}

func fillBucketEncryption(client *s3.S3, bucket string, ov *BucketOverview) {
	out, err := client.GetBucketEncryption(&s3.GetBucketEncryptionInput{Bucket: aws.String(bucket)})
	if err != nil {
		ov.Encryption = "none / n/a"
		return
	}
	rules := out.ServerSideEncryptionConfiguration
	if rules != nil && len(rules.Rules) > 0 && rules.Rules[0].ApplyServerSideEncryptionByDefault != nil {
		ov.Encryption = aws.StringValue(rules.Rules[0].ApplyServerSideEncryptionByDefault.SSEAlgorithm)
		return
	}
	ov.Encryption = "none"
}

func fillPublicAccessBlock(client *s3.S3, bucket string, ov *BucketOverview) {
	out, err := client.GetPublicAccessBlock(&s3.GetPublicAccessBlockInput{Bucket: aws.String(bucket)})
	if err != nil || out.PublicAccessBlockConfiguration == nil {
		ov.PublicAccessBlock = "not set / n/a"
		return
	}
	cfg := out.PublicAccessBlockConfiguration
	ov.PublicAccessBlock = fmt.Sprintf("BlockPublicAcls=%v IgnorePublicAcls=%v BlockPublicPolicy=%v RestrictPublicBuckets=%v",
		aws.BoolValue(cfg.BlockPublicAcls), aws.BoolValue(cfg.IgnorePublicAcls),
		aws.BoolValue(cfg.BlockPublicPolicy), aws.BoolValue(cfg.RestrictPublicBuckets))
}

func sampleBucketObjects(client *s3.S3, bucket, prefix string, ov *BucketOverview) {
	var count, total int64
	truncated := false
	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucket),
		MaxKeys: aws.Int64(1000),
	}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}
	err := client.ListObjectsV2Pages(input, func(page *s3.ListObjectsV2Output, last bool) bool {
		for _, obj := range page.Contents {
			count++
			if obj.Size != nil {
				total += *obj.Size
			}
		}
		if aws.BoolValue(page.IsTruncated) && count >= 5000 {
			truncated = true
			return false
		}
		return !last
	})
	if err != nil {
		ov.ObjectCountApprox = "error: " + err.Error()
		ov.TotalSizeApprox = "n/a"
		return
	}
	suffix := ""
	if truncated {
		suffix = "+"
	}
	ov.ObjectCountApprox = fmt.Sprintf("%d%s", count, suffix)
	ov.TotalSizeApprox = formatSize(total) + suffix
}

// VersionRow is one object version listing row.
type VersionRow struct {
	Key          string
	VersionID    string
	IsLatest     string
	Size         string
	LastModified string
	StorageClass string
}

// ListObjectVersions lists versions under a prefix (max 200).
func (c *S3Client) ListObjectVersions(bucket, prefix string) ([]VersionRow, error) {
	client, _, err := c.bucketClient(bucket)
	if err != nil {
		return nil, err
	}
	input := &s3.ListObjectVersionsInput{
		Bucket:  aws.String(bucket),
		MaxKeys: aws.Int64(200),
	}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}
	out, err := client.ListObjectVersions(input)
	if err != nil {
		return nil, err
	}
	rows := make([]VersionRow, 0, len(out.Versions))
	for _, v := range out.Versions {
		size := ""
		if v.Size != nil {
			size = formatSize(*v.Size)
		}
		mod := ""
		if v.LastModified != nil {
			mod = v.LastModified.Format(time.RFC3339)
		}
		latest := "no"
		if aws.BoolValue(v.IsLatest) {
			latest = "yes"
		}
		storage := aws.StringValue(v.StorageClass)
		if storage == "" {
			storage = "STANDARD"
		}
		rows = append(rows, VersionRow{
			Key:          aws.StringValue(v.Key),
			VersionID:    aws.StringValue(v.VersionId),
			IsLatest:     latest,
			Size:         size,
			LastModified: mod,
			StorageClass: storage,
		})
	}
	return rows, nil
}

// ACLRow is one ACL grant.
type ACLRow struct {
	Grantee    string
	Type       string
	Permission string
}

// GetBucketACL returns bucket ACL grants.
func (c *S3Client) GetBucketACL(bucket string) (owner string, rows []ACLRow, err error) {
	client, _, err := c.bucketClient(bucket)
	if err != nil {
		return "", nil, err
	}
	out, err := client.GetBucketAcl(&s3.GetBucketAclInput{Bucket: aws.String(bucket)})
	if err != nil {
		return "", nil, err
	}
	if out.Owner != nil {
		owner = aws.StringValue(out.Owner.DisplayName)
		if owner == "" {
			owner = aws.StringValue(out.Owner.ID)
		}
	}
	for _, g := range out.Grants {
		rows = append(rows, ACLRow{
			Permission: aws.StringValue(g.Permission),
			Type:       granteeType(g.Grantee),
			Grantee:    granteeName(g.Grantee),
		})
	}
	return owner, rows, nil
}

func granteeType(g *s3.Grantee) string {
	if g == nil {
		return ""
	}
	return aws.StringValue(g.Type)
}

func granteeName(g *s3.Grantee) string {
	if g == nil {
		return ""
	}
	for _, v := range []string{
		aws.StringValue(g.DisplayName),
		aws.StringValue(g.ID),
		aws.StringValue(g.URI),
		aws.StringValue(g.EmailAddress),
	} {
		if v != "" {
			return v
		}
	}
	return ""
}

// LifecycleRow is one lifecycle rule summary.
type LifecycleRow struct {
	ID      string
	Status  string
	Prefix  string
	Summary string
}

// ListLifecycleRules returns bucket lifecycle configuration.
func (c *S3Client) ListLifecycleRules(bucket string) ([]LifecycleRow, error) {
	client, _, err := c.bucketClient(bucket)
	if err != nil {
		return nil, err
	}
	out, err := client.GetBucketLifecycleConfiguration(&s3.GetBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return nil, err
	}
	rows := make([]LifecycleRow, 0, len(out.Rules))
	for _, rule := range out.Rules {
		rows = append(rows, lifecycleRuleRow(rule))
	}
	return rows, nil
}

func lifecycleRulePrefix(rule *s3.LifecycleRule) string {
	if rule.Filter != nil && rule.Filter.Prefix != nil {
		return aws.StringValue(rule.Filter.Prefix)
	}
	if rule.Prefix != nil {
		return aws.StringValue(rule.Prefix)
	}
	return ""
}

func lifecycleRuleSummary(rule *s3.LifecycleRule) string {
	var parts []string
	if rule.Expiration != nil && rule.Expiration.Days != nil {
		parts = append(parts, fmt.Sprintf("expire %dd", *rule.Expiration.Days))
	}
	for _, t := range rule.Transitions {
		days := int64(0)
		if t.Days != nil {
			days = *t.Days
		}
		parts = append(parts, fmt.Sprintf("→%s %dd", aws.StringValue(t.StorageClass), days))
	}
	if rule.AbortIncompleteMultipartUpload != nil && rule.AbortIncompleteMultipartUpload.DaysAfterInitiation != nil {
		parts = append(parts, fmt.Sprintf("abort MPU %dd", *rule.AbortIncompleteMultipartUpload.DaysAfterInitiation))
	}
	summary := strings.Join(parts, ", ")
	if summary == "" {
		return "-"
	}
	return summary
}

func lifecycleRuleRow(rule *s3.LifecycleRule) LifecycleRow {
	return LifecycleRow{
		ID:      aws.StringValue(rule.ID),
		Status:  aws.StringValue(rule.Status),
		Prefix:  lifecycleRulePrefix(rule),
		Summary: lifecycleRuleSummary(rule),
	}
}

// MultipartRow is an incomplete multipart upload.
type MultipartRow struct {
	Key       string
	UploadID  string
	Initiated string
}

// ListMultipartUploads lists incomplete multipart uploads.
func (c *S3Client) ListMultipartUploads(bucket, prefix string) ([]MultipartRow, error) {
	client, _, err := c.bucketClient(bucket)
	if err != nil {
		return nil, err
	}
	input := &s3.ListMultipartUploadsInput{
		Bucket:     aws.String(bucket),
		MaxUploads: aws.Int64(100),
	}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}
	out, err := client.ListMultipartUploads(input)
	if err != nil {
		return nil, err
	}
	rows := make([]MultipartRow, 0, len(out.Uploads))
	for _, u := range out.Uploads {
		initiated := ""
		if u.Initiated != nil {
			initiated = u.Initiated.Format(time.RFC3339)
		}
		rows = append(rows, MultipartRow{
			Key:       aws.StringValue(u.Key),
			UploadID:  aws.StringValue(u.UploadId),
			Initiated: initiated,
		})
	}
	return rows, nil
}

// AbortMultipartUpload aborts an incomplete upload.
func (c *S3Client) AbortMultipartUpload(bucket, key, uploadID string) error {
	client, _, err := c.bucketClient(bucket)
	if err != nil {
		return err
	}
	_, err = client.AbortMultipartUpload(&s3.AbortMultipartUploadInput{
		Bucket:   aws.String(bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	return err
}

// PeekObject reads up to maxBytes of an object for a preview modal.
func (c *S3Client) PeekObject(bucket, key string, maxBytes int64) (string, error) {
	client, _, err := c.bucketClient(bucket)
	if err != nil {
		return "", err
	}
	out, err := client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Range:  aws.String(fmt.Sprintf("bytes=0-%d", maxBytes-1)),
	})
	if err != nil {
		return "", err
	}
	defer out.Body.Close()
	data, err := io.ReadAll(io.LimitReader(out.Body, maxBytes))
	if err != nil {
		return "", err
	}
	ct := aws.StringValue(out.ContentType)
	if strings.HasPrefix(ct, "text/") || strings.Contains(ct, "json") || strings.Contains(ct, "xml") || ct == "" {
		return string(data), nil
	}
	return fmt.Sprintf("(binary content-type=%s, showing %d bytes hex)\n%x", ct, len(data), data[:min(64, len(data))]), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
