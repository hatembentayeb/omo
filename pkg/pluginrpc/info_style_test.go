package pluginrpc

import (
	"strings"
	"testing"
)

func TestColorizeInfoPanel(t *testing.T) {
	in := "[green]Docker Manager[white]\nHost: local\nURL: unix:///var/run/docker.sock\nView: containers\nContainers: 6\nStatus: Connected"
	out := ColorizeInfoPanel(in)
	for _, want := range []string{infoOrange, infoValue, "Plugin", "Host", "URL", "View", " : "} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if !strings.Contains(stripColorTags(out), "Plugin") || !strings.Contains(stripColorTags(out), "Docker Manager") {
		t.Fatalf("want Plugin key, got:\n%s", out)
	}
	if strings.Count(out, infoOrange) < 2 {
		t.Fatalf("want orange keys, got:\n%s", out)
	}
	out2 := ColorizeInfoPanel(out)
	if stripColorTags(out2) != stripColorTags(out) {
		t.Fatalf("not idempotent\n%s\nvs\n%s", out, out2)
	}
}

func TestColorizeInfoPanelAlignsColons(t *testing.T) {
	out := ColorizeInfoPanel("Redis Manager\nHost: local\nStatus: Connected")
	plain := stripColorTags(out)
	lines := strings.Split(plain, "\n")
	if len(lines) < 3 {
		t.Fatalf("want 3 lines, got %q", plain)
	}
	pluginColon := strings.Index(lines[0], " : ")
	hostColon := strings.Index(lines[1], " : ")
	statusColon := strings.Index(lines[2], " : ")
	if pluginColon < 0 || hostColon < 0 || pluginColon != hostColon || hostColon != statusColon {
		t.Fatalf("colons not aligned:\n%s", plain)
	}
	if !strings.HasPrefix(strings.TrimSpace(lines[0]), "Plugin") {
		t.Fatalf("want Plugin key on first line, got %q", lines[0])
	}
}

func TestFormatInfoColorsExtra(t *testing.T) {
	out := FormatInfo("Redis Manager\nServer: localhost:6379", "Keys: 12")
	for _, want := range []string{infoOrange, "Keys", infoValue, " : "} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestNotConnectedInfo(t *testing.T) {
	out := NotConnectedInfo("Docker Manager", "configure a host")
	plain := stripColorTags(out)
	if !strings.Contains(plain, "Plugin") || !strings.Contains(plain, "Docker Manager") {
		t.Fatalf("want Plugin key, got:\n%s", out)
	}
	if !strings.Contains(out, infoOrange) || !strings.Contains(out, "Not Connected") {
		t.Fatalf("unexpected:\n%s", out)
	}
	if !strings.Contains(out, "["+infoValue+"]Not Connected") && !strings.Contains(out, "[white]Not Connected") {
		t.Fatalf("want white value, got:\n%s", out)
	}
}
