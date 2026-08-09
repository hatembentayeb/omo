package kafka

import (
	"fmt"
	"strconv"
	"strings"

	"omo/pkg/pluginrpc"
)

func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Brokers", Action: "goto_brokers"},
		{Key: "1", Label: "Topics", Action: "goto_topics"},
		{Key: "2", Label: "Consumers", Action: "goto_consumers"},
		{Key: "3", Label: "Partitions", Action: "goto_partitions"},
		{Key: "4", Label: "Messages", Action: "goto_messages"},
	}
}

func brokersActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "I", Label: "Info", Action: "info"},
	}
}

func topicsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "I", Label: "Info", Action: "info"},
		{Key: "P", Label: "Partitions", Action: "show_partitions"},
		{Key: "M", Label: "Messages", Action: "show_messages"},
	}
}

func consumersActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "I", Label: "Info", Action: "info"},
		{Key: "O", Label: "Offsets", Action: "view_offsets"},
	}
}

func partitionsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "I", Label: "Info", Action: "info"},
	}
}

func messagesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "I", Label: "Info", Action: "view_message"},
	}
}

func helpSections() []pluginrpc.HelpSection {
	return pluginrpc.HelpNav(viewNavBindings(), nil,
		pluginrpc.HelpSection{Title: "Brokers", Bindings: brokersActions()},
		pluginrpc.HelpSection{Title: "Topics", Bindings: topicsActions()},
		pluginrpc.HelpSection{Title: "Consumers", Bindings: consumersActions()},
		pluginrpc.HelpSection{Title: "Partitions", Bindings: partitionsActions()},
		pluginrpc.HelpSection{Title: "Messages", Bindings: messagesActions()},
	)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	Help:  helpSections,
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
	return pluginrpc.FormatInfo(msg, extra)
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = kafkaViewBrokers
	}
	s.currentView = viewID

	if err := s.ensureConnectedLocked(); err != nil {
		return ui.NotConnectedErr(viewID, "Kafka Manager", err)
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
	rows = pluginrpc.EnsureRows(rows, []string{"No brokers found", "", ""})
	return ui.Connected(kafkaViewBrokers, "Kafka Brokers", s.baseInfo(fmt.Sprintf("Brokers: %d", len(brokers))), []string{"ID", "Address", "Controller"}, rows, "ID", brokersActions()...), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"No topics found", "", "", ""})
	return ui.Connected(kafkaViewTopics, "Kafka Topics", s.baseInfo(fmt.Sprintf("Topics: %d", len(topics))), []string{"Name", "Partitions", "Replication", "Internal"}, rows, "Name", topicsActions()...), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"No consumer groups found", "", "", "", ""})
	return ui.Connected(kafkaViewConsumers, "Kafka Consumers", s.baseInfo(fmt.Sprintf("Consumer Groups: %d", len(groups))), []string{"Group ID", "State", "Members", "Protocol", "Protocol Type"}, rows, "Group ID", consumersActions()...), nil
}

func (s *Service) viewPartitionsLocked() (pluginrpc.ViewData, error) {
	if s.selectedTopic == "" {
		return ui.Connected(kafkaViewPartitions, "Kafka Partitions", s.baseInfo(""), []string{"Partition", "Leader", "Replicas", "ISR", "Oldest", "Newest", "Messages"}, [][]string{{"No topic selected", "Press 1 then P on a topic", "", "", "", "", ""}}, "Partition"), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"No partitions found", "", "", "", "", "", ""})
	return ui.Connected(kafkaViewPartitions, "Kafka Partitions", s.baseInfo(fmt.Sprintf("Partitions: %d", len(partitions))), []string{"Partition", "Leader", "Replicas", "ISR", "Oldest", "Newest", "Messages"}, rows, "Partition", partitionsActions()...), nil
}

func (s *Service) viewMessagesLocked() (pluginrpc.ViewData, error) {
	if s.selectedTopic == "" {
		return ui.Connected(kafkaViewMessages, "Kafka Messages", s.baseInfo(""), []string{"Partition", "Offset", "Key", "Value", "Timestamp"}, [][]string{{"No topic selected", "Press 1 then M on a topic", "", "", ""}}, "Offset"), nil
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
	rows = pluginrpc.EnsureRows(rows, []string{"No messages found", "", "", "", ""})
	return ui.Connected(kafkaViewMessages, "Kafka Messages", s.baseInfo(fmt.Sprintf("Messages: %d (latest)", len(messages))), []string{"Partition", "Offset", "Key", "Value", "Timestamp"}, rows, "Offset", messagesActions()...), nil
}
