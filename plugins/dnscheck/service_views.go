package dnscheck

import (
	"fmt"
	"strings"
	"time"

	"omo/pkg/pluginrpc"
)

func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Overview", Action: "goto_overview"},
		{Key: "1", Label: "Records", Action: "goto_records"},
		{Key: "2", Label: "Nameservers", Action: "goto_nameservers"},
		{Key: "3", Label: "SSL", Action: "goto_ssl"},
		{Key: "4", Label: "Mail", Action: "goto_mail"},
		{Key: "5", Label: "HTTP", Action: "goto_http"},
		{Key: "6", Label: "DNSSEC", Action: "goto_dnssec"},
		{Key: "7", Label: "Reverse", Action: "goto_reverse"},
		{Key: "8", Label: "Ports", Action: "goto_reachability"},
		{Key: "9", Label: "WHOIS", Action: "goto_whois"},
	}
}

func lookupActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "L", Label: "Lookup", Action: "lookup_domain"},
		{Key: "N", Label: "Resolver", Action: "set_resolver"},
		{Key: "G", Label: "Cycle NS", Action: "cycle_resolver"},
	}
}

func recordsActions() []pluginrpc.KeyBinding {
	return append(lookupActions(),
		pluginrpc.KeyBinding{Key: "E", Label: "Details", Action: "record_details"},
		pluginrpc.KeyBinding{Key: "T", Label: "Filter Type", Action: "filter_record_type"},
		pluginrpc.KeyBinding{Key: "C", Label: "Clear Filter", Action: "clear_record_filters"},
	)
}

func detailsActions() []pluginrpc.KeyBinding {
	return append(lookupActions(),
		pluginrpc.KeyBinding{Key: "E", Label: "Details", Action: "row_details"},
	)
}

func helpSections() []pluginrpc.HelpSection {
	return pluginrpc.HelpNav(viewNavBindings(), nil,
		pluginrpc.HelpSection{Title: "Lookup", Bindings: lookupActions()},
		pluginrpc.HelpSection{Title: "Records", Bindings: recordsActions()},
		pluginrpc.HelpSection{Title: "Details", Bindings: detailsActions()},
	)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	Help:  helpSections,
}

func (s *Service) baseInfo(extra string) string {
	domain := s.domain
	if domain == "" {
		domain = "(none — press L)"
	}
	msg := fmt.Sprintf("%s\nDomain: %s\nResolver: %s\nView: %s",
		brandName, domain, resolverLabel(s.resolver), s.currentView)
	if s.report != nil && !s.report.QueriedAt.IsZero() && s.domain != "" {
		msg += "\nQueried: " + s.report.QueriedAt.Format("15:04:05")
		if s.report.DNSMeta.Duration > 0 {
			msg += "\nDNS RTT: " + formatDuration(s.report.DNSMeta.Duration)
		}
	}
	return pluginrpc.FormatInfo(msg, extra)
}

func (s *Service) viewDashboardLocked() (pluginrpc.ViewData, error) {
	if s.domain == "" {
		return pluginrpc.Widget("DNS Check", "not configured", "", [][2]string{
			{"Domain", "set KeePass URL"},
			{"Resolver", resolverLabel(s.resolver)},
		}), nil
	}
	// Prefer a warm report; avoid the full overview (DNS+SSL+HTTP) pulse cost.
	report := s.report
	if report == nil || report.Domain != s.domain || report.Resolver != s.resolver {
		return pluginrpc.Widget("DNS Check", "idle", s.domain, [][2]string{
			{"Domain", s.domain},
			{"Resolver", resolverLabel(s.resolver)},
			{"Result", "open plugin to lookup"},
			{"Cache", "cold"},
		}), nil
	}
	status := "connected"
	result := fmt.Sprintf("%d records", len(report.Records))
	if report.DNSMeta.Error != "" {
		status = "error"
		result = pluginrpc.Truncate(report.DNSMeta.Error, 36)
	}
	ssl := "-"
	if report.SSL != nil {
		if report.SSL.Leaf != nil {
			ssl = sslExpiryLabel(report.SSL.Leaf.NotAfter)
		} else if report.SSL.Error != "" {
			ssl = pluginrpc.Truncate(report.SSL.Error, 24)
		} else {
			ssl = sslStatusWord(time.Time{}, report.SSL.Verified, report.SSL.Error)
		}
	}
	return pluginrpc.Widget("DNS Check", status, s.domain, [][2]string{
		{"Domain", s.domain},
		{"Resolver", resolverLabel(s.resolver)},
		{"Result", result},
		{"SSL", pluginrpc.Truncate(ssl, 24)},
	}), nil
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewOverview
	}
	s.currentView = viewID

	if s.domain == "" {
		return s.emptyViewLocked(viewID)
	}

	switch viewID {
	case viewRecords:
		return s.viewRecordsLocked()
	case viewNameservers:
		return s.viewNameserversLocked()
	case viewSSL:
		return s.viewSSLLocked()
	case viewMail:
		return s.viewMailLocked()
	case viewHTTP:
		return s.viewHTTPLocked()
	case viewDNSSEC:
		return s.viewDNSSECLocked()
	case viewReverse:
		return s.viewReverseLocked()
	case viewReachability:
		return s.viewReachabilityLocked()
	case viewWhois:
		return s.viewWhoisLocked()
	default:
		return s.viewOverviewLocked()
	}
}

func (s *Service) emptyViewLocked(viewID string) (pluginrpc.ViewData, error) {
	rows := [][]string{
		{"Hint", "Press L and enter a domain (like Google Admin Toolbox Dig)."},
		{"Also", "Optional KeePass URL field pre-fills the name."},
		{"SSL", "View 3 checks certificate expiry and chain."},
		{"Mail", "View 4 shows MX, SPF, DKIM, DMARC, BIMI."},
	}
	return ui.OK(viewID, brandName, s.baseInfo(""), []string{"", ""}, rows, "", lookupActions()...), nil
}

func (s *Service) viewOverviewLocked() (pluginrpc.ViewData, error) {
	s.ensureOverviewLocked()
	rep := s.report
	var rows [][]string

	add := func(k, v string) { rows = append(rows, []string{k, v}) }
	add("[yellow::b]Query", "")
	add("Domain", s.domain)
	add("Resolver", resolverLabel(s.resolver)+"  "+exchangeServer(s.resolver))
	if rep.DNSMeta.Error != "" {
		add("DNS", "[red]"+rep.DNSMeta.Error+"[white]")
	} else {
		add("DNS", fmt.Sprintf("[green]ok[white]  %s  %d records", formatDuration(rep.DNSMeta.Duration), len(rep.Records)))
	}

	a := recordValues(rep.Records, "A")
	aaaa := recordValues(rep.Records, "AAAA")
	add("[yellow::b]Addresses", "")
	if len(a) == 0 && len(aaaa) == 0 {
		add("A / AAAA", "[red]none[white]")
	} else {
		if len(a) > 0 {
			add("A", strings.Join(a, ", "))
		}
		if len(aaaa) > 0 {
			add("AAAA", strings.Join(aaaa, ", "))
		}
	}

	ns := recordValues(rep.Records, "NS")
	add("NS", orDash(strings.Join(ns, ", ")))
	mx := recordsOf(rep.Records, "MX")
	if len(mx) == 0 {
		add("MX", "[yellow]none[white]")
	} else {
		add("MX", fmt.Sprintf("%d exchanger(s)", len(mx)))
	}

	add("[yellow::b]TLS", "")
	if rep.SSL == nil {
		add("SSL", "-")
	} else if !rep.SSL.OK || rep.SSL.Leaf == nil {
		err := rep.SSL.Error
		if err == "" {
			err = "no certificate"
		}
		add("SSL", "[red]"+err+"[white]")
	} else {
		leaf := rep.SSL.Leaf
		add("Status", sslStatusWord(leaf.NotAfter, rep.SSL.Verified, ""))
		add("Expires", sslExpiryLabel(leaf.NotAfter))
		add("Issuer", shortDN(leaf.Issuer))
		add("Protocol", joinSlash(rep.SSL.Version, rep.SSL.Cipher, rep.SSL.ALPN))
		add("Verified", boolMark(rep.SSL.Verified))
	}

	add("[yellow::b]HTTP", "")
	if rep.HTTP == nil {
		add("HTTP", "-")
	} else if !rep.HTTP.OK {
		add("HTTP", "[red]"+rep.HTTP.Error+"[white]")
	} else {
		add("Status", colorStatus(rep.HTTP.Status))
		add("URL", truncate(rep.HTTP.FinalURL, 80))
		add("HSTS", orDash(rep.HTTP.HSTS))
	}

	spf, dmarc := false, false
	for _, r := range recordsOf(rep.Records, "TXT") {
		if looksLikeSPF(r.Data) {
			spf = true
		}
	}
	for _, r := range rep.Records {
		if r.Type == "TXT" && strings.HasPrefix(strings.ToLower(r.Name), "_dmarc.") && looksLikeDMARC(r.Data) {
			dmarc = true
		}
	}
	add("[yellow::b]Mail auth", "")
	add("SPF", boolMark(spf))
	add("DMARC", boolMark(dmarc))
	add("DNSSEC DS", boolMark(hasType(rep.Records, "DS")))

	rows = pluginrpc.EnsureRows(rows, []string{"-", "No data"})
	return ui.OK(viewOverview, "Domain Overview — "+s.domain, s.baseInfo(""), []string{"Check", "Result"}, rows, "Check", detailsActions()...), nil
}

func (s *Service) viewRecordsLocked() (pluginrpc.ViewData, error) {
	s.ensureDNSLocked()
	rep := s.report
	rows := make([][]string, 0, len(rep.Records))
	for _, r := range rep.Records {
		if s.typeFilter != "" && !strings.EqualFold(r.Type, s.typeFilter) {
			continue
		}
		if r.Section != "ANSWER" && s.typeFilter == "" {
			// Keep authority SOA/NS visible; skip glue-only extras unless filtered.
			if r.Section == "ADDITIONAL" {
				continue
			}
		}
		rows = append(rows, []string{r.Section, r.Type, r.Name, formatTTL(r.TTL), r.Data})
	}
	rows = pluginrpc.EnsureRows(rows, pluginrpc.DashRow(5, "No records (try another type or resolver)"))
	extra := ""
	if s.typeFilter != "" {
		extra = "Filter: " + s.typeFilter
	}
	title := "DNS Records — " + s.domain
	return ui.OK(viewRecords, title, s.baseInfo(extra), []string{"Section", "Type", "Name", "TTL", "Data"}, rows, "Type", recordsActions()...), nil
}

func (s *Service) viewNameserversLocked() (pluginrpc.ViewData, error) {
	s.ensureDNSLocked()
	rep := s.report
	var rows [][]string
	rows = append(rows, []string{"[yellow::b]Resolver", "", "", "", ""})
	rows = append(rows, []string{"QUERY", "NS", exchangeServer(s.resolver), "-", resolverLabel(s.resolver)})
	for _, r := range recordsOf(rep.Records, "NS") {
		rows = append(rows, []string{r.Section, r.Type, r.Name, formatTTL(r.TTL), r.Data})
	}
	for _, r := range recordsOf(rep.Records, "SOA") {
		rows = append(rows, []string{r.Section, r.Type, r.Name, formatTTL(r.TTL), r.Data})
	}
	for _, r := range rep.Records {
		if r.Section == "AUTHORITY" {
			rows = append(rows, []string{r.Section, r.Type, r.Name, formatTTL(r.TTL), r.Data})
		}
	}
	rows = pluginrpc.EnsureRows(rows, pluginrpc.DashRow(5, "No NS/SOA"))
	return ui.OK(viewNameservers, "Nameservers — "+s.domain, s.baseInfo(""), []string{"Section", "Type", "Name", "TTL", "Data"}, rows, "Type", recordsActions()...), nil
}

func (s *Service) viewSSLLocked() (pluginrpc.ViewData, error) {
	s.ensureSSLLocked()
	rep := s.report
	var rows [][]string
	ssl := rep.SSL
	if ssl == nil {
		rows = [][]string{{"SSL", "not checked"}}
	} else if !ssl.OK || ssl.Leaf == nil {
		err := ssl.Error
		if err == "" {
			err = "no certificate"
		}
		rows = [][]string{{"Error", err}}
	} else {
		leaf := ssl.Leaf
		rows = append(rows,
			[]string{"Status", sslStatusWord(leaf.NotAfter, ssl.Verified, "")},
			[]string{"Verified", boolMark(ssl.Verified)},
			[]string{"Connect", ssl.ConnectHost},
			[]string{"Protocol", ssl.Version},
			[]string{"Cipher", ssl.Cipher},
			[]string{"ALPN", orDash(ssl.ALPN)},
			[]string{"Subject", leaf.Subject},
			[]string{"Issuer", leaf.Issuer},
			[]string{"Not before", leaf.NotBefore.UTC().Format("2006-01-02 15:04:05 UTC")},
			[]string{"Not after", sslExpiryLabel(leaf.NotAfter)},
			[]string{"Serial", leaf.Serial},
			[]string{"Signature", leaf.SigAlgo},
			[]string{"SANs", orDash(strings.Join(leaf.SANs, ", "))},
			[]string{"Self-signed", boolMark(leaf.SelfSigned)},
		)
		if ssl.Error != "" && !ssl.Verified {
			rows = append(rows, []string{"Verify", "[yellow]" + ssl.Error + "[white]"})
		}
		if len(ssl.Chain) > 1 {
			rows = append(rows, []string{"[yellow::b]Chain", ""})
			for _, c := range ssl.Chain {
				role := "leaf"
				if c.Position > 0 {
					role = fmt.Sprintf("inter %d", c.Position)
				}
				if c.IsCA && c.Position > 0 {
					role = fmt.Sprintf("CA %d", c.Position)
				}
				rows = append(rows, []string{role, shortDN(c.Subject) + "  exp " + c.NotAfter.UTC().Format("2006-01-02")})
			}
		}
	}
	return ui.OK(viewSSL, "TLS / SSL — "+s.domain, s.baseInfo(""), []string{"Field", "Value"}, rows, "Field", detailsActions()...), nil
}

func (s *Service) viewMailLocked() (pluginrpc.ViewData, error) {
	s.ensureMailTXTLocked()
	rep := s.report
	var rows [][]string
	for _, r := range recordsOf(rep.Records, "MX") {
		rows = append(rows, []string{"MX", r.Name, formatTTL(r.TTL), r.Data})
	}
	for _, r := range recordsOf(rep.Records, "TXT") {
		kind := "TXT"
		switch {
		case looksLikeSPF(r.Data):
			kind = "SPF"
		case looksLikeDMARC(r.Data):
			kind = "DMARC"
		case looksLikeBIMI(r.Data):
			kind = "BIMI"
		case looksLikeDKIM(r.Data) || strings.Contains(strings.ToLower(r.Name), "_domainkey"):
			kind = "DKIM"
		default:
			continue
		}
		rows = append(rows, []string{kind, r.Name, formatTTL(r.TTL), r.Data})
	}
	for _, r := range rep.Records {
		if r.Type != "TXT" {
			continue
		}
		low := strings.ToLower(r.Name)
		if strings.HasPrefix(low, "_dmarc.") || strings.Contains(low, "_domainkey") || strings.Contains(low, "_bimi") {
			if looksLikeSPF(r.Data) || looksLikeDMARC(r.Data) || looksLikeDKIM(r.Data) || looksLikeBIMI(r.Data) {
				continue
			}
			rows = append(rows, []string{"TXT", r.Name, formatTTL(r.TTL), r.Data})
		}
	}
	rows = pluginrpc.EnsureRows(rows, []string{"-", "-", "-", "No MX / SPF / DKIM / DMARC"})
	return ui.OK(viewMail, "Mail — "+s.domain, s.baseInfo(""), []string{"Kind", "Name", "TTL", "Data"}, rows, "Kind", recordsActions()...), nil
}

func (s *Service) viewHTTPLocked() (pluginrpc.ViewData, error) {
	s.ensureHTTPLocked()
	h := s.report.HTTP
	var rows [][]string
	if h == nil {
		rows = [][]string{{"HTTP", "not checked"}}
	} else if !h.OK {
		rows = [][]string{{"Error", h.Error}}
	} else {
		rows = append(rows,
			[]string{"Status", colorStatus(h.Status)},
			[]string{"Final URL", h.FinalURL},
			[]string{"Server", orDash(h.Server)},
			[]string{"Content-Type", orDash(h.ContentType)},
			[]string{"HSTS", orDash(h.HSTS)},
			[]string{"CSP", orDash(truncate(h.CSP, 120))},
			[]string{"X-Frame-Options", orDash(h.XFO)},
			[]string{"X-Content-Type-Options", orDash(h.XCTO)},
			[]string{"Referrer-Policy", orDash(h.Referrer)},
		)
		if len(h.Hops) > 0 {
			rows = append(rows, []string{"[yellow::b]Redirects", ""})
			for i, hop := range h.Hops {
				st := "-"
				if hop.Status > 0 {
					st = colorStatus(hop.Status)
				}
				rows = append(rows, []string{fmt.Sprintf("hop %d", i+1), st + "  " + hop.URL})
			}
		}
		if len(h.HeaderLines) > 0 {
			rows = append(rows, []string{"[yellow::b]Headers", ""})
			for _, kv := range h.HeaderLines {
				rows = append(rows, []string{kv[0], truncate(kv[1], 140)})
			}
		}
	}
	return ui.OK(viewHTTP, "HTTP — "+s.domain, s.baseInfo(""), []string{"Field", "Value"}, rows, "Field", detailsActions()...), nil
}

func (s *Service) viewDNSSECLocked() (pluginrpc.ViewData, error) {
	s.ensureDNSLocked()
	rep := s.report
	var rows [][]string
	found := false
	for _, t := range []string{"DS", "DNSKEY", "RRSIG", "NSEC", "NSEC3"} {
		for _, r := range recordsOf(rep.Records, t) {
			found = true
			rows = append(rows, []string{r.Type, r.Name, formatTTL(r.TTL), r.Data})
		}
	}
	if !found {
		// also show authority DS-less hint
		rows = append(rows, []string{"DS", s.domain, "-", "[yellow]no DS in parent — zone likely unsigned[white]"})
	}
	return ui.OK(viewDNSSEC, "DNSSEC — "+s.domain, s.baseInfo(""), []string{"Type", "Name", "TTL", "Data"}, rows, "Type", recordsActions()...), nil
}

func (s *Service) viewReverseLocked() (pluginrpc.ViewData, error) {
	s.ensureDNSLocked()
	rep := s.report
	ips := allIPsFromRecords(rep.Records)
	var rows [][]string
	if len(ips) == 0 {
		rows = [][]string{{"-", "-", "No A/AAAA to reverse"}}
	} else {
		for _, ip := range ips {
			ptrs := lookupPTR(ip, s.resolver)
			if len(ptrs) == 0 {
				rows = append(rows, []string{ip, ptrName(ip), "[yellow]no PTR[white]"})
				continue
			}
			for _, p := range ptrs {
				rows = append(rows, []string{ip, p.Name, p.Data})
			}
		}
	}
	return ui.OK(viewReverse, "Reverse DNS — "+s.domain, s.baseInfo(""), []string{"IP", "PTR name", "Target"}, rows, "IP", detailsActions()...), nil
}

func (s *Service) viewReachabilityLocked() (pluginrpc.ViewData, error) {
	s.ensurePortsLocked()
	var rows [][]string
	for _, p := range s.report.Ports {
		state := "[red]closed[white]"
		detail := p.Error
		if p.Open {
			state = "[green]open[white]"
			detail = formatDuration(p.Latency)
		}
		rows = append(rows, []string{fmt.Sprintf("%d", p.Port), p.Name, state, detail})
	}
	rows = pluginrpc.EnsureRows(rows, pluginrpc.DashRow(4, "No probes"))
	return ui.OK(viewReachability, "Reachability — "+s.domain, s.baseInfo(""), []string{"Port", "Service", "State", "Detail"}, rows, "Port", detailsActions()...), nil
}

func (s *Service) viewWhoisLocked() (pluginrpc.ViewData, error) {
	s.ensureWhoisLocked()
	w := s.report.Whois
	var rows [][]string
	if w == nil {
		rows = [][]string{{"WHOIS", "not checked"}}
	} else if !w.OK {
		rows = [][]string{{"Error", w.Error}}
	} else {
		rows = append(rows,
			[]string{"Server", orDash(w.Server)},
			[]string{"Registrar", orDash(w.Registrar)},
			[]string{"Created", orDash(w.Created)},
			[]string{"Updated", orDash(w.Updated)},
			[]string{"Expires", orDash(w.Expires)},
			[]string{"Status", orDash(truncate(w.Status, 120))},
			[]string{"Name servers", orDash(strings.Join(w.NameServers, ", "))},
		)
		if w.Raw != "" {
			rows = append(rows, []string{"[yellow::b]Raw", ""})
			for _, line := range strings.Split(w.Raw, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "%") || strings.HasPrefix(line, "#") {
					continue
				}
				if kv := strings.SplitN(line, ":", 2); len(kv) == 2 && len(kv[0]) < 40 {
					rows = append(rows, []string{strings.TrimSpace(kv[0]), truncate(strings.TrimSpace(kv[1]), 140)})
					continue
				}
				rows = append(rows, []string{"", truncate(line, 140)})
				if len(rows) > 80 {
					rows = append(rows, []string{"", "… truncated — press E for full dump"})
					break
				}
			}
		}
	}
	return ui.OK(viewWhois, "WHOIS — "+s.domain, s.baseInfo(""), []string{"Field", "Value"}, rows, "Field", detailsActions()...), nil
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func shortDN(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.Index(s, "CN="); i >= 0 {
		rest := s[i+3:]
		if j := strings.Index(rest, ","); j >= 0 {
			rest = rest[:j]
		}
		return rest
	}
	return truncate(s, 60)
}

func joinSlash(parts ...string) string {
	var out []string
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " / ")
}
