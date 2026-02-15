package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (pv *PostgresView) newLogsView() *ui.CoreView {
	cores := ui.NewCoreView(pv.app, "PostgreSQL Activity Log")
	cores.SetTableHeaders([]string{"PID", "User", "Database", "State", "Duration", "Started At", "Query"})
	cores.SetRefreshCallback(pv.refreshLogs)
	cores.AddKeyBinding("R", "Refresh", pv.refresh)
	cores.AddKeyBinding("?", "Help", pv.showHelp)
	cores.AddKeyBinding("E", "Full Query", pv.showFullQuery)
	pv.addCommonBindings(cores)
	cores.SetActionCallback(pv.handleAction)

	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		pv.showFullQuery()
	})

	cores.RegisterHandlers()
	return cores
}

func (pv *PostgresView) refreshLogs() ([][]string, error) {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		return [][]string{{"Status", "", "Not Connected", "", "", "", ""}}, nil
	}

	entries, err := pv.pgClient.GetActivityLog()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(entries))
	for _, e := range entries {
		query := e.Query
		if len(query) > 80 {
			query = query[:80] + "..."
		}
		data = append(data, []string{
			fmt.Sprintf("%d", e.PID),
			e.User,
			e.Database,
			e.State,
			e.Duration,
			e.StartedAt,
			query,
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "-", "-", "-", "-", "-", "No activity"})
	}

	return data, nil
}

func (pv *PostgresView) showFullQuery() {
	row := pv.logsView.GetSelectedRowData()
	if len(row) < 7 {
		pv.logsView.Log("[yellow]No entry selected")
		return
	}

	pid := row[0]
	user := row[1]
	db := row[2]
	state := row[3]
	query := row[6]

	content := fmt.Sprintf("[yellow]PID:[white] %s\n[yellow]User:[white] %s\n[yellow]Database:[white] %s\n[yellow]State:[white] %s\n\n[yellow]Query:[white]\n%s",
		pid, user, db, state, query)

	ui.ShowInfoModal(
		pv.pages, pv.app, "Query Detail", content,
		func() {
			pv.app.SetFocus(pv.logsView.GetTable())
		},
	)
}
