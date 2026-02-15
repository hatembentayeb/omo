package main

import (
	"omo/pkg/ui"
)

const (
	viewRoot       = "docker"
	viewContainers = "containers"
	viewImages     = "images"
	viewNetworks   = "networks"
	viewVolumes    = "volumes"
	viewStats      = "stats"
	viewInspect    = "inspect"
	viewLogs       = "logs"
	viewCompose    = "compose"
	viewSystem     = "system"
)

func (dv *DockerView) currentCores() *ui.CoreView {
	switch dv.currentViewName {
	case viewImages:
		return dv.imagesView
	case viewNetworks:
		return dv.networksView
	case viewVolumes:
		return dv.volumesView
	case viewStats:
		return dv.statsView
	case viewInspect:
		return dv.inspectView
	case viewLogs:
		return dv.logsView
	case viewCompose:
		return dv.composeView
	case viewSystem:
		return dv.systemView
	default:
		return dv.containersView
	}
}

func (dv *DockerView) setViewStack(cores *ui.CoreView, viewName string) {
	if cores == nil {
		return
	}

	stack := []string{viewRoot, viewContainers}
	if viewName != viewContainers {
		stack = append(stack, viewName)
	}
	cores.SetViewStack(stack)
}

func (dv *DockerView) switchToView(viewName string) {
	pageName := "docker-" + viewName
	dv.currentViewName = viewName
	dv.viewPages.SwitchToPage(pageName)

	dv.setViewStack(dv.currentCores(), viewName)
	dv.refresh()
	current := dv.currentCores()
	if current != nil {
		dv.app.SetFocus(current.GetTable())
	}
}

func (dv *DockerView) showContainers() {
	dv.switchToView(viewContainers)
}

func (dv *DockerView) showImages() {
	dv.switchToView(viewImages)
}

func (dv *DockerView) showNetworks() {
	dv.switchToView(viewNetworks)
}

func (dv *DockerView) showVolumes() {
	dv.switchToView(viewVolumes)
}

func (dv *DockerView) showStats() {
	dv.switchToView(viewStats)
}

func (dv *DockerView) showInspect() {
	dv.switchToView(viewInspect)
}

func (dv *DockerView) showLogs() {
	dv.switchToView(viewLogs)
}

func (dv *DockerView) showCompose() {
	dv.switchToView(viewCompose)
}

func (dv *DockerView) showSystem() {
	dv.switchToView(viewSystem)
}
