package verify

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Verifying a scroll asks where the view ENDED UP.
//
// The host reports that SetScrollPercent or ScrollVertical returned without throwing. That is
// delivery — the region accepted the call — and this package has learned three times now that a
// mechanism accepting an action is not evidence the requested effect occurred. So the verdict
// comes from the position the world reports, before against after, and the question is different
// for each of the four meanings:
//
//	top         the position IS the top boundary
//	bottom      the position IS the bottom boundary
//	page_down   the position INCREASED, or was already at the bottom
//	page_up     the position DECREASED, or was already at the top
//
// The two already-at-the-boundary cases are successes rather than failures, and that is not
// leniency: a person at the bottom of a document who asks to go down one page has been given
// exactly what they asked for, and reporting a failure would invite them to ask again.
//
// A position nobody could read verifies NOTHING. It does not fail either — the call may well have
// landed — so it is inconclusive, and it carries delivery evidence so the retry guard will not
// repeat it. Repeating a delivered page scroll moves two pages.

func scrollWorld(at time.Time, percent string) directorapi.WorldState {
	attrs := map[string]any{"native_id": "42.9", "patterns": "scroll"}
	if percent != "" {
		attrs[directorapi.StateScrollPercent] = percent
	}
	return directorapi.WorldState{
		Timestamp: at,
		Elements: map[directorapi.ElementID]*directorapi.Element{
			"e9": {
				ID: "e9", Label: "Text editor", Role: directorapi.RoleTextField,
				WindowID: "hwnd:1", Enabled: true, Visible: true, Attributes: attrs,
			},
		},
		Windows: []directorapi.Window{{
			ID: "hwnd:1", Application: "notepad", Focused: true,
		}},
	}
}

func scrollAction(where directorapi.ScrollWhere) directorapi.SemanticAction {
	return directorapi.SemanticAction{
		Kind: directorapi.SemanticScroll, Scroll: where,
		Target: directorapi.ElementReference{ID: "e9"},
	}
}

func verifyScrollCase(t *testing.T, where directorapi.ScrollWhere, before, after string) directorapi.VerificationResult {
	t.Helper()
	return New().Verify(scrollAction(where),
		directorapi.ResolvedTarget{ElementID: "e9", Label: "Text editor"},
		scrollWorld(t0, before), scrollWorld(t0.Add(time.Second), after))
}

func TestScrollVerificationAsksWhereTheViewEndedUp(t *testing.T) {
	for _, c := range []struct {
		name          string
		where         directorapi.ScrollWhere
		before, after string
		want          bool
	}{
		{"to the top, arrived", directorapi.ScrollTop, "80", "0", true},
		{"to the top, did not arrive", directorapi.ScrollTop, "80", "40", false},
		{"to the bottom, arrived", directorapi.ScrollBottom, "10", "100", true},
		{"to the bottom, did not arrive", directorapi.ScrollBottom, "10", "60", false},

		{"down a page, moved", directorapi.ScrollPageDown, "0", "45", true},
		{"down a page, did not move", directorapi.ScrollPageDown, "40", "40", false},
		{"down a page, went the wrong way", directorapi.ScrollPageDown, "40", "10", false},
		{"down a page, already at the bottom", directorapi.ScrollPageDown, "100", "100", true},

		{"up a page, moved", directorapi.ScrollPageUp, "60", "15", true},
		{"up a page, did not move", directorapi.ScrollPageUp, "60", "60", false},
		{"up a page, went the wrong way", directorapi.ScrollPageUp, "60", "90", false},
		{"up a page, already at the top", directorapi.ScrollPageUp, "0", "0", true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := verifyScrollCase(t, c.where, c.before, c.after)
			if got.Success != c.want {
				t.Errorf("%s from %s%% to %s%% verified=%v, want %v\n  said: %s",
					c.where, c.before, c.after, got.Success, c.want, got.Reason)
			}
		})
	}
}

// TestAnAlreadyAtTheBoundaryScrollSaysSo — the sentence has to tell a person why nothing moved,
// or "verified" over an unchanged view reads as a bug.
func TestAnAlreadyAtTheBoundaryScrollSaysSo(t *testing.T) {
	got := verifyScrollCase(t, directorapi.ScrollPageDown, "100", "100")
	if !strings.Contains(strings.ToLower(got.Reason), "already") {
		t.Errorf("the reason does not explain that the view was already there: %q", got.Reason)
	}
}

// TestAnUnreadablePositionVerifiesNothingAndIsNotRetried is the delivery-is-not-proof half.
//
// A region whose position the provider does not report cannot confirm or deny the scroll. The
// honest answer is inconclusive — and it must carry delivery evidence, because the retry guard
// repeats an unconfirmed action unless something says it already arrived, and repeating a page
// scroll moves two pages.
func TestAnUnreadablePositionVerifiesNothingAndIsNotRetried(t *testing.T) {
	got := verifyScrollCase(t, directorapi.ScrollPageDown, "", "")

	if got.Success {
		t.Fatal("a scroll was verified against a position nobody could read")
	}
	if !got.Inconclusive {
		t.Errorf("an unreadable position came back as a failure rather than as inconclusive: %q",
			got.Reason)
	}
	if !Delivered(got) {
		t.Errorf("no delivery evidence, so the retry guard would scroll again and move two "+
			"pages: evidence=%s", evidenceKinds(got))
	}
}

// TestAScrollWithNoDestinationVerifiesNothing — the four meanings are the whole set, and a
// verdict for a fifth would be invented.
func TestAScrollWithNoDestinationVerifiesNothing(t *testing.T) {
	got := verifyScrollCase(t, directorapi.ScrollWhere("sideways"), "0", "50")
	if got.Success {
		t.Errorf("a destination Marco does not implement was verified: %q", got.Reason)
	}
}
