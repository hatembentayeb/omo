package bunnydns

import "strconv"

const (
	viewZones         = "zones"
	viewRecords       = "records"
	viewNameservers   = "nameservers"
	viewDNSSEC        = "dnssec"
	viewStats         = "stats"
	viewExport        = "export"
	viewScan          = "scan"
	viewAvailability  = "availability"
	viewCertificates  = "certificates"

	apiBaseDefault = "https://api.bunny.net"
)

var dnsRecordTypeNames = map[int]string{
	0: "A", 1: "AAAA", 2: "CNAME", 3: "TXT", 4: "MX",
	5: "Redirect", 6: "Flatten", 7: "PullZone", 8: "SRV", 9: "CAA",
	10: "PTR", 11: "Script", 12: "NS", 13: "SVCB", 14: "HTTPS", 15: "TLSA",
}

var dnsRecordTypeCodes = map[string]int{
	"A": 0, "AAAA": 1, "CNAME": 2, "TXT": 3, "MX": 4,
	"REDIRECT": 5, "FLATTEN": 6, "PULLZONE": 7, "SRV": 8, "CAA": 9,
	"PTR": 10, "SCRIPT": 11, "NS": 12, "SVCB": 13, "HTTPS": 14, "TLSA": 15,
}

func recordTypeName(t int) string {
	if n, ok := dnsRecordTypeNames[t]; ok {
		return n
	}
	return strconv.Itoa(t)
}
