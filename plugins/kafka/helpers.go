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

	text := fmt.Sprintf(`[yellow]Message Details[white]

[aqua]Topic:[white]     %s
[aqua]Partition:[white] %d
[aqua]Offset:[white]    %d
[aqua]Timestamp:[white] %s
[aqua]Key:[white]       %s
`,
		topic, msg.Partition, msg.Offset, ts, key)

	if len(msg.Headers) > 0 {
		text += "\n[aqua]Headers:[white]\n"
		for k, v := range msg.Headers {
			text += fmt.Sprintf("  %s: %s\n", k, v)
		}
	}

	value := msg.Value
	if len(value) > 2000 {
		value = value[:2000] + "\n... (truncated)"
	}
	text += fmt.Sprintf("\n[aqua]Value:[white]\n%s", value)
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
