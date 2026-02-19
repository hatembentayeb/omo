package pluginapi

import (
	"fmt"
	"sync"
)

// SecretsProvider is the minimal interface plugins use to read/write secrets.
// The concrete implementation lives in pkg/secrets; pluginapi only holds the
// accessor so plugins depend on the interface, not the implementation.
type SecretsProvider interface {
	Get(path string) (*SecretEntry, error)
	Put(path string, entry *SecretEntry) error
	Delete(path string) error
	List(prefix string) ([]string, error)
	Reload() error
	Close() error
}

// SecretEntry mirrors secrets.Entry so plugins don't need to import
// the secrets package directly.
type SecretEntry struct {
	Title            string
	UserName         string
	Password         string
	URL              string
	Notes            string
	CustomAttributes map[string]string
}

var (
	globalSecrets   SecretsProvider
	globalSecretsMu sync.RWMutex
)

// SetSecretsProvider registers the global secrets provider.
// Called once by the host during startup.
func SetSecretsProvider(p SecretsProvider) {
	globalSecretsMu.Lock()
	defer globalSecretsMu.Unlock()
	globalSecrets = p
}

// Secrets returns the global secrets provider.
// Panics if the provider has not been initialised.
func Secrets() SecretsProvider {
	globalSecretsMu.RLock()
	defer globalSecretsMu.RUnlock()
	if globalSecrets == nil {
		panic("pluginapi.Secrets() called before SetSecretsProvider()")
	}
	return globalSecrets
}

// HasSecrets reports whether a secrets provider has been initialised.
func HasSecrets() bool {
	globalSecretsMu.RLock()
	defer globalSecretsMu.RUnlock()
	return globalSecrets != nil
}

// ResolveSecret is a convenience helper for plugins.
// Given a secret path (e.g. "redis/production/main-instance"), it returns
// the matching SecretEntry, or an error if the provider is unavailable
// or the entry does not exist.
func ResolveSecret(path string) (*SecretEntry, error) {
	if !HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}
	return Secrets().Get(path)
}
