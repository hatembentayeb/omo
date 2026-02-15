package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (pv *PostgresView) newDatabasesView() *ui.CoreView {
	cores := ui.NewCoreView(pv.app, "PostgreSQL Databases")
	cores.SetTableHeaders([]string{"Name", "Owner", "Encoding", "Collation", "Size", "Tablespace", "Conn Limit"})
	cores.SetRefreshCallback(pv.refreshDatabases)
	cores.AddKeyBinding("R", "Refresh", pv.refresh)
	cores.AddKeyBinding("?", "Help", pv.showHelp)
	cores.AddKeyBinding("N", "New DB", pv.showCreateDatabaseForm)
	cores.AddKeyBinding("D", "Drop DB", pv.showDropDatabaseConfirmation)
	pv.addCommonBindings(cores)
	cores.SetActionCallback(pv.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (pv *PostgresView) refreshDatabases() ([][]string, error) {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		return [][]string{{"Status", "Not Connected", "", "", "", "", ""}}, nil
	}

	databases, err := pv.pgClient.GetDatabases()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(databases))
	for _, d := range databases {
		data = append(data, []string{
			d.Name,
			d.Owner,
			d.Encoding,
			d.Collation,
			d.Size,
			d.Tablespace,
			fmt.Sprintf("%d", d.ConnLimit),
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "-", "-", "-", "-", "-", "No databases"})
	}

	return data, nil
}

func (pv *PostgresView) showCreateDatabaseForm() {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		pv.currentCores().Log("[yellow]Not connected")
		return
	}

	ui.ShowCompactStyledInputModal(
		pv.pages, pv.app, "Create Database", "Database Name", "", 25, nil,
		func(name string, cancelled bool) {
			if cancelled || name == "" {
				pv.app.SetFocus(pv.currentCores().GetTable())
				return
			}
			ui.ShowCompactStyledInputModal(
				pv.pages, pv.app, "Create Database", "Owner (empty=current)", "", 25, nil,
				func(owner string, cancelled bool) {
					if cancelled {
						pv.app.SetFocus(pv.currentCores().GetTable())
						return
					}
					err := pv.pgClient.CreateDatabase(name, owner, "UTF8")
					if err != nil {
						pv.currentCores().Log(fmt.Sprintf("[red]Failed to create database: %v", err))
					} else {
						pv.currentCores().Log(fmt.Sprintf("[green]Created database: %s", name))
						pv.refresh()
					}
					pv.app.SetFocus(pv.currentCores().GetTable())
				},
			)
		},
	)
}

func (pv *PostgresView) showDropDatabaseConfirmation() {
	row := pv.databasesView.GetSelectedRowData()
	if len(row) == 0 {
		pv.currentCores().Log("[yellow]No database selected")
		return
	}
	dbName := row[0]

	ui.ShowStandardConfirmationModal(
		pv.pages, pv.app, "Drop Database",
		fmt.Sprintf("Are you sure you want to drop database '[red]%s[white]'?\nThis action is [red]IRREVERSIBLE[white]!", dbName),
		func(confirmed bool) {
			if confirmed {
				if err := pv.pgClient.DropDatabase(dbName); err != nil {
					pv.currentCores().Log(fmt.Sprintf("[red]Failed to drop database: %v", err))
				} else {
					pv.currentCores().Log(fmt.Sprintf("[yellow]Dropped database: %s", dbName))
					pv.refresh()
				}
			}
			pv.app.SetFocus(pv.currentCores().GetTable())
		},
	)
}
