package main

// THE WIDGET'S SETTINGS PANE: the backend's two halves, and the rendered page saving one setting
// at a time, redrawing with a theme it saved, and saying a refusal under the setting.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
)

func widgetSettingsAt(t *testing.T, contents string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "overlay.json")
	if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MARCO_OVERLAY_CONFIG", p)
	for _, k := range []string{"THEME", "IDLE", "MONITOR", "SIZE", "LEADER", "FONT", "MAXLINES"} {
		t.Setenv("MARCO_OVERLAY_"+k, "")
	}
	t.Setenv("MARCO_VOICE", "")
	return p
}

func savedFile(t *testing.T, p string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// THE MONITOR IS COUNTED FROM 1 IN THE PANE, AS THE WIDGET'S PANEL SHOWS IT, AND FROM 0 IN THE
// FILE, BOTH WAYS.
func TestTheWidgetSettingsCountMonitorsFromOne(t *testing.T) {
	p := widgetSettingsAt(t, `{"monitor":1,"theme":"nord"}`)
	a := &Acts{}
	form := a.WidgetSettings()
	if form.Values["monitor"] != float64(2) {
		t.Errorf("the second monitor shows as %v, want 2", form.Values["monitor"])
	}
	if form.Values["theme"] != "nord" || len(form.Fields) == 0 || form.Path != p {
		t.Errorf("the pane does not show what the widget runs with: %+v", form)
	}
	if r := a.SaveWidgetSettings(map[string]any{"monitor": float64(1)}); r.Refused || len(r.Saved) != 1 {
		t.Fatalf("saving the first monitor: %+v", r)
	}
	if got := savedFile(t, p)["monitor"]; got != float64(0) {
		t.Errorf("monitor 1 was saved as %v, want 0", got)
	}
}

// A VALUE THE WIDGET WOULD NOT USE IS REFUSED IN A SENTENCE, AND NOTHING IS WRITTEN.
func TestTheWidgetSettingsRefuseInASentence(t *testing.T) {
	p := widgetSettingsAt(t, `{"theme":"nord"}`)
	r := (&Acts{}).SaveWidgetSettings(map[string]any{"corner": "middle"})
	if !r.Refused || len(r.Saved) != 0 {
		t.Fatalf("a corner the widget does not know was not refused: %+v", r)
	}
	if !strings.HasPrefix(r.Says, `"middle" is not a corner`) || !strings.HasSuffix(r.Says, ".") {
		t.Errorf("the refusal reads %q", r.Says)
	}
	if m := savedFile(t, p); len(m) != 1 {
		t.Errorf("a refused save wrote %v", m)
	}
}

// widgetPaneHarness opens the pane on the clean-machine window, clicks the nord swatch, then sets
// the corner to one the backend refuses, and reports what was sent and what the page shows.
func widgetPaneHarness(t *testing.T) string {
	t.Helper()
	widgetSettingsAt(t, `{"theme":"default","corner":"top-right"}`)
	form, err := json.Marshal((&Acts{}).WidgetSettings())
	if err != nil {
		t.Fatal(err)
	}
	return readyHarnessWith(aCleanMachine) + `
;(function(){
  const st = window.setTimeout
  const form = ` + string(form) + `
  window.__sent = []
  let theme = 'default'
  window.go.main.Acts.Features = () => Promise.resolve({acts: false, scenes: false,
    views: ['routines', 'history', 'help', 'widget']})
  window.go.main.Acts.WidgetSettings = () => Promise.resolve(JSON.parse(JSON.stringify(form)))
  window.go.main.Acts.SaveWidgetSettings = sent => {
    window.__sent.push(JSON.stringify(sent))
    if (sent.corner === 'bottom-left') {
      return Promise.resolve({saved: [], refused: true, says: 'Not that corner.'})
    }
    if (sent.theme) { theme = sent.theme }
    return Promise.resolve({saved: Object.keys(sent), refused: false, says: 'Saved.'})
  }
  window.go.main.Acts.Theme = () => Promise.resolve({asked: theme, theme: theme, light: false,
    accentFrom: 'name', tokens: theme === 'nord' ? {'--ground': '#2e3440'} : {}})
  st(() => { show('widget') }, 400)
  st(() => {
    const b = document.querySelector('#ws-theme .swatch[data-value="nord"]')
    if (b) { b.click() }
  }, 900)
  st(() => {
    const c = document.getElementById('ws-corner')
    if (c) { c.value = 'bottom-left'; c.dispatchEvent(new Event('change')) }
  }, 1400)
  st(() => {
    const on = document.querySelector('#ws-theme .swatch.on')
    const corner = document.getElementById('ws-corner')
    const says = corner ? corner.closest('.wsrow').querySelector('.wssays').textContent : '-'
    document.title = 'shown=' + !document.getElementById('view-widget').classList.contains('hide') +
      ' rows=' + document.querySelectorAll('#wsGroups .wsrow').length +
      ' on=' + (on ? on.dataset.value : '-') +
      ' ground=' + getComputedStyle(document.body).backgroundColor +
      ' corner=' + (corner ? corner.value : '-') +
      ' says=' + says + ' sent=' + window.__sent.join(';')
  }, 5000)
})();
`
}

// THE RENDERED PANE SAVES ONE SETTING AT A TIME, REDRAWS WITH A THEME IT SAVED, AND SAYS A
// REFUSAL UNDER THE SETTING WITH THE CONTROL PUT BACK.
func TestTheWidgetSettingsPaneSavesAndRedraws(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, widgetPaneHarness(t))
	for _, want := range []string{
		"shown=true",
		"on=nord",
		"ground=rgb(46, 52, 64)",
		"corner=top-right",
		"says=Not that corner.",
		`sent={"theme":"nord"};{"corner":"bottom-left"}`,
	} {
		if !strings.Contains(title, want) {
			t.Errorf("the pane reports %q, want it to contain %q", title, want)
		}
	}
	if !strings.Contains(title, fmt.Sprintf("rows=%d", len(widgetconfig.Fields()))) {
		t.Errorf("the pane does not draw every setting: %q", title)
	}
}
