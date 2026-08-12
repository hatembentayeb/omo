package pluginrpc

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Info panel palette (tview dynamic color tags).
const (
	infoBrand   = "#FF6B00" // plugin title
	infoLabel   = "#8BE9FD" // Host / URL / View …
	infoValue   = "#F8F8F2" // default value
	infoHost    = "#F1FA8C" // host / url / endpoint
	infoView    = "#BD93F9" // current view id
	infoOK      = "#50FA7B" // connected / healthy
	infoWarn    = "#FFB86C" // counts / warnings
	infoBad     = "#FF5555" // errors / disconnected
	infoMuted   = "#6272A4" // secondary
)

var (
	reColorTag = regexp.MustCompile(`\[[^\]]*\]`)
	reInfoKV   = regexp.MustCompile(`^([^:\n]+):\s*(.*)$`)
)

// FormatInfo appends an optional extra line and colorizes the info panel.
func FormatInfo(base, extra string) string {
	if extra != "" {
		base = base + "\n" + extra
	}
	return ColorizeInfoPanel(base)
}

// NotConnectedInfo builds the standard disconnected info panel.
func NotConnectedInfo(brand, detail string) string {
	msg := brand + "\nStatus: Not Connected"
	if detail != "" {
		msg += "\n" + detail
	}
	return ColorizeInfoPanel(msg)
}

// InfoTitle returns a branded first line for plugin info panels.
func InfoTitle(title string) string {
	return fmt.Sprintf("[%s::b]%s[white]", infoBrand, title)
}

// InfoLine returns a single colored "Label: value" line.
func InfoLine(label, value string) string {
	return formatInfoKV(label, value)
}

// ColorizeInfoPanel styles plugin header info for the left Info column.
// Plain "Label: value" lines become cyan labels + typed values. Existing color
// tags are preserved when a line is already fully styled.
func ColorizeInfoPanel(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	if strings.TrimSpace(text) == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	for i, line := range lines {
		out = append(out, colorizeInfoLine(line, i == 0))
	}
	return strings.Join(out, "\n")
}

func colorizeInfoLine(line string, first bool) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return line
	}

	plain := stripColorTags(trimmed)
	if first && !strings.Contains(plain, ":") {
		return fmt.Sprintf("[%s::b]%s[white]", infoBrand, plain)
	}

	// Already looks intentionally styled (multiple tags beyond a simple prefix).
	if strings.Count(trimmed, "[") >= 2 && strings.Contains(trimmed, "]:") {
		return line
	}

	m := reInfoKV.FindStringSubmatch(plain)
	if m == nil {
		if first {
			return fmt.Sprintf("[%s::b]%s[white]", infoBrand, plain)
		}
		return fmt.Sprintf("[%s]%s[white]", infoMuted, plain)
	}

	label := strings.TrimSpace(m[1])
	value := strings.TrimSpace(m[2])
	return formatInfoKV(label, value)
}

func formatInfoKV(label, value string) string {
	return fmt.Sprintf("[%s::b]%s:[white] [%s]%s[white]", infoLabel, label, valueColor(label, value), value)
}

func valueColor(label, value string) string {
	l := strings.ToLower(strings.TrimSpace(label))
	v := strings.ToLower(strings.TrimSpace(value))

	switch {
	case strings.Contains(v, "not connected") || v == "error" || v == "failed" || v == "offline":
		return infoBad
	case v == "connected" || strings.HasPrefix(v, "connected ") || v == "ok" || v == "healthy" || v == "up to date":
		return infoOK
	}

	switch {
	case l == "view" || strings.HasSuffix(l, " view"):
		return infoView
	case l == "status" || l == "state":
		return infoOK
	case containsAny(l, "host", "url", "server", "endpoint", "address", "bootstrap", "socket"):
		return infoHost
	case containsAny(l, "cluster", "context", "namespace", "profile", "region", "account", "repo", "bucket", "instance", "vhost"):
		return infoHost
	case containsAny(l, "db", "database", "user", "owner"):
		return "#8BE9FD"
	case looksLikeCountLabel(l) || isMostlyNumber(value):
		return infoWarn
	default:
		return infoValue
	}
}

func looksLikeCountLabel(label string) bool {
	return containsAny(label,
		"container", "image", "network", "volume", "key", "topic", "broker",
		"queue", "exchange", "process", "forward", "active", "entry", "plugin",
		"pod", "service", "workload", "message", "budget",
	)
}

func containsAny(s string, parts ...string) bool {
	for _, p := range parts {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func isMostlyNumber(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	digits := 0
	for _, r := range s {
		if unicode.IsDigit(r) {
			digits++
		}
	}
	return digits > 0 && digits >= len(s)/2
}

func stripColorTags(s string) string {
	return reColorTag.ReplaceAllString(s, "")
}
