package pluginrpc

import (
	"fmt"
	"sort"
)

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

// HelpNav builds the standard "?" help preamble: Views (0-N), optional More Views,
// then action sections, then Global. Pass nil/empty more when all views fit on 0-9.
func HelpNav(views, more []KeyBinding, actions ...HelpSection) []HelpSection {
	last := 0
	if n := len(views); n > 0 {
		last = n - 1
	}
	out := make([]HelpSection, 0, 2+len(actions)+1)
	out = append(out, HelpSection{
		Title:    fmt.Sprintf("Views (0-%d)", last),
		Bindings: views,
	})
	if len(more) > 0 {
		out = append(out, HelpSection{Title: "More Views", Bindings: more})
	}
	out = append(out, actions...)
	return HelpWithGlobal(out...)
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

// TableSpec holds arguments for a decorated table view.
type TableSpec struct {
	View, Title, Info, Status, SelectionKey string
	Headers                                 []string
	Rows                                    [][]string
}

// Table builds a decorated table view with an explicit status.
func (u ViewUI) Table(spec TableSpec, actions ...KeyBinding) ViewData {
	return u.Decorate(Table(spec.View, spec.Title, spec.Info, spec.Status, spec.Headers, spec.Rows, spec.SelectionKey), actions...)
}

// Connected builds a decorated table with status "connected".
func (u ViewUI) Connected(viewID, title, info string, headers []string, rows [][]string, sel string, actions ...KeyBinding) ViewData {
	return u.Table(TableSpec{
		View: viewID, Title: title, Info: info, Status: "connected",
		Headers: headers, Rows: rows, SelectionKey: sel,
	}, actions...)
}

// OK builds a decorated table with status "ok".
func (u ViewUI) OK(viewID, title, info string, headers []string, rows [][]string, sel string, actions ...KeyBinding) ViewData {
	return u.Table(TableSpec{
		View: viewID, Title: title, Info: info, Status: "ok",
		Headers: headers, Rows: rows, SelectionKey: sel,
	}, actions...)
}

// StatusError decorates a Status/Detail error table.
func (u ViewUI) StatusError(viewID, title, info, status, detail string, actions ...KeyBinding) ViewData {
	return u.Decorate(StatusErrorView(viewID, title, info, status, detail), actions...)
}

// NotConnected is the standard yellow disconnect panel.
func (u ViewUI) NotConnected(viewID, brand, detail string, actions ...KeyBinding) ViewData {
	return u.StatusError(viewID, brand, NotConnectedInfo(brand, detail), "not connected", detail, actions...)
}

// NotConnectedErr is NotConnected with err.Error() detail, ready for a GetView return.
func (u ViewUI) NotConnectedErr(viewID, brand string, err error, actions ...KeyBinding) (ViewData, error) {
	detail := ""
	if err != nil {
		detail = err.Error()
	}
	return u.NotConnected(viewID, brand, detail, actions...), nil
}

// Decorate wires Views / overflow KeyBindings / Actions / HelpSections onto a view
// snapshot. moreViews may be nil when all views fit on digits 0-9.
func Decorate(view ViewData, views, moreViews []KeyBinding, help []HelpSection, actions ...KeyBinding) ViewData {
	logsBody := view.LogsBody
	view.ViewBindings = views
	view.KeyBindings = moreViews
	view.Actions = actions
	view.HelpSections = help
	view.LogsBody = logsBody
	return view
}

// Logs builds a decorated logs view (host renders LogsBody in the content area).
func Logs(viewID, title, info, status, body string) ViewData {
	return ViewData{
		View:     viewID,
		Title:    title,
		Info:     info,
		Status:   status,
		LogsBody: body,
	}
}

// Logs builds a decorated connected logs view.
func (u ViewUI) Logs(viewID, title, info, body string, actions ...KeyBinding) ViewData {
	return u.Decorate(Logs(viewID, title, info, "connected", body), actions...)
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
