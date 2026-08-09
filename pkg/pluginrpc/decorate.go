package pluginrpc

import "sort"

// GlobalHelpBindings are host-handled shortcuts shown in every plugin's "?" help.
func GlobalHelpBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "R", Label: "Refresh"},
		{Key: "?", Label: "Help (this screen)"},
		{Key: "/", Label: "Filter"},
		{Key: "^t", Label: "Switch target"},
		{Key: "ESC", Label: "Back / home"},
	}
}

// GlobalHelpSection is the standard Global group for HelpSections.
func GlobalHelpSection() HelpSection {
	return HelpSection{Title: "Global", Bindings: GlobalHelpBindings()}
}

// HelpWithGlobal appends the shared Global help section.
func HelpWithGlobal(sections ...HelpSection) []HelpSection {
	out := make([]HelpSection, 0, len(sections)+1)
	out = append(out, sections...)
	return append(out, GlobalHelpSection())
}

// ViewUI binds a plugin's Views / overflow / Help helpers for Decorate.
type ViewUI struct {
	Views func() []KeyBinding
	More  func() []KeyBinding // optional; nil when all views fit on 0-9
	Help  func() []HelpSection
}

// Decorate wires Views / Actions / HelpSections onto a view snapshot.
func (u ViewUI) Decorate(view ViewData, actions ...KeyBinding) ViewData {
	var more []KeyBinding
	if u.More != nil {
		more = u.More()
	}
	var help []HelpSection
	if u.Help != nil {
		help = u.Help()
	}
	var views []KeyBinding
	if u.Views != nil {
		views = u.Views()
	}
	return Decorate(view, views, more, help, actions...)
}

// Decorate wires Views / overflow KeyBindings / Actions / HelpSections onto a view
// snapshot. moreViews may be nil when all views fit on digits 0-9.
func Decorate(view ViewData, views, moreViews []KeyBinding, help []HelpSection, actions ...KeyBinding) ViewData {
	view.ViewBindings = views
	view.KeyBindings = moreViews
	view.Actions = actions
	view.HelpSections = help
	view.LogLines = nil
	return view
}

// FormatInfo appends an optional extra line to a base info panel string.
func FormatInfo(base, extra string) string {
	if extra == "" {
		return base
	}
	return base + "\n" + extra
}

// Table builds a standard selectable table ViewData.
func Table(viewID, title, info, status string, headers []string, rows [][]string, selectionKey string) ViewData {
	return ViewData{
		View:         viewID,
		Title:        title,
		Info:         info,
		Status:       status,
		Headers:      headers,
		Rows:         rows,
		SelectionKey: selectionKey,
	}
}

// StatusErrorView is a minimal Status/Detail table for connection or config failures.
func StatusErrorView(viewID, title, info, status, detail string) ViewData {
	return ViewData{
		View:    viewID,
		Title:   title,
		Info:    info,
		Status:  status,
		Headers: []string{"Status", "Detail"},
		Rows:    [][]string{{"error", detail}},
	}
}

// EnsureRows returns rows, or a single placeholder row when empty.
func EnsureRows(rows [][]string, placeholder []string) [][]string {
	if len(rows) == 0 {
		return [][]string{placeholder}
	}
	return rows
}

// SortedKVRows returns alphabetically sorted key/value table rows.
func SortedKVRows(m map[string]string) [][]string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	rows := make([][]string, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, []string{k, m[k]})
	}
	return rows
}

// Truncate shortens s to max runes with an ellipsis when needed.
func Truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
