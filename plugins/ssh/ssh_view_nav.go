package main

import "omo/pkg/ui"

const (
	viewRoot        = "ssh"
	viewServers     = "servers"
	viewOverview    = "overview"
	viewProcesses   = "processes"
	viewDisk        = "disk"
	viewNetwork     = "network"
	viewDocker      = "docker"
	viewServices    = "services"
	viewExec        = "exec"
)

func (sv *SSHView) currentCores() *ui.CoreView {
	switch sv.currentView {
	case viewOverview:
		return sv.overviewView
	case viewProcesses:
		return sv.processesView
	case viewDisk:
		return sv.diskView
	case viewNetwork:
		return sv.networkView
	case viewDocker:
		return sv.dockerView
	case viewServices:
		return sv.servicesView
	case viewExec:
		return sv.execView
	default:
		return sv.serversView
	}
}

func (sv *SSHView) setViewStack(cores *ui.CoreView, viewName string) {
	if cores == nil {
		return
	}
	stack := []string{viewRoot, viewServers}
	if viewName != viewServers {
		stack = append(stack, viewName)
	}
	cores.SetViewStack(stack)
}

func (sv *SSHView) switchView(viewName string) {
	pageName := "ssh-" + viewName
	sv.currentView = viewName
	sv.viewPages.SwitchToPage(pageName)

	sv.setViewStack(sv.currentCores(), viewName)
	sv.refresh()
	current := sv.currentCores()
	if current != nil {
		sv.app.SetFocus(current.GetTable())
	}
}

func (sv *SSHView) showServers()  { sv.switchView(viewServers) }
func (sv *SSHView) showOverview() { sv.switchView(viewOverview) }
func (sv *SSHView) showProcesses() { sv.switchView(viewProcesses) }
func (sv *SSHView) showDisk()     { sv.switchView(viewDisk) }
func (sv *SSHView) showNetwork()  { sv.switchView(viewNetwork) }
func (sv *SSHView) showDocker()   { sv.switchView(viewDocker) }
func (sv *SSHView) showServices() { sv.switchView(viewServices) }
func (sv *SSHView) showExec()     { sv.switchView(viewExec) }
