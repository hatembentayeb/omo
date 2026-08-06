package kafka

import (
	"fmt"
	"strconv"
	"strings"

	"omo/pkg/pluginrpc"
)

func navBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "B", Label: "Brokers", Action: "goto_brokers"},
		{Key: "T", Label: "Topics", Action: "goto_topics"},
		{Key: "G", Label: "Consumers", Action: "goto_consumers"},
		{Key: "P", Label: "Partitions", Action: "goto_partitions"},
		{Key: "M", Label: "Messages", Action: "goto_messages"},
	}
}

func withNav(extra ...pluginrpc.KeyBinding) []pluginrpc.KeyBinding {
	out := make([]pluginrpc.KeyBinding, 0, len(extra)+len(navBindings())+1)
	out = append(out, pluginrpc.KeyBinding{Key: "R", Label: "Refresh", Action: "refresh"})
	out = append(out, extra...)
	out = append(out, navBindings()...)
	return out
}

func (s *Service) baseInfo(extra string) string {
	name := s.name
	if name == "" {
		name = s.bootstrap
	}
	msg := fmt.Sprintf("[green]Kafka Manager[white]\nCluster: %s\nBrokers: %s\nStatus: Connected\nView: %s",
		name, s.bootstrap, s.currentView)
	if s.selectedTopic != "" {
		msg += "\nTopic: " + s.selectedTopic
	}
	if extra != "" {
		msg += "\n" + extra
	}
	return msg
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = kafkaViewBrokers
	}
	s.currentView = viewID

	if err := s.ensureConnectedLocked(); err != nil {
		return pluginrpc.ViewData{
			View:    viewID,
			Title:   "Kafka Manager",
			Info:    "[yellow]Kafka Manager[white]\nStatus: Not Connected\n" + err.Error(),
			Status:  "not connected",
			Headers: []string{"Status", "Detail"},
			Rows:    [][]string{{"error", err.Error()}},
			KeyBindings: []pluginrpc.KeyBinding{
				{Key: "R", Label: "Refresh", Action: "refresh"},
			},
		}, nil
	}

	switch viewID {
	case kafkaViewTopics:
		return s.viewTopicsLocked()
	case kafkaViewConsumers:
		return s.viewConsumersLocked()
	case kafkaViewPartitions:
		return s.viewPartitionsLocked()
	case kafkaViewMessages:
		return s.viewMessagesLocked()
	default:
		return s.viewBrokersLocked()
	}
}

func (s *Service) viewBrokersLocked() (pluginrpc.ViewData, error) {
	brokers, err := s.client.GetBrokers()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(brokers))
	for _, broker := range brokers {
		controller := ""
		if broker.Controller {
			controller = "Yes"
		}
		rows = append(rows, []string{
			strconv.FormatInt(int64(broker.ID), 10),
			broker.Address,
			controller,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"No brokers found", "", ""})
	}
	return pluginrpc.ViewData{
		View:         kafkaViewBrokers,
		Title:        "Kafka Brokers",
		Info:         s.baseInfo(fmt.Sprintf("Brokers: %d", len(brokers))),
		Status:       "connected",
		Headers:      []string{"ID", "Address", "Controller"},
		Rows:         rows,
		SelectionKey: "ID",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "I", Label: "Info", Action: "info"},
		),
	}, nil
}

func (s *Service) viewTopicsLocked() (pluginrpc.ViewData, error) {
	topics, err := s.client.GetTopics()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(topics))
	for _, topic := range topics {
		internal := ""
		if topic.Internal {
			internal = "Yes"
		}
		rows = append(rows, []string{
			topic.Name,
			strconv.FormatInt(int64(topic.Partitions), 10),
			strconv.Itoa(int(topic.ReplicationFactor)),
			internal,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"No topics found", "", "", ""})
	}
	return pluginrpc.ViewData{
		View:         kafkaViewTopics,
		Title:        "Kafka Topics",
		Info:         s.baseInfo(fmt.Sprintf("Topics: %d", len(topics))),
		Status:       "connected",
		Headers:      []string{"Name", "Partitions", "Replication", "Internal"},
		Rows:         rows,
		SelectionKey: "Name",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "I", Label: "Info", Action: "info"},
			pluginrpc.KeyBinding{Key: "P", Label: "Partitions", Action: "show_partitions"},
			pluginrpc.KeyBinding{Key: "M", Label: "Messages", Action: "show_messages"},
		),
	}, nil
}

func (s *Service) viewConsumersLocked() (pluginrpc.ViewData, error) {
	groups, err := s.client.GetConsumerGroups()
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(groups))
	for _, group := range groups {
		rows = append(rows, []string{
			group.GroupID,
			group.State,
			strconv.Itoa(group.Members),
			group.Protocol,
			group.ProtocolType,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"No consumer groups found", "", "", "", ""})
	}
	return pluginrpc.ViewData{
		View:         kafkaViewConsumers,
		Title:        "Kafka Consumers",
		Info:         s.baseInfo(fmt.Sprintf("Consumer Groups: %d", len(groups))),
		Status:       "connected",
		Headers:      []string{"Group ID", "State", "Members", "Protocol", "Protocol Type"},
		Rows:         rows,
		SelectionKey: "Group ID",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "I", Label: "Info", Action: "info"},
			pluginrpc.KeyBinding{Key: "O", Label: "Offsets", Action: "view_offsets"},
		),
	}, nil
}

func (s *Service) viewPartitionsLocked() (pluginrpc.ViewData, error) {
	if s.selectedTopic == "" {
		return pluginrpc.ViewData{
			View:         kafkaViewPartitions,
			Title:        "Kafka Partitions",
			Info:         s.baseInfo(""),
			Status:       "connected",
			Headers:      []string{"Partition", "Leader", "Replicas", "ISR", "Oldest", "Newest", "Messages"},
			Rows:         [][]string{{"No topic selected", "Press T then P on a topic", "", "", "", "", ""}},
			SelectionKey: "Partition",
			KeyBindings:  withNav(),
		}, nil
	}
	partitions, err := s.client.GetTopicPartitions(s.selectedTopic)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	rows := make([][]string, 0, len(partitions))
	for _, p := range partitions {
		messages := p.NewestOffset - p.OldestOffset
		if messages < 0 {
			messages = 0
		}
		rows = append(rows, []string{
			strconv.FormatInt(int64(p.ID), 10),
			strconv.FormatInt(int64(p.Leader), 10),
			formatInt32Slice(p.Replicas),
			formatInt32Slice(p.ISR),
			strconv.FormatInt(p.OldestOffset, 10),
			strconv.FormatInt(p.NewestOffset, 10),
			formatLargeNumber(messages),
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"No partitions found", "", "", "", "", "", ""})
	}
	return pluginrpc.ViewData{
		View:         kafkaViewPartitions,
		Title:        "Kafka Partitions",
		Info:         s.baseInfo(fmt.Sprintf("Partitions: %d", len(partitions))),
		Status:       "connected",
		Headers:      []string{"Partition", "Leader", "Replicas", "ISR", "Oldest", "Newest", "Messages"},
		Rows:         rows,
		SelectionKey: "Partition",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "I", Label: "Info", Action: "info"},
		),
	}, nil
}

func (s *Service) viewMessagesLocked() (pluginrpc.ViewData, error) {
	if s.selectedTopic == "" {
		return pluginrpc.ViewData{
			View:         kafkaViewMessages,
			Title:        "Kafka Messages",
			Info:         s.baseInfo(""),
			Status:       "connected",
			Headers:      []string{"Partition", "Offset", "Key", "Value", "Timestamp"},
			Rows:         [][]string{{"No topic selected", "Press T then M on a topic", "", "", ""}},
			SelectionKey: "Offset",
			KeyBindings:  withNav(),
		}, nil
	}
	messages, err := s.client.ConsumeMessages(s.selectedTopic, 100)
	if err != nil {
		return pluginrpc.ViewData{}, err
	}
	s.cachedMessages = messages
	rows := make([][]string, 0, len(messages))
	for _, msg := range messages {
		value := msg.Value
		if len(value) > 120 {
			value = value[:120] + "..."
		}
		value = strings.ReplaceAll(value, "\n", " ")
		key := msg.Key
		if key == "" {
			key = "(null)"
		}
		ts := ""
		if !msg.Timestamp.IsZero() {
			ts = msg.Timestamp.Format("15:04:05.000")
		}
		rows = append(rows, []string{
			strconv.FormatInt(int64(msg.Partition), 10),
			strconv.FormatInt(msg.Offset, 10),
			key,
			value,
			ts,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, []string{"No messages found", "", "", "", ""})
	}
	return pluginrpc.ViewData{
		View:         kafkaViewMessages,
		Title:        "Kafka Messages",
		Info:         s.baseInfo(fmt.Sprintf("Messages: %d (latest)", len(messages))),
		Status:       "connected",
		Headers:      []string{"Partition", "Offset", "Key", "Value", "Timestamp"},
		Rows:         rows,
		SelectionKey: "Offset",
		KeyBindings: withNav(
			pluginrpc.KeyBinding{Key: "I", Label: "Info", Action: "view_message"},
		),
	}, nil
}
