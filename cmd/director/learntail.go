package main

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/chaynes-simpleclouds/marco/internal/director/learn"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// The rest of the lifecycle, as the Learn coordinator reaches it.
//
// Every method here forwards to the path that already exists and adds no decision of its own:
//
//	Question  → the proposal ledger, read
//	Granted   → the one ephemeral rehearsal grant, read
//	Rehearse  → Runtime.Rehearse, which finds the grant and hands the whole attempt to the
//	            live runner
//	Lowering  → Runtime.LearnedPlay, a READ that recomputes the judgement and raises the
//	            naming question the judgement demands
//	Save      → Runtime.LearnedPlay with Save, the one persistence path
//
// The coordinator cannot import any of that — its boundary test forbids `marcoexec`, `rehearse`
// and every platform package — which is the point of the interface. This adapter is where the
// two meet, and it is deliberately thin enough to read in one sitting.

type learnTail struct {
	rt *Runtime
	// app is which application the session is about, read LATE.
	//
	// A function rather than a string because the answer is not known when Learn starts:
	// a window may be selected by ephemeral id, and which application that turns out to be is
	// something the first pass discovers. A value captured at construction would be empty.
	app func() string
	// phrase is what the AUDIENCE called this behaviour, read late for the same reason.
	//
	// It is what they will ask for, and it is not the play.s Marco identity: a person who had
	// Marco learn "Open Mouse Settings" gets `do MouseSettings.s Open` in the file and expects
	// their own words to work at the prompt.
	phrase func() string
	// live says the authorised rehearsal emits REAL input.
	//
	// True in production, because "want me to try it once?" means trying it. A dry attempt
	// changes nothing on screen and therefore cannot verify a destination, so it would fail
	// honestly rather than mislead — but it would also waste the user's yes.
	live bool
}

var (
	_ learn.Tail = (*learnTail)(nil)
	// The optional halves. Asserted so a signature drift silently demotes this tail to one
	// that "cannot know" instead of failing to build — which is how a diagnostic that
	// existed came to be absent from every surface for two live runs.
	_ learn.GrantDiagnoser    = (*learnTail)(nil)
	_ learn.QuestionDiagnoser = (*learnTail)(nil)
)

// Question finds the open proposal of one kind for this route.
//
// A READ over the ledgers that already hold it. There is no path here that could raise a question:
// proposals are raised by the machinery that judged the evidence, and a coordinator with its own
// question system would be a second one to keep in agreement forever.
func (t *learnTail) Question(route observe.RelationshipRef, kind observe.AskKind) (
	learn.Question, bool) {

	g := t.rt.observations
	if g == nil {
		return learn.Question{}, false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	for i := len(g.finished) - 1; i >= 0; i-- {
		res := g.finished[i]
		for _, p := range res.Proposals.Proposals {
			if p.Ask != kind || p.Response != observe.ResponseNone {
				continue
			}
			// A rehearsal question is about a ROUTE; a naming question is about a screen,
			// and the screens that block this play are its own endpoints.
			if kind == observe.AskRehearse &&
				(p.Relationship == nil || *p.Relationship != route) {
				continue
			}
			q := learn.Question{ID: p.ID, SessionID: res.Session.ID}
			if p.Screen != nil {
				if p.Screen.ID != route.From && p.Screen.ID != route.To {
					continue
				}
				q.Screen = p.Screen.ID
			}
			return q, true
		}
	}
	return learn.Question{}, false
}

// Granted reports whether an explicit yes created authority for THIS route.
//
// Fail-closed by construction: the grant is created in exactly one place — a confirmed answer to
// a rehearsal question — so silence, a decline, a not-now and a malformed answer all leave this
// false without anything here having to check for them.
func (t *learnTail) Granted(route observe.RelationshipRef) bool {
	g := t.rt.observations
	if g == nil {
		return false
	}
	g.mu.RLock()
	last := g.last
	g.mu.RUnlock()
	grant := last.Grant()
	return grant != nil && grant.Active() && grant.Relationship == route
}

// GrantRefusal reports why the most recent yes created no authority, empty when it did — the
// optional half of Granted, probed by the coordinator through learn.GrantDiagnoser.
//
// It exists because every cause used to be a silent return: the person answered yes, no
// grant appeared, and ten minutes later Learn reported `rehearsal_declined` as though
// nobody had answered at all. The runner now records the closed reason; this carries it up.
func (t *learnTail) GrantRefusal(route observe.RelationshipRef) string {
	g := t.rt.observations
	if g == nil {
		return ""
	}
	g.mu.RLock()
	last := g.last
	g.mu.RUnlock()
	// ABOUT THIS ROUTE. A sequential edge review asks one leg at a time, and it used to read
	// whatever reason was recorded last — so a refusal from answering leg 2 was reported as
	// leg 1.s, confidently and wrongly, and could end leg 1 on it.
	//
	// Deleting the route argument must fail TestAGrantRefusalIsAboutTheRouteItWasRecordedFor.
	return string(last.AuthorizationRefusedFor(route))
}

// Rehearse spends the grant through the ordinary entry point.
func (t *learnTail) Rehearse(context.Context) (learn.Attempt, error) {
	v, err := t.rt.Rehearse(service.ObserveRehearse{Live: t.live})
	if err != nil {
		return learn.Attempt{}, err
	}
	return learn.Attempt{
		Attempted: v.Attempted, Completed: v.Completed,
		Terminal: v.Terminal, Refusal: v.Refusal, Live: v.Live, Detail: v.Detail,
		Steps: attemptSteps(v.Steps),
	}, nil
}

// Lowering recomputes whether this route may be written down.
//
// A READ. It also raises the naming question the judgement demands, because `LearnedPlay` does
// that on every read and has since the naming lifecycle landed — Learn does not ask for it and
// could not.
func (t *learnTail) Lowering(route observe.RelationshipRef) (learn.Readiness, error) {
	v, err := t.rt.LearnedPlay(service.LearnedQuery{Application: t.app()})
	if err != nil {
		return learn.Readiness{}, err
	}
	for _, p := range v.Plays {
		if p.From != route.From || p.To != route.To {
			continue
		}
		r := learn.Readiness{Eligible: p.Eligible, Refusals: p.Refusals, Source: p.Source}
		if len(p.Unnamed) > 0 {
			r.Unnamed = p.Unnamed[0]
		}
		return r, nil
	}
	return learn.Readiness{}, fmt.Errorf(
		"nothing Marco has learned describes going from %s to %s in %s",
		route.From, route.To, t.app())
}

// Save writes the play through the one persistence path AND registers it.
//
// Saving is "keep this" and registering is "and let me ask for it" — two steps the lifecycle
// keeps separate, and `LearnedPlay` still exposes them separately for the developer CLI. Learn
// asks for BOTH in one call because a learn that ends "you can ask me to do it later" has
// promised discoverability; leaving the artifact in `learned/`, where the resolver deliberately
// cannot see it, makes that sentence false.
//
// The permission that gates this is the Audience's yes to the learn itself, not a second one:
// nothing reaches here without a completed, authorised demonstration.
func (t *learnTail) Save(route observe.RelationshipRef, actor, verb string) (learn.Saved, error) {
	v, err := t.rt.LearnedPlay(service.LearnedQuery{
		Application: t.app(), Name: actor, Verb: verb, Phrase: t.taught(),
		From: route.From, To: route.To,
		// SAVED AND REGISTERED IN ONE ACT. Marco.s completion sentence is "you can ask me
		// to do it later", and that was false while the artifact sat in `learned/` — a
		// directory the resolver deliberately cannot see. Saving without registering
		// promises a capability that `marco routes` reports does not exist.
		//
		// Deleting Register must fail TestALearnedPlayIsRegisteredWhenItIsSaved.
		Save: true, Register: true,
	})
	return savedFrom(v, err)
}

// savedFrom reads one lifecycle answer, INCLUDING the half-done one.
//
// # A registration failure is not a lost play
//
// `lifecycle` publishes `v.Saved` the moment the artifact is on disk, and returns the register
// half's error alongside it. Collapsing that to a bare error made the Learn coordinator refuse
// `save_failed` — "I couldn't save it. Nothing was learned." — over a file the user could open,
// read and edit. The second Learn of a phrase already taken hit this every time, and the person
// was told their work was gone rather than that the name was.
//
// Saved-and-not-registered is a real state with its own refusal (`play_not_registered`) and its
// own sentence. This is what routes it there, carrying WHY as `Reason`.
//
// Deleting the partial branch must fail TestASaveThatCannotRegisterStillReportsTheArtifact.
func savedFrom(v service.LearnedView, err error) (learn.Saved, error) {
	if err != nil {
		if v.Saved != nil && v.Saved.Saved {
			return learn.Saved{
				Name: v.Saved.Name, Saved: true, Registered: false,
				Source: v.Saved.Source, Reason: err.Error(),
			}, nil
		}
		return learn.Saved{}, err
	}
	if v.Saved == nil {
		return learn.Saved{}, fmt.Errorf("the save reported nothing")
	}
	return learn.Saved{
		Name: v.Saved.Name, Saved: v.Saved.Saved,
		Registered: v.Saved.Registered, Source: v.Saved.Source,
	}, nil
}

// ── the one phrase a person writes ────────────────────────────────────────────

// playNameFor turns what the user asked for into the two halves of a Marco sentence.
//
// # Why two
//
// Because a play is a sentence: `do Downloads's Open …`. A thing, and what it does. Director has
// no business inventing either from a screen's text, so both come from the person — and the
// phrase they typed is the shortest way to give both at once.
//
//	"open downloads"       →  actor Downloads,     verb Open
//	"open mouse settings"  →  actor MouseSettings, verb Open
//
// The first word is what it does and the REST is the thing, because that is how the request
// reads aloud.
//
// # Why the rest, rather than exactly one more word
//
// It used to demand exactly two words, and that was a leftover from when the phrase WAS the
// play's name. Under the goal-centric model they are two different artifacts: the phrase is the
// OUTCOME's name, kept verbatim on the durable goal in the person's own words, and the actor and
// verb are only the sentence a saved play becomes. Nothing about the language requires the two to
// be the same string — `MuteEveryone` is `CheckPlayName`'s own example of a legal actor.
//
// What the old rule cost was the acceptance criterion itself: `Learn "open mouse settings"` — the
// phrase in the roadmap, in E2E.md and in the milestone's own statement of what a normal person
// should be able to say — was refused before anything was observed, with an instruction to learn
// two flags. That is Marco's protocol leaking into the user's sentence, which is the exact thing
// [[ADR-056-a-goal-is-a-destination-not-a-route]] exists to remove.
//
// The original objection stands and is answered rather than overruled: a phrase welded into an
// identifier SILENTLY is a developer identifier wearing the user's words. So the derivation is
// said out loud — `Derived` carries it back, and the surface prints what the play will be called.
//
// Validated against the real naming rules, here, where they live — the coordinator cannot reach
// `marcoexec` and must not.
func playNameFor(phrase, actorFlag, verbFlag string) (actor, verb string, err error) {
	if actorFlag != "" || verbFlag != "" {
		actor, verb = actorFlag, verbFlag
		if actor == "" || verb == "" {
			return "", "", fmt.Errorf(
				"--actor and --verb go together: a play is a sentence, `do Downloads's Open`")
		}
	} else {
		words := strings.Fields(phrase)
		if len(words) < 2 {
			// One word cannot be a sentence of two. Refused with the two ways forward,
			// and BEFORE anybody is asked to demonstrate anything.
			return "", "", fmt.Errorf(
				"%q is one word, and a play is a sentence of two — what it does, and what "+
					"it does it to, like %q.\nGive a name with both parts, or say them: "+
					"--actor Downloads --verb Open",
				phrase, "open downloads")
		}
		verb = title(words[0])
		for _, w := range words[1:] {
			actor += title(w)
		}
	}
	if err := marcoexec.CheckPlayName(actor); err != nil {
		return "", "", fmt.Errorf("the name for the thing itself: %w", err)
	}
	if err := marcoexec.CheckPlayName(verb); err != nil {
		return "", "", fmt.Errorf("the name for what it does: %w", err)
	}
	return actor, verb, nil
}

// title capitalises one word without touching the rest of it.
//
// `strings.Title` is deprecated and lowercases nothing; this leaves `OK` as `OK` and makes
// `downloads` into `Downloads`, which is what a person typing lower case meant.
func title(w string) string {
	if w == "" {
		return w
	}
	r := []rune(w)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// attemptSteps narrows the rehearsal's own step records to what a surface may see.
func attemptSteps(in []service.RehearsalStepView) []learn.AttemptStep {
	out := make([]learn.AttemptStep, 0, len(in))
	for _, s := range in {
		out = append(out, learn.AttemptStep{
			Step: s.Step, Intents: s.Intents, Expected: s.Expected,
			Observed: s.Observed, Outcome: s.Outcome, Detail: s.Detail,
		})
	}
	return out
}

// QuestionRefusal says why no question of this kind is open for this route.
//
// The optional half of Question, and the answer to a silence that had three causes and one
// appearance. In order of how it reads to somebody stuck:
//
//   - the judgement refused, and says so in its own closed vocabulary;
//   - a question exists and has already been answered, so there is nothing left to ask;
//   - nothing judged this route at all.
//
// The middle case is worth its own sentence: "you already answered this" and "Marco never asked"
// are opposite situations that both present as a stage that will not move.
//
// Deleting this must fail TestTheTailSaysWhyThereIsNoQuestion.
func (t *learnTail) QuestionRefusal(route observe.RelationshipRef, kind observe.AskKind) string {
	g := t.rt.observations
	if g == nil {
		return ""
	}
	g.mu.RLock()
	defer g.mu.RUnlock()

	answered := false
	judged := false
	for i := len(g.finished) - 1; i >= 0; i-- {
		res := g.finished[i]
		// WHAT THE MOST RECENT JUDGEMENT CONCLUDED.
		//
		// The NEWEST session that judged this route wins, whatever it concluded — a clean
		// judgement included. Skipping clean ones and carrying on backwards reported an
		// old failure as the current reason: live, the panel said `another_question_open`
		// beside `questions open: 0`, which is not merely unhelpful but false, and it sent
		// two rounds of diagnosis at a budget that was not the problem.
		//
		// A diagnostic that can be stale is worse than none: it is trusted.
		//
		// Deleting the `judged` guard must fail TestAStaleRefusalIsNotReportedAsCurrent.
		for _, j := range res.Rehearsals {
			if j.Relationship != route {
				continue
			}
			judged = true
			if len(j.Refusals) == 0 {
				break
			}
			words := make([]string, 0, len(j.Refusals))
			for _, r := range j.Refusals {
				words = append(words, string(r))
			}
			return strings.Join(words, ", ")
		}
		if judged {
			// It was judged most recently and refused nothing, so the budget and the
			// evidence were both fine. Why there is still no question is a different
			// fact, and saying so beats naming a reason that has passed.
			break
		}
		for _, p := range res.Proposals.Proposals {
			if p.Ask != kind {
				continue
			}
			if kind == observe.AskRehearse &&
				(p.Relationship == nil || *p.Relationship != route) {
				continue
			}
			if p.Response != observe.ResponseNone {
				answered = true
			}
		}
	}
	if answered {
		return "it was already answered"
	}
	if judged {
		// The honest end of the road: the evidence was judged, nothing was refused, and no
		// question exists anyway. That is a fact about Marco rather than about the person,
		// and it is the one worth surfacing — every stale reason reported instead of it
		// cost a round of chasing the wrong layer.
		return "the evidence was judged and refused nothing, and no question was raised — " +
			"which is a fault in Marco, not in the demonstration"
	}
	return ""
}

// OfferRehearsal puts ONE required edge of the demonstration under review to the Audience.
//
// # What it changes, and what it does not
//
// It recomputes the judgement for that route — against what Marco believes NOW, like every other
// rehearsal decision — and reviews it with the interruption slot conflict lifted for this route
// alone. Everything else is the ordinary path: the proposal machinery judges the evidence, an
// ineligible route produces no question, and an eligible one produces a question the Audience
// still has to answer. No grant is created here and none could be.
//
// The passive budget is untouched. `MaxProposals` still bounds the ledger, one proposal per route
// is still enforced by identity, and nothing outside an explicit Learn episode can reach this.
//
// Deleting the widened policy must fail TestTheSecondEdgeGetsItsOwnQuestion.
func (t *learnTail) OfferRehearsal(route observe.RelationshipRef) error {
	g := t.rt.observations
	if g == nil {
		return fmt.Errorf("this Director has no observation registry")
	}
	g.mu.RLock()
	application := ""
	for i := len(g.finished) - 1; i >= 0; i-- {
		if g.finished[i].Session.Application != "" {
			application = g.finished[i].Session.Application
			break
		}
	}
	memory := g.memory
	g.mu.RUnlock()
	if application == "" || memory == nil {
		return fmt.Errorf("nothing has been observed yet")
	}
	judgement, ok := g.judgeNow(application, route)
	if !ok {
		return fmt.Errorf("the evidence for this step is no longer there")
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	for i := len(g.finished) - 1; i >= 0; i-- {
		res := &g.finished[i]
		if !strings.EqualFold(res.Session.Application, application) {
			continue
		}
		// THE CONTINUATION ALLOWANCE, for this route and this call only.
		//
		// Normalised first, for the reason the invited-rehearsal exemption records: the
		// registry sets no policy, so a zero value here would have the defaults
		// substituted underneath and put the budget of one straight back.
		th := observe.DefaultProposalThresholds()
		th.MaxOpen = th.MaxProposals
		// A grant already outstanding stops Marco asking for a second — the ordinary rule,
		// unchanged. The continuation lifts the interruption conflict, never the authority.
		// Authorised for THIS route, not "some authority exists" — see the same rule in
		// Runner.reviewRehearsal. A grant for another leg must not silence this one.
		gr := g.last.Grant()
		granted := gr != nil && gr.Active() && gr.Relationship == route
		res.Proposals.ReviewRehearsal(judgement, memory.Topology(application), granted, th)
		return nil
	}
	return fmt.Errorf("no finished session to ask against")
}

// AnswerToRehearsal is what the Audience said about this route, and whether they said anything.
//
// The positive half of the silence `Question` cannot explain. A proposal whose `Response` is no
// longer `ResponseNone` was put to somebody and came back — yes, no, or not now — and none of
// those is "Marco never managed to ask".
//
// The answer travels rather than a bare bool, because the review must not read a yes as a refusal
// while the grant that yes creates is still on its way. See RehearsalAnswer.
//
// Newest first: a route asked about twice is answered by the most recent answer.
//
// Deleting this must fail TestTheTailReportsWhatWasAnsweredAboutARehearsal.
func (t *learnTail) AnswerToRehearsal(route observe.RelationshipRef) (observe.UserResponse, bool) {
	g := t.rt.observations
	if g == nil {
		return observe.ResponseNone, false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	for i := len(g.finished) - 1; i >= 0; i-- {
		for _, p := range g.finished[i].Proposals.Proposals {
			// SETTLED, not merely answered. A retracted proposal has no response and is
			// closed for good — reporting it as unasked stalls a review on a question
			// nobody will raise again. See RehearsalAnswer.
			if p.Ask != observe.AskRehearse {
				continue
			}
			// SETTLED: somebody responded, or the question was explicitly closed.
			//
			// Fail closed on anything else. A status this does not recognise is not
			// evidence that a leg is over, and concluding it was would retire a step on
			// a malformed record — the opposite of the stall this exists to end.
			closed := p.Status == observe.ProposalAnswered ||
				p.Status == observe.ProposalDeclined
			if p.Response == observe.ResponseNone && !closed {
				continue
			}
			if p.Relationship != nil && *p.Relationship == route {
				return p.Response, true
			}
		}
	}
	return observe.ResponseNone, false
}

// SaveRoute writes the play for a whole ordered walk, through the same persistence path.
//
// The only difference from Save is which behaviour is written down: the walk the Audience
// demonstrated rather than the single edge the review happens to be pointing at. See
// learn.RouteSaver.
//
// Deleting the walk from the query must fail TestSavingAMultiEdgeRouteWritesTheWholeWalk.
func (t *learnTail) SaveRoute(walk []observe.RelationshipRef, actor, verb string) (
	learn.Saved, error) {

	if len(walk) == 0 {
		return learn.Saved{}, fmt.Errorf("a demonstration needs at least one step to be written down")
	}
	steps := make([]service.LearnedStep, 0, len(walk))
	for _, e := range walk {
		steps = append(steps, service.LearnedStep{From: e.From, To: e.To})
	}
	v, err := t.rt.LearnedPlay(service.LearnedQuery{
		Application: t.app(), Name: actor, Verb: verb, Phrase: t.taught(),
		From: walk[0].From, To: walk[len(walk)-1].To,
		Walk: steps, Save: true, Register: true,
	})
	return savedFrom(v, err)
}

// taught is the Audience's own words for this behaviour, empty when nothing supplies them.
func (t *learnTail) taught() string {
	if t.phrase == nil {
		return ""
	}
	return t.phrase()
}
