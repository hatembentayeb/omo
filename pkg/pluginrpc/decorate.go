package pluginrpc

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
