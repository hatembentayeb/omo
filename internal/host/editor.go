package host

import (
	"log"
	"os"
	"strings"

	"omo/pkg/ui"

	"github.com/gdamore/tcell/v2"
	"github.com/pgavlin/femto"
	"github.com/pgavlin/femto/runtime"
	"github.com/rivo/tview"
)

func saveBuffer(b *femto.Buffer, path string) error {
	return os.WriteFile(path, []byte(b.String()), 0600)
}

func newEditor(app *tview.Application, pages *tview.Pages, path string) *femto.View {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("could not read %v: %v", path, err)
	}

	var colorscheme femto.Colorscheme
	if monokai := runtime.Files.FindFile(femto.RTColorscheme, "monokai"); monokai != nil {
		if data, err := monokai.Data(); err == nil {
			colorscheme = femto.ParseColorscheme(string(data))
		}
	}

	buffer := femto.NewBufferFromString(string(content), path)
	root := femto.NewView(buffer)

	root.SetRuntimeFiles(runtime.Files)
	root.SetColorscheme(colorscheme)
	root.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlS:
			saveBuffer(buffer, path)
			return nil
		case tcell.KeyCtrlQ:
			app.Stop()
			return nil
		case tcell.KeyCtrlH:
			ui.ShowInfoModal(
				pages,
				app,
				"Editor Help",
				femtoHelpText(),
				nil,
			)
			return nil
		}
		return event
	})

	return root
}

func femtoHelpText() string {
	return strings.TrimSpace(`
[yellow]Femto Editor Help[white]

[green]File[white]
Ctrl+S  - Save file
Ctrl+Q  - Quit editor
Ctrl+H  - Show this help

[green]Navigation[white]
Arrow keys  - Move cursor
Home/End    - Line start/end
PgUp/PgDn   - Page up/down

[green]Editing[white]
Ctrl+Z  - Undo
Ctrl+Y  - Redo
Ctrl+C  - Copy
Ctrl+X  - Cut
Ctrl+V  - Paste
Ctrl+A  - Select all
`)
}
