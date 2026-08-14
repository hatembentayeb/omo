package bunnydns

import (
	"fmt"
	"sort"
	"strconv"

	"omo/pkg/pluginrpc"
)

func viewNavBindings() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "0", Label: "Zones", Action: "goto_zones"},
		{Key: "1", Label: "Records", Action: "goto_records"},
		{Key: "2", Label: "Nameservers", Action: "goto_nameservers"},
		{Key: "3", Label: "DNSSEC", Action: "goto_dnssec"},
		{Key: "4", Label: "Stats", Action: "goto_stats"},
		{Key: "5", Label: "Export", Action: "goto_export"},
		{Key: "6", Label: "Scan", Action: "goto_scan"},
		{Key: "7", Label: "Availability", Action: "goto_availability"},
		{Key: "8", Label: "Certificates", Action: "goto_certificates"},
	}
}

func zonesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "N", Label: "New Zone", Action: "create_zone"},
		{Key: "D", Label: "Delete", Action: "delete_zone"},
		{Key: "E", Label: "Details", Action: "zone_details"},
		{Key: "O", Label: "Open Records", Action: "goto_records"},
		{Key: "X", Label: "Export CSV", Action: "export_zones"},
	}
}

func recordsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "N", Label: "New Record", Action: "create_record"},
		{Key: "U", Label: "Update", Action: "update_record"},
		{Key: "D", Label: "Delete", Action: "delete_record"},
		{Key: "E", Label: "Details", Action: "record_details"},
		{Key: "T", Label: "Filter Type", Action: "filter_record_type"},
		{Key: "S", Label: "Filter Table", Action: "search_records"},
		{Key: "C", Label: "Clear Filters", Action: "clear_record_filters"},
		{Key: "X", Label: "Export CSV", Action: "export_records"},
	}
}

func nameserversActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Zone Details", Action: "zone_details"},
		{Key: "M", Label: "Set SOA Email", Action: "set_soa_email"},
		{Key: "L", Label: "Toggle Logging", Action: "toggle_logging"},
		{Key: "C", Label: "Toggle Custom NS", Action: "toggle_custom_ns"},
	}
}

func dnssecActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Enable DNSSEC", Action: "enable_dnssec"},
		{Key: "X", Label: "Disable DNSSEC", Action: "disable_dnssec"},
		{Key: "D", Label: "DS Details", Action: "dnssec_details"},
	}
}

func statsActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "E", Label: "Zone Details", Action: "zone_details"},
	}
}

func exportActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "Z", Label: "Export Zones", Action: "export_zones"},
		{Key: "R", Label: "Export Records", Action: "export_records"},
	}
}

func scanActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "S", Label: "Trigger Scan", Action: "trigger_scan"},
		{Key: "V", Label: "View Result", Action: "view_scan"},
	}
}

func availabilityActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "A", Label: "Check Domain", Action: "check_availability"},
		{Key: "E", Label: "Details", Action: "availability_details"},
		{Key: "C", Label: "Clear History", Action: "clear_availability"},
	}
}

func certificatesActions() []pluginrpc.KeyBinding {
	return []pluginrpc.KeyBinding{
		{Key: "I", Label: "Issue Wildcard", Action: "issue_wildcard_cert"},
		{Key: "E", Label: "Zone Details", Action: "zone_details"},
		{Key: "S", Label: "DNSSEC View", Action: "goto_dnssec"},
	}
}

func helpSections() []pluginrpc.HelpSection {
	return pluginrpc.HelpNav(viewNavBindings(), nil,
		pluginrpc.HelpSection{Title: "Zones", Bindings: zonesActions()},
		pluginrpc.HelpSection{Title: "Records", Bindings: recordsActions()},
		pluginrpc.HelpSection{Title: "Nameservers", Bindings: nameserversActions()},
		pluginrpc.HelpSection{Title: "DNSSEC", Bindings: dnssecActions()},
		pluginrpc.HelpSection{Title: "Stats", Bindings: statsActions()},
		pluginrpc.HelpSection{Title: "Export", Bindings: exportActions()},
		pluginrpc.HelpSection{Title: "Scan", Bindings: scanActions()},
		pluginrpc.HelpSection{Title: "Availability", Bindings: availabilityActions()},
		pluginrpc.HelpSection{Title: "Certificates", Bindings: certificatesActions()},
	)
}

var ui = pluginrpc.ViewUI{
	Views: viewNavBindings,
	Help:  helpSections,
}

func (s *Service) baseInfo(extra string) string {
	zone := "(none)"
	dnssec := "-"
	if s.selectedZone != nil {
		zone = s.selectedZone.Domain
		if s.selectedZone.DnsSecEnabled {
			dnssec = "on"
		} else {
			dnssec = "off"
		}
	}
	msg := fmt.Sprintf("Bunny DNS\nAccount: %s\nAPI: %s\nZone: %s\nDNSSEC: %s\nView: %s",
		s.name, s.baseURL, zone, dnssec, s.currentView)
	return pluginrpc.FormatInfo(msg, extra)
}

func (s *Service) buildViewLocked(viewID string) (pluginrpc.ViewData, error) {
	if viewID == "" {
		viewID = viewZones
	}
	s.currentView = viewID

	if err := s.ensureClientLocked(); err != nil {
		return ui.NotConnectedErr(viewID, "Bunny DNS", err)
	}

	switch viewID {
	case viewRecords:
		return s.viewRecordsLocked()
	case viewNameservers:
		return s.viewNameserversLocked()
	case viewDNSSEC:
		return s.viewDNSSECLocked()
	case viewStats:
		return s.viewStatsLocked()
	case viewExport:
		return s.viewExportLocked()
	case viewScan:
		return s.viewScanLocked()
	case viewAvailability:
		return s.viewAvailabilityLocked()
	case viewCertificates:
		return s.viewCertificatesLocked()
	default:
		return s.viewZonesLocked()
	}
}

func (s *Service) viewZonesLocked() (pluginrpc.ViewData, error) {
	zones, err := s.client.ListZones("")
	if err != nil {
		return ui.NotConnectedErr(viewZones, "Bunny DNS", err)
	}
	rows := make([][]string, 0, len(zones))
	for _, z := range zones {
		ns, sec, mark := "no", "off", ""
		if z.NameserversDetected {
			ns = "yes"
		}
		if z.DnsSecEnabled {
			sec = "on"
		}
		if s.selectedZone != nil && s.selectedZone.Id == z.Id {
			mark = " *"
		}
		rows = append(rows, []string{
			strconv.FormatInt(z.Id, 10),
			z.Domain + mark,
			strconv.Itoa(len(z.Records)),
			ns, sec, z.Nameserver1, z.DateModified,
		})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"—", "No zones", "0", "—", "—", "—", "—"})
	return ui.Connected(viewZones, "DNS Zones", s.baseInfo(fmt.Sprintf("Zones: %d", len(zones))),
		[]string{"ID", "Domain", "Records", "NS OK", "DNSSEC", "NS1", "Modified"},
		rows, "ID", zonesActions()...), nil
}

func (s *Service) viewRecordsLocked() (pluginrpc.ViewData, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return ui.Connected(viewRecords, "DNS Records", s.baseInfo(err.Error()),
			[]string{"ID", "Type", "Name", "Value", "TTL", "Priority", "Disabled"},
			[][]string{{"—", "—", "Highlight a zone on view 0", "—", "—", "—", "—"}},
			"ID", recordsActions()...), nil
	}
	recs, err := s.client.ListRecords(z.Id, s.recordTypeFilter, "")
	if err != nil {
		return ui.NotConnectedErr(viewRecords, "Bunny DNS", err)
	}
	rows := make([][]string, 0, len(recs))
	for _, r := range recs {
		dis, val := "no", r.Value
		if r.Disabled {
			dis = "yes"
		}
		if len(val) > 64 {
			val = val[:61] + "…"
		}
		rows = append(rows, []string{
			strconv.FormatInt(r.Id, 10), recordTypeName(r.Type), r.Name, val,
			strconv.Itoa(r.Ttl), strconv.Itoa(r.Priority), dis,
		})
	}
	extra := fmt.Sprintf("Records: %d", len(recs))
	if s.recordTypeFilter != "" {
		extra += " · type=" + s.recordTypeFilter
	}
	rows = pluginrpc.EnsureRows(rows, []string{"—", "—", "No records", "—", "—", "—", "—"})
	return ui.Connected(viewRecords, "DNS Records — "+z.Domain, s.baseInfo(extra),
		[]string{"ID", "Type", "Name", "Value", "TTL", "Priority", "Disabled"},
		rows, "ID", recordsActions()...), nil
}

func (s *Service) viewNameserversLocked() (pluginrpc.ViewData, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return needZoneView(viewNameservers, "Nameservers", err, nameserversActions()...)
	}
	if fresh, err := s.client.GetZone(z.Id); err == nil {
		s.selectedZone = fresh
		z = fresh
	}
	rows := [][]string{
		{"Domain", z.Domain},
		{"Nameserver1", z.Nameserver1},
		{"Nameserver2", z.Nameserver2},
		{"NS Detected", strconv.FormatBool(z.NameserversDetected)},
		{"Custom NS", strconv.FormatBool(z.CustomNameserversEnabled)},
		{"SOA Email", z.SoaEmail},
		{"Logging", strconv.FormatBool(z.LoggingEnabled)},
		{"DNSSEC", strconv.FormatBool(z.DnsSecEnabled)},
		{"Created", z.DateCreated},
		{"Modified", z.DateModified},
	}
	return ui.Connected(viewNameservers, "Nameservers — "+z.Domain, s.baseInfo(""),
		[]string{"Property", "Value"}, rows, "Property", nameserversActions()...), nil
}

func (s *Service) viewDNSSECLocked() (pluginrpc.ViewData, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return needZoneView(viewDNSSEC, "DNSSEC", err, dnssecActions()...)
	}
	rows := [][]string{
		{"Domain", z.Domain},
		{"DNSSEC Enabled", strconv.FormatBool(z.DnsSecEnabled)},
		{"Hint", "E enable (DS modal) · X disable · D details"},
	}
	if s.lastDNSSEC != nil {
		rows = append(rows,
			[]string{"Last DS", truncate(s.lastDNSSEC.DsRecord, 48)},
			[]string{"KeyTag", strconv.Itoa(s.lastDNSSEC.KeyTag)},
		)
	}
	return ui.Connected(viewDNSSEC, "DNSSEC — "+z.Domain, s.baseInfo(""),
		[]string{"Property", "Value"}, rows, "Property", dnssecActions()...), nil
}

func (s *Service) viewStatsLocked() (pluginrpc.ViewData, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return needZoneView(viewStats, "DNS Stats", err, statsActions()...)
	}
	st, err := s.client.GetStatistics(z.Id)
	if err != nil {
		return ui.NotConnectedErr(viewStats, "Bunny DNS", err)
	}
	rows := [][]string{{"TotalQueriesServed", strconv.FormatInt(st.TotalQueriesServed, 10)}}
	type kv struct {
		k string
		v int64
	}
	var types []kv
	for k, v := range st.QueriesByTypeChart {
		types = append(types, kv{k, v})
	}
	sort.Slice(types, func(i, j int) bool { return types[i].v > types[j].v })
	for i, t := range types {
		if i >= 12 {
			break
		}
		rows = append(rows, []string{"Type " + t.k, strconv.FormatInt(t.v, 10)})
	}
	var days []kv
	for k, v := range st.QueriesServedChart {
		days = append(days, kv{k, int64(v)})
	}
	sort.Slice(days, func(i, j int) bool { return days[i].k < days[j].k })
	if len(days) > 10 {
		days = days[len(days)-10:]
	}
	for _, d := range days {
		rows = append(rows, []string{"Day " + d.k, strconv.FormatInt(d.v, 10)})
	}
	return ui.Connected(viewStats, "Stats — "+z.Domain, s.baseInfo(fmt.Sprintf("Queries: %d", st.TotalQueriesServed)),
		[]string{"Metric", "Value"}, rows, "Metric", statsActions()...), nil
}

func (s *Service) viewExportLocked() (pluginrpc.ViewData, error) {
	if err := s.ensureClientLocked(); err != nil {
		return ui.NotConnectedErr(viewExport, "Bunny DNS", err)
	}
	zone := "(highlight a zone on view 0)"
	if s.selectedZone != nil && s.selectedZone.Domain != "" {
		zone = s.selectedZone.Domain
	}
	rows := [][]string{
		{"Export Zones", "Z → ~/.omo/exports/bunnydns/zones-*.csv"},
		{"Export Records", "R → ~/.omo/exports/bunnydns/<zone>-*.csv (current: " + zone + ")"},
		{"Shortcuts", "Zones X · Records X"},
	}
	return ui.Connected(viewExport, "Export", s.baseInfo(""),
		[]string{"Action", "Detail"}, rows, "Action", exportActions()...), nil
}

func (s *Service) viewScanLocked() (pluginrpc.ViewData, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return needZoneView(viewScan, "DNS Scan", err, scanActions()...)
	}
	status := "(none yet — press V)"
	if s.lastScanJSON != "" {
		status = "cached · press V to view"
	}
	rows := [][]string{
		{"Trigger", "Press S to scan existing DNS records"},
		{"Result", status},
		{"Domain", z.Domain},
	}
	return ui.Connected(viewScan, "Scan — "+z.Domain, s.baseInfo(""),
		[]string{"Action", "Detail"}, rows, "Action", scanActions()...), nil
}

func (s *Service) viewAvailabilityLocked() (pluginrpc.ViewData, error) {
	rows := make([][]string, 0, len(s.availHistory))
	for _, h := range s.availHistory {
		rows = append(rows, []string{h.Domain, h.Available, h.Message, h.When})
	}
	rows = pluginrpc.EnsureRows(rows, []string{"—", "—", "Press A to check a domain", "—"})
	return ui.Connected(viewAvailability, "Domain Availability", s.baseInfo(fmt.Sprintf("Checks: %d", len(s.availHistory))),
		[]string{"Domain", "Available", "Message", "Checked"},
		rows, "Domain", availabilityActions()...), nil
}

func (s *Service) viewCertificatesLocked() (pluginrpc.ViewData, error) {
	z, err := s.requireZoneLocked()
	if err != nil {
		return needZoneView(viewCertificates, "Certificates", err, certificatesActions()...)
	}
	last := s.lastCertMsg
	if last == "" {
		last = "(none yet)"
	}
	rows := [][]string{
		{"Domain", z.Domain},
		{"DNSSEC", strconv.FormatBool(z.DnsSecEnabled)},
		{"Last issue", last},
		{"Hint", "I issues wildcard cert · S opens DNSSEC"},
	}
	return ui.Connected(viewCertificates, "Certificates — "+z.Domain, s.baseInfo(""),
		[]string{"Property", "Value"}, rows, "Property", certificatesActions()...), nil
}

func needZoneView(viewID, title string, err error, actions ...pluginrpc.KeyBinding) (pluginrpc.ViewData, error) {
	return ui.Connected(viewID, title, pluginrpc.FormatInfo("Bunny DNS", err.Error()),
		[]string{"Property", "Value"},
		[][]string{{"Status", err.Error()}, {"Hint", "Open view 0 and press Enter on a zone"}},
		"Property", actions...), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
