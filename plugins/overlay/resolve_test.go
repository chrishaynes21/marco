package main

// WHAT THE WIDGET DOES WITH DIRECTOR'S ANSWER, THROUGH THE DOOR IT COMES IN BY.
//
// The moment under test is the one that already existed: nothing took the phrase, and the widget
// is about to offer to record a routine. Somebody who typed "make the pictures nicer" may have
// one under another name, and that offer is then the one answer that is certainly wrong.
//
// # Why these go through `dispatchIntake` and not only the helper
//
// Because the defect was in the JOIN. The helper answered a bool and the door read every false as
// permission to offer a recording — so "which of your two routines called settings did you mean?"
// became "shall I record another one", and a Director that could not be asked produced the same
// offer, which is a local fallback by another route in a design whose whole point is that there
// is not one. A test of the helper alone passed throughout.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
)

// answering makes the resolve seam reply with one answer, and puts it back afterwards.
func answering(t *testing.T, r resolved, why string) {
	t.Helper()
	was := askWhichRoutine
	askWhichRoutine = func(string) (resolved, string) { return r, why }
	t.Cleanup(func() { askWhichRoutine = was })
}

// nothingTookIt makes the intake child answer the way it does when no play and no Director
// could take a phrase, and records every child the door starts.
func nothingTookIt(t *testing.T) *[][]string {
	t.Helper()
	var started [][]string
	was := spawnIntakeChild
	spawnIntakeChild = func(h *model, phrase string, track bool, args ...string) childRun {
		started = append(started, args)
		// THE SHAPE `offersLearn` READS: the engine announced no route and the
		// outcome word says nothing took it.
		return childRun{result: string(outcome.Unavailable)}
	}
	t.Cleanup(func() { spawnIntakeChild = was })
	return &started
}

// aRun is Director having named a routine, with every part of the identity.
func aRun() resolved {
	var r resolved
	r.Answer.Kind = "run"
	r.Answer.ID = "arkascended/focus/ark-graphics"
	r.Answer.App, r.Answer.Slug, r.Answer.Focus = "arkascended", "ark-graphics", true
	r.Answer.Name, r.Answer.Scope = "ark graphics", "app-anywhere"
	r.Answer.Reply = "Opening it."
	r.Answer.Context.Says = "read against arkascended, which is in front now"
	r.Answer.Source, r.Answer.Resolver = "advisor", "the configured resolver answered this"
	return r
}

// settled waits for the door's own goroutine to finish with the phrase.
//
// `dispatchIntake` returns immediately on purpose — a replay runs for minutes and a spoken stop
// arrives as a second phrase — so this waits for the prompt the asynchronous half raises rather
// than assuming it has happened.
func settled(t *testing.T, until func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !until() {
		if time.Now().After(deadline) {
			t.Fatal("the door never finished with the phrase")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// AND A ROUTINE DIRECTOR NAMED IS OFFERED INSTEAD OF A RECORDING.
func TestTheDoorOffersTheRoutineDirectorNamed(t *testing.T) {
	h := &model{}
	nothingTookIt(t)
	answering(t, aRun(), "")

	dispatchIntake(h, "make the pictures nicer", invoke.SourceTyped)
	settled(t, anOfferIsPending)

	// THE OFFER IS THE RUN, AND THERE IS NO LEARN OFFER BESIDE IT.
	r := takePendingRoutine()
	if r.id != "arkascended/focus/ark-graphics" {
		t.Fatalf("the pending offer is %+v", r)
	}
	if name := takePendingLearn(); name != "" {
		t.Errorf("a learn offer was raised as well: %q", name)
	}
	// AND THE WHOLE IDENTITY IS WHAT IS PENDING: a name is not unique and neither is a
	// slug, so the three parts an invocation is built from travel together.
	if r.app != "arkascended" || r.slug != "ark-graphics" || !r.focus {
		t.Errorf("the offer does not carry the whole identity: %+v", r)
	}
}

// AND THE INVOCATION A YES PRODUCES IS THE IDENTITY, NOT THE WORDS.
func TestSayingYesRunsTheIdentityAndNotTheName(t *testing.T) {
	h := &model{}
	started := nothingTookIt(t)
	answering(t, aRun(), "")
	dispatchIntake(h, "make the pictures nicer", invoke.SourceTyped)
	settled(t, anOfferIsPending)

	before := len(*started)
	runOfferedRoutine(h, takePendingRoutine())
	settled(t, func() bool { return len(*started) > before })

	args := (*started)[len(*started)-1]
	want := []string{"do", "--play=ark-graphics", "--app=arkascended", "--focus"}
	if strings.Join(args, " ") != strings.Join(want, " ") {
		t.Errorf("the invocation is %v, want %v", args, want)
	}
	// AND THE PHRASE IS NOWHERE IN IT, which is the point: the words would be resolved
	// again, by the registry's priority order, and with two routines of that name the
	// one performed would be whichever the second lookup lands on.
	for _, a := range args {
		if strings.Contains(a, "pictures") {
			t.Errorf("the invocation carries the words that were typed: %v", args)
		}
	}
}

// ── AND NOTHING ELSE BECOMES A RECORDING SESSION ─────────────────────────────

// WHICH IS THE DEFECT THIS IS ABOUT, THROUGH THE DOOR.
//
// A clarification, a chat reply, a refusal, a mismatch and a timeout are five answers with one
// thing in common: none of them is Marco saying it has nothing. Only Director's own `learn`
// decision is.
func TestOnlyDirectorsLearnDecisionBecomesALearnOffer(t *testing.T) {
	clarify := func() resolved {
		var r resolved
		r.Answer.Kind = "clarify"
		r.Answer.Reply = `"settings" is the name of 2 plays`
		return r
	}
	chat := func() resolved {
		var r resolved
		r.Answer.Kind = "chat"
		r.Answer.Reply = "I press things somebody has shown me."
		return r
	}
	runWithNothing := func() resolved {
		var r resolved
		r.Answer.Kind = "run" // and no identity, which must not become an offer
		r.Answer.Reply = "Opening it."
		return r
	}
	unavailable := func() resolved {
		var r resolved
		r.Why = "the Director is not running, so nothing can resolve this request"
		return r
	}
	mismatch := func() resolved {
		var r resolved
		r.Mismatch = "this was resolved against the library at A and you asked about B"
		return r
	}

	for _, how := range []struct {
		say   string
		make  func() resolved
		why   string
		want  string
		learn bool
	}{
		{"a clarification", clarify, "", "name of 2 plays", false},
		{"a chat reply", chat, "", "shown me", false},
		{"a run with nothing to run", runWithNothing, "", "Opening it", false},
		{"a Director that is not running", unavailable, "", "not running", false},
		{"a library mismatch", mismatch, "", "library at A", false},
		{"a timeout", func() resolved { return resolved{} },
			"the Director did not answer which play that is within 6s", "within 6s", false},
		{"nothing readable", func() resolved { return resolved{} },
			"what answered which play that is could not be read", "could not be read", false},
		// AND THE ONE THAT IS. Without this the rule above is satisfied by a widget
		// that has stopped offering to learn anything at all.
		{"Director saying it is something to record", func() resolved {
			var r resolved
			r.Answer.Kind = "learn"
			r.Answer.Reply = "I have nothing like that saved."
			return r
		}, "", "nothing like that", true},
	} {
		h := &model{}
		nothingTookIt(t)
		answering(t, how.make(), how.why)

		dispatchIntake(h, "make the pictures nicer", invoke.SourceTyped)
		// THE REPLY IS LOGGED BEFORE THE LEARN OFFER IS MADE, on the door's goroutine, so when
		// one is wanted it is the offer that is waited for: waiting for the reply alone read the
		// prompt in between, and failed about one run in fifteen.
		settled(t, func() bool {
			if how.learn {
				return anOfferIsPending()
			}
			return strings.Contains(strings.Join(said(h), " "), how.want) ||
				anOfferIsPending()
		})

		if r := takePendingRoutine(); r.id != "" {
			t.Errorf("%s: something was offered to run: %+v", how.say, r)
		}
		// THE LEARN OFFER, OR ITS ABSENCE.
		//
		// Read from the prompt the door raises rather than from the helper's answer,
		// because the join is what was wrong.
		offered := takePendingLearn() != ""
		if offered != how.learn {
			t.Errorf("%s: learn offered=%v, want %v (said: %v)", how.say, offered,
				how.learn, said(h))
		}
		// AND WHAT HAPPENED IS SAID, whichever it was.
		if !strings.Contains(strings.Join(said(h), " "), how.want) {
			t.Errorf("%s: the HUD does not say %q: %v", how.say, how.want, said(h))
		}
	}
}

// AND THE RESOLVER CHILD DOES NOT OUTLIVE THE ASK.
//
// The bound used to be a `select` around a goroutine: the wait ended and the child kept running,
// holding a pipe nobody would read and, through it, whatever the resolver plugin had started. A
// surface that abandons a process it spawned is the shape of the eight `marco do` processes this
// repository spent a release gate on.
//
// So the bound and the reaping are one act — `CommandContext` plus `Output` — and this holds the
// thing that can be held without a desktop: that the ask RETURNS, with a reason, rather than
// leaving the caller waiting on something it has stopped listening to.
func TestTheAskIsBoundedAndSaysSo(t *testing.T) {
	was := askWhichRoutine
	askWhichRoutine = func(string) (resolved, string) {
		// What the real one answers when its context ends.
		return resolved{}, "the Director did not answer which play that is within 6s"
	}
	t.Cleanup(func() { askWhichRoutine = was })

	h := &model{}
	done := make(chan disposition, 1)
	go func() { done <- offerWhatMarcoHas(h, "make the pictures nicer") }()
	select {
	case got := <-done:
		if got != saidWhyNot {
			t.Errorf("a timeout produced disposition %v, want the one that offers "+
				"nothing further", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the ask did not return, so the caller is waiting on something it has " +
			"stopped listening to")
	}
	if !strings.Contains(strings.Join(said(h), " "), "within 6s") {
		t.Errorf("the timeout is not reported: %v", said(h))
	}
}

// anOfferIsPending peeks at the two offer slots WITHOUT taking either.
//
// Its own helper because the first version of these tests waited on `takePendingLearn`, which
// clears what it reads — so the wait consumed the very offer the assertion two lines later was
// about, and the test failed for being a test rather than for anything the code did.
func anOfferIsPending() bool {
	pendingPromptMu.Lock()
	defer pendingPromptMu.Unlock()
	return pendingPrompt != "" || pendingRoutine.id != ""
}

// said is the HUD's log lines, under the lock the surface uses.
func said(h *model) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.logs...)
}
