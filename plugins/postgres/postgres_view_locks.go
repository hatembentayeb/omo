package main

import (
	"fmt"

	"omo/pkg/ui"
)

func (pv *PostgresView) newLocksView() *ui.CoreView {
	cores := ui.NewCoreView(pv.app, "PostgreSQL Locks")
	cores.SetTableHeaders([]string{"PID", "Lock Type", "Mode", "Relation", "Granted", "Wait Start"})
	cores.SetRefreshCallback(pv.refreshLocks)
	cores.AddKeyBinding("R", "Refresh", pv.refresh)
	cores.AddKeyBinding("?", "Help", pv.showHelp)
	pv.addCommonBindings(cores)
	cores.SetActionCallback(pv.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (pv *PostgresView) refreshLocks() ([][]string, error) {
	if pv.pgClient == nil || !pv.pgClient.IsConnected() {
		return [][]string{{"Status", "Not Connected", "", "", "", ""}}, nil
	}

	locks, err := pv.pgClient.GetLocks()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(locks))
	for _, lk := range locks {
		granted := "[green]Yes"
		if !lk.Granted {
			granted = "[red]No (Waiting)"
		}
		data = append(data, []string{
			fmt.Sprintf("%d", lk.PID),
			lk.LockType,
			lk.Mode,
			lk.Relation,
			granted,
			lk.WaitStart,
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "-", "-", "-", "-", "No locks"})
	}

	return data, nil
}
