package ui

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const logsOSC52MaxBytes = 200_000

// copyToClipboard copies text to the system clipboard.
// Native tools (wl-copy / xclip / pbcopy / termux-clipboard-set) are tried first —
// OSC52 to os.Stdout is ignored by tcell's /dev/tty, which is why log "copy all"
// appeared broken.
func copyToClipboard(text string) error {
	if text == "" {
		return fmt.Errorf("nothing to copy")
	}
	if err := copyViaCommand(text); err == nil {
		return nil
	}
	return copyOSC52(text)
}

// pasteFromClipboard returns trimmed clipboard text, or an error when empty/unavailable.
// Prefer Termux / Wayland / X11 / macOS paste tools (no OSC52 paste).
func pasteFromClipboard() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	type spec struct {
		name string
		args []string
	}
	candidates := []spec{
		{"termux-clipboard-get", nil},
		{"wl-paste", []string{"--no-newline"}},
		{"xclip", []string{"-selection", "clipboard", "-o"}},
		{"xsel", []string{"--clipboard", "--output"}},
		{"pbpaste", nil},
	}
	var last error
	for _, c := range candidates {
		if _, err := exec.LookPath(c.name); err != nil {
			continue
		}
		cmd := exec.CommandContext(ctx, c.name, c.args...)
		out, err := cmd.Output()
		if err != nil {
			last = fmt.Errorf("%s: %w", c.name, err)
			continue
		}
		text := strings.TrimSpace(string(out))
		if text == "" {
			last = fmt.Errorf("%s: empty clipboard", c.name)
			continue
		}
		return text, nil
	}
	if last != nil {
		return "", last
	}
	return "", fmt.Errorf("no clipboard paste tool (termux-clipboard-get, wl-paste, xclip, xsel, pbpaste)")
}

func copyViaCommand(text string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	type spec struct {
		name string
		args []string
	}
	candidates := []spec{
		{"termux-clipboard-set", nil},
		{"wl-copy", nil},
		{"xclip", []string{"-selection", "clipboard"}},
		{"xsel", []string{"--clipboard", "--input"}},
		{"pbcopy", nil},
	}
	var last error
	for _, c := range candidates {
		if _, err := exec.LookPath(c.name); err != nil {
			continue
		}
		cmd := exec.CommandContext(ctx, c.name, c.args...)
		cmd.Stdin = strings.NewReader(text)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			last = fmt.Errorf("%s: %w", c.name, err)
			continue
		}
		return nil
	}
	if last != nil {
		return last
	}
	return fmt.Errorf("no clipboard tool (termux-clipboard-set, wl-copy, xclip, xsel, pbcopy)")
}

func copyOSC52(text string) error {
	if len(text) > logsOSC52MaxBytes {
		return fmt.Errorf("too large to copy (%d bytes)", len(text))
	}
	payload := base64.StdEncoding.EncodeToString([]byte(text))
	seq := osc52Sequence(payload)
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		_, werr := os.Stdout.WriteString(seq)
		return werr
	}
	defer tty.Close()
	_, err = tty.WriteString(seq)
	return err
}

func osc52Sequence(b64 string) string {
	inner := "\033]52;c;" + b64 + "\a"
	if os.Getenv("TMUX") != "" {
		return "\033Ptmux;\033" + inner + "\033\\"
	}
	return inner
}

// PasteFromClipboard returns trimmed system clipboard text.
func PasteFromClipboard() (string, error) {
	return pasteFromClipboard()
}

// CopyToClipboard copies text to the system clipboard.
func CopyToClipboard(text string) error {
	return copyToClipboard(text)
}
