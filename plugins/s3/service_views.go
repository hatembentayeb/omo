package s3

import (
	"fmt"

	"omo/pkg/pluginrpc"
)

func navBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "B", Label: "Buckets", Action: "goto_buckets"},
		{Key: "O", Label: "Objects", Action: "goto_objects"},
	}
}

func withNav(extra ...pluginrpc.KeyBinding) []pluginrpc.KeyBinding {
	out := make([]pluginrpc.KeyBinding, 0, len(extra)+len(navBindings())+1)
	out = append(out, pluginrpc.KeyBinding{Key: "R", Label: "Refresh", Action: "refresh"})
	out = append(out, extra...)
	out = append(out, navBindings()...)
	return out
}

func (s *Service) baseInfo(extra string) string {
	msg := fmt.Sprintf("[green]S3 Manager[white]\nProfile: %s\nRegion: %s\nView: %s",
		s.profile, s.region, s.currentView)
	if s.currentBucket != "" {
		prefix := s.currentPrefix
		if prefix == "" {
			prefix = "/"
		}
		msg += fmt.Sprintf("\nBucket: %s\nPrefix: %s", s.currentBucket, prefix)
	}
	if extra != "" {
		msg += "\n" + extra
	}
	return msg
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = s3ViewBuckets
	}
	s.currentView = viewID

	if err := s.ensureConnectedLocked(); err != nil {
		return pluginrpc.ViewData{
			View:    viewID,
			Title:   "S3 Manager",
			Info:    "[yellow]S3 Manager[white]\nStatus: Not Connected\n" + err.Error(),
			Status:  "not connected",
			Headers: []string{"Status", "Detail"},
			Rows:    [][]string{{"error", err.Error()}},
			KeyBindings: []pluginrpc.KeyBinding{
				{Key: "R", Label: "Refresh", Action: "refresh"},
			},
		}, nil
	}

	switch viewID {
	case s3ViewObjects:
		return s.viewObjectsLocked()
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
		rows = [][]string{{"No buckets", "", ""}}
	}
	return pluginrpc.ViewData{
		View:         s3ViewBuckets,
		Title:        "S3 Buckets",
		Info:         s.baseInfo(fmt.Sprintf("Buckets: %d", len(buckets))),
		Status:       "connected",
		Headers:      []string{"Name", "Creation Date", "Region"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings:  withNav(),
	}, nil
}

func (s *Service) viewObjectsLocked() (pluginrpc.ViewData, error) {
	if s.currentBucket == "" {
		return pluginrpc.ViewData{
			View:         s3ViewObjects,
			Title:        "S3 Objects",
			Info:         s.baseInfo("No bucket selected"),
			Status:       "connected",
			Headers:      []string{"Name", "Size", "Last Modified", "Storage Class"},
			Rows:         [][]string{{"No bucket selected", "Press B to go back", "", ""}},
			SelectionKey: "Name",
			KeyBindings:  withNav(),
		}, nil
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
		rows = [][]string{{"Empty bucket", "", "", ""}}
	}
	return pluginrpc.ViewData{
		View:         s3ViewObjects,
		Title:        "S3 Objects",
		Info:         s.baseInfo(""),
		Status:       "connected",
		Headers:      []string{"Name", "Size", "Last Modified", "Storage Class"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "U", Label: "Up Dir", Action: "up"},
			pluginrpc.KeyBinding{Key: "I", Label: "Info", Action: "object_info"},
			pluginrpc.KeyBinding{Key: "Enter", Label: "Open", Action: "navigate"},
		),
	}, nil
}
