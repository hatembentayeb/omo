package dnscheck

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

// Service is the RPC-facing DNS / TLS / domain inspector (no tview).
type Service struct {
	mu          sync.Mutex
	domain      string
	resolver    string
	typeFilter  string
	currentView string
	report      *domainReport
}

func NewService() *Service {
	return &Service{
		resolver:    defaultResolvers[0].Addr,
		currentView: viewOverview,
	}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "dnscheck",
		Version:     "1.0.0",
		Description: "Dig-style DNS lookup, SSL expiry, mail auth, HTTP, and WHOIS for any domain",
		Author:      "OhMyOps Team",
		License:     "MIT",
		Tags:        []string{"dns", "ssl", "tls", "domain", "dig", "whois", "networking"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/dnscheck",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("dnscheck.Configure begin")
	if req.Settings == nil {
		return nil
	}
	domain := normalizeDomain(firstNonEmpty(req.Settings["domain"], req.Settings["url"], req.Settings["host"]))
	if isIP(domain) || (domain != "" && !strings.Contains(domain, "://")) {
		if domain != s.domain {
			s.domain = domain
			s.report = nil
		}
	}
	if r := strings.TrimSpace(firstNonEmpty(req.Settings["resolver"], req.Settings["nameserver"])); r != "" {
		s.resolver = r
	}
	pluginrpc.RPCLog("dnscheck.Configure domain=%s resolver=%s", s.domain, s.resolver)
	return nil
}

func (s *Service) GetView(req pluginrpc.ViewRequest) (pluginrpc.ViewData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.View == pluginrpc.DashboardView {
		return s.viewDashboardLocked()
	}
	viewID := req.View
	if viewID == "" {
		viewID = s.currentView
	}
	if viewID == "" {
		viewID = viewOverview
	}
	pluginrpc.RPCLog("dnscheck.GetView begin view=%s domain=%s", viewID, s.domain)
	start := time.Now()
	view, err := s.buildViewLocked(viewID)
	if err != nil {
		pluginrpc.RPCLog("dnscheck.GetView err=%v", err)
		return pluginrpc.ViewData{}, err
	}
	pluginrpc.RPCLog("dnscheck.GetView OK view=%s rows=%d dur=%s", view.View, len(view.Rows), time.Since(start))
	return view, nil
}

func (s *Service) DoAction(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	action := req.Action
	if strings.HasPrefix(action, "goto_") {
		viewID := strings.TrimPrefix(action, "goto_")
		view, err := s.buildViewLocked(viewID)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "switched to " + viewID, Next: &view}, nil
	}

	switch action {
	case "refresh", "":
		s.invalidateLocked(s.currentView)
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "refreshed", Next: &view}, nil

	case "lookup_domain":
		domain := normalizeDomain(firstNonEmpty(req.Payload["domain"], req.Payload["key"], req.Payload["col0"]))
		if domain == "" {
			return pluginrpc.ActionResult{OK: false, Message: "domain required"}, nil
		}
		s.domain = domain
		s.report = nil
		s.currentView = viewOverview
		view, err := s.buildViewLocked(viewOverview)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "looked up " + domain, Next: &view, Reaction: "dig!"}, nil

	case "set_resolver":
		r := strings.TrimSpace(firstNonEmpty(req.Payload["resolver"], req.Payload["nameserver"], req.Payload["key"]))
		if r == "" {
			return pluginrpc.ActionResult{OK: false, Message: "resolver required"}, nil
		}
		s.resolver = r
		s.report = nil
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "resolver " + resolverLabel(s.resolver), Next: &view}, nil

	case "cycle_resolver":
		s.resolver = cycleResolver(s.resolver)
		s.report = nil
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "resolver " + resolverLabel(s.resolver), Next: &view}, nil

	case "filter_record_type":
		s.typeFilter = strings.ToUpper(strings.TrimSpace(req.Payload["type"]))
		if s.report != nil {
			s.report.viewFetched[viewRecords] = false
		}
		view, err := s.buildViewLocked(viewRecords)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		msg := "all types"
		if s.typeFilter != "" {
			msg = "type " + s.typeFilter
		}
		return pluginrpc.ActionResult{OK: true, Message: msg, Next: &view}, nil

	case "clear_record_filters":
		s.typeFilter = ""
		if s.report != nil {
			s.report.viewFetched[viewRecords] = false
		}
		view, err := s.buildViewLocked(viewRecords)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "filters cleared", Next: &view}, nil

	case "record_details", "row_details":
		title, body := s.rowDetailsLocked(req.Payload)
		return pluginrpc.ActionResult{OK: true, Message: "details", ModalTitle: title, ModalBody: body}, nil

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) Stop() error {
	return nil
}

func (s *Service) invalidateLocked(viewID string) {
	if s.report == nil {
		return
	}
	if viewID == "" || viewID == viewOverview {
		s.report = nil
		return
	}
	if s.report.viewFetched != nil {
		s.report.viewFetched[viewID] = false
		if viewID == viewRecords {
			s.report.Records = nil
		}
	}
}

func (s *Service) ensureReportLocked() *domainReport {
	if s.report == nil || s.report.Domain != s.domain || s.report.Resolver != s.resolver {
		s.report = &domainReport{
			Domain:      s.domain,
			Resolver:    s.resolver,
			QueriedAt:   time.Now(),
			viewFetched: map[string]bool{},
		}
	}
	return s.report
}

func (s *Service) ensureDNSLocked() {
	if s.domain == "" {
		return
	}
	rep := s.ensureReportLocked()
	if len(rep.Records) > 0 && rep.viewFetched[viewRecords] {
		return
	}
	recs, meta := lookupRecords(s.domain, s.resolver, s.typeFilter)
	rep.Records = recs
	rep.DNSMeta = meta
	rep.QueriedAt = time.Now()
	rep.viewFetched[viewRecords] = true
}

func (s *Service) ensureMailTXTLocked() {
	if s.domain == "" {
		return
	}
	rep := s.ensureReportLocked()
	if rep.viewFetched[viewMail] {
		return
	}
	s.ensureDNSLocked()
	extra := mailExtras(s.domain, s.resolver)
	rep.Records = dedupeRecords(append(rep.Records, extra...))
	rep.viewFetched[viewMail] = true
}

func (s *Service) ensureSSLLocked() {
	if s.domain == "" {
		return
	}
	rep := s.ensureReportLocked()
	if rep.viewFetched[viewSSL] {
		return
	}
	s.ensureDNSLocked()
	ips := ipv4FromRecords(rep.Records)
	if len(ips) == 0 {
		ips = allIPsFromRecords(rep.Records)
	}
	rep.SSL = checkTLS(s.domain, ips)
	rep.viewFetched[viewSSL] = true
}

func (s *Service) ensureHTTPLocked() {
	if s.domain == "" {
		return
	}
	rep := s.ensureReportLocked()
	if rep.viewFetched[viewHTTP] {
		return
	}
	rep.HTTP = checkHTTP(s.domain)
	rep.viewFetched[viewHTTP] = true
}

func (s *Service) ensureWhoisLocked() {
	if s.domain == "" {
		return
	}
	rep := s.ensureReportLocked()
	if rep.viewFetched[viewWhois] {
		return
	}
	rep.Whois = checkWhois(s.domain)
	rep.viewFetched[viewWhois] = true
}

func (s *Service) ensurePortsLocked() {
	if s.domain == "" {
		return
	}
	rep := s.ensureReportLocked()
	if rep.viewFetched[viewReachability] {
		return
	}
	rep.Ports = checkPorts(s.domain)
	rep.viewFetched[viewReachability] = true
}

func (s *Service) ensureOverviewLocked() {
	if s.domain == "" {
		return
	}
	s.ensureDNSLocked()
	rep := s.report
	needSSL := !rep.viewFetched[viewSSL]
	needHTTP := !rep.viewFetched[viewHTTP]
	if !needSSL && !needHTTP {
		return
	}
	var (
		wg   sync.WaitGroup
		ssl  *sslReport
		http *httpReport
	)
	if needSSL {
		ips := ipv4FromRecords(rep.Records)
		if len(ips) == 0 {
			ips = allIPsFromRecords(rep.Records)
		}
		domain := s.domain
		wg.Add(1)
		go func() {
			defer wg.Done()
			ssl = checkTLS(domain, ips)
		}()
	}
	if needHTTP {
		domain := s.domain
		wg.Add(1)
		go func() {
			defer wg.Done()
			http = checkHTTP(domain)
		}()
	}
	wg.Wait()
	if needSSL {
		rep.SSL = ssl
		rep.viewFetched[viewSSL] = true
	}
	if needHTTP {
		rep.HTTP = http
		rep.viewFetched[viewHTTP] = true
	}
}

func (s *Service) rowDetailsLocked(payload map[string]string) (string, string) {
	if payload == nil {
		payload = map[string]string{}
	}
	key := firstNonEmpty(payload["key"], payload["col0"])
	var b strings.Builder
	title := "Details"
	switch s.currentView {
	case viewRecords, viewNameservers, viewMail, viewDNSSEC, viewReverse:
		title = firstNonEmpty(payload["col1"], key, "Record")
		fmt.Fprintf(&b, "Section:  %s\n", payload["col0"])
		fmt.Fprintf(&b, "Type:     %s\n", payload["col1"])
		fmt.Fprintf(&b, "Name:     %s\n", payload["col2"])
		fmt.Fprintf(&b, "TTL:      %s\n", payload["col3"])
		fmt.Fprintf(&b, "Data:     %s\n", payload["col4"])
		if m := meaningOf(payload["col1"]); m != "" {
			fmt.Fprintf(&b, "Meaning:  %s\n", m)
		}
	case viewSSL:
		title = "Certificate"
		fmt.Fprintf(&b, "Field: %s\nValue: %s\n", payload["col0"], payload["col1"])
		if s.report != nil && s.report.SSL != nil && s.report.SSL.Leaf != nil {
			c := s.report.SSL.Leaf
			fmt.Fprintf(&b, "\nSubject:    %s\nIssuer:     %s\nNot before: %s\nNot after:  %s\nSerial:     %s\nSig:        %s\nSANs:       %s\n",
				c.Subject, c.Issuer,
				c.NotBefore.UTC().Format(time.RFC3339),
				c.NotAfter.UTC().Format(time.RFC3339),
				c.Serial, c.SigAlgo, strings.Join(c.SANs, ", "))
		}
	case viewHTTP:
		title = firstNonEmpty(payload["col0"], "HTTP")
		fmt.Fprintf(&b, "%s\n%s\n", payload["col0"], payload["col1"])
	case viewWhois:
		title = "WHOIS"
		fmt.Fprintf(&b, "%s: %s\n", payload["col0"], payload["col1"])
		if s.report != nil && s.report.Whois != nil && s.report.Whois.Raw != "" {
			b.WriteString("\n")
			b.WriteString(truncate(s.report.Whois.Raw, 4000))
		}
	default:
		fmt.Fprintf(&b, "%s\n%s\n", payload["col0"], payload["col1"])
		if payload["col2"] != "" {
			fmt.Fprintf(&b, "%s\n", payload["col2"])
		}
	}
	return title, strings.TrimSpace(b.String())
}
