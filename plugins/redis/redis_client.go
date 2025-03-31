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

	info, err := c.client.Info(c.ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get server info: %v", err)
	}

	// Parse the INFO response into a map
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

	// Extract the most important fields
	result := map[string]string{
		"redis_version":     infoMap["redis_version"],
		"uptime_in_days":    infoMap["uptime_in_days"],
		"connected_clients": infoMap["connected_clients"],
		"used_memory_human": infoMap["used_memory_human"],
	}

	return result, nil
}
