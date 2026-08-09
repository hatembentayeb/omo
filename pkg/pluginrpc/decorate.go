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

// Table builds a decorated table view with an explicit status.
func (u ViewUI) Table(viewID, title, info, status string, headers []string, rows [][]string, sel string, actions ...KeyBinding) ViewData {
	return u.Decorate(Table(viewID, title, info, status, headers, rows, sel), actions...)
}

// Connected builds a decorated table with status "connected".
func (u ViewUI) Connected(viewID, title, info string, headers []string, rows [][]string, sel string, actions ...KeyBinding) ViewData {
	return u.Table(viewID, title, info, "connected", headers, rows, sel, actions...)
}

// OK builds a decorated table with status "ok".
func (u ViewUI) OK(viewID, title, info string, headers []string, rows [][]string, sel string, actions ...KeyBinding) ViewData {
	return u.Table(viewID, title, info, "ok", headers, rows, sel, actions...)
}

// StatusError decorates a Status/Detail error table.
func (u ViewUI) StatusError(viewID, title, info, status, detail string, actions ...KeyBinding) ViewData {
	return u.Decorate(StatusErrorView(viewID, title, info, status, detail), actions...)
}

// NotConnected is the standard yellow disconnect panel.
func (u ViewUI) NotConnected(viewID, brand, detail string, actions ...KeyBinding) ViewData {
	return u.StatusError(viewID, brand, NotConnectedInfo(brand, detail), "not connected", detail, actions...)
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

// NotConnectedInfo builds the standard yellow "Not Connected" info panel.
func NotConnectedInfo(brand, detail string) string {
	msg := "[yellow]" + brand + "[white]\nStatus: Not Connected"
	if detail == "" {
		return msg
	}
	return msg + "\n" + detail
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

// DashRow builds a placeholder row of cols cells with message in the last cell
// (or msgCol when msgCol >= 0).
func DashRow(cols int, message string) []string {
	row := make([]string, cols)
	for i := range row {
		row[i] = "-"
	}
	if cols == 0 {
		return row
	}
	row[cols-1] = message
	return row
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

// MapRows maps items to table rows.
func MapRows[T any](items []T, fn func(T) []string) [][]string {
	rows := make([][]string, 0, len(items))
	for _, item := range items {
		rows = append(rows, fn(item))
	}
	return rows
}

// Truncate shortens s to max bytes with an ellipsis when needed.
func Truncate(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}
