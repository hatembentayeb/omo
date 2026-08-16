package dnscheck

import (
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"
)

func systemResolver() string {
	cfg, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err == nil && len(cfg.Servers) > 0 {
		port := cfg.Port
		if port == "" {
			port = "53"
		}
		return net.JoinHostPort(cfg.Servers[0], port)
	}
	if host := strings.TrimSpace(os.Getenv("OMO_DNS_RESOLVER")); host != "" {
		return resolverAddr(host)
	}
	return "8.8.8.8:53"
}

func exchangeServer(resolver string) string {
	if resolver == "" || strings.EqualFold(resolver, "system") {
		return systemResolver()
	}
	return resolverAddr(resolver)
}

func queryType(name string, qtype uint16, resolver string, dnssec bool) (*dns.Msg, time.Duration, error) {
	server := exchangeServer(resolver)
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), qtype)
	m.RecursionDesired = true
	if dnssec {
		m.SetEdns0(4096, true)
	}
	c := new(dns.Client)
	c.Timeout = queryTimeout
	c.UDPSize = 4096
	in, rtt, err := c.Exchange(m, server)
	if err != nil {
		// retry over TCP (truncation / UDP blocked)
		c.Net = "tcp"
		in, rtt, err = c.Exchange(m, server)
	}
	return in, rtt, err
}

func collectRecords(name string, qtype uint16, resolver string, dnssec bool) ([]dnsRecord, time.Duration, error) {
	in, rtt, err := queryType(name, qtype, resolver, dnssec)
	if err != nil {
		return nil, rtt, err
	}
	if in == nil {
		return nil, rtt, fmt.Errorf("empty DNS response")
	}
	var out []dnsRecord
	out = append(out, sectionRecords("ANSWER", in.Answer)...)
	out = append(out, sectionRecords("AUTHORITY", in.Ns)...)
	out = append(out, sectionRecords("ADDITIONAL", in.Extra)...)
	return out, rtt, nil
}

func sectionRecords(section string, rr []dns.RR) []dnsRecord {
	out := make([]dnsRecord, 0, len(rr))
	for _, r := range rr {
		if r == nil {
			continue
		}
		hdr := r.Header()
		if hdr.Rrtype == dns.TypeOPT {
			continue
		}
		out = append(out, dnsRecord{
			Section: section,
			Type:    dns.TypeToString[hdr.Rrtype],
			Name:    strings.TrimSuffix(hdr.Name, "."),
			TTL:     hdr.Ttl,
			Data:    strings.TrimSpace(strings.TrimPrefix(r.String(), hdr.String())),
		})
	}
	return out
}

func lookupRecords(domain, resolver, typeFilter string) ([]dnsRecord, queryMeta) {
	meta := queryMeta{Server: exchangeServer(resolver)}
	start := time.Now()

	types := recordTypesToQuery(typeFilter)
	var (
		mu   sync.Mutex
		all  []dnsRecord
		errs []string
		max  time.Duration
	)
	var wg sync.WaitGroup
	for _, qt := range types {
		qt := qt
		name := domain
		wg.Add(1)
		go func() {
			defer wg.Done()
			recs, rtt, err := collectRecords(name, qt, resolver, qt == dns.TypeDS || qt == dns.TypeDNSKEY || qt == dns.TypeRRSIG)
			mu.Lock()
			defer mu.Unlock()
			if rtt > max {
				max = rtt
			}
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v", dns.TypeToString[qt], err))
				return
			}
			all = append(all, recs...)
		}()
	}

	// SRV lookups (only when not filtering to a single non-SRV type)
	if typeFilter == "" || strings.EqualFold(typeFilter, "SRV") {
		for _, spec := range srvSpecs {
			spec := spec
			wg.Add(1)
			go func() {
				defer wg.Done()
				qname := fmt.Sprintf("_%s._%s.%s", spec.Service, spec.Proto, domain)
				recs, rtt, err := collectRecords(qname, dns.TypeSRV, resolver, false)
				mu.Lock()
				defer mu.Unlock()
				if rtt > max {
					max = rtt
				}
				if err != nil {
					return
				}
				for _, rec := range recs {
					if rec.Type == "SRV" {
						all = append(all, rec)
					}
				}
			}()
		}
	}

	wg.Wait()
	meta.Duration = time.Since(start)
	if max > 0 && meta.Duration < max {
		meta.Duration = max
	}
	if len(errs) > 0 && len(all) == 0 {
		meta.Error = strings.Join(errs, "; ")
	}
	return dedupeRecords(all), meta
}

func recordTypesToQuery(filter string) []uint16 {
	f := strings.ToUpper(strings.TrimSpace(filter))
	if f != "" && f != "ALL" && f != "ANY" {
		if code, ok := dns.StringToType[f]; ok {
			return []uint16{code}
		}
	}
	return []uint16{
		dns.TypeA, dns.TypeAAAA, dns.TypeCNAME, dns.TypeMX, dns.TypeTXT,
		dns.TypeNS, dns.TypeSOA, dns.TypeCAA, dns.TypeHTTPS, dns.TypeSVCB,
		dns.TypeDS, dns.TypeDNSKEY,
	}
}

func dedupeRecords(in []dnsRecord) []dnsRecord {
	seen := make(map[string]bool, len(in))
	out := make([]dnsRecord, 0, len(in))
	for _, r := range in {
		key := r.Section + "|" + r.Type + "|" + r.Name + "|" + r.Data
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, r)
	}
	order := map[string]int{}
	for i, t := range recordTypeOrder {
		order[t] = i
	}
	sectionOrder := map[string]int{"ANSWER": 0, "AUTHORITY": 1, "ADDITIONAL": 2}
	sortRecords(out, func(a, b dnsRecord) bool {
		sa, sb := sectionOrder[a.Section], sectionOrder[b.Section]
		if sa != sb {
			return sa < sb
		}
		ta, oka := order[a.Type]
		tb, okb := order[b.Type]
		if !oka {
			ta = 100
		}
		if !okb {
			tb = 100
		}
		if ta != tb {
			return ta < tb
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Data < b.Data
	})
	return out
}

func sortRecords(recs []dnsRecord, less func(a, b dnsRecord) bool) {
	// insertion sort keeps this file stdlib-light; n is small
	for i := 1; i < len(recs); i++ {
		j := i
		for j > 0 && less(recs[j], recs[j-1]) {
			recs[j], recs[j-1] = recs[j-1], recs[j]
			j--
		}
	}
}

func recordsOf(recs []dnsRecord, typ string) []dnsRecord {
	typ = strings.ToUpper(typ)
	var out []dnsRecord
	for _, r := range recs {
		if r.Type == typ && r.Section == "ANSWER" {
			out = append(out, r)
		}
	}
	return out
}

func recordValues(recs []dnsRecord, typ string) []string {
	var out []string
	for _, r := range recordsOf(recs, typ) {
		out = append(out, r.Data)
	}
	return out
}

func hasType(recs []dnsRecord, typ string) bool {
	return len(recordsOf(recs, typ)) > 0
}

func txtRecordsNamed(recs []dnsRecord, name string) []dnsRecord {
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	var out []dnsRecord
	for _, r := range recs {
		if r.Type == "TXT" && strings.EqualFold(strings.TrimSuffix(r.Name, "."), name) {
			out = append(out, r)
		}
	}
	return out
}

func lookupTXTName(name, resolver string) []dnsRecord {
	recs, _, err := collectRecords(name, dns.TypeTXT, resolver, false)
	if err != nil {
		return nil
	}
	return recordsOf(recs, "TXT")
}

func lookupPTR(ip, resolver string) []dnsRecord {
	name := ptrName(ip)
	if name == "" {
		return nil
	}
	recs, _, err := collectRecords(name, dns.TypePTR, resolver, false)
	if err != nil {
		return nil
	}
	return recordsOf(recs, "PTR")
}

func mailExtras(domain, resolver string) []dnsRecord {
	var out []dnsRecord
	dmarc := "_dmarc." + domain
	out = append(out, lookupTXTName(dmarc, resolver)...)
	out = append(out, lookupTXTName("default._bimi."+domain, resolver)...)
	for _, sel := range dkimSelectors {
		out = append(out, lookupTXTName(fmt.Sprintf("%s._domainkey.%s", sel, domain), resolver)...)
	}
	return dedupeRecords(out)
}
