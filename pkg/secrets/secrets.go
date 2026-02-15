// Package secrets provides a KeePass-backed secrets provider for omo.
//
// The KeePass database lives at ~/.omo/secrets/omo.kdbx and is
// authenticated with a key file at ~/.omo/keys/omo.key. Both are
// bootstrapped automatically on first run.
//
// Secret paths follow the convention: pluginName/environment/entryName
// where each entry contains Title, UserName, Password, URL, Notes, and
// optional custom string attributes (e.g. certificates, private keys).
package secrets

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"omo/pkg/pluginapi"

	gkp "github.com/tobischo/gokeepasslib/v3"
	w "github.com/tobischo/gokeepasslib/v3/wrappers"
)

// ──────────────────────────────────────────────────────────────────────
// Public types
// ──────────────────────────────────────────────────────────────────────

// Entry represents a single secret entry with standard KeePass fields
// plus arbitrary custom attributes.
type Entry struct {
	Title            string
	UserName         string
	Password         string
	URL              string
	Notes            string
	CustomAttributes map[string]string // e.g. "tls_cert", "private_key"
}

// Provider is the interface every secrets backend must implement.
// Plugins receive a read/write Provider to resolve secret: paths from
// their YAML configs.
type Provider interface {
	// Get retrieves a secret entry by path (pluginName/environment/entryName).
	Get(path string) (*Entry, error)

	// Put creates or updates a secret entry at the given path.
	Put(path string, entry *Entry) error

	// Delete removes a secret entry.
	Delete(path string) error

	// List returns all entry paths under a prefix (e.g. "redis" or "redis/production").
	List(prefix string) ([]string, error)

	// Close flushes any pending changes and releases resources.
	Close() error
}

// ──────────────────────────────────────────────────────────────────────
// KeePass implementation
// ──────────────────────────────────────────────────────────────────────

// KeePassProvider is a Provider backed by a .kdbx file.
type KeePassProvider struct {
	mu       sync.Mutex
	db       *gkp.Database
	dbPath   string
	keyPath  string
	dirty    bool
}

// well-known KeePass value keys
const (
	kvTitle    = "Title"
	kvUserName = "UserName"
	kvPassword = "Password"
	kvURL      = "URL"
	kvNotes    = "Notes"
)

// well-known standard keys (used to skip when building custom attributes)
var standardKeys = map[string]bool{
	kvTitle:    true,
	kvUserName: true,
	kvPassword: true,
	kvURL:      true,
	kvNotes:    true,
}

// DefaultDBPath returns ~/.omo/secrets/omo.kdbx
func DefaultDBPath() string {
	return filepath.Join(pluginapi.OmoDir(), "secrets", "omo.kdbx")
}

// DefaultKeyPath returns ~/.omo/keys/omo.key
func DefaultKeyPath() string {
	return filepath.Join(pluginapi.OmoDir(), "keys", "omo.key")
}

// New opens (or bootstraps) the KeePass database and returns a Provider.
func New() (Provider, error) {
	return NewWithPaths(DefaultDBPath(), DefaultKeyPath())
}

// NewWithPaths opens (or bootstraps) a KeePass database at the given paths.
func NewWithPaths(dbPath, keyPath string) (Provider, error) {
	kp := &KeePassProvider{
		dbPath:  dbPath,
		keyPath: keyPath,
	}

	if err := kp.ensureDirs(); err != nil {
		return nil, fmt.Errorf("secrets: create directories: %w", err)
	}

	// Bootstrap key file if missing.
	if _, err := os.Stat(keyPath); errors.Is(err, os.ErrNotExist) {
		if err := kp.generateKeyFile(); err != nil {
			return nil, fmt.Errorf("secrets: generate key file: %w", err)
		}
	}

	// Bootstrap database if missing.
	if _, err := os.Stat(dbPath); errors.Is(err, os.ErrNotExist) {
		if err := kp.createDatabase(); err != nil {
			return nil, fmt.Errorf("secrets: create database: %w", err)
		}
	}

	// Open existing database.
	if err := kp.openDatabase(); err != nil {
		return nil, fmt.Errorf("secrets: open database: %w", err)
	}

	return kp, nil
}

// ──────────────────────────────────────────────────────────────────────
// Provider interface
// ──────────────────────────────────────────────────────────────────────

func (kp *KeePassProvider) Get(path string) (*Entry, error) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	parts, err := parsePath(path)
	if err != nil {
		return nil, err
	}

	entry, err := kp.findEntry(parts)
	if err != nil {
		return nil, err
	}

	return gkpEntryToEntry(entry), nil
}

func (kp *KeePassProvider) Put(path string, entry *Entry) error {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	parts, err := parsePath(path)
	if err != nil {
		return err
	}

	group := kp.ensureGroups(parts[:2]) // pluginName/environment
	entryTitle := parts[2]

	// Try to find and update an existing entry.
	for i := range group.Entries {
		if getKVValue(group.Entries[i], kvTitle) == entryTitle {
			group.Entries[i] = entryToGKPEntry(entry, entryTitle)
			kp.dirty = true
			return kp.flush()
		}
	}

	// Create new entry.
	group.Entries = append(group.Entries, entryToGKPEntry(entry, entryTitle))
	kp.dirty = true
	return kp.flush()
}

func (kp *KeePassProvider) Delete(path string) error {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	parts, err := parsePath(path)
	if err != nil {
		return err
	}

	group, err := kp.findGroup(parts[:2])
	if err != nil {
		return err
	}

	entryTitle := parts[2]
	for i := range group.Entries {
		if getKVValue(group.Entries[i], kvTitle) == entryTitle {
			group.Entries = append(group.Entries[:i], group.Entries[i+1:]...)
			kp.dirty = true
			return kp.flush()
		}
	}

	return fmt.Errorf("secrets: entry %q not found", path)
}

func (kp *KeePassProvider) List(prefix string) ([]string, error) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	var paths []string
	root := kp.rootGroup()

	for gi := range root.Groups {
		pluginGroup := &root.Groups[gi]
		pluginName := pluginGroup.Name

		for ei := range pluginGroup.Groups {
			envGroup := &pluginGroup.Groups[ei]
			envName := envGroup.Name

			for _, entry := range envGroup.Entries {
				p := pluginName + "/" + envName + "/" + getKVValue(entry, kvTitle)
				if prefix == "" || strings.HasPrefix(p, prefix) {
					paths = append(paths, p)
				}
			}
		}
	}

	return paths, nil
}

func (kp *KeePassProvider) Close() error {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	if kp.dirty {
		return kp.flush()
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────
// Internal helpers – bootstrap
// ──────────────────────────────────────────────────────────────────────

func (kp *KeePassProvider) ensureDirs() error {
	for _, dir := range []string{
		filepath.Dir(kp.dbPath),
		filepath.Dir(kp.keyPath),
	} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

// generateKeyFile creates a 32-byte random key in the KeePass v2.0 XML key
// file format, which is compatible with KeePassXC and gokeepasslib.
//
// Format:
//
//	<?xml version="1.0" encoding="utf-8"?>
//	<KeyFile>
//	  <Meta><Version>2.0</Version></Meta>
//	  <Key><Data Hash="AABBCCDD">hex-encoded-32-bytes</Data></Key>
//	</KeyFile>
func (kp *KeePassProvider) generateKeyFile() error {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return err
	}

	keyHex := strings.ToUpper(hex.EncodeToString(key))
	hash := sha256.Sum256(key)
	hashPrefix := strings.ToUpper(hex.EncodeToString(hash[:4]))

	xml := fmt.Sprintf(
		"<?xml version=\"1.0\" encoding=\"utf-8\"?>\n"+
			"<KeyFile>\n"+
			"\t<Meta>\n"+
			"\t\t<Version>2.0</Version>\n"+
			"\t</Meta>\n"+
			"\t<Key>\n"+
			"\t\t<Data Hash=\"%s\">%s</Data>\n"+
			"\t</Key>\n"+
			"</KeyFile>\n",
		hashPrefix, keyHex,
	)

	return os.WriteFile(kp.keyPath, []byte(xml), 0600)
}

// createDatabase initialises an empty KDBX4 file with key-file-only credentials.
func (kp *KeePassProvider) createDatabase() error {
	creds, err := gkp.NewKeyCredentials(kp.keyPath)
	if err != nil {
		return fmt.Errorf("parse key file: %w", err)
	}

	db := gkp.NewDatabase(
		gkp.WithDatabaseKDBXVersion4(),
	)
	db.Content.Meta.DatabaseName = "omo"
	db.Credentials = creds

	rootGroup := gkp.NewGroup()
	rootGroup.Name = "omo"
	db.Content.Root.Groups = []gkp.Group{rootGroup}

	// Lock protected entries before encoding.
	db.LockProtectedEntries()

	f, err := os.OpenFile(kp.dbPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := gkp.NewEncoder(f)
	return enc.Encode(db)
}

// openDatabase reads and decodes the KDBX4 file into memory.
func (kp *KeePassProvider) openDatabase() error {
	creds, err := gkp.NewKeyCredentials(kp.keyPath)
	if err != nil {
		return fmt.Errorf("parse key file: %w", err)
	}

	f, err := os.Open(kp.dbPath)
	if err != nil {
		return err
	}
	defer f.Close()

	db := gkp.NewDatabase(
		gkp.WithDatabaseKDBXVersion4(),
	)
	db.Credentials = creds

	dec := gkp.NewDecoder(f)
	if err := dec.Decode(db); err != nil {
		return fmt.Errorf("decode database: %w", err)
	}

	db.UnlockProtectedEntries()
	kp.db = db
	kp.dirty = false
	return nil
}

// flush writes the in-memory database back to disk.
func (kp *KeePassProvider) flush() error {
	kp.db.LockProtectedEntries()
	defer kp.db.UnlockProtectedEntries()

	f, err := os.OpenFile(kp.dbPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("secrets: write database: %w", err)
	}
	defer f.Close()

	enc := gkp.NewEncoder(f)
	if err := enc.Encode(kp.db); err != nil {
		return fmt.Errorf("secrets: encode database: %w", err)
	}

	kp.dirty = false
	return nil
}

// ──────────────────────────────────────────────────────────────────────
// Internal helpers – navigation
// ──────────────────────────────────────────────────────────────────────

// parsePath splits "pluginName/environment/entryName" and validates it.
func parsePath(path string) ([]string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("secrets: invalid path %q – expected pluginName/environment/entryName", path)
	}
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("secrets: path %q has an empty segment", path)
		}
	}
	return parts, nil
}

// rootGroup returns the top-level "omo" group.
func (kp *KeePassProvider) rootGroup() *gkp.Group {
	return &kp.db.Content.Root.Groups[0]
}

// findGroup navigates to pluginName/environment group.
func (kp *KeePassProvider) findGroup(parts []string) (*gkp.Group, error) {
	root := kp.rootGroup()
	pluginGroup := findSubGroup(root, parts[0])
	if pluginGroup == nil {
		return nil, fmt.Errorf("secrets: plugin group %q not found", parts[0])
	}
	envGroup := findSubGroup(pluginGroup, parts[1])
	if envGroup == nil {
		return nil, fmt.Errorf("secrets: environment group %q not found in %q", parts[1], parts[0])
	}
	return envGroup, nil
}

// findEntry locates the KeePass entry by [pluginName, environment, entryName].
func (kp *KeePassProvider) findEntry(parts []string) (gkp.Entry, error) {
	group, err := kp.findGroup(parts[:2])
	if err != nil {
		return gkp.Entry{}, err
	}

	entryTitle := parts[2]
	for _, entry := range group.Entries {
		if getKVValue(entry, kvTitle) == entryTitle {
			return entry, nil
		}
	}

	return gkp.Entry{}, fmt.Errorf("secrets: entry %q not found in %s/%s", entryTitle, parts[0], parts[1])
}

// ensureGroups creates the plugin/environment group hierarchy if needed, returning
// the deepest group.
func (kp *KeePassProvider) ensureGroups(parts []string) *gkp.Group {
	root := kp.rootGroup()
	pluginGroup := findOrCreateSubGroup(root, parts[0])
	envGroup := findOrCreateSubGroup(pluginGroup, parts[1])
	return envGroup
}

func findSubGroup(parent *gkp.Group, name string) *gkp.Group {
	for i := range parent.Groups {
		if parent.Groups[i].Name == name {
			return &parent.Groups[i]
		}
	}
	return nil
}

func findOrCreateSubGroup(parent *gkp.Group, name string) *gkp.Group {
	if g := findSubGroup(parent, name); g != nil {
		return g
	}
	ng := gkp.NewGroup()
	ng.Name = name
	parent.Groups = append(parent.Groups, ng)
	return &parent.Groups[len(parent.Groups)-1]
}

// ──────────────────────────────────────────────────────────────────────
// Internal helpers – conversion
// ──────────────────────────────────────────────────────────────────────

// getKVValue reads a named string value from an entry.
func getKVValue(e gkp.Entry, key string) string {
	for _, v := range e.Values {
		if v.Key == key {
			return v.Value.Content
		}
	}
	return ""
}

func gkpEntryToEntry(e gkp.Entry) *Entry {
	out := &Entry{
		Title:            getKVValue(e, kvTitle),
		UserName:         getKVValue(e, kvUserName),
		Password:         getKVValue(e, kvPassword),
		URL:              getKVValue(e, kvURL),
		Notes:            getKVValue(e, kvNotes),
		CustomAttributes: make(map[string]string),
	}
	for _, v := range e.Values {
		if !standardKeys[v.Key] {
			out.CustomAttributes[v.Key] = v.Value.Content
		}
	}
	return out
}

func entryToGKPEntry(e *Entry, title string) gkp.Entry {
	entry := gkp.NewEntry()
	entry.Values = []gkp.ValueData{
		mkValue(kvTitle, title, false),
		mkValue(kvUserName, e.UserName, false),
		mkValue(kvPassword, e.Password, true),
		mkValue(kvURL, e.URL, false),
		mkValue(kvNotes, e.Notes, false),
	}
	for k, v := range e.CustomAttributes {
		entry.Values = append(entry.Values, mkValue(k, v, true))
	}
	return entry
}

func mkValue(key, value string, protected bool) gkp.ValueData {
	return gkp.ValueData{
		Key: key,
		Value: gkp.V{
			Content:   value,
			Protected: w.NewBoolWrapper(protected),
		},
	}
}
