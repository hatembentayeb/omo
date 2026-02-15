package main

import (
	"omo/pkg/ui"
)

const (
	s3ViewRoot    = "s3"
	s3ViewBuckets = "buckets"
	s3ViewObjects = "objects"
	s3ViewMetrics = "metrics"
)

func (bv *BucketsView) currentCores() *ui.CoreView {
	switch bv.currentViewName {
	case s3ViewObjects:
		return bv.objectsView
	default:
		return bv.cores
	}
}

func (bv *BucketsView) setViewStack(cores *ui.CoreView, viewName string) {
	if cores == nil {
		return
	}

	stack := []string{s3ViewRoot, s3ViewBuckets}
	if viewName != s3ViewBuckets {
		stack = append(stack, viewName)
	}
	cores.SetViewStack(stack)
}

func (bv *BucketsView) switchView(viewName string) {
	pageName := "s3-" + viewName
	bv.currentViewName = viewName
	bv.viewPages.SwitchToPage(pageName)

	bv.setViewStack(bv.currentCores(), viewName)
	current := bv.currentCores()
	if current != nil {
		current.RefreshData()
		bv.app.SetFocus(current.GetTable())
	}
}

func (bv *BucketsView) showBucketsView() {
	bv.currentPrefix = ""
	bv.switchView(s3ViewBuckets)
}

func (bv *BucketsView) showObjectsView() {
	bv.switchView(s3ViewObjects)
}
