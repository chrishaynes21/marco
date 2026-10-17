package uiact

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// GOING SOMEWHERE IS ONE REQUEST, HOWEVER A PERSON SAYS IT.
//
// Measured live: "open installed apps" resolved the control at once and "show me installed
// apps", "go to installed apps", "take me to installed apps" and "navigate to installed apps"
// all failed BEFORE resolution, with "I only understand click, focus, move, repeat and text
// editing so far" — about a control that was plainly on screen.
//
// So the limit was neither perception nor resolution: one English word opened a destination
// and its ordinary synonyms did not.
func TestADestinationIsReachedHoweverThePersonAsks(t *testing.T) {
	for _, phrase := range []string{
		"open installed apps",
		"show installed apps",
		"show me installed apps",
		"go to installed apps",
		"goto installed apps",
		"jump to installed apps",
		"navigate to installed apps",
		"switch to installed apps",
		"take me to installed apps",
		"bring up installed apps",
	} {
		got, ok := Parse(phrase)
		if !ok {
			t.Errorf("%q was not understood at all", phrase)
			continue
		}
		if got.Kind != directorapi.SemanticOpen {
			t.Errorf("%q read as %q, want open", phrase, got.Kind)
		}
		if got.Target != "installed apps" {
			t.Errorf("%q carried the target %q", phrase, got.Target)
		}
	}
}

// AND THE PHRASES THAT MEANT SOMETHING ELSE STILL DO.
//
// Each of these shares a prefix with one of the destination phrasings above, and each is
// matched by a longer or earlier rule. A regression here would be a request that used to move
// the view now trying to press a control called "the top".
func TestTheDestinationPhrasingsDoNotSwallowWhatTheyResemble(t *testing.T) {
	for _, tc := range []struct {
		phrase string
		want   directorapi.SemanticActionKind
	}{
		{"go to the top", directorapi.SemanticScroll},
		{"go to the bottom", directorapi.SemanticScroll},
		{"jump to the top", directorapi.SemanticScroll},
		{"jump to the bottom", directorapi.SemanticScroll},
		{"show context menu", directorapi.SemanticShowContextMenu},
		{"open up the Documents folder", directorapi.SemanticExpand},
		{"select Alpha.txt", directorapi.SemanticSelect},
		{"close up the folder", directorapi.SemanticCollapse},
	} {
		got, ok := Parse(tc.phrase)
		if !ok {
			t.Errorf("%q was not understood at all", tc.phrase)
			continue
		}
		if got.Kind != tc.want {
			t.Errorf("%q read as %q, want %q", tc.phrase, got.Kind, tc.want)
		}
	}
}
