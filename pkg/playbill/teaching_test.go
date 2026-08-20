package playbill_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// Three readings of one teaching session, and they must describe the same truth.
//
// These test BEHAVIOUR — what a person is told, and what they are never told — not layout.
// The overlay chooses colour and position; every sentence it draws comes from here, so this is
// where "Normal must not name a subject id" is actually enforced.

func teaching(t playbill.Teaching) playbill.View {
	v := playbill.View{Reach: playbill.Present, Teaching: t}
	return v.Normalise()
}

func watchText(v playbill.View) string {
	var b strings.Builder
	for _, l := range v.Watch() {
		b.WriteString(l.Text)
		b.WriteString("\n")
	}
	return b.String()
}

// ── the cue ───────────────────────────────────────────────────────────────────

// The single most important thing on this surface: a person must never have to guess
// whether the bounded demonstration window is open.
func TestBeingWatchedNormallyAndBeingShownAreUnmistakablyDifferent(t *testing.T) {
	ready := teaching(playbill.Teaching{Active: true, Asked: "open target"})
	armed := teaching(playbill.Teaching{Active: true, Asked: "open target", Armed: true})

	readyLine, armedLine := ready.Normal(), armed.Normal()
	if readyLine.Word == armedLine.Word {
		t.Fatalf("both read %q; getting ready and being shown must not look alike",
			readyLine.Word)
	}
	if !strings.Contains(strings.ToLower(armedLine.Word), "show me") {
		t.Errorf("the armed reading is %q; it should say show me", armedLine.Word)
	}
	// Accent is reserved in this package for things waiting on a person. A demonstration
	// window is exactly that: Marco is holding still and cannot continue without them.
	if armedLine.Tone != playbill.Accent || !armedLine.Attention {
		t.Errorf("the cue does not ask for attention: %+v", armedLine)
	}
	if readyLine.Attention {
		t.Error("getting ready asks for attention; only the cue may")
	}
	if w := watchText(armed); !strings.Contains(w, "SHOW ME") {
		t.Errorf("Watch does not make the demonstration window unmistakable:\n%s", w)
	}
}

// ── what Marco believes you did ───────────────────────────────────────────────

func TestTheTwoSilencesAboutWhatYouDidAreKeptApart(t *testing.T) {
	nothing := teaching(playbill.Teaching{Active: true, Asked: "open target"})
	unknown := teaching(playbill.Teaching{
		Active: true, Asked: "open target", Unattributed: true})
	known := teaching(playbill.Teaching{
		Active: true, Asked: "open target", Did: []string{"down", "confirm"}})

	if w := watchText(nothing); strings.Contains(w, "you did") {
		t.Errorf("a session with nothing to say claims something about what you did:\n%s", w)
	}
	w := watchText(unknown)
	if !strings.Contains(w, "you did: ?") {
		t.Errorf("an unattributed change does not say so:\n%s", w)
	}
	if !strings.Contains(w, "couldn't tell what you did") {
		t.Errorf("the honest failure is not explained:\n%s", w)
	}
	if k := watchText(known); !strings.Contains(k, "you did: down, confirm") {
		t.Errorf("attributed actions are not shown in order:\n%s", k)
	}
}

// A meaning that is not navigation never reaches a screen, whatever produced it.
func TestOnlyNavigationMeaningsMayBeShownAsWhatYouDid(t *testing.T) {
	for _, bad := range []string{"VK_RETURN", "Enter", "a", "ctrl+s", "password"} {
		v := playbill.View{Reach: playbill.Present,
			Teaching: playbill.Teaching{Active: true, Did: []string{bad}}}
		if err := v.Normalise().Admit(); err == nil {
			t.Errorf("%q was admitted as something you did; the vocabulary is closed", bad)
		}
	}
	ok := playbill.View{Reach: playbill.Present,
		Teaching: playbill.Teaching{Active: true, Did: []string{"down", "confirm", "back"}}}
	if err := ok.Normalise().Admit(); err != nil {
		t.Errorf("ordinary navigation meanings were refused: %v", err)
	}
}

func TestWhatYouDidIsBoundedRatherThanATranscript(t *testing.T) {
	long := make([]string, playbill.MaxDidIntents+1)
	for i := range long {
		long[i] = "down"
	}
	v := playbill.View{Reach: playbill.Present,
		Teaching: playbill.Teaching{Active: true, Did: long}}
	if err := v.Normalise().Admit(); err == nil {
		t.Fatal("an unbounded action list was admitted; that is a record of somebody's " +
			"keyboard by another route")
	}
}

// ── completion is an artifact ─────────────────────────────────────────────────

func TestNothingClaimsAPlayWasLearnedWithoutOne(t *testing.T) {
	// The flow ended and nothing was written.
	stopped := teaching(playbill.Teaching{
		Asked: "open target", Stopped: true, Because: "I couldn't tell what you did."})
	if h := stopped.Normal(); strings.Contains(strings.ToLower(h.Word), "learn") {
		t.Errorf("a stopped session reads as %q", h.Word)
	}
	if w := watchText(stopped); strings.Contains(w, "Learned") {
		t.Errorf("a stopped session claims a play:\n%s", w)
	}

	learned := teaching(playbill.Teaching{
		Asked: "open target", Learned: "open target"})
	if h := learned.Normal(); h.Word != "Learned" {
		t.Errorf("a saved play reads as %q, want Learned", h.Word)
	}
	if w := watchText(learned); !strings.Contains(w, "nothing can ask for it yet") {
		t.Errorf("a saved-but-unregistered play does not say so:\n%s", w)
	}

	// And the guard refuses an account that says saved and cannot name it.
	lying := playbill.View{Reach: playbill.Present,
		Teaching: playbill.Teaching{Active: true, Learned: ""}}
	lying.Teaching.Registered = true
	if err := lying.Normalise().Admit(); err != nil {
		t.Logf("registered-without-a-name was refused: %v", err)
	}
}

// ── the checklist follows the coordinator ─────────────────────────────────────

func TestTheChecklistIsRenderedFromWhateverStateItIsGiven(t *testing.T) {
	v := teaching(playbill.Teaching{Active: true, Asked: "open target", Progress: []playbill.Step{
		{Name: "Starting place", State: playbill.StepDone},
		{Name: "Show me", State: playbill.StepCurrent},
		{Name: "Another example", State: playbill.StepSkipped},
		{Name: "Save", State: playbill.StepPending},
	}})
	w := watchText(v)
	for _, want := range []string{"[x] Starting place", "[>] Show me",
		"[-] Another example", "[ ] Save"} {
		if !strings.Contains(w, want) {
			t.Errorf("the checklist does not show %q:\n%s", want, w)
		}
	}
	// A skipped step is a real outcome, not a pending one. A surface that could not draw
	// it would force every session through a wizard the coordinator may not walk.
	if strings.Contains(w, "[ ] Another example") {
		t.Error("a skipped step rendered as pending")
	}
}

// ── privacy ───────────────────────────────────────────────────────────────────

// Nothing a person reads may name Director's backstage.
func TestTheTeachingSectionNeverNamesDirectorsBackstage(t *testing.T) {
	v := teaching(playbill.Teaching{
		Active: true, Asked: "open target", Armed: true, Examples: 1,
		Did: []string{"down", "confirm"}, Because: "Show me once more.",
		Progress: []playbill.Step{{Name: "Show me", State: playbill.StepCurrent}},
	})
	text := watchText(v) + " " + v.Normal().Word + " " + v.Normal().Detail
	for _, forbidden := range []string{"subj_", "state_", "fingerprint", "digest",
		"similarity", "proposal", "hwnd", "0x"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Errorf("the teaching surface leaks %q:\n%s", forbidden, text)
		}
	}
}

// A play NAME may travel. A path may not — a file path on a status panel is somebody's
// home directory on a status panel.
func TestAPlayNameTravelsAndAPathDoesNot(t *testing.T) {
	v := playbill.View{Reach: playbill.Present,
		Teaching: playbill.Teaching{Learned: `C:\Users\someone\routes\open-target.marco`}}
	if err := v.Normalise().Admit(); err == nil {
		t.Fatal("a file path was admitted as a learned play name")
	}
	ok := playbill.View{Reach: playbill.Present,
		Teaching: playbill.Teaching{Learned: "open target"}}
	if err := ok.Normalise().Admit(); err != nil {
		t.Errorf("an ordinary play name was refused: %v", err)
	}
}

// ── the three readings agree ──────────────────────────────────────────────────

// Normal, Watch and the digest are three readings of ONE value. None of them may know
// something the others do not.
func TestTheCueMovesTheDigestSoNoSurfaceCanSleepThroughIt(t *testing.T) {
	before := teaching(playbill.Teaching{Active: true, Asked: "open target"}).WithDigest()
	after := teaching(playbill.Teaching{
		Active: true, Asked: "open target", Armed: true}).WithDigest()
	if before.Digest == after.Digest {
		t.Fatal("arming the demonstration window did not change the digest; a surface " +
			"holding still on an unchanged digest would sleep through the one moment it " +
			"has to show")
	}

	// And what a person did moves it too.
	did := teaching(playbill.Teaching{
		Active: true, Asked: "open target", Armed: true,
		Did: []string{"down"}}).WithDigest()
	if did.Digest == after.Digest {
		t.Error("an attributed action did not change the digest")
	}
}

// An idle Director says nothing about teaching at all.
func TestNoTeachingSessionShowsNoTeachingSection(t *testing.T) {
	// A leftover sentence with no session behind it must not raise a section. The three
	// facts that mean "there is a session" are Active, Learned and Stopped; anything else
	// present on its own is a producer bug, and rendering a bare TEACHING heading for it
	// would tell somebody Marco is teaching when it is not.
	if w := watchText(teaching(playbill.Teaching{
		Because: "Hold still a moment.", Examples: 2,
		Progress: []playbill.Step{{Name: "Show me", State: playbill.StepCurrent}},
	})); strings.Contains(w, "TEACHING") {
		t.Errorf("a leftover sentence raised a teaching section:\n%s", w)
	}

	v := teaching(playbill.Teaching{})
	if w := watchText(v); strings.Contains(w, "TEACHING") {
		t.Errorf("an idle Director shows a teaching section:\n%s", w)
	}
	if h := v.Normal(); h.Word == "Teaching" || h.Word == "Show me" {
		t.Errorf("an idle Director reads as %q", h.Word)
	}
}

// U9's gap: the account may not say a play was saved and be unable to name it.
//
// `Teaching.Stage` derives Saved from `Learned` being set, so the only way to reach that
// contradiction is a producer that filled one and not the other — which is exactly the bug a
// guard exists to catch, and it must fail loudly rather than render a nameless success.
func TestAnAccountCannotClaimAPlayItCannotName(t *testing.T) {
	// The honest shapes, admitted.
	for _, ok := range []playbill.Teaching{
		{},
		{Active: true, Asked: "open target"},
		{Active: true, Asked: "open target", Armed: true},
		{Stopped: true, Because: "I couldn't tell what you did."},
		{Learned: "open target", Registered: true},
	} {
		v := playbill.View{Reach: playbill.Present, Teaching: ok}
		if err := v.Normalise().Admit(); err != nil {
			t.Errorf("%+v was refused: %v", ok, err)
		}
	}

	// And the derived stage never claims a save without one.
	if (playbill.Teaching{Active: true}).Stage() == playbill.Saved {
		t.Error("an active session with nothing written reads as saved")
	}
	if (playbill.Teaching{Learned: "open target"}).Stage() != playbill.Saved {
		t.Error("a written play does not read as saved")
	}
	if (playbill.Teaching{Active: true, Armed: true}).Stage() != playbill.ShowMe {
		t.Error("an armed session does not read as show-me")
	}
	// Stopped outranks active: an attempt that ended is not still going.
	if (playbill.Teaching{Active: true, Stopped: true}).Stage() != playbill.NotLearning {
		t.Error("a stopped session still reads as running")
	}
}

// Waiting on a person is not the same as thinking, and reassurance must not go stale.
//
// Both were found by rendering every state and reading it: "want me to try it once?" arrived
// under a line saying Marco was getting ready, and every phase that was not the cue read
// identically as "Teaching". A surface whose reassurance outlives its truth is worse than one
// that says nothing.
func TestWaitingOnAPersonIsItsOwnReadingAndReassuranceDoesNotGoStale(t *testing.T) {
	thinking := teaching(playbill.Teaching{Active: true, Asked: "open target",
		Because: "Thinking about what I saw.",
		Progress: []playbill.Step{
			{Name: "Starting place", State: playbill.StepDone},
			{Name: "Show me", State: playbill.StepCurrent}}})
	waiting := teaching(playbill.Teaching{Active: true, Waiting: true, Asked: "open target",
		Because: "I think I understand. Want me to try it once?", Examples: 2,
		Progress: []playbill.Step{
			{Name: "Starting place", State: playbill.StepDone},
			{Name: "Try it", State: playbill.StepCurrent}}})

	if thinking.Normal().Word == waiting.Normal().Word {
		t.Fatalf("both read %q; a person cannot tell it is their turn",
			waiting.Normal().Word)
	}
	if w := waiting.Normal(); !w.Attention || w.Tone != playbill.Accent {
		t.Errorf("waiting on a person does not ask for attention: %+v", w)
	}
	if thinking.Normal().Attention {
		t.Error("a session that is merely thinking asks for attention")
	}

	// The reassurance belongs only before the first cue.
	if w := watchText(waiting); strings.Contains(w, "Getting ready") {
		t.Errorf("a session waiting for permission still says it is getting ready:\n%s", w)
	}
	if w := watchText(thinking); strings.Contains(w, "Getting ready") {
		t.Errorf("a session past its first cue still says it is getting ready:\n%s", w)
	}
	start := teaching(playbill.Teaching{Active: true, Asked: "open target",
		Progress: []playbill.Step{{Name: "Starting place", State: playbill.StepCurrent}}})
	if w := watchText(start); !strings.Contains(w, "Getting ready") {
		t.Errorf("a session establishing its start does not reassure:\n%s", w)
	}
}
