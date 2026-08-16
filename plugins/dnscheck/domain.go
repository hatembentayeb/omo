package dnscheck

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/idna"
)

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// normalizeDomain strips scheme/path, trailing dots, and lowercases a hostname.
func normalizeDomain(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "//")
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	s = strings.Trim(s, ".")
	s = strings.ToLower(s)
	if ascii, err := idna.ToASCII(s); err == nil && ascii != "" {
		s = ascii
	}
	return s
}

func isIP(s string) bool {
	return net.ParseIP(s) != nil
}

func resolverAddr(preset string) string {
	p := strings.TrimSpace(preset)
	if p == "" || strings.EqualFold(p, "system") {
		return "system"
	}
	p = strings.TrimPrefix(p, "udp://")
	p = strings.TrimPrefix(p, "tcp://")
	if host, port, err := net.SplitHostPort(p); err == nil {
		if port == "" {
			return net.JoinHostPort(host, "53")
		}
		return net.JoinHostPort(host, port)
	}
	if ip := net.ParseIP(p); ip != nil {
		return net.JoinHostPort(p, "53")
	}
	// hostname resolver
	if strings.Contains(p, ".") && !strings.Contains(p, ":") {
		return net.JoinHostPort(p, "53")
	}
	return net.JoinHostPort(p, "53")
}

func resolverLabel(addr string) string {
	a := strings.TrimSpace(addr)
	if a == "" || strings.EqualFold(a, "system") {
		return "System"
	}
	host := a
	if h, _, err := net.SplitHostPort(a); err == nil {
		host = h
	}
	for _, p := range defaultResolvers {
		if p.Addr == host || p.Addr == a {
			return p.Label + " (" + host + ")"
		}
	}
	return host
}

func cycleResolver(current string) string {
	cur := strings.TrimSpace(current)
	if cur == "" {
		cur = defaultResolvers[0].Addr
	}
	host := cur
	if h, _, err := net.SplitHostPort(cur); err == nil {
		host = h
	}
	for i, p := range defaultResolvers {
		if p.Addr == host || p.Addr == cur || (p.Addr == "system" && (cur == "system" || cur == "")) {
			return defaultResolvers[(i+1)%len(defaultResolvers)].Addr
		}
	}
	return defaultResolvers[0].Addr
}

func formatTTL(ttl uint32) string {
	d := time.Duration(ttl) * time.Second
	if d < time.Minute {
		return fmt.Sprintf("%ds", ttl)
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if m == 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dh %dm", h, m)
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

func daysUntil(t time.Time) int {
	if t.IsZero() {
		return 0
	}
	return int(time.Until(t).Hours() / 24)
}

func sslExpiryLabel(notAfter time.Time) string {
	if notAfter.IsZero() {
		return "-"
	}
	days := daysUntil(notAfter)
	date := notAfter.UTC().Format("2006-01-02")
	switch {
	case days < 0:
		return fmt.Sprintf("[red]%s expired %d days ago[white]", date, -days)
	case days < 7:
		return fmt.Sprintf("[red]%s (%d days)[white]", date, days)
	case days < 30:
		return fmt.Sprintf("[yellow]%s (%d days)[white]", date, days)
	default:
		return fmt.Sprintf("[green]%s (%d days)[white]", date, days)
	}
}

func sslStatusWord(notAfter time.Time, verified bool, err string) string {
	if err != "" {
		return "[red]error[white]"
	}
	if notAfter.IsZero() {
		return "[yellow]unknown[white]"
	}
	days := daysUntil(notAfter)
	switch {
	case days < 0:
		return "[red]expired[white]"
	case days < 7:
		return "[red]critical[white]"
	case days < 30:
		return "[yellow]expiring[white]"
	case !verified:
		return "[yellow]untrusted[white]"
	default:
		return "[green]ok[white]"
	}
}

func boolMark(ok bool) string {
	if ok {
		return "[green]yes[white]"
	}
	return "[red]no[white]"
}

func colorStatus(code int) string {
	switch {
	case code >= 200 && code < 300:
		return fmt.Sprintf("[green]%d[white]", code)
	case code >= 300 && code < 400:
		return fmt.Sprintf("[yellow]%d[white]", code)
	case code >= 400:
		return fmt.Sprintf("[red]%d[white]", code)
	default:
		return strconv.Itoa(code)
	}
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if n <= 0 || len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

func looksLikeSPF(s string) bool {
	t := strings.TrimSpace(strings.ToLower(s))
	t = strings.Trim(t, `"`)
	return strings.HasPrefix(t, "v=spf1")
}

func looksLikeDMARC(s string) bool {
	t := strings.TrimSpace(strings.ToLower(s))
	t = strings.Trim(t, `"`)
	return strings.HasPrefix(t, "v=dmarc1")
}

func looksLikeBIMI(s string) bool {
	t := strings.TrimSpace(strings.ToLower(s))
	t = strings.Trim(t, `"`)
	return strings.HasPrefix(t, "v=bimi1")
}

func looksLikeDKIM(s string) bool {
	t := strings.TrimSpace(strings.ToLower(s))
	t = strings.Trim(t, `"`)
	return strings.Contains(t, "v=dkim1") || (strings.Contains(t, "p=") && strings.Contains(t, "k="))
}

func cleanPrintable(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' {
			return ' '
		}
		if r == '\n' || r == '\r' {
			return r
		}
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, s)
}

func ptrName(ip string) string {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	if v4 := parsed.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa", v4[3], v4[2], v4[1], v4[0])
	}
	rev, err := reverseaddr(parsed)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(rev, ".")
}

func reverseaddr(ip net.IP) (string, error) {
	if ip == nil {
		return "", fmt.Errorf("invalid ip")
	}
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d.%d.%d.in-addr.arpa.", v4[3], v4[2], v4[1], v4[0]), nil
	}
	ip = ip.To16()
	if ip == nil {
		return "", fmt.Errorf("invalid ip")
	}
	buf := make([]byte, 0, len(ip)*4+len("ip6.arpa."))
	for i := len(ip) - 1; i >= 0; i-- {
		buf = append(buf, hexDigit[ip[i]&0xF], '.', hexDigit[ip[i]>>4], '.')
	}
	buf = append(buf, "ip6.arpa."...)
	return string(buf), nil
}

const hexDigit = "0123456789abcdef"

func meaningOf(t string) string {
	if m, ok := recordTypeMeaning[strings.ToUpper(t)]; ok {
		return m
	}
	return ""
}
