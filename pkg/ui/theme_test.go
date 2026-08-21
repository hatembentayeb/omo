package ui

import (
	"strings"
	"testing"
)

func TestParseOmarchyColors(t *testing.T) {
	src := `
accent = "#89b4fa"
foreground = "#cdd6f4"
background = "#1e1e2e"
selection_foreground = "#1e1e2e"
selection_background = "#f5e0dc"
color1 = "#f38ba8"
color3 = "#f9e2af"
color7 = "#bac2de"
color15 = "#a6adc8"
`
	c := parseOmarchyColors(src)
	if c.Accent != "#89b4fa" || c.Background != "#1e1e2e" {
		t.Fatalf("parse: %+v", c)
	}
	p := PaletteFromOmarchy(c)
	if p.AppBg != "#1e1e2e" {
		t.Fatalf("bg %s", p.AppBg)
	}
	if p.Border != "#89b4fa" {
		t.Fatalf("border %s", p.Border)
	}
	if p.ViewKey != "#f38ba8" {
		t.Fatalf("view %s", p.ViewKey)
	}
}

func TestPrettyThemeName(t *testing.T) {
	if got := prettyThemeName("tokyo-night"); got != "Tokyo Night" {
		t.Fatalf("got %q", got)
	}
	if got := prettyThemeName("catppuccin-latte"); got != "Catppuccin Latte" {
		t.Fatalf("got %q", got)
	}
}

func TestApplyNamedThemeOmo(t *testing.T) {
	ApplyNamedTheme(ThemeOmo)
	if ActiveThemeID() != ThemeOmo {
		t.Fatalf("id %s", ActiveThemeID())
	}
	if HexAppBg != defaultPalette.AppBg {
		t.Fatalf("bg %s", HexAppBg)
	}
	if !strings.HasPrefix(HexHighlight, "#") {
		t.Fatalf("highlight %s", HexHighlight)
	}
}

func TestListThemesIncludesBuiltins(t *testing.T) {
	list := ListThemes()
	var omo, tokyo bool
	for _, th := range list {
		if th.ID == ThemeOmo {
			omo = true
		}
		if th.ID == "tokyo-night" {
			tokyo = true
		}
		if th.ID == "omarchy" {
			t.Fatal("Follow Omarchy must not depend on the desktop")
		}
	}
	if !omo || !tokyo {
		t.Fatalf("want omo + tokyo-night in %#v", ids(list))
	}
	if len(list) < 20 {
		t.Fatalf("want bundled omarchy palettes, got %d: %v", len(list), ids(list))
	}
}

func TestApplyNamedThemeTokyoNight(t *testing.T) {
	ApplyNamedTheme("tokyo-night")
	if ActiveThemeID() != "tokyo-night" {
		t.Fatalf("id %s", ActiveThemeID())
	}
	if HexAppBg == defaultPalette.AppBg {
		t.Fatal("tokyo-night should not use the omo background")
	}
	ApplyNamedTheme(ThemeOmo)
}

func ids(list []Theme) []string {
	out := make([]string, len(list))
	for i, t := range list {
		out[i] = t.ID
	}
	return out
}
