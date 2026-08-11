package k8sportforward

import (
	"strings"
)

// classifyWorkload returns a coarse type used to group databases vs apps.
func classifyWorkload(name string, labels map[string]string) string {
	hay := strings.ToLower(name)
	for k, v := range labels {
		hay += " " + strings.ToLower(k) + "=" + strings.ToLower(v)
	}

	checks := []struct {
		kind string
		keys []string
	}{
		{"database", []string{"postgres", "postgresql", "mysql", "mariadb", "mongo", "mongodb", "redis", "valkey", "cassandra", "cockroach", "elasticsearch", "opensearch", "clickhouse", "timescale", "neo4j", "influx"}},
		{"queue", []string{"kafka", "rabbitmq", "rabbit", "nats", "pulsar", "activemq", "sqs"}},
		{"cache", []string{"memcached", "varnish", "keydb"}},
		{"search", []string{"solr", "meilisearch", "typesense"}},
		{"observability", []string{"prometheus", "grafana", "loki", "tempo", "jaeger", "otel", "opentelemetry", "thanos"}},
	}
	for _, c := range checks {
		for _, k := range c.keys {
			if strings.Contains(hay, k) {
				return c.kind
			}
		}
	}
	if labels["app.kubernetes.io/component"] != "" {
		comp := strings.ToLower(labels["app.kubernetes.io/component"])
		if strings.Contains(comp, "database") || strings.Contains(comp, "db") {
			return "database"
		}
	}
	return "app"
}
