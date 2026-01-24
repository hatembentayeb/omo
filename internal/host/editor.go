package host

import (
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/pgavlin/femto"
	"github.com/pgavlin/femto/runtime"
	"github.com/rivo/tview"
)

func saveBuffer(b *femto.Buffer, path string) error {
	return os.WriteFile(path, []byte(b.String()), 0600)
}

func newEditor(app *tview.Application, path string) *femto.View {
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
		}
		return event
	})

	return root
}
