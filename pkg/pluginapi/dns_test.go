package pluginapi

import (
	"runtime"
	"testing"
)

func TestNeedsPublicDNSTermux(t *testing.T) {
	t.Setenv("TERMUX_VERSION", "0.118")
	if !needsPublicDNS() {
		t.Fatal("TERMUX_VERSION should force public DNS")
	}
}

func TestNeedsPublicDNSPrefix(t *testing.T) {
	t.Setenv("TERMUX_VERSION", "")
	t.Setenv("TERMUX_PREFIX", "")
	t.Setenv("PREFIX", "/data/data/com.termux/files/usr")
	if !needsPublicDNS() {
		t.Fatal("Termux PREFIX should force public DNS")
	}
}

func TestNeedsPublicDNSDesktop(t *testing.T) {
	if runtime.GOOS == "android" {
		t.Skip("android always uses public DNS")
	}
	t.Setenv("TERMUX_VERSION", "")
	t.Setenv("TERMUX_PREFIX", "")
	t.Setenv("PREFIX", "/usr")
	if needsPublicDNS() {
		t.Fatal("desktop with resolv.conf should not force public DNS")
	}
}
