package dnscheck

import "time"

type resolverPreset struct {
	Label string
	Addr  string
}

type dnsRecord struct {
	Section string
	Type    string
	Name    string
	TTL     uint32
	Data    string
}

type sslCert struct {
	Position   int
	Subject    string
	Issuer     string
	NotBefore  time.Time
	NotAfter   time.Time
	SANs       []string
	Serial     string
	SigAlgo    string
	DNSNames   []string
	IsCA       bool
	SelfSigned bool
}

type sslReport struct {
	OK          bool
	Error       string
	Version     string
	Cipher      string
	ALPN        string
	Verified    bool
	Leaf        *sslCert
	Chain       []sslCert
	QueriedAt   time.Time
	ConnectHost string
}

type httpHop struct {
	URL    string
	Status int
	Via    string
}

type httpReport struct {
	OK          bool
	Error       string
	FinalURL    string
	Status      int
	HSTS        string
	Server      string
	CSP         string
	XFO         string
	XCTO        string
	Referrer    string
	Location    string
	ContentType string
	Hops        []httpHop
	HeaderLines [][2]string
	QueriedAt   time.Time
}

type portResult struct {
	Port    int
	Name    string
	Open    bool
	Latency time.Duration
	Error   string
}

type whoisReport struct {
	OK          bool
	Error       string
	Server      string
	Registrar   string
	Created     string
	Updated     string
	Expires     string
	Status      string
	NameServers []string
	Raw         string
	QueriedAt   time.Time
}

type queryMeta struct {
	Duration time.Duration
	Server   string
	Error    string
}

type domainReport struct {
	Domain    string
	Resolver  string
	QueriedAt time.Time

	Records []dnsRecord
	DNSMeta queryMeta

	SSL   *sslReport
	HTTP  *httpReport
	Whois *whoisReport
	Ports []portResult

	viewFetched map[string]bool
}
