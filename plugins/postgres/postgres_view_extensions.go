package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (pv *PostgresView) newExtensionsView() *ui.CoreView {
	cores := ui.NewCoreView(pv.app, "PostgreSQL Extensions")
	cores.SetTableHeaders([]string{"Name", "Version", "Schema", "Installed", "Comment"})
	cores.SetRefreshCallback(pv.refreshExtensions)
	cores.AddKeyBinding("R", "Refresh", pv.refresh)
	cores.AddKeyBinding("?", "Help", pv.showHelp)
	cores.AddKeyBinding("N", "Install Ext", pv.showInstallExtensionForm)
	cores.AddKeyBinding("D", "Drop Ext", pv.showDropExtensionConfirmation)
	pv.addCommonBindings(cores)
	cores.SetActionCallback(pv.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (pv *PostgresView) refreshExtensions() ([][]string, error) {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		return [][]string{{"Status", "", "", "Not Connected", ""}}, nil
	}

	extensions, err := pv.pgClient.GetExtensions()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(extensions))
	for _, ext := range extensions {
		installed := "[red]No"
		if ext.Installed {
			installed = "[green]Yes"
		}
		comment := ext.Comment
		if len(comment) > 80 {
			comment = comment[:80] + "..."
		}
		data = append(data, []string{
			ext.Name,
			ext.Version,
			ext.Schema,
			installed,
			comment,
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "-", "-", "-", "No extensions"})
	}

	return data, nil
}

func (pv *PostgresView) showInstallExtensionForm() {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		pv.currentCores().Log("[yellow]Not connected")
		return
	}

	// If an extension is selected, use it as default
	defaultName := ""
	row := pv.extensionsView.GetSelectedRowData()
	if len(row) > 0 {
		defaultName = row[0]
	}

	ui.ShowCompactStyledInputModal(
		pv.pages, pv.app, "Install Extension", "Extension Name", defaultName, 25, nil,
		func(name string, cancelled bool) {
			if cancelled || name == "" {
				pv.app.SetFocus(pv.currentCores().GetTable())
				return
			}
			err := pv.pgClient.CreateExtension(name)
			if err != nil {
				pv.currentCores().Log(fmt.Sprintf("[red]Failed to install extension: %v", err))
			} else {
				pv.currentCores().Log(fmt.Sprintf("[green]Installed extension: %s", name))
				pv.refresh()
			}
			pv.app.SetFocus(pv.currentCores().GetTable())
		},
	)
}

func (pv *PostgresView) showDropExtensionConfirmation() {
	row := pv.extensionsView.GetSelectedRowData()
	if len(row) == 0 {
		pv.currentCores().Log("[yellow]No extension selected")
		return
	}
	extName := row[0]

	ui.ShowStandardConfirmationModal(
		pv.pages, pv.app, "Drop Extension",
		fmt.Sprintf("Are you sure you want to drop extension '[red]%s[white]'?", extName),
		func(confirmed bool) {
			if confirmed {
				if err := pv.pgClient.DropExtension(extName); err != nil {
					pv.currentCores().Log(fmt.Sprintf("[red]Failed to drop extension: %v", err))
				} else {
					pv.currentCores().Log(fmt.Sprintf("[yellow]Dropped extension: %s", extName))
					pv.refresh()
				}
			}
			pv.app.SetFocus(pv.currentCores().GetTable())
		},
	)
}
