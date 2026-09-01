package windowref_test

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// THE WINDOW IN FRONT CAN BE SELECTED, AND NAMING ITS APPLICATION IS A DIFFERENT QUESTION.
//
// # The measured failure
//
// Ambient watching asked the desktop what was in front, threw the answer away, and asked the
// resolver for that window by EXECUTABLE NAME. Windows hosts Settings, XBOX and Realtek Audio
// Console in one `applicationframehost`, so with the audio console open every ambient session over
// Settings resolved as `ambiguous` and skipped every reading — `Samples: 0  skipped: 39` — and
// because starting the session succeeded, nothing reported a failure. A person walked
// Home → Bluetooth & devices → Mouse three times and Marco noticed nothing, silently.
//
// Two windows of one executable is the case that matters, so it is the case this drives.
//
// Deleting the Foreground case from Resolve must fail this.
func TestTheWindowInFrontCanBeSelected(t *testing.T) {
	p := &twoWindowDesktop{}
	dir := windowref.NewDirectory()

	// BY NAME: ambiguous, which is the honest answer to the wrong question.
	_, res, why := windowref.Resolve(t.Context(), p, dir,
		windowref.Selector{Application: "applicationframehost"})
	if res != windowref.AmbiguousSelector {
		t.Fatalf("two windows of one executable resolved %s (%s); the fixture is not the "+
			"case being tested", res, why)
	}

	// BY FOREGROUND: exactly the window the person is using.
	got, res, why := windowref.Resolve(t.Context(), p, dir, windowref.Selector{Foreground: true})
	if res != windowref.Resolved {
		t.Fatalf("the window in front resolved %s (%s)", res, why)
	}
	if got.Title != "Settings" {
		t.Errorf("resolved to %q, want the foreground window. Choosing by executable name "+
			"picks a window the person is not looking at, or refuses entirely.", got.Title)
	}
}

// AND IT IS A PRIMARY SELECTOR, so it cannot be combined with another or left out.
func TestTheForegroundSelectorIsOneOfTheChoices(t *testing.T) {
	if (windowref.Selector{Foreground: true}).Zero() {
		t.Error("a foreground selector reads as nothing selected")
	}
	if err := (windowref.Selector{Foreground: true}).Validate(); err != nil {
		t.Errorf("a foreground selector was refused: %v", err)
	}
	both := windowref.Selector{Foreground: true, Application: "chrome"}
	if err := both.Validate(); err == nil {
		t.Error("a selector naming both the foreground and an application was accepted; " +
			"two primaries is a query, and a query that matched two windows would need a " +
			"tie-break nobody asked for")
	}
	if got := (windowref.Selector{Foreground: true}).Describe(); got == "nothing" {
		t.Error("a foreground selector describes itself as nothing")
	}
}

// twoWindowDesktop is one executable owning two windows, one of them in front — the exact shape
// of a Windows machine with Settings and Realtek Audio Console open.
type twoWindowDesktop struct{}

func (d *twoWindowDesktop) windows() []windowref.Candidate {
	return []windowref.Candidate{
		{ID: "w1", Handle: 1, ProcessID: 31724, Application: "applicationframehost",
			Title: "Settings", Foreground: true, Visible: true, OnScreen: true,
			Bounds: directorapi.Rect{Width: 1200, Height: 800}},
		{ID: "w2", Handle: 2, ProcessID: 31724, Application: "applicationframehost",
			Title: "Realtek Audio Console", Visible: true, OnScreen: true,
			Bounds: directorapi.Rect{Width: 900, Height: 600}},
	}
}

func (d *twoWindowDesktop) Live(_ context.Context, h uintptr) (windowref.Candidate, bool) {
	for _, c := range d.windows() {
		if c.Handle == h {
			return c, true
		}
	}
	return windowref.Candidate{}, false
}

func (d *twoWindowDesktop) ProcessAlive(context.Context, uint32) bool { return true }

func (d *twoWindowDesktop) Candidates(_ context.Context, application string) []windowref.Candidate {
	var out []windowref.Candidate
	for _, c := range d.windows() {
		if application == "" || strings.EqualFold(c.Application, application) {
			out = append(out, c)
		}
	}
	return out
}

// AllCandidates is the whole desktop, which is what `Foreground` reads.
func (d *twoWindowDesktop) AllCandidates(context.Context) []windowref.Candidate {
	return d.windows()
}
