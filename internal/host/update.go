package host

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/ui"

	"github.com/rivo/tview"
)

const githubLatestReleaseURL = "https://api.github.com/repos/hatembentayeb/omo/releases/latest"

// PaneView sits under the plugins list on the same status row as Dashboard / Packages.
func (h *Host) PaneView() tview.Primitive {
	h.paneBar = tview.NewTextView()
	h.paneBar.SetDynamicColors(true)
	h.paneBar.SetTextAlign(tview.AlignLeft)
	h.paneBar.SetWrap(false)
	h.paneBar.SetBackgroundColor(ui.ColorAppBg)
	h.paneBar.SetText(" " + formatPaneTabs(h.pluginsOn))
	return h.paneBar
}

func (h *Host) setVersionText() {
	if h == nil || h.versionBar == nil {
		return
	}
	h.versionBar.SetText(formatVersionLine(h.version, h.latestTag))
}

func formatVersionLine(current, latest string) string {
	cur := displayVersion(current)
	line := fmt.Sprintf("[%s]%s[-]", ui.HexLabel, cur)
	if updateAvailable(current, latest) {
		line += fmt.Sprintf("  [%s]↑ %s[-]", ui.HexInfoKey, displayVersion(latest))
	}
	return line
}

func displayVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "dev"
	}
	if isDevVersion(v) {
		return v
	}
	if !strings.HasPrefix(strings.ToLower(v), "v") {
		return "v" + v
	}
	return v
}

func (h *Host) pollGitHubUpdate() {
	latest, err := fetchLatestGitHubTag()
	if err != nil || latest == "" {
		return
	}
	apply := func() {
		h.latestTag = latest
		h.setVersionText()
	}
	if h.App == nil {
		apply()
		return
	}
	h.App.QueueUpdateDraw(apply)
}

func fetchLatestGitHubTag() (string, error) {
	client := pluginapi.NewHTTPClient(4 * time.Second)
	req, err := http.NewRequest(http.MethodGet, githubLatestReleaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "omo (https://github.com/hatembentayeb/omo)")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return "", fmt.Errorf("github releases: %s", resp.Status)
	}
	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.TagName), nil
}

func updateAvailable(current, latest string) bool {
	if strings.TrimSpace(latest) == "" {
		return false
	}
	if isDevVersion(current) {
		return true
	}
	return versionLess(normalizeVersion(current), normalizeVersion(latest))
}

func isDevVersion(v string) bool {
	n := strings.ToLower(strings.TrimSpace(v))
	return n == "" || n == "dev" || strings.HasPrefix(n, "dev-") || strings.HasPrefix(n, "dev+")
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	return v
}

func versionLess(a, b string) bool {
	as, bs := versionParts(a), versionParts(b)
	n := len(as)
	if len(bs) > n {
		n = len(bs)
	}
	for i := 0; i < n; i++ {
		var av, bv int
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		if av != bv {
			return av < bv
		}
	}
	return false
}

func versionParts(v string) []int {
	if v == "" {
		return nil
	}
	bits := strings.Split(v, ".")
	out := make([]int, 0, len(bits))
	for _, bit := range bits {
		n, err := strconv.Atoi(bit)
		if err != nil {
			n = 0
		}
		out = append(out, n)
	}
	return out
}
