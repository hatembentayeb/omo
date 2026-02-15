package secrets

import (
	"omo/pkg/pluginapi"
)

// Adapter wraps a KeePassProvider and implements pluginapi.SecretsProvider
// so the host can set it as the global provider without plugins importing
// the secrets package directly.
type Adapter struct {
	inner Provider
}

// NewAdapter creates a pluginapi.SecretsProvider backed by the given Provider.
func NewAdapter(p Provider) pluginapi.SecretsProvider {
	return &Adapter{inner: p}
}

func (a *Adapter) Get(path string) (*pluginapi.SecretEntry, error) {
	e, err := a.inner.Get(path)
	if err != nil {
		return nil, err
	}
	return toPluginEntry(e), nil
}

func (a *Adapter) Put(path string, entry *pluginapi.SecretEntry) error {
	return a.inner.Put(path, fromPluginEntry(entry))
}

func (a *Adapter) Delete(path string) error {
	return a.inner.Delete(path)
}

func (a *Adapter) List(prefix string) ([]string, error) {
	return a.inner.List(prefix)
}

func (a *Adapter) Close() error {
	return a.inner.Close()
}

// ── conversion helpers ──

func toPluginEntry(e *Entry) *pluginapi.SecretEntry {
	ca := make(map[string]string, len(e.CustomAttributes))
	for k, v := range e.CustomAttributes {
		ca[k] = v
	}
	return &pluginapi.SecretEntry{
		Title:            e.Title,
		UserName:         e.UserName,
		Password:         e.Password,
		URL:              e.URL,
		Notes:            e.Notes,
		CustomAttributes: ca,
	}
}

func fromPluginEntry(e *pluginapi.SecretEntry) *Entry {
	ca := make(map[string]string, len(e.CustomAttributes))
	for k, v := range e.CustomAttributes {
		ca[k] = v
	}
	return &Entry{
		Title:            e.Title,
		UserName:         e.UserName,
		Password:         e.Password,
		URL:              e.URL,
		Notes:            e.Notes,
		CustomAttributes: ca,
	}
}
