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
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
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
