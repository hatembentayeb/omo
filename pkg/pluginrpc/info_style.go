package pluginrpc

import (
	"fmt"
	"regexp"
	"strings"
)

// Info panel palette (tview dynamic color tags). Mutated by the host theme.
var (
	infoOrange = "#e09201" // plugin title and keys
	infoValue  = "#f5efe6" // values
)

// SetInfoColors updates info-panel key/value colors to match the host theme.
func SetInfoColors(key, value string) {
	if key != "" {
		infoOrange = key
	}
	if value != "" {
		infoValue = value
	}
}

var (
	reColorTag = regexp.MustCompile(`\[[^\]]*\]`)
	reInfoKV   = regexp.MustCompile(`^([^:\n]+?)\s*:\s*(.*)$`)
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
	return fmt.Sprintf("[%s::b]%s[%s]", infoOrange, title, infoValue)
}

// InfoLine returns a single colored "Label: value" line.
func InfoLine(label, value string) string {
	return formatInfoKV(label, value, len(label))
}

// ColorizeInfoPanel styles plugin header info for the left Info column.
// "Label: value" lines use orange keys and cream values. Existing color
// tags are preserved when a line is already fully styled.
func ColorizeInfoPanel(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimRight(text, "\n")
	if strings.TrimSpace(text) == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	labelWidth := infoLabelWidth(lines)
	for i, line := range lines {
		out = append(out, colorizeInfoLine(line, i == 0, labelWidth))
	}
	return strings.Join(out, "\n")
}

func infoLabelWidth(lines []string) int {
	width := 0
	for i, line := range lines {
		plain := strings.TrimSpace(stripColorTags(line))
		if plain == "" {
			continue
		}
		if i == 0 && !strings.Contains(plain, ":") {
			continue
		}
		m := reInfoKV.FindStringSubmatch(plain)
		if m == nil {
			continue
		}
		if n := len(strings.TrimSpace(m[1])); n > width {
			width = n
		}
	}
	return width
}

func colorizeInfoLine(line string, first bool, labelWidth int) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return line
	}

	plain := stripColorTags(trimmed)
	if first && !strings.Contains(plain, ":") {
		return fmt.Sprintf("[%s::b]%s[%s]", infoOrange, plain, infoValue)
	}

	// Already colorized (title or KV with tags).
	if strings.Count(trimmed, "[") >= 2 {
		return line
	}

	m := reInfoKV.FindStringSubmatch(plain)
	if m == nil {
		if first {
			return fmt.Sprintf("[%s::b]%s[%s]", infoOrange, plain, infoValue)
		}
		return fmt.Sprintf("[%s]%s[-]", infoValue, plain)
	}

	label := strings.TrimSpace(m[1])
	value := strings.TrimSpace(m[2])
	return formatInfoKV(label, value, labelWidth)
}

func formatInfoKV(label, value string, width int) string {
	if width < len(label) {
		width = len(label)
	}
	padded := label + strings.Repeat(" ", width-len(label))
	return fmt.Sprintf("[%s::b]%s[-] : [%s]%s[-]", infoOrange, padded, infoValue, value)
}

func stripColorTags(s string) string {
	return reColorTag.ReplaceAllString(s, "")
}
