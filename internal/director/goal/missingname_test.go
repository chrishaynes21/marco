package goal

// A REQUEST UNDERSTOOD BUT UNDER-SPECIFIED IS A QUESTION, NOT A REFUSAL.
//
// Measured live. "Make a new folder called Reports" expanded into Explorer's procedure and ran.
// "Make a new folder" answered
//
//	I only understand click, focus, move, repeat and text editing so far
//
// — because no pattern matched, so it was never a goal, so nothing ever asked what to call it.
// Marco had the goal, had the Explorer procedure, and was missing one word.

import "testing"

func TestAFolderRequestWithNoNameIsRecognisedAndAsksForOne(t *testing.T) {
	for _, phrase := range []string{
		"make a new folder", "make a folder", "create a new folder",
		"create a folder", "new folder",
	} {
		g, ok := Parse(phrase)
		if !ok {
			t.Errorf("%q was not recognised as a goal at all", phrase)
			continue
		}
		if g.Kind != CreateFolder {
			t.Errorf("%q read as %q", phrase, g.Kind)
		}
		if name := g.Param(ParamName); name != "" {
			t.Errorf("%q invented the name %q", phrase, name)
		}
		// AND THE PROCEDURE SAYS WHAT IS MISSING, which is what turns this into a
		// question somebody can answer rather than a sentence they cannot act on.
		missing := explorerCreateFolder().Missing(g)
		if len(missing) != 1 || missing[0] != RequiresName {
			t.Errorf("%q: missing = %v, want exactly the name", phrase, missing)
		}
		question := Refusal{Goal: g.Kind, Missing: missing}.Question()
		if question == "" || !containsText(question, "what should it be called") {
			t.Errorf("%q asks %q", phrase, question)
		}
	}
}

// AND THE NAME IS STILL TAKEN WHEN IT IS THERE.
//
// Longest-prefix matching is what keeps both readings: the bare patterns must not swallow the
// named ones, or adding the question would have cost the answer.
func TestTheNamedFormsStillCarryTheirName(t *testing.T) {
	for _, tc := range []struct{ phrase, want string }{
		{"make a new folder called Reports", "Reports"},
		{"create a folder named Q3", "Q3"},
		{"new folder called Archive", "Archive"},
		{"make a folder called My Notes", "My Notes"},
	} {
		g, ok := Parse(tc.phrase)
		if !ok {
			t.Errorf("%q was not recognised", tc.phrase)
			continue
		}
		if got := g.Param(ParamName); got != tc.want {
			t.Errorf("%q named %q, want %q", tc.phrase, got, tc.want)
		}
		if missing := explorerCreateFolder().Missing(g); len(missing) > 0 {
			t.Errorf("%q reports %v missing", tc.phrase, missing)
		}
	}
}

func containsText(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
