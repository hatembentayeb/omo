package main

import (
	"fmt"
	"strconv"

	"github.com/rivo/tview"

	"omo/ui"
)

// PartitionInfo represents information about a Kafka partition
type PartitionInfo struct {
	ID           int
	Topic        string
	Leader       int
	Replicas     []int
	ISRs         []int
	Size         int64
	MessageCount int64
	OffsetLag    int64
	Status       string
}

// PartitionsView manages the UI for viewing Kafka partitions
type PartitionsView struct {
	app            *tview.Application
	pages          *tview.Pages
	cores          *ui.Cores
	kafkaClient    *KafkaClient
	currentCluster string
	currentTopic   string
	partitions     []PartitionInfo
}

// NewPartitionsView creates a new partitions view
func NewPartitionsView(app *tview.Application, pages *tview.Pages, kafkaClient *KafkaClient, cluster string, topic string) *PartitionsView {
	pv := &PartitionsView{
		app:            app,
		pages:          pages,
		kafkaClient:    kafkaClient,
		currentCluster: cluster,
		currentTopic:   topic,
		partitions:     []PartitionInfo{},
	}

	// Create Cores UI component
	pv.cores = ui.NewCores(app, "")

	// Set table headers
	pv.cores.SetTableHeaders([]string{"ID", "Leader", "Replicas", "ISR", "Size", "First Offset", "Last Offset", "Messages"})

	// Set up refresh callback to make 'R' key work properly
	pv.cores.SetRefreshCallback(func() ([][]string, error) {
		return pv.refreshPartitions()
	})

	// Set action callback to handle keypresses
	pv.cores.SetActionCallback(func(action string, payload map[string]interface{}) error {
		if action == "keypress" {
			if key, ok := payload["key"].(string); ok {
				switch key {
				case "R":
					pv.refresh()
					return nil
				case "?":
					pv.showHelp()
					return nil
				case "I":
					pv.showPartitionInfo()
					return nil
				case "C":
					pv.showConsumers()
					return nil
				case "B":
					pv.returnToTopics()
					return nil
				}
			}
		}
		return nil
	})

	// Add key bindings
	pv.cores.AddKeyBinding("R", "Refresh", nil)
	pv.cores.AddKeyBinding("?", "Help", nil)
	pv.cores.AddKeyBinding("I", "Info", nil)
	pv.cores.AddKeyBinding("C", "Consumers", nil)
	pv.cores.AddKeyBinding("B", "Back", nil)

	// Set row selection callback for tracking selection
	pv.cores.SetRowSelectedCallback(func(row int) {
		if row >= 0 && row < len(pv.partitions) {
			pv.cores.Log(fmt.Sprintf("[blue]Selected partition: %d (Leader: %d)",
				pv.partitions[row].ID, pv.partitions[row].Leader))
		}
	})

	// Register the key handlers to actually handle the key events
	pv.cores.RegisterHandlers()

	// Initial refresh to show data
	pv.refresh()

	return pv
}

// GetMainUI returns the main UI component
func (pv *PartitionsView) GetMainUI() tview.Primitive {
	// Ensure table gets focus when this view is shown
	pv.app.SetFocus(pv.cores.GetTable())
	return pv.cores.GetLayout()
}

// refreshPartitions refreshes the partitions list
func (pv *PartitionsView) refreshPartitions() ([][]string, error) {
	if pv.kafkaClient == nil || pv.currentCluster == "" || pv.currentTopic == "" {
		// No client, cluster or topic, show empty data
		pv.partitions = []PartitionInfo{}
		pv.cores.SetTableData([][]string{})
		pv.cores.SetInfoText("[yellow]Kafka Partitions[white]\nCluster: Not Connected\nTopic: Not Selected")
		return [][]string{}, nil
	}

	// In a real implementation, we would fetch actual partition data
	// For now, let's simulate some sample data
	pv.partitions = []PartitionInfo{
		{ID: 0, Topic: pv.currentTopic, Leader: 1, Replicas: []int{1, 2, 3}, ISRs: []int{1, 2, 3}, Size: 345 * 1024 * 1024, MessageCount: 3245156, OffsetLag: 0, Status: "Online"},
		{ID: 1, Topic: pv.currentTopic, Leader: 2, Replicas: []int{2, 3, 1}, ISRs: []int{2, 3, 1}, Size: 389 * 1024 * 1024, MessageCount: 3687423, OffsetLag: 0, Status: "Online"},
		{ID: 2, Topic: pv.currentTopic, Leader: 3, Replicas: []int{3, 1, 2}, ISRs: []int{3, 1, 2}, Size: 412 * 1024 * 1024, MessageCount: 3891045, OffsetLag: 0, Status: "Online"},
		{ID: 3, Topic: pv.currentTopic, Leader: 1, Replicas: []int{1, 3, 2}, ISRs: []int{1, 3, 2}, Size: 378 * 1024 * 1024, MessageCount: 3541891, OffsetLag: 0, Status: "Online"},
		{ID: 4, Topic: pv.currentTopic, Leader: 2, Replicas: []int{2, 1, 3}, ISRs: []int{2, 1, 3}, Size: 402 * 1024 * 1024, MessageCount: 3798234, OffsetLag: 0, Status: "Online"},
		{ID: 5, Topic: pv.currentTopic, Leader: 3, Replicas: []int{3, 2, 1}, ISRs: []int{3, 2, 1}, Size: 356 * 1024 * 1024, MessageCount: 3321567, OffsetLag: 0, Status: "Online"},
	}

	// Convert to table data
	tableData := make([][]string, len(pv.partitions))
	for i, partition := range pv.partitions {
		// Format replicas as a comma-separated list
		replicas := ""
		for j, r := range partition.Replicas {
			if j > 0 {
				replicas += ", "
			}
			replicas += strconv.Itoa(r)
		}

		// Format ISRs as a comma-separated list
		isrs := ""
		for j, isr := range partition.ISRs {
			if j > 0 {
				isrs += ", "
			}
			isrs += strconv.Itoa(isr)
		}

		tableData[i] = []string{
			strconv.Itoa(partition.ID),
			strconv.Itoa(partition.Leader),
			replicas,
			isrs,
			formatSize(partition.Size),
			formatCount(partition.MessageCount),
			partition.Status,
		}
	}

	// Update table data
	pv.cores.SetTableData(tableData)

	// Update info text
	pv.cores.SetInfoText(fmt.Sprintf("[yellow]Kafka Partitions[white]\nCluster: %s\nTopic: %s\nPartitions: %d",
		pv.currentCluster, pv.currentTopic, len(pv.partitions)))

	return tableData, nil
}

// refresh manually refreshes the partitions list
func (pv *PartitionsView) refresh() {
	pv.cores.RefreshData()
}

// showHelp shows the help modal
func (pv *PartitionsView) showHelp() {
	helpText := `[yellow]Kafka Partitions View Help[white]

[aqua]Key Bindings:[white]
[green]R[white] - Refresh partitions list
[green]I[white] - Show detailed information about the selected partition
[green]C[white] - Show consumers for the selected partition
[green]B[white] - Return to topics view
[green]?[white] - Show this help information
[green]ESC[white] - Close modal dialogs

[aqua]Usage Tips:[white]
- Select a partition by clicking on it or using arrow keys
- Use the refresh button to update the partitions list
- You can sort the list by clicking on column headers
`

	ui.ShowInfoModal(
		pv.pages,
		pv.app,
		"Help",
		helpText,
		func() {
			// Ensure table regains focus after modal is closed
			pv.app.SetFocus(pv.cores.GetTable())
		},
	)
}

// showPartitionInfo shows detailed information about the selected partition
func (pv *PartitionsView) showPartitionInfo() {
	selectedRow := pv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(pv.partitions) {
		pv.cores.Log("[red]No partition selected")
		return
	}

	partition := pv.partitions[selectedRow]

	// Format replicas as a comma-separated list
	replicas := ""
	for i, r := range partition.Replicas {
		if i > 0 {
			replicas += ", "
		}
		replicas += strconv.Itoa(r)
	}

	// Format ISRs as a comma-separated list
	isrs := ""
	for i, isr := range partition.ISRs {
		if i > 0 {
			isrs += ", "
		}
		isrs += strconv.Itoa(isr)
	}

	// In a real implementation, we'd get more detailed information about the partition
	infoText := fmt.Sprintf(`[yellow]Partition Details[white]

[aqua]Partition ID:[white] %d
[aqua]Topic:[white] %s
[aqua]Status:[white] %s
[aqua]Leader Broker:[white] %d
[aqua]Replicas:[white] %s
[aqua]In-Sync Replicas:[white] %s
[aqua]Size:[white] %s
[aqua]Message Count:[white] %s
[aqua]First Offset:[white] 0
[aqua]Last Offset:[white] %d
[aqua]Leader Epoch:[white] 5
[aqua]Preferred Leader:[white] %t
[aqua]Under-Replicated:[white] %t
`,
		partition.ID, partition.Topic, partition.Status,
		partition.Leader, replicas, isrs,
		formatSize(partition.Size), formatCount(partition.MessageCount),
		partition.MessageCount,
		partition.Leader == partition.Replicas[0],     // Is preferred leader
		len(partition.ISRs) < len(partition.Replicas)) // Is under-replicated

	ui.ShowInfoModal(
		pv.pages,
		pv.app,
		fmt.Sprintf("Partition %d Information", partition.ID),
		infoText,
		func() {
			// Ensure table regains focus after modal is closed
			pv.app.SetFocus(pv.cores.GetTable())
		},
	)
}

// showConsumers shows consumers for the selected partition
func (pv *PartitionsView) showConsumers() {
	selectedRow := pv.cores.GetSelectedRow()
	if selectedRow < 0 || selectedRow >= len(pv.partitions) {
		pv.cores.Log("[red]No partition selected")
		return
	}

	partition := pv.partitions[selectedRow]

	// Create a consumers view for this partition
	consumersView := NewConsumersView(pv.app, pv.pages, pv.kafkaClient, pv.currentCluster, pv.currentTopic)

	// Copy the current navigation stack and push new view
	consumersView.cores.CopyNavigationStackFrom(pv.cores)
	consumersView.cores.PushView("consumers")

	// Add the consumers view as a new page
	pv.pages.AddPage("consumers-view", consumersView.GetMainUI(), true, true)

	// Switch to the consumers view
	pv.pages.SwitchToPage("consumers-view")

	pv.cores.Log(fmt.Sprintf("[blue]Showing consumers for partition %d", partition.ID))
}

// returnToTopics switches back to the topics view
func (pv *PartitionsView) returnToTopics() {
	pv.cores.Log("[blue]Returning to topics view")
	pv.cores.PopView() // Remove current view from stack
	pv.pages.SwitchToPage("topics")
}

// formatSize formats a byte size in a human-readable format
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	var size float64
	var unit string

	switch {
	case bytes >= TB:
		size = float64(bytes) / TB
		unit = "TB"
	case bytes >= GB:
		size = float64(bytes) / GB
		unit = "GB"
	case bytes >= MB:
		size = float64(bytes) / MB
		unit = "MB"
	case bytes >= KB:
		size = float64(bytes) / KB
		unit = "KB"
	default:
		size = float64(bytes)
		unit = "bytes"
	}

	return fmt.Sprintf("%.1f %s", size, unit)
}

// formatCount formats a large number in a human-readable format
func formatCount(count int64) string {
	if count >= 1000000 {
		return fmt.Sprintf("%.2f M", float64(count)/1000000)
	} else if count >= 1000 {
		return fmt.Sprintf("%.1f K", float64(count)/1000)
	}
	return fmt.Sprintf("%d", count)
}
