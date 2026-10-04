package verify

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A control that cannot report its value has not reported an empty one.
//
// # Measured, on Character Map
//
// With Advanced view on, the search field is present and enabled:
//
//	text_field "Search for :"   Enabled: true   Patterns: "scroll"   Value: ""
//
// It exposes no ValuePattern, so UIA answers `Value: ""` for it whatever it actually contains.
// `type euro into the Search for field` came back:
//
//	FAILED: the edit could not be verified: the control contains nothing, not "euro"
//
// Two things are wrong with that. The control did not say it was empty — it said nothing, and
// verifyEdit compared the nothing to the intended text and read a mismatch. And `failed` is the
// word that invites a person to type it again, into a field that may already hold the text.
//
// This is [[ADR-006-unknown-is-not-false]] for the fourth time in this codebase, in its fourth
// disguise: an absent reading rounded to a definite one, always in the unsafe direction.
//
// # How the two are told apart
//
// By whether the control exposes a value capability at all, which is a fact the resolution now
// carries — see [directorapi.ControlFacts]. A control that reports a value and reports it empty
// genuinely is empty, and an edit that left it empty genuinely failed. A control with no value
// capability is unreadable, and the honest verdict is that the edit could not be checked.

func editWorldWithPatterns(at time.Time, value, patterns string) directorapi.WorldState {
	return directorapi.WorldState{
		Timestamp: at,
		Elements: map[directorapi.ElementID]*directorapi.Element{
			"e1": {
				ID: "e1", Label: "Search for :", Role: directorapi.RoleTextField,
				WindowID: "hwnd:1", Enabled: true, Visible: true, Value: value,
				Attributes: map[string]any{"patterns": patterns, "native_id": "1"},
			},
		},
		Windows: []directorapi.Window{{
			ID: "hwnd:1", Application: "charmap", Focused: true,
		}},
	}
}

func resolvedSearchField(w directorapi.WorldState) directorapi.ResolvedTarget {
	el := w.Elements["e1"]
	return directorapi.ResolvedTarget{
		ElementID: "e1", Label: el.Label, Role: el.Role,
		Control: directorapi.FactsOf(el),
	}
}

// TestAnEditIntoAnUnreadableControlIsUnverifiedNotFailed is the defect.
func TestAnEditIntoAnUnreadableControlIsUnverifiedNotFailed(t *testing.T) {
	// No value capability: UIA answers "" whatever the field holds.
	before := editWorldWithPatterns(t0, "", "scroll")
	after := editWorldWithPatterns(t0.Add(time.Second), "", "scroll")

	got := New().Verify(setText("euro"), resolvedSearchField(after), before, after)

	if got.Success {
		t.Fatal("an edit was verified against a control that reports no value at all")
	}
	if !got.Inconclusive {
		t.Errorf("A CONTROL THAT CANNOT REPORT ITS VALUE WAS READ AS REPORTING AN EMPTY ONE.\n\n"+
			"  said: %s\n\n"+
			"`failed` here invites somebody to type the text again into a field that may "+
			"already hold it. The control exposes no value capability, so \"\" is an absent "+
			"reading and not a definite one.", got.Reason)
	}
	if strings.Contains(got.Reason, "not \"euro\"") {
		t.Errorf("the reason compares the intended text to a reading that does not exist: %q",
			got.Reason)
	}
}

// TestAnEditIntoAReadableControlThatStayedEmptyStillFails is the other side.
//
// A control that CAN report its value and reports it empty genuinely is empty, and an edit that
// left it that way genuinely failed. Without this, every failed edit would become "could not be
// checked" and the verdict would stop meaning anything.
func TestAnEditIntoAReadableControlThatStayedEmptyStillFails(t *testing.T) {
	before := editWorldWithPatterns(t0, "", "value")
	after := editWorldWithPatterns(t0.Add(time.Second), "", "value")

	got := New().Verify(setText("euro"), resolvedSearchField(after), before, after)

	if got.Success {
		t.Fatal("an edit that left a readable control empty was verified")
	}
	if got.Inconclusive {
		t.Errorf("a control that reports its value and reports it EMPTY was treated as "+
			"unreadable: %q", got.Reason)
	}
}

// TestAnEditThatLandedIsStillVerified keeps the ordinary case.
func TestAnEditThatLandedIsStillVerified(t *testing.T) {
	before := editWorldWithPatterns(t0, "", "value")
	after := editWorldWithPatterns(t0.Add(time.Second), "euro", "value")

	got := New().Verify(setText("euro"), resolvedSearchField(after), before, after)
	if !got.Success {
		t.Errorf("an edit that landed did not verify: %q", got.Reason)
	}
}

// TestAnUnrecordedCapabilityDoesNotExcuseTheEdit holds the line against the fix going too far.
//
// A resolution with no ControlFacts at all says nothing about whether the control can report its
// value. Treating that as "unreadable" would excuse every failed edit on every path that does not
// record the facts, which is the opposite mistake.
func TestAnUnrecordedCapabilityDoesNotExcuseTheEdit(t *testing.T) {
	before := editWorldWithPatterns(t0, "", "value")
	after := editWorldWithPatterns(t0.Add(time.Second), "", "value")

	got := New().Verify(setText("euro"),
		directorapi.ResolvedTarget{ElementID: "e1", Label: "Search for :"}, before, after)

	if got.Inconclusive {
		t.Errorf("a resolution that recorded no capabilities excused a failed edit: %q",
			got.Reason)
	}
}
