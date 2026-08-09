package s3

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

const (
	errNoBucketSelected = "no bucket selected"
	errNoUploadSelected = "no upload selected"
	errNoObjectSelected = "no object selected"
)

// Service is the RPC-facing S3 backend (no tview).
type Service struct {
	mu            sync.Mutex
	client        *S3Client
	profile       string
	region        string
	accessKey     string
	secretKey     string
	endpoint      string
	currentBucket string
	currentPrefix string
	currentView   string
}

// NewService creates an S3 RPC service.
func NewService() *Service {
	return &Service{
		client:      NewS3Client(),
		region:      "us-east-1",
		currentView: s3ViewBuckets,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "s3",
		Version:     "1.3.0",
		Description: "Browse and manage S3 buckets, objects, ACL, lifecycle, versions",
		Author:      "HATMAN",
		License:     "Apache-2.0",
		Tags:        []string{"storage", "cloud", "aws", "s3"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/s3",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("Service.Configure begin")

	if req.Settings == nil {
		return fmt.Errorf("missing settings")
	}
	s.profile = req.Settings["name"]
	s.accessKey = req.Settings["username"]
	s.secretKey = req.Settings["password"]
	s.endpoint = req.Settings["host"]
	if s.endpoint == "" {
		s.endpoint = req.Settings["url"]
	}
	s.region = req.Settings["region"]
	if s.region == "" {
		s.region = "us-east-1"
	}
	if s.endpoint != "" && !strings.Contains(s.endpoint, "://") && !strings.Contains(s.endpoint, ".") {
		s.region = s.endpoint
		s.endpoint = ""
	}
	if s.profile == "" && s.accessKey == "" {
		return fmt.Errorf("name (profile) or username (access key) required")
	}

	pluginrpc.RPCLog("Service.Configure profile=%s region=%s endpoint=%s", s.profile, s.region, s.endpoint)
	if s.client != nil && s.client.IsConnected() {
		s.client.Disconnect()
	}
	s.currentBucket = ""
	s.currentPrefix = ""
	return nil
}

func (s *Service) GetView(req pluginrpc.ViewRequest) (pluginrpc.ViewData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	viewID := req.View
	if viewID == "" {
		viewID = s.currentView
	}
	if viewID == "" {
		viewID = s3ViewBuckets
	}
	pluginrpc.RPCLog("Service.GetView begin view=%s", viewID)
	start := time.Now()
	view, err := s.buildViewLocked(viewID)
	if err != nil {
		pluginrpc.RPCLog("Service.GetView err=%v", err)
		return pluginrpc.ViewData{}, err
	}
	pluginrpc.RPCLog("Service.GetView OK view=%s rows=%d dur=%s", view.View, len(view.Rows), time.Since(start))
	return view, nil
}

func (s *Service) DoAction(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	action := req.Action
	key := req.Payload["key"]

	if strings.HasPrefix(action, "goto_") {
		return s.gotoViewLocked(strings.TrimPrefix(action, "goto_"), key)
	}

	switch action {
	case "refresh", "":
		return s.refreshLocked()

	case "open_objects":
		bucket := key
		if bucket == "" {
			bucket = s.currentBucket
		}
		if bucket == "" || bucket == "(no buckets)" {
			return pluginrpc.ActionResult{OK: false, Message: errNoBucketSelected}, nil
		}
		s.currentBucket = bucket
		s.currentPrefix = ""
		view, err := s.buildViewLocked(s3ViewObjects)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "opened " + bucket, Next: &view}, nil

	case "navigate":
		return s.navigateLocked(req)

	case "up":
		return s.navigateUpLocked()

	case "object_info":
		return s.objectInfoLocked(req)

	case "bucket_info":
		return s.bucketInfoLocked(key)

	case "peek":
		return s.peekLocked(req)

	case "presign":
		return s.presignLocked(req)

	case "copy_uri":
		return s.copyURILocked(req)

	case "create_bucket":
		name := strings.TrimSpace(req.Payload["name"])
		if name == "" {
			name = strings.TrimSpace(key)
		}
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "bucket name required"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.CreateBucket(name, s.region); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, err := s.buildViewLocked(s3ViewBuckets)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "created " + name, Next: &view}, nil

	case "create_folder":
		name := strings.TrimSpace(req.Payload["name"])
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "folder name required"}, nil
		}
		if s.currentBucket == "" {
			return pluginrpc.ActionResult{OK: false, Message: errNoBucketSelected}, nil
		}
		name = strings.Trim(name, "/")
		folderKey := s.currentPrefix + name + "/"
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.PutEmptyObject(s.currentBucket, folderKey); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, err := s.buildViewLocked(s3ViewObjects)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "created " + folderKey, Next: &view}, nil

	case "delete":
		return s.deleteLocked(req)

	case "abort_upload":
		if s.currentBucket == "" || key == "" {
			return pluginrpc.ActionResult{OK: false, Message: errNoUploadSelected}, nil
		}
		uploadID := req.Payload["col3"]
		if uploadID == "" {
			uploadID = req.Payload["col1"]
		}
		if uploadID == "" || strings.Contains(uploadID, "…") {
			return pluginrpc.ActionResult{OK: false, Message: "full upload id required (col3)"}, nil
		}
		if err := s.client.AbortMultipartUpload(s.currentBucket, key, uploadID); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, err := s.buildViewLocked(s3ViewUploads)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "aborted upload", Next: &view}, nil

	case "version_info":
		body := fmt.Sprintf("Key: %s\nVersion: %s\nLatest: %s\nSize: %s\nModified: %s\nBucket: %s\nURI: s3://%s/%s",
			key, req.Payload["col1"], req.Payload["col2"], req.Payload["col3"], req.Payload["col4"],
			s.currentBucket, s.currentBucket, key)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Version", ModalBody: body}, nil

	case "acl_info":
		if key == "" || strings.HasPrefix(key, "(error)") {
			return pluginrpc.ActionResult{OK: false, Message: "no grant selected"}, nil
		}
		body := fmt.Sprintf("Bucket: %s\nGrantee: %s\nType: %s\nPermission: %s\nURI: s3://%s/",
			s.currentBucket, key, req.Payload["col1"], req.Payload["col2"], s.currentBucket)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "ACL Grant", ModalBody: body}, nil

	case "lifecycle_info":
		if key == "" || key == "(none)" {
			return pluginrpc.ActionResult{OK: false, Message: "no rule selected"}, nil
		}
		body := fmt.Sprintf("Bucket: %s\nRule ID: %s\nStatus: %s\nPrefix: %s\nSummary: %s",
			s.currentBucket, key, req.Payload["col1"], req.Payload["col2"], req.Payload["col3"])
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Lifecycle Rule", ModalBody: body}, nil

	case "upload_info":
		if key == "" || key == "(none)" {
			return pluginrpc.ActionResult{OK: false, Message: errNoUploadSelected}, nil
		}
		uploadID := req.Payload["col3"]
		if uploadID == "" {
			uploadID = req.Payload["col1"]
		}
		body := fmt.Sprintf("Bucket: %s\nKey: %s\nUpload ID: %s\nInitiated: %s\nURI: s3://%s/%s",
			s.currentBucket, key, uploadID, req.Payload["col2"], s.currentBucket, key)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Multipart Upload", ModalBody: body}, nil

	case "browse_key":
		return s.browseKeyLocked(key)

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) browseKeyLocked(key string) (pluginrpc.ActionResult, error) {
	if s.currentBucket == "" {
		return pluginrpc.ActionResult{OK: false, Message: errNoBucketSelected}, nil
	}
	if key == "" || strings.HasPrefix(key, "(") {
		view, err := s.buildViewLocked(s3ViewObjects)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "objects", Next: &view}, nil
	}
	// Open the object's parent prefix in the Objects browser.
	if idx := strings.LastIndex(key, "/"); idx >= 0 {
		s.currentPrefix = key[:idx+1]
	} else {
		s.currentPrefix = ""
	}
	view, err := s.buildViewLocked(s3ViewObjects)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	return pluginrpc.ActionResult{OK: true, Message: "browse " + s.currentPrefix, Next: &view}, nil
}

func (s *Service) gotoViewLocked(viewID, key string) (pluginrpc.ActionResult, error) {
	needsBucket := viewID == s3ViewObjects || viewID == s3ViewOverview ||
		viewID == s3ViewVersions || viewID == s3ViewACL ||
		viewID == s3ViewLifecycle || viewID == s3ViewUploads

	if viewID == s3ViewObjects || viewID == s3ViewOverview {
		if key != "" && s.currentView == s3ViewBuckets &&
			key != "(no buckets)" && !strings.HasPrefix(key, "(") {
			s.currentBucket = key
			s.currentPrefix = ""
		}
	}
	if needsBucket && s.currentBucket == "" {
		return pluginrpc.ActionResult{OK: false, Message: "no bucket selected — open one from Buckets (0)"}, nil
	}
	if viewID == s3ViewBuckets {
		// keep currentBucket so Overview can still be useful after browse; clear only on explicit reset
	}
	view, err := s.buildViewLocked(viewID)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	return pluginrpc.ActionResult{OK: true, Message: "switched to " + viewID, Next: &view}, nil
}

func (s *Service) refreshLocked() (pluginrpc.ActionResult, error) {
	view, err := s.buildViewLocked(s.currentView)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	return pluginrpc.ActionResult{OK: true, Message: "refreshed", Next: &view}, nil
}

func (s *Service) navigateLocked(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	name := req.Payload["key"]
	storage := req.Payload["col3"]
	if name == "../" {
		return s.navigateUpLocked()
	}
	if storage == "Directory" || strings.HasSuffix(name, "/") {
		s.currentPrefix = s.currentPrefix + name
		view, err := s.buildViewLocked(s3ViewObjects)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "cd " + s.currentPrefix, Next: &view}, nil
	}
	return s.objectInfoLocked(req)
}

func (s *Service) navigateUpLocked() (pluginrpc.ActionResult, error) {
	if s.currentPrefix == "" {
		view, err := s.buildViewLocked(s3ViewBuckets)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "buckets", Next: &view}, nil
	}
	prefix := strings.TrimSuffix(s.currentPrefix, "/")
	lastSlash := strings.LastIndex(prefix, "/")
	if lastSlash >= 0 {
		s.currentPrefix = prefix[:lastSlash+1]
	} else {
		s.currentPrefix = ""
	}
	view, err := s.buildViewLocked(s3ViewObjects)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	return pluginrpc.ActionResult{OK: true, Message: "up", Next: &view}, nil
}

func (s *Service) objectInfoLocked(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	name := req.Payload["key"]
	if name == "" || name == "../" || req.Payload["col3"] == "Directory" || strings.HasSuffix(name, "/") {
		return pluginrpc.ActionResult{OK: false, Message: errNoObjectSelected}, nil
	}
	if s.currentBucket == "" {
		return pluginrpc.ActionResult{OK: false, Message: errNoBucketSelected}, nil
	}
	fullKey := s.currentPrefix + name
	info, err := s.client.HeadObject(s.currentBucket, fullKey)
	if err != nil {
		body := fmt.Sprintf("Key: %s\nBucket: %s\nSize: %s\nLast Modified: %s\nStorage Class: %s\nProfile: %s\n\nHeadObject error:\n%v",
			fullKey, s.currentBucket, req.Payload["col1"], req.Payload["col2"], req.Payload["col3"], s.profile, err)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Object: " + name, ModalBody: body}, nil
	}
	var meta strings.Builder
	for k, v := range info.Metadata {
		fmt.Fprintf(&meta, "  %s=%s\n", k, v)
	}
	metaStr := meta.String()
	if metaStr == "" {
		metaStr = "  (none)\n"
	}
	body := fmt.Sprintf("Key: %s\nBucket: %s\nURI: s3://%s/%s\nSize: %s\nContent-Type: %s\nETag: %s\nLast Modified: %s\nStorage Class: %s\nProfile: %s\n\nMetadata:\n%s",
		info.Key, s.currentBucket, s.currentBucket, info.Key, info.Size, info.ContentType, info.ETag,
		info.LastModified, info.StorageClass, s.profile, metaStr)
	return pluginrpc.ActionResult{OK: true, ModalTitle: "Object: " + name, ModalBody: body}, nil
}

func (s *Service) bucketInfoLocked(key string) (pluginrpc.ActionResult, error) {
	bucket := key
	if bucket == "" || strings.HasPrefix(bucket, "(") {
		bucket = s.currentBucket
	}
	if bucket == "" {
		return pluginrpc.ActionResult{OK: false, Message: errNoBucketSelected}, nil
	}
	was := s.currentBucket
	s.currentBucket = bucket
	ov, err := s.client.GetBucketOverview(bucket, "")
	if was != "" && was != bucket && s.currentView != s3ViewBuckets {
		s.currentBucket = was
	}
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	body := fmt.Sprintf("Bucket: %s\nURI: s3://%s/\nRegion: %s\nVersioning: %s\nEncryption: %s\nPublic access: %s\nObjects (sample): %s\nSize (sample): %s\nProfile: %s",
		bucket, bucket, ov.Region, ov.Versioning, ov.Encryption, ov.PublicAccessBlock,
		ov.ObjectCountApprox, ov.TotalSizeApprox, s.profile)
	return pluginrpc.ActionResult{OK: true, ModalTitle: "Bucket: " + bucket, ModalBody: body}, nil
}

func (s *Service) peekLocked(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	name := req.Payload["key"]
	if name == "" || name == "../" || req.Payload["col3"] == "Directory" || strings.HasSuffix(name, "/") {
		return pluginrpc.ActionResult{OK: false, Message: errNoObjectSelected}, nil
	}
	fullKey := s.currentPrefix + name
	data, err := s.client.PeekObject(s.currentBucket, fullKey, 4096)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	return pluginrpc.ActionResult{OK: true, ModalTitle: "Peek: " + name, ModalBody: data}, nil
}

func (s *Service) presignLocked(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	name := req.Payload["key"]
	if name == "" || name == "../" || req.Payload["col3"] == "Directory" || strings.HasSuffix(name, "/") {
		return pluginrpc.ActionResult{OK: false, Message: errNoObjectSelected}, nil
	}
	fullKey := s.currentPrefix + name
	url, err := s.client.PresignGet(s.currentBucket, fullKey, time.Hour)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	body := fmt.Sprintf("GET presigned URL (1h):\n\n%s\n\nKey: s3://%s/%s", url, s.currentBucket, fullKey)
	return pluginrpc.ActionResult{OK: true, ModalTitle: "Presign: " + name, ModalBody: body}, nil
}

func (s *Service) copyURILocked(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	uri, errMsg := s.resolveCopyURI(req)
	if errMsg != "" {
		return pluginrpc.ActionResult{OK: false, Message: errMsg}, nil
	}
	return pluginrpc.ActionResult{OK: true, ModalTitle: "S3 URI", ModalBody: uri + "\n\n(copy from here)"}, nil
}

func (s *Service) resolveCopyURI(req pluginrpc.ActionRequest) (uri, errMsg string) {
	switch s.currentView {
	case s3ViewBuckets:
		if req.Payload["key"] == "" || strings.HasPrefix(req.Payload["key"], "(") {
			return "", errNoBucketSelected
		}
		return "s3://" + req.Payload["key"] + "/", ""
	case s3ViewOverview, s3ViewACL, s3ViewLifecycle:
		if s.currentBucket == "" {
			return "", errNoBucketSelected
		}
		return fmt.Sprintf("s3://%s/%s", s.currentBucket, s.currentPrefix), ""
	case s3ViewVersions, s3ViewUploads:
		name := req.Payload["key"]
		if name == "" || strings.HasPrefix(name, "(") {
			return fmt.Sprintf("s3://%s/%s", s.currentBucket, s.currentPrefix), ""
		}
		return fmt.Sprintf("s3://%s/%s", s.currentBucket, name), ""
	default:
		return s.objectViewURI(req.Payload["key"]), ""
	}
}

func (s *Service) objectViewURI(name string) string {
	if name == "" || name == "../" {
		return fmt.Sprintf("s3://%s/%s", s.currentBucket, s.currentPrefix)
	}
	return fmt.Sprintf("s3://%s/%s%s", s.currentBucket, s.currentPrefix, name)
}

func (s *Service) deleteLocked(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	if err := s.ensureConnectedLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}

	switch s.currentView {
	case s3ViewBuckets:
		return s.deleteBucketLocked(req.Payload["key"])
	case s3ViewObjects:
		return s.deleteObjectLocked(req)
	case s3ViewUploads:
		return s.abortUploadLocked(req.Payload["key"], req.Payload["col3"])
	default:
		return pluginrpc.ActionResult{OK: false, Message: "delete not available in this view"}, nil
	}
}

func (s *Service) deleteBucketLocked(name string) (pluginrpc.ActionResult, error) {
	if name == "" || strings.HasPrefix(name, "(") {
		return pluginrpc.ActionResult{OK: false, Message: errNoBucketSelected}, nil
	}
	if err := s.client.DeleteBucket(name); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	if s.currentBucket == name {
		s.currentBucket = ""
		s.currentPrefix = ""
	}
	view, err := s.buildViewLocked(s3ViewBuckets)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	return pluginrpc.ActionResult{OK: true, Message: "deleted bucket " + name, Next: &view}, nil
}

func (s *Service) deleteObjectLocked(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	name := req.Payload["key"]
	if name == "" || name == "../" || req.Payload["col3"] == "Directory" || strings.HasSuffix(name, "/") {
		return pluginrpc.ActionResult{OK: false, Message: "select a file object to delete (not a folder)"}, nil
	}
	fullKey := s.currentPrefix + name
	if err := s.client.DeleteObject(s.currentBucket, fullKey); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	view, err := s.buildViewLocked(s3ViewObjects)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	return pluginrpc.ActionResult{OK: true, Message: "deleted " + fullKey, Next: &view}, nil
}

func (s *Service) abortUploadLocked(name, uploadID string) (pluginrpc.ActionResult, error) {
	if name == "" || uploadID == "" {
		return pluginrpc.ActionResult{OK: false, Message: errNoUploadSelected}, nil
	}
	if err := s.client.AbortMultipartUpload(s.currentBucket, name, uploadID); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	view, err := s.buildViewLocked(s3ViewUploads)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	return pluginrpc.ActionResult{OK: true, Message: "aborted upload", Next: &view}, nil
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil {
		s.client.Disconnect()
	}
	return nil
}

func (s *Service) ensureConnectedLocked() error {
	if s.client != nil && s.client.IsConnected() {
		return nil
	}
	if s.profile == "" && s.accessKey == "" {
		return fmt.Errorf("not configured")
	}
	return s.client.Connect(s.profile, s.region, s.accessKey, s.secretKey, s.endpoint)
}
