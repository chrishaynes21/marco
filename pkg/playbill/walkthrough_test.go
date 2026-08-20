package playbill_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// TestWalkthrough prints every teaching state as a person would see it. `go test -run Walkthrough -v`.
func TestWalkthrough(t *testing.T) {
	states := []struct {
		what string
		tv   playbill.Teaching
	}{
		{"Ready (idle)", playbill.Teaching{}},
		{"Establishing", playbill.Teaching{Active: true, Asked: "open target",
			Because:  "Hold still a moment while I learn where we're starting.",
			Progress: steps(0)}},
		{"SHOW ME", playbill.Teaching{Active: true, Asked: "open target", Armed: true,
			Because: "Okay — go ahead and show me.", Progress: steps(1)}},
		{"Watching, action seen", playbill.Teaching{Active: true, Asked: "open target",
			Armed: true, Did: []string{"down", "confirm"}, Progress: steps(1)}},
		{"Action unknown", playbill.Teaching{Active: true, Asked: "open target",
			Unattributed: true, Examples: 1, Progress: steps(2),
			Because: "I saw where you ended up, but I couldn't tell what you did."}},
		{"Another example", playbill.Teaching{Active: true, Asked: "open target",
			Examples: 1, Did: []string{"down", "confirm"}, Progress: steps(2),
			Because: "I'd like to see that once more, from where we started."}},
		{"Ready to try", playbill.Teaching{Active: true, Waiting: true, Asked: "open target", Examples: 2,
			Progress: steps(3), Because: "I think I understand. Want me to try it once?"}},
		{"Rehearsing", playbill.Teaching{Active: true, Asked: "open target", Examples: 2,
			Progress: steps(3), Because: "Trying it."}},
		{"Naming", playbill.Teaching{Active: true, Waiting: true, Asked: "open target", Examples: 2,
			Progress: steps(4), Because: "What do you call this screen?"}},
		{"Learned", playbill.Teaching{Asked: "open target", Learned: "open target",
			Examples: 2, Progress: steps(5)}},
		{"Refused", playbill.Teaching{Asked: "open target", Stopped: true, Unattributed: true,
			Because: "I saw where you ended up, but I couldn't tell what you did."}},
		{"Cancelled", playbill.Teaching{Asked: "open target", Stopped: true,
			Because: "Stopped. I haven't kept anything from that."}},
	}
	for _, s := range states {
		v := playbill.View{Reach: playbill.Present, Teaching: s.tv}.Normalise()
		if err := v.Admit(); err != nil {
			t.Errorf("%s was refused by the guard: %v", s.what, err)
			continue
		}
		h := v.Normal()
		attention := ""
		if h.Attention {
			attention = "   [!]"
		}
		t.Logf("\n== %s ==\nNORMAL:  %s - %s%s\nWATCH:",
			s.what, h.Word, h.Detail, attention)
		var seen bool
		for _, l := range v.Watch() {
			if l.Text == "TEACHING" {
				seen = true
			}
			if seen {
				t.Logf("  %s%s\n", strings.Repeat("  ", l.Indent), l.Text)
			}
		}
		if !seen {
			t.Log("  (no teaching section)")
		}
	}
}

func steps(current int) []playbill.Step {
	names := []string{"Starting place", "Show me", "Another example", "Try it",
		"Name screens", "Save"}
	out := make([]playbill.Step, 0, len(names))
	for i, n := range names {
		st := playbill.StepPending
		switch {
		case i < current:
			st = playbill.StepDone
		case i == current:
			st = playbill.StepCurrent
		}
		out = append(out, playbill.Step{Name: n, State: st})
	}
	return out
}
