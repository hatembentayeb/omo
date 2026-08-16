package dnscheck

import "time"

const (
	viewOverview     = "overview"
	viewRecords      = "records"
	viewNameservers  = "nameservers"
	viewSSL          = "ssl"
	viewMail         = "mail"
	viewHTTP         = "http"
	viewDNSSEC       = "dnssec"
	viewReverse      = "reverse"
	viewReachability = "reachability"
	viewWhois        = "whois"

	brandName = "DNS Check"

	queryTimeout = 5 * time.Second
	dialTimeout  = 6 * time.Second
	httpTimeout  = 10 * time.Second
	whoisTimeout = 8 * time.Second
)

var defaultResolvers = []resolverPreset{
	{Label: "Google", Addr: "8.8.8.8"},
	{Label: "Cloudflare", Addr: "1.1.1.1"},
	{Label: "Quad9", Addr: "9.9.9.9"},
	{Label: "OpenDNS", Addr: "208.67.222.222"},
	{Label: "System", Addr: "system"},
}

var recordTypeOrder = []string{
	"A", "AAAA", "CNAME", "MX", "NS", "SOA", "TXT", "CAA",
	"HTTPS", "SVCB", "SRV", "DS", "DNSKEY", "RRSIG", "NSEC", "NSEC3",
}

var dkimSelectors = []string{
	"google", "default", "selector1", "selector2", "k1", "k2",
	"s1", "s2", "dkim", "mail", "smtp", "cm", "s1024", "s2048",
}

var srvSpecs = []struct {
	Service, Proto string
}{
	{"http", "tcp"},
	{"https", "tcp"},
	{"autodiscover", "tcp"},
	{"caldav", "tcp"},
	{"carddav", "tcp"},
	{"caldavs", "tcp"},
	{"carddavs", "tcp"},
	{"imaps", "tcp"},
	{"submission", "tcp"},
	{"pop3s", "tcp"},
	{"sip", "tcp"},
	{"xmpp-server", "tcp"},
}

var commonPorts = []struct {
	Port int
	Name string
}{
	{80, "HTTP"},
	{443, "HTTPS"},
	{25, "SMTP"},
	{465, "SMTPS"},
	{587, "Submission"},
	{993, "IMAPS"},
	{995, "POP3S"},
	{22, "SSH"},
	{53, "DNS"},
}

var recordTypeMeaning = map[string]string{
	"A":      "IPv4 address",
	"AAAA":   "IPv6 address",
	"CNAME":  "canonical name",
	"MX":     "mail exchanger",
	"TXT":    "text / verification",
	"NS":     "nameserver",
	"SOA":    "start of authority",
	"CAA":    "certificate authority",
	"SRV":    "service locator",
	"HTTPS":  "HTTPS service bind",
	"SVCB":   "service bind",
	"DS":     "delegation signer",
	"DNSKEY": "DNSSEC key",
	"RRSIG":  "DNSSEC signature",
	"NSEC":   "DNSSEC next-secure",
	"PTR":    "pointer / reverse",
}
