package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"omo/pkg/pluginapi"
)

var defaultRabbitMQEnvironments = []string{
	"development",
	"production",
	"staging",
	"sandbox",
	"local",
	"test",
}

// RabbitMQInstance is built entirely from a KeePass entry at runtime.
//
// KeePass Entry Schema (path: rabbitmq/<environment>/<name>):
//
//	Title    → instance display name
//	URL      → hostname or IP address
//	UserName → RabbitMQ username
//	Password → RabbitMQ password
//	Notes    → description / notes
//
//	Custom Attributes:
//	  amqp_port → AMQP port (default: 5672)
//	  mgmt_port → Management API port (default: 15672)
//	  vhost     → virtual host (default: "/")
//	  use_tls   → "true" or "false" (default: false)
//	  tags      → comma-separated tags (e.g. "prod,cluster-a")
type RabbitMQInstance struct {
	Name        string
	Description string
	Environment string
	Host        string
	AMQPPort    int
	MgmtPort    int
	Username    string
	Password    string
	VHost       string
	UseTLS      bool
	Tags        []string
}

// RabbitMQUIConfig holds hardcoded UI defaults for the RabbitMQ plugin.
type RabbitMQUIConfig struct {
	RefreshInterval   int
	MaxQueuesDisplay  int
	EnableConnections bool
	EnableChannels    bool
	DefaultView       string
}

func DefaultRabbitMQUIConfig() RabbitMQUIConfig {
	return RabbitMQUIConfig{
		RefreshInterval:   10,
		MaxQueuesDisplay:  200,
		EnableConnections: true,
		EnableChannels:    true,
		DefaultView:       "overview",
	}
}

// DiscoverInstances reads KeePass groups under "rabbitmq/" and builds
// RabbitMQInstance objects from the entries.
func DiscoverInstances() ([]RabbitMQInstance, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	ensureRabbitMQKeePassGroups()

	paths, err := pluginapi.Secrets().List("rabbitmq")
	if err != nil {
		return nil, fmt.Errorf("list rabbitmq secrets: %w", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no RabbitMQ entries in KeePass (create entries under rabbitmq/<environment>/<name>)")
	}

	var instances []RabbitMQInstance
	for _, path := range paths {
		env := extractRabbitMQEnvironment(path)
		if env == "" {
			continue
		}

		entry, err := pluginapi.Secrets().Get(path)
		if err != nil {
			continue
		}

		inst := entryToRabbitMQInstance(entry, env)
		instances = append(instances, inst)
	}

	sort.Slice(instances, func(i, j int) bool {
		if instances[i].Environment != instances[j].Environment {
			return instances[i].Environment < instances[j].Environment
		}
		return instances[i].Name < instances[j].Name
	})

	return instances, nil
}

func extractRabbitMQEnvironment(path string) string {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

func entryToRabbitMQInstance(entry *pluginapi.SecretEntry, env string) RabbitMQInstance {
	inst := RabbitMQInstance{
		Name:        entry.Title,
		Description: entry.Notes,
		Environment: env,
		Host:        entry.URL,
		Username:    entry.UserName,
		Password:    entry.Password,
		AMQPPort:    5672,
		MgmtPort:    15672,
		VHost:       "/",
	}

	if entry.CustomAttributes == nil {
		return inst
	}

	ca := entry.CustomAttributes

	if v, ok := ca["amqp_port"]; ok {
		if p, err := strconv.Atoi(v); err == nil {
			inst.AMQPPort = p
		}
	}
	if v, ok := ca["mgmt_port"]; ok {
		if p, err := strconv.Atoi(v); err == nil {
			inst.MgmtPort = p
		}
	}
	if v, ok := ca["vhost"]; ok && v != "" {
		inst.VHost = v
	}
	if v, ok := ca["use_tls"]; ok {
		inst.UseTLS = v == "true" || v == "1" || v == "yes"
	}
	if v, ok := ca["tags"]; ok {
		for _, t := range strings.Split(v, ",") {
			t = strings.TrimSpace(t)
			if t != "" {
				inst.Tags = append(inst.Tags, t)
			}
		}
	}

	return inst
}

// ensureRabbitMQKeePassGroups creates environment groups in KeePass
// with placeholder entries so the folder structure is visible in KeePassXC.
func ensureRabbitMQKeePassGroups() {
	if !pluginapi.HasSecrets() {
		return
	}

	requiredAttrs := map[string]string{
		"amqp_port": "5672",
		"mgmt_port": "15672",
		"vhost":     "/",
		"use_tls":   "false",
		"tags":      "",
	}

	for _, env := range defaultRabbitMQEnvironments {
		prefix := fmt.Sprintf("rabbitmq/%s", env)
		existing, err := pluginapi.Secrets().List(prefix)
		if err == nil && len(existing) > 0 {
			backfillAttributes(existing, requiredAttrs)
			continue
		}
		path := fmt.Sprintf("rabbitmq/%s/example", env)
		_ = pluginapi.Secrets().Put(path, &pluginapi.SecretEntry{
			Title:    "example",
			UserName: "guest",
			Password: "guest",
			URL:      "localhost",
			Notes:    fmt.Sprintf("RabbitMQ %s placeholder. Replace with real RabbitMQ details.", env),
			CustomAttributes: requiredAttrs,
		})
	}
}

func backfillAttributes(entryPaths []string, required map[string]string) {
	for _, entryPath := range entryPaths {
		entry, err := pluginapi.Secrets().Get(entryPath)
		if err != nil || entry == nil {
			continue
		}
		if entry.CustomAttributes == nil {
			entry.CustomAttributes = make(map[string]string)
		}
		updated := false
		for attr, defaultVal := range required {
			if _, exists := entry.CustomAttributes[attr]; !exists {
				entry.CustomAttributes[attr] = defaultVal
				updated = true
			}
		}
		if updated {
			_ = pluginapi.Secrets().Put(entryPath, entry)
		}
	}
}

// GetRabbitMQUIConfig returns the hardcoded UI configuration.
func GetRabbitMQUIConfig() (RabbitMQUIConfig, error) {
	return DefaultRabbitMQUIConfig(), nil
}
