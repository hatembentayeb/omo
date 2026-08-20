package host

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"omo/pkg/ui"

	"github.com/rivo/tview"
)

// LogoMood is the OMO mark shown at the top-right of the plugin header.
// It can flash short ASCII/emoji reactions when plugin actions succeed or fail.
type LogoMood struct {
	tv  *tview.TextView
	app *tview.Application

	gen atomic.Uint64
}

func newLogoMood(app *tview.Application) *LogoMood {
	tv := tview.NewTextView()
	tv.SetDynamicColors(true)
	tv.SetTextAlign(tview.AlignCenter)
	tv.SetBackgroundColor(ui.ColorAppBg)
	lm := &LogoMood{tv: tv, app: app}
	tv.SetText(lm.defaultText())
	return lm
}

func (l *LogoMood) View() *tview.TextView { return l.tv }

func (l *LogoMood) defaultText() string {
	return fmt.Sprintf("[%s::b]█▀█ █▀▄▀█ █▀█\n█▄█ █ ▀ █ █▄█", ui.HexActionKey)
}

// Restyle redraws the mark after a palette change.
func (l *LogoMood) Restyle() {
	if l == nil || l.tv == nil {
		return
	}
	l.tv.SetBackgroundColor(ui.ColorAppBg)
	l.tv.SetText(l.defaultText())
}

// FlashPending shows a quick "working…" beat before the action returns.
func (l *LogoMood) FlashPending(action string) {
	face, word := moodPending(action)
	hold := 800 * time.Millisecond
	if isQuietAction(action) {
		hold = 400 * time.Millisecond
	}
	l.play(face, word, "#FFB86C", hold, false)
}

// FlashResult shows a success or failure reaction, then restores the logo.
func (l *LogoMood) FlashResult(ok bool, action, reaction string) {
	face, word, color := moodResult(ok, action, reaction)
	hold := 2 * time.Second
	switch {
	case !ok:
		hold = 2500 * time.Millisecond
	case isQuietAction(action):
		hold = 900 * time.Millisecond
	}
	l.play(face, word, color, hold, true)
}

func (l *LogoMood) play(faces []string, word, color string, hold time.Duration, restore bool) {
	if l == nil || l.tv == nil || l.app == nil {
		return
	}
	id := l.gen.Add(1)
	word = clipWords(word, 12)

	go func() {
		for i, face := range faces {
			if l.gen.Load() != id {
				return
			}
			text := formatMood(face, word, color)
			l.app.QueueUpdateDraw(func() {
				if l.gen.Load() != id {
					return
				}
				l.tv.SetText(text)
			})
			if i < len(faces)-1 {
				time.Sleep(120 * time.Millisecond)
			}
		}
		time.Sleep(hold)
		if !restore || l.gen.Load() != id {
			return
		}
		l.app.QueueUpdateDraw(func() {
			if l.gen.Load() != id {
				return
			}
			l.tv.SetText(l.defaultText())
		})
	}()
}

func formatMood(face, word, color string) string {
	if word == "" {
		return fmt.Sprintf("[%s::b]%s[-]", color, face)
	}
	return fmt.Sprintf("[%s::b]%s\n[white]%s[-]", color, face, word)
}

func clipWords(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	parts := strings.Fields(s)
	if len(parts) > 2 {
		parts = parts[:2]
	}
	s = strings.Join(parts, " ")
	r := []rune(s)
	if len(r) > maxRunes {
		return string(r[:maxRunes])
	}
	return s
}

func isQuietAction(action string) bool {
	a := strings.ToLower(strings.TrimSpace(action))
	return strings.HasPrefix(a, "goto_") ||
		a == "refresh" || a == "" ||
		strings.HasPrefix(a, "view_") ||
		a == "info" || a == "help" ||
		a == "diff" || a == "server_info"
}

func moodPending(action string) (faces []string, word string) {
	kind := actionKind(action)
	word = pendingWord(kind, action)
	switch kind {
	case moodForward:
		return []string{"  (•_•)  ", " (•_•)> ", " (•_•)>¤"}, word
	case moodDelete, moodStop:
		return []string{"  (•_•)  ", "  (•_•)z "}, word
	case moodSync:
		return []string{"  ↻ (•_•)", "  ↻ (•‿•)"}, word
	case moodQuiet:
		return []string{"  (•_•)  "}, word
	default:
		return []string{"  (•_•)  ", "  (•‿•)  "}, word
	}
}

func moodResult(ok bool, action, reaction string) (faces []string, word, color string) {
	kind := actionKind(action)
	word = strings.TrimSpace(reaction)
	if word == "" {
		if ok {
			word = successWord(kind, action)
		} else {
			word = failWord(kind, action)
		}
	}

	if !ok {
		return []string{
			"  (╯°□°)╯ ",
			" (╯°□°)╯︵",
			"ᗜ_ᗜ 💢   ",
		}, word, "#FF5555"
	}

	switch kind {
	case moodForward:
		return []string{"  (•‿•)  ", " \\(•‿•)/ ", " ᕕ( ᐛ )ᕗ"}, word, "#50FA7B"
	case moodDelete, moodStop:
		return []string{"  (•‿•)  ", "  (•‿•)/ ", "  👋 bye "}, word, "#8BE9FD"
	case moodCreate, moodStart:
		return []string{"  (•‿•)  ", " \\(•‿•)/ ", "  ✧ yay ✧"}, word, "#50FA7B"
	case moodSync:
		return []string{"  ↻ (•‿•)", "  ↻ (•ᴗ•)", "  ✓ sync "}, word, "#FFB86C"
	case moodConnect:
		return []string{"  (•‿•)  ", "  🔌 hey ", " \\(•‿•)/ "}, word, "#8BE9FD"
	case moodQuiet:
		return []string{"  (•‿•)  ", "  (•ᴗ•)  "}, word, "#BD93F9"
	default:
		return []string{"  (•‿•)  ", " \\(•‿•)/ ", "  ✧(•‿•)✧"}, word, "#50FA7B"
	}
}

type moodKind int

const (
	moodDefault moodKind = iota
	moodForward
	moodStart
	moodStop
	moodCreate
	moodDelete
	moodSync
	moodConnect
	moodQuiet
)

func actionKind(action string) moodKind {
	a := strings.ToLower(strings.TrimSpace(action))
	switch {
	case isQuietAction(a):
		return moodQuiet
	case strings.Contains(a, "forward") && !strings.Contains(a, "stop"):
		return moodForward
	case hasToken(a, "stop", "kill", "close", "disconnect", "pause"):
		return moodStop
	case hasToken(a, "delete", "remove", "drop", "purge", "flush", "prune", "destroy"):
		return moodDelete
	case hasToken(a, "create", "add", "new", "insert", "put", "set", "stage", "publish", "write"):
		return moodCreate
	case hasToken(a, "start", "run", "exec", "execute", "apply", "pop", "checkout", "merge", "cherry"):
		return moodStart
	case hasToken(a, "sync", "fetch", "pull", "push", "refresh", "reload", "update", "install"):
		return moodSync
	case hasToken(a, "connect", "shell", "ssh", "login", "attach"):
		return moodConnect
	default:
		return moodDefault
	}
}

func hasToken(action string, tokens ...string) bool {
	parts := strings.FieldsFunc(action, func(r rune) bool {
		return r == '_' || r == '-' || r == '/' || r == ' '
	})
	set := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		set[p] = struct{}{}
	}
	// also allow substring for compound names
	for _, t := range tokens {
		if _, ok := set[t]; ok {
			return true
		}
		if strings.Contains(action, t) {
			return true
		}
	}
	return false
}

func pendingWord(kind moodKind, action string) string {
	switch kind {
	case moodForward:
		return "fwd…"
	case moodSync:
		return "sync…"
	case moodQuiet:
		return "…"
	default:
		if w := verbHint(action); w != "" {
			return w + "…"
		}
		return "…"
	}
}

func successWord(kind moodKind, action string) string {
	// Prefer curated verb → short reaction.
	if w := mapSuccessVerb(action); w != "" {
		return w
	}
	switch kind {
	case moodForward:
		return "yay!"
	case moodStop:
		return "stopped"
	case moodDelete:
		return "poof"
	case moodCreate:
		return "created"
	case moodStart:
		return "go!"
	case moodSync:
		return "synced"
	case moodConnect:
		return "in"
	case moodQuiet:
		return "ok"
	default:
		if w := verbHint(action); w != "" {
			return w
		}
		return "done"
	}
}

func failWord(kind moodKind, action string) string {
	if w := mapFailVerb(action); w != "" {
		return w
	}
	switch kind {
	case moodForward:
		return "nope"
	case moodQuiet:
		return "hmm"
	default:
		return "ouch"
	}
}

func mapSuccessVerb(action string) string {
	a := strings.ToLower(action)
	// longer / specific first
	pairs := [][2]string{
		{"stop_all", "cleared"},
		{"start_forward", "yay!"},
		{"cherry_pick", "picked"},
		{"close_connection", "closed"},
		{"create_queue", "queued"},
		{"create_exchange", "made"},
		{"delete_queue", "poof"},
		{"delete_exchange", "poof"},
		{"purge_queue", "empty"},
		{"browse_messages", "peek"},
		{"view_details", "👀"},
		{"view_message", "👀"},
		{"view_offsets", "👀"},
		{"view_stash", "👀"},
		{"server_info", "ℹ️"},
		{"connection_info", "ℹ️"},
		{"suggest_port", "port"},
		{"filter_namespace", "filter"},
		{"clear_namespace_filter", "all"},
		{"select_database", "db"},
		{"select_db", "db"},
		{"select_repo", "repo"},
		{"toggle_granularity", "toggle"},
		{"set_time_period", "range"},
		{"apply_stash", "applied"},
		{"pop_stash", "pop!"},
		{"push_tag", "tagged"},
		{"fetch_remote", "fetched"},
		{"prune_remote", "pruned"},
		{"create_account", "user+"},
		{"create_token", "token"},
		{"create_project", "proj+"},
		{"create_key", "key+"},
		{"create_bucket", "bucket"},
		{"create_folder", "folder"},
		{"refresh_app", "fresh"},
		{"open_objects", "open"},
		{"show_partitions", "parts"},
		{"show_messages", "msgs"},
	}
	for _, p := range pairs {
		if a == p[0] || strings.HasSuffix(a, "_"+p[0]) {
			return p[1]
		}
		if strings.Contains(a, p[0]) {
			return p[1]
		}
	}
	singles := [][2]string{
		{"forward", "yay!"},
		{"publish", "sent"},
		{"subscribe", "sub"},
		{"fetch", "fetched"},
		{"pull", "pulled"},
		{"push", "pushed"},
		{"sync", "synced"},
		{"stage", "staged"},
		{"unstage", "unstaged"},
		{"checkout", "switched"},
		{"restore", "restored"},
		{"revert", "reverted"},
		{"merge", "merged"},
		{"delete", "poof"},
		{"remove", "gone"},
		{"purge", "empty"},
		{"flush", "flush!"},
		{"kill", "dead"},
		{"stop", "stopped"},
		{"start", "go!"},
		{"create", "created"},
		{"connect", "in"},
		{"shell", "sh"},
		{"execute", "ran"},
		{"install", "in!"},
		{"update", "upd"},
		{"restart", "again"},
		{"scale", "scaled"},
		{"copy", "copied"},
		{"rename", "renamed"},
		{"download", "got"},
		{"upload", "up"},
		{"build", "built"},
		{"deploy", "shipped"},
		{"rollback", "back"},
		{"approve", "yes"},
		{"deny", "no"},
		{"enable", "on"},
		{"disable", "off"},
		{"refresh", "fresh"},
		{"filter", "ok"},
		{"logs", "📜"},
		{"info", "ℹ️"},
		{"diff", "Δ"},
	}
	for _, p := range singles {
		if hasToken(a, p[0]) {
			return p[1]
		}
	}
	return ""
}

func mapFailVerb(action string) string {
	a := strings.ToLower(action)
	switch {
	case strings.Contains(a, "forward"):
		return "nope"
	case strings.Contains(a, "connect"), strings.Contains(a, "shell"):
		return "offline"
	case strings.Contains(a, "push"), strings.Contains(a, "pull"), strings.Contains(a, "fetch"):
		return "reject"
	case strings.Contains(a, "delete"), strings.Contains(a, "purge"):
		return "stuck"
	case strings.Contains(a, "create"), strings.Contains(a, "publish"):
		return "fail"
	default:
		return ""
	}
}

func verbHint(action string) string {
	a := strings.ToLower(strings.TrimSpace(action))
	a = strings.TrimPrefix(a, "goto_")
	parts := strings.FieldsFunc(a, func(r rune) bool {
		return r == '_' || r == '-' || r == '/'
	})
	// skip boring prefixes
	skip := map[string]struct{}{
		"view": {}, "show": {}, "get": {}, "goto": {}, "open": {},
	}
	for _, p := range parts {
		if _, ok := skip[p]; ok {
			continue
		}
		if len(p) <= 10 {
			return p
		}
		return string([]rune(p)[:10])
	}
	return ""
}
