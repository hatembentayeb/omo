package kafka

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

// Service is the RPC-facing kafka backend (no tview).
type Service struct {
	mu             sync.Mutex
	client         *KafkaClient
	bootstrap      string
	name           string
	enableSASL     bool
	saslMechanism  string
	username       string
	password       string
	enableSSL      bool
	sslCACert      string
	sslCert        string
	sslKey         string
	currentView    string
	selectedTopic  string
	cachedMessages []MessageInfo
}

// NewService creates a kafka RPC service.
func NewService() *Service {
	return &Service{
		client:      NewKafkaClient(),
		currentView: kafkaViewBrokers,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "kafka",
		Version:     "2.0.0",
		Description: "Manage Kafka brokers, topics, and consumers",
		Author:      "HATMAN",
		License:     "MIT",
		Tags:        []string{"messaging", "streaming", "broker"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/ohmyops-v2/plugins/kafka",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("Service.Configure begin")

	if req.Settings == nil {
		return fmt.Errorf("missing settings")
	}
	s.bootstrap = firstNonEmpty(req.Settings["host"], req.Settings["url"], req.Settings["bootstrap_servers"])
	s.name = req.Settings["name"]
	s.username = req.Settings["username"]
	s.password = req.Settings["password"]
	s.saslMechanism = req.Settings["sasl_mechanism"]
	s.enableSASL = truthy(req.Settings["enable_sasl"])
	s.enableSSL = truthy(req.Settings["enable_ssl"])
	s.sslCACert = req.Settings["ssl_ca_cert"]
	s.sslCert = req.Settings["ssl_cert"]
	s.sslKey = req.Settings["ssl_key"]

	// If credentials present but enable_sasl unset, enable SASL automatically.
	if !s.enableSASL && s.username != "" && s.password != "" && s.saslMechanism != "" {
		s.enableSASL = true
	}
	if s.bootstrap == "" {
		return fmt.Errorf("bootstrap servers (host/url) required")
	}

	pluginrpc.RPCLog("Service.Configure bootstrap=%s sasl=%v ssl=%v mech=%s",
		s.bootstrap, s.enableSASL, s.enableSSL, s.saslMechanism)

	if s.client != nil && s.client.IsConnected() {
		s.client.Disconnect()
	}
	return nil
}

func (s *Service) GetView(req pluginrpc.ViewRequest) (pluginrpc.ViewData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	viewID := req.View
	if viewID == "" {
		viewID = s.currentView
	}
	if viewID == "" {
		viewID = kafkaViewBrokers
	}
	pluginrpc.RPCLog("Service.GetView begin view=%s bootstrap=%s connected=%v", viewID, s.bootstrap, s.client != nil && s.client.IsConnected())
	start := time.Now()
	view, err := s.buildViewLocked(viewID)
	if err != nil {
		pluginrpc.RPCLog("Service.GetView err=%v", err)
		return pluginrpc.ViewData{}, err
	}
	pluginrpc.RPCLog("Service.GetView OK view=%s rows=%d dur=%s", view.View, len(view.Rows), time.Since(start))
	return view, nil
}

func (s *Service) DoAction(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	action := req.Action
	if strings.HasPrefix(action, "goto_") {
		viewID := strings.TrimPrefix(action, "goto_")
		// Selecting topic before partitions/messages
		if viewID == kafkaViewPartitions || viewID == kafkaViewMessages {
			if topic := firstNonEmpty(req.Payload["topic"], req.Payload["key"]); topic != "" {
				if !strings.HasPrefix(topic, "No ") && !strings.HasPrefix(topic, "Error") && !strings.HasPrefix(topic, "Not ") {
					s.selectedTopic = topic
				}
			}
		}
		view, err := s.buildViewLocked(viewID)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "switched to " + viewID, Next: &view}, nil
	}

	switch action {
	case "refresh", "":
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "refreshed", Next: &view}, nil

	case "show_partitions":
		topic := firstNonEmpty(req.Payload["topic"], req.Payload["key"])
		if topic == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no topic selected"}, nil
		}
		s.selectedTopic = topic
		view, err := s.buildViewLocked(kafkaViewPartitions)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "partitions for " + topic, Next: &view}, nil

	case "show_messages", "peek_messages":
		topic := firstNonEmpty(req.Payload["topic"], req.Payload["key"], s.selectedTopic)
		if topic == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no topic selected"}, nil
		}
		s.selectedTopic = topic
		view, err := s.buildViewLocked(kafkaViewMessages)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "messages for " + topic, Next: &view}, nil

	case "info", "view_info":
		return s.infoModalLocked(req)

	case "view_offsets", "offsets":
		group := firstNonEmpty(req.Payload["group"], req.Payload["key"])
		if group == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no consumer group selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		offsets, err := s.client.GetConsumerGroupOffsets(group)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Consumer Group: %s\n\n", group))
		b.WriteString(fmt.Sprintf("%-40s %8s %12s %12s\n", "Topic", "Part", "Offset", "Lag"))
		for _, o := range offsets {
			b.WriteString(fmt.Sprintf("%-40s %8d %12d %12d\n", o.Topic, o.Partition, o.Offset, o.Lag))
		}
		if len(offsets) == 0 {
			b.WriteString("(no offsets)")
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Offsets: " + group, ModalBody: b.String()}, nil

	case "view_message":
		part := firstNonEmpty(req.Payload["partition"], req.Payload["col0"], req.Payload["key"])
		offset := firstNonEmpty(req.Payload["offset"], req.Payload["col1"])
		for _, msg := range s.cachedMessages {
			if strconv.FormatInt(int64(msg.Partition), 10) == part && strconv.FormatInt(msg.Offset, 10) == offset {
				return pluginrpc.ActionResult{
					OK:         true,
					ModalTitle: fmt.Sprintf("Message p=%s o=%s", part, offset),
					ModalBody:  formatFullMessageDetail(s.selectedTopic, &msg),
				}, nil
			}
		}
		// Fallback from row cols
		key := firstNonEmpty(req.Payload["col2"], "(null)")
		value := req.Payload["col3"]
		body := fmt.Sprintf("Topic: %s\nPartition: %s\nOffset: %s\nKey: %s\n\n%s",
			s.selectedTopic, part, offset, key, value)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Message", ModalBody: body}, nil

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) infoModalLocked(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	key := firstNonEmpty(req.Payload["key"], req.Payload["name"])
	if key == "" {
		return pluginrpc.ActionResult{OK: false, Message: "nothing selected"}, nil
	}
	if err := s.ensureConnectedLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	switch s.currentView {
	case kafkaViewBrokers:
		body := fmt.Sprintf("Broker ID: %s\nAddress: %s\nController: %s\nCluster: %s",
			firstNonEmpty(req.Payload["col0"], key),
			req.Payload["col1"],
			req.Payload["col2"],
			s.name)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Broker #" + key, ModalBody: body}, nil
	case kafkaViewTopics:
		topics, err := s.client.GetTopics()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		for _, t := range topics {
			if t.Name == key {
				var cfg strings.Builder
				for k, v := range t.ConfigEntries {
					val := ""
					if v != nil {
						val = *v
					}
					cfg.WriteString(fmt.Sprintf("  %s = %s\n", k, val))
				}
				body := fmt.Sprintf("Name: %s\nPartitions: %d\nReplication: %d\nInternal: %v\n\nConfig:\n%s",
					t.Name, t.Partitions, t.ReplicationFactor, t.Internal, cfg.String())
				return pluginrpc.ActionResult{OK: true, ModalTitle: "Topic: " + key, ModalBody: body}, nil
			}
		}
	case kafkaViewConsumers:
		body := fmt.Sprintf("Group ID: %s\nState: %s\nMembers: %s\nProtocol: %s\nProtocol Type: %s\nCluster: %s",
			key, req.Payload["col1"], req.Payload["col2"], req.Payload["col3"], req.Payload["col4"], s.name)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Consumer Group: " + key, ModalBody: body}, nil
	case kafkaViewPartitions:
		body := fmt.Sprintf("Topic: %s\nPartition: %s\nLeader: %s\nReplicas: %s\nISR: %s\nOldest: %s\nNewest: %s\nMessages: %s",
			s.selectedTopic, key, req.Payload["col1"], req.Payload["col2"], req.Payload["col3"],
			req.Payload["col4"], req.Payload["col5"], req.Payload["col6"])
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Partition #" + key, ModalBody: body}, nil
	case kafkaViewMessages:
		part := firstNonEmpty(req.Payload["partition"], req.Payload["col0"], req.Payload["key"])
		offset := firstNonEmpty(req.Payload["offset"], req.Payload["col1"])
		for i := range s.cachedMessages {
			msg := &s.cachedMessages[i]
			if strconv.FormatInt(int64(msg.Partition), 10) == part && strconv.FormatInt(msg.Offset, 10) == offset {
				return pluginrpc.ActionResult{
					OK:         true,
					ModalTitle: fmt.Sprintf("Message p=%s o=%s", part, offset),
					ModalBody:  formatFullMessageDetail(s.selectedTopic, msg),
				}, nil
			}
		}
		body := fmt.Sprintf("Topic: %s\nPartition: %s\nOffset: %s\nKey: %s\n\n%s",
			s.selectedTopic, part, offset,
			firstNonEmpty(req.Payload["col2"], "(null)"),
			req.Payload["col3"])
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Message", ModalBody: body}, nil
	}
	return pluginrpc.ActionResult{OK: false, Message: "not found: " + key}, nil
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client != nil && s.client.IsConnected() {
		s.client.Disconnect()
	}
	return nil
}

func (s *Service) ensureConnectedLocked() error {
	if s.client != nil && s.client.IsConnected() {
		return nil
	}
	if s.bootstrap == "" {
		return fmt.Errorf("not configured (host did not call Configure)")
	}
	inst := &KafkaInstance{
		Name:             s.name,
		BootstrapServers: s.bootstrap,
		Security: KafkaSecurity{
			EnableSASL:    s.enableSASL,
			SASLMechanism: s.saslMechanism,
			Username:      s.username,
			Password:      s.password,
			EnableSSL:     s.enableSSL,
			SSLCACert:     s.sslCACert,
			SSLCert:       s.sslCert,
			SSLKey:        s.sslKey,
		},
	}
	name := s.name
	if name == "" {
		name = "kafka"
	}
	pluginrpc.RPCLog("ensureConnected: dialing %s sasl=%v ssl=%v …", s.bootstrap, s.enableSASL, s.enableSSL)
	start := time.Now()
	err := s.client.Connect(name, s.bootstrap, inst)
	pluginrpc.RPCLog("ensureConnected: dial done err=%v dur=%s", err, time.Since(start))
	return err
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func truthy(v string) bool {
	return v == "true" || v == "1" || v == "yes"
}
