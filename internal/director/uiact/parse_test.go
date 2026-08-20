package uiact_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The planner's half of the milestone:
//
//	Planner prefers semantic actions over raw clicks.
//
// What these check is that the phrases a person actually says land on the right VERB,
// and — just as important — that the phrases belonging to other parsers are declined.

func TestOrdinaryRequestsBecomeSemanticActions(t *testing.T) {
	cases := []struct {
		phrase string
		kind   directorapi.SemanticActionKind
		target string
	}{
		{"expand the tree", directorapi.SemanticExpand, "tree"},
		{"expand Downloads", directorapi.SemanticExpand, "Downloads"},
		{"collapse that section", directorapi.SemanticCollapse, "section"},
		{"toggle the sidebar", directorapi.SemanticToggle, "sidebar"},
		{"check the Remember me box", directorapi.SemanticCheck, "Remember me box"},
		{"uncheck notifications", directorapi.SemanticUncheck, "notifications"},
		{"open Downloads", directorapi.SemanticOpen, "Downloads"},
		{"press File", directorapi.SemanticInvoke, "File"},
		{"activate the Save button", directorapi.SemanticInvoke, "Save button"},
		{"close the dialog", directorapi.SemanticClose, "dialog"},
		{"deselect the row", directorapi.SemanticDeselect, "row"},
		{"scroll to the last message", directorapi.SemanticScrollHere, "last message"},
		{"pin the tab", directorapi.SemanticPin, "tab"},

		// Whole-phrase requests, which name no control at all.
		{"refresh", directorapi.SemanticRefresh, ""},
		{"go back", directorapi.SemanticBack, ""},
		{"go forward", directorapi.SemanticForward, ""},
		{"dismiss", directorapi.SemanticDismiss, ""},
		{"submit", directorapi.SemanticSubmit, ""},
		{"maximize the window", directorapi.SemanticMaximize, ""},
		{"minimize", directorapi.SemanticMinimize, ""},
	}
	for _, c := range cases {
		t.Run(c.phrase, func(t *testing.T) {
			req, ok := uiact.Parse(c.phrase)
			if !ok {
				t.Fatalf("%q was not recognised as a semantic request", c.phrase)
			}
			if req.Kind != c.kind {
				t.Errorf("kind = %s, want %s", req.Kind, c.kind)
			}
			if req.Target != c.target {
				t.Errorf("target = %q, want %q", req.Target, c.target)
			}
		})
	}
}

// TestTheUsersCapitalisationSurvives: a label is the user's data.
//
// Matching happens in lower case for a simpler grammar, but handing "file" to the
// resolver instead of "File" would make every match depend on case-insensitivity that a
// future ranker is free to tighten.
func TestTheUsersCapitalisationSurvives(t *testing.T) {
	req, ok := uiact.Parse("press File")
	if !ok {
		t.Fatal("not recognised")
	}
	if req.Target != "File" {
		t.Errorf("target = %q, want %q — the phrase is lower-cased for matching, not for "+
			"the label it produces", req.Target, "File")
	}
}

func TestAnOrdinalTurnsASelectIntoAChoose(t *testing.T) {
	req, ok := uiact.Parse("select the second result")
	if !ok {
		t.Fatal("not recognised")
	}
	if req.Kind != directorapi.SemanticChoose {
		t.Errorf("kind = %s, want choose — an ordinal names a position among matches, "+
			"not a control called \"second result\"", req.Kind)
	}
	if req.Ordinal != 2 {
		t.Errorf("ordinal = %d, want 2", req.Ordinal)
	}
	if req.Target != "result" {
		t.Errorf("target = %q, want %q", req.Target, "result")
	}
}

// TestPointingIsNotALabel — "select this" must not search for a control called "this".
func TestPointingIsNotALabel(t *testing.T) {
	for _, phrase := range []string{"select this", "expand that", "toggle it"} {
		req, ok := uiact.Parse(phrase)
		if !ok {
			t.Fatalf("%q was not recognised", phrase)
		}
		if req.Target != "" {
			t.Errorf("%q: target = %q, want empty — the user pointed rather than named",
				phrase, req.Target)
		}
		if !req.Deictic {
			t.Errorf("%q: not marked deictic, so the resolver has nothing to go on", phrase)
		}
	}
}

// TestPhrasesBelongingToOtherParsersAreDeclined is the conservatism rule.
//
// The editing parser owns "undo", "select all", "copy" and "paste" and implements them
// against a control's value API rather than as chords. Claiming them here would trade a
// verified text-state change for a keystroke, so this parser must decline them — the
// intent layer runs editing first, and a phrase this one grabbed would never get there.
func TestPhrasesBelongingToOtherParsersAreDeclined(t *testing.T) {
	for _, phrase := range []string{
		// Not verbs at all.
		"frobnicate the thing", "what is on screen", "",
		// A verb with nothing to act on.
		"expand", "collapse", "toggle",
		// Reads like a verb, is not a request to act on a control.
		"expand on that point",
	} {
		if req, ok := uiact.Parse(phrase); ok {
			t.Errorf("%q was claimed as %s; it should fall through to the ordinary planner",
				phrase, req.Kind)
		}
	}
}

func TestALiteralClickIsNotClaimed(t *testing.T) {
	// "Click X" names the GESTURE. A user who says click means click, and turning it
	// into an invoke would override an explicit instruction.
	if req, ok := uiact.Parse("click Save"); ok {
		t.Errorf("\"click Save\" was claimed as %s; a literal click request must stay a click",
			req.Kind)
	}
}

func TestTrailingPunctuationFromSpeechIsIgnored(t *testing.T) {
	req, ok := uiact.Parse("expand the tree.")
	if !ok || req.Target != "tree" {
		t.Errorf("got (%+v, %v), want the trailing stop to be dropped", req, ok)
	}
}
