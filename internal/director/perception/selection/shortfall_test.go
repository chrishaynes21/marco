package selection_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// The reason a candidate was refused, and that saying it did not change the rule.
//
// Experiment-025 is why this exists: ninety-two directional presses at a menu, and the
// session's whole account of the failure was "no list has been seen handing a mark between
// its members (8 candidates watched over 106 frames)". Five different defects produce that
// sentence and they call for five different responses.

// qualifiedBefore is the predicate as it stood before Evidence.Unmet existed, copied
// literally from the commit that is being refactored.
//
// It is here so the refactor can be PROVEN not to have changed a classification, which is the
// one thing the person authorising this instrumentation asked to be sure of. A test that
// exercised only the new path would agree with itself.
func qualifiedBefore(e selection.Evidence) bool {
	judged := e.Frames - e.Unreadable
	if judged <= 0 {
		return false
	}
	return judged >= selection.MinObservations &&
		e.Exclusive >= selection.MinExclusive &&
		e.Exclusive > selection.SharedPenalty*e.Shared &&
		e.Transfers >= selection.MinTransfers &&
		e.Holders >= selection.MinHolders
}

// Saying why did not change what qualifies.
//
// Deleting any clause of Evidence.Unmet must fail this.
func TestNamingTheShortfallDidNotChangeTheRule(t *testing.T) {
	// A grid wide enough to straddle every threshold in both directions, including the
	// degenerate rows — zero frames, all of them unreadable, more shared than exclusive.
	vals := []int{0, 1, 2, 3, 7, 8, 9, 10, 11, 25}
	cases, disagreements := 0, 0
	for _, frames := range vals {
		for _, unreadable := range vals {
			for _, excl := range vals {
				for _, shared := range vals {
					for _, transfers := range []int{0, 1, 2, 5} {
						for _, holders := range []int{0, 1, 2, 4} {
							e := selection.Evidence{
								Rows: 5, Frames: frames, Unreadable: unreadable,
								Exclusive: excl, Shared: shared,
								Transfers: transfers, Holders: holders,
							}
							cases++
							want := qualifiedBefore(e)
							got := len(e.Unmet()) == 0
							if got != want {
								disagreements++
								if disagreements < 4 {
									t.Errorf("Unmet()==empty is %v, old rule said %v, for %s",
										got, want, e)
								}
							}
						}
					}
				}
			}
		}
	}
	if cases < 10000 {
		t.Fatalf("only %d combinations exercised; the grid is not straddling the "+
			"thresholds it is supposed to straddle", cases)
	}
	if disagreements > 0 {
		t.Errorf("%d of %d combinations classify differently than before the refactor",
			disagreements, cases)
	}
}

// And the shortfall names the thing that is actually wrong, distinguishably.
//
// One case per branch, each the measured shape of a different real defect. Deleting a branch
// must fail this.
func TestTheShortfallNamesWhichRequirementFailed(t *testing.T) {
	for _, tc := range []struct {
		name string
		e    selection.Evidence
		want string
		have int
	}{{
		name: "nothing could be measured, a capture finding and not a list finding",
		e:    selection.Evidence{Rows: 8, Frames: 40, Unreadable: 40},
		want: "judged_frames", have: 0,
	}, {
		name: "candidate identity churning, so nothing is watched long enough",
		e: selection.Evidence{Rows: 8, Frames: 4, Exclusive: 3, Transfers: 2,
			Holders: 2},
		want: "judged_frames", have: 4,
	}, {
		name: "no mark visible in the shading at all — Experiment-025's pause menu",
		e:    selection.Evidence{Rows: 5, Frames: 27, Quiet: 27},
		want: "exclusive_marks", have: 0,
	}, {
		name: "a heads-up display, several values moving at once, correctly refused",
		e: selection.Evidence{Rows: 6, Frames: 40, Exclusive: 9, Shared: 30,
			Transfers: 5, Holders: 4},
		want: "exclusive_over_shared", have: 9,
	}, {
		name: "marked, but the mark never moved",
		e: selection.Evidence{Rows: 5, Frames: 30, Exclusive: 28, Quiet: 2,
			Transfers: 0, Holders: 1},
		want: "transfers", have: 0,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			unmet := tc.e.Unmet()
			if len(unmet) == 0 {
				t.Fatalf("%s qualifies, so this case tests nothing", tc.e)
			}
			var found *selection.Shortfall
			var keys []string
			for i := range unmet {
				keys = append(keys, unmet[i].Requirement)
				if unmet[i].Requirement == tc.want {
					found = &unmet[i]
				}
			}
			if found == nil {
				t.Fatalf("the shortfall is reported as %s; the defect here is %q, and a "+
					"reader pointed at the wrong requirement looks in the wrong place",
					strings.Join(keys, ", "), tc.want)
			}
			if found.Have != tc.have {
				t.Errorf("%s reports have=%d, want %d", tc.want, found.Have, tc.have)
			}
			if found.Want <= found.Have {
				t.Errorf("%s wants %d and has %d, which is not a shortfall",
					tc.want, found.Want, found.Have)
			}
			if found.Why == "" {
				t.Errorf("%s carries no sentence a person can read", tc.want)
			}
		})
	}
}

// A candidate nobody could measure says ONLY that.
//
// It is the case that reads worst if it is got wrong. Every other requirement is also unmet
// when nothing was judged — zero exclusive frames, zero transfers, one holder — so without
// this the report answers "why was this refused" with four requirements, three of which are
// consequences of the first and none of which is actionable. The real finding is about the
// capture or the detector and not about the list at all.
//
// Deleting the judged<=0 early return in Evidence.Unmet must fail this.
func TestACandidateNobodyCouldMeasureSaysOnlyThat(t *testing.T) {
	e := selection.Evidence{Rows: 8, Frames: 40, Unreadable: 40}
	unmet := e.Unmet()
	if len(unmet) != 1 {
		var keys []string
		for _, u := range unmet {
			keys = append(keys, u.Requirement)
		}
		t.Fatalf("a candidate judged on no frame reports %d shortfalls (%s); it should "+
			"report the one that is a finding, because the others are its consequences "+
			"and point a reader at the list instead of at the capture",
			len(unmet), strings.Join(keys, ", "))
	}
	if unmet[0].Requirement != "judged_frames" {
		t.Errorf("the single shortfall is %q, want judged_frames", unmet[0].Requirement)
	}
	if !strings.Contains(unmet[0].Why, "judged") {
		t.Errorf("the sentence does not say the frames could not be judged: %q",
			unmet[0].Why)
	}
}

// A candidate that qualifies falls short of nothing.
func TestAQualifyingCandidateFallsShortOfNothing(t *testing.T) {
	e := selection.Evidence{
		Rows: 5, Frames: 30, Exclusive: 20, Shared: 1, Quiet: 9,
		Transfers: 6, Holders: 4,
	}
	if u := e.Unmet(); len(u) != 0 {
		t.Errorf("a list plainly being navigated reports %d shortfall(s): %v", len(u), u)
	}
	if !qualifiedBefore(e) {
		t.Fatal("the fixture does not qualify under the old rule either, so it is the " +
			"wrong fixture")
	}
	if e.Judged() != 30 {
		t.Errorf("Judged() = %d, want 30", e.Judged())
	}
}
