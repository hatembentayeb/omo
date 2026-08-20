package host

import (
	"strings"
	"testing"
)

func TestRenderWave(t *testing.T) {
	got := renderWave(0, 12)
	if !strings.Contains(got, "▁") || !strings.Contains(got, "█") {
		t.Fatalf("wave missing glyphs: %s", got)
	}
	plain := stripTviewTags(got)
	if len([]rune(plain)) != 12 {
		t.Fatalf("wave width %d, want 12 (%q)", len([]rune(plain)), plain)
	}
	shifted := stripTviewTags(renderWave(1, 12))
	if shifted == plain {
		t.Fatal("wave did not shift")
	}
}

func stripTviewTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '[':
			in = true
		case r == ']':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}
