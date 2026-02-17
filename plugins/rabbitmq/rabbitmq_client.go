package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitMQClient handles interactions with RabbitMQ via Management HTTP API and AMQP
type RabbitMQClient struct {
	mgmtURL     string
	amqpURL     string
	username    string
	password    string
	vhost       string
	httpClient  *http.Client
	amqpConn    *amqp.Connection
	connected   bool
	clusterName string
	lastRefresh time.Time
	mu          sync.Mutex
}

// RateDetail represents a rate details object from the Management API
type RateDetail struct {
	Rate float64 `json:"rate"`
}

// OverviewInfo represents the RabbitMQ overview from the management API
type OverviewInfo struct {
	ManagementVersion string `json:"management_version"`
	RabbitMQVersion   string `json:"rabbitmq_version"`
	ErlangVersion     string `json:"erlang_version"`
	ClusterName       string `json:"cluster_name"`
	Node              string `json:"node"`
	MessageStats      struct {
		Publish            int64       `json:"publish"`
		PublishDetails     *RateDetail `json:"publish_details,omitempty"`
		DeliverGet         int64       `json:"deliver_get"`
		DeliverGetDetails  *RateDetail `json:"deliver_get_details,omitempty"`
		Ack                int64       `json:"ack"`
		AckDetails         *RateDetail `json:"ack_details,omitempty"`
		Confirm            int64       `json:"confirm"`
		ConfirmDetails     *RateDetail `json:"confirm_details,omitempty"`
		Redeliver          int64       `json:"redeliver"`
		RedeliverDetails   *RateDetail `json:"redeliver_details,omitempty"`
		Deliver            int64       `json:"deliver"`
		Get                int64       `json:"get"`
		ReturnUnroutable   int64       `json:"return_unroutable"`
		DiskReads          int64       `json:"disk_reads"`
		DiskWrites         int64       `json:"disk_writes"`
	} `json:"message_stats"`
	QueueTotals struct {
		Messages       int64 `json:"messages"`
		MessagesReady  int64 `json:"messages_ready"`
		MessagesUnack  int64 `json:"messages_unacknowledged"`
	} `json:"queue_totals"`
	ObjectTotals struct {
		Queues      int `json:"queues"`
		Exchanges   int `json:"exchanges"`
		Connections int `json:"connections"`
		Channels    int `json:"channels"`
		Consumers   int `json:"consumers"`
	} `json:"object_totals"`
	Listeners []struct {
		Protocol string `json:"protocol"`
		Port     int    `json:"port"`
	} `json:"listeners"`
}

// QueueInfo represents a RabbitMQ queue
type QueueInfo struct {
	Name       string `json:"name"`
	VHost      string `json:"vhost"`
	Durable    bool   `json:"durable"`
	AutoDelete bool   `json:"auto_delete"`
	Exclusive  bool   `json:"exclusive"`
	Messages   int64  `json:"messages"`
	Ready      int64  `json:"messages_ready"`
	Unacked    int64  `json:"messages_unacknowledged"`
	Consumers  int    `json:"consumers"`
	State      string `json:"state"`
	Node       string `json:"node"`
	Type       string `json:"type"`
	Memory     int64  `json:"memory"`
	MessageStats struct {
		PublishIn   int64 `json:"publish"`
		DeliverGet  int64 `json:"deliver_get"`
		Ack         int64 `json:"ack"`
		Redeliver   int64 `json:"redeliver"`
	} `json:"message_stats"`
}

// ExchangeInfo represents a RabbitMQ exchange
type ExchangeInfo struct {
	Name       string `json:"name"`
	VHost      string `json:"vhost"`
	Type       string `json:"type"`
	Durable    bool   `json:"durable"`
	AutoDelete bool   `json:"auto_delete"`
	Internal   bool   `json:"internal"`
	MessageStats struct {
		PublishIn  int64 `json:"publish_in"`
		PublishOut int64 `json:"publish_out"`
	} `json:"message_stats"`
}

// BindingInfo represents a RabbitMQ binding
type BindingInfo struct {
	Source          string `json:"source"`
	VHost           string `json:"vhost"`
	Destination     string `json:"destination"`
	DestinationType string `json:"destination_type"`
	RoutingKey      string `json:"routing_key"`
	PropertiesKey   string `json:"properties_key"`
}

// ConnectionInfo represents a RabbitMQ connection
type ConnectionInfo struct {
	Name      string `json:"name"`
	Node      string `json:"node"`
	User      string `json:"user"`
	VHost     string `json:"vhost"`
	State     string `json:"state"`
	Protocol  string `json:"protocol"`
	PeerHost  string `json:"peer_host"`
	PeerPort  int    `json:"peer_port"`
	Channels  int    `json:"channels"`
	SSL       bool   `json:"ssl"`
	RecvOct   int64  `json:"recv_oct"`
	SendOct   int64  `json:"send_oct"`
	Connected int64  `json:"connected_at"`
}

// ChannelInfo represents a RabbitMQ channel
type ChannelInfo struct {
	Name          string `json:"name"`
	Node          string `json:"node"`
	User          string `json:"user"`
	VHost         string `json:"vhost"`
	Number        int    `json:"number"`
	State         string `json:"state"`
	Consumers     int    `json:"consumer_count"`
	PrefetchCount int    `json:"prefetch_count"`
	Confirm       bool   `json:"confirm"`
	Transactional bool   `json:"transactional"`
	MessagesUnack int64  `json:"messages_unacknowledged"`
	MessagesUnconfirmed int64 `json:"messages_unconfirmed"`
	Connection    string `json:"connection_details,omitempty"`
}

// NodeInfo represents a RabbitMQ cluster node
type NodeInfo struct {
	Name         string  `json:"name"`
	Type         string  `json:"type"`
	Running      bool    `json:"running"`
	MemUsed      int64   `json:"mem_used"`
	MemLimit     int64   `json:"mem_limit"`
	DiskFree     int64   `json:"disk_free"`
	DiskFreeLimit int64  `json:"disk_free_limit"`
	FDUsed       int     `json:"fd_used"`
	FDTotal      int     `json:"fd_total"`
	SocketsUsed  int     `json:"sockets_used"`
	SocketsTotal int     `json:"sockets_total"`
	ProcUsed     int     `json:"proc_used"`
	ProcTotal    int     `json:"proc_total"`
	Uptime       int64   `json:"uptime"`
	RatesMode    string  `json:"rates_mode"`
}

// VHostInfo represents a RabbitMQ virtual host
type VHostInfo struct {
	Name     string `json:"name"`
	Messages int64  `json:"messages"`
	Tracing  bool   `json:"tracing"`
}

// PublishMessage represents a message to publish
type PublishMessage struct {
	Exchange   string
	RoutingKey string
	Body       string
	Headers    map[string]interface{}
}

// NewRabbitMQClient creates a new client
func NewRabbitMQClient() *RabbitMQClient {
	return &RabbitMQClient{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		connected:  false,
	}
}

// Connect connects to a RabbitMQ instance
func (c *RabbitMQClient) Connect(instance RabbitMQInstance) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if instance.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	if instance.AMQPPort == 0 {
		instance.AMQPPort = 5672
	}
	if instance.MgmtPort == 0 {
		instance.MgmtPort = 15672
	}
	if instance.VHost == "" {
		instance.VHost = "/"
	}
	if instance.Username == "" {
		instance.Username = "guest"
	}
	if instance.Password == "" {
		instance.Password = "guest"
	}

	scheme := "http"
	amqpScheme := "amqp"
	if instance.UseTLS {
		scheme = "https"
		amqpScheme = "amqps"
	}

	c.mgmtURL = fmt.Sprintf("%s://%s:%d/api", scheme, instance.Host, instance.MgmtPort)
	c.amqpURL = fmt.Sprintf("%s://%s:%s@%s:%d/%s",
		amqpScheme,
		url.QueryEscape(instance.Username),
		url.QueryEscape(instance.Password),
		instance.Host, instance.AMQPPort,
		url.QueryEscape(instance.VHost))
	c.username = instance.Username
	c.password = instance.Password
	c.vhost = instance.VHost
	c.clusterName = instance.Name

	// Test management API connection
	_, err := c.GetOverview()
	if err != nil {
		return fmt.Errorf("management API connection failed: %v", err)
	}

	c.connected = true
	c.lastRefresh = time.Now()
	return nil
}

// Disconnect closes the connection
func (c *RabbitMQClient) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.amqpConn != nil && !c.amqpConn.IsClosed() {
		c.amqpConn.Close()
	}
	c.amqpConn = nil
	c.connected = false
}

// IsConnected returns whether the client is connected
func (c *RabbitMQClient) IsConnected() bool {
	return c.connected
}

// GetClusterName returns the current cluster name
func (c *RabbitMQClient) GetClusterName() string {
	return c.clusterName
}

// apiGet performs a GET request against the Management API
func (c *RabbitMQClient) apiGet(path string) ([]byte, error) {
	reqURL := c.mgmtURL + path
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// apiDelete performs a DELETE request against the Management API
func (c *RabbitMQClient) apiDelete(path string) error {
	reqURL := c.mgmtURL + path
	req, err := http.NewRequest("DELETE", reqURL, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// apiPut performs a PUT request against the Management API
func (c *RabbitMQClient) apiPut(path string, jsonBody string) error {
	reqURL := c.mgmtURL + path
	req, err := http.NewRequest("PUT", reqURL, strings.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// apiPost performs a POST request against the Management API
func (c *RabbitMQClient) apiPost(path string, jsonBody string) ([]byte, error) {
	reqURL := c.mgmtURL + path
	req, err := http.NewRequest("POST", reqURL, strings.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.username, c.password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// GetOverview returns the RabbitMQ overview
func (c *RabbitMQClient) GetOverview() (*OverviewInfo, error) {
	data, err := c.apiGet("/overview")
	if err != nil {
		return nil, err
	}

	var overview OverviewInfo
	if err := json.Unmarshal(data, &overview); err != nil {
		return nil, fmt.Errorf("failed to parse overview: %v", err)
	}

	return &overview, nil
}

// GetQueues returns all queues for the current vhost
func (c *RabbitMQClient) GetQueues() ([]QueueInfo, error) {
	path := fmt.Sprintf("/queues/%s", url.PathEscape(c.vhost))
	data, err := c.apiGet(path)
	if err != nil {
		return nil, err
	}

	var queues []QueueInfo
	if err := json.Unmarshal(data, &queues); err != nil {
		return nil, fmt.Errorf("failed to parse queues: %v", err)
	}

	return queues, nil
}

// GetExchanges returns all exchanges for the current vhost
func (c *RabbitMQClient) GetExchanges() ([]ExchangeInfo, error) {
	path := fmt.Sprintf("/exchanges/%s", url.PathEscape(c.vhost))
	data, err := c.apiGet(path)
	if err != nil {
		return nil, err
	}

	var exchanges []ExchangeInfo
	if err := json.Unmarshal(data, &exchanges); err != nil {
		return nil, fmt.Errorf("failed to parse exchanges: %v", err)
	}

	return exchanges, nil
}

// GetBindings returns all bindings for the current vhost
func (c *RabbitMQClient) GetBindings() ([]BindingInfo, error) {
	path := fmt.Sprintf("/bindings/%s", url.PathEscape(c.vhost))
	data, err := c.apiGet(path)
	if err != nil {
		return nil, err
	}

	var bindings []BindingInfo
	if err := json.Unmarshal(data, &bindings); err != nil {
		return nil, fmt.Errorf("failed to parse bindings: %v", err)
	}

	return bindings, nil
}

// GetConnections returns all connections
func (c *RabbitMQClient) GetConnections() ([]ConnectionInfo, error) {
	data, err := c.apiGet("/connections")
	if err != nil {
		return nil, err
	}

	var connections []ConnectionInfo
	if err := json.Unmarshal(data, &connections); err != nil {
		return nil, fmt.Errorf("failed to parse connections: %v", err)
	}

	return connections, nil
}

// GetChannels returns all channels
func (c *RabbitMQClient) GetChannels() ([]ChannelInfo, error) {
	data, err := c.apiGet("/channels")
	if err != nil {
		return nil, err
	}

	var channels []ChannelInfo
	if err := json.Unmarshal(data, &channels); err != nil {
		return nil, fmt.Errorf("failed to parse channels: %v", err)
	}

	return channels, nil
}

// GetNodes returns all cluster nodes
func (c *RabbitMQClient) GetNodes() ([]NodeInfo, error) {
	data, err := c.apiGet("/nodes")
	if err != nil {
		return nil, err
	}

	var nodes []NodeInfo
	if err := json.Unmarshal(data, &nodes); err != nil {
		return nil, fmt.Errorf("failed to parse nodes: %v", err)
	}

	return nodes, nil
}

// GetVHosts returns all virtual hosts
func (c *RabbitMQClient) GetVHosts() ([]VHostInfo, error) {
	data, err := c.apiGet("/vhosts")
	if err != nil {
		return nil, err
	}

	var vhosts []VHostInfo
	if err := json.Unmarshal(data, &vhosts); err != nil {
		return nil, fmt.Errorf("failed to parse vhosts: %v", err)
	}

	return vhosts, nil
}

// GetQueueMessages fetches messages from a queue (non-destructive peek)
func (c *RabbitMQClient) GetQueueMessages(queueName string, count int) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("/queues/%s/%s/get", url.PathEscape(c.vhost), url.PathEscape(queueName))
	body := fmt.Sprintf(`{"count":%d,"ackmode":"ack_requeue_true","encoding":"auto"}`, count)
	data, err := c.apiPost(path, body)
	if err != nil {
		return nil, err
	}

	var messages []map[string]interface{}
	if err := json.Unmarshal(data, &messages); err != nil {
		return nil, fmt.Errorf("failed to parse messages: %v", err)
	}

	return messages, nil
}

// CreateQueue creates a new queue
func (c *RabbitMQClient) CreateQueue(name string, durable bool, autoDelete bool) error {
	path := fmt.Sprintf("/queues/%s/%s", url.PathEscape(c.vhost), url.PathEscape(name))
	body := fmt.Sprintf(`{"durable":%t,"auto_delete":%t}`, durable, autoDelete)
	return c.apiPut(path, body)
}

// DeleteQueue deletes a queue
func (c *RabbitMQClient) DeleteQueue(name string) error {
	path := fmt.Sprintf("/queues/%s/%s", url.PathEscape(c.vhost), url.PathEscape(name))
	return c.apiDelete(path)
}

// PurgeQueue purges all messages from a queue
func (c *RabbitMQClient) PurgeQueue(name string) error {
	path := fmt.Sprintf("/queues/%s/%s/contents", url.PathEscape(c.vhost), url.PathEscape(name))
	return c.apiDelete(path)
}

// CreateExchange creates a new exchange
func (c *RabbitMQClient) CreateExchange(name, exType string, durable bool) error {
	path := fmt.Sprintf("/exchanges/%s/%s", url.PathEscape(c.vhost), url.PathEscape(name))
	body := fmt.Sprintf(`{"type":%q,"durable":%t}`, exType, durable)
	return c.apiPut(path, body)
}

// DeleteExchange deletes an exchange
func (c *RabbitMQClient) DeleteExchange(name string) error {
	path := fmt.Sprintf("/exchanges/%s/%s", url.PathEscape(c.vhost), url.PathEscape(name))
	return c.apiDelete(path)
}

// CreateBinding creates a binding between exchange and queue
func (c *RabbitMQClient) CreateBinding(source, destination, routingKey string) error {
	path := fmt.Sprintf("/bindings/%s/e/%s/q/%s",
		url.PathEscape(c.vhost), url.PathEscape(source), url.PathEscape(destination))
	body := fmt.Sprintf(`{"routing_key":%q}`, routingKey)
	_, err := c.apiPost(path, body)
	return err
}

// CloseConnection forcefully closes a connection
func (c *RabbitMQClient) CloseConnection(name string) error {
	path := fmt.Sprintf("/connections/%s", url.PathEscape(name))
	return c.apiDelete(path)
}

// PublishMessageToExchange publishes a message via the Management API
func (c *RabbitMQClient) PublishMessageToExchange(exchange, routingKey, payload string) error {
	path := fmt.Sprintf("/exchanges/%s/%s/publish", url.PathEscape(c.vhost), url.PathEscape(exchange))
	body := fmt.Sprintf(`{"properties":{},"routing_key":%q,"payload":%q,"payload_encoding":"string"}`,
		routingKey, payload)
	data, err := c.apiPost(path, body)
	if err != nil {
		return err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return fmt.Errorf("failed to parse publish result: %v", err)
	}

	if routed, ok := result["routed"].(bool); ok && !routed {
		return fmt.Errorf("message was not routed (no matching binding)")
	}

	return nil
}

// PublishMessageAMQP publishes a message via AMQP protocol
func (c *RabbitMQClient) PublishMessageAMQP(exchange, routingKey, body string) error {
	conn, err := amqp.Dial(c.amqpURL)
	if err != nil {
		return fmt.Errorf("AMQP connection failed: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel: %v", err)
	}
	defer ch.Close()

	return ch.Publish(exchange, routingKey, false, false, amqp.Publishing{
		ContentType: "text/plain",
		Body:        []byte(body),
		Timestamp:   time.Now(),
	})
}
