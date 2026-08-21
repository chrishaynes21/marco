package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// One page is one Place, however the window is sized.
//
// # Where these numbers come from
//
// Not invented. Every composition below was read out of a real `semantic-memory.json` that a
// live Marco filled in over several sessions against Windows Settings. It had accumulated TWELVE
// durable Places for THREE pages, plus two loading frames and a search state — because each page
// had been recorded at a different window size and every size was a different screen.
//
// That store is the fixture. If the rule cannot make one Place out of the three recordings of
// Home, it has not fixed the thing that was actually wrong.
//
// # What must NOT happen
//
// Settings pages share enormous amounts of structure — the same nav rail, the same caption, the
// same search box. The failure this rule must not have is merging Home with Bluetooth & devices
// because eighty per cent of the tree is identical, and the cases below hold that as hard as they
// hold the merges.

// The three recordings of each page, exactly as the store had them.
func settingsHome() []observe.StructureSignature {
	return []observe.StructureSignature{
		composed(map[string]int{"button": 15, "combo_box": 1, "group": 20, "image": 14,
			"link": 1, "list": 2, "list_item": 22, "menu": 1, "menu_item": 1,
			"pane": 3, "text": 32, "text_field": 1, "unknown": 2, "window": 4},
			observe.TermBack, observe.TermSettings),
		composed(map[string]int{"button": 18, "combo_box": 1, "group": 20, "image": 14,
			"link": 1, "list": 2, "list_item": 22, "menu": 1, "menu_item": 1,
			"pane": 3, "scroll_bar": 1, "text": 32, "text_field": 1, "unknown": 2,
			"window": 4},
			observe.TermBack, observe.TermSettings),
		composed(map[string]int{"button": 18, "combo_box": 1, "group": 27, "image": 14,
			"link": 3, "list": 2, "list_item": 22, "menu": 1, "menu_item": 1,
			"pane": 4, "text": 49, "text_field": 1, "unknown": 1, "window": 4},
			observe.TermBack, observe.TermSettings),
	}
}

func settingsBluetooth() []observe.StructureSignature {
	return []observe.StructureSignature{
		composed(map[string]int{"button": 13, "group": 10, "image": 13, "list": 1,
			"list_item": 20, "menu": 1, "menu_item": 1, "pane": 3, "scroll_bar": 1,
			"text": 24, "text_field": 1, "unknown": 2, "window": 4},
			observe.TermBack, observe.TermSettings),
		composed(map[string]int{"button": 10, "group": 10, "image": 13, "list": 1,
			"list_item": 20, "menu": 1, "menu_item": 1, "pane": 3, "text": 24,
			"text_field": 1, "unknown": 2, "window": 4},
			observe.TermBack, observe.TermSettings),
		composed(map[string]int{"button": 10, "group": 12, "image": 13, "list": 2,
			"list_item": 21, "menu": 1, "menu_item": 1, "pane": 4, "text": 26,
			"text_field": 1, "unknown": 1, "window": 4},
			observe.TermBack, observe.TermSettings),
	}
}

func settingsMouse() []observe.StructureSignature {
	return []observe.StructureSignature{
		composed(map[string]int{"button": 16, "combo_box": 3, "group": 9, "image": 13,
			"list_item": 15, "menu": 1, "menu_item": 1, "pane": 3, "scroll_bar": 1,
			"slider": 2, "text": 21, "text_field": 1, "unknown": 2, "window": 4},
			observe.TermBack, observe.TermControls, observe.TermSettings),
		composed(map[string]int{"button": 13, "combo_box": 3, "group": 9, "image": 13,
			"list_item": 15, "menu": 1, "menu_item": 1, "pane": 3, "slider": 2,
			"text": 21, "text_field": 1, "unknown": 2, "window": 4},
			observe.TermBack, observe.TermControls, observe.TermSettings),
		composed(map[string]int{"button": 14, "combo_box": 3, "group": 14, "image": 13,
			"link": 6, "list_item": 15, "menu": 1, "menu_item": 1, "pane": 4,
			"slider": 2, "text": 29, "text_field": 1, "unknown": 1, "window": 4},
			observe.TermBack, observe.TermControls, observe.TermSettings),
	}
}

// settingsLoading is Home with a progress bar — the page part-way through arriving.
func settingsLoading() observe.StructureSignature {
	return composed(map[string]int{"button": 15, "combo_box": 1, "group": 20, "image": 15,
		"link": 1, "list": 2, "list_item": 22, "menu": 1, "menu_item": 1, "pane": 3,
		"progress_bar": 1, "text": 32, "text_field": 1, "unknown": 2, "window": 4},
		observe.TermBack, observe.TermSettings)
}

// CASES A–D. Every recording of one page is that page.
func TestOneSettingsPageIsOnePlaceAcrossLayouts(t *testing.T) {
	for _, family := range []struct {
		name string
		of   []observe.StructureSignature
	}{
		{"Home", settingsHome()},
		{"Bluetooth & devices", settingsBluetooth()},
		{"Mouse", settingsMouse()},
	} {
		t.Run(family.name, func(t *testing.T) {
			for i, a := range family.of {
				for j, b := range family.of {
					if i >= j {
						continue
					}
					got := observe.ExplainStructure(a, b)
					if got.Verdict != observe.MatchSame {
						t.Errorf("recording %d and recording %d of %s came out %q.\n"+
							"They are the same page at different window sizes, "+
							"and a store that tells them apart mints a new "+
							"Place every time somebody drags an edge.\nwhy: %+v",
							i+1, j+1, family.name, got.Verdict, got.Why)
					}
				}
			}
		})
	}
}

// CASES E–F, and the one this rule must not break.
//
// Settings pages share their nav rail, their caption, their search box and most of their tree.
// The whole risk of tolerating layout is that it tolerates the difference between pages too.
func TestSettingsPagesStayDistinct(t *testing.T) {
	families := map[string][]observe.StructureSignature{
		"Home":                settingsHome(),
		"Bluetooth & devices": settingsBluetooth(),
		"Mouse":               settingsMouse(),
	}
	for aName, a := range families {
		for bName, b := range families {
			if aName >= bName {
				continue
			}
			t.Run(aName+" vs "+bName, func(t *testing.T) {
				for i, x := range a {
					for j, y := range b {
						got := observe.ExplainStructure(x, y)
						if got.Verdict == observe.MatchSame {
							t.Errorf("%s recording %d matched %s recording "+
								"%d.\nThese are different pages, and "+
								"merging them would attach somebody's "+
								"answer to the wrong screen.\nwhy: %+v",
								aName, i+1, bName, j+1, got.Why)
						}
					}
				}
			})
		}
	}
}

// CASE H. A page part-way through arriving is not the page.
//
// A progress bar is deliberately NOT a layout role. Its arrival says the screen started loading,
// which is a real event and a different Stage — and the store this fixture came from had turned
// two such frames into durable Places, which is the other half of the same bug.
func TestALoadingFrameIsNotThePage(t *testing.T) {
	for i, home := range settingsHome() {
		got := observe.ExplainStructure(settingsLoading(), home)
		if got.Verdict == observe.MatchSame {
			t.Errorf("a loading frame matched Home recording %d. A progress bar arriving "+
				"is not a window being resized: the page is not there yet, and "+
				"treating it as Home would let a route start from a screen that is "+
				"still assembling.\nwhy: %+v", i+1, got.Why)
		}
	}
}

// A SMALL COMPOSITION IS STILL COMPARED EXACTLY, and this is the floor doing its work.
//
// The tolerance is a share of the larger count, and below seven that share rounds to one — which
// is exactly what it was before any of this. So the worry the original comment names, that two
// would start merging a four-item menu with a six-item one, is untouched: those still differ.
//
// Deleting the floor must fail this. So must making the share large enough to reach small counts.
func TestASmallCompositionIsStillComparedExactly(t *testing.T) {
	four := composed(map[string]int{"menu_item": 4}, observe.TermSettings)
	five := composed(map[string]int{"menu_item": 5}, observe.TermSettings)
	six := composed(map[string]int{"menu_item": 6}, observe.TermSettings)

	// One is still forgiven — a detector missing a control in one frame is why the floor
	// exists at all.
	if got := observe.ExplainStructure(four, five).Verdict; got != observe.MatchSame {
		t.Errorf("a four-item menu and a five-item one came out %q; one missed detection "+
			"must not lose a screen", got)
	}
	// Two is still a different menu.
	if got := observe.ExplainStructure(four, six); got.Verdict == observe.MatchSame {
		t.Errorf("a four-item menu matched a six-item one.\nThat is the merge this "+
			"comparison has refused since it was written, and a proportional "+
			"tolerance must not reach down far enough to permit it.\nwhy: %+v", got.Why)
	}
}

// AND THE SHARE IS SYMMETRIC. Asking whether A matches B must answer the same as B against A.
func TestTheToleranceDoesNotDependOnWhichWayItIsAsked(t *testing.T) {
	for _, family := range [][]observe.StructureSignature{
		settingsHome(), settingsBluetooth(), settingsMouse(),
	} {
		for _, a := range family {
			for _, b := range family {
				there := observe.ExplainStructure(a, b).Verdict
				back := observe.ExplainStructure(b, a).Verdict
				if there != back {
					t.Fatalf("A against B is %q and B against A is %q. A tolerance "+
						"taken from one side rather than the larger count makes "+
						"identity depend on which way it was asked.", there, back)
				}
			}
		}
	}
}

// page is one screen's durable signature.
func composed(roles map[string]int, terms ...observe.InterfaceTerm) observe.StructureSignature {
	return observe.StructureSignature{
		Subject: observe.SubjectState, Roles: roles, Terms: terms, TermsKnown: true,
	}
}

// CASES A–D AT THE STORE. Every recording of one page establishes ONE durable Place.
//
// # The property ambient Observe needs
//
// This is the whole roadmap in one assertion. A store offered the same page at three window
// sizes must end with one subject, not three, because otherwise memory grows with how long
// somebody watched rather than with what Marco learned.
//
// It goes through `Recall`, deliberately: establishment asks the identity layer "would Marco
// recognise this", never "is this exact signature already in the file". So the count below is
// decided by the same comparison a live recognition uses, and the two cannot drift apart.
func TestRepeatedLayoutsDoNotGrowTheDurablePlaceCount(t *testing.T) {
	for _, family := range []struct {
		name string
		of   []observe.StructureSignature
	}{
		{"Home", settingsHome()},
		{"Bluetooth & devices", settingsBluetooth()},
		{"Mouse", settingsMouse()},
	} {
		t.Run(family.name, func(t *testing.T) {
			seen := map[string]bool{}
			for i, sig := range family.of {
				id, ok := establishInto(seen, sig, family.of[:i])
				if !ok {
					t.Fatalf("recording %d could not be established", i+1)
				}
				seen[id] = true
			}
			if len(seen) != 1 {
				t.Fatalf("%d durable Places for one page recorded at %d window sizes.\n"+
					"Observation time must not imply memory growth: eight hours on "+
					"one screen, with somebody dragging the window edge, would "+
					"otherwise mint a Place every time.", len(seen), len(family.of))
			}
		})
	}
	// AND THE THREE PAGES REMAIN THREE. The same loop across families must not collapse them.
	all := map[string]bool{}
	var everything []observe.StructureSignature
	for _, family := range [][]observe.StructureSignature{
		settingsHome(), settingsBluetooth(), settingsMouse(),
	} {
		for _, sig := range family {
			id, _ := establishInto(all, sig, everything)
			all[id] = true
			everything = append(everything, sig)
		}
	}
	if len(all) != 3 {
		t.Fatalf("nine recordings of three pages became %d durable Places, want 3", len(all))
	}
}

// establishInto is the store's own rule: an existing subject that RECALLS as the same place is
// returned rather than added.
func establishInto(seen map[string]bool, sig observe.StructureSignature,
	already []observe.StructureSignature) (string, bool) {

	var remembered []observe.RememberedSubject
	for i, s := range already {
		remembered = append(remembered, observe.RememberedSubject{
			ID: "subj_" + string(rune('a'+i)), Structure: s,
		})
	}
	if got, verdict := observe.Recall(sig, remembered); verdict == observe.MatchSame {
		return got.ID, true
	}
	return "subj_" + string(rune('a'+len(already))), true
}
