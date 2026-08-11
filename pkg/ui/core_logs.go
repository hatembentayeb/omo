package ui

import (
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

const (
	logsContentPage   = "logs"
	tableContentPage  = "table"
	logsMaxLines      = 50000
	logsOSC52MaxBytes = 200_000
)

// ShowLogs replaces the table content area with the log viewer.
// Header (Info | Views | Actions) stays visible; Actions list log shortcuts.
// ESC restores the table. onRefresh, if non-nil, is invoked on R.
func (c *CoreView) ShowLogs(title, body string, onRefresh func() (string, error)) {
	if c.contentPages == nil || c.app == nil {
		return
	}
	// Replace any existing logs pane.
	if c.logs != nil {
		c.contentPages.RemovePage(logsContentPage)
		c.logs = nil
	}

	v := &logsView{
		core:       c,
		app:        c.app,
		title:      title,
		onRefresh:  onRefresh,
		wrap:       false,
		autoscroll: true,
		lineNums:   true,
		marks:      map[int]struct{}{},
	}
	v.setBody(body)

	v.titleBar = tview.NewTextView()
	v.titleBar.SetDynamicColors(true)
	v.titleBar.SetTextAlign(tview.AlignLeft)
	v.titleBar.SetBackgroundColor(tcell.ColorDefault)
	v.titleBar.SetBorder(false)

	v.controlsBar = tview.NewTextView()
	v.controlsBar.SetDynamicColors(true)
	v.controlsBar.SetTextAlign(tview.AlignCenter)
	v.controlsBar.SetBackgroundColor(tcell.ColorDefault)
	v.controlsBar.SetBorder(false)

	v.metaBar = tview.NewTextView()
	v.metaBar.SetDynamicColors(true)
	v.metaBar.SetTextAlign(tview.AlignRight)
	v.metaBar.SetBackgroundColor(tcell.ColorDefault)
	v.metaBar.SetBorder(false)

	v.text = tview.NewTextView()
	v.text.SetDynamicColors(true)
	v.text.SetRegions(true)
	v.text.SetScrollable(true)
	v.text.SetWrap(v.wrap)
	v.text.SetWordWrap(v.wrap)
	v.text.SetBackgroundColor(tcell.ColorDefault)
	v.text.SetBorder(false)

	v.footer = tview.NewTextView()
	v.footer.SetDynamicColors(true)
	v.footer.SetTextAlign(tview.AlignLeft)
	v.footer.SetBackgroundColor(tcell.ColorDefault)
	v.footer.SetBorder(false)

	v.searchInput = tview.NewInputField()
	v.searchInput.SetLabel(" / ")
	v.searchInput.SetLabelColor(tcell.ColorYellow)
	v.searchInput.SetFieldBackgroundColor(tcell.ColorDefault)
	v.searchInput.SetFieldTextColor(tcell.ColorWhite)
	v.searchInput.SetPlaceholder("find…")
	v.searchInput.SetPlaceholderTextColor(tcell.ColorGray)

	v.footerRow = tview.NewFlex().SetDirection(tview.FlexColumn)
	v.footerRow.SetBackgroundColor(tcell.ColorDefault)
	v.footerRow.AddItem(v.footer, 0, 1, false)

	topRow := tview.NewFlex().SetDirection(tview.FlexColumn)
	topRow.SetBackgroundColor(tcell.ColorDefault)
	topRow.AddItem(v.titleBar, 0, 1, false)
	topRow.AddItem(v.controlsBar, 0, 2, false)
	topRow.AddItem(v.metaBar, 0, 1, false)

	layout := tview.NewFlex().SetDirection(tview.FlexRow)
	layout.SetBackgroundColor(tcell.ColorDefault)
	layout.AddItem(topRow, 1, 0, false)
	layout.AddItem(v.text, 0, 1, true)
	layout.AddItem(v.footerRow, 1, 0, false)

	v.searchInput.SetDoneFunc(func(key tcell.Key) {
		switch key {
		case tcell.KeyEscape:
			v.exitSearch(false)
		case tcell.KeyEnter:
			v.exitSearch(true)
		}
	})
	v.searchInput.SetChangedFunc(func(text string) {
		v.query = text
		v.rebuildMatches()
		v.render()
	})
	v.text.SetInputCapture(v.handleTextKeys)

	c.logs = v
	c.contentPages.AddAndSwitchToPage(logsContentPage, layout, true)

	c.installLogsKeyBindings()
	v.render()
	if v.autoscroll {
		v.cursor = len(v.lines) - 1
		if v.cursor < 0 {
			v.cursor = 0
		}
		v.render()
		v.text.ScrollToEnd()
	}
	c.app.SetFocus(v.text)
}

// CloseLogs restores the table content area and previous Actions bindings.
func (c *CoreView) CloseLogs() {
	if c.logs == nil {
		return
	}
	if c.contentPages != nil {
		c.contentPages.RemovePage(logsContentPage)
		c.contentPages.SwitchToPage(tableContentPage)
	}
	c.logs = nil
	c.restoreKeyBindingsAfterLogs()
	if c.app != nil && c.table != nil {
		c.app.SetFocus(c.table)
	}
}

// IsLogsOpen reports whether the log viewer is replacing the table.
func (c *CoreView) IsLogsOpen() bool {
	return c.logs != nil
}

// FocusContent focuses the logs pane if open, otherwise the table.
func (c *CoreView) FocusContent() {
	if c.app == nil {
		return
	}
	if c.logs != nil && c.logs.text != nil {
		c.app.SetFocus(c.logs.text)
		return
	}
	if c.table != nil {
		c.app.SetFocus(c.table)
	}
}

// ReassertLogsActions refreshes the Actions column for an open logs view
// (e.g. after Apply rebinds plugin keys).
func (c *CoreView) ReassertLogsActions() {
	if c.logs == nil {
		return
	}
	c.installLogsKeyBindings()
}

func (c *CoreView) installLogsKeyBindings() {
	if c.logs == nil {
		return
	}
	// Snapshot current plugin bindings so CloseLogs / later Apply can restore them.
	c.savedKeyBindings = copyStringMap(c.keyBindings)
	c.savedKeyHandlers = copyFuncMap(c.keyHandlers)
	c.logsKeysSaved = true

	viewHandlers := map[string]func(){}
	for k := range c.viewBindings {
		if h, ok := c.keyHandlers[k]; ok && h != nil {
			viewHandlers[k] = h
		}
	}

	v := c.logs
	c.keyBindings = map[string]string{
		"a":   "Autoscroll",
		"w":   "Wrap",
		"#":   "Line nums",
		"m":   "Mark",
		"[":   "Prev mark",
		"]":   "Next mark",
		"/":   "Find",
		"n":   "Next hit",
		"N":   "Prev hit",
		"y":   "Copy all",
		"Y":   "Copy marks",
		"g":   "Top",
		"G":   "Bottom",
		"R":   "Refresh",
		"ESC": "Back",
	}
	c.keyHandlers = map[string]func(){
		"a": func() { v.toggleAutoscroll() },
		"w": func() { v.toggleWrap() },
		"#": func() { v.toggleLineNums() },
		"m": func() {
			v.toggleMark(v.cursor)
			v.render()
		},
		"[": func() { v.jumpMark(false) },
		"]": func() { v.jumpMark(true) },
		"/": func() { v.enterSearch() },
		"n": func() { v.jumpMatch(true) },
		"N": func() { v.jumpMatch(false) },
		"y": func() { v.copyAll() },
		"Y": func() { v.copyMarked() },
		"g": func() { v.jumpTop() },
		"G": func() { v.jumpBottom() },
		"R": func() { v.refresh() },
	}
	for k, h := range viewHandlers {
		c.keyHandlers[k] = h
	}
	c.refreshHeaderPanels()
}

func (c *CoreView) restoreKeyBindingsAfterLogs() {
	if !c.logsKeysSaved {
		return
	}
	c.keyBindings = c.savedKeyBindings
	c.keyHandlers = c.savedKeyHandlers
	c.savedKeyBindings = nil
	c.savedKeyHandlers = nil
	c.logsKeysSaved = false
	c.refreshHeaderPanels()
}

func copyStringMap(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func copyFuncMap(in map[string]func()) map[string]func() {
	out := make(map[string]func(), len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

type logsView struct {
	core      *CoreView
	app       *tview.Application
	title     string
	onRefresh func() (string, error)

	lines     []string
	truncated bool
	totalRaw  int

	wrap       bool
	autoscroll bool
	lineNums   bool
	marks      map[int]struct{}
	cursor     int

	query     string
	matches   []int
	matchIdx  int
	searching bool

	statusMsg string

	titleBar    *tview.TextView
	controlsBar *tview.TextView
	metaBar     *tview.TextView
	text        *tview.TextView
	footer      *tview.TextView
	footerRow   *tview.Flex
	searchInput *tview.InputField
}

func (v *logsView) setBody(body string) {
	raw := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	if len(raw) > 0 && raw[len(raw)-1] == "" {
		raw = raw[:len(raw)-1]
	}
	v.totalRaw = len(raw)
	v.truncated = false
	if len(raw) > logsMaxLines {
		raw = raw[len(raw)-logsMaxLines:]
		v.truncated = true
	}
	v.lines = raw
	if v.cursor >= len(v.lines) {
		v.cursor = len(v.lines) - 1
	}
	if v.cursor < 0 {
		v.cursor = 0
	}
	for idx := range v.marks {
		if idx < 0 || idx >= len(v.lines) {
			delete(v.marks, idx)
		}
	}
	v.rebuildMatches()
}

func (v *logsView) close() {
	c := v.core
	if c == nil {
		return
	}
	// Leave like any other view: navigate_back → plugin goto home → Apply closes logs.
	if len(c.navStack) > 1 {
		c.PopView()
	}
	if c.onAction != nil {
		_ = c.onAction("navigate_back", map[string]interface{}{
			"current_view": c.GetCurrentView(),
		})
		return
	}
	c.CloseLogs()
}

func (v *logsView) toggleWrap() {
	v.wrap = !v.wrap
	v.text.SetWrap(v.wrap)
	v.text.SetWordWrap(v.wrap)
	v.render()
}

func (v *logsView) toggleAutoscroll() {
	v.autoscroll = !v.autoscroll
	v.updateChrome()
	v.core.refreshHeaderPanels()
}

func (v *logsView) toggleLineNums() {
	v.lineNums = !v.lineNums
	v.render()
}

func (v *logsView) handleTextKeys(event *tcell.EventKey) *tcell.EventKey {
	if v.searching {
		return event
	}

	if event.Key() == tcell.KeyEscape {
		v.close()
		return nil
	}

	switch event.Key() {
	case tcell.KeyUp, tcell.KeyCtrlP:
		v.moveCursor(-1)
		return nil
	case tcell.KeyDown, tcell.KeyCtrlN:
		v.moveCursor(1)
		return nil
	case tcell.KeyPgUp:
		v.moveCursor(-20)
		return nil
	case tcell.KeyPgDn:
		v.moveCursor(20)
		return nil
	case tcell.KeyHome:
		v.jumpTop()
		return nil
	case tcell.KeyEnd:
		v.jumpBottom()
		return nil
	case tcell.KeyRune:
		key := string(event.Rune())
		// Digit view switches: leave via goto (Apply closes logs).
		if len(key) == 1 && key[0] >= '0' && key[0] <= '9' && v.core != nil {
			if handler, ok := v.core.keyHandlers[key]; ok && handler != nil {
				handler()
				return nil
			}
		}
		// Prefer Actions-column handlers so header stays in sync.
		if v.core != nil {
			if handler, ok := v.core.keyHandlers[key]; ok && handler != nil {
				handler()
				return nil
			}
		}
		switch event.Rune() {
		case 'n':
			v.jumpMatch(true)
			return nil
		case 'N':
			v.jumpMatch(false)
			return nil
		case 'j':
			v.moveCursor(1)
			return nil
		case 'k':
			v.moveCursor(-1)
			return nil
		}
	}
	return event
}

func (v *logsView) moveCursor(delta int) {
	if len(v.lines) == 0 {
		return
	}
	v.cursor += delta
	if v.cursor < 0 {
		v.cursor = 0
	}
	if v.cursor >= len(v.lines) {
		v.cursor = len(v.lines) - 1
	}
	if delta != 0 {
		v.autoscroll = false
	}
	v.render()
	v.text.Highlight(strconv.Itoa(v.cursor)).ScrollToHighlight()
}

func (v *logsView) jumpTop() {
	v.autoscroll = false
	v.cursor = 0
	v.render()
	v.text.ScrollToBeginning()
	v.text.Highlight(strconv.Itoa(v.cursor))
}

func (v *logsView) jumpBottom() {
	if len(v.lines) == 0 {
		return
	}
	v.cursor = len(v.lines) - 1
	v.render()
	v.text.ScrollToEnd()
	v.text.Highlight(strconv.Itoa(v.cursor))
}

func (v *logsView) toggleMark(idx int) {
	if idx < 0 || idx >= len(v.lines) {
		return
	}
	if _, ok := v.marks[idx]; ok {
		delete(v.marks, idx)
	} else {
		v.marks[idx] = struct{}{}
	}
}

func (v *logsView) jumpMark(forward bool) {
	if len(v.marks) == 0 {
		v.statusMsg = "no marks"
		v.updateChrome()
		return
	}
	idxs := make([]int, 0, len(v.marks))
	for i := range v.marks {
		idxs = append(idxs, i)
	}
	sort.Ints(idxs)
	if forward {
		for _, i := range idxs {
			if i > v.cursor {
				v.autoscroll = false
				v.cursor = i
				v.render()
				v.text.Highlight(strconv.Itoa(v.cursor)).ScrollToHighlight()
				return
			}
		}
		v.autoscroll = false
		v.cursor = idxs[0]
	} else {
		for i := len(idxs) - 1; i >= 0; i-- {
			if idxs[i] < v.cursor {
				v.autoscroll = false
				v.cursor = idxs[i]
				v.render()
				v.text.Highlight(strconv.Itoa(v.cursor)).ScrollToHighlight()
				return
			}
		}
		v.autoscroll = false
		v.cursor = idxs[len(idxs)-1]
	}
	v.render()
	v.text.Highlight(strconv.Itoa(v.cursor)).ScrollToHighlight()
}

func (v *logsView) enterSearch() {
	v.searching = true
	v.footerRow.Clear()
	v.footerRow.AddItem(v.searchInput, 0, 1, true)
	v.searchInput.SetText(v.query)
	v.app.SetFocus(v.searchInput)
}

func (v *logsView) exitSearch(apply bool) {
	v.searching = false
	if !apply {
		v.query = ""
		v.matches = nil
		v.matchIdx = 0
	} else {
		v.query = v.searchInput.GetText()
		v.rebuildMatches()
		if len(v.matches) > 0 {
			v.matchIdx = 0
			v.cursor = v.matches[0]
			v.autoscroll = false
		}
	}
	v.footerRow.Clear()
	v.footerRow.AddItem(v.footer, 0, 1, false)
	v.render()
	v.app.SetFocus(v.text)
	if apply && len(v.matches) > 0 {
		v.text.Highlight(strconv.Itoa(v.cursor)).ScrollToHighlight()
	}
}

func (v *logsView) rebuildMatches() {
	v.matches = nil
	v.matchIdx = 0
	q := strings.TrimSpace(v.query)
	if q == "" {
		return
	}
	lower := strings.ToLower(q)
	for i, line := range v.lines {
		if strings.Contains(strings.ToLower(line), lower) {
			v.matches = append(v.matches, i)
		}
	}
}

func (v *logsView) jumpMatch(forward bool) {
	if len(v.matches) == 0 {
		v.statusMsg = "no matches"
		v.updateChrome()
		return
	}
	if forward {
		v.matchIdx = (v.matchIdx + 1) % len(v.matches)
	} else {
		v.matchIdx--
		if v.matchIdx < 0 {
			v.matchIdx = len(v.matches) - 1
		}
	}
	v.autoscroll = false
	v.cursor = v.matches[v.matchIdx]
	v.render()
	v.text.Highlight(strconv.Itoa(v.cursor)).ScrollToHighlight()
}

func (v *logsView) refresh() {
	if v.onRefresh == nil {
		v.statusMsg = "refresh unavailable"
		v.updateChrome()
		return
	}
	v.statusMsg = "refreshing…"
	v.updateChrome()
	go func() {
		body, err := v.onRefresh()
		v.app.QueueUpdateDraw(func() {
			if err != nil {
				v.statusMsg = "refresh failed: " + err.Error()
				v.updateChrome()
				return
			}
			v.setBody(body)
			v.statusMsg = "refreshed"
			if v.autoscroll && len(v.lines) > 0 {
				v.cursor = len(v.lines) - 1
			}
			v.render()
			if v.autoscroll {
				v.text.ScrollToEnd()
			} else {
				v.text.Highlight(strconv.Itoa(v.cursor)).ScrollToHighlight()
			}
		})
	}()
}

func (v *logsView) copyAll() {
	text := strings.Join(v.lines, "\n")
	if err := copyOSC52(text); err != nil {
		v.statusMsg = err.Error()
	} else {
		v.statusMsg = fmt.Sprintf("copied %d lines", len(v.lines))
	}
	v.updateChrome()
}

func (v *logsView) copyMarked() {
	if len(v.marks) == 0 {
		v.statusMsg = "no marks to copy"
		v.updateChrome()
		return
	}
	idxs := make([]int, 0, len(v.marks))
	for i := range v.marks {
		idxs = append(idxs, i)
	}
	sort.Ints(idxs)
	parts := make([]string, 0, len(idxs))
	for _, i := range idxs {
		parts = append(parts, v.lines[i])
	}
	if err := copyOSC52(strings.Join(parts, "\n")); err != nil {
		v.statusMsg = err.Error()
	} else {
		v.statusMsg = fmt.Sprintf("copied %d marked lines", len(parts))
	}
	v.updateChrome()
}

func copyOSC52(text string) error {
	if len(text) > logsOSC52MaxBytes {
		return fmt.Errorf("too large to copy (%d bytes)", len(text))
	}
	payload := base64.StdEncoding.EncodeToString([]byte(text))
	_, err := os.Stdout.WriteString("\033]52;c;" + payload + "\a")
	return err
}

func (v *logsView) render() {
	var b strings.Builder
	width := len(strconv.Itoa(len(v.lines)))
	if width < 1 {
		width = 1
	}
	q := strings.TrimSpace(v.query)
	for i, line := range v.lines {
		id := strconv.Itoa(i)
		b.WriteString(`["`)
		b.WriteString(id)
		b.WriteString(`"]`)

		_, marked := v.marks[i]
		if marked {
			b.WriteString("[yellow]")
		}

		if v.lineNums {
			b.WriteString("[gray]")
			b.WriteString(fmt.Sprintf("%*d", width, i+1))
			b.WriteString("│[-]")
			if marked {
				b.WriteString("[yellow]")
			}
		}

		if q != "" {
			b.WriteString(highlightQuery(line, q, marked))
		} else {
			b.WriteString(escapeTview(line))
		}

		if marked {
			b.WriteString("[-]")
		}
		b.WriteString(`[""]`)
		b.WriteByte('\n')
	}
	v.text.SetText(b.String())
	if len(v.lines) > 0 {
		v.text.Highlight(strconv.Itoa(v.cursor))
	}
	v.updateChrome()
}

func (v *logsView) updateChrome() {
	trunc := ""
	if v.truncated {
		trunc = fmt.Sprintf(" [gray](last %d/%d)[-]", len(v.lines), v.totalRaw)
	}
	v.titleBar.SetText(fmt.Sprintf(" [orange]%s[-]%s", escapeTview(v.title), trunc))

	onOff := func(on bool) string {
		if on {
			return "[green]on[-]"
		}
		return "[red]off[-]"
	}
	v.controlsBar.SetText(fmt.Sprintf(
		"autoscroll: %s  wrap: %s  linenums: %s  find: %s  marks: %s",
		onOff(v.autoscroll),
		onOff(v.wrap),
		onOff(v.lineNums),
		v.findStatus(),
		v.marksStatus(),
	))

	v.metaBar.SetText(fmt.Sprintf("[gray]%d lines[-] ", len(v.lines)))

	status := ""
	if v.statusMsg != "" {
		status = "[yellow]" + escapeTview(v.statusMsg) + "[-] "
		v.statusMsg = ""
	}
	v.footer.SetText(status) // status only; all actions live in Actions column + controls
}

func (v *logsView) findStatus() string {
	q := strings.TrimSpace(v.query)
	if q == "" {
		return "[red]off[-]"
	}
	if len(v.matches) == 0 {
		return "[red]0[-]"
	}
	return fmt.Sprintf("[green]%d/%d[-]", v.matchIdx+1, len(v.matches))
}

func (v *logsView) marksStatus() string {
	if len(v.marks) == 0 {
		return "[red]off[-]"
	}
	return fmt.Sprintf("[green]%d[-]", len(v.marks))
}

func escapeTview(s string) string {
	return strings.ReplaceAll(s, "[", "[[")
}

func highlightQuery(line, query string, marked bool) string {
	if query == "" {
		return escapeTview(line)
	}
	lowerLine := strings.ToLower(line)
	lowerQ := strings.ToLower(query)
	var b strings.Builder
	rest := line
	restLower := lowerLine
	for {
		idx := strings.Index(restLower, lowerQ)
		if idx < 0 {
			b.WriteString(escapeTview(rest))
			break
		}
		b.WriteString(escapeTview(rest[:idx]))
		match := rest[idx : idx+len(query)]
		b.WriteString("[#ffcc00::b]")
		b.WriteString(escapeTview(match))
		if marked {
			b.WriteString("[yellow]")
		} else {
			b.WriteString("[-]")
		}
		rest = rest[idx+len(query):]
		restLower = restLower[idx+len(lowerQ):]
	}
	return b.String()
}
