package s3

import (
	"fmt"
	"strings"

	"omo/pkg/pluginrpc"
)

func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Buckets", Action: "goto_buckets"},
		{Key: "1", Label: "Objects", Action: "goto_objects"},
		{Key: "2", Label: "Overview", Action: "goto_overview"},
		{Key: "3", Label: "Versions", Action: "goto_versions"},
		{Key: "4", Label: "ACL", Action: "goto_acl"},
		{Key: "5", Label: "Lifecycle", Action: "goto_lifecycle"},
		{Key: "6", Label: "Uploads", Action: "goto_uploads"},
	}
}

// Shared verbs across views:
//
//	E = primary open / browse
//	I = info / details modal
//	C = copy S3 URI
//	D = delete / abort (when applicable)
func bucketsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Open", Action: "open_objects"},
		{Key: "I", Label: "Info", Action: "bucket_info"},
		{Key: "N", Label: "New Bucket", Action: "create_bucket"},
		{Key: "D", Label: "Delete", Action: "delete"},
		{Key: "C", Label: "Copy URI", Action: "copy_uri"},
	}
}

func objectsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Open", Action: "navigate"},
		{Key: "U", Label: "Up", Action: "up"},
		{Key: "I", Label: "Info", Action: "object_info"},
		{Key: "V", Label: "Peek", Action: "peek"},
		{Key: "N", Label: "New Folder", Action: "create_folder"},
		{Key: "P", Label: "Presign", Action: "presign"},
		{Key: "D", Label: "Delete", Action: "delete"},
		{Key: "C", Label: "Copy URI", Action: "copy_uri"},
	}
}

func overviewActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Browse", Action: "open_objects"},
		{Key: "I", Label: "Info", Action: "bucket_info"},
		{Key: "C", Label: "Copy URI", Action: "copy_uri"},
	}
}

func versionsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Browse", Action: "browse_key"},
		{Key: "I", Label: "Info", Action: "version_info"},
		{Key: "C", Label: "Copy URI", Action: "copy_uri"},
	}
}

func aclActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Browse", Action: "open_objects"},
		{Key: "I", Label: "Info", Action: "acl_info"},
		{Key: "C", Label: "Copy URI", Action: "copy_uri"},
	}
}

func lifecycleActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Browse", Action: "open_objects"},
		{Key: "I", Label: "Info", Action: "lifecycle_info"},
		{Key: "C", Label: "Copy URI", Action: "copy_uri"},
	}
}

func uploadsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Browse", Action: "browse_key"},
		{Key: "I", Label: "Info", Action: "upload_info"},
		{Key: "D", Label: "Abort", Action: "abort_upload"},
		{Key: "C", Label: "Copy URI", Action: "copy_uri"},
	}
}

func noBucketActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Pick Bucket", Action: "goto_buckets"},
	}
}

func actionsForView(viewID string) []pluginrpc.KeyBinding {
	switch viewID {
	case s3ViewBuckets:
		return bucketsActions()
	case s3ViewObjects:
		return objectsActions()
	case s3ViewOverview:
		return overviewActions()
	case s3ViewVersions:
		return versionsActions()
	case s3ViewACL:
		return aclActions()
	case s3ViewLifecycle:
		return lifecycleActions()
	case s3ViewUploads:
		return uploadsActions()
	default:
		return bucketsActions()
	}
}

func helpSections() []pluginrpc.HelpSection {
	return pluginrpc.HelpWithGlobal([]pluginrpc.HelpSection{
		{Title: "Views (0-6)", Bindings: viewNavBindings()},
		{Title: "Buckets", Bindings: bucketsActions()},
		{Title: "Objects", Bindings: objectsActions()},
		{Title: "Overview", Bindings: overviewActions()},
		{Title: "Versions", Bindings: versionsActions()},
		{Title: "ACL", Bindings: aclActions()},
		{Title: "Lifecycle", Bindings: lifecycleActions()},
		{Title: "Uploads", Bindings: uploadsActions()},
	}...)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	Help:  helpSections,
}

func (s *Service) baseInfo(extra string) string {
	msg := fmt.Sprintf("[green]S3 Manager[white]\nProfile: %s\nRegion: %s\nView: %s",
		s.profile, s.region, s.currentView)
	if s.endpoint != "" {
		msg += "\nEndpoint: " + s.endpoint
	}
	if s.currentBucket != "" {
		prefix := s.currentPrefix
		if prefix == "" {
			prefix = "/"
		}
		msg += fmt.Sprintf("\nBucket: %s\nPrefix: %s", s.currentBucket, prefix)
	}
	return pluginrpc.FormatInfo(msg, extra)
}

func (s *Service) disconnectedView(viewID string, err error) pluginrpc.ViewData {
	return ui.Decorate(pluginrpc.StatusErrorView(
		viewID, "S3 Manager",
		"[yellow]S3 Manager[white]\nStatus: Not Connected\n"+err.Error(),
		"not connected", err.Error(),
	), actionsForView(viewID)...)
}

func (s *Service) needBucketView(viewID string) pluginrpc.ViewData {
	return ui.Decorate(pluginrpc.ViewData{
		View:         viewID,
		Title:        "S3 — " + viewID,
		Info:         s.baseInfo("Select a bucket first (view 0)"),
		Status:       "connected",
		Headers:      []string{"Status", "Hint"},
		Rows:         [][]string{{"No bucket selected", "Press 0 or E to pick a bucket"}},
		SelectionKey: "Status",
	}, noBucketActions()...)
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = s3ViewBuckets
	}
	s.currentView = viewID

	if err := s.ensureConnectedLocked(); err != nil {
		return s.disconnectedView(viewID, err), nil
	}

	switch viewID {
	case s3ViewObjects:
		return s.viewObjectsLocked()
	case s3ViewOverview:
		return s.viewOverviewLocked()
	case s3ViewVersions:
		return s.viewVersionsLocked()
	case s3ViewACL:
		return s.viewACLLocked()
	case s3ViewLifecycle:
		return s.viewLifecycleLocked()
	case s3ViewUploads:
		return s.viewUploadsLocked()
	default:
		return s.viewBucketsLocked()
	}
}

func (s *Service) viewBucketsLocked() (pluginrpc.ViewData, error) {
	buckets, err := s.client.ListBuckets()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(buckets))
	for _, b := range buckets {
		rows = append(rows, []string{b.Name, b.CreationDate, b.Region})
	}
	if len(rows) == 0 {
		rows = [][]string{{"(no buckets)", "", ""}}
	}
	return ui.Decorate(pluginrpc.ViewData{
		View:         s3ViewBuckets,
		Title:        "S3 Buckets",
		Info:         s.baseInfo(fmt.Sprintf("Buckets: %d", len(buckets))),
		Status:       "connected",
		Headers:      []string{"Name", "Created", "Region"},
		Rows:         rows,
		SelectionKey: "Name",
	}, bucketsActions()...), nil
}

func (s *Service) viewObjectsLocked() (pluginrpc.ViewData, error) {
	if s.currentBucket == "" {
		return s.needBucketView(s3ViewObjects), nil
	}
	objects, err := s.client.ListObjects(s.currentBucket, s.currentPrefix)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(objects))
	for _, o := range objects {
		rows = append(rows, []string{o.Name, o.Size, o.LastModified, o.StorageClass})
	}
	if len(rows) == 0 {
		rows = [][]string{{"(empty)", "", "", ""}}
	}
	return ui.Decorate(pluginrpc.ViewData{
		View:         s3ViewObjects,
		Title:        "S3 Objects",
		Info:         s.baseInfo(fmt.Sprintf("Entries: %d", len(objects))),
		Status:       "connected",
		Headers:      []string{"Name", "Size", "Last Modified", "Storage Class"},
		Rows:         rows,
		SelectionKey: "Name",
	}, objectsActions()...), nil
}

func (s *Service) viewOverviewLocked() (pluginrpc.ViewData, error) {
	if s.currentBucket == "" {
		return s.needBucketView(s3ViewOverview), nil
	}
	ov, err := s.client.GetBucketOverview(s.currentBucket, s.currentPrefix)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := [][]string{
		{"Region", ov.Region},
		{"Versioning", ov.Versioning},
		{"Encryption", ov.Encryption},
		{"Public access", ov.PublicAccessBlock},
		{"Objects (sample)", ov.ObjectCountApprox},
		{"Size (sample)", ov.TotalSizeApprox},
		{"Prefix", emptyDash(ov.PrefixSampled)},
		{"URI", fmt.Sprintf("s3://%s/%s", s.currentBucket, s.currentPrefix)},
	}
	return ui.Decorate(pluginrpc.ViewData{
		View:         s3ViewOverview,
		Title:        "S3 Bucket Overview",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Property", "Value"},
		Rows:         rows,
		SelectionKey: "Property",
	}, overviewActions()...), nil
}

func (s *Service) viewVersionsLocked() (pluginrpc.ViewData, error) {
	if s.currentBucket == "" {
		return s.needBucketView(s3ViewVersions), nil
	}
	versions, err := s.client.ListObjectVersions(s.currentBucket, s.currentPrefix)
	if err != nil {
		return ui.Decorate(pluginrpc.ViewData{
			View:         s3ViewVersions,
			Title:        "S3 Versions",
			Info:         s.baseInfo(err.Error()),
			Status:       "connected",
			Headers:      []string{"Key", "Version", "Latest", "Size", "Modified"},
			Rows:         [][]string{{"(error)", err.Error(), "", "", ""}},
			SelectionKey: "Key",
		}, versionsActions()...), nil
	}
	rows := make([][]string, 0, len(versions))
	for _, v := range versions {
		// Keep full VersionID in the Version column so Info/actions stay accurate.
		rows = append(rows, []string{v.Key, v.VersionID, v.IsLatest, v.Size, v.LastModified})
	}
	if len(rows) == 0 {
		rows = [][]string{{"(none)", "", "", "", ""}}
	}
	return ui.Decorate(pluginrpc.ViewData{
		View:         s3ViewVersions,
		Title:        "S3 Object Versions",
		Info:         s.baseInfo(fmt.Sprintf("Versions: %d (max 200)", len(versions))),
		Status:       "connected",
		Headers:      []string{"Key", "Version", "Latest", "Size", "Modified"},
		Rows:         rows,
		SelectionKey: "Key",
	}, versionsActions()...), nil
}

func (s *Service) viewACLLocked() (pluginrpc.ViewData, error) {
	if s.currentBucket == "" {
		return s.needBucketView(s3ViewACL), nil
	}
	owner, grants, err := s.client.GetBucketACL(s.currentBucket)
	if err != nil {
		return ui.Decorate(pluginrpc.ViewData{
			View:         s3ViewACL,
			Title:        "S3 Bucket ACL",
			Info:         s.baseInfo(err.Error()),
			Status:       "connected",
			Headers:      []string{"Grantee", "Type", "Permission"},
			Rows:         [][]string{{"(error)", err.Error(), ""}},
			SelectionKey: "Grantee",
		}, aclActions()...), nil
	}
	rows := make([][]string, 0, len(grants)+1)
	rows = append(rows, []string{"(owner) " + owner, "Owner", "FULL_CONTROL"})
	for _, g := range grants {
		rows = append(rows, []string{g.Grantee, g.Type, g.Permission})
	}
	return ui.Decorate(pluginrpc.ViewData{
		View:         s3ViewACL,
		Title:        "S3 Bucket ACL",
		Info:         s.baseInfo(fmt.Sprintf("Grants: %d", len(grants))),
		Status:       "connected",
		Headers:      []string{"Grantee", "Type", "Permission"},
		Rows:         rows,
		SelectionKey: "Grantee",
	}, aclActions()...), nil
}

func (s *Service) viewLifecycleLocked() (pluginrpc.ViewData, error) {
	if s.currentBucket == "" {
		return s.needBucketView(s3ViewLifecycle), nil
	}
	rules, err := s.client.ListLifecycleRules(s.currentBucket)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "NoSuchLifecycleConfiguration") || strings.Contains(msg, "Lifecycle") {
			msg = "No lifecycle configuration"
		}
		return ui.Decorate(pluginrpc.ViewData{
			View:         s3ViewLifecycle,
			Title:        "S3 Lifecycle",
			Info:         s.baseInfo(msg),
			Status:       "connected",
			Headers:      []string{"ID", "Status", "Prefix", "Summary"},
			Rows:         [][]string{{"(none)", "", "", msg}},
			SelectionKey: "ID",
		}, lifecycleActions()...), nil
	}
	rows := make([][]string, 0, len(rules))
	for _, r := range rules {
		rows = append(rows, []string{r.ID, r.Status, emptyDash(r.Prefix), r.Summary})
	}
	if len(rows) == 0 {
		rows = [][]string{{"(none)", "", "", ""}}
	}
	return ui.Decorate(pluginrpc.ViewData{
		View:         s3ViewLifecycle,
		Title:        "S3 Lifecycle",
		Info:         s.baseInfo(fmt.Sprintf("Rules: %d", len(rules))),
		Status:       "connected",
		Headers:      []string{"ID", "Status", "Prefix", "Summary"},
		Rows:         rows,
		SelectionKey: "ID",
	}, lifecycleActions()...), nil
}

func (s *Service) viewUploadsLocked() (pluginrpc.ViewData, error) {
	if s.currentBucket == "" {
		return s.needBucketView(s3ViewUploads), nil
	}
	uploads, err := s.client.ListMultipartUploads(s.currentBucket, s.currentPrefix)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(uploads))
	for _, u := range uploads {
		rows = append(rows, []string{u.Key, shortID(u.UploadID), u.Initiated, u.UploadID})
	}
	if len(rows) == 0 {
		rows = [][]string{{"(none)", "", "", ""}}
	}
	return ui.Decorate(pluginrpc.ViewData{
		View:         s3ViewUploads,
		Title:        "S3 Multipart Uploads",
		Info:         s.baseInfo(fmt.Sprintf("Incomplete: %d", len(uploads))),
		Status:       "connected",
		Headers:      []string{"Key", "Upload ID", "Initiated", "Full Upload ID"},
		Rows:         rows,
		SelectionKey: "Key",
	}, uploadsActions()...), nil
}

func emptyDash(s string) string {
	if s == "" {
		return "/"
	}
	return s
}

func shortID(id string) string {
	if len(id) <= 12 {
		return id
	}
	return id[:8] + "…"
}
