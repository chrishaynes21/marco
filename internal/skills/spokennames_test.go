package skills

import (
	"slices"
	"testing"
)

// ONLY AN APPLICATION'S OWN NAME IS SPOKEN (ADR-344). The recogniser is told what each open program
// is called — its executable, and the name its window gives it where the title marks that name as
// the application's — and never a page, a document or a person out of a title.
func TestOnlyAnApplicationsOwnNameIsSpoken(t *testing.T) {
	ws := []Window{
		{Image: "code", Title: "plan.md - marco - Visual Studio Code"},
		{Image: "discord", Title: "#raids | Clan - Discord"},
		{Image: "steam", Title: "Steam"},
		{Image: "destiny2", Title: "Destiny 2"},
		{Image: "signal", Title: "Alex Rivera"},
		{Image: "overlay", Title: "Marco"},
		{Image: "code", Title: "other.go - Visual Studio Code"},
	}
	got := SpokenNames(ws)
	for _, want := range []string{"code", "Visual Studio Code", "discord", "Discord", "steam", "Steam",
		"destiny2", "Destiny 2", "signal"} {
		if !slices.Contains(got, want) {
			t.Errorf("%q is not spoken: %q", want, got)
		}
	}
	for _, never := range []string{"Alex Rivera", "plan.md", "marco", "overlay", "Marco", "#raids",
		"Clan", "other.go"} {
		if slices.Contains(got, never) {
			t.Errorf("%q is spoken, and it is not an application's own name: %q", never, got)
		}
	}
	if n := len(got); n != 9 { // one program, once: the second code window adds nothing
		t.Errorf("spoken = %d names, want 9: %q", n, got)
	}
}

// THE WORDS OF A FOCUS ARE SPOKEN: what begins and names each skill, and the everyday asks.
func TestTheWordsOfASkillAreSpoken(t *testing.T) {
	got := Words()
	for _, want := range []string{"focus", "switch to", "focus on", "search the web", "go back to"} {
		if !slices.Contains(got, want) {
			t.Errorf("%q is not among the words: %q", want, got)
		}
	}
}
