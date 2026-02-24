package main

import (
	"time"

	"omo/pkg/pluginapi"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type SSHPlugin struct {
	Name        string
	Description string
	sshView     *SSHView
}

func (p *SSHPlugin) Start(app *tview.Application) tview.Primitive {
	pluginapi.Log().Info("starting plugin")
	pages := tview.NewPages()
	sshView := NewSSHView(app, pages)
	p.sshView = sshView

	mainUI := sshView.GetMainUI()

	pages.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlT {
			if p.sshView != nil {
				p.sshView.ShowConnectionSelector()
			}
			return nil
		}
		return event
	})

	pages.AddPage("ssh", mainUI, true, true)
	app.SetFocus(p.sshView.serversView.GetTable())

	go func() {
		p.sshView.AutoConnectToDefaultInstance()
	}()

	return pages
}

func (p *SSHPlugin) Stop() {
	if p.sshView != nil {
		p.sshView.Stop()
	}
}

func (p *SSHPlugin) GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "ssh",
		Version:     "1.0.0",
		Description: "SSH server management and remote execution",
		Author:      "ohmyops",
		License:     "MIT",
		Tags:        []string{"ssh", "remote", "server", "devops"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/ssh",
	}
}

var OhmyopsPlugin SSHPlugin

func init() {
	OhmyopsPlugin.Name = "SSH Manager"
	OhmyopsPlugin.Description = "Manage SSH connections and remote servers"
}

func GetMetadata() pluginapi.PluginMetadata {
	return pluginapi.PluginMetadata{
		Name:        "ssh",
		Version:     "1.0.0",
		Description: "SSH server management and remote execution",
		Author:      "ohmyops",
		License:     "MIT",
		Tags:        []string{"ssh", "remote", "server", "devops"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/ssh",
	}
}
