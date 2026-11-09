package main

// THE COMPACT SHELL, AND WHAT IT MUST NOT COST.
//
// An open routine was five ruled rows and two disclosures; it is now one row of tabs whose
// panels open beneath it. A layout like that can lose work in three ways the old one could
// not: a tab that rebuilds its panel drops what was typed into it, a redraw of the list that
// rebuilds the open row drops what was typed anywhere in it, and opening another routine
// throws away whatever the person was in the middle of. These hold all three, together with the
// two emphasis rules the shell exists for: no tab open until one is asked for, and one filled
// button -- the open routine's Play -- while a routine is open.
//
// See [[Studio-coherence-audit-2026-11-02]], findings 1, 2 and 3.

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func TestTabsAndRedrawsKeepWhatWasTypedAndOneButtonIsFilled(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, stepEditBackend+`
const typeIn = (id, v) => {
  const el = document.getElementById(id)
  el.value = v
  el.dispatchEvent(new Event('input'))
}
const open = (panel) => !document.getElementById(panel).classList.contains('hide')
const PANELS = ['routineTabSteps', 'routineTabNames', 'routineTabWhere',
  'routineTabSource', 'routineMaintain']
let noneOpen = 0, recordQuiet = 0, othersOutlined = 0, onlySteps = 0, onlyNames = 0
let stepsKept = '', phraseKept = '', selected = '', redrawn = 0, afterRedraw = ''
let phraseAfterRedraw = '', tabAfterRedraw = 0, warned = '', keptOnLeave = ''

setTimeout(() => {
  pickId('notepad/focus/address')
}, 1000)
setTimeout(() => {
  noneOpen = PANELS.every(p => !open(p)) ? 1 : 0
  recordQuiet = document.getElementById('recordRevealBtn').classList.contains('go') ? 0 : 1
  const other = document.getElementById('routinePlay-notepad-focus-other')
  const bg = getComputedStyle(other).backgroundColor
  othersOutlined = (bg === 'rgba(0, 0, 0, 0)' || bg === 'transparent') ? 1 : 0
  press('routineStepsBtn')
}, 1200)
setTimeout(() => {
  onlySteps = (open('routineTabSteps') && PANELS.filter(open).length === 1) ? 1 : 0
  selected = document.getElementById('routineStepsBtn').getAttribute('aria-selected')
  typeIn('routineStepMs-1', '800')
  press('routineNamesTab')
  onlyNames = (open('routineTabNames') && PANELS.filter(open).length === 1) ? 1 : 0
  document.getElementById('routineAnswerPhrase').value = 'another phrase'
}, 1500)
setTimeout(() => {
  // BACK AND FORTH: each panel still holds what was typed into it.
  press('routineStepsBtn')
  stepsKept = document.getElementById('routineStepMs-1').value
  press('routineNamesTab')
  phraseKept = document.getElementById('routineAnswerPhrase').value
  // AND A REDRAW OF THE WHOLE LIST, forced by a routine that changed underneath.
  const was = window.go.main.Acts.Routines
  window.go.main.Acts.Routines = () => was().then(rs => rs.map(r =>
    Object.assign({}, r, {saved_at: '2026-11-02 10:00'})))
  window.drawsBefore = document.getElementById('routineList').innerHTML.length
}, 1800)
setTimeout(() => {
  redrawn = document.getElementById('routineList').textContent.includes('2026-11-02') ? 1 : 0
  phraseAfterRedraw = document.getElementById('routineAnswerPhrase').value
  tabAfterRedraw = open('routineTabNames') ? 1 : 0
  press('routineStepsBtn')
  afterRedraw = document.getElementById('routineStepMs-1').value
}, 3400)
setTimeout(() => {
  // AND OPENING ANOTHER ROUTINE ASKS FIRST, AND KEEPS EVERYTHING WHILE IT ASKS.
  document.getElementById('routineOpen-notepad-focus-other').click()
  const w = document.getElementById('routineLeaving')
  warned = w.classList.contains('hide') ? '' : w.textContent
  keptOnLeave = document.getElementById('routineStepMs-1').value
}, 3700)
setTimeout(() => {
  document.title = 'none=' + noneOpen + ' recordquiet=' + recordQuiet +
    ' outlined=' + othersOutlined + ' onlysteps=' + onlySteps +
    ' selected=' + (selected === 'true' ? 1 : 0) + ' onlynames=' + onlyNames +
    ' stepskept=' + (stepsKept === '800' ? 1 : 0) +
    ' phrasekept=' + (phraseKept === 'another phrase' ? 1 : 0) +
    ' redrawn=' + redrawn +
    ' phraseredraw=' + (phraseAfterRedraw === 'another phrase' ? 1 : 0) +
    ' tabredraw=' + tabAfterRedraw +
    ' stepsredraw=' + (afterRedraw === '800' ? 1 : 0) +
    ' warned=' + (warned.includes('change you made to its steps') ? 1 : 0) +
    ' keptonleave=' + (keptOnLeave === '800' ? 1 : 0)
}, 4200)`)
	if !strings.Contains(title, "none=") {
		t.Fatalf("the page never reported its counters: %q", title)
	}
	n := func(key string) int {
		m := regexp.MustCompile(`\b` + key + `=(\d+)`).FindStringSubmatch(title)
		if m == nil {
			t.Fatalf("no %s in %q", key, title)
		}
		v, _ := strconv.Atoi(m[1])
		return v
	}
	for _, c := range []struct{ key, why string }{
		{"none", "a play opened with a tab already open"},
		{"recordquiet", "Record is still filled while a play is open"},
		{"outlined", "a shut row's Play is filled, so the open play's is not the one filled button"},
		{"onlysteps", "pressing Steps did not open its panel alone"},
		{"selected", "the open tab does not say it is selected"},
		{"onlynames", "pressing Other names did not open its panel alone"},
		{"stepskept", "a typed wait was lost by switching tabs"},
		{"phrasekept", "a typed phrase was lost by switching tabs"},
		{"redrawn", "the list was never redrawn, so this proves nothing about a redraw"},
		{"phraseredraw", "a redraw of the list lost a typed phrase"},
		{"tabredraw", "a redraw of the list closed the open tab"},
		{"stepsredraw", "a redraw of the list lost a typed wait"},
		{"warned", "opening another play did not say what it would throw away"},
		{"keptonleave", "the typing was lost while the window asked"},
	} {
		if n(c.key) != 1 {
			t.Errorf("%s: %q", c.why, title)
		}
	}
}

// EVERY TOKEN THAT COLOURS TEXT CAN BE READ, IN BOTH THEMES.
//
// `--faint` was 3.2:1 on a card in the dark theme and 3.6:1 in the light one, and it coloured
// the section labels, every disclosure, Help's section titles and the step numbers. Computed
// from the stylesheet's own tokens rather than from a list here, so changing a colour is
// checked when it is changed.
func TestEveryTextTokenMeetsContrastOnEverySurface(t *testing.T) {
	page := thePage(t)
	dark := between(t, page, ":root{", "}")
	light := between(t, page, `:root[data-theme="light"]{`, "}")
	for theme, block := range map[string]string{"dark": dark, "light": light} {
		tok := func(name string) string {
			m := regexp.MustCompile(`--` + name + `:(#[0-9a-fA-F]{6})`).FindStringSubmatch(block)
			if m == nil {
				t.Fatalf("the %s theme has no --%s", theme, name)
			}
			return m[1]
		}
		for _, fg := range []string{"ink", "dim", "faint", "spot", "no", "warn"} {
			for _, bg := range []string{"ground", "raise", "sink", "spot-bg"} {
				if r := contrast(tok(fg), tok(bg)); r < 4.5 {
					t.Errorf("%s theme: --%s on --%s is %.2f:1, below 4.5:1", theme, fg, bg, r)
				}
			}
		}
		// AND EACH SEMANTIC COLOUR ON ITS OWN TINT, which is where it is drawn.
		for _, p := range [][2]string{{"ok", "ok-bg"}, {"warn", "warn-bg"}, {"no", "no-bg"}} {
			if r := contrast(tok(p[0]), tok(p[1])); r < 4.5 {
				t.Errorf("%s theme: --%s on --%s is %.2f:1, below 4.5:1", theme, p[0], p[1], r)
			}
		}
	}
}

func contrast(a, b string) float64 {
	lum := func(h string) float64 {
		ch := func(i int) float64 {
			v, _ := strconv.ParseUint(h[i:i+2], 16, 8)
			c := float64(v) / 255
			if c <= 0.03928 {
				return c / 12.92
			}
			return math.Pow((c+0.055)/1.055, 2.4)
		}
		return 0.2126*ch(1) + 0.7152*ch(3) + 0.0722*ch(5)
	}
	x, y := lum(a), lum(b)
	if x < y {
		x, y = y, x
	}
	return (x + 0.05) / (y + 0.05)
}
