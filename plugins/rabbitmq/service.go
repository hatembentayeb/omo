package rabbitmq

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

// Service is the RPC-facing rabbitmq backend (no tview).
type Service struct {
	mu          sync.Mutex
	client      *RabbitMQClient
	host        string
	amqpPort    int
	mgmtPort    int
	username    string
	password    string
	vhost       string
	useTLS      bool
	name        string
	currentView string
}

// NewService creates a rabbitmq RPC service.
func NewService() *Service {
	return &Service{
		client:      NewRabbitMQClient(),
		amqpPort:    5672,
		mgmtPort:    15672,
		vhost:       "/",
		currentView: rmqViewOverview,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "rabbitmq",
		Version:     "1.0.0",
		Description: "Manage RabbitMQ queues, exchanges, bindings, and connections",
		Author:      "HATMAN",
		License:     "MIT",
		Tags:        []string{"messaging", "broker", "amqp"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/ohmyops/omo-rabbitmq",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("Service.Configure begin")

	if req.Settings == nil {
		return fmt.Errorf("missing settings")
	}
	s.host = firstNonEmpty(req.Settings["host"], req.Settings["url"])
	s.username = req.Settings["username"]
	s.password = req.Settings["password"]
	s.name = req.Settings["name"]
	s.amqpPort = 5672
	s.mgmtPort = 15672
	s.vhost = "/"
	s.useTLS = false

	if v := req.Settings["amqp_port"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.amqpPort = n
		}
	}
	if v := req.Settings["mgmt_port"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.mgmtPort = n
		}
	}
	// allow generic port as mgmt port fallback
	if v := req.Settings["port"]; v != "" && req.Settings["mgmt_port"] == "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.mgmtPort = n
		}
	}
	if v := req.Settings["vhost"]; v != "" {
		s.vhost = v
	}
	if v := req.Settings["use_tls"]; v != "" {
		s.useTLS = v == "true" || v == "1" || v == "yes"
	}
	if s.host == "" {
		return fmt.Errorf("host is required")
	}

	pluginrpc.RPCLog("Service.Configure host=%s amqp=%d mgmt=%d user=%s vhost=%s",
		s.host, s.amqpPort, s.mgmtPort, s.username, s.vhost)

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
		viewID = rmqViewOverview
	}
	pluginrpc.RPCLog("Service.GetView begin view=%s host=%s connected=%v", viewID, s.host, s.client != nil && s.client.IsConnected())
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

	case "create_queue":
		name := firstNonEmpty(req.Payload["name"], req.Payload["key"], req.Payload["queue"])
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "queue name required"}, nil
		}
		durable := req.Payload["durable"] != "false"
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.CreateQueue(name, durable, false); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(rmqViewQueues)
		return pluginrpc.ActionResult{OK: true, Message: "created queue " + name, Next: &view}, nil

	case "delete_queue", "delete":
		name := firstNonEmpty(req.Payload["name"], req.Payload["key"])
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no queue/exchange selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if s.currentView == rmqViewExchanges || action == "delete_exchange" {
			if name == "(default)" {
				name = ""
			}
			if err := s.client.DeleteExchange(name); err != nil {
				return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
			}
			view, _ := s.buildViewLocked(rmqViewExchanges)
			return pluginrpc.ActionResult{OK: true, Message: "deleted exchange " + name, Next: &view}, nil
		}
		if err := s.client.DeleteQueue(name); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(rmqViewQueues)
		return pluginrpc.ActionResult{OK: true, Message: "deleted queue " + name, Next: &view}, nil

	case "delete_exchange":
		name := firstNonEmpty(req.Payload["name"], req.Payload["key"])
		if name == "(default)" {
			name = ""
		}
		if name == "" && req.Payload["key"] != "(default)" {
			return pluginrpc.ActionResult{OK: false, Message: "no exchange selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.DeleteExchange(name); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(rmqViewExchanges)
		return pluginrpc.ActionResult{OK: true, Message: "deleted exchange", Next: &view}, nil

	case "purge_queue", "purge":
		name := firstNonEmpty(req.Payload["name"], req.Payload["key"])
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no queue selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.PurgeQueue(name); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(rmqViewQueues)
		return pluginrpc.ActionResult{OK: true, Message: "purged queue " + name, Next: &view}, nil

	case "create_exchange":
		name := firstNonEmpty(req.Payload["name"], req.Payload["key"])
		exType := req.Payload["type"]
		if exType == "" {
			exType = "direct"
		}
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "exchange name required"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.CreateExchange(name, exType, true); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(rmqViewExchanges)
		return pluginrpc.ActionResult{OK: true, Message: "created exchange " + name, Next: &view}, nil

	case "publish":
		queue := firstNonEmpty(req.Payload["queue"], req.Payload["key"], req.Payload["routing_key"])
		message := firstNonEmpty(req.Payload["message"], req.Payload["payload"], req.Payload["body"])
		exchange := req.Payload["exchange"]
		if queue == "" || message == "" {
			return pluginrpc.ActionResult{OK: false, Message: "queue/routing key and message required"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.PublishMessageToExchange(exchange, queue, message); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(rmqViewQueues)
		return pluginrpc.ActionResult{OK: true, Message: "published to " + queue, Next: &view}, nil

	case "browse_messages", "messages":
		name := firstNonEmpty(req.Payload["name"], req.Payload["key"], req.Payload["queue"])
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no queue selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		msgs, err := s.client.GetQueueMessages(name, 10)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Queue: %s\nMessages peeked: %d\n\n", name, len(msgs)))
		for i, m := range msgs {
			payload, _ := m["payload"].(string)
			rk, _ := m["routing_key"].(string)
			b.WriteString(fmt.Sprintf("--- #%d routing_key=%s ---\n%s\n\n", i+1, rk, payload))
		}
		if len(msgs) == 0 {
			b.WriteString("(empty)")
		}
		return pluginrpc.ActionResult{OK: true, ModalTitle: "Messages: " + name, ModalBody: b.String()}, nil

	case "close_connection":
		name := firstNonEmpty(req.Payload["name"], req.Payload["key"])
		if name == "" {
			return pluginrpc.ActionResult{OK: false, Message: "no connection selected"}, nil
		}
		if err := s.ensureConnectedLocked(); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		if err := s.client.CloseConnection(name); err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		view, _ := s.buildViewLocked(rmqViewConnections)
		return pluginrpc.ActionResult{OK: true, Message: "closed connection", Next: &view}, nil

	case "info", "view_info":
		return s.infoModalLocked(req)

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
	case rmqViewQueues:
		queues, err := s.client.GetQueues()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		for _, q := range queues {
			if q.Name == key {
				body := fmt.Sprintf("Name: %s\nVHost: %s\nState: %s\nMessages: %d\nReady: %d\nUnacked: %d\nConsumers: %d\nType: %s\nNode: %s\nDurable: %v\nAutoDelete: %v",
					q.Name, q.VHost, q.State, q.Messages, q.Ready, q.Unacked, q.Consumers, q.Type, q.Node, q.Durable, q.AutoDelete)
				return pluginrpc.ActionResult{OK: true, ModalTitle: "Queue: " + key, ModalBody: body}, nil
			}
		}
	case rmqViewExchanges:
		name := key
		if name == "(default)" {
			name = ""
		}
		exchanges, err := s.client.GetExchanges()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		for _, ex := range exchanges {
			if ex.Name == name {
				display := ex.Name
				if display == "" {
					display = "(default)"
				}
				body := fmt.Sprintf("Name: %s\nType: %s\nVHost: %s\nDurable: %v\nAutoDelete: %v\nInternal: %v\nPublishIn: %d\nPublishOut: %d",
					display, ex.Type, ex.VHost, ex.Durable, ex.AutoDelete, ex.Internal, ex.MessageStats.PublishIn, ex.MessageStats.PublishOut)
				return pluginrpc.ActionResult{OK: true, ModalTitle: "Exchange: " + display, ModalBody: body}, nil
			}
		}
	case rmqViewConnections:
		conns, err := s.client.GetConnections()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		for _, c := range conns {
			if c.Name == key {
				body := fmt.Sprintf("Name: %s\nUser: %s\nVHost: %s\nState: %s\nProtocol: %s\nPeer: %s:%d\nChannels: %d\nSSL: %v\nNode: %s",
					c.Name, c.User, c.VHost, c.State, c.Protocol, c.PeerHost, c.PeerPort, c.Channels, c.SSL, c.Node)
				return pluginrpc.ActionResult{OK: true, ModalTitle: "Connection", ModalBody: body}, nil
			}
		}
	case rmqViewChannels:
		channels, err := s.client.GetChannels()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		for _, ch := range channels {
			if ch.Name == key {
				body := fmt.Sprintf("Name: %s\nUser: %s\nVHost: %s\nState: %s\nConsumers: %d\nPrefetch: %d\nUnacked: %d\nConfirm: %v\nNode: %s",
					ch.Name, ch.User, ch.VHost, ch.State, ch.Consumers, ch.PrefetchCount, ch.MessagesUnack, ch.Confirm, ch.Node)
				return pluginrpc.ActionResult{OK: true, ModalTitle: "Channel", ModalBody: body}, nil
			}
		}
	case rmqViewNodes:
		nodes, err := s.client.GetNodes()
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		for _, n := range nodes {
			if n.Name == key {
				body := fmt.Sprintf("Name: %s\nType: %s\nRunning: %v\nMemUsed: %d\nDiskFree: %d\nFD: %d/%d\nSockets: %d/%d\nUptimeMs: %d",
					n.Name, n.Type, n.Running, n.MemUsed, n.DiskFree, n.FDUsed, n.FDTotal, n.SocketsUsed, n.SocketsTotal, n.Uptime)
				return pluginrpc.ActionResult{OK: true, ModalTitle: "Node: " + key, ModalBody: body}, nil
			}
		}
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
	if s.host == "" {
		return fmt.Errorf("not configured (host did not call Configure)")
	}
	inst := RabbitMQInstance{
		Name:     s.name,
		Host:     s.host,
		AMQPPort: s.amqpPort,
		MgmtPort: s.mgmtPort,
		Username: s.username,
		Password: s.password,
		VHost:    s.vhost,
		UseTLS:   s.useTLS,
	}
	pluginrpc.RPCLog("ensureConnected: dialing %s mgmt=%d …", s.host, s.mgmtPort)
	start := time.Now()
	err := s.client.Connect(inst)
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
