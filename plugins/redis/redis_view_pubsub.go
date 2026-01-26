package main

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"omo/pkg/ui"
)

// pubsubSubscription holds the active subscription state
var (
	activePubSubSubscription *PubSubSubscription
	pubsubMutex              sync.Mutex
)

func (rv *RedisView) newPubSubView() *ui.Cores {
	cores := ui.NewCores(rv.app, "Redis PubSub")
	cores.SetTableHeaders([]string{"Channel", "Subscribers", "Type"})
	cores.SetRefreshCallback(rv.refreshPubSub)
	cores.AddKeyBinding("K", "Keys", rv.showKeys)
	cores.AddKeyBinding("I", "Server Info", rv.showServerInfo)
	cores.AddKeyBinding("L", "Slowlog", rv.showSlowlog)
	cores.AddKeyBinding("T", "Stats", rv.showStats)
	cores.AddKeyBinding("C", "Clients", rv.showClients)
	cores.AddKeyBinding("G", "Config", rv.showConfig)
	cores.AddKeyBinding("M", "Memory", rv.showMemory)
	cores.AddKeyBinding("P", "Persistence", rv.showPersistence)
	cores.AddKeyBinding("Y", "Replication", rv.showReplication)
	cores.AddKeyBinding("B", "PubSub", rv.showPubSub)
	cores.AddKeyBinding("A", "Key Analysis", rv.showKeyAnalysis)
	cores.AddKeyBinding("W", "Databases", rv.showDatabases)
	cores.AddKeyBinding("X", "Cmd Stats", rv.showCommandStats)
	cores.AddKeyBinding("Z", "Latency", rv.showLatency)
	cores.AddKeyBinding("Enter", "Subscribe", nil) // Handled via SetSelectedFunc
	cores.AddKeyBinding("U", "Publish", rv.showPublishModal)

	// Handle Enter key to subscribe to selected channel
	cores.GetTable().SetSelectedFunc(func(row, column int) {
		if row <= 0 {
			return
		}
		rv.subscribeToSelectedChannel()
	})

	cores.SetActionCallback(rv.handleAction)
	cores.RegisterHandlers()
	return cores
}

func (rv *RedisView) refreshPubSub() ([][]string, error) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		return [][]string{{"Status", "Not Connected", "-"}}, nil
	}

	channels, err := rv.redisClient.GetPubSubChannels()
	if err != nil {
		return [][]string{}, err
	}

	data := make([][]string, 0, len(channels))
	for _, ch := range channels {
		channelType := "Channel"
		if ch.Pattern {
			channelType = "Pattern"
		}

		data = append(data, []string{
			ch.Channel,
			fmt.Sprintf("%d", ch.Subscribers),
			channelType,
		})
	}

	if len(data) == 0 {
		data = append(data, []string{"-", "0", "No active channels"})
	}

	return data, nil
}

// subscribeToSelectedChannel subscribes to the selected channel and shows messages
func (rv *RedisView) subscribeToSelectedChannel() {
	row := rv.pubsubView.GetSelectedRowData()
	if len(row) == 0 {
		rv.pubsubView.Log("[red]No channel selected")
		return
	}

	channel := row[0]
	if channel == "-" || channel == "*" {
		rv.pubsubView.Log("[yellow]Cannot subscribe to this entry")
		return
	}

	rv.showPubSubMessageModal(channel)
}

// showPubSubMessageModal shows a modal that displays messages from a channel
func (rv *RedisView) showPubSubMessageModal(channel string) {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		rv.pubsubView.Log("[red]Not connected to Redis")
		return
	}

	// Close any existing subscription
	pubsubMutex.Lock()
	if activePubSubSubscription != nil {
		activePubSubSubscription.Close()
		activePubSubSubscription = nil
	}
	pubsubMutex.Unlock()

	// Subscribe to the channel
	sub, err := rv.redisClient.SubscribeToChannel(channel)
	if err != nil {
		rv.pubsubView.Log(fmt.Sprintf("[red]Failed to subscribe: %v", err))
		return
	}

	pubsubMutex.Lock()
	activePubSubSubscription = sub
	pubsubMutex.Unlock()

	// Create the message view
	textView := tview.NewTextView()
	textView.SetDynamicColors(true)
	textView.SetScrollable(true)
	textView.SetWordWrap(true)
	textView.SetBorder(true)
	textView.SetBorderColor(tcell.ColorAqua)
	textView.SetTitle(fmt.Sprintf(" Messages: %s ", channel))
	textView.SetTitleColor(tcell.ColorYellow)
	textView.SetTitleAlign(tview.AlignCenter)
	textView.SetBorderPadding(0, 0, 1, 1)
	textView.SetBackgroundColor(tcell.ColorDefault)
	textView.SetChangedFunc(func() {
		rv.app.Draw()
	})

	// Initial text
	textView.SetText("[gray]Waiting for messages... (Press ESC to close)[white]\n\n")

	// Message buffer
	var messages []string
	const maxMessages = 100

	// Start goroutine to receive messages
	done := make(chan struct{})
	go func() {
		for {
			select {
			case msg, ok := <-sub.Messages:
				if !ok {
					return
				}
				timestamp := msg.Timestamp.Format("15:04:05")
				msgLine := fmt.Sprintf("[aqua]%s[white] [yellow]%s[white]\n%s\n",
					timestamp, msg.Channel, msg.Payload)

				messages = append(messages, msgLine)
				if len(messages) > maxMessages {
					messages = messages[1:]
				}

				rv.app.QueueUpdateDraw(func() {
					var sb strings.Builder
					sb.WriteString("[gray]Live messages (ESC to close, newest at bottom)[white]\n")
					sb.WriteString(strings.Repeat("─", 60) + "\n")
					for _, m := range messages {
						sb.WriteString(m)
						sb.WriteString("\n")
					}
					textView.SetText(sb.String())
					textView.ScrollToEnd()
				})
			case <-done:
				return
			}
		}
	}()

	// Create layout
	width := 80
	height := 24

	innerFlex := tview.NewFlex()
	innerFlex.SetDirection(tview.FlexRow)
	innerFlex.SetBackgroundColor(tcell.ColorDefault)
	innerFlex.AddItem(nil, 0, 1, false).
		AddItem(textView, height, 1, true).
		AddItem(nil, 0, 1, false)

	flex := tview.NewFlex()
	flex.SetBackgroundColor(tcell.ColorDefault)
	flex.AddItem(nil, 0, 1, false).
		AddItem(innerFlex, width, 1, true).
		AddItem(nil, 0, 1, false)

	const pageID = "pubsub-messages"

	// Custom input capture to handle ESC
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			close(done)
			pubsubMutex.Lock()
			if activePubSubSubscription != nil {
				activePubSubSubscription.Close()
				activePubSubSubscription = nil
			}
			pubsubMutex.Unlock()
			rv.pages.RemovePage(pageID)
			rv.app.SetFocus(rv.pubsubView.GetTable())
			return nil
		}
		return event
	})

	rv.pages.AddPage(pageID, flex, true, true)
	rv.app.SetFocus(textView)
	rv.pubsubView.Log(fmt.Sprintf("[green]Subscribed to: %s", channel))
}

// showPublishModal shows a modal to publish a message to a channel
func (rv *RedisView) showPublishModal() {
	if rv.redisClient == nil || !rv.redisClient.IsConnected() {
		rv.pubsubView.Log("[red]Not connected to Redis")
		return
	}

	// Get selected channel as default
	defaultChannel := ""
	row := rv.pubsubView.GetSelectedRowData()
	if len(row) > 0 && row[0] != "-" && row[0] != "*" {
		defaultChannel = row[0]
	}

	// Create form
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetBorderColor(tcell.ColorAqua)
	form.SetTitle(" Publish Message ")
	form.SetTitleColor(tcell.ColorYellow)
	form.SetTitleAlign(tview.AlignCenter)
	form.SetBackgroundColor(tcell.ColorDefault)
	form.SetFieldBackgroundColor(tcell.ColorDarkSlateGray)
	form.SetButtonBackgroundColor(tcell.ColorDarkCyan)

	var channelInput, messageInput string
	channelInput = defaultChannel

	form.AddInputField("Channel", defaultChannel, 40, nil, func(text string) {
		channelInput = text
	})
	form.AddInputField("Message", "", 40, nil, func(text string) {
		messageInput = text
	})

	const pageID = "pubsub-publish"

	form.AddButton("Publish", func() {
		if channelInput == "" {
			rv.pubsubView.Log("[red]Channel name required")
			return
		}
		if messageInput == "" {
			rv.pubsubView.Log("[red]Message required")
			return
		}

		err := rv.redisClient.PublishMessage(channelInput, messageInput)
		if err != nil {
			rv.pubsubView.Log(fmt.Sprintf("[red]Publish failed: %v", err))
		} else {
			rv.pubsubView.Log(fmt.Sprintf("[green]Published to %s", channelInput))
		}
		rv.pages.RemovePage(pageID)
		rv.app.SetFocus(rv.pubsubView.GetTable())
	})

	form.AddButton("Cancel", func() {
		rv.pages.RemovePage(pageID)
		rv.app.SetFocus(rv.pubsubView.GetTable())
	})

	// Handle ESC
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			rv.pages.RemovePage(pageID)
			rv.app.SetFocus(rv.pubsubView.GetTable())
			return nil
		}
		return event
	})

	// Layout
	width := 60
	height := 12

	innerFlex := tview.NewFlex()
	innerFlex.SetDirection(tview.FlexRow)
	innerFlex.SetBackgroundColor(tcell.ColorDefault)
	innerFlex.AddItem(nil, 0, 1, false).
		AddItem(form, height, 1, true).
		AddItem(nil, 0, 1, false)

	flex := tview.NewFlex()
	flex.SetBackgroundColor(tcell.ColorDefault)
	flex.AddItem(nil, 0, 1, false).
		AddItem(innerFlex, width, 1, true).
		AddItem(nil, 0, 1, false)

	rv.pages.AddPage(pageID, flex, true, true)
	rv.app.SetFocus(form)
}
