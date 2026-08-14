package bunnydns

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/pluginrpc"
)

type availCheck struct {
	Domain    string
	Available string
	Message   string
	When      string
}

// Service is the Bunny DNS RPC plugin.
type Service struct {
	mu               sync.Mutex
	client           *Client
	accessKey        string
	baseURL          string
	name             string
	currentView      string
	selectedZone     *DnsZone
	recordTypeFilter string
	recordSearch     string
	lastScanJSON     string
	lastDNSSEC       *DnsSecInfo
	lastCertMsg      string
	availHistory     []availCheck
}

func NewService() *Service {
	return &Service{currentView: viewZones, baseURL: apiBaseDefault}
}

func (s *Service) GetMetadata() (pluginapi.PluginMetadata, error) {
	return pluginapi.PluginMetadata{
		Name:        "bunnydns",
		Version:     "1.0.0",
		Description: "Manage Bunny.net DNS zones, records, DNSSEC, stats, and certificates",
		Author:      "OhMyOps",
		License:     "MIT",
		Tags:        []string{"dns", "bunny", "networking", "domains"},
		Arch:        []string{"amd64", "arm64"},
		LastUpdated: time.Now(),
		URL:         "https://github.com/hatembentayeb/omo/plugins/bunnydns",
	}, nil
}

func (s *Service) Configure(req pluginrpc.ConfigureRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	pluginrpc.RPCLog("bunnydns.Configure begin")
	if req.Settings == nil {
		return fmt.Errorf("missing settings")
	}
	key := firstNonEmpty(req.Settings["password"], req.Settings["access_key"], req.Settings["api_key"], req.Settings["AccessKey"])
	if key == "" {
		return fmt.Errorf("AccessKey required — set KeePass Password (or access_key attr)")
	}
	base := firstNonEmpty(req.Settings["url"], req.Settings["host"], req.Settings["api_base"], apiBaseDefault)
	if !strings.HasPrefix(base, "http") {
		base = apiBaseDefault
	}
	s.accessKey = key
	s.baseURL = strings.TrimRight(base, "/")
	s.name = firstNonEmpty(req.Settings["name"], req.Settings["title"], "bunny")
	s.client = NewClient(s.accessKey, s.baseURL)
	s.selectedZone = nil
	s.availHistory = nil
	s.lastDNSSEC = nil
	s.lastCertMsg = ""
	pluginrpc.RPCLog("bunnydns.Configure ok name=%s base=%s", s.name, s.baseURL)
	return nil
}

func (s *Service) GetView(req pluginrpc.ViewRequest) (pluginrpc.ViewData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	viewID := req.View
	if viewID == "" {
		viewID = s.currentView
	}
	if viewID == "" {
		viewID = viewZones
	}
	return s.buildViewLocked(viewID)
}

func (s *Service) DoAction(req pluginrpc.ActionRequest) (pluginrpc.ActionResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	action := req.Action
	if strings.HasPrefix(action, "goto_") {
		viewID := strings.TrimPrefix(action, "goto_")
		view, err := s.buildViewLocked(viewID)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "switched to " + viewID, Next: &view}, nil
	}

	switch action {
	case "refresh", "":
		view, err := s.buildViewLocked(s.currentView)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		return pluginrpc.ActionResult{OK: true, Message: "refreshed", Next: &view, Reaction: "fresh"}, nil

	case "select_zone":
		return s.actionSelectZoneLocked(req.Payload)
	case "create_zone":
		return s.actionCreateZoneLocked(req.Payload)
	case "delete_zone":
		return s.actionDeleteZoneLocked(req.Payload)
	case "delete":
		if s.currentView == viewRecords {
			return s.actionDeleteRecordLocked(req.Payload)
		}
		return s.actionDeleteZoneLocked(req.Payload)
	case "zone_details":
		return s.actionZoneDetailsLocked(req.Payload)

	case "create_record":
		return s.actionCreateRecordLocked(req.Payload)
	case "update_record":
		return s.actionUpdateRecordLocked(req.Payload)
	case "delete_record":
		return s.actionDeleteRecordLocked(req.Payload)
	case "record_details":
		return s.actionRecordDetailsLocked(req.Payload)
	case "filter_record_type":
		s.recordTypeFilter = strings.ToUpper(strings.TrimSpace(req.Payload["type"]))
		view, err := s.buildViewLocked(viewRecords)
		if err != nil {
			return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
		}
		msg := "showing all types"
		if s.recordTypeFilter != "" {
			msg = "filtered type " + s.recordTypeFilter
		}
		return pluginrpc.ActionResult{OK: true, Message: msg, Next: &view, Reaction: "filter"}, nil
	case "search_records":
		// Host handles S as local CoreView table filter — no API.
		return pluginrpc.ActionResult{OK: true, Message: "use table filter"}, nil
	case "clear_record_filters":
		s.recordTypeFilter = ""
		s.recordSearch = ""
		view, _ := s.buildViewLocked(viewRecords)
		return pluginrpc.ActionResult{OK: true, Message: "filters cleared", Next: &view, Reaction: "ok"}, nil

	case "set_soa_email":
		return s.actionSetSoaEmailLocked(req.Payload)
	case "toggle_logging":
		return s.actionToggleLoggingLocked()
	case "toggle_custom_ns":
		return s.actionToggleCustomNSLocked()

	case "enable_dnssec":
		return s.actionEnableDNSSECLocked()
	case "disable_dnssec":
		return s.actionDisableDNSSECLocked()
	case "dnssec_details":
		return s.actionDNSSECDetailsLocked()

	case "export_zone", "export_zones":
		return s.actionExportZonesLocked()
	case "export_records":
		return s.actionExportRecordsLocked()
	case "trigger_scan":
		return s.actionTriggerScanLocked()
	case "view_scan":
		return s.actionViewScanLocked()

	case "check_availability":
		return s.actionCheckAvailabilityLocked(req.Payload)
	case "availability_details":
		return s.actionAvailabilityDetailsLocked(req.Payload)
	case "clear_availability":
		s.availHistory = nil
		view, _ := s.buildViewLocked(viewAvailability)
		return pluginrpc.ActionResult{OK: true, Message: "history cleared", Next: &view, Reaction: "ok"}, nil

	case "issue_wildcard_cert":
		return s.actionIssueWildcardLocked()

	default:
		return pluginrpc.ActionResult{OK: false, Message: "unknown action: " + action}, nil
	}
}

func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.client = nil
	return nil
}

func (s *Service) ensureClientLocked() error {
	if s.client == nil || !s.client.Connected() {
		return fmt.Errorf("not configured — set KeePass Password to your Bunny AccessKey")
	}
	return nil
}

func (s *Service) requireZoneLocked() (*DnsZone, error) {
	if err := s.ensureClientLocked(); err != nil {
		return nil, err
	}
	if s.selectedZone == nil || s.selectedZone.Id == 0 {
		return nil, fmt.Errorf("highlight a zone on view 0 (Zones)")
	}
	return s.selectedZone, nil
}

// actionSelectZoneLocked sets the current zone from the highlighted row.
// No view switch — arrow highlight is enough; Enter / O opens Records.
func (s *Service) actionSelectZoneLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	if s.currentView != viewZones {
		// Stale highlight callbacks after leaving Zones must not overwrite the zone.
		return pluginrpc.ActionResult{OK: true, Message: "ignored"}, nil
	}
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	id, err := parseID(firstNonEmpty(payload["id"], payload["key"], payload["col0"]))
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: "highlight a zone row", Reaction: "pick?"}, nil
	}
	domain := firstNonEmpty(payload["col1"], payload["domain"])
	if domain == "" || domain == "—" {
		return pluginrpc.ActionResult{OK: false, Message: "highlight a zone row", Reaction: "pick?"}, nil
	}
	if _, isType := dnsRecordTypeCodes[strings.ToUpper(domain)]; isType {
		return pluginrpc.ActionResult{OK: true, Message: "ignored"}, nil
	}
	if s.selectedZone != nil && s.selectedZone.Id == id {
		if s.selectedZone.Domain == "" {
			s.selectedZone.Domain = domain
		}
		return pluginrpc.ActionResult{OK: true, Message: "zone " + s.selectedZone.Domain}, nil
	}
	s.selectedZone = &DnsZone{Id: id, Domain: domain}
	return pluginrpc.ActionResult{OK: true, Message: "zone " + domain, Reaction: "zone"}, nil
}

func (s *Service) actionCreateZoneLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	domain := strings.TrimSpace(firstNonEmpty(payload["domain"], payload["name"]))
	if domain == "" {
		return pluginrpc.ActionResult{OK: false, Message: "domain required"}, nil
	}
	z, err := s.client.AddZone(domain)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	s.selectedZone = z
	view, _ := s.buildViewLocked(viewZones)
	return pluginrpc.ActionResult{OK: true, Message: "created zone " + domain, Next: &view, Reaction: "created"}, nil
}

func (s *Service) actionDeleteZoneLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	id, err := parseID(firstNonEmpty(payload["id"], payload["key"], payload["col0"]))
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: "select a zone", Reaction: "pick?"}, nil
	}
	domain := firstNonEmpty(payload["col1"], payload["domain"])
	if err := s.client.DeleteZone(id); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	if s.selectedZone != nil && s.selectedZone.Id == id {
		s.selectedZone = nil
	}
	view, _ := s.buildViewLocked(viewZones)
	return pluginrpc.ActionResult{OK: true, Message: "deleted " + domain, Next: &view, Reaction: "poof"}, nil
}

func (s *Service) actionZoneDetailsLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	id, err := parseID(firstNonEmpty(payload["id"], payload["key"], payload["col0"]))
	if err != nil {
		if s.selectedZone != nil {
			id = s.selectedZone.Id
		} else {
			return pluginrpc.ActionResult{OK: false, Message: "select a zone"}, nil
		}
	}
	z, err := s.client.GetZone(id)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	body := fmt.Sprintf(
		"ID:           %d\nDomain:       %s\nCreated:      %s\nModified:     %s\nNS1:          %s\nNS2:          %s\nNS Detected:  %v\nCustom NS:    %v\nSOA Email:    %s\nDNSSEC:       %v\nLogging:      %v\nRecords:      %d\n",
		z.Id, z.Domain, z.DateCreated, z.DateModified, z.Nameserver1, z.Nameserver2,
		z.NameserversDetected, z.CustomNameserversEnabled, z.SoaEmail, z.DnsSecEnabled, z.LoggingEnabled, len(z.Records),
	)
	return pluginrpc.ActionResult{OK: true, ModalTitle: "Zone " + z.Domain, ModalBody: body}, nil
}

func (s *Service) actionCreateRecordLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	typ := strings.ToUpper(strings.TrimSpace(firstNonEmpty(payload["type"], "A")))
	code, ok := parseRecordType(typ)
	if !ok {
		return pluginrpc.ActionResult{OK: false, Message: "unknown type " + typ}, nil
	}
	name := strings.TrimSpace(payload["name"])
	value := strings.TrimSpace(payload["value"])
	if value == "" {
		return pluginrpc.ActionResult{OK: false, Message: "value required"}, nil
	}
	ttl := 300
	if t := strings.TrimSpace(payload["ttl"]); t != "" {
		if n, err := strconv.Atoi(t); err == nil {
			ttl = n
		}
	}
	rec := map[string]any{"Type": code, "Name": name, "Value": value, "Ttl": ttl}
	if p := strings.TrimSpace(payload["priority"]); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			rec["Priority"] = n
		}
	}
	created, err := s.client.AddRecord(z.Id, rec)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	view, _ := s.buildViewLocked(viewRecords)
	return pluginrpc.ActionResult{
		OK: true, Message: fmt.Sprintf("added %s %s", typ, name), Next: &view, Reaction: "yay!",
		ModalTitle: "Record Created",
		ModalBody:  fmt.Sprintf("ID: %d\nType: %s\nName: %s\nValue: %s\nTTL: %d\n", created.Id, typ, name, value, ttl),
	}, nil
}

func (s *Service) actionUpdateRecordLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	id, err := parseID(firstNonEmpty(payload["id"], payload["key"], payload["col0"]))
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: "select a record"}, nil
	}
	value := strings.TrimSpace(payload["value"])
	if value == "" {
		return pluginrpc.ActionResult{OK: false, Message: "value required"}, nil
	}
	patch := map[string]any{"Value": value}
	if t := strings.TrimSpace(payload["ttl"]); t != "" {
		if n, err := strconv.Atoi(t); err == nil {
			patch["Ttl"] = n
		}
	}
	if n := strings.TrimSpace(payload["name"]); n != "" {
		patch["Name"] = n
	}
	if err := s.client.UpdateRecord(z.Id, id, patch); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	view, _ := s.buildViewLocked(viewRecords)
	return pluginrpc.ActionResult{OK: true, Message: "updated record", Next: &view, Reaction: "upd"}, nil
}

func (s *Service) actionDeleteRecordLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	id, err := parseID(firstNonEmpty(payload["id"], payload["key"], payload["col0"]))
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: "select a record"}, nil
	}
	if err := s.client.DeleteRecord(z.Id, id); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	view, _ := s.buildViewLocked(viewRecords)
	return pluginrpc.ActionResult{OK: true, Message: "deleted record", Next: &view, Reaction: "poof"}, nil
}

func (s *Service) actionRecordDetailsLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	id := firstNonEmpty(payload["id"], payload["key"], payload["col0"])
	if id == "" || id == "—" {
		return pluginrpc.ActionResult{OK: false, Message: "highlight a record"}, nil
	}
	// Prefer the highlighted table row — no extra Bunny API round-trip.
	typ := firstNonEmpty(payload["col1"], payload["type"])
	name := firstNonEmpty(payload["col2"], payload["name"])
	value := firstNonEmpty(payload["col3"], payload["value"])
	ttl := firstNonEmpty(payload["col4"], payload["ttl"])
	prio := firstNonEmpty(payload["col5"], payload["priority"])
	dis := firstNonEmpty(payload["col6"], payload["disabled"])
	if typ != "" || name != "" || value != "" {
		body := fmt.Sprintf(
			"ID:       %s\nType:     %s\nName:     %s\nValue:    %s\nTTL:      %s\nPriority: %s\nDisabled: %s\n",
			id, typ, name, value, ttl, prio, dis,
		)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "DNS Record", ModalBody: body}, nil
	}
	z, err := s.requireZoneLocked()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	recID, err := parseID(id)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: "highlight a record"}, nil
	}
	recs, err := s.client.ListRecords(z.Id, "", "")
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	for i := range recs {
		if recs[i].Id != recID {
			continue
		}
		r := &recs[i]
		body := fmt.Sprintf(
			"ID:       %d\nType:     %s\nName:     %s\nValue:    %s\nTTL:      %d\nPriority: %d\nWeight:   %d\nPort:     %d\nDisabled: %v\nComment:  %s\nAccelerated: %v\n",
			r.Id, recordTypeName(r.Type), r.Name, r.Value, r.Ttl, r.Priority, r.Weight, r.Port, r.Disabled, r.Comment, r.Accelerated,
		)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "DNS Record", ModalBody: body}, nil
	}
	return pluginrpc.ActionResult{OK: false, Message: "record not found"}, nil
}

func (s *Service) actionSetSoaEmailLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	email := strings.TrimSpace(payload["email"])
	if email == "" {
		return pluginrpc.ActionResult{OK: false, Message: "email required"}, nil
	}
	if err := s.client.UpdateZone(z.Id, map[string]any{"SoaEmail": email}); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	z.SoaEmail = email
	view, _ := s.buildViewLocked(viewNameservers)
	return pluginrpc.ActionResult{OK: true, Message: "SOA email updated", Next: &view, Reaction: "ok"}, nil
}

func (s *Service) actionToggleLoggingLocked() (pluginrpc.ActionResult, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	next := !z.LoggingEnabled
	if err := s.client.UpdateZone(z.Id, map[string]any{"LoggingEnabled": next}); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	z.LoggingEnabled = next
	view, _ := s.buildViewLocked(viewNameservers)
	word := "on"
	if !next {
		word = "off"
	}
	return pluginrpc.ActionResult{OK: true, Message: "logging " + word, Next: &view, Reaction: word}, nil
}

func (s *Service) actionToggleCustomNSLocked() (pluginrpc.ActionResult, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	next := !z.CustomNameserversEnabled
	if err := s.client.UpdateZone(z.Id, map[string]any{"CustomNameserversEnabled": next}); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	z.CustomNameserversEnabled = next
	view, _ := s.buildViewLocked(viewNameservers)
	word := "on"
	if !next {
		word = "off"
	}
	return pluginrpc.ActionResult{OK: true, Message: "custom NS " + word, Next: &view, Reaction: word}, nil
}

func (s *Service) actionEnableDNSSECLocked() (pluginrpc.ActionResult, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	info, err := s.client.EnableDNSSEC(z.Id)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	s.lastDNSSEC = info
	z.DnsSecEnabled = true
	view, _ := s.buildViewLocked(viewDNSSEC)
	body := fmt.Sprintf("Enabled: %v\nDS: %s\nDigest: %s (%s)\nKeyTag: %d\nAlgorithm: %d\n",
		info.Enabled, info.DsRecord, info.Digest, info.DigestType, info.KeyTag, info.Algorithm)
	return pluginrpc.ActionResult{
		OK: true, Message: "DNSSEC enabled", Next: &view, Reaction: "sec!",
		ModalTitle: "DNSSEC Enabled", ModalBody: body,
	}, nil
}

func (s *Service) actionDisableDNSSECLocked() (pluginrpc.ActionResult, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	if err := s.client.DisableDNSSEC(z.Id); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	z.DnsSecEnabled = false
	s.lastDNSSEC = nil
	view, _ := s.buildViewLocked(viewDNSSEC)
	return pluginrpc.ActionResult{OK: true, Message: "DNSSEC disabled", Next: &view, Reaction: "off"}, nil
}

func (s *Service) actionDNSSECDetailsLocked() (pluginrpc.ActionResult, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	if s.lastDNSSEC == nil {
		body := fmt.Sprintf("Domain: %s\nDNSSEC Enabled: %v\n\nEnable DNSSEC (E) to retrieve DS record details.", z.Domain, z.DnsSecEnabled)
		return pluginrpc.ActionResult{OK: true, ModalTitle: "DNSSEC", ModalBody: body}, nil
	}
	info := s.lastDNSSEC
	body := fmt.Sprintf("Enabled: %v\nDS: %s\nDigest: %s (%s)\nKeyTag: %d\nAlgorithm: %d\nPublicKey:\n%s\n",
		info.Enabled, info.DsRecord, info.Digest, info.DigestType, info.KeyTag, info.Algorithm, info.PublicKey)
	return pluginrpc.ActionResult{OK: true, ModalTitle: "DNSSEC Details", ModalBody: body}, nil
}

func (s *Service) actionExportZonesLocked() (pluginrpc.ActionResult, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	zones, err := s.client.ListZones("")
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	dir := pluginapi.PluginExportsDir("bunnydns")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: "mkdir " + dir + ": " + err.Error()}, nil
	}
	path := filepath.Join(dir, fmt.Sprintf("zones-%s.csv", time.Now().Format("20060102-150405")))
	f, err := os.Create(path)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: "create " + path + ": " + err.Error()}, nil
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{"ID", "Domain", "Records", "NS OK", "DNSSEC", "NS1", "NS2", "SOA Email", "Logging", "Modified", "Created"})
	for _, z := range zones {
		ns, sec, logging := "no", "off", "off"
		if z.NameserversDetected {
			ns = "yes"
		}
		if z.DnsSecEnabled {
			sec = "on"
		}
		if z.LoggingEnabled {
			logging = "on"
		}
		_ = w.Write([]string{
			strconv.FormatInt(z.Id, 10),
			z.Domain,
			strconv.Itoa(len(z.Records)),
			ns,
			sec,
			z.Nameserver1,
			z.Nameserver2,
			z.SoaEmail,
			logging,
			z.DateModified,
			z.DateCreated,
		})
	}
	w.Flush()
	closeErr := f.Close()
	if err := w.Error(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	if closeErr != nil {
		return pluginrpc.ActionResult{OK: false, Message: closeErr.Error()}, nil
	}
	body := fmt.Sprintf("Wrote %d zones\n\n%s", len(zones), path)
	return pluginrpc.ActionResult{
		OK: true, Reaction: "export", Message: "exported " + path,
		ModalTitle: "Export Zones CSV", ModalBody: body,
	}, nil
}

func (s *Service) actionExportRecordsLocked() (pluginrpc.ActionResult, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	recs, err := s.client.ListRecords(z.Id, "", "")
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	dir := pluginapi.PluginExportsDir("bunnydns")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: "mkdir " + dir + ": " + err.Error()}, nil
	}
	safe := strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ':', ' ', '\t':
			return '_'
		default:
			return r
		}
	}, z.Domain)
	if safe == "" {
		safe = fmt.Sprintf("zone-%d", z.Id)
	}
	path := filepath.Join(dir, fmt.Sprintf("%s-records-%s.csv", safe, time.Now().Format("20060102-150405")))
	f, err := os.Create(path)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: "create " + path + ": " + err.Error()}, nil
	}
	w := csv.NewWriter(f)
	_ = w.Write([]string{
		"Id", "Type", "Name", "Value", "TTL", "Priority", "Weight", "Port",
		"Flags", "Tag", "Disabled", "Comment", "Accelerated",
	})
	for _, r := range recs {
		_ = w.Write([]string{
			strconv.FormatInt(r.Id, 10),
			recordTypeName(r.Type),
			r.Name,
			r.Value,
			strconv.Itoa(r.Ttl),
			strconv.Itoa(r.Priority),
			strconv.Itoa(r.Weight),
			strconv.Itoa(r.Port),
			strconv.Itoa(r.Flags),
			r.Tag,
			strconv.FormatBool(r.Disabled),
			r.Comment,
			strconv.FormatBool(r.Accelerated),
		})
	}
	w.Flush()
	closeErr := f.Close()
	if err := w.Error(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	if closeErr != nil {
		return pluginrpc.ActionResult{OK: false, Message: closeErr.Error()}, nil
	}
	body := fmt.Sprintf("Wrote %d records for %s\n\n%s", len(recs), z.Domain, path)
	return pluginrpc.ActionResult{
		OK: true, Reaction: "export", Message: "exported " + path,
		ModalTitle: "Export Records CSV", ModalBody: body,
	}, nil
}

func (s *Service) actionTriggerScanLocked() (pluginrpc.ActionResult, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	if err := s.client.TriggerScan(z.Id, z.Domain); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	view, _ := s.buildViewLocked(viewScan)
	return pluginrpc.ActionResult{OK: true, Message: "scan triggered", Next: &view, Reaction: "scan"}, nil
}

func (s *Service) actionViewScanLocked() (pluginrpc.ActionResult, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	raw, err := s.client.GetScanResult(z.Id)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	s.lastScanJSON = string(raw)
	pretty := s.lastScanJSON
	var buf any
	if json.Unmarshal(raw, &buf) == nil {
		if b, err := json.MarshalIndent(buf, "", "  "); err == nil {
			pretty = string(b)
		}
	}
	if len(pretty) > 8000 {
		pretty = pretty[:8000] + "\n… truncated …"
	}
	view, _ := s.buildViewLocked(viewScan)
	return pluginrpc.ActionResult{OK: true, Next: &view, Reaction: "ok", ModalTitle: "Scan Result", ModalBody: pretty}, nil
}

func (s *Service) actionCheckAvailabilityLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	if err := s.ensureClientLocked(); err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	domain := strings.TrimSpace(firstNonEmpty(payload["domain"], payload["name"], payload["col0"]))
	if domain == "" {
		return pluginrpc.ActionResult{OK: false, Message: "domain required"}, nil
	}
	res, err := s.client.CheckAvailability(domain)
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Reaction: "nope"}, nil
	}
	avail := "no"
	word := "busy"
	if res.Available {
		avail = "yes"
		word = "free!"
	}
	s.availHistory = append([]availCheck{{
		Domain: domain, Available: avail, Message: res.Message, When: time.Now().Format(time.RFC3339),
	}}, s.availHistory...)
	if len(s.availHistory) > 50 {
		s.availHistory = s.availHistory[:50]
	}
	view, _ := s.buildViewLocked(viewAvailability)
	body := fmt.Sprintf("Domain:    %s\nAvailable: %v\nMessage:   %s\n", domain, res.Available, res.Message)
	return pluginrpc.ActionResult{
		OK: true, Next: &view, Reaction: word,
		ModalTitle: "Availability", ModalBody: body,
	}, nil
}

func (s *Service) actionAvailabilityDetailsLocked(payload map[string]string) (pluginrpc.ActionResult, error) {
	domain := firstNonEmpty(payload["key"], payload["col0"])
	for _, h := range s.availHistory {
		if h.Domain == domain || h.When == domain {
			body := fmt.Sprintf("Domain:    %s\nAvailable: %s\nMessage:   %s\nChecked:   %s\n", h.Domain, h.Available, h.Message, h.When)
			return pluginrpc.ActionResult{OK: true, ModalTitle: "Check " + h.Domain, ModalBody: body}, nil
		}
	}
	return pluginrpc.ActionResult{OK: false, Message: "select a check row"}, nil
}

func (s *Service) actionIssueWildcardLocked() (pluginrpc.ActionResult, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return pluginrpc.ActionResult{OK: false, Message: err.Error()}, nil
	}
	if err := s.client.IssueWildcardCert(z.Id); err != nil {
		s.lastCertMsg = err.Error()
		view, _ := s.buildViewLocked(viewCertificates)
		return pluginrpc.ActionResult{OK: false, Message: err.Error(), Next: &view, Reaction: "nope"}, nil
	}
	s.lastCertMsg = "wildcard certificate issuance requested at " + time.Now().Format(time.RFC3339)
	view, _ := s.buildViewLocked(viewCertificates)
	return pluginrpc.ActionResult{OK: true, Message: s.lastCertMsg, Next: &view, Reaction: "ssl"}, nil
}

func parseID(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty id")
	}
	return strconv.ParseInt(s, 10, 64)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
