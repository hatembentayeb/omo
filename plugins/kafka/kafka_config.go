package main

import (
	"fmt"
	"os"
	"path/filepath"

	"omo/pkg/pluginapi"

	"gopkg.in/yaml.v3"
)

// kafkaConfigHeader is prepended to the auto-generated config YAML.
const kafkaConfigHeader = `# Kafka Plugin Configuration
# Path: ~/.omo/configs/kafka/kafka.yaml
#
# KeePass Secret Schema (secret path: kafka/<environment>/<name>):
#   Title    → instance name
#   URL      → bootstrap servers (e.g. "broker1:9092,broker2:9092")
#   UserName → SASL username
#   Password → SASL password
#   Custom Attributes:
#     ssl_ca_cert → path to SSL CA certificate
#     ssl_cert    → path to SSL client certificate
#     ssl_key     → path to SSL client key
#
# When "secret" is set, connection fields are resolved from KeePass.
# YAML values take precedence over KeePass values (override only blanks).
`

// KafkaConfig represents the configuration for the Kafka plugin
type KafkaConfig struct {
	Instances []KafkaInstance `yaml:"instances"`
	UI        KafkaUIConfig   `yaml:"ui"`
}

// KafkaInstance represents a configured Kafka cluster.
// When the Secret field is set (e.g. "kafka/production/main-cluster"),
// it references a KeePass entry whose fields override BootstrapServers,
// Username, Password, etc. at load time.
type KafkaInstance struct {
	Name             string        `yaml:"name"`
	Description      string        `yaml:"description"`
	Secret           string        `yaml:"secret,omitempty"` // KeePass path: pluginName/env/entryName
	BootstrapServers string        `yaml:"bootstrap_servers"`
	Security         KafkaSecurity `yaml:"security"`
}

// KafkaSecurity represents security configuration for a Kafka cluster
type KafkaSecurity struct {
	EnableSASL    bool   `yaml:"enable_sasl"`
	SASLMechanism string `yaml:"sasl_mechanism"`
	Username      string `yaml:"username"`
	Password      string `yaml:"password"`
	EnableSSL     bool   `yaml:"enable_ssl"`
	SSLCACert     string `yaml:"ssl_ca_cert"`
	SSLCert       string `yaml:"ssl_cert"`
	SSLKey        string `yaml:"ssl_key"`
}

// KafkaUIConfig represents UI configuration options
type KafkaUIConfig struct {
	RefreshInterval      int    `yaml:"refresh_interval"`
	MaxTopicsDisplay     int    `yaml:"max_topics_display"`
	MaxPartitionsDisplay int    `yaml:"max_partitions_display"`
	DefaultView          string `yaml:"default_view"`
	EnableMetrics        bool   `yaml:"enable_metrics"`
	EnableConsumerGroups bool   `yaml:"enable_consumer_groups"`
	EnableMessageViewer  bool   `yaml:"enable_message_viewer"`
}

// DefaultKafkaConfig returns the default configuration
func DefaultKafkaConfig() *KafkaConfig {
	return &KafkaConfig{
		Instances: []KafkaInstance{},
		UI: KafkaUIConfig{
			RefreshInterval:      10,
			MaxTopicsDisplay:     100,
			MaxPartitionsDisplay: 50,
			DefaultView:          "brokers",
			EnableMetrics:        true,
			EnableConsumerGroups: true,
			EnableMessageViewer:  true,
		},
	}
}

// LoadKafkaConfig loads the Kafka configuration from the specified file.
// Default path: ~/.omo/configs/kafka/kafka.yaml
//
// After unmarshalling, any instance with a non-empty Secret field will
// have its connection fields resolved from the KeePass secrets provider.
func LoadKafkaConfig(configPath string) (*KafkaConfig, error) {
	if configPath == "" {
		configPath = pluginapi.PluginConfigPath("kafka")
	}

	// Auto-create default config if missing
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		_ = writeDefaultConfig(configPath, kafkaConfigHeader, DefaultKafkaConfig())
	}

	// Read the configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("error reading config file: %v", err)
	}

	// Unmarshal the configuration
	config := DefaultKafkaConfig()
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file: %v", err)
	}

	// Init KeePass placeholder if no entries exist yet
	initKafkaKeePass()

	// Resolve secrets for instances that reference KeePass entries.
	if err := resolveKafkaSecrets(config); err != nil {
		return nil, fmt.Errorf("error resolving secrets: %v", err)
	}

	return config, nil
}

// initKafkaKeePass seeds placeholder KeePass entries for Kafka if none exist.
func initKafkaKeePass() {
	if !pluginapi.HasSecrets() {
		return
	}
	entries, err := pluginapi.Secrets().List("kafka")
	if err != nil || len(entries) > 0 {
		return
	}
	_ = pluginapi.Secrets().Put("kafka/default/example", &pluginapi.SecretEntry{
		Title:    "example",
		UserName: "",
		Password: "",
		URL:      "localhost:9092",
		Notes:    "Kafka placeholder. Set URL (bootstrap servers), UserName/Password (SASL).",
		CustomAttributes: map[string]string{
			"ssl_ca_cert": "",
			"ssl_cert":    "",
			"ssl_key":     "",
		},
	})
}

func resolveKafkaInstanceSecret(inst *KafkaInstance, entry *pluginapi.SecretEntry) {
	if inst.BootstrapServers == "" && entry.URL != "" {
		inst.BootstrapServers = entry.URL
	}
	if inst.Security.Username == "" && entry.UserName != "" {
		inst.Security.Username = entry.UserName
	}
	if inst.Security.Password == "" && entry.Password != "" {
		inst.Security.Password = entry.Password
	}
	if inst.Name == "" && entry.Title != "" {
		inst.Name = entry.Title
	}
	for attr, field := range map[string]*string{
		"ssl_ca_cert": &inst.Security.SSLCACert,
		"ssl_cert":    &inst.Security.SSLCert,
		"ssl_key":     &inst.Security.SSLKey,
	} {
		if *field == "" {
			if v, ok := entry.CustomAttributes[attr]; ok {
				*field = v
			}
		}
	}
}

// resolveKafkaSecrets iterates over instances and populates connection
// fields from the secrets provider when a secret path is defined.
func resolveKafkaSecrets(config *KafkaConfig) error {
	if !pluginapi.HasSecrets() {
		return nil
	}

	for i := range config.Instances {
		inst := &config.Instances[i]
		if inst.Secret == "" {
			continue
		}

		entry, err := pluginapi.ResolveSecret(inst.Secret)
		if err != nil {
			return fmt.Errorf("instance %q: %w", inst.Name, err)
		}

		resolveKafkaInstanceSecret(inst, entry)
	}
	return nil
}

// GetAvailableKafkaInstances returns the list of configured Kafka instances
func GetAvailableKafkaInstances() ([]KafkaInstance, error) {
	config, err := LoadKafkaConfig("")
	if err != nil {
		return nil, err
	}
	return config.Instances, nil
}

// GetKafkaUIConfig returns the UI configuration
func GetKafkaUIConfig() (KafkaUIConfig, error) {
	config, err := LoadKafkaConfig("")
	if err != nil {
		return KafkaUIConfig{}, err
	}
	return config.UI, nil
}

// writeDefaultConfig marshals the default config struct to YAML and writes it to disk.
func writeDefaultConfig(configPath, header string, cfg interface{}) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, []byte(header+"\n"+string(data)), 0644)
}
