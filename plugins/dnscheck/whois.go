package dnscheck

import (
	"bufio"
	"fmt"
	"net"
	"regexp"
	"strings"
	"time"
)

var (
	reWhoisField = regexp.MustCompile(`(?i)^([A-Za-z0-9][A-Za-z0-9 _/-]*):\s*(.+)$`)
)

func checkWhois(domain string) *whoisReport {
	rep := &whoisReport{QueriedAt: time.Now()}
	tld := domain
	if i := strings.LastIndex(domain, "."); i >= 0 {
		tld = domain[i+1:]
	}

	raw, server, err := whoisQuery("whois.iana.org", tld, whoisTimeout)
	if err != nil {
		rep.Error = err.Error()
		return rep
	}
	refer := whoisValue(raw, "refer", "whois")
	if refer != "" && !strings.EqualFold(refer, "whois.iana.org") {
		server = refer
		raw2, _, err2 := whoisQuery(refer, domain, whoisTimeout)
		if err2 != nil {
			rep.Error = err2.Error()
			rep.Server = server
			rep.Raw = raw
			return rep
		}
		raw = raw2
	} else {
		// IANA only knows the TLD; still try a common registry if refer missing.
		raw2, _, err2 := whoisQuery(server, domain, whoisTimeout)
		if err2 == nil && len(raw2) > len(raw) {
			raw = raw2
		}
	}

	rep.OK = true
	rep.Server = server
	rep.Raw = cleanPrintable(raw)
	rep.Registrar = firstNonEmpty(
		whoisValue(raw, "Registrar", "Sponsoring Registrar"),
		whoisValue(raw, "registrar"),
	)
	rep.Created = firstNonEmpty(
		whoisValue(raw, "Creation Date", "created", "Created Date", "Domain Registration Date"),
	)
	rep.Updated = firstNonEmpty(
		whoisValue(raw, "Updated Date", "last-modified", "Last Updated On"),
	)
	rep.Expires = firstNonEmpty(
		whoisValue(raw, "Registry Expiry Date", "Expiry Date", "Expiration Date", "paid-till", "expires"),
	)
	rep.Status = strings.Join(whoisValues(raw, "Domain Status", "status"), "; ")
	rep.NameServers = whoisValues(raw, "Name Server", "nserver")
	return rep
}

func whoisQuery(server, query string, timeout time.Duration) (string, string, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return "", "", fmt.Errorf("empty whois server")
	}
	addr := server
	if !strings.Contains(server, ":") {
		addr = net.JoinHostPort(server, "43")
	}
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return "", server, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := fmt.Fprintf(conn, "%s\r\n", query); err != nil {
		return "", server, err
	}
	var b strings.Builder
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		b.WriteString(sc.Text())
		b.WriteByte('\n')
		if b.Len() > 512*1024 {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return b.String(), server, err
	}
	if b.Len() == 0 {
		return "", server, fmt.Errorf("empty WHOIS reply from %s", server)
	}
	return b.String(), server, nil
}

func whoisValue(raw string, keys ...string) string {
	vals := whoisValues(raw, keys...)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

func whoisValues(raw string, keys ...string) []string {
	want := make(map[string]bool, len(keys))
	for _, k := range keys {
		want[strings.ToLower(strings.TrimSpace(k))] = true
	}
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		m := reWhoisField.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(m[1]))
		if !want[key] {
			continue
		}
		val := strings.TrimSpace(m[2])
		low := strings.ToLower(val)
		if val == "" || seen[low] {
			continue
		}
		seen[low] = true
		out = append(out, val)
	}
	return out
}
