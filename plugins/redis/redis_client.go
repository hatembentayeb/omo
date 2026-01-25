package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisConnection represents a connection to a Redis server
type RedisConnection struct {
	Host     string
	Port     string
	Password string
	Database int
}

// RedisClient is a client for interacting with Redis
type RedisClient struct {
	conn        *RedisConnection
	client      *redis.Client
	ctx         context.Context
	connected   bool
	lastRefresh time.Time
}

// SlowLogEntry represents a Redis slowlog entry
type SlowLogEntry struct {
	ID        int64
	Timestamp time.Time
	Duration  time.Duration
	Command   string
	Client    string
}

// ClientInfo represents a Redis client connection summary.
type ClientInfo struct {
	ID    string
	Addr  string
	Name  string
	Age   string
	Idle  string
	Flags string
	Cmd   string
	DB    string
}

// PubSubChannel represents a PubSub channel with subscriber count
type PubSubChannel struct {
	Channel     string
	Subscribers int64
	Pattern     bool
}

// KeyPattern represents aggregated statistics for a key pattern
type KeyPattern struct {
	Pattern      string
	Count        int
	SampleKeys   []string
	AvgTTL       int64
	Types        map[string]int
	TotalSize    int64
}

// DatabaseInfo represents information about a Redis database
type DatabaseInfo struct {
	ID      int
	Keys    int64
	Expires int64
	AvgTTL  int64
}

// CommandStat represents statistics for a Redis command
type CommandStat struct {
	Command string
	Calls   int64
	Usec    int64
	UsecPerCall float64
}

// LatencyEvent represents a latency event
type LatencyEvent struct {
	Event     string
	Timestamp time.Time
	Latency   int64
}

// NewRedisClient creates a new Redis client
func NewRedisClient() *RedisClient {
	return &RedisClient{
		conn:        nil,
		client:      nil,
		ctx:         context.Background(),
		connected:   false,
		lastRefresh: time.Time{},
	}
}

// Connect connects to a Redis server
func (c *RedisClient) Connect(host, port, password string, db int) error {
	if host == "" {
		return errors.New("host cannot be empty")
	}

	c.conn = &RedisConnection{
		Host:     host,
		Port:     port,
		Password: password,
		Database: db,
	}

	// Create a new Redis client with timeout
	c.client = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", host, port),
		Password:     password,
		DB:           db,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		DialTimeout:  3 * time.Second,
	})

	// Ping the Redis server to check the connection
	ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
	defer cancel()

	_, err := c.client.Ping(ctx).Result()
	if err != nil {
		// Close the client to prevent resource leaks
		c.client.Close()
		c.client = nil
		return fmt.Errorf("failed to connect to Redis: %v", err)
	}

	c.connected = true
	c.lastRefresh = time.Now()

	return nil
}

// ConnectToInstance connects to a preconfigured Redis instance
func (c *RedisClient) ConnectToInstance(instance RedisInstance) error {
	return c.Connect(
		instance.Host,
		strconv.Itoa(instance.Port),
		instance.Password,
		instance.Database,
	)
}

// GetInstancesFromConfig retrieves Redis instances from configuration
func (c *RedisClient) GetInstancesFromConfig() ([]RedisInstance, error) {
	instances, err := GetAvailableInstances()
	if err != nil {
		return nil, fmt.Errorf("failed to get Redis instances from config: %v", err)
	}
	return instances, nil
}

// Disconnect disconnects from the Redis server
func (c *RedisClient) Disconnect() error {
	if !c.connected {
		return errors.New("not connected to any Redis server")
	}

	if c.client != nil {
		if err := c.client.Close(); err != nil {
			return fmt.Errorf("error closing Redis connection: %v", err)
		}
	}

	c.conn = nil
	c.client = nil
	c.connected = false

	return nil
}

// IsConnected returns whether the client is connected
func (c *RedisClient) IsConnected() bool {
	return c.connected && c.client != nil
}

// GetCurrentConnection returns the current connection details
func (c *RedisClient) GetCurrentConnection() *RedisConnection {
	return c.conn
}

// GetLastRefreshTime returns the time of the last refresh
func (c *RedisClient) GetLastRefreshTime() time.Time {
	return c.lastRefresh
}

// SetLastRefreshTime sets the time of the last refresh
func (c *RedisClient) SetLastRefreshTime(t time.Time) {
	c.lastRefresh = t
}

// SelectDB selects a Redis database
func (c *RedisClient) SelectDB(db int) error {
	if !c.connected || c.client == nil {
		return errors.New("not connected to any Redis server")
	}

	// Create a new client with the selected database
	newClient := redis.NewClient(&redis.Options{
		Addr:     c.client.Options().Addr,
		Password: c.client.Options().Password,
		DB:       db,
	})

	// Test the connection
	_, err := newClient.Ping(c.ctx).Result()
	if err != nil {
		newClient.Close()
		return fmt.Errorf("failed to select database %d: %v", db, err)
	}

	// Close the old client and update the current one
	if c.client != nil {
		c.client.Close()
	}

	c.client = newClient
	c.conn.Database = db

	return nil
}

// GetKeys retrieves keys from Redis
func (c *RedisClient) GetKeys(pattern string) ([]string, error) {
	if !c.connected || c.client == nil {
		return nil, errors.New("not connected to any Redis server")
	}

	// Use SCAN to get keys matching the pattern
	var keys []string
	var cursor uint64 = 0

	// Get UI config to limit the number of keys
	uiConfig, configErr := GetUIConfig()
	maxKeys := 1000 // Default max keys
	if configErr == nil {
		maxKeys = uiConfig.MaxKeysDisplay
	}

	for {
		keys_batch, next_cursor, err := c.client.Scan(c.ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("error scanning keys: %v", err)
		}

		keys = append(keys, keys_batch...)
		cursor = next_cursor

		// Stop if we've reached the max keys or the cursor is back to 0
		if cursor == 0 || len(keys) >= maxKeys {
			break
		}
	}

	// Truncate if we got more keys than the max
	if len(keys) > maxKeys {
		keys = keys[:maxKeys]
	}

	return keys, nil
}

// ScanKeys retrieves a page of keys using Redis SCAN.
func (c *RedisClient) ScanKeys(pattern string, cursor uint64, count int64) ([]string, uint64, error) {
	if !c.connected || c.client == nil {
		return nil, 0, errors.New("not connected to any Redis server")
	}
	if count <= 0 {
		count = 100
	}
	keys, nextCursor, err := c.client.Scan(c.ctx, cursor, pattern, count).Result()
	if err != nil {
		return nil, 0, fmt.Errorf("error scanning keys: %v", err)
	}
	return keys, nextCursor, nil
}

// GetKeyInfo gets information about a key
func (c *RedisClient) GetKeyInfo(key string) (map[string]string, error) {
	if !c.connected || c.client == nil {
		return nil, errors.New("not connected to any Redis server")
	}

	if key == "" {
		return nil, errors.New("key cannot be empty")
	}

	// Get key type
	keyType, err := c.client.Type(c.ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get key type: %v", err)
	}

	// Get key TTL
	ttl, err := c.client.TTL(c.ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get key TTL: %v", err)
	}

	// Format TTL
	ttlStr := fmt.Sprintf("%d", int64(ttl.Seconds()))
	if ttl == -1 {
		ttlStr = "No expiration"
	} else if ttl == -2 {
		ttlStr = "Not found"
	}

	// Get size based on key type
	size := "0"
	switch strings.ToLower(keyType) {
	case "string":
		val, err := c.client.Get(c.ctx, key).Result()
		if err == nil {
			size = fmt.Sprintf("%d", len(val))
		}
	case "hash":
		count, err := c.client.HLen(c.ctx, key).Result()
		if err == nil {
			size = fmt.Sprintf("%d fields", count)
		}
	case "list":
		count, err := c.client.LLen(c.ctx, key).Result()
		if err == nil {
			size = fmt.Sprintf("%d items", count)
		}
	case "set":
		count, err := c.client.SCard(c.ctx, key).Result()
		if err == nil {
			size = fmt.Sprintf("%d members", count)
		}
	case "zset":
		count, err := c.client.ZCard(c.ctx, key).Result()
		if err == nil {
			size = fmt.Sprintf("%d members", count)
		}
	}

	return map[string]string{
		"type": keyType,
		"ttl":  ttlStr,
		"size": size,
	}, nil
}

// GetKeyContent gets the content of a key
func (c *RedisClient) GetKeyContent(key string) (string, error) {
	if !c.connected || c.client == nil {
		return "", errors.New("not connected to any Redis server")
	}

	if key == "" {
		return "", errors.New("key cannot be empty")
	}

	// Get key type
	keyType, err := c.client.Type(c.ctx, key).Result()
	if err != nil {
		return "", fmt.Errorf("failed to get key type: %v", err)
	}

	var content strings.Builder

	// Get content based on key type
	switch strings.ToLower(keyType) {
	case "string":
		value, err := c.client.Get(c.ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("failed to get string value: %v", err)
		}
		content.WriteString(value)

	case "hash":
		values, err := c.client.HGetAll(c.ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("failed to get hash values: %v", err)
		}

		i := 1
		for field, value := range values {
			content.WriteString(fmt.Sprintf("%d) \"%s\" => \"%s\"\n", i, field, value))
			i++
		}

	case "list":
		values, err := c.client.LRange(c.ctx, key, 0, -1).Result()
		if err != nil {
			return "", fmt.Errorf("failed to get list values: %v", err)
		}

		for i, value := range values {
			content.WriteString(fmt.Sprintf("%d) \"%s\"\n", i+1, value))
		}

	case "set":
		values, err := c.client.SMembers(c.ctx, key).Result()
		if err != nil {
			return "", fmt.Errorf("failed to get set members: %v", err)
		}

		for i, value := range values {
			content.WriteString(fmt.Sprintf("%d) \"%s\"\n", i+1, value))
		}

	case "zset":
		values, err := c.client.ZRangeWithScores(c.ctx, key, 0, -1).Result()
		if err != nil {
			return "", fmt.Errorf("failed to get sorted set values: %v", err)
		}

		for i, z := range values {
			content.WriteString(fmt.Sprintf("%d) \"%v\" [score: %v]\n", i+1, z.Member, z.Score))
		}

	default:
		return fmt.Sprintf("Unknown key type: %s", keyType), nil
	}

	return content.String(), nil
}

// DeleteKey deletes a key
func (c *RedisClient) DeleteKey(key string) error {
	if !c.connected || c.client == nil {
		return errors.New("not connected to any Redis server")
	}

	if key == "" {
		return errors.New("key cannot be empty")
	}

	result, err := c.client.Del(c.ctx, key).Result()
	if err != nil {
		return fmt.Errorf("failed to delete key: %v", err)
	}

	if result == 0 {
		return errors.New("key not found")
	}

	return nil
}

// FlushDB deletes all keys in the current database
func (c *RedisClient) FlushDB() error {
	if !c.connected || c.client == nil {
		return errors.New("not connected to any Redis server")
	}

	_, err := c.client.FlushDB(c.ctx).Result()
	if err != nil {
		return fmt.Errorf("failed to flush database: %v", err)
	}

	return nil
}

// SetKey sets a key with a value
func (c *RedisClient) SetKey(key, value string, ttl int64) error {
	if !c.connected || c.client == nil {
		return errors.New("not connected to any Redis server")
	}

	if key == "" {
		return errors.New("key cannot be empty")
	}

	var expiration time.Duration
	if ttl < 0 {
		expiration = 0 // No expiration
	} else {
		expiration = time.Duration(ttl) * time.Second
	}

	_, err := c.client.Set(c.ctx, key, value, expiration).Result()
	if err != nil {
		return fmt.Errorf("failed to set key: %v", err)
	}

	return nil
}

// GetServerInfo gets information about the Redis server
func (c *RedisClient) GetServerInfo() (map[string]string, error) {
	if !c.connected || c.client == nil {
		return nil, errors.New("not connected to any Redis server")
	}

	infoMap, err := c.GetInfoMap()
	if err != nil {
		return nil, err
	}

	// Extract the most important fields
	result := map[string]string{
		"redis_version":     infoMap["redis_version"],
		"uptime_in_days":    infoMap["uptime_in_days"],
		"connected_clients": infoMap["connected_clients"],
		"used_memory_human": infoMap["used_memory_human"],
	}

	return result, nil
}

// GetInfoRaw returns the raw INFO response.
func (c *RedisClient) GetInfoRaw() (string, error) {
	if !c.connected || c.client == nil {
		return "", errors.New("not connected to any Redis server")
	}

	info, err := c.client.Info(c.ctx).Result()
	if err != nil {
		return "", fmt.Errorf("failed to get server info: %v", err)
	}

	return info, nil
}

// GetInfoMap retrieves the Redis INFO data as a map.
func (c *RedisClient) GetInfoMap() (map[string]string, error) {
	if !c.connected || c.client == nil {
		return nil, errors.New("not connected to any Redis server")
	}

	info, err := c.GetInfoRaw()
	if err != nil {
		return nil, err
	}

	infoMap := make(map[string]string)
	lines := strings.Split(info, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			infoMap[parts[0]] = parts[1]
		}
	}

	return infoMap, nil
}

// GetInfoSectionMap returns INFO data for a specific section (e.g. replication, persistence).
func (c *RedisClient) GetInfoSectionMap(section string) (map[string]string, error) {
	info, err := c.GetInfoRaw()
	if err != nil {
		return nil, err
	}

	section = strings.ToLower(strings.TrimSpace(section))
	infoMap := make(map[string]string)
	lines := strings.Split(info, "\n")
	inSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "#") {
			sectionName := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "#")))
			inSection = sectionName == section
			continue
		}

		if !inSection {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			infoMap[parts[0]] = parts[1]
		}
	}

	return infoMap, nil
}

// GetSlowLog returns recent slowlog entries.
func (c *RedisClient) GetSlowLog(limit int64) ([]SlowLogEntry, error) {
	if !c.connected || c.client == nil {
		return nil, errors.New("not connected to any Redis server")
	}

	logs, err := c.client.SlowLogGet(c.ctx, limit).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get slowlog: %v", err)
	}

	entries := make([]SlowLogEntry, 0, len(logs))
	for _, entry := range logs {
		command := strings.TrimSpace(strings.Join(entry.Args, " "))
		client := entry.ClientAddr
		if entry.ClientName != "" {
			client = fmt.Sprintf("%s (%s)", entry.ClientAddr, entry.ClientName)
		}
		entries = append(entries, SlowLogEntry{
			ID:        entry.ID,
			Timestamp: entry.Time,
			Duration:  entry.Duration,
			Command:   command,
			Client:    client,
		})
	}

	return entries, nil
}

// GetClients returns a list of connected clients.
func (c *RedisClient) GetClients() ([]ClientInfo, error) {
	if !c.connected || c.client == nil {
		return nil, errors.New("not connected to any Redis server")
	}

	raw, err := c.client.ClientList(c.ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get client list: %v", err)
	}

	clients := make([]ClientInfo, 0)
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		entry := ClientInfo{}
		for _, field := range fields {
			parts := strings.SplitN(field, "=", 2)
			if len(parts) != 2 {
				continue
			}
			switch parts[0] {
			case "id":
				entry.ID = parts[1]
			case "addr":
				entry.Addr = parts[1]
			case "name":
				entry.Name = parts[1]
			case "age":
				entry.Age = parts[1]
			case "idle":
				entry.Idle = parts[1]
			case "flags":
				entry.Flags = parts[1]
			case "cmd":
				entry.Cmd = parts[1]
			case "db":
				entry.DB = parts[1]
			}
		}

		clients = append(clients, entry)
	}

	return clients, nil
}

// GetConfig returns Redis config values matching a pattern.
func (c *RedisClient) GetConfig(pattern string) (map[string]string, error) {
	if !c.connected || c.client == nil {
		return nil, errors.New("not connected to any Redis server")
	}

	if pattern == "" {
		pattern = "*"
	}

	values, err := c.client.ConfigGet(c.ctx, pattern).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get config: %v", err)
	}

	config := make(map[string]string)
	if len(values) == 0 {
		return config, nil
	}

	// go-redis returns map[string]string for ConfigGet.
	for key, value := range values {
		config[key] = value
	}

	return config, nil
}

// GetMemoryStats returns MEMORY STATS as a map.
func (c *RedisClient) GetMemoryStats() (map[string]string, error) {
	if !c.connected || c.client == nil {
		return nil, errors.New("not connected to any Redis server")
	}

	values, err := c.client.Do(c.ctx, "MEMORY", "STATS").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory stats: %v", err)
	}

	stats := make(map[string]string)
	list, ok := values.([]interface{})
	if !ok {
		return stats, nil
	}

	for i := 0; i+1 < len(list); i += 2 {
		key, ok := list[i].(string)
		if !ok {
			continue
		}
		stats[key] = fmt.Sprintf("%v", list[i+1])
	}

	return stats, nil
}

// GetMemoryDoctor returns MEMORY DOCTOR output.
func (c *RedisClient) GetMemoryDoctor() (string, error) {
	if !c.connected || c.client == nil {
		return "", errors.New("not connected to any Redis server")
	}

	result, err := c.client.Do(c.ctx, "MEMORY", "DOCTOR").Result()
	if err != nil {
		return "", fmt.Errorf("failed to run memory doctor: %v", err)
	}

	switch v := result.(type) {
	case string:
		return v, nil
	default:
		return fmt.Sprintf("%v", result), nil
	}
}

// GetPubSubChannels returns all active PubSub channels with subscriber counts
func (c *RedisClient) GetPubSubChannels() ([]PubSubChannel, error) {
	if !c.connected || c.client == nil {
		return nil, errors.New("not connected to any Redis server")
	}

	// Get all channels
	channels, err := c.client.PubSubChannels(c.ctx, "*").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get pubsub channels: %v", err)
	}

	result := make([]PubSubChannel, 0, len(channels))
	
	// Get subscriber counts for each channel
	if len(channels) > 0 {
		numSub, err := c.client.PubSubNumSub(c.ctx, channels...).Result()
		if err == nil {
			for channel, count := range numSub {
				result = append(result, PubSubChannel{
					Channel:     channel,
					Subscribers: count,
					Pattern:     false,
				})
			}
		}
	}

	// Get pattern subscriptions
	patterns, err := c.client.PubSubNumPat(c.ctx).Result()
	if err == nil && patterns > 0 {
		result = append(result, PubSubChannel{
			Channel:     "*",
			Subscribers: patterns,
			Pattern:     true,
		})
	}

	return result, nil
}

// AnalyzeKeyPatterns analyzes keys and groups them by pattern prefix
func (c *RedisClient) AnalyzeKeyPatterns(maxKeys int) ([]KeyPattern, error) {
	if !c.connected || c.client == nil {
		return nil, errors.New("not connected to any Redis server")
	}

	if maxKeys <= 0 {
		maxKeys = 1000
	}

	// Scan keys
	keys, err := c.GetKeys("*")
	if err != nil {
		return nil, err
	}

	if len(keys) > maxKeys {
		keys = keys[:maxKeys]
	}

	// Group keys by pattern (prefix before first colon)
	patterns := make(map[string]*KeyPattern)
	
	for _, key := range keys {
		// Extract pattern (prefix before first colon or full key if no colon)
		pattern := key
		if idx := strings.Index(key, ":"); idx > 0 {
			pattern = key[:idx] + ":*"
		}

		if _, exists := patterns[pattern]; !exists {
			patterns[pattern] = &KeyPattern{
				Pattern:    pattern,
				Count:      0,
				SampleKeys: make([]string, 0, 3),
				Types:      make(map[string]int),
			}
		}

		p := patterns[pattern]
		p.Count++

		// Add sample keys (max 3)
		if len(p.SampleKeys) < 3 {
			p.SampleKeys = append(p.SampleKeys, key)
		}

		// Get key type
		keyType, err := c.client.Type(c.ctx, key).Result()
		if err == nil {
			p.Types[keyType]++
		}

		// Get TTL for average calculation
		ttl, err := c.client.TTL(c.ctx, key).Result()
		if err == nil && ttl > 0 {
			p.AvgTTL += int64(ttl.Seconds())
		}
	}

	// Convert map to slice and calculate averages
	result := make([]KeyPattern, 0, len(patterns))
	for _, p := range patterns {
		if p.Count > 0 && p.AvgTTL > 0 {
			p.AvgTTL = p.AvgTTL / int64(p.Count)
		}
		result = append(result, *p)
	}

	return result, nil
}

// GetAllDatabases returns information about all databases
func (c *RedisClient) GetAllDatabases() ([]DatabaseInfo, error) {
	if !c.connected || c.client == nil {
		return nil, errors.New("not connected to any Redis server")
	}

	infoMap, err := c.GetInfoMap()
	if err != nil {
		return nil, err
	}

	databases := make([]DatabaseInfo, 0)
	
	// Parse keyspace section for database info
	for key, value := range infoMap {
		if strings.HasPrefix(key, "db") {
			dbNum, err := strconv.Atoi(strings.TrimPrefix(key, "db"))
			if err != nil {
				continue
			}

			dbInfo := DatabaseInfo{ID: dbNum}
			
			// Parse the value: "keys=123,expires=45,avg_ttl=67890"
			parts := strings.Split(value, ",")
			for _, part := range parts {
				kv := strings.SplitN(part, "=", 2)
				if len(kv) != 2 {
					continue
				}
				
				switch kv[0] {
				case "keys":
					if val, err := strconv.ParseInt(kv[1], 10, 64); err == nil {
						dbInfo.Keys = val
					}
				case "expires":
					if val, err := strconv.ParseInt(kv[1], 10, 64); err == nil {
						dbInfo.Expires = val
					}
				case "avg_ttl":
					if val, err := strconv.ParseInt(kv[1], 10, 64); err == nil {
						dbInfo.AvgTTL = val / 1000 // Convert to seconds
					}
				}
			}
			
			databases = append(databases, dbInfo)
		}
	}

	return databases, nil
}

// GetCommandStats returns statistics for all Redis commands
func (c *RedisClient) GetCommandStats() ([]CommandStat, error) {
	if !c.connected || c.client == nil {
		return nil, errors.New("not connected to any Redis server")
	}

	infoMap, err := c.GetInfoMap()
	if err != nil {
		return nil, err
	}

	stats := make([]CommandStat, 0)
	
	// Parse commandstats section
	for key, value := range infoMap {
		if strings.HasPrefix(key, "cmdstat_") {
			cmdName := strings.TrimPrefix(key, "cmdstat_")
			
			stat := CommandStat{Command: cmdName}
			
			// Parse value: "calls=123,usec=456,usec_per_call=3.71"
			parts := strings.Split(value, ",")
			for _, part := range parts {
				kv := strings.SplitN(part, "=", 2)
				if len(kv) != 2 {
					continue
				}
				
				switch kv[0] {
				case "calls":
					if val, err := strconv.ParseInt(kv[1], 10, 64); err == nil {
						stat.Calls = val
					}
				case "usec":
					if val, err := strconv.ParseInt(kv[1], 10, 64); err == nil {
						stat.Usec = val
					}
				case "usec_per_call":
					if val, err := strconv.ParseFloat(kv[1], 64); err == nil {
						stat.UsecPerCall = val
					}
				}
			}
			
			stats = append(stats, stat)
		}
	}

	return stats, nil
}

// GetLatencyHistory returns latency history for all events
func (c *RedisClient) GetLatencyHistory() ([]LatencyEvent, error) {
	if !c.connected || c.client == nil {
		return nil, errors.New("not connected to any Redis server")
	}

	// Get latest latency events
	result, err := c.client.Do(c.ctx, "LATENCY", "LATEST").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get latency history: %v", err)
	}

	events := make([]LatencyEvent, 0)
	
	// Parse result
	list, ok := result.([]interface{})
	if !ok {
		return events, nil
	}

	for _, item := range list {
		eventData, ok := item.([]interface{})
		if !ok || len(eventData) < 3 {
			continue
		}

		event := LatencyEvent{}
		
		if name, ok := eventData[0].(string); ok {
			event.Event = name
		}
		
		if ts, ok := eventData[1].(int64); ok {
			event.Timestamp = time.Unix(ts, 0)
		}
		
		if latency, ok := eventData[2].(int64); ok {
			event.Latency = latency
		}
		
		events = append(events, event)
	}

	return events, nil
}

