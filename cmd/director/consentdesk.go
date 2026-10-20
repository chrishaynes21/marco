package main

// Answering Marco without taking the keyboard away from the application.
//
// # The problem this exists for
//
// A click carries no address. It lands wherever the pointer is, in whatever window is in front
// at that instant, which is why `workflowrun` re-observes the world immediately before every
// press and refuses when the target does not lead the desktop.
//
// That guard is correct, and it makes a terminal prompt unanswerable. Typing an answer makes the
// terminal the foreground window, so by the time somebody has pressed `y` the precondition for
// the press they just permitted is gone. Measured, not predicted: a supervised run reached the
// authorisation prompt and every grant was refused with "rocketleague does not lead the desktop".
//
// Reading does not take the foreground. Typing does. So the answer has to arrive as a key
// observed by a hook rather than read from a console — see `navsource.Consent`, which is where
// the three keys are admitted and why.
//
// # What this adds on top of the key
//
// A key is an answer only beside the question it answers, and the ways that can go wrong are the
// reason this is a type rather than a channel read:
//
//   - a key pressed BEFORE a question was asked must not answer it. The desk drains what it
//     holds at the moment a question opens, and counts what it dropped.
//   - one key must answer at most one question. A single goroutine owns the stream, so there is
//     no second reader to hand the same press to.
//   - a key pressed when nothing is being asked is not an error and not an answer. It is
//     counted, except for stop, which is always obeyed — a person pressing stop between actions
//     is exactly the case where nothing is pending.
//   - a question that nobody answers must expire rather than wait. An open prompt holds the
//     capture loop, and a run that blocked forever on a person who walked away would sit with a
//     grant half-given.
//
// It grants nothing and presses nothing. It turns a keypress into an answer and hands it back;
// what that answer permits is `workflowrun`'s to decide, and the only route to an effect is
// still `marcoexec`.

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
)

// consentDesk routes answers from the hook to whoever is waiting for one.
type consentDesk struct {
	keys <-chan navsource.Consent
	// stop is called for every stop key, whether or not a question is open.
	stop func()

	asks    chan *consentAsk
	cancels chan *consentAsk
	done    chan struct{}

	// stray counts answers that arrived with no question open, and drained counts those
	// discarded because they predated the question they would have answered.
	//
	// Reported rather than swallowed: a person who says "I pressed F7" and a run that says
	// nobody authorised anything are both telling the truth when a press landed in one of
	// these, and without a number there is no way to tell that from a hook that was never
	// installed.
	stray   int
	drained int
}

// consentAsk is one open question.
type consentAsk struct {
	reply chan navsource.Consent
}

func newConsentDesk(keys <-chan navsource.Consent, stop func()) *consentDesk {
	return &consentDesk{
		keys: keys, stop: stop,
		asks:    make(chan *consentAsk),
		cancels: make(chan *consentAsk),
		done:    make(chan struct{}),
	}
}

// run owns the stream. One goroutine, so an answer cannot be delivered twice.
func (d *consentDesk) run(ctx context.Context) {
	defer close(d.done)
	var pending *consentAsk
	for {
		select {
		case <-ctx.Done():
			return
		case a := <-d.asks:
			// WHATEVER IS ALREADY HERE PREDATES THE QUESTION, so it answers
			// nothing. Drained inside the same goroutine that receives keys,
			// which is what makes "before" and "after" well defined at all.
			//
			// Belt and braces over the `pending == nil` branch below, which is
			// what discards an early key in every ordinary case and is tested.
			// This closes the one case that branch cannot: a key already sitting
			// in the channel when a question opens, where the select below is
			// free to pick either case and would sometimes hand it to the
			// question it predates. No test can force that ordering from
			// outside, so a mutation run cannot kill this loop — it is here for
			// a race, and the count is what makes it visible when it fires.
		drain:
			for {
				select {
				case c, ok := <-d.keys:
					if !ok {
						return
					}
					// A STOP IS NEVER STALE.
					//
					// This drained every queued key without looking at
					// it, so a stop sitting in the channel when a
					// question opened was COUNTED as answering nothing
					// and thrown away. Discarding a premature approval
					// is the point of this loop — an approval answers
					// one question and must not answer the next one.
					// A stop answers no question at all: it ends the
					// run, and it means that whenever it was pressed.
					//
					// The same rule the ordinary branch below applies,
					// which is why it is spelled the same way.
					if c == navsource.ConsentStop {
						if d.stop != nil {
							d.stop()
						}
						continue
					}
					d.drained++
				default:
					break drain
				}
			}
			pending = a
		case a := <-d.cancels:
			if pending == a {
				pending = nil
			}
		case c, ok := <-d.keys:
			if !ok {
				return
			}
			// STOP IS OBEYED WHETHER OR NOT ANYTHING IS PENDING. A person
			// pressing stop between two actions is the case a desk that only
			// listened during prompts would miss.
			if c == navsource.ConsentStop {
				if d.stop != nil {
					d.stop()
				}
			}
			if pending == nil {
				d.stray++
				continue
			}
			pending.reply <- c
			pending = nil
		}
	}
}

// ask opens one question and waits for one answer.
//
// Returns false when nobody answered in time, when the run was cancelled, or when the desk has
// shut down. False is a complete answer: it means no permission was given.
func (d *consentDesk) ask(ctx context.Context, within time.Duration) (navsource.Consent, bool) {
	a := &consentAsk{reply: make(chan navsource.Consent, 1)}
	select {
	case d.asks <- a:
	case <-ctx.Done():
		return "", false
	case <-d.done:
		return "", false
	}
	timer := time.NewTimer(within)
	defer timer.Stop()
	select {
	case c := <-a.reply:
		return c, true
	case <-timer.C:
	case <-ctx.Done():
	case <-d.done:
		return "", false
	}
	// WITHDRAWN, so a key pressed after this moment is stray rather than an answer to a
	// question nobody is listening to any more.
	select {
	case d.cancels <- a:
	case <-d.done:
	}
	select {
	case c := <-a.reply:
		return c, true // it arrived while the withdrawal was in flight
	default:
	}
	return "", false
}

// counts is what the desk discarded, for the report.
func (d *consentDesk) counts() (stray, drained int) { return d.stray, d.drained }

// ── the granter ───────────────────────────────────────────────────────────────

// hotkeyGranter asks for the right to perform one action, and is answered by a key.
//
// It is a `workflowrun.Granter` and nothing else. It cannot see a screen, cannot press anything,
// and the authorisation it returns is checked by `Spendable` against the step, the window and the
// observation before it buys anything.
type hotkeyGranter struct {
	desk *consentDesk
	out  io.Writer
	live bool
	// within is how long a question stays open, and life how long a granted authorisation
	// lasts. Both short: the press happens in the same instant the grant is returned, and
	// a grant that outlived the moment would be the session flag ADR-183 refuses.
	within time.Duration
	life   time.Duration
	now    func() time.Time
	// events is the structured account of what this desk asked and what came back. Nil
	// when nobody asked for one, and every emission is safe on nil.
	events *runstate.Writer
	// scenes is WHICH interaction a step is, asked rather than guessed.
	//
	// The desk is handed a step by the coordinator, and a step is not an identity: for an
	// Act composed by picking it is not the position a person is watching, and it carries
	// nothing of the name the person gave it. See interactionidentity.go.
	scenes *performedInteractions
	// asked and granted are the PRESSES somebody was asked about and allowed; proposed
	// and agreed are the STEPS. Four numbers and not two, because agreeing to a step and
	// permitting a press are two questions and a report that counted only one of them
	// would say somebody answered once when they answered twice.
	asked    int
	granted  int
	proposed int
	agreed   int
	// screens and placed are the SCREENS somebody was asked to confirm and confirmed.
	// A fifth and sixth number for the same reason: saying "yes, that is the screen I am
	// looking at" is a third question, and it is the one that used to be unanswerable.
	screens int
	placed  int
}

func (g *hotkeyGranter) clock() time.Time {
	if g.now != nil {
		return g.now()
	}
	return time.Now()
}

func (g *hotkeyGranter) Authorise(ctx context.Context, in workflowrun.Intent) (
	workflowrun.Authorisation, bool) {

	g.asked++
	what := "write down"
	if g.live {
		what = "SEND"
	}
	fmt.Fprintf(g.out, "\n  %s\n", in.Describe())
	if in.Because != "" {
		fmt.Fprintf(g.out, "    found: %s\n", in.Because)
	}
	fmt.Fprintf(g.out, "    target: %s\n", in.Target(1920, 1080))
	fmt.Fprintf(g.out, "  May Marco %s a left click there, once?  %s yes   %s no   %s stop"+
		"   (%s)\n", what,
		navsource.ConsentKeys[navsource.ConsentApprove],
		navsource.ConsentKeys[navsource.ConsentRefuse],
		navsource.ConsentKeys[navsource.ConsentStop], g.within)

	// THE QUESTION, SAID BY THE THING THAT ASKS IT. See `asking`.
	at := g.scenes.at(in.Step, "")
	g.asking(fmt.Sprintf("press-%d-%d", in.Step, g.asked), runstate.Asking, at,
		fmt.Sprintf("May Marco %s a left click on %s, once?", what,
			in.Target(1920, 1080)))
	c, answered := g.desk.ask(ctx, g.within)
	if !answered {
		fmt.Fprintf(g.out, "  nobody answered within %s, so nothing was sent\n", g.within)
		// WITHDRAWN. Absent from the next snapshot, so no front end can leave it
		// on screen for somebody to answer a second time.
		g.answered(runstate.Verifying, at)
		return workflowrun.Authorisation{}, false
	}
	if c != navsource.ConsentApprove {
		fmt.Fprintf(g.out, "  answered %s\n", c)
		g.answered(runstate.Verifying, at)
		return workflowrun.Authorisation{}, false
	}
	g.granted++
	fmt.Fprintf(g.out, "  answered %s\n", c)
	// PERMITTED, so a press is what happens next — which the asker knows for certain
	// and a reader of prose would have had to infer.
	g.answered(runstate.Acting, at)
	return workflowrun.Authorisation{
		Step: in.Step,
		By: fmt.Sprintf("the person at the keyboard (%s)",
			navsource.ConsentKeys[navsource.ConsentApprove]),
		// THE OBSERVATION AND THE WINDOW THIS WAS ASKED ABOUT, copied from the
		// question. Copying them is not evidence that the world stood still — the
		// runner re-observes it — and it is what makes a cached permission from an
		// earlier screen or another window unspendable.
		Epoch: in.Epoch, Generation: in.Generation, Window: in.Window,
		Expires: g.clock().Add(g.life),
	}, true
}

// agree asks whether somebody agrees to a step at all, which is not the same question as whether
// Marco may press it now.
//
// Two questions, two keypresses, deliberately. A build that took one answer for both would let a
// person agree that a step belongs in the workflow and be taken to have permitted it to happen
// on the screen in front of them.
func (g *hotkeyGranter) agree(ctx context.Context, step int, control, from, to string) bool {
	g.proposed++
	fmt.Fprintf(g.out, "\n  Step %d of the saved workflow: press %s on %s, expecting %s.\n",
		step, control, from, to)
	fmt.Fprintf(g.out, "  Do you agree this is a step Marco should perform?  %s yes   "+
		"%s no   %s stop   (%s)\n",
		navsource.ConsentKeys[navsource.ConsentApprove],
		navsource.ConsentKeys[navsource.ConsentRefuse],
		navsource.ConsentKeys[navsource.ConsentStop], g.within)
	at := g.scenes.at(step, "")
	g.asking(fmt.Sprintf("agree-%d-%d", step, g.proposed), runstate.Agreeing, at,
		"Do you agree this is a step Marco should perform?")
	c, answered := g.desk.ask(ctx, g.within)
	if !answered {
		fmt.Fprintf(g.out, "  nobody answered within %s\n", g.within)
		g.answered(runstate.Agreeing, at)
		return false
	}
	fmt.Fprintf(g.out, "  answered %s\n", c)
	if c == navsource.ConsentApprove {
		g.agreed++
		// AGREED, so the next thing asked is permission for the press itself.
		g.answered(runstate.Asking, at)
		return true
	}
	g.answered(runstate.Agreeing, at)
	return false
}

// confirmScreen asks whether the saved screen Marco thinks it is looking at is the screen the
// person is actually on.
//
// # The third question, and the only one that is not about an action
//
// A run has to know where it is standing before it can know what the next step is. The typed
// path asks for that by printing the candidates and reading a label off standard input — and
// `--hotkeys` exists precisely because typing an answer takes the foreground away from the
// application, which then refuses the press the answer permitted. So in hotkey mode nobody could
// be asked at all, and the run refused every first observation with *name the screen with --from
// and start again*. From a widget there is no terminal to name it in: the one path a person had
// to the half of the product they made with their own hands dead-ended here.
//
// # Why one yes/no is enough, and why it is not a silent choice
//
// The ranking is an arrangement similarity over the artifact's own saved screens, and it cannot
// say whether the right answer is in the list — that is ADR-186's whole point, and it is what a
// person adds. So Marco names its best candidate and the SCORE it rests on, and the person says
// yes or no with the same key that authorises everything else.
//
// A no is not a worse answer: the screen is declined exactly as `none` declines it on the typed
// path, nothing is pressed, and the run goes on looking. Taking the top candidate without asking
// would be the thing this must not do — it would spend an unasked authorisation on a screen
// nobody had looked at.
func (g *hotkeyGranter) confirmScreen(ctx context.Context, label string, score float64) (
	navsource.Consent, bool) {

	g.screens++
	fmt.Fprintf(g.out, "\n  Marco thinks the screen in front of you is %s "+
		"(its arrangement agrees %.3f).\n", label, score)
	fmt.Fprintf(g.out, "  Is that the screen you are on?  %s yes   %s no   %s stop"+
		"   (%s)\n",
		navsource.ConsentKeys[navsource.ConsentApprove],
		navsource.ConsentKeys[navsource.ConsentRefuse],
		navsource.ConsentKeys[navsource.ConsentStop], g.within)
	g.asking(fmt.Sprintf("screen-%s-%d", label, g.screens), runstate.Naming, nil,
		fmt.Sprintf("Marco thinks the screen in front of you is %s (its arrangement "+
			"agrees %.3f). Is that the screen you are on?", label, score))
	c, answered := g.desk.ask(ctx, g.within)
	if !answered {
		fmt.Fprintf(g.out, "  nobody answered within %s, so the screen is not "+
			"established and nothing was pressed\n", g.within)
		g.answered(runstate.Naming, nil)
		return "", false
	}
	fmt.Fprintf(g.out, "  answered %s\n", c)
	if c == navsource.ConsentApprove {
		g.placed++
	}
	g.answered(runstate.Naming, nil)
	return c, true
}

// ── THE STRUCTURED ACCOUNT OF A QUESTION, SAID BY THE THING THAT ASKS IT ──────
//
// Every prompt below emits three times: the question going up, and then exactly one of the two
// ways it comes down — answered, or withdrawn unanswered. A front end rendering the latest
// snapshot therefore cannot leave a stale prompt on screen, which is the likeliest thing for a
// person to answer twice.
//
// The words are carried INSIDE the event for the transcript and are never what a reader decides
// from. That is the whole point: Studio used to match on "May Marco", so improving the wording of
// a consent prompt would have silently changed what a window showed somebody about a press they
// were about to authorise.

// say emits one snapshot for this granter's stream, when there is one.
func (g *hotkeyGranter) say(e runstate.Event) { g.events.Say(e) }

// asking is the question going up: the phase, the words, the keys it accepts, and when it stops
// accepting them.
func (g *hotkeyGranter) asking(id string, phase runstate.Phase, in *runstate.Interaction,
	asks string) {

	g.say(runstate.Event{Phase: phase, Interaction: in,
		Question: &runstate.Question{ID: id, Asks: asks,
			Keys:  consentKeys(),
			Until: g.clock().Add(g.within)}})
}

// answered is the question coming down, either way.
//
// `phase` is where the run goes next, which the ASKER knows and a reader of prose would have to
// guess: a permitted press is about to happen, a refused one is not.
func (g *hotkeyGranter) answered(phase runstate.Phase, in *runstate.Interaction) {
	g.say(runstate.Event{Phase: phase, Interaction: in})
}

// consentKeys is what every consent question accepts, taken from the key vocabulary itself so a
// front end shows the keys the runner is really listening for.
func consentKeys() []runstate.Key {
	return []runstate.Key{
		{Key: navsource.ConsentKeys[navsource.ConsentApprove], Means: "yes"},
		{Key: navsource.ConsentKeys[navsource.ConsentRefuse], Means: "no"},
		{Key: navsource.ConsentKeys[navsource.ConsentStop], Means: "stop"},
	}
}

// startKeys is what the START question accepts, which is NOT what a consent question accepts.
//
// `started` listens for yes and for stop, and a refusal key answers nothing there: no question
// about an action is open yet, and treating "no" as a cancellation would make the three keys mean
// something different before the run from what they mean during it.
//
// So the start question advertises two. Advertising all three was a window telling somebody a key
// would answer a question that the thing holding the key hook deliberately ignores — and the
// honest failure of that is worse than useless: a person presses it, nothing happens, and the
// natural next thought is that the run is not listening at all.
func startKeys() []runstate.Key {
	return []runstate.Key{
		{Key: navsource.ConsentKeys[navsource.ConsentApprove], Means: "yes, start now"},
		{Key: navsource.ConsentKeys[navsource.ConsentStop], Means: "cancel the run"},
	}
}
