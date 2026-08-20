package kafka

import (
	"fmt"
	"strconv"
	"strings"
)

func formatFullMessageDetail(topic string, msg *MessageInfo) string {
	key := msg.Key
	if key == "" {
		key = "(null)"
	}
	ts := ""
	if !msg.Timestamp.IsZero() {
		ts = msg.Timestamp.Format("2006-01-02 15:04:05.000")
	}

	text := fmt.Sprintf(`[yellow]Message Details[#f5efe6]

[#e09201]Topic:[#f5efe6]     %s
[#e09201]Partition:[#f5efe6] %d
[#e09201]Offset:[#f5efe6]    %d
[#e09201]Timestamp:[#f5efe6] %s
[#e09201]Key:[#f5efe6]       %s
`,
		topic, msg.Partition, msg.Offset, ts, key)

	if len(msg.Headers) > 0 {
		text += "\n[#e09201]Headers:[#f5efe6]\n"
		for k, v := range msg.Headers {
			text += fmt.Sprintf("  %s: %s\n", k, v)
		}
	}

	value := msg.Value
	if len(value) > 2000 {
		value = value[:2000] + "\n... (truncated)"
	}
	text += fmt.Sprintf("\n[#e09201]Value:[#f5efe6]\n%s", value)
	return text
}

func formatInt32Slice(slice []int32) string {
	if len(slice) == 0 {
		return ""
	}
	parts := make([]string, len(slice))
	for i, v := range slice {
		parts[i] = strconv.FormatInt(int64(v), 10)
	}
	return strings.Join(parts, ", ")
}

func formatLargeNumber(n int64) string {
	if n >= 1_000_000_000 {
		return fmt.Sprintf("%.2f B", float64(n)/1_000_000_000)
	} else if n >= 1_000_000 {
		return fmt.Sprintf("%.2f M", float64(n)/1_000_000)
	} else if n >= 1_000 {
		return fmt.Sprintf("%.1f K", float64(n)/1_000)
	}
	return strconv.FormatInt(n, 10)
}
