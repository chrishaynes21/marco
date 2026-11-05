package main

// A ROUTINE'S ROW SAYS ONLY WHAT CHANGES WHAT SOMEBODY DOES NEXT.
//
// The owner, on the routine list:
//
//	Window checks are implementation detail, not a useful routine status. Showing "Unchecked"
//	makes people think they've missed a verification step. Remove both Unchecked and
//	Window-checked from routine rows. Keep the checks underneath. [...] Ready ordinarily needs
//	no badge. Can't run shows a short reason and remedy. [...] Don't replace the badges with
//	another classification users have to learn.
//
// Rendered, because a badge is a fact about the drawn row: the markup test beside this one reads
// the line that chooses the word, and this one reads what a person sees. See
// [[ADR-309-a-routine-row-says-only-what-changes-the-next-action]].

import "testing"

func TestARoutineRowShowsABadgeOnlyWhenItCannotRun(t *testing.T) {
	browser := aBrowser()
	if browser == "" {
		t.Skip("no headless Chromium on this machine to render the page in")
	}
	title := renderForCounters(t, browser, theRowBackend+`
let r = {}
const pillOf = (k) => {
  const p = document.getElementById('routine-' + k).querySelector('.rowline > .pill')
  return p ? p.textContent : ''
}
setTimeout(() => {
  r.rows = document.querySelectorAll('#routineList > .card.pick').length
  // READY, GUARDED AND LEGACY ALIKE: NO BADGE.
  r.guardedBare = pillOf(K1) === '' ? 1 : 0
  r.legacyBare = pillOf(K2) === '' ? 1 : 0
  // CAN'T RUN, AND ITS REASON AND REMEDY BESIDE PLAY.
  r.cantPill = pillOf(K3) === "Can't run" ? 1 : 0
  r.cantBeside = (document.getElementById('routineSays-' + K3).textContent || '') ===
    'Its recording is incomplete. Record it again.' ? 1 : 0
  // AND NO WORD ABOUT CHECKING ANYWHERE IN THE LIST.
  const list = document.getElementById('routineList').textContent
  r.listSaysCheck = /check/i.test(list) ? 1 : 0
  press('routineOpen-' + K2)
}, 1100)
setTimeout(() => {
  // THE LEGACY ROUTINE, OPEN: its checks sentence is under More, not in front of Play.
  const checks = document.getElementById('routineChecks')
  r.checksUnderMore = checks.closest('#routineMaintain') ? 1 : 0
  r.checksFilled = checks.textContent.indexOf('not checked') >= 0 ? 1 : 0
  r.intoSaysCheck = /check/i.test(document.getElementById('routineIntoSays').textContent)
    ? 1 : 0
  press('routineOpen-' + K3)
}, 1700)
setTimeout(() => {
  // THE ONE THAT CANNOT RUN, OPEN: the banner is the short reason and remedy, and the
  // whole account is under More.
  const why = document.getElementById('routineWhy')
  r.bannerShown = why.classList.contains('hide') ? 0 : 1
  r.bannerShort = why.textContent === 'Its recording is incomplete. Record it again.'
    ? 1 : 0
  const full = document.getElementById('routineWhyFull')
  r.fullUnderMore = full.closest('#routineMaintain') ? 1 : 0
  r.fullKept = full.textContent.indexOf('how wide that window was') >= 0 ? 1 : 0
  document.title = 'err=0 ' + Object.keys(r).map(k => k + '=' + r[k]).join(' ')
}, 2300)`)
	n := counters(t, title, "rows=")

	if n("rows") != 3 {
		t.Fatalf("%d rows drawn, want 3: %s", n("rows"), title)
	}
	if n("guardedBare") != 1 || n("legacyBare") != 1 {
		t.Errorf("a routine that is ready to play carries a badge. Ready needs none, "+
			"whatever its window checks are: %s", title)
	}
	if n("cantPill") != 1 {
		t.Errorf("a routine that cannot run is not badged `Can't run`: %s", title)
	}
	if n("cantBeside") != 1 {
		t.Errorf("a routine that cannot run does not say its short reason and what "+
			"to do beside its Play: %s", title)
	}
	if n("listSaysCheck") != 0 {
		t.Errorf("the routine list says something about checking. The window checks "+
			"are how Marco works, and on a row they read as a step somebody "+
			"missed: %s", title)
	}
	if n("checksUnderMore") != 1 || n("checksFilled") != 1 {
		t.Errorf("a routine's checks sentence is not under More, or is not there at "+
			"all: %s", title)
	}
	if n("intoSaysCheck") != 0 {
		t.Errorf("the line beside Play talks about checking: %s", title)
	}
	if n("bannerShown") != 1 || n("bannerShort") != 1 {
		t.Errorf("the open routine that cannot run does not lead with the short "+
			"reason and remedy: %s", title)
	}
	if n("fullUnderMore") != 1 || n("fullKept") != 1 {
		t.Errorf("the whole refusal is not kept under More. Shortening the banner "+
			"must not lose which measurement is missing: %s", title)
	}
}
