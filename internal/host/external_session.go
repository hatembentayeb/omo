package host

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"omo/pkg/pluginrpc"
)

// launchExternalSession suspends the TUI and runs a local command (interactive ssh, etc.).
// Must be called from the tview thread (e.g. inside QueueUpdate).
func (r *RPCRenderer) launchExternalSession(sess pluginrpc.ExternalSession) {
	if len(sess.Argv) == 0 {
		return
	}
	r.app.Suspend(func() {
		fmt.Print("\033[H\033[2J")
		if sess.Banner != "" {
			fmt.Print(sess.Banner)
			if !strings.HasSuffix(sess.Banner, "\n") {
				fmt.Println()
			}
			fmt.Println()
		}
		cmd := exec.Command(sess.Argv[0], sess.Argv[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if sess.Password != "" {
			askpass, cleanup := writeAskpassHelper(sess.Password)
			if cleanup != nil {
				defer cleanup()
			}
			if askpass != "" {
				cmd.Env = append(os.Environ(),
					"SSH_ASKPASS="+askpass,
					"SSH_ASKPASS_REQUIRE=force",
					"DISPLAY=:0",
				)
			}
		}

		if err := cmd.Run(); err != nil {
			fmt.Printf("\nExited: %v\nPress Enter to return to omo...\n", err)
			fmt.Scanln()
		}
	})
	r.FocusTable()
}

func writeAskpassHelper(password string) (string, func()) {
	f, err := os.CreateTemp("", "omo-askpass-*.sh")
	if err != nil {
		return "", nil
	}
	escaped := strings.ReplaceAll(password, "'", "'\\''")
	script := fmt.Sprintf("#!/bin/sh\necho '%s'\n", escaped)
	if _, err := f.WriteString(script); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", nil
	}
	f.Close()
	if err := os.Chmod(f.Name(), 0700); err != nil {
		os.Remove(f.Name())
		return "", nil
	}
	return f.Name(), func() { os.Remove(f.Name()) }
}
