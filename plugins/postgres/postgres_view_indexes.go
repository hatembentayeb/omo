package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (pv *PostgresView) newIndexesView() *ui.CoreView {
	cores := ui.NewCoreView(pv.app, "PostgreSQL Indexes")
	cores.SetTableHeaders([]string{"Schema", "Table", "Index", "Size", "Scans", "Tuples Read", "Tuples Fetched"})
	cores.SetRefreshCallback(pv.refreshIndexes)
	cores.AddKeyBinding("R", "Refresh", pv.refresh)
	cores.AddKeyBinding("?", "Help", pv.showHelp)
	cores.AddKeyBinding("E", "Index Def", pv.showIndexDefinition)
	pv.addCommonBindings(cores)
	cores.SetActionCallback(pv.handleAction)

	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		pv.showIndexDefinition()
	})

	cores.RegisterHandlers()
	return cores
}

func (pv *PostgresView) refreshIndexes() ([][]string, error) {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		return [][]string{{"Status", "Not Connected", "", "", "", "", ""}}, nil
	}

	indexes, err := pv.pgClient.GetIndexes()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(indexes))
	for _, idx := range indexes {
		data = append(data, []string{
			idx.Schema,
			idx.Table,
			idx.Name,
			idx.Size,
			fmt.Sprintf("%d", idx.Scans),
			fmt.Sprintf("%d", idx.TupRead),
			fmt.Sprintf("%d", idx.TupFetch),
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "-", "-", "-", "-", "-", "No indexes"})
	}

	return data, nil
}

func (pv *PostgresView) showIndexDefinition() {
	row := pv.indexesView.GetSelectedRowData()
	if len(row) < 3 {
		pv.indexesView.Log("[yellow]No index selected")
		return
	}

	indexes, err := pv.pgClient.GetIndexes()
	if err != nil {
		pv.indexesView.Log(fmt.Sprintf("[red]Failed to get index definition: %v", err))
		return
	}

	indexName := row[2]
	var indexDef string
	for _, idx := range indexes {
		if idx.Name == indexName {
			indexDef = idx.IndexDef
			break
		}
	}

	content := fmt.Sprintf("[yellow]Index:[white] %s\n[yellow]Table:[white] %s.%s\n\n[yellow]Definition:[white]\n%s",
		row[2], row[0], row[1], indexDef)

	ui.ShowInfoModal(
		pv.pages, pv.app, "Index Definition", content,
		func() {
			pv.app.SetFocus(pv.indexesView.GetTable())
		},
	)
}
