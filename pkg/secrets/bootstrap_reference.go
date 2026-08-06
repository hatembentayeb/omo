package secrets

import (
	"errors"
	"fmt"
	"os"

	"omo/pkg/pluginapi"
)

const (
	referenceEnv      = "default"
	referenceInstance = "default_config"
)

// pluginReferenceDefinitions drive KeePass paths:
//
//	<plugin>/default/default_config
//
// Entries use empty credentials and Notes as field reference only (no sample hosts or passwords).
var pluginReferenceDefinitions = []struct {
	plugin string
	notes  string
}{
	{
		plugin: "docker",
		notes: "Reference path: docker/<environment>/<instance>\n\n" +
			"URL — Docker host (unix socket or tcp)\n" +
			"Custom: cert_path, tls, tls_verify, tags",
	},
	{
		plugin: "redis",
		notes: "Reference path: redis/<environment>/<instance>\n\n" +
			"URL — host\nUserName — ACL user (optional)\nPassword — auth (optional)\n" +
			"Custom: port, database, tags",
	},
	{
		plugin: "kafka",
		notes: "Reference path: kafka/<environment>/<instance>\n\n" +
			"URL — bootstrap servers (comma-separated)\nUserName / Password — SASL (optional)\n" +
			"Custom: sasl_mechanism, enable_sasl, enable_ssl, ssl_ca_cert, ssl_cert, ssl_key, tags",
	},
	{
		plugin: "rabbitmq",
		notes: "Reference path: rabbitmq/<environment>/<instance>\n\n" +
			"URL — host\nUserName / Password\n" +
			"Custom: amqp_port, mgmt_port, vhost, use_tls, tags",
	},
	{
		plugin: "postgres",
		notes: "Reference path: postgres/<environment>/<instance>\n\n" +
			"URL — host\nUserName / Password\n" +
			"Custom: port, database, sslmode, tags",
	},
	{
		plugin: "ssh",
		notes: "Reference path: ssh/<environment>/<instance>\n\n" +
			"URL — host or IP\nUserName / Password (if password auth)\n" +
			"Custom: port, auth_method, private_key, key_path, passphrase, proxy_command, jump_host, jump_key, jump_key_path, fingerprint, tags, startup_cmd, keep_alive, env_*",
	},
	{
		plugin: "argocd",
		notes: "Reference path: argocd/<environment>/<instance>\n\n" +
			"URL — Argo CD server\nUserName / Password — or use auth_token in Custom\n" +
			"Custom: auth_token, insecure, kubeconfig, kubeconfig_path, namespace, tags",
	},
	{
		plugin: "k8suser",
		notes: "Reference path: k8suser/<environment>/<instance>\n\n" +
			"URL — API server\nPassword — bearer token (if used)\n" +
			"Custom: kubeconfig, context, ca_cert, tags",
	},
	{
		plugin: "awsCosts",
		notes: "Reference path: awsCosts/<environment>/<instance>\n\n" +
			"URL — region\nUserName — access key ID\nPassword — secret key\n" +
			"Custom: role_arn, tags",
	},
	{
		plugin: "s3",
		notes: "Reference path: s3/<environment>/<instance>\n\n" +
			"URL — endpoint\nUserName — access key ID\nPassword — secret key\n" +
			"Custom: region, role_arn, tags",
	},
	{
		plugin: "git",
		notes: "Reference path: git/<environment>/<instance>\n\n" +
			"URL — remote\nUserName / Password (token)\n" +
			"Custom: path (local repo path), tags",
	},
	{
		plugin: "github",
		notes: "Reference path: github/<environment>/<instance>\n\n" +
			"Password — personal access token\nUserName — org name when type=org\nURL — API base (empty = github.com API)\n" +
			"Custom: type (user|org)",
	},
}

// ensureReferenceTemplates creates <plugin>/default/default_config when missing.
// If an entry already exists at that path (including user-created), it is left unchanged.
func (kp *KeePassProvider) ensureReferenceTemplates() error {
	for _, def := range pluginReferenceDefinitions {
		path := fmt.Sprintf("%s/%s/%s", def.plugin, referenceEnv, referenceInstance)
		if _, err := kp.Get(path); err == nil {
			continue
		}
		if err := kp.Put(path, &Entry{
			Notes: def.notes,
			CustomAttributes: map[string]string{
				pluginapi.ReferenceEntryAttr: pluginapi.ReferenceEntryValue,
			},
		}); err != nil {
			return fmt.Errorf("secrets: reference template %s: %w", path, err)
		}
	}
	return nil
}

// ResetDB removes the KeePass file at DefaultDBPath. The next New() recreates it
// with a fresh tree and reference templates. The key file is not removed.
func ResetDB() error {
	path := DefaultDBPath()
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("secrets: remove database: %w", err)
	}
	return nil
}
