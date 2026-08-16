package ui

import (
	"strings"
	"testing"
)

func TestOsc52Sequence(t *testing.T) {
	got := osc52Sequence("abc")
	if !strings.Contains(got, "]52;c;abc") {
		t.Fatalf("missing OSC52 payload: %q", got)
	}
}

func TestOsc52SequenceTmux(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1,0")
	got := osc52Sequence("abc")
	if !strings.HasPrefix(got, "\033Ptmux;") {
		t.Fatalf("expected tmux DCS wrap: %q", got)
	}
}

func TestCopyToClipboardEmpty(t *testing.T) {
	if err := copyToClipboard(""); err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestCopyOSC52TooLarge(t *testing.T) {
	big := strings.Repeat("x", logsOSC52MaxBytes+1)
	if err := copyOSC52(big); err == nil {
		t.Fatal("expected size error")
	}
}
