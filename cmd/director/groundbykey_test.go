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
		t.Errorf("no action was performed after one was authorised (%s: %s)",
			rec.Outcome, rec.Because)
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
	img, err := newCapture(windows).CaptureWindow(context.Background(), w)
	if err != nil {
		t.Skipf("the window in front could not be photographed: %v", err)
	}
	res := structure.Propose(img.Image, structure.DefaultOptions())
	regions := observe.NormaliseRects(img.Image.Bounds(), boundsOf(res))
	if len(regions) < 2 {
		t.Skipf("the window in front proposes %d region(s); a routine needs a "+
			"control to re-acquire and an arrangement to settle", len(regions))
	}
	// THE BIGGEST REGION AS THE CONTROL, because the largest thing on a screen is the
	// likeliest to still be there on the next look — this test is about the sequence,
	// not about how little evidence a reacquisition can work from.
	best := regions[0]
	for _, r := range regions[1:] {
		if r.Width*r.Height > best.Width*best.Height {
			best = r
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
