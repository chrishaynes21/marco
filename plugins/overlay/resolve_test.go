package main

// WHAT THE WIDGET DOES WITH DIRECTOR'S ANSWER.
//
// The moment under test is the one that already existed: nothing took the phrase, and the widget
// is about to offer to record a routine. A person who typed "make the pictures nicer" may have a
// routine for it under another name, and that offer is then the one answer that is certainly
// wrong.
//
// Through the surface's own seam rather than by spawning `marco resolve`: what is worth holding
// here is what the widget DOES with each shape of answer, and the resolution itself is held where
// it lives, in `internal/resolveroute`.

import (
	"strings"
	"testing"
)

// answering makes the resolve seam reply with one answer, and puts it back afterwards.
func answering(t *testing.T, r resolved, ok bool) {
	t.Helper()
	was := askWhichRoutine
	askWhichRoutine = func(string) (resolved, bool) { return r, ok }
	t.Cleanup(func() { askWhichRoutine = was })
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

// AND IT OFFERS WHAT DIRECTOR NAMED INSTEAD OF OFFERING TO RECORD ONE.
func TestTheWidgetOffersTheRoutineDirectorNamed(t *testing.T) {
	h := &model{}
	answering(t, aRun(), true)

	if !offerWhatMarcoHas(h, "make the pictures nicer") {
		t.Fatal("the learn offer was not replaced, so somebody is being asked to " +
			"record a routine they already have")
	}
	// IT IS AN OFFER AND NOT A RUN. Nothing has been performed; what is pending is a
	// question.
	if r := pendingRoutine; r.id != "arkascended/focus/ark-graphics" {
		t.Fatalf("the pending offer is %+v", r)
	}
	// AND THE WHOLE IDENTITY IS WHAT IS PENDING, in the three parts an invocation is
	// built from: a name is not unique and neither is a slug.
	taken := takePendingRoutine()
	if taken.app != "arkascended" || taken.slug != "ark-graphics" || !taken.focus {
		t.Errorf("the offer does not carry the whole identity: %+v", taken)
	}
	// AND IT IS TAKEN ONCE. A second Enter must not run it again.
	if again := takePendingRoutine(); again.id != "" {
		t.Errorf("the offer survived being taken: %+v", again)
	}
}

// AND NO ANSWER IS NOT AN ANSWER OF NONE.
//
// Three ways there is no answer, and none of them may become a silent offer to record something:
// a Director that could not be asked, one that refused, and a library that is not the one this
// phrase is about. Each is SAID, and the learn offer then stands as it always did.
func TestTheWidgetSaysWhyThereIsNoAnswerAndStillOffersLearn(t *testing.T) {
	for _, how := range []struct {
		say  string
		make func() resolved
		want string
	}{
		{"the Director is not running", func() resolved {
			var r resolved
			r.Why = "the Director is not running, so nothing can resolve this request"
			return r
		}, "not running"},
		{"it speaks another protocol", func() resolved {
			var r resolved
			r.Why = "the Director is running and speaks a different protocol from " +
				"this command"
			return r
		}, "different protocol"},
		{"a different library", func() resolved {
			var r resolved
			r.Mismatch = "this was resolved against the library at A and you asked " +
				"about B"
			return r
		}, "library at A"},
	} {
		h := &model{}
		answering(t, how.make(), true)
		if offerWhatMarcoHas(h, "make the pictures nicer") {
			t.Errorf("%s: the learn offer was replaced by something that is not an "+
				"answer", how.say)
		}
		if r := takePendingRoutine(); r.id != "" {
			t.Errorf("%s: something was offered to run: %+v", how.say, r)
		}
		if !strings.Contains(strings.Join(said(h), " "), how.want) {
			t.Errorf("%s: the HUD does not say why: %v", how.say, said(h))
		}
	}
}

// AND A RESOLUTION THAT NAMED NO ROUTINE LETS THE LEARN OFFER STAND.
//
// The ordinary case for something Marco has never been shown. The sentence is still worth having
// — it says what was eligible and who answered — and then the offer to record one is the right
// answer rather than a wrong one.
func TestAResolutionThatNamedNothingLeavesTheLearnOffer(t *testing.T) {
	var none resolved
	none.Answer.Kind = "learn"
	none.Answer.Reply = "I have nothing like that saved."
	h := &model{}
	answering(t, none, true)

	if offerWhatMarcoHas(h, "make the pictures nicer") {
		t.Error("a resolution that named no routine replaced the learn offer")
	}
	if !strings.Contains(strings.Join(said(h), " "), "nothing like that") {
		t.Errorf("what Director said was not shown: %v", said(h))
	}
	if r := takePendingRoutine(); r.id != "" {
		t.Errorf("something was offered to run: %+v", r)
	}
}

// AND A SEAM THAT ANSWERED NOTHING AT ALL IS THE SAME AS NO ANSWER.
//
// `marco resolve` printing nothing, or something unreadable: the widget says nothing it cannot
// support and falls back to the offer it would have made anyway.
func TestAnUnreadableAnswerLeavesTheLearnOffer(t *testing.T) {
	h := &model{}
	answering(t, resolved{}, false)
	if offerWhatMarcoHas(h, "make the pictures nicer") {
		t.Error("an unreadable answer replaced the learn offer")
	}
	if r := takePendingRoutine(); r.id != "" {
		t.Errorf("something was offered to run: %+v", r)
	}
}

// said is the HUD's log lines, under the lock the surface uses.
func said(h *model) []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string(nil), h.logs...)
}
