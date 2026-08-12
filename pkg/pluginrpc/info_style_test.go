package pluginrpc

import (
	"strings"
	"testing"
)

func TestColorizeInfoPanel(t *testing.T) {
	in := "[green]Docker Manager[white]\nHost: local\nURL: unix:///var/run/docker.sock\nView: containers\nContainers: 6\nStatus: Connected"
	out := ColorizeInfoPanel(in)
	for _, want := range []string{infoBrand, infoLabel, infoHost, infoView, infoWarn, infoOK, "Host:", "URL:", "View:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	out2 := ColorizeInfoPanel(out)
	if stripColorTags(out2) != stripColorTags(out) {
		t.Fatalf("not idempotent\n%s\nvs\n%s", out, out2)
	}
}

func TestFormatInfoColorsExtra(t *testing.T) {
	out := FormatInfo("Redis Manager\nServer: localhost:6379", "Keys: 12")
	for _, want := range []string{infoBrand, "Keys:", infoWarn, infoHost} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestNotConnectedInfo(t *testing.T) {
	out := NotConnectedInfo("Docker Manager", "configure a host")
	if !strings.Contains(out, infoBrand) || !strings.Contains(out, infoBad) {
		t.Fatalf("unexpected:\n%s", out)
	}
}
