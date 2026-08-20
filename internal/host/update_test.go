package host

import (
	"strings"
	"testing"
)

func TestUpdateAvailable(t *testing.T) {
	if !updateAvailable("dev", "v1.2.3") {
		t.Fatal("dev should offer a release")
	}
	if !updateAvailable("0.1.0", "v0.2.0") {
		t.Fatal("want update 0.1.0 -> 0.2.0")
	}
	if updateAvailable("v0.2.0", "0.2.0") {
		t.Fatal("same version is not an update")
	}
	if updateAvailable("v1.0.0", "") {
		t.Fatal("empty latest is not an update")
	}
	if updateAvailable("v2.0.0", "v1.9.9") {
		t.Fatal("older latest is not an update")
	}
}

func TestFormatVersionLine(t *testing.T) {
	got := formatVersionLine("0.1.0", "v0.2.0")
	for _, want := range []string{"v0.1.0", "↑", "v0.2.0"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %s", want, got)
		}
	}
	got = formatVersionLine("v0.2.0", "0.2.0")
	if strings.Contains(got, "↑") {
		t.Fatalf("same version should not show update: %s", got)
	}
}
