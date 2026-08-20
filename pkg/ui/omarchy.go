package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// OmarchyColors is the colors.toml schema used by Omarchy themes.
type OmarchyColors struct {
	Accent              string
	Cursor              string
	Foreground          string
	Background          string
	SelectionForeground string
	SelectionBackground string
	Color0              string
	Color1              string
	Color2              string
	Color3              string
	Color4              string
	Color5              string
	Color6              string
	Color7              string
	Color8              string
	Color15             string
}

func omarchyThemeRoots() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	return []string{
		filepath.Join(home, ".config", "omarchy", "themes"),
		filepath.Join(home, ".local", "share", "omarchy", "themes"),
	}
}

func omarchyCurrentName() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}
	b, err := os.ReadFile(filepath.Join(home, ".config", "omarchy", "current", "theme.name"))
	if err != nil {
		return "", false
	}
	name := strings.TrimSpace(string(b))
	if name == "" {
		return "", false
	}
	return name, true
}

func omarchyCurrentPalette() (Palette, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Palette{}, false
	}
	path := filepath.Join(home, ".config", "omarchy", "current", "theme", "colors.toml")
	p, err := paletteFromColorsFile(path)
	if err != nil {
		return Palette{}, false
	}
	return p, true
}

func discoverOmarchyThemes() []Theme {
	var out []Theme
	seen := map[string]bool{}
	for _, root := range omarchyThemeRoots() {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			id := entry.Name()
			if seen[id] {
				continue
			}
			path := filepath.Join(root, id, "colors.toml")
			p, err := paletteFromColorsFile(path)
			if err != nil {
				continue
			}
			seen[id] = true
			src := "omarchy"
			if strings.Contains(root, filepath.Join(".config", "omarchy")) {
				src = "omarchy custom"
			}
			out = append(out, Theme{
				ID:      id,
				Name:    prettyThemeName(id),
				Source:  src,
				Palette: p,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func paletteFromColorsFile(path string) (Palette, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Palette{}, err
	}
	return PaletteFromOmarchy(parseOmarchyColors(string(b))), nil
}

func parseOmarchyColors(src string) OmarchyColors {
	var c OmarchyColors
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		switch key {
		case "accent":
			c.Accent = val
		case "cursor":
			c.Cursor = val
		case "foreground":
			c.Foreground = val
		case "background":
			c.Background = val
		case "selection_foreground":
			c.SelectionForeground = val
		case "selection_background":
			c.SelectionBackground = val
		case "color0":
			c.Color0 = val
		case "color1":
			c.Color1 = val
		case "color2":
			c.Color2 = val
		case "color3":
			c.Color3 = val
		case "color4":
			c.Color4 = val
		case "color5":
			c.Color5 = val
		case "color6":
			c.Color6 = val
		case "color7":
			c.Color7 = val
		case "color8":
			c.Color8 = val
		case "color15":
			c.Color15 = val
		}
	}
	return c
}

// PaletteFromOmarchy maps an Omarchy terminal scheme onto OMO chrome roles.
func PaletteFromOmarchy(c OmarchyColors) Palette {
	bg := firstHex(c.Background, defaultPalette.AppBg)
	fg := firstHex(c.Foreground, c.Color15, defaultPalette.Row)
	accent := firstHex(c.Accent, c.Color4, defaultPalette.Border)
	selBg := firstHex(c.SelectionBackground, accent)
	selFg := firstHex(c.SelectionForeground, bg)
	label := firstHex(c.Color7, c.Color8, fg)
	if strings.EqualFold(label, bg) {
		label = fg
	}
	return Palette{
		AppBg:         bg,
		Row:           fg,
		Highlight:     selBg,
		HighlightText: selFg,
		Border:        accent,
		ViewKey:       firstHex(c.Color1, c.Color5, accent),
		ActionKey:     firstHex(accent, c.Color3),
		Label:         label,
		InfoKey:       firstHex(c.Color3, accent),
		Value:         firstHex(c.Color15, fg),
	}
}

func firstHex(vals ...string) string {
	for _, v := range vals {
		v = strings.TrimSpace(v)
		if v != "" {
			return ensureHash(v)
		}
	}
	return ""
}

func prettyThemeName(id string) string {
	parts := strings.Split(id, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
