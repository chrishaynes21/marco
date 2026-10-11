package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// WHICH WINDOW THE FRAME WAS ACTUALLY OF.
//
// Experiment-025 captured the Windows taskbar on one look of 107 — a dense row of icons and
// buttons across the bottom of the frame, a text field, eleven equal slivers — and every guard
// in the observation path was satisfied. `target_losses` was 0, the window generation never
// changed, and provenance resolved cleanly, because all three are about window IDENTITY and
// this is not an identity failure: capturing by window can return the pixels of whatever is
// covering the target, under the target's own name.
//
// A frame of somebody else's window folded into a belief about a game teaches a tracker another
// application's layout as this one's. So the question is asked, the answer is carried, and the
// look is refused.

// ── the decision, against a candidate rather than a desktop ───────────────────

// The target's own window, a sibling of the same application, and a different program.
func TestFrontnessTellsTheTargetFromWhateverIsCoveringIt(t *testing.T) {
	target := windowref.Ref{Handle: 4242, Application: "trackmania", ID: "hwnd:4242"}

	for _, tc := range []struct {
		name  string
		front windowref.Candidate
		want  observe.Frontness
	}{{
		name:  "the target itself",
		front: windowref.Candidate{Handle: 4242, Application: "trackmania"},
		want:  observe.FrontTarget,
	}, {
		// A game commonly has a launcher, a splash window and a render window. Refusing
		// those frames would discard real evidence to catch a case this is not for.
		name:  "a sibling window of the same application",
		front: windowref.Candidate{Handle: 99, Application: "Trackmania"},
		want:  observe.FrontTarget,
	}, {
		name:  "the console this protocol prints its banner in",
		front: windowref.Candidate{Handle: 77, Application: "cmd"},
		want:  observe.FrontOther,
	}, {
		// The measured case: Explorer owns the taskbar.
		name:  "the desktop shell, which is what Experiment-025 captured",
		front: windowref.Candidate{Handle: 12, Application: "explorer"},
		want:  observe.FrontOther,
	}, {
		// Not a finding. A desktop the platform could not describe is not evidence that
		// something was covering the target, and reporting it as one would throw away
		// looks for a reason nobody measured.
		name:  "a foreground window that can be neither named nor numbered",
		front: windowref.Candidate{},
		want:  observe.FrontUnknown,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := frontnessOf(tc.front, target); got != tc.want {
				t.Errorf("frontnessOf = %q, want %q", got, tc.want)
			}
		})
	}
}

// And a target nobody has resolved cannot be compared against anything.
func TestFrontnessOfNoTargetIsUnknownRatherThanRefused(t *testing.T) {
	var rt *Runtime
	if got := rt.targetFrontness(context.Background(), windowref.Ref{}); got != observe.FrontUnknown {
		t.Errorf("a nil Director reports frontness %q, want unknown", got)
	}
	// A Director with a platform but no window to compare: still not a finding.
	live := &Runtime{}
	if got := live.targetFrontness(context.Background(), windowref.Ref{}); got != observe.FrontUnknown {
		t.Errorf("frontness against a zero window is %q, want unknown", got)
	}
}

// ── the belief refuses the look ───────────────────────────────────────────────

// A frame taken while another window was in front is not folded into the interface belief.
//
// Deleting the `case sample.Shadow.Front == FrontOther` arm of interfaceLookFrom must fail
// this.
func TestAFrameTakenBehindAnotherWindowIsNotGameEvidence(t *testing.T) {
	// The same nine-row list, with its mark walking the rows, which
	// TestAListWhoseMarkMovesIsIdentifiedFromGeometryAlone establishes IS identified when
	// the frames are the target's. Here every frame was taken behind another window.
	row := -1
	rt := visionShadowRuntime(t, unreadableMenuFixture(), nil,
		movingMarkCapture{generation: 1, row: &row})
	var totals observe.ShadowTotals
	for i, r := range []int{-1, -1, -1, -1, -1, 0, 0, 2, 2, 4, 4, 6, 6, 1, 1, 3, 3, 5, 5, 7} {
		row = r
		s := sampleWithVision(t, rt)
		s.Sequence = i + 1
		s.Timestamp = time.Unix(1760000000, 0).Add(time.Duration(i) * time.Second)
		// What the capture would carry on a desktop where something else is in front.
		s.Shadow.Front = observe.FrontOther
		totals.Observe(s)
	}

	if len(totals.Candidates) != 0 {
		t.Errorf("the tracker learned %d candidate list(s) from frames of another "+
			"window; a belief about a game must not be built out of somebody else's "+
			"layout", len(totals.Candidates))
	}
	if totals.Interface.Status != observe.InterfaceUnknown {
		t.Errorf("the interface belief is %q after twenty frames of another window, "+
			"want unknown", totals.Interface.Status)
	}
	if totals.Interface.Selected >= 0 {
		t.Errorf("entry %d is reported selected from frames that were not of the target",
			totals.Interface.Selected)
	}
	// AND IT SAYS SO, rather than reporting an empty screen. The two are opposite
	// findings: one is about the game, the other about the desktop.
	if !strings.Contains(totals.Interface.Because, "in front") {
		t.Errorf("the belief explains itself as %q, which does not say the frames were "+
			"not of the target", totals.Interface.Because)
	}
}

// The session counts them, so a report can say how much of the run this took.
//
// Deleting the `switch s.Front` block in ShadowTotals.Observe must fail this.
func TestTheSessionCountsLooksTakenBehindAnotherWindow(t *testing.T) {
	var totals observe.ShadowTotals
	fronts := []observe.Frontness{
		observe.FrontTarget, observe.FrontTarget, observe.FrontOther,
		observe.FrontUnknown, observe.FrontTarget, observe.FrontOther,
	}
	for i, f := range fronts {
		totals.Observe(observe.Sample{
			Sequence: i + 1, WindowGeneration: 1,
			Timestamp: time.Unix(1760000000, 0).Add(time.Duration(i) * time.Second),
			Shadow: &observe.ShadowSample{
				Detector: "screenparser", Ran: true, TargetProven: true, Front: f,
			},
		})
	}
	if totals.Behind != 2 {
		t.Errorf("the session counts %d look(s) behind another window, want 2", totals.Behind)
	}
	if totals.FrontUnclear != 1 {
		t.Errorf("the session counts %d unattributable look(s), want 1 — kept apart "+
			"because a question nobody could answer is not an answer",
			totals.FrontUnclear)
	}
}

// The sample carries it, through the sampler production uses.
//
// Deleting `sh.Front = s.rt.targetFrontness(...)` in liveSampler.Sample must fail this.
func TestTheSampleSaysWhichWindowWasInFront(t *testing.T) {
	rt := visionShadowRuntime(t, unreadableMenuFixture(), nil,
		paintedFixtureCapture{generation: 1})
	// The fixture Director has no window platform, so the honest answer is "nobody could
	// say" — and the field must be SET to that rather than left at a zero that happens to
	// mean the same thing by accident. A Frontness that is never assigned looks exactly
	// like one assigned FrontUnknown, so this asserts on the production path's behaviour
	// for a target it CAN resolve as well.
	sample := sampleWithVision(t, rt)
	if sample.Shadow == nil {
		t.Fatal("no shadow record")
	}
	if sample.Shadow.Front != observe.FrontUnknown {
		t.Errorf("frontness is %q on a Director with no window platform, want unknown",
			sample.Shadow.Front)
	}

	// With a platform that puts a different program in front, the sample says so.
	rt.winPlatform = frontPlatform{
		front: windowref.Candidate{Handle: 55, Application: "explorer", ID: "hwnd:55",
			Bounds: directorapi.Rect{Width: 1920, Height: 1080}, Visible: true,
			OnScreen: true, Foreground: true},
	}
	sample = sampleWithVision(t, rt)
	if sample.Shadow.Front != observe.FrontOther {
		t.Errorf("frontness is %q while Explorer is in front, want other",
			sample.Shadow.Front)
	}
}

// The report says it, in the place a person reads.
//
// Deleting the foreground block in renderShadow must fail this.
func TestTheReportSaysHowManyLooksWereBehindAnotherWindow(t *testing.T) {
	var b strings.Builder
	renderShadow(&b, observe.ShadowTotals{
		Detector: "screenparser", Inferences: 107, Behind: 1, FrontUnclear: 3,
	})
	out := b.String()
	if !strings.Contains(out, "foreground ") {
		t.Fatalf("the report never says any frame was of another window:\n%s", out)
	}
	for _, want := range []string{"1 look(s) taken while ANOTHER window was in front",
		"3 more could not be attributed either way"} {
		if !strings.Contains(out, want) {
			t.Errorf("the foreground account is missing %q:\n%s", want, out)
		}
	}
	// A run where every frame was the target's prints nothing: a zero here is the normal
	// case, and a line for it would train a reader to skip the line.
	var clean strings.Builder
	renderShadow(&clean, observe.ShadowTotals{Detector: "screenparser", Inferences: 107})
	if strings.Contains(clean.String(), "foreground ") {
		t.Errorf("a run with no misattributed frame reports a foreground account:\n%s",
			clean.String())
	}
}

// frontPlatform is a desktop with one window in front and nothing else on it.
//
// AllCandidates is the method windowref.Foreground actually scans - it is an OPTIONAL
// interface the platform may implement, and a platform without it resolves no foreground at
// all. The real Windows provider implements it (internal/platform/winprovider), which is what
// makes targetFrontness answer anything in production rather than being inert.
type frontPlatform struct{ front windowref.Candidate }

func (p frontPlatform) Live(_ context.Context, handle uintptr) (windowref.Candidate, bool) {
	if handle == p.front.Handle {
		return p.front, true
	}
	return windowref.Candidate{}, false
}

func (p frontPlatform) ProcessAlive(_ context.Context, _ uint32) bool { return true }

func (p frontPlatform) Candidates(_ context.Context, _ string) []windowref.Candidate {
	return []windowref.Candidate{p.front}
}

func (p frontPlatform) AllCandidates(_ context.Context) []windowref.Candidate {
	return []windowref.Candidate{p.front}
}
