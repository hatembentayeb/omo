package host

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"omo/pkg/pluginapi"
	"omo/pkg/ui"

	"github.com/rivo/tview"
)

type proverb struct {
	Text   string
	Author string
}

type headerProverb struct {
	app   *tview.Application
	root  *tview.Flex
	quote *tview.TextView
	wave  *tview.TextView

	gen     atomic.Uint64
	mu      sync.Mutex
	current proverb
	step    int
	pool    []proverb
	index   int
}

var fallbackProverbs = []proverb{
	{Text: "Slow is smooth, and smooth is fast.", Author: "ops lore"},
	{Text: "The best time to add observability is before you need it.", Author: "SRE"},
	{Text: "Make it work, make it right, make it fast — in that order.", Author: "Kent Beck"},
	{Text: "A system is only as available as its worst hidden dependency.", Author: "SRE"},
	{Text: "If it is not in version control, it does not exist.", Author: "ops lore"},
	{Text: "You cannot improve what you cannot see.", Author: "SRE"},
	{Text: "Hope is not a strategy.", Author: "ops lore"},
	{Text: "Fix the process, not just the incident.", Author: "SRE"},
}

func newHeaderProverb(app *tview.Application) *headerProverb {
	quote := tview.NewTextView()
	quote.SetDynamicColors(true)
	quote.SetWrap(true)
	quote.SetWordWrap(true)
	quote.SetTextAlign(tview.AlignCenter)
	quote.SetBackgroundColor(ui.ColorAppBg)
	quote.SetBorderPadding(0, 0, 1, 1)

	wave := tview.NewTextView()
	wave.SetDynamicColors(true)
	wave.SetWrap(false)
	wave.SetTextAlign(tview.AlignCenter)
	wave.SetBackgroundColor(ui.ColorAppBg)
	wave.SetBorderPadding(0, 0, 1, 1)

	root := tview.NewFlex()
	root.SetDirection(tview.FlexRow)
	root.SetBackgroundColor(ui.ColorAppBg)
	root.AddItem(quote, 0, 1, false)
	root.AddItem(wave, 1, 0, false)

	hp := &headerProverb{
		app:   app,
		root:  root,
		quote: quote,
		wave:  wave,
		pool:  append([]proverb(nil), fallbackProverbs...),
	}
	hp.current = hp.pool[0]
	hp.paint()
	return hp
}

func (p *headerProverb) View() tview.Primitive {
	if p == nil {
		return tview.NewBox().SetBackgroundColor(ui.ColorAppBg)
	}
	return p.root
}

func (p *headerProverb) Start() {
	if p == nil || p.app == nil {
		return
	}
	id := p.gen.Add(1)
	go p.loop(id)
	go p.refreshPool(id)
}

func (p *headerProverb) Stop() {
	if p == nil {
		return
	}
	p.gen.Add(1)
}

func (p *headerProverb) Restyle() {
	if p == nil {
		return
	}
	p.quote.SetBackgroundColor(ui.ColorAppBg)
	p.wave.SetBackgroundColor(ui.ColorAppBg)
	p.root.SetBackgroundColor(ui.ColorAppBg)
	p.paint()
}

func (p *headerProverb) loop(id uint64) {
	tick := time.NewTicker(90 * time.Millisecond)
	rotate := time.NewTicker(18 * time.Second)
	defer tick.Stop()
	defer rotate.Stop()
	for {
		if p.gen.Load() != id {
			return
		}
		select {
		case <-tick.C:
			p.mu.Lock()
			p.step++
			p.mu.Unlock()
			p.app.QueueUpdateDraw(func() {
				if p.gen.Load() != id {
					return
				}
				p.paintWave()
			})
		case <-rotate.C:
			p.next()
			p.app.QueueUpdateDraw(func() {
				if p.gen.Load() != id {
					return
				}
				p.paint()
			})
		}
	}
}

func (p *headerProverb) next() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.pool) == 0 {
		return
	}
	p.index = (p.index + 1) % len(p.pool)
	p.current = p.pool[p.index]
}

func (p *headerProverb) paint() {
	if p == nil || p.quote == nil {
		return
	}
	p.mu.Lock()
	cur := p.current
	p.mu.Unlock()
	text := strings.TrimSpace(cur.Text)
	author := strings.TrimSpace(cur.Author)
	body := fmt.Sprintf("[%s::b]“%s”[-]", ui.HexValue, text)
	if author != "" {
		body += fmt.Sprintf("\n[%s]— %s[-]", ui.HexLabel, author)
	}
	p.quote.SetText(body)
	p.paintWave()
}

func (p *headerProverb) paintWave() {
	if p == nil || p.wave == nil {
		return
	}
	p.mu.Lock()
	step := p.step
	p.mu.Unlock()
	width := 48
	if _, _, w, _ := p.wave.GetInnerRect(); w > 8 {
		width = w
	}
	p.wave.SetText(renderWave(step, width))
}

func renderWave(step, width int) string {
	if width < 8 {
		width = 8
	}
	glyphs := []rune("▁▂▃▄▅▆▇█▇▆▅▄▃▂")
	n := len(glyphs)
	var b strings.Builder
	b.Grow(width + 16)
	b.WriteString("[" + ui.HexActionKey + "]")
	for i := 0; i < width; i++ {
		b.WriteRune(glyphs[(i+step)%n])
	}
	b.WriteString("[-]")
	return b.String()
}

func (p *headerProverb) refreshPool(id uint64) {
	quotes := fetchProverbs(4)
	if p.gen.Load() != id || len(quotes) == 0 {
		return
	}
	p.mu.Lock()
	p.pool = append(fallbackProverbs, quotes...)
	p.current = quotes[0]
	p.index = len(fallbackProverbs)
	p.mu.Unlock()
	p.app.QueueUpdateDraw(func() {
		if p.gen.Load() != id {
			return
		}
		p.paint()
	})
}

func fetchProverbs(n int) []proverb {
	if n < 1 {
		n = 1
	}
	out := fetchQuotable(n)
	if len(out) == 0 {
		out = fetchZenQuotes()
	}
	if len(out) == 0 {
		if one, ok := fetchAdviceSlip(); ok {
			out = []proverb{one}
		}
	}
	return out
}

func fetchQuotable(n int) []proverb {
	url := fmt.Sprintf("https://api.quotable.io/quotes/random?limit=%d&maxLength=110", n)
	body, err := httpGet(url)
	if err != nil {
		return nil
	}
	var rows []struct {
		Content string `json:"content"`
		Author  string `json:"author"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil
	}
	out := make([]proverb, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.Content) == "" {
			continue
		}
		out = append(out, proverb{Text: row.Content, Author: row.Author})
	}
	return out
}

func fetchZenQuotes() []proverb {
	body, err := httpGet("https://zenquotes.io/api/quotes")
	if err != nil {
		return nil
	}
	var rows []struct {
		Q string `json:"q"`
		A string `json:"a"`
	}
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil
	}
	out := make([]proverb, 0, 8)
	for i, row := range rows {
		if i >= 8 || strings.TrimSpace(row.Q) == "" {
			continue
		}
		out = append(out, proverb{Text: row.Q, Author: row.A})
	}
	return out
}

func fetchAdviceSlip() (proverb, bool) {
	body, err := httpGet("https://api.adviceslip.com/advice")
	if err != nil {
		return proverb{}, false
	}
	var payload struct {
		Slip struct {
			Advice string `json:"advice"`
		} `json:"slip"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || strings.TrimSpace(payload.Slip.Advice) == "" {
		return proverb{}, false
	}
	return proverb{Text: payload.Slip.Advice, Author: "Advice"}, true
}

func httpGet(url string) ([]byte, error) {
	client := pluginapi.NewHTTPClient(4 * time.Second)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "omo (https://github.com/hatembentayeb/omo)")
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("%s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}
