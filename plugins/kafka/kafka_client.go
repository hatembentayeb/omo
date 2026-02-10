package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/IBM/sarama"
)

// KafkaClient handles interactions with Kafka clusters using sarama
type KafkaClient struct {
	client         sarama.Client
	admin          sarama.ClusterAdmin
	config         *sarama.Config
	currentCluster string
	brokers        string // bootstrap servers
	connected      bool
	lastRefresh    time.Time
	mu             sync.Mutex
}

// BrokerInfo represents a Kafka broker's information
type BrokerInfo struct {
	ID         int32
	Address    string
	Rack       string
	Controller bool
}

// TopicInfo represents information about a Kafka topic
type TopicInfo struct {
	Name              string
	Partitions        int32
	ReplicationFactor int16
	Internal          bool
	ConfigEntries     map[string]*string
}

// ConsumerGroupInfo represents information about a Kafka consumer group
type ConsumerGroupInfo struct {
	GroupID      string
	State        string
	Members      int
	Protocol     string
	ProtocolType string
}

// PartitionDetail represents detailed information about a topic partition
type PartitionDetail struct {
	ID           int32
	Leader       int32
	Replicas     []int32
	ISR          []int32
	OldestOffset int64
	NewestOffset int64
}

// ConsumerGroupOffset represents offset info for a consumer group
type ConsumerGroupOffset struct {
	Topic     string
	Partition int32
	Offset    int64
	Lag       int64
}

// NewKafkaClient creates a new Kafka client
func NewKafkaClient() *KafkaClient {
	return &KafkaClient{
		connected: false,
	}
}

// buildSaramaConfig builds a sarama config from a KafkaInstance
func buildSaramaConfig(instance *KafkaInstance) *sarama.Config {
	config := sarama.NewConfig()
	config.Version = sarama.V2_8_0_0 // reasonable default, works with most clusters
	config.Admin.Timeout = 10 * time.Second
	config.Net.DialTimeout = 10 * time.Second
	config.Net.ReadTimeout = 10 * time.Second
	config.Net.WriteTimeout = 10 * time.Second
	config.Metadata.Retry.Max = 3
	config.Metadata.Retry.Backoff = 250 * time.Millisecond

	if instance != nil && instance.Security.EnableSASL {
		config.Net.SASL.Enable = true
		config.Net.SASL.User = instance.Security.Username
		config.Net.SASL.Password = instance.Security.Password
		switch strings.ToUpper(instance.Security.SASLMechanism) {
		case "SCRAM-SHA-256":
			config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
			config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &XDGSCRAMClient{HashGeneratorFcn: SHA256}
			}
		case "SCRAM-SHA-512":
			config.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
			config.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &XDGSCRAMClient{HashGeneratorFcn: SHA512}
			}
		default:
			config.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		}
	}

	if instance != nil && instance.Security.EnableSSL {
		config.Net.TLS.Enable = true
		// TLS config would be built from cert paths here if needed
	}

	return config
}

// Connect connects to a Kafka cluster using bootstrap servers
func (c *KafkaClient) Connect(clusterName, bootstrapServers string, instance *KafkaInstance) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		c.disconnectLocked()
	}

	brokers := strings.Split(bootstrapServers, ",")
	for i, b := range brokers {
		brokers[i] = strings.TrimSpace(b)
	}

	cfg := buildSaramaConfig(instance)
	c.config = cfg

	client, err := sarama.NewClient(brokers, cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to Kafka cluster %s: %v", clusterName, err)
	}

	admin, err := sarama.NewClusterAdminFromClient(client)
	if err != nil {
		client.Close()
		return fmt.Errorf("failed to create admin client for %s: %v", clusterName, err)
	}

	c.client = client
	c.admin = admin
	c.currentCluster = clusterName
	c.brokers = bootstrapServers
	c.connected = true
	c.lastRefresh = time.Now()

	return nil
}

// Disconnect disconnects from the current Kafka cluster
func (c *KafkaClient) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disconnectLocked()
}

func (c *KafkaClient) disconnectLocked() {
	if c.admin != nil {
		c.admin.Close()
		c.admin = nil
	}
	if c.client != nil {
		c.client.Close()
		c.client = nil
	}
	c.connected = false
	c.currentCluster = ""
	c.brokers = ""
}

// IsConnected returns whether the client is connected
func (c *KafkaClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// GetCurrentCluster returns the name of the current cluster
func (c *KafkaClient) GetCurrentCluster() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.currentCluster
}

// GetBrokers returns the list of brokers in the cluster
func (c *KafkaClient) GetBrokers() ([]BrokerInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.client == nil {
		return nil, fmt.Errorf("not connected to any cluster")
	}

	// Refresh metadata
	if err := c.client.RefreshMetadata(); err != nil {
		return nil, fmt.Errorf("failed to refresh metadata: %v", err)
	}

	brokers := c.client.Brokers()
	controllerID, _ := c.client.Controller()

	var controllerBrokerID int32 = -1
	if controllerID != nil {
		controllerBrokerID = controllerID.ID()
	}

	result := make([]BrokerInfo, 0, len(brokers))
	for _, b := range brokers {
		info := BrokerInfo{
			ID:         b.ID(),
			Address:    b.Addr(),
			Rack:       "", // sarama doesn't expose rack directly on Broker
			Controller: b.ID() == controllerBrokerID,
		}
		result = append(result, info)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	c.lastRefresh = time.Now()
	return result, nil
}

// GetTopics returns the list of topics in the cluster
func (c *KafkaClient) GetTopics() ([]TopicInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.admin == nil {
		return nil, fmt.Errorf("not connected to any cluster")
	}

	topics, err := c.admin.ListTopics()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %v", err)
	}

	result := make([]TopicInfo, 0, len(topics))
	for name, detail := range topics {
		info := TopicInfo{
			Name:              name,
			Partitions:        detail.NumPartitions,
			ReplicationFactor: detail.ReplicationFactor,
			ConfigEntries:     detail.ConfigEntries,
		}
		// Check if topic is internal (starts with __ convention)
		if strings.HasPrefix(name, "__") {
			info.Internal = true
		}
		result = append(result, info)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// GetTopicPartitions returns partition details for a specific topic
func (c *KafkaClient) GetTopicPartitions(topic string) ([]PartitionDetail, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.client == nil {
		return nil, fmt.Errorf("not connected to any cluster")
	}

	partitions, err := c.client.Partitions(topic)
	if err != nil {
		return nil, fmt.Errorf("failed to get partitions for topic %s: %v", topic, err)
	}

	result := make([]PartitionDetail, 0, len(partitions))
	for _, p := range partitions {
		detail := PartitionDetail{
			ID: p,
		}

		// Get leader
		leader, err := c.client.Leader(topic, p)
		if err == nil && leader != nil {
			detail.Leader = leader.ID()
		} else {
			detail.Leader = -1
		}

		// Get replicas
		replicas, err := c.client.Replicas(topic, p)
		if err == nil {
			detail.Replicas = replicas
		}

		// Get ISR
		isr, err := c.client.InSyncReplicas(topic, p)
		if err == nil {
			detail.ISR = isr
		}

		// Get oldest offset
		oldest, err := c.client.GetOffset(topic, p, sarama.OffsetOldest)
		if err == nil {
			detail.OldestOffset = oldest
		}

		// Get newest offset
		newest, err := c.client.GetOffset(topic, p, sarama.OffsetNewest)
		if err == nil {
			detail.NewestOffset = newest
		}

		result = append(result, detail)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, nil
}

// GetConsumerGroups returns the list of consumer groups
func (c *KafkaClient) GetConsumerGroups() ([]ConsumerGroupInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.admin == nil {
		return nil, fmt.Errorf("not connected to any cluster")
	}

	groups, err := c.admin.ListConsumerGroups()
	if err != nil {
		return nil, fmt.Errorf("failed to list consumer groups: %v", err)
	}

	groupNames := make([]string, 0, len(groups))
	groupTypes := make(map[string]string)
	for name, protocolType := range groups {
		groupNames = append(groupNames, name)
		groupTypes[name] = protocolType
	}

	if len(groupNames) == 0 {
		return []ConsumerGroupInfo{}, nil
	}

	// Describe all groups for state and member info
	descriptions, err := c.admin.DescribeConsumerGroups(groupNames)
	if err != nil {
		// Fall back to basic info if describe fails
		result := make([]ConsumerGroupInfo, 0, len(groups))
		for name, protocolType := range groups {
			result = append(result, ConsumerGroupInfo{
				GroupID:      name,
				ProtocolType: protocolType,
				State:        "Unknown",
			})
		}
		sort.Slice(result, func(i, j int) bool {
			return result[i].GroupID < result[j].GroupID
		})
		return result, nil
	}

	result := make([]ConsumerGroupInfo, 0, len(descriptions))
	for _, desc := range descriptions {
		info := ConsumerGroupInfo{
			GroupID:      desc.GroupId,
			State:        desc.State,
			Members:      len(desc.Members),
			Protocol:     desc.Protocol,
			ProtocolType: desc.ProtocolType,
		}
		result = append(result, info)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].GroupID < result[j].GroupID
	})

	return result, nil
}

// GetConsumerGroupOffsets returns offset information for a consumer group
func (c *KafkaClient) GetConsumerGroupOffsets(groupID string) ([]ConsumerGroupOffset, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.admin == nil {
		return nil, fmt.Errorf("not connected to any cluster")
	}

	// Get all topics to check offsets against
	topics, err := c.client.Topics()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %v", err)
	}

	// Build topic-partitions map
	topicPartitions := make(map[string][]int32)
	for _, topic := range topics {
		partitions, err := c.client.Partitions(topic)
		if err != nil {
			continue
		}
		topicPartitions[topic] = partitions
	}

	// Fetch consumer group offsets
	offsetResponse, err := c.admin.ListConsumerGroupOffsets(groupID, topicPartitions)
	if err != nil {
		return nil, fmt.Errorf("failed to get offsets for group %s: %v", groupID, err)
	}

	var result []ConsumerGroupOffset
	for topic, partitions := range offsetResponse.Blocks {
		for partition, block := range partitions {
			if block.Offset == -1 {
				// No committed offset for this partition
				continue
			}

			// Get the latest offset for lag calculation
			newestOffset, err := c.client.GetOffset(topic, partition, sarama.OffsetNewest)
			if err != nil {
				continue
			}

			lag := newestOffset - block.Offset
			if lag < 0 {
				lag = 0
			}

			result = append(result, ConsumerGroupOffset{
				Topic:     topic,
				Partition: partition,
				Offset:    block.Offset,
				Lag:       lag,
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Topic != result[j].Topic {
			return result[i].Topic < result[j].Topic
		}
		return result[i].Partition < result[j].Partition
	})

	return result, nil
}

// MessageInfo represents a single Kafka message
type MessageInfo struct {
	Partition int32
	Offset    int64
	Key       string
	Value     string
	Timestamp time.Time
	Headers   map[string]string
}

// ConsumeMessages reads the latest N messages from a topic across all partitions.
// It reads from the newest offset backwards (tail behavior).
func (c *KafkaClient) ConsumeMessages(topic string, maxMessages int) ([]MessageInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.client == nil {
		return nil, fmt.Errorf("not connected to any cluster")
	}

	consumer, err := sarama.NewConsumerFromClient(c.client)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %v", err)
	}
	defer consumer.Close()

	partitions, err := consumer.Partitions(topic)
	if err != nil {
		return nil, fmt.Errorf("failed to get partitions for topic %s: %v", topic, err)
	}

	// Calculate how many messages per partition
	perPartition := maxMessages / len(partitions)
	if perPartition < 1 {
		perPartition = 1
	}

	var allMessages []MessageInfo
	var msgMu sync.Mutex
	var wg sync.WaitGroup

	for _, partition := range partitions {
		wg.Add(1)
		go func(p int32) {
			defer wg.Done()

			newestOffset, err := c.client.GetOffset(topic, p, sarama.OffsetNewest)
			if err != nil || newestOffset <= 0 {
				return
			}

			oldestOffset, err := c.client.GetOffset(topic, p, sarama.OffsetOldest)
			if err != nil {
				return
			}

			// Start reading from (newest - perPartition), clamped to oldest
			startOffset := newestOffset - int64(perPartition)
			if startOffset < oldestOffset {
				startOffset = oldestOffset
			}

			pc, err := consumer.ConsumePartition(topic, p, startOffset)
			if err != nil {
				return
			}
			defer pc.Close()

			count := 0
			timeout := time.After(5 * time.Second)
			for count < perPartition {
				select {
				case msg, ok := <-pc.Messages():
					if !ok {
						return
					}
					info := MessageInfo{
						Partition: msg.Partition,
						Offset:    msg.Offset,
						Key:       string(msg.Key),
						Value:     string(msg.Value),
						Timestamp: msg.Timestamp,
						Headers:   make(map[string]string),
					}
					for _, h := range msg.Headers {
						info.Headers[string(h.Key)] = string(h.Value)
					}
					msgMu.Lock()
					allMessages = append(allMessages, info)
					msgMu.Unlock()
					count++
				case <-timeout:
					return
				}
			}
		}(partition)
	}

	wg.Wait()

	// Sort by timestamp descending (newest first)
	sort.Slice(allMessages, func(i, j int) bool {
		if allMessages[i].Timestamp.Equal(allMessages[j].Timestamp) {
			return allMessages[i].Offset > allMessages[j].Offset
		}
		return allMessages[i].Timestamp.After(allMessages[j].Timestamp)
	})

	// Cap to maxMessages
	if len(allMessages) > maxMessages {
		allMessages = allMessages[:maxMessages]
	}

	return allMessages, nil
}

// GetLastRefreshTime returns the time of the last refresh
func (c *KafkaClient) GetLastRefreshTime() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastRefresh
}

// SetLastRefreshTime sets the time of the last refresh
func (c *KafkaClient) SetLastRefreshTime(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastRefresh = t
}
