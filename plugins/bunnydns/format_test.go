package bunnydns

import "testing"

func TestFormatCount(t *testing.T) {
	if got := formatCount(125430); got != "125,430" {
		t.Fatalf("formatCount(125430)=%q", got)
	}
	if got := formatCount(42); got != "42" {
		t.Fatalf("formatCount(42)=%q", got)
	}
}

func TestRecordTypeLabel(t *testing.T) {
	if got := recordTypeLabel("0"); got != "A — IPv4 addresses" {
		t.Fatalf("numeric 0: %q", got)
	}
	if got := recordTypeLabel("A"); got != "A — IPv4 addresses" {
		t.Fatalf("name A: %q", got)
	}
	if got := recordTypeLabel("Type 1"); got != "AAAA — IPv6 addresses" {
		t.Fatalf("Type 1: %q", got)
	}
}

func TestFormatChartDay(t *testing.T) {
	got := formatChartDay("2026-08-15T00:00:00")
	if got != "Sat 15 Aug" {
		t.Fatalf("iso day: %q", got)
	}
}
