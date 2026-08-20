package ui

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	ThemeOmo     = "omo"
	ThemeOmarchy = "omarchy"
)

// Palette is the OMO chrome colors derived from an Omarchy (or built-in) scheme.
type Palette struct {
	AppBg         string
	Row           string
	Highlight     string
	HighlightText string
	Border        string
	ViewKey       string
	ActionKey     string
	Label         string
	InfoKey       string
	Value         string
}

// Theme is a named palette the user can apply from the Themes picker.
type Theme struct {
	ID      string
	Name    string
	Source  string
	Palette Palette
}

var activeThemeID = ThemeOmo

var defaultPalette = Palette{
	AppBg:         "#120e0a",
	Row:           "#e6d3b8",
	Highlight:     "#e8b86d",
	HighlightText: "#1a1208",
	Border:        "#c9a35b",
	ViewKey:       "#e07a5f",
	ActionKey:     "#e8b86d",
	Label:         "#c4b8a8",
	InfoKey:       "#e09201",
	Value:         "#f5efe6",
}

// ActiveThemeID is the persisted theme id (omo, omarchy, tokyo-night, …).
func ActiveThemeID() string { return activeThemeID }

func applyTviewStyles() {
	tview.Styles.PrimitiveBackgroundColor = ColorAppBg
	tview.Styles.ContrastBackgroundColor = ColorAppBg
	tview.Styles.BorderColor = ColorBorder
	tview.Styles.GraphicsColor = ColorBorder
	tview.Styles.TitleColor = ColorBorder
}

// ApplyPalette installs p as the live chrome colors.
func ApplyPalette(p Palette) {
	p = p.normalized()
	HexAppBg = p.AppBg
	HexRow = p.Row
	HexHighlight = p.Highlight
	HexHighlightText = p.HighlightText
	HexBorder = p.Border
	HexViewKey = p.ViewKey
	HexActionKey = p.ActionKey
	HexLabel = p.Label
	HexInfoKey = p.InfoKey
	HexValue = p.Value
	ColorAppBg = tcell.GetColor(HexAppBg)
	ColorTableRow = tcell.GetColor(HexRow)
	ColorHighlight = tcell.GetColor(HexHighlight)
	ColorHighlightText = tcell.GetColor(HexHighlightText)
	ColorBorder = tcell.GetColor(HexBorder)
	applyTviewStyles()
}

func (p Palette) normalized() Palette {
	fill := func(v, fallback string) string {
		if strings.TrimSpace(v) == "" {
			return fallback
		}
		return ensureHash(v)
	}
	p.AppBg = fill(p.AppBg, defaultPalette.AppBg)
	p.Row = fill(p.Row, defaultPalette.Row)
	p.Highlight = fill(p.Highlight, defaultPalette.Highlight)
	p.HighlightText = fill(p.HighlightText, defaultPalette.HighlightText)
	p.Border = fill(p.Border, defaultPalette.Border)
	p.ViewKey = fill(p.ViewKey, defaultPalette.ViewKey)
	p.ActionKey = fill(p.ActionKey, defaultPalette.ActionKey)
	p.Label = fill(p.Label, defaultPalette.Label)
	p.InfoKey = fill(p.InfoKey, defaultPalette.InfoKey)
	p.Value = fill(p.Value, defaultPalette.Value)
	return p
}

func ensureHash(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}
	if !strings.HasPrefix(v, "#") {
		return "#" + v
	}
	return v
}

func themeFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".omo", "theme")
}

func loadSavedThemeID() string {
	path := themeFile()
	if path == "" {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func saveThemeID(id string) error {
	path := themeFile()
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.TrimSpace(id)+"\n"), 0o644)
}

// LoadSavedTheme applies ~/.omo/theme, or follows Omarchy when unset.
func LoadSavedTheme() string {
	id := loadSavedThemeID()
	if id == "" {
		if _, ok := omarchyCurrentName(); ok {
			id = ThemeOmarchy
		} else {
			id = ThemeOmo
		}
	}
	ApplyNamedTheme(id)
	return activeThemeID
}

// ApplyNamedTheme looks up id (omo, omarchy, or an Omarchy theme folder) and applies it.
func ApplyNamedTheme(id string) {
	id = strings.TrimSpace(strings.ToLower(id))
	if id == "" {
		id = ThemeOmo
	}
	theme, ok := lookupTheme(id)
	if !ok {
		theme = Theme{ID: ThemeOmo, Name: "Omo", Source: "built-in", Palette: defaultPalette}
	}
	activeThemeID = theme.ID
	ApplyPalette(theme.Palette)
}

// ApplyAndSaveTheme applies id and writes ~/.omo/theme.
func ApplyAndSaveTheme(id string) {
	ApplyNamedTheme(id)
	_ = saveThemeID(activeThemeID)
}

func lookupTheme(id string) (Theme, bool) {
	for _, t := range ListThemes() {
		if t.ID == id {
			return t, true
		}
	}
	return Theme{}, false
}

// ListThemes returns Follow Omarchy, Omo, then every Omarchy colors.toml theme.
func ListThemes() []Theme {
	out := []Theme{
		followOmarchyTheme(),
		{ID: ThemeOmo, Name: "Omo", Source: "built-in", Palette: defaultPalette},
	}
	seen := map[string]bool{ThemeOmo: true, ThemeOmarchy: true}
	for _, t := range discoverOmarchyThemes() {
		if seen[t.ID] {
			continue
		}
		seen[t.ID] = true
		out = append(out, t)
	}
	return out
}

func followOmarchyTheme() Theme {
	name, ok := omarchyCurrentName()
	src := "desktop"
	if ok {
		src = "desktop · " + prettyThemeName(name)
	} else {
		src = "desktop (not found)"
	}
	p := defaultPalette
	if pal, found := omarchyCurrentPalette(); found {
		p = pal
	}
	return Theme{ID: ThemeOmarchy, Name: "Follow Omarchy", Source: src, Palette: p}
}
