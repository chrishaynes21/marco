package verify

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// An edit that threw something away says so.
//
// # Measured, on Windows Notepad
//
// A blank document, then four ordinary requests, with Notepad's own status bar as the witness:
//
//	type hello world                                   0 characters -> 11
//	type the quick brown fox jumps over the lazy dog   11 -> 43
//	type ABC                                            43 -> 3
//	type XY                                             3 -> 2
//
// Every one of those is `edit.SetText`, which the planner maps "type", "enter", "write" and "put"
// to — against "add" and "append", which map to `edit.AppendText`. So "type X means the control's
// value becomes X" is a deliberate language decision with a word for the other meaning, and
// changing it is a product choice rather than a defect.
//
// What IS a defect is the sentence Marco said while doing it:
//
//	COMPLETED: the control contains "XY"
//
// True, and silent about the three characters that are now gone. The information was right there:
// verifyEdit already reads `beforeValue` out of the before-world for the append case, and the
// set_text branch ignored it. A person who asks for text and is told the control contains it has
// no way to learn that a document was emptied to make room.
//
// This is reporting, not policy. It changes no vocabulary, adds no confirmation friction to the
// ordinary case of filling in an empty field, and makes the one case that can lose work
// self-announcing on every provider.

func editWorld(at time.Time, id directorapi.ElementID, value string) directorapi.WorldState {
	return directorapi.WorldState{
		Timestamp: at,
		Elements: map[directorapi.ElementID]*directorapi.Element{
			id: {
				ID: id, Label: "Text editor", Role: directorapi.RoleTextField,
				WindowID: "hwnd:1", Enabled: true, Visible: true, Value: value,
			},
		},
		Windows: []directorapi.Window{{
			ID: "hwnd:1", Application: "notepad", Title: "Untitled - Notepad", Focused: true,
		}},
	}
}

func setText(text string) directorapi.EditAction {
	return directorapi.EditAction{
		Operation: "set_text", Text: text,
		Target: directorapi.ElementReference{ID: "e1"},
	}
}

// TestReplacingTextSaysWhatWasReplaced is the defect.
func TestReplacingTextSaysWhatWasReplaced(t *testing.T) {
	before := editWorld(t0, "e1", "the quick brown fox")
	after := editWorld(t0.Add(time.Second), "e1", "ABC")

	got := New().Verify(setText("ABC"),
		directorapi.ResolvedTarget{ElementID: "e1", Label: "Text editor"}, before, after)

	if !got.Success {
		t.Fatalf("a successful replacement did not verify: %s", got.Reason)
	}
	if !strings.Contains(got.Reason, "the quick brown fox") {
		t.Errorf("THE DISCARDED TEXT IS NOT MENTIONED.\n\n  said: %s\n\n"+
			"19 characters were thrown away to make room and the sentence is silent about "+
			"it. On a document that is somebody's work; the before-value was already read "+
			"out of the before-world a few lines up.", got.Reason)
	}
}

// TestFillingAnEmptyControlSaysNothingAboutReplacing is the ordinary case, kept quiet.
//
// Most "type into X" requests go into an empty field and discard nothing. Announcing a
// replacement there would be noise, and worse — it would train people to ignore the one sentence
// that matters.
func TestFillingAnEmptyControlSaysNothingAboutReplacing(t *testing.T) {
	before := editWorld(t0, "e1", "")
	after := editWorld(t0.Add(time.Second), "e1", "hello")

	got := New().Verify(setText("hello"),
		directorapi.ResolvedTarget{ElementID: "e1", Label: "Text editor"}, before, after)

	if !got.Success {
		t.Fatalf("filling an empty control did not verify: %s", got.Reason)
	}
	if strings.Contains(strings.ToLower(got.Reason), "replac") {
		t.Errorf("an empty control was described as having been replaced: %s", got.Reason)
	}
}

// TestReplacingTextWithTheSameTextSaysNothing — rewriting a value with itself discards nothing a
// person would miss, and reporting it as a replacement would be true and useless.
func TestReplacingTextWithTheSameTextSaysNothing(t *testing.T) {
	before := editWorld(t0, "e1", "hello")
	after := editWorld(t0.Add(time.Second), "e1", "hello")

	got := New().Verify(setText("hello"),
		directorapi.ResolvedTarget{ElementID: "e1", Label: "Text editor"}, before, after)

	if strings.Contains(strings.ToLower(got.Reason), "replac") {
		t.Errorf("setting a value to what it already was was reported as a replacement: %s",
			got.Reason)
	}
}

// TestAnUnreadableBeforeValueDoesNotClaimAReplacement holds the three-valued line.
//
// A before-world that did not contain the control cannot tell us whether anything was discarded.
// Saying "nothing was replaced" there would be the same unknown-as-false mistake ADR-006 is
// about, and saying "something was" would be invention. It says neither.
func TestAnUnreadableBeforeValueDoesNotClaimAReplacement(t *testing.T) {
	before := directorapi.WorldState{
		Timestamp: t0,
		Elements:  map[directorapi.ElementID]*directorapi.Element{},
		Windows: []directorapi.Window{{
			ID: "hwnd:1", Application: "notepad", Focused: true,
		}},
	}
	after := editWorld(t0.Add(time.Second), "e1", "ABC")

	got := New().Verify(setText("ABC"),
		directorapi.ResolvedTarget{ElementID: "e1", Label: "Text editor"}, before, after)

	if strings.Contains(strings.ToLower(got.Reason), "replac") {
		t.Errorf("a replacement was claimed with no before-value to compare: %s", got.Reason)
	}
	if !got.Success {
		t.Errorf("the edit itself should still verify on the after-value: %s", got.Reason)
	}
}
