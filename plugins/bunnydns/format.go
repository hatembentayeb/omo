package bunnydns

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func formatCount(n int64) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return sign + s
	}
	var b strings.Builder
	b.Grow(len(s) + len(s)/3 + len(sign))
	b.WriteString(sign)
	rem := len(s) % 3
	if rem == 0 {
		rem = 3
	}
	b.WriteString(s[:rem])
	for i := rem; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func formatCountShort(n int64) string {
	abs := n
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case abs >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case abs >= 10_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	default:
		return formatCount(n)
	}
}

func formatPercent(part, total int64) string {
	if total <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(part)/float64(total))
}

func sparkBar(v, max float64, width int) string {
	if width < 1 || max <= 0 || v <= 0 {
		return ""
	}
	n := int(v/max*float64(width) + 0.5)
	if n < 1 {
		n = 1
	}
	if n > width {
		n = width
	}
	return strings.Repeat("█", n)
}

func formatChartDay(k string) string {
	k = strings.TrimSpace(k)
	if k == "" {
		return "—"
	}
	if t, ok := parseChartTime(k); ok {
		return t.Format("Mon 2 Jan")
	}
	return k
}

func parseChartTime(k string) (time.Time, bool) {
	k = strings.TrimSpace(k)
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"01/02/2006",
		"2006/01/02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, k); err == nil {
			return t, true
		}
	}
	if isAllDigits(k) {
		n, err := strconv.ParseInt(k, 10, 64)
		if err == nil && n > 0 {
			switch {
			case n > 1e16: // .NET ticks-ish; convert from 100ns since year 1
				unix := (n - 621355968000000000) / 1e7
				if unix > 0 {
					return time.Unix(unix, 0).UTC(), true
				}
			case n > 1e12:
				return time.UnixMilli(n).UTC(), true
			case n > 1e9:
				return time.Unix(n, 0).UTC(), true
			}
		}
	}
	return time.Time{}, false
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func recordTypeLabel(key string) string {
	name := strings.TrimSpace(key)
	name = strings.TrimPrefix(name, "Type ")
	name = strings.TrimPrefix(name, "TYPE_")
	if code, ok := parseRecordType(name); ok {
		name = recordTypeName(code)
	}
	if meaning, ok := recordTypeMeaning[name]; ok {
		return name + " — " + meaning
	}
	return name
}

func chartTotal(m map[string]float64) int64 {
	var sum float64
	for _, v := range m {
		sum += v
	}
	return int64(sum + 0.5)
}

func formatCheckedAt(s string) string {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Format("2 Jan 15:04")
	}
	if t, ok := parseChartTime(s); ok {
		return t.Format("2 Jan 15:04")
	}
	return s
}
