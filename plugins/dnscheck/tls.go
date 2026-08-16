package dnscheck

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

func checkTLS(domain string, ips []string) *sslReport {
	rep := &sslReport{QueriedAt: time.Now(), ConnectHost: domain + ":443"}
	addr := domain + ":443"
	if len(ips) > 0 {
		ip := ips[0]
		if strings.Contains(ip, ":") {
			addr = net.JoinHostPort(ip, "443")
		} else {
			addr = net.JoinHostPort(ip, "443")
		}
	}

	dialer := &net.Dialer{Timeout: dialTimeout}
	conn, err := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
		ServerName:         domain,
		InsecureSkipVerify: true, // inspect even when untrusted; we report Verify result
		MinVersion:         tls.VersionTLS10,
		NextProtos:         []string{"h2", "http/1.1"},
	})
	if err != nil {
		rep.Error = err.Error()
		return rep
	}
	defer conn.Close()

	state := conn.ConnectionState()
	rep.OK = true
	rep.Version = tlsVersionName(state.Version)
	rep.Cipher = tls.CipherSuiteName(state.CipherSuite)
	rep.ALPN = state.NegotiatedProtocol

	if err := verifyChain(domain, state.PeerCertificates); err != nil {
		rep.Verified = false
		if rep.Error == "" {
			rep.Error = "verify: " + err.Error()
		}
	} else {
		rep.Verified = true
	}

	for i, c := range state.PeerCertificates {
		sc := sslCert{
			Position:   i,
			Subject:    c.Subject.String(),
			Issuer:     c.Issuer.String(),
			NotBefore:  c.NotBefore,
			NotAfter:   c.NotAfter,
			SANs:       append([]string{}, c.DNSNames...),
			DNSNames:   append([]string{}, c.DNSNames...),
			Serial:     strings.ToUpper(hex.EncodeToString(c.SerialNumber.Bytes())),
			SigAlgo:    c.SignatureAlgorithm.String(),
			IsCA:       c.IsCA,
			SelfSigned: c.Issuer.String() == c.Subject.String(),
		}
		if i == 0 {
			leaf := sc
			rep.Leaf = &leaf
		}
		rep.Chain = append(rep.Chain, sc)
	}
	return rep
}

func verifyChain(serverName string, certs []*x509.Certificate) error {
	if len(certs) == 0 {
		return fmt.Errorf("no certificates")
	}
	opts := x509.VerifyOptions{
		DNSName:       serverName,
		Intermediates: x509.NewCertPool(),
		CurrentTime:   time.Now(),
	}
	for _, c := range certs[1:] {
		opts.Intermediates.AddCert(c)
	}
	_, err := certs[0].Verify(opts)
	return err
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS13:
		return "TLS 1.3"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS10:
		return "TLS 1.0"
	default:
		return fmt.Sprintf("0x%04x", v)
	}
}

func ipv4FromRecords(recs []dnsRecord) []string {
	var ips []string
	for _, r := range recordsOf(recs, "A") {
		if ip := net.ParseIP(strings.TrimSpace(r.Data)); ip != nil {
			ips = append(ips, ip.String())
		}
	}
	return ips
}

func allIPsFromRecords(recs []dnsRecord) []string {
	var ips []string
	for _, t := range []string{"A", "AAAA"} {
		for _, r := range recordsOf(recs, t) {
			if ip := net.ParseIP(strings.TrimSpace(r.Data)); ip != nil {
				ips = append(ips, ip.String())
			}
		}
	}
	return ips
}
