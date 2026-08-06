package kafka

import (
	"fmt"
	"sort"
	"strings"

	"omo/pkg/pluginapi"
)

// KafkaInstance is built entirely from a KeePass entry at runtime.
//
// KeePass Entry Schema (path: kafka/<environment>/<name>):
//
//	Title    → instance display name
//	URL      → bootstrap servers (e.g. "broker1:9092,broker2:9092")
//	UserName → SASL username
//	Password → SASL password
//	Notes    → description / notes
//
//	Custom Attributes:
//	  sasl_mechanism → SASL mechanism (e.g. "PLAIN", "SCRAM-SHA-256")
//	  enable_sasl    → "true" to enable SASL auth (default: false)
//	  enable_ssl     → "true" to enable SSL (default: false)
//	  ssl_ca_cert    → path to SSL CA certificate
//	  ssl_cert       → path to SSL client certificate
//	  ssl_key        → path to SSL client key
//	  tags           → comma-separated tags
type KafkaInstance struct {
	Name             string
	Description      string
	Environment      string
	BootstrapServers string
	Security         KafkaSecurity
	Tags             []string
}

type KafkaSecurity struct {
	EnableSASL    bool
	SASLMechanism string
	Username      string
	Password      string
	EnableSSL     bool
	SSLCACert     string
	SSLCert       string
	SSLKey        string
}

// KafkaUIConfig holds hardcoded UI defaults for the Kafka plugin.
type KafkaUIConfig struct {
	RefreshInterval      int
	MaxTopicsDisplay     int
	MaxPartitionsDisplay int
	DefaultView          string
	EnableMetrics        bool
	EnableConsumerGroups bool
	EnableMessageViewer  bool
}

func DefaultKafkaUIConfig() KafkaUIConfig {
	return KafkaUIConfig{
		RefreshInterval:      10,
		MaxTopicsDisplay:     100,
		MaxPartitionsDisplay: 50,
		DefaultView:          "brokers",
		EnableMetrics:        true,
		EnableConsumerGroups: true,
		EnableMessageViewer:  true,
	}
}

// DiscoverInstances reads KeePass groups under "kafka/" and builds
// KafkaInstance objects from the entries.
func DiscoverInstances() ([]KafkaInstance, error) {
	if !pluginapi.HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}

	if err := pluginapi.Secrets().Reload(); err != nil {
		return nil, fmt.Errorf("reload secrets: %w", err)
	}

	paths, err := pluginapi.ListNonReferenceSecrets("kafka")
	if err != nil {
		return nil, fmt.Errorf("list kafka secrets: %w", err)
	}

	if len(paths) == 0 {
		return nil, fmt.Errorf("no Kafka entries in KeePass (create entries under kafka/<environment>/<name>)")
	}

	var instances []KafkaInstance
	for _, path := range paths {
		env := extractEnvironment(path)
		if env == "" {
			continue
		}

		entry, err := pluginapi.Secrets().Get(path)
		if err != nil {
			continue
		}

		inst := entryToKafkaInstance(entry, env)
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

func extractEnvironment(path string) string {
	parts := strings.SplitN(path, "/", 3)
	if len(parts) < 3 {
		return ""
	}
	return parts[1]
}

func entryToKafkaInstance(entry *pluginapi.SecretEntry, env string) KafkaInstance {
	inst := KafkaInstance{
		Name:             entry.Title,
		Description:      entry.Notes,
		Environment:      env,
		BootstrapServers: entry.URL,
		Security: KafkaSecurity{
			Username: entry.UserName,
			Password: entry.Password,
		},
	}

	if entry.CustomAttributes == nil {
		return inst
	}

	ca := entry.CustomAttributes

	if v, ok := ca["sasl_mechanism"]; ok {
		inst.Security.SASLMechanism = v
	}
	if v, ok := ca["enable_sasl"]; ok {
		inst.Security.EnableSASL = v == "true" || v == "1" || v == "yes"
	}
	if v, ok := ca["enable_ssl"]; ok {
		inst.Security.EnableSSL = v == "true" || v == "1" || v == "yes"
	}
	if v, ok := ca["ssl_ca_cert"]; ok {
		inst.Security.SSLCACert = v
	}
	if v, ok := ca["ssl_cert"]; ok {
		inst.Security.SSLCert = v
	}
	if v, ok := ca["ssl_key"]; ok {
		inst.Security.SSLKey = v
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

// GetAvailableKafkaInstances returns all discovered Kafka instances.
func GetAvailableKafkaInstances() ([]KafkaInstance, error) {
	return DiscoverInstances()
}

// GetKafkaUIConfig returns the hardcoded UI configuration.
func GetKafkaUIConfig() (KafkaUIConfig, error) {
	return DefaultKafkaUIConfig(), nil
}
