package main

import (
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// RBACAccount represents an account parsed from argocd-cm.
type RBACAccount struct {
	Name         string
	Capabilities []string // e.g. "apiKey", "login"
	Enabled      bool
}

// PolicyRule represents a single casbin policy line from argocd-rbac-cm.
// Format: p, <subject>, <resource>, <action>, <object>, <effect>
type PolicyRule struct {
	Subject  string // e.g. "role:admin"
	Resource string // e.g. "applications"
	Action   string // e.g. "get", "create", "*"
	Object   string // e.g. "*", "default/*"
	Effect   string // "allow" or "deny"
}

func (pr PolicyRule) String() string {
	return fmt.Sprintf("p, %s, %s, %s, %s, %s",
		pr.Subject, pr.Resource, pr.Action, pr.Object, pr.Effect)
}

// GroupBinding represents a group mapping: g, <user>, <role>
type GroupBinding struct {
	User string
	Role string
}

func (gb GroupBinding) String() string {
	return fmt.Sprintf("g, %s, %s", gb.User, gb.Role)
}

// RBACConfig holds the full parsed RBAC state.
type RBACConfig struct {
	Accounts      []RBACAccount
	Policies      []PolicyRule
	Groups        []GroupBinding
	DefaultPolicy string // policy.default value
}

// ParseArgoCM extracts account definitions from argocd-cm ConfigMap data.
// Keys like "accounts.<name>" hold capabilities, "accounts.<name>.enabled" hold enabled state.
func ParseArgoCM(cm *corev1.ConfigMap) []RBACAccount {
	if cm == nil || cm.Data == nil {
		return nil
	}

	accountCaps := make(map[string]string)
	accountEnabled := make(map[string]bool)
	seen := make(map[string]bool)

	for key, value := range cm.Data {
		if !strings.HasPrefix(key, "accounts.") {
			continue
		}
		rest := key[len("accounts."):]

		if strings.HasSuffix(rest, ".enabled") {
			name := rest[:len(rest)-len(".enabled")]
			seen[name] = true
			accountEnabled[name] = value == "true"
		} else if !strings.Contains(rest, ".") {
			seen[name(rest)] = true
			accountCaps[rest] = value
		}
	}

	var accounts []RBACAccount
	for acctName := range seen {
		caps := parseCaps(accountCaps[acctName])
		enabled := true
		if v, ok := accountEnabled[acctName]; ok {
			enabled = v
		}
		accounts = append(accounts, RBACAccount{
			Name:         acctName,
			Capabilities: caps,
			Enabled:      enabled,
		})
	}

	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].Name < accounts[j].Name
	})
	return accounts
}

func name(s string) string { return s }

func parseCaps(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	var caps []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			caps = append(caps, p)
		}
	}
	return caps
}

// ParseRBACCM extracts policies, groups, and default policy from argocd-rbac-cm.
func ParseRBACCM(cm *corev1.ConfigMap) ([]PolicyRule, []GroupBinding, string) {
	if cm == nil || cm.Data == nil {
		return nil, nil, ""
	}

	defaultPolicy := cm.Data["policy.default"]
	csv := cm.Data["policy.csv"]

	var policies []PolicyRule
	var groups []GroupBinding

	for _, line := range strings.Split(csv, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := splitCSVLine(line)
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "p":
			if len(parts) >= 6 {
				policies = append(policies, PolicyRule{
					Subject:  parts[1],
					Resource: parts[2],
					Action:   parts[3],
					Object:   parts[4],
					Effect:   parts[5],
				})
			}
		case "g":
			if len(parts) >= 3 {
				groups = append(groups, GroupBinding{
					User: parts[1],
					Role: parts[2],
				})
			}
		}
	}

	return policies, groups, defaultPolicy
}

func splitCSVLine(line string) []string {
	parts := strings.Split(line, ",")
	var result []string
	for _, p := range parts {
		result = append(result, strings.TrimSpace(p))
	}
	return result
}

// ApplyArgoCM writes RBAC accounts back into the argocd-cm ConfigMap.
// It removes all existing accounts.* keys first, then writes the new ones.
func ApplyArgoCM(cm *corev1.ConfigMap, accounts []RBACAccount) {
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	// Remove existing account keys
	for key := range cm.Data {
		if strings.HasPrefix(key, "accounts.") {
			delete(cm.Data, key)
		}
	}

	for _, acct := range accounts {
		if len(acct.Capabilities) > 0 {
			cm.Data["accounts."+acct.Name] = strings.Join(acct.Capabilities, ", ")
		}
		if !acct.Enabled {
			cm.Data["accounts."+acct.Name+".enabled"] = "false"
		} else {
			cm.Data["accounts."+acct.Name+".enabled"] = "true"
		}
	}
}

// ApplyRBACCM writes policies and groups back into argocd-rbac-cm ConfigMap.
func ApplyRBACCM(cm *corev1.ConfigMap, policies []PolicyRule, groups []GroupBinding, defaultPolicy string) {
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}

	var lines []string
	for _, p := range policies {
		lines = append(lines, p.String())
	}
	for _, g := range groups {
		lines = append(lines, g.String())
	}

	cm.Data["policy.csv"] = strings.Join(lines, "\n")
	if defaultPolicy != "" {
		cm.Data["policy.default"] = defaultPolicy
	}
}
