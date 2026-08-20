package host

import (
	"fmt"
	"strings"
	"testing"

	"omo/pkg/ui"
)

func TestFormatPaneTabs(t *testing.T) {
	on := formatPaneTabs(true)
	if !strings.Contains(on, "Plugins") || !strings.Contains(on, "View") {
		t.Fatalf("want both tabs, got %s", on)
	}
	if !strings.Contains(on, ":"+ui.HexHighlight+":b] Plugins") {
		t.Fatalf("want Plugins highlighted, got %s", on)
	}
	off := formatPaneTabs(false)
	if !strings.Contains(off, ":"+ui.HexHighlight+":b] View") {
		t.Fatalf("want View highlighted, got %s", off)
	}
}

func TestFormatHostChrome(t *testing.T) {
	on := formatHostChrome(true)
	for _, want := range []string{"Dashboard", "Packages", "Info", "Themes", " D ", " p ", " i ", " t "} {
		if !strings.Contains(on, want) {
			t.Fatalf("missing %q in %s", want, on)
		}
	}
	off := formatHostChrome(false)
	if strings.Contains(off, fmt.Sprintf(":%s:b]", ui.HexHighlight)) {
		t.Fatalf("dim chrome should not fill pills: %s", off)
	}
	if !strings.Contains(off, "<D>") || !strings.Contains(off, "<p>") || !strings.Contains(off, "<i>") || !strings.Contains(off, "<t>") {
		t.Fatalf("want muted keys, got %s", off)
	}
}
