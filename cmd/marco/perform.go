package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// The bridge between a learned play the resolver found and the Director that can actually
// walk it.
//
// # Why a learned play cannot run here
//
// `marco` has no perception. A learned play's first line asks the Screen whether the place it
// starts on is showing, and `marco` can only answer "I could not check" — so the play refuses at
// its own first line, in this process, every time. Meanwhile the Director next door has the
// world model, the verified edges and the one walker that can spend a bounded authority on real
// input. Measured live, the two halves never met: the play was demonstrated, verified, written
// down, registered, resolved — and then run by the half that cannot see.
//
// So a learned play is PERFORMED rather than RUN. Everything else — authored plays, taught
// plays, and a learned play somebody has since edited — goes down the local path completely
// unchanged, because for those `marco` is the right engine and always was.
//
// # Where this sits relative to the door
//
// After it. [[ADR-029-resolution-is-not-permission]] puts the authority seam between resolving a
// play and performing it, and delegation is performance. The question is asked in this process,
// by the front-end the person is actually looking at, and only an allowed play reaches here.
//
// # What this is NOT
//
// Not a second resolver, not a second transport, not a second performer, and not a fallback.
// It is `service.Client` — the one the overlay already speaks — carrying `PerformQuery`, the one
// request in the protocol that can act, to `Runtime.PerformGoal`, the one walker that verifies
// after every edge.

// directorPerformer is the whole of what this bridge needs from the Director.
//
// Two methods, so a test can stand in for a running service without a socket, a process or any
// real input — and so nothing else about the Director leaks into `marco do`. A wider interface
// would be an invitation to ask the Director something else from here, and this file has no
// business doing that.
type directorPerformer interface {
	// Perform carries out one learned outcome and reports what it did.
	Perform(service.PerformQuery) (service.PerformView, error)
	Close() error
}

// dialPerformer is how the bridge reaches a Director.
//
// A package variable purely so a test can substitute a fake endpoint: `marco do` must be
// exercisable end to end without starting a service that drives the real mouse. Production never
// reassigns it.
var dialPerformer = liveDirectorPerformer

// liveDirectorPerformer connects to the Director, starting it if it is not up.
//
// The EXISTING client — `directorReach`, the same connect-and-retry-once every other `marco
// director …` sub-command uses. There is one way for this binary to reach the service, and a
// second dialer here would be a second set of timeouts, a second auto-start policy and a second
// answer to "is it running".
func liveDirectorPerformer() (directorPerformer, error) {
	c, err := directorReach()
	if err != nil {
		return nil, err
	}
	return servicePerformer{c: c}, nil
}

// servicePerformer is the real client, speaking the frozen protocol.
type servicePerformer struct{ c *service.Client }

func (p servicePerformer) Perform(q service.PerformQuery) (service.PerformView, error) {
	raw, err := p.c.Observation(service.ObserveQuery{Perform: &q})
	if err != nil {
		return service.PerformView{}, err
	}
	var v service.PerformView
	if err := json.Unmarshal(raw, &v); err != nil {
		return service.PerformView{}, fmt.Errorf("the Director's reply was unreadable: %w", err)
	}
	return v, nil
}

func (p servicePerformer) Close() error { return p.c.Close() }

// performLearned hands one learned play to the Director and reports honestly what happened.
//
// It returns an error for every outcome in which the play did not do what the person asked, and
// nil only when a fresh look confirmed arrival — because `marco do` turns that error into a
// non-zero exit, and the overlay reads the exit code to decide whether to say "ran" or "failed".
// A refusal reported as exit 0 is the Audience being told it worked when nothing happened.
//
// Deleting the call to this from dispatchDo must fail TestALearnedPlayIsPerformedByTheDirector.
func performLearned(d orchestrator.Deps, r orchestrator.Resolved,
	named map[string]string, positional []string) error {

	out := outOf(d)

	// ARGUMENTS, SAID OUT LOUD RATHER THAN DROPPED.
	//
	// A learned play has no placeholders: Director lowers a verified walk into fixed
	// capability calls, and there is nowhere for `with a, b` to go. Substituting them
	// silently into nothing would let somebody watch the wrong thing happen and believe they
	// had asked for it.
	if len(named)+len(positional) > 0 {
		return fmt.Errorf("%q is a play Marco learned by watching, and it takes no arguments; "+
			"drop the %s and ask for it by name", r.Phrase, argWords(named, positional))
	}

	name, application, subject := performIdentity(d, r)
	mlog.Info("perform: delegating to the Director",
		"route", r.Route.Slug, "goal", name, "application", application, "subject", subject)

	client, err := dialPerformer()
	if err != nil {
		// NO LOCAL FALLBACK. Running it here would refuse at the play's own first line with
		// "Marco could not check", and the generated play CATCHES that refusal — so the
		// process would exit 0 and the overlay would report success for a play that never
		// ran. An honest failure naming the Director is the only truthful answer.
		return fmt.Errorf("%q is a play Marco learned by watching, so the Director has to "+
			"perform it — and the Director is not available: %w", r.Phrase, err)
	}
	defer client.Close()

	view, err := client.Perform(service.PerformQuery{
		Name: name, Application: application, Subject: subject,
	})
	if err != nil {
		return fmt.Errorf("the Director could not perform %q: %w", r.Phrase, err)
	}
	return renderPerform(out, r.Phrase, view)
}

// renderPerform prints what the Director did and says whether it counts as having happened.
//
// # Three outcomes, and they are not two
//
//	ARRIVED    every planned edge was walked and verified, and a FRESH look confirms the
//	           Audience is where they asked to be. This is the only success.
//	ALREADY    there was nothing to do. A true, useful answer — and not a performed play, so
//	           it must not be reported as one.
//	REFUSED    anything else, including a route that got half way. Reported as failure, with
//	           the steps that did verify, because "it stopped at step 2 of 3" and "it never
//	           started" are different facts and only one of them is progress.
//
// Deleting the per-step report, or widening the success test to "some step verified", must fail
// TestAPartlyWalkedRouteIsNotASuccess.
func renderPerform(out io.Writer, phrase string, v service.PerformView) error {
	verified := 0
	for i, s := range v.Steps {
		mark := "verified"
		if !s.Verified {
			mark = s.Refusal
			if mark == "" {
				mark = "did not verify"
			}
		} else {
			verified++
		}
		fmt.Fprintf(out, "  step %d of %d: %s\n", i+1, len(v.Steps), mark)
		if s.Detail != "" {
			fmt.Fprintf(out, "      %s\n", s.Detail)
		}
	}

	// PLAN-ONLY. `PerformGoal` reports Arrived with no steps when the Audience was already
	// standing on the outcome. Nothing was performed, and saying "ran" here would credit the
	// play with a state of the world it did not produce.
	if v.Arrived && len(v.Steps) == 0 {
		say := v.Say
		if say == "" {
			say = "You're already there."
		}
		fmt.Fprintf(out, "%s Marco performed no steps.\n", say)
		return nil
	}

	// SUCCESS is arrival AND every step verified. Not "the first edge worked", and not "the
	// plan ran to the end" — the Director takes a fresh look before it says Arrived, and
	// this insists on both halves so neither alone can pass for the whole.
	if v.Arrived && len(v.Steps) > 0 && verified == len(v.Steps) && v.Refusal == "" {
		if v.Say != "" {
			fmt.Fprintln(out, v.Say)
		}
		fmt.Fprintf(out, "%s: performed %d of %d steps.\n", phrase, verified, len(v.Steps))
		return nil
	}

	// EVERYTHING ELSE IS A FAILURE, and it says how far it got.
	say := strings.TrimSpace(v.Say)
	if say == "" {
		say = refusalWords(v.Refusal)
	}
	if len(v.Steps) > 0 {
		return fmt.Errorf("%s (%d of %d steps verified; %s)",
			say, verified, len(v.Steps), refusalWords(v.Refusal))
	}
	return fmt.Errorf("%s (%s)", say, refusalWords(v.Refusal))
}

// refusalWords renders the closed refusal vocabulary without pretending it is a sentence.
//
// The vocabulary is the Director's and is left as it is: a caller reading a log wants the exact
// word `step_unverified`, not a paraphrase that cannot be grepped. The Say beside it is the
// sentence a person reads.
func refusalWords(refusal string) string {
	if strings.TrimSpace(refusal) == "" {
		return "no reason given"
	}
	return refusal
}

// performIdentity is WHICH learned outcome to ask for, and in which application.
//
// # The join, and why it is by name today
//
// `PerformQuery` carries a name, and `PerformGoal` matches it case-insensitively against
// `Goal.Name` — the words the person used when the behaviour was learned. The play's slug came
// from `routes.Slug` of those same words (see cmd/director/learnedplay.go's Phrase branch, fed by
// the learn session's Name — the same string the goal was written under), so turning the slug
// back into words with `prettyRoute` recovers the name for every phrase made of plain words.
//
// It is NOT a general inverse: `routes.Slug` discards punctuation and collapses runs, so a
// behaviour learned as "open mouse settings (win11)" or "e-mail steve" slugs to something whose
// words no longer equal the goal's. Those cases refuse honestly with `not_learned` rather than
// performing the wrong thing, which is the right way for an imperfect join to fail.
//
// # The subject is the identity, and the name is only a label
//
// So the join is now on `Origin.To` — the DURABLE remembered subject this play ends on — which is
// the same id the goal was written under as `Goal.Subject`, in the same breath, by the same learn
// pass. It is exact for every phrase, survives a rename of either side, and needs no matching of
// words at all. The name still travels, because `PerformGoal` falls back to it for a goal with no
// sidecar and because a log with a subject id in it and no words is unreadable.
//
// The application comes from the sidecar rather than from the route's folder, because the
// sidecar is what Director wrote and the folder is where somebody could later move the file.
//
// Deleting the subject must fail TestTheLearnedPlayNameJoinRoundTrips, which now expects the
// awkward names to reach their goal instead of refusing.
func performIdentity(d orchestrator.Deps, r orchestrator.Resolved) (name, application, subject string) {
	name = prettyRoute(r.Route.Slug)
	application = r.Route.App
	if o, state := d.Reg.Origin(r.Route); state.Verified() {
		if a := strings.TrimSpace(o.Application); a != "" {
			application = a
		}
		subject = strings.TrimSpace(o.To)
	}
	return name, application, subject
}

// argWords names the arguments somebody supplied, so the refusal says what to drop.
func argWords(named map[string]string, positional []string) string {
	var parts []string
	for k := range named {
		parts = append(parts, k+":")
	}
	if n := len(positional); n > 0 {
		parts = append(parts, fmt.Sprintf("%d positional argument(s)", n))
	}
	return strings.Join(parts, ", ")
}

// outOf is where a person reads what happened.
//
// `Deps.Out` when the caller supplied one, so a test can capture it; stdout otherwise, which is
// where the overlay's stream reader is listening.
func outOf(d orchestrator.Deps) io.Writer {
	if d.Out != nil {
		return d.Out
	}
	return os.Stdout
}

// learnedOrigin is a compile-time reminder that this file reads provenance and never writes it.
var _ = routes.Origin{}
