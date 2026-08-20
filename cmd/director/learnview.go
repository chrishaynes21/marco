package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
)

// What a control surface needs to show a person while Marco is learning.
//
// # A projection, and nothing but
//
// Every field here is READ off the coordinator's session, which is the canonical state. Nothing
// is computed that the session does not already know, nothing is remembered between reads, and
// there is no path from this file to a pass, a capture, an authority or the store. A surface that
// showed something this projection could not produce would be showing something Marco does not
// actually believe.
//
// # Why it exists rather than the surface reading the session
//
// Because the alternative is a surface inferring state from prose. The plain reading is a
// sentence for a person — "Okay, go ahead and show me" — and a UI that switched its buttons on
// the presence of that substring would break the first time somebody improved the wording. Part
// of what a control surface needs is not in the sentence at all: how many actions have been
// captured, what the last one landed on, whether Marco is waiting for the person or the person is
// waiting for Marco.

// LearnStage is the human-facing lifecycle a surface renders.
//
// Deliberately COARSER than teach.Phase and deliberately separate from it. The phases are
// orchestration states — which internal thing the coordinator is blocked on — and several of them
// mean one thing to a person. `establishing_start`, `ready_for_demo` and `capturing` are all "go
// ahead and do it"; `evaluating` and `establishing_destination` are both "hold on".
//
// It is a projection, so the domain keeps its own vocabulary and a surface never has to learn it.
type LearnStage string

const (
	LearnIdle LearnStage = "idle"
	// LearnWaitingToStart is Marco waiting for the person to go to their application.
	LearnWaitingToStart LearnStage = "waiting_for_demonstration"
	// LearnLearning is capture running: whatever the person does now is the demonstration.
	LearnLearning LearnStage = "learning"
	// LearnFinishing is Stop pressed and bounded interpretation still finishing.
	LearnFinishing LearnStage = "finishing"
	// LearnNeedsAnother is Marco asking for the route a second time.
	LearnNeedsAnother LearnStage = "needs_another_example"
	// LearnReadyToTry has a candidate and is waiting for permission.
	LearnReadyToTry LearnStage = "ready_to_try"
	// LearnWaitingToTry has permission and is waiting for the right screen to appear.
	LearnWaitingToTry LearnStage = "waiting_to_try"
	// LearnTrying is the authorised attempt, running.
	LearnTrying LearnStage = "trying"
	// LearnNaming is waiting for the person to name a screen.
	LearnNaming LearnStage = "naming"
	// LearnUnderstood is finished, with something learned.
	LearnUnderstood LearnStage = "understood"
	// LearnRefused is finished, with an honest reason.
	LearnRefused LearnStage = "refused"
	// LearnStopped is finished because the person cancelled.
	LearnStopped LearnStage = "stopped"
)

// stageOf projects a coordinator phase onto the lifecycle a person sees.
//
// A total function over the phase vocabulary: an unmapped phase would leave a surface showing
// nothing, so the default is the honest "working" rather than a blank.
func stageOf(s teach.Session, finishing bool) LearnStage {
	switch s.Phase {
	case teach.WaitingForDemonstration:
		return LearnWaitingToStart
	case teach.EstablishingStart, teach.ReadyForDemo, teach.Capturing:
		if finishing {
			return LearnFinishing
		}
		return LearnLearning
	case teach.EstablishingDestination, teach.Evaluating, teach.Lowering:
		return LearnFinishing
	case teach.NeedsAnotherExample:
		return LearnNeedsAnother
	case teach.ReadyToRehearse:
		return LearnReadyToTry
	case teach.WaitingForStart:
		return LearnWaitingToTry
	case teach.Rehearsing:
		return LearnTrying
	case teach.Naming:
		return LearnNaming
	case teach.Complete:
		return LearnUnderstood
	case teach.Refused:
		return LearnRefused
	case teach.Cancelled:
		return LearnStopped
	}
	return LearnFinishing
}

// learnView is the whole of what a control surface renders.
type learnView struct {
	// Running says a session exists at all. Everything else is meaningless without it.
	Running bool       `json:"running"`
	Stage   LearnStage `json:"stage"`
	// Name is the person's own words for what they asked Marco to learn.
	Name string `json:"name,omitempty"`
	// Saying is the plain sentence, for a person to read. A surface may show it; it must
	// not parse it.
	Saying string `json:"saying,omitempty"`
	// Watching is the application Marco is observing, empty before it has one.
	Watching string `json:"watching,omitempty"`
	// TargetLocked says the window this attempt is about has been fixed.
	//
	// # Why a person needs this BEFORE they demonstrate
	//
	// Because the alternative is finding out afterwards. Live: somebody pressed Start, walked
	// to Settings past File Explorer, and Marco latched Explorer — watched it for the whole
	// pass, saw nothing, and said so only at the end. The window was decided in the first
	// second and the demonstration was already wasted.
	//
	// Distinct from Watching being non-empty on purpose: an application name can be known
	// while the settle rule is still counting, and "nearly" is exactly when somebody must not
	// start clicking. See teachSubjectSettle.
	TargetLocked bool `json:"target_locked"`
	// Here is where Marco thinks you are RIGHT NOW, and whether it recognises it.
	//
	// Present whether or not anything is being taught: hardening place identity means
	// walking an application and watching this change, which is a different activity from
	// teaching and must not require starting a demonstration to see.
	Here *HerePlace `json:"here,omitempty"`
	// Trail is the ordered walk of places this session has passed through, bounded.
	//
	// Derived from the session.s own crossings -- the canonical ordered record -- so the
	// diagnostic and the evidence cannot disagree about where somebody has been.
	Trail []TrailStep `json:"trail,omitempty"`
	// Asking is every question waiting for an answer, so a surface can offer one. See
	// OpenQuestion: a question nobody can answer is worse than one nobody is asked.
	Asking []OpenQuestion `json:"asking,omitempty"`
	// QuestionsOpen is how many questions are waiting for an answer.
	//
	// The interruption budget is one, and a question already open is why a rehearsal question
	// could not be raised — which presented as "Want me to try?" with no button, three runs
	// running. A person about to demonstrate should be able to see the slot is free.
	QuestionsOpen int `json:"questions_open"`
	// Place is the durable place Marco recognises as where this began.
	Place string `json:"place,omitempty"`
	// Captured is how many actions Marco has taken as demonstration evidence.
	//
	// The number a person checks to answer "did it get my click?" — the question that has
	// otherwise needed a terminal. Read from the producer's own account, so it counts what
	// was ADMITTED rather than what a later layer managed to interpret.
	Captured int `json:"captured"`
	// Targets is how many of those landed on a control Marco could name, and Unnamed how
	// many landed on one whose name is withheld. Kept apart because they mean opposite
	// things about whether the demonstration can be reproduced.
	Targets int `json:"targets"`
	Unnamed int `json:"unnamed,omitempty"`
	// Offered is how many controls Marco could see to aim at, so "nothing captured" can be
	// told from "nothing was on offer".
	Offered int `json:"offered,omitempty"`
	// Route is the transition Marco read out of the demonstration, empty until it has one.
	Route string `json:"route,omitempty"`
	// Steps is the demonstrated route leg by leg, with how far each review has got.
	Steps []RouteStepView `json:"steps,omitempty"`
	// Verified and Required are how many of the demonstrated route.s legs are verified, and
	// how many there are. "Verified 1 / 2" is the panel.s honest account of a two-hop task
	// with one leg still unreviewed.
	Verified int `json:"verified_edges,omitempty"`
	Required int `json:"required_edges,omitempty"`
	// RouteStatus is what the route as a whole amounts to: unreviewed, partial or verified.
	RouteStatus string `json:"route_status,omitempty"`
	// Goal is what a saved play would be called.
	Goal string `json:"goal,omitempty"`
	// Learned says a durable play exists — the ONLY basis for claiming anything was learned.
	Learned bool `json:"learned,omitempty"`
	// Refused is the closed reason, and Detail the diagnostics behind it.
	Refused teach.Refusal `json:"refused,omitempty"`
	Detail  []string      `json:"detail,omitempty"`
	// CanStop, CanTry and CanCancel say which controls apply right now, so a surface does
	// not have to re-derive the lifecycle from the stage.
	CanStop bool `json:"can_stop"`
	// CanName says the person is being asked to name a screen, and may answer or skip.
	CanName   bool `json:"can_name"`
	CanTry    bool `json:"can_try"`
	CanCancel bool `json:"can_cancel"`
	// Question and QuestionID address an open question, when there is one.
	// QuestionID addresses an open question; the wording belongs to the proposal.
	QuestionID observe.ProposalID `json:"question_id,omitempty"`
	// Places is what Marco knows about where it has been, and what those places are
	// called. Present so a person can correct a name without a question being open.
	Places []KnownPlace `json:"places,omitempty"`
	// Waiting is the place an authorised rehearsal is waiting to see, grounded in words.
	//
	// # The failure this closes
	//
	// Marco says "I'll try it when we're back there" and then waits forever, because "there"
	// is a durable place the person is standing on a TWIN of — one Settings page recorded
	// twice, differing by three buttons. Nothing on the surface said which place was meant or
	// that Marco believed the person was somewhere else, so the only way to find out was to
	// read semantic-memory.json by hand. Which is what happened.
	//
	// The same rule as naming, one screen later: Marco may not refer the Audience to a place
	// it cannot ground for them. See [[ADR-069-a-name-is-authored-and-can-be-taken-back]].
	Waiting *KnownPlace `json:"waiting,omitempty"`
	// Elsewhere is where Marco last recognised the person, when that is NOT the place it is
	// waiting for. Nil when they agree, or when Marco has not settled on anything.
	//
	// Two grounded descriptions side by side are what make the situation readable: "waiting
	// for a screen with 13 buttons, you are on one with 10" is a sentence somebody can act
	// on. One of them alone is not.
	Elsewhere *KnownPlace `json:"elsewhere,omitempty"`
	// Naming is the place an open naming question is ABOUT, so the surface can show which
	// screen it means rather than asking about an unidentifiable one.
	Naming    *KnownPlace       `json:"naming,omitempty"`
	SessionID observe.SessionID `json:"session_id,omitempty"`
}

// learnViewOf projects one session.
func learnViewOf(s teach.Session, running, finishing bool) learnView {
	v := learnView{
		Running:   running,
		Stage:     stageOf(s, finishing),
		Name:      s.Name,
		Saying:    s.Say(),
		Watching:  s.Application,
		Place:     s.Start,
		Captured:  s.Input.Classified,
		Targets:   s.Input.PointerResolved,
		Unnamed:   s.Input.PointerUnnamed,
		Offered:   s.Input.ControlsOffered,
		Refused:   s.Refusal,
		SessionID: s.SessionID,
	}
	// THE WINDOW IS FIXED once the coordinator is past waiting for a subject.
	//
	// Derived from the phase rather than plumbed from the selector, because the phase IS the
	// fact: WaitingForDemonstration means AwaitSubject has not returned, and every phase after
	// it runs against a window that was settled on and will not change. A second source for
	// the same truth could disagree with this one.
	//
	// Deleting this must fail TestThePanelSaysWhetherTheTargetIsLocked.
	v.TargetLocked = v.Stage != LearnIdle && v.Stage != LearnWaitingToStart
	if s.Route.From != "" || s.Route.To != "" {
		v.Route = string(s.Route.From) + " → " + string(s.Route.To)
	}
	if s.Actor != "" && s.Verb != "" {
		v.Goal = s.Actor + "'s " + s.Verb
	}
	if s.Saved != nil {
		v.Learned = true
	}
	// The question is ADDRESSED, never paraphrased here: the proposal owns its own wording,
	// and a surface fetches it the same way every other reader does.
	if q := s.Question; q != nil {
		v.QuestionID, v.SessionID = q.ID, q.SessionID
	}
	// THE DIAGNOSTICS ARE SHOWN WHENEVER THE PERSON CANNOT ACT.
	//
	// They used to appear only on Refused, which withheld them in the one state where they
	// were needed most: Marco saying "Want me to try?" with nothing able to take the answer.
	// The coordinator records exactly why — "a yes created no authority: …" — and a person
	// staring at a button that does nothing had no way to reach it.
	//
	// Deleting the deadEnd clause must fail TestADeadEndSaysWhyInsteadOfOfferingAButton.
	// A QUESTION MARCO CANNOT TAKE AN ANSWER TO IS NOT ASKED.
	//
	// The coordinator's own sentence is "I think I got it. Want me to try?", and it is right
	// about the phase and wrong about the world: there is no open proposal, so nothing can
	// accept a yes. Withholding the button without withholding the question was the worse
	// half of a fix — a person reads an offer, finds no way to take it, and has nothing to
	// go on. Better a button that fails than a question with no button; better than both is
	// not to ask.
	//
	// The projection composes this rather than the panel, for the same reason it composes
	// everything else: a surface that rewrote Marco's sentences would be deciding what Marco
	// means. Here the view knows something the coordinator does not — that the tail has no
	// question to offer — and saying so is its job.
	//
	// Deleting this must fail TestADeadEndDoesNotAskAQuestionItCannotTake.
	if deadEnd(s, v.Stage) {
		v.Saying = "I think I got it, but I can't ask you for permission right now."
	}
	// WHAT AN ATTEMPT DID IS READABLE FOR AS LONG AS THE SESSION LIVES.
	//
	// Not gated on the stage. The gate was added one state at a time — refused, then the
	// dead end, then the patient wait — and each time it turned out to be hiding the answer
	// in some state nobody had listed yet. It happened four times. The fourth was `naming`:
	// a rehearsal ran, stopped after its first step, and the panel showed nothing about it
	// because the session had moved on to asking for a name.
	//
	// An attempt that ran is a fact. There is no state in which "what did the attempt do"
	// stops being worth answering, and enumerating the states where it is allowed to be
	// answered has been wrong every single time.
	//
	// Deleting the s.Attempt clause must fail TestAnAttemptIsReadableWhateverTheStage.
	if s.Attempt != nil {
		v.Detail = append(v.Detail, attemptDetail(s.Attempt)...)
	}
	if s.Phase == teach.Refused || blocked(s, v.Stage) {
		// The coordinator's own account of why it is stuck. The ATTEMPT is reported above,
		// unconditionally — `stopped_at_step` is true and useless without the step lines,
		// and which stage the session happens to be in has nothing to do with whether they
		// are worth reading.
		v.Detail = append(v.Detail, s.Diagnostics...)
	}
	applyControls(&v, s)
	return v
}

// applyControls decides which controls a surface may offer, once, for every surface.
//
// Stop is offered while there is anything the person can end. Try is offered only when the
// coordinator is actually waiting for permission — an authority decision is never inferred from a
// stage that merely looks ready.
func applyControls(v *learnView, s teach.Session) {
	switch v.Stage {
	case LearnWaitingToStart, LearnLearning, LearnNeedsAnother:
		v.CanStop, v.CanCancel = true, true
	case LearnFinishing, LearnTrying, LearnWaitingToTry:
		v.CanCancel = true
		// AND STOP, because a review can be waiting on something the person cannot supply.
		//
		// The route review walks legs one at a time and waits for an answer to each. A leg
		// whose question was retracted, refused for budget, or never raised leaves the panel
		// saying "waiting for your answer" with no question anywhere and no control to press
		// — measured live, twice. Cancel was the only way out, and cancel throws the whole
		// episode away including legs that verified.
		//
		// Stop means the same thing here as it does during the demonstration: the person
		// says they are done. What verified is kept; what did not is reported as unreviewed.
		//
		// Deleting this must fail TestAReviewCanAlwaysBeEndedByThePerson.
		v.CanStop = true
	case LearnReadyToTry:
		// TRY IS OFFERED ONLY WHEN THERE IS SOMETHING TO ANSWER.
		//
		// The stage says Marco is waiting for permission; the QUESTION is what permission
		// gets given to. A session can sit in this phase with no open question — the
		// proposal was answered and the yes created no authority, or it was raised against
		// a route that has since moved — and then the button's only possible outcome is
		// "Marco has not offered to try anything yet".
		//
		// Live, that read as "stuck on Want me to try?": press, get refused for 700ms,
		// watch the poll paint the same question back. An offer that cannot be accepted is
		// worse than no offer, because the person keeps pressing it.
		//
		// Deleting `!deadEnd(...)` must fail TestADeadEndSaysWhyInsteadOfOfferingAButton.
		v.CanTry, v.CanCancel = !deadEnd(s, v.Stage), true
	case LearnNaming:
		v.CanName, v.CanCancel = true, true
	}
}

// deadEnd reports that Marco is asking for permission nothing can accept.
//
// One shape, and it is the one seen live: the coordinator is waiting for a rehearsal grant and
// there is no open question to answer. `awaitGrant` will eventually time this out into an honest
// refusal, but until it does the surface shows an offer, and every press comes back
// "Marco has not offered to try anything yet".
//
// Deliberately narrow. It does not guess at other stages and it does not decide anything — it
// only says that a specific offer cannot be taken up, so the surface can withhold the button and
// show the reason instead.
func deadEnd(s teach.Session, stage LearnStage) bool {
	return stage == LearnReadyToTry && s.Question == nil
}

// blocked reports that Marco is stuck on something the person cannot see.
//
// Two shapes, both found live, both of which look identical from the outside — a stage that does
// not change and a sentence that does not change:
//
//   - a dead end: permission offered with nothing able to accept it;
//   - a patient wait: an authorised rehearsal re-attempting every cycle and refusing every cycle,
//     because Marco cannot see the screen the route begins on.
//
// The coordinator records the reason in both — "a yes created no authority: …", "waiting for the
// start: …" — and the view used to withhold diagnostics unless the phase was Refused, which
// neither of these is. So the one place a person needed the reason was the one place it was kept
// from them, twice, in two different states, on two consecutive live runs.
//
// Deleting the patient-wait clause must fail TestAPatientWaitSaysWhatItIsRefusing.
func blocked(s teach.Session, stage LearnStage) bool {
	return deadEnd(s, stage) || stage == LearnWaitingToTry
}

// idleLearnView is what a surface shows when nothing is being taught.
func idleLearnView() learnView { return learnView{Stage: LearnIdle} }

// Learning is the request handler behind a control surface's Learn panel.
//
// It reads the SAME coordinator every other surface reads. There is no second lifecycle here: a
// surface that could learn something the command line could not would be a second implementation
// of the thing this whole subsystem is careful to have exactly one of.
func (r *Runtime) Learning() learnView {
	if r == nil || r.teach == nil {
		return idleLearnView()
	}
	s, ok := r.teach.read()
	if !ok {
		// Nothing is being taught, and a person may still want to correct a name.
		v := idleLearnView()
		// WHERE THEY ARE STANDING COUNTS HERE TOO.
		//
		// This branch used to pass no current place at all, so while nothing was being taught
		// the list marked no row as "here" however far somebody walked. That is the branch
		// people actually name screens in — Marco asks for a word mid-episode and there is no
		// way to tell which screen it means, so the sane move is to press Watch, walk the
		// application, and name each place while looking at it. The one affordance that makes
		// that work was live during teaching and dead the rest of the time.
		//
		// Same source as the teaching branch below, and for the same reason: `here` is where
		// the person is now.
		//
		// Deleting the lookup must fail TestHereIsMarkedWhileNothingIsBeingTaught.
		v.Places = r.placesKnown(r.lastObservedApplication(), r.observations.placeNowSubject())
		// READ BEFORE ARMING. Somebody checking the panel is clear before they press
		// Start needs this here, not only once a session exists.
		v.QuestionsOpen = r.openQuestions()
		v.Asking = r.asking()
		return r.withPlace(v)
	}
	v := learnViewOf(s, r.teach.running(), r.teach.finishing())
	v.QuestionsOpen = r.openQuestions()
	v.Asking = r.asking()
	// THE WHOLE demonstrated route, named, with how much of it is verified. Enriched here
	// rather than in learnViewOf because naming a place needs memory, and the projection is
	// deliberately a pure reading of the session.
	r.routeProgress(s, &v)
	// WHAT MARCO KNOWS ABOUT WHERE IT HAS BEEN, always — not only while it is asking.
	// Authorship is not a mode: somebody who realises a screen is misnamed should be able
	// to fix it then, rather than waiting for Marco to raise the subject again.
	// WHERE THE PERSON IS, live — not s.Start.
	//
	// s.Start is the place the demonstration BEGAN on and is deliberately pinned; it is the
	// right answer to "where did this route start" and the wrong answer to "where are you".
	// Passing it here marked the start row as "here" no matter where somebody was standing,
	// so the panel confidently mislabelled the one thing it was added to ground.
	//
	// Deleting this must fail TestHereMeansWhereYouAreNowNotWhereTheDemonstrationBegan.
	here := r.observations.placeNowSubject()
	v.Places = r.placesKnown(s.Application, here)
	// AND WHICH PLACE an open question is about.
	//
	// The failure this closes: two Settings pages produced identical wording, the person
	// named the wrong one, and the word was then reserved against the one they meant. A
	// naming question that cannot say WHICH screen it means is a question nobody can answer
	// correctly except by luck.
	//
	// Deleting this must fail TestANamingQuestionSaysWhichPlaceItMeans.
	v = r.withPlace(v)
	if s.Question != nil && s.Question.Screen != "" {
		for i := range v.Places {
			if v.Places[i].Handle == s.Question.Screen {
				naming := v.Places[i]
				v.Naming = &naming
				break
			}
		}
	}
	// AND WHICH PLACE a patient rehearsal is waiting for, beside where Marco thinks the
	// person actually is.
	//
	// A grant waits for the screen the route BEGINS on, which is not necessarily the place
	// the session last established — and when a screen has been recorded twice, the two are
	// different durable subjects that can never match. Saying both, in words, turns a silent
	// forever-wait into something a person can diagnose.
	//
	// Deleting this must fail TestAPatientRehearsalSaysWhichPlaceItIsWaitingFor.
	if v.Stage == LearnWaitingToTry {
		v.Waiting = placeIn(v.Places, s.Route.From)
		// Compared against the LIVE place. Against s.Start this could never fire — the
		// demonstration necessarily began on the route's start — so the warning that was
		// supposed to explain a forever-wait was structurally unreachable.
		if p := placeIn(v.Places, here); p != nil && here != s.Route.From {
			v.Elsewhere = p
		}
	}
	return v
}

// placeIn is one known place by handle, nil when there is no such place.
//
// Nil rather than a zero KnownPlace: a surface must be able to tell "the place Marco means" from
// "Marco means a place it can no longer describe", and an empty description renders as a blank
// line that reads like neither.
func placeIn(places []KnownPlace, handle string) *KnownPlace {
	if handle == "" {
		return nil
	}
	for i := range places {
		if places[i].Handle == handle {
			p := places[i]
			return &p
		}
	}
	return nil
}

// trimName is the person's phrase, bounded and stripped.
//
// Bounded because it arrives from a web form and ends up in a session, a goal and possibly a play
// name. The naming rules themselves live in playNameFor and are not restated here.
func trimName(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > MaxLearnNameLength {
		s = s[:MaxLearnNameLength]
	}
	return s
}

// MaxLearnNameLength bounds what a surface may submit as a name.
const MaxLearnNameLength = 120

// Learn is the control surface's whole interface to the teaching lifecycle.
//
// # One rule governs this function
//
// Every verb turns into a request the Director already serves, made in the ordinary way. Start is
// a teach. Stop is a finish. Try is an ANSWER to the rehearsal question — the same answer the
// command line gives, through the same ledger, producing the same one-attempt grant. Cancel is a
// cancel.
//
// Nothing here reaches past those into a pass, a capture, a store or an authority. If a surface
// could learn something the command line could not, or spend a grant the ledger did not issue,
// this file would be a second implementation of the subsystem it is supposed to be a window onto.
//
// Deleting the answer in the Try branch and calling the rehearsal directly must fail
// TestTryItGoesThroughTheRealAuthorityPath.
func (r *Runtime) Learn(ctx context.Context, q service.ObserveLearn) (learnView, error) {
	if r == nil || r.teach == nil {
		return idleLearnView(), fmt.Errorf("this Director cannot be taught")
	}
	switch {
	case q.Start:
		name := trimName(q.Name)
		if name == "" {
			return r.Learning(), fmt.Errorf("say what you want Marco to learn first")
		}
		// TEACHING TAKES THE OBSERVATION SLOT BACK FROM LIGHT MODE.
		//
		// One session runs at a time. Adding Watch broke Start outright: the Light Mode
		// session held the slot and teaching came back "observation session observe_2 is
		// already running; cancel it before starting another" — true, unhelpful, and
		// blaming the person for a conflict Marco created between two of its own features.
		//
		// Watching is an instrument; a demonstration is the work. Somebody pressing Start
		// has said which they want. Only a session Light Mode itself started is yielded.
		//
		// Deleting this must fail TestTeachingTakesTheSlotBackFromLightMode.
		r.yieldWatching()
		// Surface: true is the whole difference between this and `director teach`. It
		// says the request came from Marco's own UI, so the window it came from is
		// Marco's and the session waits for a real subject rather than pinning the
		// control panel the person just clicked. See ObserveTeach.Surface.
		if _, err := r.Teaching(ctx, service.ObserveTeach{Name: name, Surface: true}); err != nil {
			return r.Learning(), err
		}
	case q.Stop:
		// STOP BEFORE ANYTHING WAS DEMONSTRATED IS ABANDONMENT, NOT COMPLETION.
		//
		// Finish means "that was the whole demonstration": it ends the pass that is
		// running and keeps what it saw. While the session is still waiting for a window
		// there IS no pass, so Finish reaches observations.Cancel("") — which finds
		// nothing running and does nothing at all. The session stays in the wait, the
		// panel keeps saying "go ahead", and the person cannot start again because the
		// old attempt never ended.
		//
		// Live: somebody pressed Stop, watched nothing happen, and was stuck. There is
		// nothing to keep before the first pass, so the only honest reading of Stop here
		// is the one that gets them out.
		//
		// Deleting this must fail TestStopBeforeAnythingWasShownActuallyStops.
		verb := service.ObserveTeach{Finish: true}
		if s, ok := r.teach.read(); ok && s.Phase == teach.WaitingForDemonstration {
			verb = service.ObserveTeach{Cancel: true}
		}
		if _, err := r.Teaching(ctx, verb); err != nil {
			return r.Learning(), err
		}
	case q.Cancel:
		if _, err := r.Teaching(ctx, service.ObserveTeach{Cancel: true}); err != nil {
			return r.Learning(), err
		}
	case q.Try:
		if err := r.tryIt(); err != nil {
			return r.Learning(), err
		}
	case q.Remember:
		if err := r.rememberHere(q.Called); err != nil {
			return r.Learning(), err
		}
	case q.Watch:
		if err := r.watchHere(); err != nil {
			return r.Learning(), err
		}
	case q.Unwatch:
		if err := r.stopWatching(); err != nil {
			return r.Learning(), err
		}
	case q.Answer != "":
		// ANSWERING one of Marco.s own questions, through the same request the command
		// line makes. The panel used to report these and offer nothing.
		if err := r.answerQuestion(q.Question, q.Session, q.Answer); err != nil {
			return r.Learning(), err
		}
	case q.Rename:
		// AUDIENCE AUTHORSHIP, with no question pending. A person deciding what a
		// place is called, or that it is called nothing after all.
		if err := r.renamePlace(q.Place, q.Called); err != nil {
			return r.Learning(), err
		}
	case q.Called != "":
		if err := r.callIt(trimName(q.Called)); err != nil {
			return r.Learning(), err
		}
	case q.Skip:
		if err := r.skipIt(); err != nil {
			return r.Learning(), err
		}
	}
	return r.Learning(), nil
}

// tryIt answers the rehearsal question with a yes, through the ordinary ledger.
//
// # Why this is an answer and not a rehearsal
//
// Because the question is what carries the authority. "Shall I have a go?" is a proposal in the
// ledger; answering it yes is what mints exactly one grant, scoped to one application, one route,
// one candidate digest and one attempt. A button that called the rehearsal directly would be
// input without a grant — the one thing the whole mechanism exists to make impossible — and it
// would look like it worked.
//
// It also gets patience for free. The coordinator holds a granted rehearsal in WaitingForStart
// until it can actually see the screen the route begins on, so pressing Try while Marco itself is
// in front costs nothing: the permission is not spent, and the attempt happens when the person
// goes back to their application.
func (r *Runtime) tryIt() error {
	s, ok := r.teach.read()
	if !ok {
		return fmt.Errorf("nothing is being taught")
	}
	q := s.Question
	if q == nil {
		return fmt.Errorf("Marco has not offered to try anything yet")
	}
	_, err := r.Observation(service.ObserveQuery{
		ID: string(q.SessionID),
		Answer: &service.ObserveAnswer{
			ProposalID: string(q.ID), Response: string(observe.ResponseConfirmed),
		},
	})
	return err
}

// callIt answers a naming question with the person's own word.
//
// Routed to the SAME ObserveScreenName request the command line makes, so the raw string becomes
// a validated ScreenName at the one request boundary where human text is converted — not here,
// and not in the browser.
func (r *Runtime) callIt(called string) error {
	q, err := r.openQuestion()
	if err != nil {
		return err
	}
	if called == "" {
		return fmt.Errorf("say what you call it, or skip")
	}
	_, err = r.Observation(service.ObserveQuery{
		ID:   string(q.SessionID),
		Name: &service.ObserveScreenName{ProposalID: string(q.ID), Name: called},
	})
	return err
}

// skipIt declines the open question without answering it.
//
// `not now` is its own answer and is never folded into `no`: a person who would rather not name a
// screen has not told Marco its interpretation was wrong.
func (r *Runtime) skipIt() error {
	q, err := r.openQuestion()
	if err != nil {
		return err
	}
	_, err = r.Observation(service.ObserveQuery{
		ID: string(q.SessionID),
		Answer: &service.ObserveAnswer{
			ProposalID: string(q.ID), Response: string(observe.ResponseDeclined),
		},
	})
	return err
}

// openQuestion is the question the session is currently waiting on.
func (r *Runtime) openQuestion() (*teach.Question, error) {
	s, ok := r.teach.read()
	if !ok {
		return nil, fmt.Errorf("nothing is being taught")
	}
	if s.Question == nil {
		return nil, fmt.Errorf("Marco is not waiting on an answer")
	}
	return s.Question, nil
}

// ── what Marco knows about where it has been, and what it is called ───────────

// KnownPlace is one durable place, as a person would read it.
//
// No subject id, no signature, no roles, no terms. A person renaming a screen is answering "which
// of these do you mean", and `subj_543793ccc326` is not an answer to that — it is the thing the
// naming failure was made of. The id travels as an opaque handle the surface hands back, never as
// something to read.
type KnownPlace struct {
	// Handle addresses this place in a later request. Opaque, never shown.
	Handle string `json:"handle"`
	// Called is the Audience's own word for it, empty when nobody has named it.
	Called string `json:"called,omitempty"`
	// Describes is how Marco would describe it to somebody who has to pick it out — what
	// it is made of, in plain words. Never a durable claim and never a name.
	//
	// DIAGNOSTIC. A surface showing this to a person in place of a name is making them read
	// Marco.s evidence to work out where they are; use Words.
	Describes string `json:"describes"`
	// Words is what to CALL this place in a sentence somebody reads.
	//
	// The canonical presentation, from observe.PlaceWords and nowhere else: the Audience.s
	// own word, then what Marco worked out, then the description as a floor. Every surface
	// reads this, so two of them cannot name the same screen differently.
	Words string `json:"words"`
	// Here says this is the place in front right now, so a person renaming what they are
	// looking at can see which row that is.
	Here bool `json:"here,omitempty"`
	// Targets is how many things Marco knows it can act on here.
	Targets int `json:"targets,omitempty"`
}

// placesKnown is every durable place in this application, with what it is called.
//
// Ordered: named places first, then by description, so the list a person reads is stable between
// refreshes and the ones they have already told Marco about are together.
func (r *Runtime) placesKnown(application, current string) []KnownPlace {
	if r == nil || r.observations == nil || r.observations.memory == nil {
		return nil
	}
	lister, ok := r.observations.memory.(interface {
		Subjects() []observe.RememberedSubject
	})
	if !ok {
		return nil
	}
	all := lister.Subjects()
	var out []KnownPlace
	for _, s := range all {
		if s.Structure.Subject != observe.SubjectState {
			continue
		}
		if application != "" && !strings.EqualFold(s.Application, application) {
			continue
		}
		out = append(out, KnownPlace{
			Handle:    s.ID,
			Called:    s.Called,
			Describes: describePlace(s),
			Words:     observe.PlaceWords(s),
			Here:      s.ID == current && current != "",
			Targets:   len(observe.TargetsGroundedIn(all, s.Application, s.ID)),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Called != "") != (out[j].Called != "") {
			return out[i].Called != ""
		}
		if out[i].Called != out[j].Called {
			return out[i].Called < out[j].Called
		}
		return out[i].Describes < out[j].Describes
	})
	return out
}

// describePlace is how Marco would point at a screen for somebody who has to tell it from
// another one.
//
// # Why this exists rather than showing the id
//
// Because the whole naming failure was a person being asked about a place they could not
// identify. Two Settings pages produced identical wording, they named the wrong one, and the word
// was then reserved forever.
//
// It describes STRUCTURE — what the screen is made of — because that is what distinguishes two
// places and is already durable. It does not read the screen: no captured text reaches here, and
// the interface terms it does use are the closed vocabulary, not anything anybody typed.
func describePlace(s observe.RememberedSubject) string {
	var parts []string
	if terms := s.Structure.Terms; len(terms) > 0 {
		words := make([]string, 0, len(terms))
		for _, t := range terms {
			words = append(words, string(t))
		}
		sort.Strings(words)
		parts = append(parts, "about "+strings.Join(words, ", "))
	}
	if n := totalRoles(s.Structure.Roles); n > 0 {
		parts = append(parts, fmt.Sprintf("%d things on it", n))
	}
	if len(parts) == 0 {
		return "a screen Marco recognises"
	}
	return strings.Join(parts, ", ")
}

func totalRoles(roles map[string]int) int {
	n := 0
	for _, v := range roles {
		n += v
	}
	return n
}

// renamePlace gives a place the Audience's word for it, or takes that word back.
//
// # Why an empty name is a retraction rather than an error
//
// Because taking a name back is one of the three things authorship has to allow, and it was the
// missing one. A person who realises they named the wrong screen has to be able to undo it —
// otherwise the word is reserved against the screen they actually meant, and the only repair is
// editing the store by hand. Which is what happened.
//
// Deleting the empty-name branch must fail TestTakingANameBackThroughTheSurface.
func (r *Runtime) renamePlace(place, called string) error {
	if strings.TrimSpace(place) == "" {
		return fmt.Errorf("say which place you mean")
	}
	if r.observations == nil || r.observations.memory == nil {
		return fmt.Errorf("this Director has no durable memory")
	}
	author, ok := r.observations.memory.(observe.ScreenAuthor)
	if !ok {
		return fmt.Errorf("this Director cannot change what places are called")
	}
	// WHICH APPLICATION comes from the PLACE, not from whatever session happens to be
	// running. A handle identifies one durable subject and that subject knows what it
	// belongs to; deriving it from context instead would mean a rename could land in the
	// wrong namespace — or, as it did first time, in no namespace at all — depending on what
	// the person had been doing beforehand. Guessing the referent from ambient state is the
	// exact class of bug this milestone exists to remove.
	application := r.applicationOfPlace(place)
	if application == "" {
		return fmt.Errorf("Marco does not know that place")
	}
	if strings.TrimSpace(called) == "" {
		return author.UnnameSubject(application, place)
	}
	// THE one conversion point for human text, as it has always been. A name becomes a
	// ScreenName here and nowhere else, so an observed label cannot reach durable memory by
	// being assigned to the right variable.
	name, err := observe.UserSuppliedScreenName(called)
	if err != nil {
		return err
	}
	return author.NameSubject(application, place, name)
}

// lastObservedApplication is whichever application was most recently watched.
func (r *Runtime) lastObservedApplication() string {
	if r.observations == nil {
		return ""
	}
	r.observations.mu.RLock()
	defer r.observations.mu.RUnlock()
	for i := len(r.observations.finished) - 1; i >= 0; i-- {
		if app := r.observations.finished[i].Session.Application; app != "" {
			return app
		}
	}
	return ""
}

// applicationOfPlace is which application a durable place belongs to.
//
// Read from the subject rather than from context. A handle names one record, and that record
// already knows its own namespace; asking the session instead would make a rename depend on what
// the person had been doing rather than on what they clicked.
func (r *Runtime) applicationOfPlace(place string) string {
	if r == nil || r.observations == nil || r.observations.memory == nil {
		return ""
	}
	lister, ok := r.observations.memory.(interface {
		Subjects() []observe.RememberedSubject
	})
	if !ok {
		return ""
	}
	for _, s := range lister.Subjects() {
		if s.ID == place {
			return s.Application
		}
	}
	return ""
}

// MaxStepsShown bounds how much of an attempt is reported.
//
// A reading, not a trace. A route long enough to overflow this is a route whose failure is not
// going to be understood from a list anyway.
const MaxStepsShown = 8

// attemptDetail is what a rehearsal did, step by step, in the terms the attempt already uses.
//
// Empty when nothing ran or nothing went wrong: a successful attempt needs no explanation, and
// printing one under a working rehearsal would read as a fault.
//
// Closed vocabulary and counts only — the same rule teach.AttemptStep already follows. No keys,
// no labels, no coordinates. What travels is what a step EXPECTED, what it OBSERVED, and how it
// came out, which is the difference between "Marco did the wrong thing" and "Marco did the right
// thing and could not tell".
func attemptDetail(a *teach.Attempt) []string {
	if a == nil {
		return nil
	}
	var out []string
	if a.Detail != "" {
		out = append(out, "the attempt says: "+a.Detail)
	}
	for _, st := range a.Steps {
		if len(out) >= MaxStepsShown {
			out = append(out, fmt.Sprintf("… and %d more step(s)",
				len(a.Steps)-MaxStepsShown))
			break
		}
		line := fmt.Sprintf("step %d: %s", st.Step, st.Outcome)
		if st.Expected != "" || st.Observed != "" {
			line += fmt.Sprintf(" — expected %s, saw %s",
				orElseWord(st.Expected, "nothing in particular"),
				orElseWord(st.Observed, "nothing"))
		}
		// WHAT THE HOST SAID, when it said anything. `input_failed` names the kind of
		// problem; this names which one. Live, a step reported input_failed with the
		// reason already computed one layer down and dropped on the floor.
		//
		// Deleting this must fail TestAFailedInputSaysWhatTheHostSaid.
		if st.Detail != "" {
			line += " (" + st.Detail + ")"
		}
		out = append(out, line)
	}
	return out
}

// orElseWord is a value or a stand-in, so a blank never renders as an empty gap.
func orElseWord(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

// openQuestions is how many questions are waiting for an answer, across every session.
//
// The interruption budget is ONE (observe.DefaultProposalThresholds), so this is very nearly a
// boolean — and it is the difference between a Learn run that can be asked about what it just saw
// and one that cannot. A question left open by an earlier pass is why a rehearsal question could
// not be raised, which presented as "Want me to try?" with no button, three live runs running.
//
// Counted across FINISHED sessions too, because that is where they live: a question outlives the
// session that raised it, which is the ordinary case rather than an edge one.
//
// Deleting this must fail TestThePanelSaysHowManyQuestionsAreOpen.
func (r *Runtime) openQuestions() int {
	if r == nil || r.observations == nil {
		return 0
	}
	g := r.observations
	g.mu.RLock()
	runner := g.active
	finished := make([]observesession.Result, len(g.finished))
	copy(finished, g.finished)
	g.mu.RUnlock()

	// COUNTED BY IDENTITY, exactly as `asking` lists them.
	//
	// The count and the list are read side by side — "Questions open: 3" above one question
	// reads as two of them having gone missing. See `asking` for why the copies exist.
	seen := map[observe.ProposalID]bool{}
	count := func(ps []observe.Proposal) {
		for _, p := range ps {
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
		}
	}
	if runner != nil {
		count(runner.Proposals().Open())
	}
	for _, res := range finished {
		count(res.Proposals.Open())
	}
	return len(seen)
}

// OpenQuestion is one question waiting for an answer, addressed so it can be given one.
//
// # Why the panel has to show these
//
// Because Marco raises them, blocks on them, and counts them at the person — and until now
// offered no way to answer any of them. A teach pass asks semantic questions of its own ("are
// these one set?"), those questions hold the interruption budget, and the panel reported
// "Questions open: 3" beside a rehearsal that could not be offered. The person could see the
// obstacle and could not touch it.
//
// A question nobody can answer is worse than a question nobody is asked. This is the same rule as
// the dead-end offer and the unreadable refusal, one layer further in: if Marco tells somebody
// about a thing, it owes them a way to act on it.
type OpenQuestion struct {
	// ID addresses the proposal an answer settles. Opaque; never shown.
	ID observe.ProposalID `json:"id"`
	// SessionID routes the answer to the session that raised it.
	SessionID observe.SessionID `json:"session_id"`
	// Question is the proposal's OWN wording. Never paraphrased here — the proposal owns
	// what it is asking, exactly as the naming question does.
	Question string `json:"question"`
	// Naming says this one wants a word rather than a yes or a no, so a surface does not
	// offer Yes/No against a question that has no such answer.
	Naming bool `json:"naming,omitempty"`
}

// asking is every question waiting for an answer, oldest first.
//
// Ordered oldest-first because that is the order they were asked in, and because the one holding
// up everything else is usually the one raised earliest.
//
// Deleting this must fail TestThePanelOffersAWayToAnswerAnOpenQuestion.
func (r *Runtime) asking() []OpenQuestion {
	if r == nil || r.observations == nil {
		return nil
	}
	g := r.observations
	g.mu.RLock()
	runner := g.active
	finished := make([]observesession.Result, len(g.finished))
	copy(finished, g.finished)
	g.mu.RUnlock()

	var out []OpenQuestion
	// ONE QUESTION PER IDENTITY, and the NEWEST session's copy is the one offered.
	//
	// # Why the same question arrives many times
	//
	// A rehearsal proposal's identity is its route, and a ledger dedupes on it — so one
	// session asks once. The ledger belongs to a session; the question belongs to a route,
	// which outlives it. A multi-pass episode therefore mints a copy per pass, and this list
	// aggregates every session, so the person reads:
	//
	//	Questions open: 3
	//	I've watched getting from Bluetooth to Mouse twice … Shall I have a go?
	//	I've watched getting from Bluetooth to Mouse twice … Shall I have a go?
	//	I've watched getting from Bluetooth to Mouse twice … Shall I have a go?
	//
	// The same identity IS the same question, so it is shown once. Deduping here rather than
	// suppressing it upstream is deliberate: a question raised only in an older session is
	// visible and UNANSWERABLE, because the yes that creates authority is applied by the
	// newest runner and it looks the proposal up in its own ledger. Every pass minting a copy
	// is what keeps the question answerable, so the copies must exist and must be counted
	// once.
	//
	// Newest wins for the same reason: that is the copy an answer can reach.
	seen := map[observe.ProposalID]int{}
	add := func(session observe.SessionID, ps []observe.Proposal) {
		for _, p := range ps {
			q := OpenQuestion{
				ID: p.ID, SessionID: session, Question: p.Question,
				Naming: p.Ask == observe.AskNameScreen,
			}
			if at, dup := seen[p.ID]; dup {
				out[at] = q
				continue
			}
			if len(out) >= MaxQuestionsShown {
				return
			}
			seen[p.ID] = len(out)
			out = append(out, q)
		}
	}

	for _, res := range finished {
		add(res.Session.ID, res.Proposals.Open())
	}
	if runner != nil {
		session, _ := runner.Snapshot()
		add(session.ID, runner.Proposals().Open())
	}
	return out
}

// MaxQuestionsShown bounds the list.
//
// The interruption budget is one, so more than a handful means something has gone wrong upstream
// rather than that somebody has a lot of answering to do.
const MaxQuestionsShown = 8

// answerQuestion settles one open question with a closed response.
//
// The SAME request the command line makes. There is no second answering path here: an answer
// given through the panel and an answer given through `director observe --answer` reach the
// ledger identically, or the two surfaces could disagree about what somebody said.
//
// Deleting this must fail TestAnAnswerFromThePanelReachesTheLedger.
func (r *Runtime) answerQuestion(id, session, response string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("say which question you are answering")
	}
	// THE RESPONSE VOCABULARY IS NOT CHECKED HERE.
	//
	// It is closed, and the ledger already enforces it — "maybe" comes back as
	// `"maybe" is not an answer; say confirmed, contradicted or declined`, which names the
	// three words and is better than anything this layer would say. A copy of the check here
	// was written first and was unfalsifiable: every mutation of it survived, because the
	// canonical refusal arrived anyway.
	//
	// A second copy of a closed vocabulary is a second thing to keep in agreement. The
	// behaviour is held by TestAnAnswerFromThePanelReachesTheLedger, which asserts the
	// refusal a person actually gets rather than which layer produced it.
	_, err := r.Observation(service.ObserveQuery{
		ID:     session,
		Answer: &service.ObserveAnswer{ProposalID: id, Response: response},
	})
	return err
}

// ── the demonstrated route, leg by leg ────────────────────────────────────────

// RouteStepView is one leg of the demonstrated route, as a person should read it.
//
// Places by NAME, never by subject id. An id is an internal handle and putting one in front of
// somebody is asking them to debug Marco rather than read it.
type RouteStepView struct {
	// From and To are what the two screens are called, or how Marco would describe them
	// when nobody has named them yet.
	From string `json:"from"`
	To   string `json:"to"`
	// Status is how far this leg's review has got, in Teach's own vocabulary.
	Status string `json:"status"`
	// Why is the reason behind a leg that ended without being verified.
	Why string `json:"why,omitempty"`
}

// placeWords is what to call one durable place: the Audience's word, else Marco's description.
//
// Falls back to a short honest phrase rather than the id, because an id shown to a person is not
// information — it is Marco failing to answer and hoping nobody notices.
func (r *Runtime) placeWords(application, subject string) string {
	if subject == "" {
		return "somewhere"
	}
	for _, p := range r.placesKnown(application, "") {
		if p.Handle != subject {
			continue
		}
		// ONE naming function. This used to read Called then Describes, which is a second
		// one — and it silently skipped the rung between them, so a Place Marco had
		// correctly inferred a name for was still presented as "about back, settings, 96
		// things on it" everywhere this route line appears.
		//
		// Deleting the Words branch must fail TestTheRouteLineUsesTheCanonicalName.
		if p.Words != "" {
			return p.Words
		}
		break
	}
	return "a screen with no name yet"
}

// routeProgress renders the demonstrated route and how much of it is verified.
//
// # Why the panel needs this
//
// A demonstration of Home → Bluetooth → Mouse is two reusable edges, and BOTH have to be reviewed
// before the goal is reachable from the start. Live, the second leg was never offered and the
// episode ended silently, so a person was told the route was learned when one step of it had
// never been tried. "Verified 1 / 2" is the difference between that and the truth.
//
// Deleting this must fail TestThePanelSaysHowMuchOfTheRouteIsVerified.
func (r *Runtime) routeProgress(s teach.Session, v *learnView) {
	if len(s.Edges) == 0 {
		return
	}
	for _, e := range s.Edges {
		v.Steps = append(v.Steps, RouteStepView{
			From:   r.placeWords(s.Application, string(e.Route.From)),
			To:     r.placeWords(s.Application, string(e.Route.To)),
			Status: string(e.Status),
			Why:    e.Why,
		})
	}
	v.Verified, v.Required = s.Verified()
	v.RouteStatus = string(s.Status())
	// The whole walk as one sentence: Home → Bluetooth → Mouse. Built from the ordered legs
	// rather than from the route under review, which is only ever one of them.
	chain := []string{v.Steps[0].From}
	for _, st := range v.Steps {
		chain = append(chain, st.To)
	}
	v.Route = strings.Join(chain, " → ")
}
