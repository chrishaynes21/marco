package observe_test

import (
	"fmt"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// THE RESPONSIVE BREAKPOINT, AND WHY IT IS STILL A FALSE MISS.
//
// # What was measured
//
// Three Windows Settings destinations, six window widths each, through the production
// perception path into `SignatureOfState` — 2026-08-28, isolated store, no detector. The
// breakpoint is between 850 and 800 px and is identical for all three pages: Windows removes the
// navigation pane. Verified SEMANTICALLY rather than by width — a `button "Open Navigation"` and
// a `button "Expand search box"` appear, and the navigation list disappears.
//
//	                    name                    terms                          identity roles
//	mouse 1500/1000/850 "Mouse"                 back controls settings         button 14 combo_box 3 image 13 list_item 15 menu 1 menu_item 1 slider 2 text_field 1 window 4
//	mouse 800/750/650   —                       back controls search settings  button 15 combo_box 3          list_item  3 menu 1 menu_item 1 slider 2              window 3
//	bt    1500/1000/850 "Devices"               audio back display settings    button 30 image 13 list 3 list_item 12 menu 1 menu_item 1 text_field 1 window 4
//	bt    800/750/650   —                       audio back display search set. button 31          list 3                menu 1 menu_item 1              window 3
//	print 1500/1000     "Printers & scanners"   back settings                  button 18 image 13 list 1 list_item 12 menu 1 menu_item 1 text_field 1 window 4
//	print 800/750/650   —                       back search settings           button 19          list 1                menu 1 menu_item 1              window 3
//
// Lost at the breakpoint, on every page: thirteen `image`, one `text_field`, the navigation's
// `list_item`s, and the selected navigation entry that told Marco what the page is CALLED.
// Gained: the `search` term, from the collapsed header's search affordance.
//
// # Why 37J changed nothing
//
// Because the loosening that would fix it is measurably unsafe on this very data, and because
// the one page it was written for cannot be fixed by it anyway.
//
//	MOUSE keeps three list items of its own, so the navigation's twelve leaving is a
//	POSITIVE COUNT DISAGREEMENT (15 against 3), not an absence. Bluetooth and Printers have
//	no content list items, so for them the same interface change is an absence. The identical
//	event reads as contradiction on one page and as silence on another, purely because of what
//	each page's own content happens to contain — and a flat role histogram cannot tell the two
//	apart, because the durable signature keeps no spatial decomposition. `ScreenSignature` has
//	cells; `StructureSignature` deliberately does not.
//
//	PRINTERS, collapsed, carries `back search settings` and nothing else. Those are the terms
//	of every Settings page. Its surviving roles are button 19, list 1, menu 1, menu_item 1,
//	window 3. There is no evidence in that reading that says which destination it is, and no
//	rule can invent one.
//
// Measured over all 45 same-destination pairs and every cross-destination pair: tolerating
// role-set absence and nesting the terms raises same-destination matches from 18 to 36 and
// produces FIFTEEN false merges, all of them Mouse against Printers — because Printers' terms
// are a subset of Mouse's. Requiring a shared distinctive term removes them, but "distinctive"
// cannot be declared statically: `settings` is the generic term of an operating system and the
// DESTINATION-BEARING term of a game's settings menu, which is what the vocabulary was built for.
//
// So the collapsed presentation stays unrecognised. A false miss is cheaper than a false merge,
// and this file is the firewall that keeps it that way.

// The measured signatures, transcribed from the live captures named above.
func settingsPage(page string, collapsed bool) observe.StructureSignature {
	roles := map[string]int{}
	var terms []observe.InterfaceTerm
	switch page {
	case "mouse":
		if collapsed {
			roles = map[string]int{"button": 15, "combo_box": 3, "list_item": 3, "menu": 1,
				"menu_item": 1, "slider": 2, "window": 3, "group": 12, "pane": 2, "text": 28,
				"link": 6, "unknown": 1}
			terms = []observe.InterfaceTerm{observe.TermBack, observe.TermControls,
				observe.TermSearch, observe.TermSettings}
		} else {
			roles = map[string]int{"button": 14, "combo_box": 3, "image": 13, "list_item": 15,
				"menu": 1, "menu_item": 1, "slider": 2, "text_field": 1, "window": 4,
				"group": 14, "pane": 4, "text": 29, "link": 6, "unknown": 1}
			terms = []observe.InterfaceTerm{observe.TermBack, observe.TermControls,
				observe.TermSettings}
		}
	case "bluetooth":
		if collapsed {
			roles = map[string]int{"button": 31, "list": 3, "menu": 1, "menu_item": 1,
				"window": 3, "group": 27, "pane": 2, "text": 51, "link": 6, "unknown": 1}
			terms = []observe.InterfaceTerm{observe.TermAudio, observe.TermBack,
				observe.TermDisplay, observe.TermSearch, observe.TermSettings}
		} else {
			roles = map[string]int{"button": 30, "image": 13, "list": 3, "list_item": 12,
				"menu": 1, "menu_item": 1, "text_field": 1, "window": 4,
				"group": 29, "pane": 4, "text": 52, "link": 6, "unknown": 1}
			terms = []observe.InterfaceTerm{observe.TermAudio, observe.TermBack,
				observe.TermDisplay, observe.TermSettings}
		}
	case "printers":
		if collapsed {
			roles = map[string]int{"button": 19, "list": 1, "menu": 1, "menu_item": 1,
				"window": 3, "group": 26, "pane": 2, "text": 40, "link": 4, "unknown": 1}
			terms = []observe.InterfaceTerm{observe.TermBack, observe.TermSearch,
				observe.TermSettings}
		} else {
			roles = map[string]int{"button": 18, "image": 13, "list": 1, "list_item": 12,
				"menu": 1, "menu_item": 1, "text_field": 1, "window": 4,
				"group": 28, "pane": 4, "text": 41, "link": 4, "unknown": 1}
			terms = []observe.InterfaceTerm{observe.TermBack, observe.TermSettings}
		}
	default:
		panic("unknown page " + page)
	}
	return observe.StructureSignature{
		Subject: observe.SubjectState, Roles: roles, Terms: terms, TermsKnown: true,
	}
}

var settingsDestinations = []string{"mouse", "bluetooth", "printers"}

// TWO DESTINATIONS NEVER MERGE, AT ANY PAIR OF PRESENTATIONS.
//
// The firewall, and the reason 37J shipped no loosening. Tolerating role-set absence and nesting
// the terms — the rule that would recognise a page across the breakpoint — merges Mouse with
// Printers at fifteen of these pairs, because `back settings` is a subset of `back controls
// settings` and every role the two share agrees within tolerance.
//
// Mutating sameRoleSet to compare only shared roles, or sameTerms to accept one set nested in
// the other, must fail this.
func TestTwoSettingsDestinationsNeverMergeAtAnyPresentation(t *testing.T) {
	for i, a := range settingsDestinations {
		for _, b := range settingsDestinations[i+1:] {
			for _, ca := range []bool{false, true} {
				for _, cb := range []bool{false, true} {
					x, y := settingsPage(a, ca), settingsPage(b, cb)
					if v := observe.CompareStructure(x, y); v == observe.MatchSame {
						t.Errorf("%s and %s merged (%s@%s = %s@%s).\n"+
							"%s\nA responsive rule that recognises one page across the "+
							"breakpoint by tolerating what the reflow removed must not "+
							"also merge two destinations that were never the same place.",
							a, b, a, presentation(ca), b, presentation(cb),
							explain(x, y))
					}
				}
			}
		}
	}
}

// A COLLAPSED READING WITH NOTHING DISTINCTIVE IN IT RESOLVES TO NOTHING.
//
// Printers, collapsed, carries the terms of every Settings page and five roles any of them
// might have. Handing that to memory and getting an answer would be Marco telling somebody
// where they are on the strength of a reading that cannot say.
//
// This is the case that makes "tolerate what the reflow removed" unsafe in general rather than
// merely unlucky: there is no evidence to be lenient WITH.
func TestACollapsedReadingWithNoDistinctiveEvidenceResolvesToNothing(t *testing.T) {
	collapsed := settingsPage("printers", true)
	remembered := []observe.RememberedSubject{
		{ID: "subj_mouse", Application: "settings", Structure: settingsPage("mouse", false)},
		{ID: "subj_bluetooth", Application: "settings", Structure: settingsPage("bluetooth", false)},
		{ID: "subj_printers", Application: "settings", Structure: settingsPage("printers", false)},
	}
	got, verdict := observe.Recall(collapsed, remembered)
	if verdict == observe.MatchSame {
		t.Errorf("a collapsed reading whose whole account is `back search settings` resolved "+
			"to %s.\nIts surviving roles are button, list, menu, menu_item and window — the "+
			"furniture every page of this application has. Recognising a destination from "+
			"that is not tolerance, it is invention.", got.ID)
	}
}

// CONTRADICTION IS NOT ABSENCE.
//
// The other half of the rule any future attempt has to obey. A reflow REMOVES evidence; a
// person who navigated brings different evidence. A collapsed Bluetooth reading positively
// carries `audio` and `display`, which the remembered Mouse place never had — that is a
// disagreement, not a silence, and no amount of leniency about what reflow removes may reach it.
func TestPositiveEvidenceForAnotherDestinationIsNotMissingEvidence(t *testing.T) {
	mouse := settingsPage("mouse", false)
	elsewhere := settingsPage("bluetooth", true)
	if v := observe.CompareStructure(elsewhere, mouse); v == observe.MatchSame {
		t.Fatalf("a reading carrying audio and display resolved to the Mouse place (%s)", v)
	}
	// And the evidence really is positive rather than merely different in quantity: the terms
	// the remembered place had are still there, and new ones have ARRIVED.
	lost, gained := termDelta(elsewhere.Terms, mouse.Terms)
	if len(gained) == 0 {
		t.Errorf("the fixture carries no arriving term, so it tests absence rather than "+
			"contradiction: lost=%v gained=%v", lost, gained)
	}
}

// AND THE FALSE MISS, STATED RATHER THAN HIDDEN.
//
// One page, two presentations, no navigation. This asserts the CURRENT answer and names the
// exact field that decides it, so that a future phase which changes it has to change this test
// deliberately and say why — and so that nobody reads the firewall above as a claim that the
// breakpoint is handled.
func TestTheResponsiveBreakpointIsStillAFalseMiss(t *testing.T) {
	for _, page := range settingsDestinations {
		wide, collapsed := settingsPage(page, false), settingsPage(page, true)
		cmp := observe.ExplainStructure(collapsed, wide)
		if cmp.Verdict == observe.MatchSame {
			t.Errorf("%s now matches across the breakpoint. That is the outcome 37J wanted "+
				"and did not ship, so it did not arrive by accident: check that "+
				"TestTwoSettingsDestinationsNeverMergeAtAnyPresentation still holds and "+
				"update this test with the rule that earned it.", page)
			continue
		}
		decisive := ""
		for _, d := range cmp.Why {
			if d.Decisive {
				decisive = d.Field
				break
			}
		}
		if decisive != "role_set" {
			t.Errorf("%s across the breakpoint is decided by %q, not the role set.\n%s\n"+
				"The measurement this file records says the navigation taking `image` and "+
				"`text_field` with it is what fires first; if that has changed, the "+
				"measurement is stale and the next phase is reasoning from the wrong cause.",
				page, decisive, explain(collapsed, wide))
		}
	}
}

func presentation(collapsed bool) string {
	if collapsed {
		return "collapsed"
	}
	return "wide"
}

func explain(a, b observe.StructureSignature) string {
	out := ""
	for _, d := range observe.ExplainStructure(a, b).Why {
		mark := " "
		if d.Decisive {
			mark = "*"
		}
		out += fmt.Sprintf("    %s %-10s %s | %s\n", mark, d.Field, d.Current, d.Remembered)
	}
	return out
}

// termDelta is what the first signature lacks that the second has, and the reverse.
func termDelta(current, remembered []observe.InterfaceTerm) (lost, gained []string) {
	has := func(list []observe.InterfaceTerm, t observe.InterfaceTerm) bool {
		for _, got := range list {
			if got == t {
				return true
			}
		}
		return false
	}
	for _, t := range remembered {
		if !has(current, t) {
			lost = append(lost, string(t))
		}
	}
	for _, t := range current {
		if !has(remembered, t) {
			gained = append(gained, string(t))
		}
	}
	return lost, gained
}

// TERMS ARE COMPARED EXACTLY, AND THAT IS WHY THE BREAKPOINT CANNOT BE BOUGHT WITH THEM.
//
// The tempting half of a responsive rule is to let one term set be nested in the other: the
// collapsed reading has everything the wide one had, plus `search`, so nesting would make them
// agree. On the Settings captures that change is invisible — the role set decides those pairs
// first — which is exactly why it needs its own gate rather than a hope that some other test
// notices.
//
// What it costs is here. Two screens of identical composition, one of which is ABOUT something
// the other is not: a settings screen and its audio page, a menu and its controls page. That is
// the shape the whole interface-term vocabulary was built to tell apart, and nesting dissolves
// it — the general screen's terms are a subset of every specialised screen's.
//
// Terms say what a screen is ABOUT. A screen that is about less is not the same screen.
//
// Making sameTerms accept one set nested in the other must fail this.
func TestAScreenAboutMoreThingsIsNotTheScreenAboutFewer(t *testing.T) {
	composition := map[string]int{
		"button": 8, "list_item": 6, "text": 12, "group": 3, "window": 1,
	}
	general := observe.StructureSignature{
		Subject: observe.SubjectState, Roles: composition, TermsKnown: true,
		Terms: []observe.InterfaceTerm{observe.TermSettings},
	}
	audio := observe.StructureSignature{
		Subject: observe.SubjectState, Roles: composition, TermsKnown: true,
		Terms: []observe.InterfaceTerm{observe.TermAudio, observe.TermSettings},
	}
	if v := observe.CompareStructure(audio, general); v == observe.MatchSame {
		t.Errorf("a screen about audio and settings matched one about settings alone (%s).\n"+
			"Their compositions are identical, so terms are the only thing telling them "+
			"apart — and a rule that accepts a subset gives every specialised page its "+
			"parent's identity.", v)
	}
	if v := observe.CompareStructure(general, audio); v == observe.MatchSame {
		t.Errorf("the same pair matched the other way round (%s); a nesting rule is not "+
			"made safe by asking it in the safer direction", v)
	}
}
