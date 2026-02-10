package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (pv *PostgresView) newTablesView() *ui.CoreView {
	cores := ui.NewCoreView(pv.app, "PostgreSQL Tables")
	cores.SetTableHeaders([]string{"Schema", "Table", "Owner", "Rows", "Size", "Total Size", "Indexes", "Tablespace"})
	cores.SetRefreshCallback(pv.refreshTables)
	cores.AddKeyBinding("R", "Refresh", pv.refresh)
	cores.AddKeyBinding("?", "Help", pv.showHelp)
	cores.AddKeyBinding("E", "Columns", pv.showTableColumns)
	pv.addCommonBindings(cores)
	cores.SetActionCallback(pv.handleAction)

	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		pv.showTableColumns()
	})

	cores.RegisterHandlers()
	return cores
}

func (pv *PostgresView) refreshTables() ([][]string, error) {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		return [][]string{{"Status", "Not Connected", "", "", "", "", "", ""}}, nil
	}

	tables, err := pv.pgClient.GetTables()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(tables))
	for _, t := range tables {
		hasIdx := "No"
		if t.HasIndexes {
			hasIdx = "Yes"
		}
		data = append(data, []string{
			t.Schema,
			t.Name,
			t.Owner,
			fmt.Sprintf("%d", t.RowCount),
			t.Size,
			t.TotalSize,
			hasIdx,
			t.Tablespace,
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "-", "-", "-", "-", "-", "-", "No tables"})
	}

	return data, nil
}

func (pv *PostgresView) showTableColumns() {
	row := pv.tablesView.GetSelectedRowData()
	if len(row) < 2 {
		pv.tablesView.Log("[yellow]No table selected")
		return
	}
	schema := row[0]
	table := row[1]

	columns, err := pv.pgClient.GetTableColumns(schema, table)
	if err != nil {
		pv.tablesView.Log(fmt.Sprintf("[red]Failed to get columns: %v", err))
		return
	}

	var content string
	content += fmt.Sprintf("[yellow]Table: %s.%s[white]\n\n", schema, table)
	content += fmt.Sprintf("%-25s %-20s %-10s %-25s %s\n", "Column", "Type", "Nullable", "Default", "Max Length")
	content += fmt.Sprintf("%-25s %-20s %-10s %-25s %s\n", "------", "----", "--------", "-------", "----------")
	for _, col := range columns {
		content += fmt.Sprintf("%-25s %-20s %-10s %-25s %s\n", col[0], col[1], col[2], col[3], col[4])
	}

	ui.ShowInfoModal(
		pv.pages, pv.app,
		fmt.Sprintf("Columns: %s.%s", schema, table),
		content,
		func() {
			pv.app.SetFocus(pv.tablesView.GetTable())
		},
	)
}
