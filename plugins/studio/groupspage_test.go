package main

// THE ROUTINES ARE GROUPED BY THE APPLICATION THEY ARE FOR.
//
// The owner: "grouping routines by the app they target so you can collapse say all ark ascended
// routines or only show one set of routines. Add collapse all and expand all options too".
//
// Rendered, because a group is a fact about the drawn list: which rows are under which head,
// which are hidden, and what a press on a head or on the bar changes. See
// [[ADR-332-routines-grouped-by-application]].

import "testing"

func TestTheRoutinesAreGroupedByTheirApplication(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, theRowBackend+`
let r = {}
const card = (k) => document.getElementById('routine-' + k)
const shown = (el) => el && !el.classList.contains('hide') ? 1 : 0
const head = (g) => document.getElementById('routineGroup-' + g)
const shut = (g) => head(g).querySelector('button').getAttribute('aria-expanded') === 'false' ? 1 : 0
const shownCards = () => [...document.querySelectorAll('#routineList > .card.pick')]
  .filter(e => shown(e)).length
setTimeout(() => {
  // ONE HEAD PER APPLICATION, IN ORDER, WITH THE ROUTINES THAT SWITCH NOTHING LAST, and each
  // routine under its own application's head.
  r.heads = [...document.querySelectorAll('#routineList > .apphead')]
    .map(h => h.querySelector('.appname').textContent).join('|') ===
    'arkascended|notepad|Any application' ? 1 : 0
  const under = (k) => { let e = card(k).previousElementSibling
    while (e && !e.classList.contains('apphead')) { e = e.previousElementSibling }
    return e ? e.id : '' }
  r.under = under(K1) === 'routineGroup-app-arkascended' &&
    under(K2) === 'routineGroup-app-notepad' && under(K3) === 'routineGroup-anywhere' ? 1 : 0
  r.count = head('app-arkascended').querySelector('.appcount').textContent === '1 routine' ? 1 : 0
  r.bar = shown(document.getElementById('routineGroupBar'))
  r.allShown = shownCards()
  r.expandDark = document.getElementById('routineExpandAll').disabled ? 1 : 0
  // SHUT ONE APPLICATION'S ROUTINES BY ITS HEAD.
  press('routineGroupBtn-app-arkascended')
  r.arkShut = shut('app-arkascended') && !shown(card(K1)) && shown(card(K2)) && shown(card(K3)) ? 1 : 0
  r.arkHeadStays = shown(head('app-arkascended'))
  r.kept = (localStorage.getItem('studio.routineGroupsShut') || '').indexOf('arkascended') >= 0 ? 1 : 0
  // A REDRAW OF THE LIST KEEPS IT SHUT.
  drawRoutines()
  r.redrawKeeps = !shown(card(K1)) && shut('app-arkascended') ? 1 : 0
  // COLLAPSE ALL, AND EXPAND ALL.
  press('routineCollapseAll')
  r.collapsed = shownCards() === 0 && shut('app-notepad') && shut('anywhere') ? 1 : 0
  r.headsStay = [...document.querySelectorAll('#routineList > .apphead')].every(h => shown(h)) ? 1 : 0
  r.collapseDark = document.getElementById('routineCollapseAll').disabled ? 1 : 0
  press('routineExpandAll')
  r.expanded = shownCards() === 3 && !shut('app-arkascended') ? 1 : 0
  // ONLY ONE APPLICATION'S ROUTINES.
  const sel = document.getElementById('routineAppShow')
  r.options = [...sel.options].map(o => o.textContent).join('|') ===
    'All applications|arkascended|notepad|Any application' ? 1 : 0
  sel.value = 'g:notepad'
  sel.dispatchEvent(new Event('change'))
  r.onlyNotepad = shownCards() === 1 && shown(card(K2)) && !shown(head('app-arkascended')) &&
    !shown(head('anywhere')) && shown(head('app-notepad')) ? 1 : 0
  sel.value = 'g:'
  sel.dispatchEvent(new Event('change'))
  r.onlyAnywhere = shownCards() === 1 && shown(card(K3)) ? 1 : 0
  sel.value = ''
  sel.dispatchEvent(new Event('change'))
  r.allAgain = shownCards() === 3 ? 1 : 0
  // A ROUTINE OPENED WHILE ITS GROUP IS SHUT, OR FILTERED AWAY, IS SHOWN.
  press('routineCollapseAll')
  sel.value = 'g:notepad'
  sel.dispatchEvent(new Event('change'))
  reloadRoutines('arkascended/focus/cloud-pull-stone')
}, 1100)
setTimeout(() => {
  r.revealed = shown(card(K1)) && !shut('app-arkascended') &&
    document.getElementById('routineAppShow').value === '' ? 1 : 0
  r.othersStayShut = !shown(card(K2)) && shut('app-notepad') ? 1 : 0
  document.title = 'err=0 ' + Object.keys(r).map(k => k + '=' + r[k]).join(' ')
}, 1700)`)
	n := counters(t, title, "heads=")

	checks := []struct{ key, says string }{
		{"heads", "one head per application, in order, with Any application last"},
		{"under", "each routine sits under its own application's head"},
		{"count", "a head says how many routines it holds"},
		{"bar", "with three applications the Show / Expand all / Collapse all bar is shown"},
		{"expandDark", "Expand all is offered while nothing is shut"},
		{"arkShut", "pressing an application's head hides that application's routines and only those"},
		{"arkHeadStays", "a shut application's head is hidden with its routines"},
		{"kept", "a shut application is not kept for the next time the window opens"},
		{"redrawKeeps", "a redraw of the list opens a shut application again"},
		{"collapsed", "Collapse all leaves a routine showing"},
		{"headsStay", "Collapse all hides an application's head"},
		{"collapseDark", "Collapse all is still offered with everything shut"},
		{"expanded", "Expand all leaves a routine hidden"},
		{"options", "Show does not offer every application and All applications"},
		{"onlyNotepad", "showing one application shows anything else"},
		{"onlyAnywhere", "showing Any application alone does not show only the routine with no application"},
		{"allAgain", "All applications does not show every routine again"},
		{"revealed", "a routine opened in a shut, filtered-away group is opened out of sight"},
		{"othersStayShut", "opening one routine opened the other shut groups too"},
	}
	if n("allShown") != 3 {
		t.Fatalf("%d routines shown at first, want all 3: %s", n("allShown"), title)
	}
	for _, c := range checks {
		if n(c.key) != 1 {
			t.Errorf("%s: %s", c.says, title)
		}
	}
}
