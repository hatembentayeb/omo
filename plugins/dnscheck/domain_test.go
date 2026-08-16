package dnscheck

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeDomain(t *testing.T) {
	cases := map[string]string{
		"Example.COM":                  "example.com",
		"https://www.Example.com/path": "www.example.com",
		"http://example.com.":          "example.com",
		"  EXAMPLE.COM  ":              "example.com",
		"example.com:443":              "example.com",
		"":                             "",
	}
	for in, want := range cases {
		if got := normalizeDomain(in); got != want {
			t.Errorf("normalizeDomain(%q)=%q want %q", in, got, want)
		}
	}
}

func TestResolverAddr(t *testing.T) {
	if got := resolverAddr("8.8.8.8"); got != "8.8.8.8:53" {
		t.Fatalf("google: %q", got)
	}
	if got := resolverAddr("1.1.1.1:53"); got != "1.1.1.1:53" {
		t.Fatalf("cf with port: %q", got)
	}
	if got := resolverAddr("system"); got != "system" {
		t.Fatalf("system: %q", got)
	}
}

func TestCycleResolver(t *testing.T) {
	next := cycleResolver("8.8.8.8")
	if next != "1.1.1.1" {
		t.Fatalf("after google: %q", next)
	}
	next = cycleResolver("system")
	if next != "8.8.8.8" {
		t.Fatalf("after system wraps: %q", next)
	}
}

func TestLooksLikeMailTXT(t *testing.T) {
	if !looksLikeSPF("v=spf1 include:_spf.google.com ~all") {
		t.Fatal("spf")
	}
	if !looksLikeDMARC("v=DMARC1; p=none") {
		t.Fatal("dmarc")
	}
	if !looksLikeDKIM("v=DKIM1; k=rsa; p=abcd") {
		t.Fatal("dkim")
	}
	if looksLikeSPF("hello world") {
		t.Fatal("not spf")
	}
}

func TestSSLExpiryLabel(t *testing.T) {
	future := time.Now().Add(90 * 24 * time.Hour)
	got := sslExpiryLabel(future)
	if !strings.Contains(got, "[green]") {
		t.Fatalf("expected green for 90d: %q", got)
	}
	past := time.Now().Add(-2 * 24 * time.Hour)
	got = sslExpiryLabel(past)
	if !strings.Contains(got, "[red]") {
		t.Fatalf("expected red for expired: %q", got)
	}
}

func TestFormatTTL(t *testing.T) {
	if got := formatTTL(30); got != "30s" {
		t.Fatalf("30s: %q", got)
	}
	if got := formatTTL(90); got != "1m 30s" {
		t.Fatalf("90s: %q", got)
	}
}

func TestPtrName(t *testing.T) {
	if got := ptrName("1.2.3.4"); got != "4.3.2.1.in-addr.arpa" {
		t.Fatalf("v4: %q", got)
	}
}
