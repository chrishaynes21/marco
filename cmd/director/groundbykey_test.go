package main

// NAMING THE SCREEN YOU ARE ON, WITH THE KEYS YOU ALREADY HOLD.
//
// # The condition
//
// A run has to know where it is standing before it can know what its next step is, and
// [[ADR-186-a-person-can-say-where-they-are]] settled that the correspondence between a saved
// screen and a live one is a person's to make: no measure can say whether the right answer is in
// the list. The typed path asks by printing the candidates and reading a label off standard
// input.
//
// `--hotkeys` exists because typing an answer takes the foreground away from the application,
// and the press the answer permits is then refused because the target no longer leads the
// desktop. So in hotkey mode `silent` was true and nobody could be asked at all: every first
// observation refused with *name the screen with --from and start again*.
//
// That is an answer for a person at a terminal in another window. It is not an answer for a
// person at a WIDGET, which is the only way the product can be used: there is no terminal to name
// a screen in, so a typed routine name reached a question nobody could answer and the run could
// not take its first action however correctly everything else behaved.
//
// # What these drive
//
// The production grounder, the production consent desk over a synthetic key source, and — for
// the one that can be reached without stealing anybody's focus — the production command, all the
// way to an authorised action.
//
// NOTHING HERE SENDS REAL INPUT. The command runs without `--live`, so the authorised action is
// compiled and written down by `recordhost` instead of reaching a mouse. That is deliberate and
// not a shortcut: a test that pressed F7 for real against whatever window happens to be in front
// of the person running it would be driving somebody's desktop.

import (
	"context"
	"fmt"
	"image"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// aDeskOverSyntheticKeys is the production desk and granter, with a hook a test can press.
func aDeskOverSyntheticKeys(t *testing.T, out io.Writer) (*hotkeyGranter, func(uint16, bool)) {
	t.Helper()
	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	desk := newConsentDesk(src.Consents(), func() {})
	go desk.run(ctx)
	return &hotkeyGranter{desk: desk, out: out, live: false,
		within: 2 * time.Second, life: 10 * time.Second}, press
}

// aScreenToName is an artifact with two saved screens, so the ranking has something to rank and
// the candidate offered is a choice rather than the only thing there is.
func aScreenToName() workflowstore.Artifact {
	a := aNamedDemonstration("the routine")
	a.Screens = []workflowstore.Screen{
		{Label: "screen_a", Arrangement: observe.VisualArrangement{
			Cells: map[string]int{"0,0": 4, "1,1": 2}, Total: 6}},
		{Label: "screen_b", Arrangement: observe.VisualArrangement{
			Cells: map[string]int{"2,2": 9}, Total: 9}},
	}
	return a
}

// TestAHotkeyRunEstablishesItsScreenByKey.
//
// The four answers, through the production grounder and the production desk. A person pressing
// F7 is the whole of what used to be missing, and the other three matter as much: a no must
// decline the screen rather than settle for it, an unanswered question must not establish
// anything, and a stop must be a stop.
func TestAHotkeyRunEstablishesItsScreenByKey(t *testing.T) {
	for _, tc := range []struct {
		name      string
		key       uint16
		press     bool
		establish bool
		says      string
	}{
		{"yes, that is the screen", navsource.KeyF7, true, true, ""},
		{"no, it is not", navsource.KeyF8, true, false, "is not screen_a"},
		{"nobody answered", 0, false, false, "nobody said whether"},
		{"stopped while being asked", navsource.KeyF9, true, false, "was stopped"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keys, press := aDeskOverSyntheticKeys(t, os.Stdout)
			art := aScreenToName()
			// THE PRODUCTION GROUNDER, SILENT, WITH THE KEYS WIRED TO IT.
			// `in` is deliberately nil: a run that reached for standard
			// input here would be the defect, not the fix.
			ground := &promptGrounder{art: art, out: os.Stdout, silent: true,
				declined:    map[observe.VisualScreenID]uint64{},
				byKey:       keys.confirmScreen,
				arrangement: art.Screens[0].Arrangement}
			s := workflowstore.NewSessionBinding()
			at := workflowrun.Sighting{Screen: "visual_1", Epoch: 3,
				Generation: 2, Front: true, At: time.Now()}

			if tc.press {
				go func() {
					// AFTER THE QUESTION IS OPEN. A key found in the
					// desk's drain answers nothing, which is correct
					// and is not what is being tested here.
					time.Sleep(120 * time.Millisecond)
					press(tc.key, true)
					press(tc.key, false)
				}()
			}
			err := ground.Establish(context.Background(), s, at)

			_, established := s.Established("screen_a", at.Epoch, at.Generation)
			if established != tc.establish {
				t.Fatalf("established=%v, want %v (err: %v)",
					established, tc.establish, err)
			}
			if tc.establish {
				if err != nil {
					t.Fatalf("a person said yes and the screen was "+
						"not established: %v", err)
				}
				if keys.placed != 1 || keys.screens != 1 {
					t.Errorf("the report counts %d of %d screens named "+
						"by key", keys.placed, keys.screens)
				}
				return
			}
			if err == nil {
				t.Fatal("nothing was established and nothing was reported")
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("the refusal is %q, and it should say %q",
					err, tc.says)
			}
			// AND THE SCREEN IS DECLINED, so the same question is not asked
			// again in the same observation — which on the typed path is what
			// `none` does.
			if ground.declined[at.Screen] != at.Epoch {
				t.Error("the screen was not declined, so the run will ask " +
					"about it again on every look")
			}
		})
	}
}

// AND THE CANDIDATE OFFERED IS THE ONE THE ARRANGEMENT RANKS FIRST, with its score.
//
// Not the first saved screen, which would be a silent choice wearing a question's clothes, and
// not a screen nobody can see: the number is printed because a person about to let Marco act
// should be told how sure the arrangement is before they agree with it.
func TestTheScreenOfferedByKeyIsTheRankedCandidateWithItsScore(t *testing.T) {
	asked := &liveOut{}
	keys, press := aDeskOverSyntheticKeys(t, asked)
	art := aScreenToName()
	ground := &promptGrounder{art: art, out: os.Stdout, silent: true,
		declined: map[observe.VisualScreenID]uint64{}, byKey: keys.confirmScreen,
		// THE LOOK AGREES WITH THE SECOND SAVED SCREEN, which is not the first
		// one in the artifact.
		arrangement: art.Screens[1].Arrangement}
	s := workflowstore.NewSessionBinding()
	at := workflowrun.Sighting{Screen: "visual_1", Epoch: 3, Generation: 2,
		Front: true, At: time.Now()}

	go func() {
		time.Sleep(120 * time.Millisecond)
		press(navsource.KeyF7, true)
		press(navsource.KeyF7, false)
	}()
	if err := ground.Establish(context.Background(), s, at); err != nil {
		t.Errorf("establishing: %v", err)
	}
	if _, ok := s.Established("screen_b", at.Epoch, at.Generation); !ok {
		t.Error("the screen the arrangement ranked first was not the one offered")
	}
	said := asked.String()
	if !strings.Contains(said, "screen_b") || !strings.Contains(said, "agrees 1.000") {
		t.Errorf("the question did not name the candidate and its score:\n%s", said)
	}
	// AND IT IS A QUESTION, not an announcement: the keys are on the line, which is the
	// only thing that makes it answerable from a widget.
	if !strings.Contains(said, navsource.ConsentKeys[navsource.ConsentApprove]+" yes") {
		t.Errorf("the question does not say which key agrees with it:\n%s", said)
	}
}

// AND THE WHOLE RUN REACHES AN AUTHORISED ACTION WITH NO `--from` AT ALL.
//
// # Why this one is worth its weight
//
// Everything above is the grounder answering a question. This is the claim the architect's
// finding was actually about: *a demonstrated name cannot reach an actionable step* — and no
// amount of unit coverage of the grounder can establish that it now can, because the blocker was
// in the sequence rather than in any one part.
//
// So the production command runs, start to action: the start key, the screen named by key, the
// step agreed to by key, the press authorised by key, and the program the authorisation produced
// written down.
//
// # Why the FOREGROUND window, and why nothing is pressed
//
// Nothing can be grounded on a window that does not lead the desktop — `Sighting.Usable` refuses
// a look that may be pixels of whatever is covering it — so a run against some other application
// cannot reach grounding at all, and bringing a window forward from a test is stealing somebody's
// focus. The window in front is the one window a test may use without touching anything.
//
// And the routine is built from a REAL LOOK at that window, so its control can be re-acquired:
// the saved readings are regions the production proposer found in these pixels, which is what a
// demonstration's readings are.
//
// There is no `--live`. The authorised action is compiled and written down rather than pressed,
// which is what makes it safe to answer F7 at all here: a real press would go to whatever the
// person running the suite has in front of them.
func TestAHotkeyRunReachesAnAuthorisedActionWithoutFrom(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	windows := winprovider.New()
	front := theForegroundWindow(t, windows)
	art := aRoutineFromALookAt(t, windows, front)
	store := mustOpenStore(t)
	if err := store.SaveSequence(art); err != nil {
		t.Fatal(err)
	}

	// THE SYNTHETIC HOOK, in place of the desktop's.
	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	was := newKeySource
	t.Cleanup(func() { newKeySource = was })
	newKeySource = func() (*navsource.Source, string) { return src, "" }

	// AND SOMEBODY ANSWERING EVERY QUESTION AS IT APPEARS.
	out := &liveOut{}
	stop := answerEveryQuestion(t, out, press)
	defer stop()

	// AND WATCH THE WINDOW FOR THE WHOLE OF THE RUN. See watchWindow: a sample at
	// each end misses a window that moved in the middle and came back, which is what
	// a game HUD does — and that is the case this test kept failing on.
	moved := watchWindow(t, windows, front, art)

	code := captureStdoutInto(t, out, func() int {
		return runPerformWorkflow([]string{
			"--foreground", "--name", art.Name, "--hotkeys",
			"--delay", "0s", "--seconds", "40", "--answer", "8s",
			"--patience", "40",
			// ── NO DWELL, BECAUSE THIS RUN IS DRY ──
			//
			// The dwell rests the pointer on the control and then
			// CHECKS that the operating system actually put it there
			// (ADR-198: a move that reported completed and did not
			// happen). In a dry run the move is written down instead of
			// performed, so the pointer never arrives and that check
			// refuses the click — correctly, and for a reason that has
			// nothing to do with grounding. The alternative is to send
			// real input from a test, against whatever window the
			// person running the suite has in front of them.
			"--dwell", "0s",
		})
	})
	said := out.String()

	// ── NOTHING BELOW IS ASKABLE OF A WINDOW THAT MOVED ──
	//
	// This guarded only the last assertion at first, and the next failure was the one
	// before it: the run established its screen and then spent its whole patience
	// without reaching an action, which is the same cause one step earlier. Every
	// assertion here presumes the window the routine was built from is still the window
	// in front, so the question is asked once, for all of them.
	//
	// What that gives up is covered without a desktop:
	// `TestAHotkeyRunEstablishesItsScreenByKey` holds the grounding, and
	// `TestOneAuthorizedStepIsLoweredToARecordingHost` holds an authorised step reaching
	// the host. What is left here, and what the skip costs, is the two of them joined.
	if moved() {
		t.Skipf("%q %v changed while the run was in flight, so nothing this test "+
			"asserts can be asked of it:\n%s", front.Title, front.Bounds, said)
	}

	// 1. THE ONE THING THAT USED TO HAPPEN MUST NOT HAPPEN.
	if strings.Contains(said, "name the screen with --from") {
		t.Fatalf("the run still refuses for want of --from, which a widget has no "+
			"way to supply:\n%s", said)
	}
	// 2. THE SCREEN WAS NAMED BY A PERSON, BY KEY.
	if !strings.Contains(said, "Is that the screen you are on?") {
		t.Fatalf("nobody was asked which screen this is:\n%s", said)
	}
	if !strings.Contains(said, "established by key") {
		t.Fatalf("the screen was never established:\n%s", said)
	}
	// 3. AND IT REACHED AN ACTION AND ASKED ABOUT IT.
	if !strings.Contains(said, "a left click there, once?") {
		t.Fatalf("the run never reached an action to ask about, so a demonstrated "+
			"name still cannot reach an actionable step:\n%s", said)
	}
	// 4. AND THE AUTHORISED ACTION HAPPENED — written down, not pressed.
	rec := theOnlyRecord(t)
	if rec.Sent == 0 {
		// AND WHICH WINDOW IT WAS LOOKING AT WHEN IT DID NOT.
		//
		// This assertion's verdict is a function of the desktop it ran on. The
		// routine above is built from a photograph of the window in front, and the
		// run looks AGAIN at the press instant; a refusal here is usually the world
		// re-check declining a look whose pixels no longer agree with the screen the
		// point was approved on. That is the guard working, on a window that did not
		// hold still — and it is indistinguishable, from this line, from a guard
		// that has started refusing a screen that did.
		//
		// It has failed that way: four times in ten full-package runs, in one early
		// cluster, with no mechanism established. See
		// docs/experiments/Experiment-094-four-failures-no-mechanism-and-a-window-that-has-to-hold-still.md
		// — which had to be reconstructed from scrollback, because nothing recorded
		// which window the run was grounded against. Now it does.
		//
		// AND THE TWO ARE NO LONGER INDISTINGUISHABLE. `windowMovedDuring` above
		// asks the pixels: reaching this line means the window still agrees with
		// the screen the point was approved on, so a refusal here is a refusal
		// about a window that DID hold still, which is the thing worth failing on.
		t.Errorf("no action was performed after one was authorised (%s: %s)\n"+
			"  grounded against %s — %q %v, which is the window in front of "+
			"whoever ran this; see Experiment-094 if the reason above is about "+
			"the screen the point was approved on",
			rec.Outcome, rec.Because, front.Application, front.Title, front.Bounds)
	}
	if rec.Real {
		t.Fatal("the run sent REAL input; this test must never do that")
	}
	if code == 2 {
		t.Errorf("the command refused the invocation itself:\n%s", said)
	}
}

// theForegroundWindow is the window in front, which is the only one a test may ground against.
func theForegroundWindow(t *testing.T, windows *winprovider.Provider) directorapi.Window {
	t.Helper()
	for _, c := range windows.AllCandidates(context.Background()) {
		if !c.Foreground || c.Minimized || !c.OnScreen || c.Application == "" {
			continue
		}
		if c.Bounds.Width <= 0 || c.Bounds.Height <= 0 {
			continue
		}
		return directorapi.Window{ID: c.ID, Application: c.Application,
			Title: c.Title, Bounds: c.Bounds}
	}
	t.Skip("no foreground window could be read, so nothing can be grounded without " +
		"taking somebody's focus")
	return directorapi.Window{}
}

// aRoutineFromALookAt builds a demonstration out of what the production proposer finds in a real
// window, scoped to that window's application.
//
// This is what `demonstrate-visual` saves, made without hands: the screen's arrangement and the
// control's readings both come from one look at the pixels that are actually there, so the
// control can be re-acquired on the next look and the run has a step it can reach.
func aRoutineFromALookAt(t *testing.T, windows *winprovider.Provider,
	w directorapi.Window) workflowstore.Artifact {

	t.Helper()

	// ── THREE LOOKS, AND THE WINDOW HAS TO HOLD STILL ACROSS ALL OF THEM ──
	//
	// Everything this helper does is a precondition about the window in front, stated and
	// skipped on. This was the one it never stated: the routine is built from a
	// photograph and then performed against a look taken SECONDS LATER, and the run
	// compares the two with the terms below. A window that changed in between fails that
	// comparison — correctly — and the caller then reports it as "no action was performed
	// after one was authorised", which reads as the grounder breaking.
	//
	// It is not. [[Experiment-094-four-failures-no-mechanism-and-a-window-that-has-to-hold-still]]
	// measured it twice: four failures in ten full-package runs with no mechanism
	// established, and then 2 of 5 against 3 of 5 at a baseline that predates the work
	// being tested — with a game in the foreground, which is a window that genuinely does
	// not hold still from one look to the next.
	//
	// Three and not two, because a window can be still between consecutive frames and not
	// still over the seconds a run takes: measured on the owner's machine with Trackmania
	// in front, a single pair agreed at 0.526 against a threshold of 0.55, which is a
	// coin flip and exactly the shape of the flake. Every pair is compared, so a window
	// that flickers and returns cannot pass on its endpoints alone.
	looks := make([][]observe.Region, 0, 3)
	for i := 0; i < 3; i++ {
		if i > 0 {
			time.Sleep(250 * time.Millisecond)
		}
		img, err := newCapture(windows).CaptureWindow(context.Background(), w)
		if err != nil {
			t.Skipf("the window in front could not be photographed: %v", err)
		}
		looks = append(looks, observe.NormaliseRects(img.Image.Bounds(),
			boundsOf(structure.Propose(img.Image,
				structure.DefaultOptions()))))
	}
	for i := range looks {
		for j := i + 1; j < len(looks); j++ {
			agree := observe.ArrangementSimilarity(
				observe.ArrangeLook(looks[i]), observe.ArrangeLook(looks[j]))
			if agree >= observe.VisualScreenMatch {
				continue
			}
			t.Skipf("%q %v is not holding still: looks %d and %d agree at "+
				"%.3f, below the %.2f the run requires, so a routine "+
				"built from one of them cannot be performed against a "+
				"look taken later", w.Title, w.Bounds, i+1, j+1, agree,
				observe.VisualScreenMatch)
		}
	}

	// ── AND THE ROUTINE IS BUILT FROM THE LAST OF THEM ──
	//
	// The last and not the first, because the gap that matters is the one between these
	// pixels and the look the run takes at the instant of the press. Building from the
	// first look put the whole of the stillness check INSIDE that gap, which made this
	// fail more often rather than less — the check is meant to narrow the question, not
	// to widen the distance it is asked over.
	regions := looks[len(looks)-1]
	if len(regions) < 2 {
		t.Skipf("the window in front proposes %d region(s); a routine needs a "+
			"control to re-acquire and an arrangement to settle", len(regions))
	}
	// THE BIGGEST CONTROL-SHAPED REGION, which is not the biggest region.
	//
	// Measured, on a live Ark window: the largest thing the proposer finds spans the full
	// height of the window, and the coordinator refuses it — *the middle of it is the
	// middle of the window and not the position of anything in it; this step has no point
	// to press and no substitute for one will be invented*. That guard is right and this
	// fixture was wrong: a demonstration's control is a control, so the region has to be
	// bounded in both axes the way one is.
	//
	// Among those, the largest, because the largest thing is the likeliest to still be
	// there on the next look — this test is about the sequence, not about how little
	// evidence a reacquisition can work from.
	best := observe.Region{}
	for _, r := range regions {
		if r.Width > 0.5 || r.Height > 0.25 || r.Width <= 0 || r.Height <= 0 {
			continue
		}
		if r.Width*r.Height > best.Width*best.Height {
			best = r
		}
	}
	if best.Width <= 0 {
		t.Skipf("the window in front proposes %d region(s) and none of them is "+
			"control-shaped, so there is nothing a demonstration could have "+
			"been of", len(regions))
	}
	// AND THE CONTROL HAS TO HAVE BEEN THERE THE WHOLE TIME, because the arrangement is
	// a count of rectangles in nine boxes and can agree while the one region this routine
	// presses comes and goes. That is the other refusal this test has produced: *nothing
	// was proposed under the approved point in the look taken at the instant of the
	// press*.
	mid := observe.PointerAt{X: best.X + best.Width/2, Y: best.Y + best.Height/2}
	for i, look := range looks {
		found := false
		for _, r := range look {
			if mid.X >= r.X && mid.X <= r.X+r.Width &&
				mid.Y >= r.Y && mid.Y <= r.Y+r.Height {
				found = true
				break
			}
		}
		if !found {
			t.Skipf("%q %v proposed nothing under the middle of this "+
				"routine's own control on look %d of %d, so the control "+
				"it would press is not reliably there", w.Title, w.Bounds,
				i+1, len(looks))
		}
	}

	return workflowstore.Artifact{
		Version: workflowstore.Version, Name: "the window in front",
		SavedAt:     time.Now().UTC(),
		Application: workflowstore.NormaliseScope(w.Application),
		Screens: []workflowstore.Screen{{Label: "screen_a",
			Arrangement: observe.ArrangeLook(regions)}},
		Controls: []workflowstore.Control{{Label: "control_1",
			Readings: []observe.Region{best}}},
		Steps: []workflowstore.Step{{
			Position: 0, From: "screen_a", To: "screen_a", Control: "control_1",
			Cause: observe.CauseClick, Seen: 1,
		}},
	}
}

// boundsOf is the proposer's rectangles, in the order it found them.
func boundsOf(res structure.Result) []image.Rectangle {
	out := make([]image.Rectangle, 0, len(res.Candidates))
	for _, c := range res.Candidates {
		out = append(out, c.Bounds)
	}
	return out
}

func mustOpenStore(t *testing.T) *demo.Store {
	t.Helper()
	store, err := demo.Open(configDir())
	if err != nil {
		t.Fatalf("opening the store: %v", err)
	}
	return store
}

// liveOut is a transcript a test may read while the command is still writing it.
//
// `captureStdout` returns what was printed AFTER the function finishes, which is right for a
// printer and useless here: the questions this test answers are printed by the thing it is
// answering, so the answers have to be decided from a transcript that is still growing.
type liveOut struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *liveOut) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *liveOut) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// captureStdoutInto runs fn with os.Stdout redirected into a transcript somebody is watching.
func captureStdoutInto(t *testing.T, out *liveOut, fn func() int) int {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 4096)
		for {
			n, rerr := r.Read(buf)
			if n > 0 {
				out.Write(buf[:n])
			}
			if rerr != nil {
				return
			}
		}
	}()
	code := fn()
	os.Stdout = saved
	_ = w.Close()
	<-done
	_ = r.Close()
	return code
}

// answerEveryQuestion presses the approve key for each question as it appears.
//
// # Why it watches the transcript rather than sleeping
//
// Every question in a hotkey run is a line printed and then a wait, and the desk DRAINS pending
// keys before it waits — a key pressed before the question is open answers nothing, which is
// correct and is the stray-press case the desk counts. So this reacts to the line, not to a
// clock.
//
// And it keeps pressing until the run says it heard one. The gap between the line being printed
// and the desk waiting is small and real, so a single press can land in the drain and be lost;
// a question that went unanswered would expire and the run would refuse for a reason that has
// nothing to do with what is being tested.
func answerEveryQuestion(t *testing.T, out *liveOut, press func(uint16, bool)) func() {
	t.Helper()
	done := make(chan struct{})
	go func() {
		started := false
		for {
			select {
			case <-done:
				return
			case <-time.After(60 * time.Millisecond):
			}
			said := out.String()
			if !started && strings.Contains(said, "press "+
				navsource.ConsentKeys[navsource.ConsentApprove]+" to start") {
				started = true
				press(navsource.KeyF7, true)
				press(navsource.KeyF7, false)
				continue
			}
			asked := strings.Count(said,
				navsource.ConsentKeys[navsource.ConsentApprove]+" yes")
			answered := strings.Count(said, "  answered ")
			if asked > answered {
				press(navsource.KeyF7, true)
				press(navsource.KeyF7, false)
			}
		}
	}()
	return func() { close(done) }
}

// watchWindow photographs a window for as long as a run lasts, and reports whether it ever
// stopped being the screen the routine was built from.
//
// # Why this is not a sample at each end
//
// Because the thing it has to catch is a window that moved IN THE MIDDLE. A game HUD holds the
// same count of rectangles in the same nine boxes while a single element flickers, so the
// arrangement before and after can both agree while the look taken at the instant of the press
// did not — and the two refusals this test produced say exactly that: *nothing was proposed
// under the approved point in the look taken at the instant of the press*, and *the screen
// changed since*. Sampling the ends called both of those a still window and failed.
//
// # Why the pixels and not the refusal's wording
//
// Because a run that refused because the world moved and a run whose world re-check has started
// refusing a world that did NOT move produce the same sentence, and the second is the defect
// this test exists to catch. Reading the message would be trusting the thing under test to say
// whether it was right. This asks the window, with the same measure and the same threshold the
// run applies, and the caller fails when the answer is that the window held still.
//
// What it costs is a capture every 400ms for the length of the run, taken beside it rather than
// in it. Nothing here reaches the run, the store, or any input path.
func watchWindow(t *testing.T, windows *winprovider.Provider, w directorapi.Window,
	art workflowstore.Artifact) func() bool {

	t.Helper()
	if len(art.Screens) == 0 {
		return func() bool { return false }
	}
	want := art.Screens[0].Arrangement
	mid := observe.PointerAt{X: 0.5, Y: 0.5}
	if len(art.Controls) > 0 && len(art.Controls[0].Readings) > 0 {
		c := art.Controls[0].Readings[0]
		mid = observe.PointerAt{X: c.X + c.Width/2, Y: c.Y + c.Height/2}
	}
	stop := make(chan struct{})
	done := make(chan bool, 1)
	go func() {
		shots := newCapture(windows)
		diverged, why := false, ""
		for {
			select {
			case <-stop:
				if why != "" {
					t.Log(why)
				}
				done <- diverged
				return
			case <-time.After(400 * time.Millisecond):
			}
			img, err := shots.CaptureWindow(context.Background(), w)
			if err != nil {
				continue
			}
			look := observe.NormaliseRects(img.Image.Bounds(),
				boundsOf(structure.Propose(img.Image,
					structure.DefaultOptions())))
			if agree := observe.ArrangementSimilarity(want,
				observe.ArrangeLook(look)); agree < observe.VisualScreenMatch {
				diverged = true
				why = fmt.Sprintf("during the run, %q agreed with the "+
					"routine's own screen at %.3f, below the %.2f it "+
					"requires", w.Title, agree, observe.VisualScreenMatch)
				continue
			}
			// AND THE CONTROL, because the arrangement is a count of
			// rectangles and can agree while the one region this routine
			// presses is not there.
			found := false
			for _, r := range look {
				if mid.X >= r.X && mid.X <= r.X+r.Width &&
					mid.Y >= r.Y && mid.Y <= r.Y+r.Height {
					found = true
					break
				}
			}
			if !found {
				diverged = true
				why = fmt.Sprintf("during the run, %q proposed nothing "+
					"under the middle of the routine's own control",
					w.Title)
			}
		}
	}()
	// THE ANSWER IS KEPT, so asking twice gives the same answer rather than `false`.
	var once sync.Once
	var moved bool
	return func() bool {
		once.Do(func() { close(stop); moved = <-done })
		return moved
	}
}
