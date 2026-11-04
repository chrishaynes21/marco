package main

// THE WIDGET'S THEME, ON THE RENDERED PAGE (S10).
//
// internal/widgetconfig holds the token values to their floors; this holds the page to using
// them. A page that never asked, asked and ignored the answer, or applied it once and never
// again, passes every Go test of the mapping. So the real page is rendered with Theme answering
// the real token sets: one palette at start-up and another after, and what the body is painted
// with is read back.

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
)

// themeHarness is the clean-machine window with Theme answering `first` on its first call and
// `then` after it. THE SLOW POLL IS TEN SECONDS, longer than the render's virtual time, so the
// harness shortens every wait of ten seconds or more to one; the page's own code is unchanged.
func themeHarness(t *testing.T, first, then string) string {
	t.Helper()
	enc := func(s string) string {
		if s == "reject" {
			return "null"
		}
		b, err := json.Marshal(widgetconfig.StudioTokens(s))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	return readyHarnessWith(aCleanMachine) + `
;(function(){
  const st = window.setTimeout
  window.setTimeout = (f, d, ...a) => st(f, d >= 10000 ? 1000 : d, ...a)
  const first = ` + enc(first) + `, then = ` + enc(then) + `
  window.__themeCalls = 0
  window.go.main.Acts.Theme = () => {
    window.__themeCalls++
    const t = window.__themeCalls === 1 ? first : then
    return t ? Promise.resolve(t) : Promise.reject(new Error('no theme'))
  }
  const read = () => {
    const root = document.documentElement
    return getComputedStyle(document.body).backgroundColor + '|' +
      (root.dataset.theme || '-') + '|' + (root.style.colorScheme || '-') + '|' +
      (root.dataset.palette || '-')
  }
  st(() => { window.__early = read() }, 600)
  st(() => {
    document.title = 'early=' + window.__early + ' late=' + read() + ' calls=' + window.__themeCalls
  }, 5200)
})();
`
}

func rgbText(hex string) string {
	var r, g, b int
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("rgb(%d, %d, %d)", r, g, b)
}

// THE PAGE DRAWS THE WIDGET'S PALETTE AT START-UP, AND A LATER CHANGE ON ITS SLOW POLL.
func TestStudioDrawsTheWidgetsThemeAndFollowsAChange(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, themeHarness(t, "nord", "light"))
	if !strings.Contains(title, "late=") {
		t.Fatalf("the page never reported: %q", title)
	}
	nord := rgbText(widgetconfig.StudioTokens("nord").Tokens["--ground"])
	light := rgbText(widgetconfig.StudioTokens("light").Tokens["--ground"])
	if want := "early=" + nord + "|dark|dark|nord "; !strings.Contains(title, want) {
		t.Errorf("at start-up the page is %q, want it to contain %q: the widget's theme was not applied", title, want)
	}
	if want := "late=" + light + "|light|light|light "; !strings.Contains(title, want) {
		t.Errorf("after a theme change the page is %q, want it to contain %q: the slow poll did not apply it", title, want)
	}
}

// AND WITH NO ANSWER, THE PAGE KEEPS ITS OWN TOKENS AND STILL RUNS. A Theme that fails must not
// take the readiness poll with it.
func TestStudioKeepsItsOwnTokensWhenThereIsNoTheme(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, themeHarness(t, "reject", "reject"))
	if !strings.Contains(title, "late=") {
		t.Fatalf("the page never reported: %q", title)
	}
	if !strings.Contains(title, "|-|-|-") {
		t.Errorf("with no theme answer the page set one anyway: %q", title)
	}
	if strings.Contains(title, "calls=0") || strings.Contains(title, "calls=1 ") {
		t.Errorf("the page stopped asking after a failed answer: %q", title)
	}
}
