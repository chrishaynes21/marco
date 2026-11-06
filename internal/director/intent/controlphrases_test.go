package intent

import "testing"

// What the recogniser is told to expect is what IsControlPhrase answers to.
func TestEveryOfferedControlPhraseStops(t *testing.T) {
	ps := ControlPhrases()
	if len(ps) == 0 {
		t.Fatal("no control phrases")
	}
	for _, p := range ps {
		if !IsControlPhrase(p) {
			t.Errorf("%q is offered and does not stop", p)
		}
	}
	if IsControlPhrase("stop the music") {
		t.Error("the list widened")
	}
}
