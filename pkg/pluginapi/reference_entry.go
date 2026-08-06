package pluginapi

import "fmt"

// ReferenceEntryAttr marks KeePass entries that exist only as empty schema
// templates (<plugin>/default/default_config). They are excluded from plugin
// discovery so nothing tries to connect using them.
const (
	ReferenceEntryAttr  = "omo_reference"
	ReferenceEntryValue = "true"
)

// IsReferenceEntry reports whether e is a template row (not a real connection).
func IsReferenceEntry(e *SecretEntry) bool {
	if e == nil || e.CustomAttributes == nil {
		return false
	}
	return e.CustomAttributes[ReferenceEntryAttr] == ReferenceEntryValue
}

// ListNonReferenceSecrets returns List(prefix) paths minus reference template entries.
func ListNonReferenceSecrets(prefix string) ([]string, error) {
	if !HasSecrets() {
		return nil, fmt.Errorf("secrets provider not available")
	}
	paths, err := Secrets().List(prefix)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		entry, err := Secrets().Get(p)
		if err != nil {
			continue
		}
		if IsReferenceEntry(entry) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}
