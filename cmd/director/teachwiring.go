package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
)

// Where the teaching coordinator meets the live desktop.
//
// The coordinator is pure: it takes an interface that runs one bounded pass and an interface that
// remembers things. Everything platform-shaped is here — the window selector, the sampler, the
// registry that owns the one active observation session — and nothing here decides anything about
// teaching.

// teachPasses runs the coordinator's passes through the ORDINARY observation registry.
//
// Not a second session mechanism. It is the same registry, the same runner, the same sampler and
// the same one-session-at-a-time rule that `observe-game` uses; the only difference is that a
// teach pass is awaited rather than left running, because its conclusion decides what the person
// is asked next.
type teachPasses struct {
	rt       *Runtime
	selector windowref.Selector
	// subject substitutes the foreground question, and nothing else.
	//
	// Nil in production, where it is Runtime.subjectContext. It exists so the settle rule in
	// AwaitSubject can be driven over a scripted sequence of foreground windows — the rule
	// only means anything across several polls, and a test cannot make a real desktop change
	// windows on cue.
	subject func(context.Context) (windowref.Candidate, error)
	// counted says this teaching episode has already claimed its one independent
	// corroboration, so every later pass folds evidence without claiming another.
	//
	// # Why this exists
	//
	// `RememberedRelationship.Sessions` means "how many separate times has this been seen" —
	// separate sittings, separate window generations, separate days — and the invitation
	// policy reads it as real-world recurrence. One teach attempt runs three bounded passes
	// back to back in a single sitting. Counting each of them would let an explicit teach
	// manufacture its own corroboration, and Marco would then offer to learn a habit it has
	// seen exactly once.
	//
	// One teach attempt is one episode. It counts once, at the first pass that actually
	// contributed a durable edge.
	counted bool
	// run is the pass itself. Nil in production, where it is the observation registry.
	//
	// A seam, and a narrow one: it exists so the episode rule above can be driven through
	// this method rather than restated in a test, because a restated rule is a second rule.
	// What it replaces — RunPass — has its own test.
	run func(ctx context.Context, b observe.Bounds, ep observesession.Episode) (
		observesession.Result, error)
}

var _ teach.Passes = (*teachPasses)(nil)

// sameEpisode reports whether this pass must not claim a further independent sighting.
func (p *teachPasses) sameEpisode() bool { return p.counted }

// episode is what a teach pass declares about itself.
//
// THE one place `EstablishPlaces` is ever set true. A person typed `teach "…"`, named a behaviour
// and asked to be watched doing it; that is the human semantic event that licenses persisting
// where they are standing. Every other session in this Director gets the zero value and persists
// no place at all.
//
// It grants nothing else. A teach pass is still an ordinary bounded observation through the
// ordinary registry, and the place it establishes carries no judgement — see
// observesession.Episode.EstablishPlaces.
//
// Deleting `EstablishPlaces: true` must fail
// TestATeachPassDeclaresTheLicenceToEstablishAPlace.
func (p *teachPasses) episode() observesession.Episode {
	// PermissionExpected, because this IS the person asking. They typed what they wanted,
	// pressed Start and demonstrated it; the question about whether Marco may try is the one
	// they are waiting for, not an interruption of something else.
	//
	// Deleting `PermissionExpected: true` must fail
	// TestATeachPassExpectsToBeAskedForPermission.
	return observesession.Episode{
		SameEpisode: p.sameEpisode(), EstablishPlaces: true, PermissionExpected: true,
	}
}

func (p *teachPasses) Observe(ctx context.Context, d time.Duration) (
	observesession.Result, error) {

	bounds := observe.DefaultBounds()
	bounds.Duration = d
	normalised, err := bounds.Normalise()
	if err != nil {
		return observesession.Result{}, err
	}
	run := p.run
	if run == nil {
		run = p.viaRegistry
	}
	res, err := run(ctx, normalised, p.episode())
	// The episode claims its corroboration once, at the first pass that actually put a
	// durable edge in the store — and every pass after it declares itself part of the same
	// episode.
	//
	// Deleting this must fail TestATeachEpisodeClaimsOneCorroborationThroughTheProductionPass.
	if res.Relationships.Created+res.Relationships.Corroborated > 0 {
		p.counted = true
	}
	return res, err
}

// Finish ends the pass that is running now, keeping everything it saw.
//
// The ordinary cancellation, which the observation layer already defines as "stop early and keep
// the evidence" — the same thing `cancel-observation` does. RunPass returns the finished record,
// so the coordinator's pipeline continues from a shorter pass rather than from nothing.
//
// Not the context: cancelling that abandons the attempt. See teach.Coordinator.Finish.
func (p *teachPasses) Finish() {
	if p.rt == nil || p.rt.observations == nil {
		return
	}
	// The empty id means "whatever is running", which is the right question here: there is
	// one active session at a time and this is the coordinator's own.
	_ = p.rt.observations.Cancel("")
}

// AwaitSubject waits until something that is not Marco is in front, and pins it.
//
// # Why the target is chosen here and not when Start was pressed
//
// Because pressing Start brings Marco to the front. Resolving the foreground at that instant
// pinned Marco's own control surface as the window to be taught about — which then got
// fingerprinted as the place the task starts from, while the person was on their way to the
// application. It is not fixable by asking the person to put the application in front first: the
// button is in Marco, so Marco is always the last thing they touched.
//
// So the choice is deferred to the first moment there is a real answer. The person switches to
// their application in the ordinary way and the demonstration begins.
//
// The window is PINNED once resolved, exactly as it was before. A selector that kept re-asking
// would follow focus onto whatever the person clicked next, halfway through their own
// demonstration.
//
// Deleting this — resolving the foreground at Start instead — must fail
// TestStartingFromMarcosOwnSurfaceWaitsForSomethingElse.
func (p *teachPasses) AwaitSubject(ctx context.Context) error {
	if !p.selector.Zero() {
		// A window was named explicitly. The person has already said what they mean, and
		// second-guessing it would be worse than useless.
		return nil
	}
	if p.run != nil {
		// The pass seam is substituted, which substitutes the DESKTOP — so whatever is
		// being watched is the caller's to decide and there is nothing here to wait for.
		return nil
	}
	if p.rt == nil && p.subject == nil {
		return fmt.Errorf("this Director cannot resolve a window")
	}
	// THE WINDOW HAS TO HOLD STILL BEFORE IT COUNTS.
	//
	// # The live failure
	//
	// Somebody pressed Start and walked to Settings. On the way the foreground was File
	// Explorer for a moment. Marco fixed on Explorer, established the start there, watched it
	// for the whole pass while the demonstration happened in Settings, and reported "I didn't
	// see anything change, so there's nothing for me to learn."
	//
	// Taking the FIRST non-Marco foreground assumes the person teleports to their
	// application. They do not: they press a button in Marco and then navigate, and
	// navigating means passing through whatever is already open. The window they mean is the
	// one they STOP on.
	//
	// So the same rule the rest of this system uses for identity — settled, not merely seen.
	// Cheap, because this is a window-layer question with no sample, no capture and no
	// interpretation behind it.
	//
	// Deleting the stability requirement must fail
	// TestAWindowPassedThroughIsNotTheOneBeingTaught.
	// The foreground question, through a seam, so the settle rule can be exercised without a
	// desktop. Same shape as `run`: what is substituted is the PLATFORM, and nothing else.
	//
	// It answers with the CANDIDATE, not a selector. Directory.Adopt mints a new ephemeral id
	// on every call, so two selectors for the same unmoved window never compare equal — the
	// first version of this settled on selectors and therefore never settled at all. It
	// polled forever, minting an id every 400ms, while somebody stood in Settings watching
	// the panel say Target locked: NO.
	ask := p.subject
	if ask == nil {
		ask = p.rt.foregroundCandidate
	}
	sel, err := awaitSettledWindow(ctx, ask, p.rt.adopt)
	if err != nil {
		return err
	}
	p.selector = sel
	return nil
}

// teachSubjectSettle is how many consecutive polls one window must be in front before it is
// taken to be the one being taught.
//
// Three, at 400ms, so roughly a second of standing still. Long enough that a window crossed on
// the way somewhere is not mistaken for the destination, short enough that somebody who went
// straight there does not notice the wait.
//
// Not a guess at intent: it is the same settled-not-merely-seen rule screen identity already
// uses, applied to which window a person means.
const teachSubjectSettle = 3

// teachSubjectPoll is how often a waiting session re-asks whether the person has gone to their
// application yet.
//
// A window-layer question and nothing else: no sample is taken, no capture runs and nothing is
// interpreted, so this is cheap enough to leave running for as long as somebody needs.
const teachSubjectPoll = 400 * time.Millisecond

// subjectContext is the foreground window, unless that window is Marco.
//
// The one place the ownership rule meets window selection. It refuses rather than substituting a
// guess: which application the person means is theirs to indicate, by going to it.
func (r *Runtime) subjectContext(ctx context.Context) (windowref.Selector, error) {
	if r.surfaceOwnsForeground() {
		return windowref.Selector{}, fmt.Errorf(
			"the window in front is Marco itself, so there is nothing to watch yet")
	}
	return r.currentContext(ctx)
}

// viaRegistry is the production pass: the ordinary registry, the ordinary runner.
func (p *teachPasses) viaRegistry(ctx context.Context, b observe.Bounds,
	ep observesession.Episode) (observesession.Result, error) {

	return p.rt.observations.RunPass(ctx,
		p.rt.newObservationTarget(), p.rt.newObservationSampler(sessionClock),
		observesession.NopEvents{}, p.selector, b, ep)
}

// teaching owns the one teach session this Director will run.
//
// EPHEMERAL, and deliberately so. There is no marshalling of it anywhere and no path that could
// restore one, so a restart ends the attempt rather than resuming a demonstration somebody agreed
// to give an hour ago.
type teaching struct {
	mu      sync.RWMutex
	coord   *teach.Coordinator
	cancel  context.CancelFunc
	session teach.Session
	active  bool
	// selector is the window this session is teaching against, resolved once at start.
	//
	// Held because a later read has to be able to ask where that window is NOW — the freshness
	// check every grounded highlight depends on — and a read carries no target of its own.
	selector windowref.Selector
}

// start begins a teach session, or reports that one is already running.
func (t *teaching) start(name string, passes teach.Passes, memory observe.Memory,
	tail teach.Tail, ground teach.Grounding, sel windowref.Selector,
	actor, verb string) (teach.Session, error) {

	t.mu.Lock()
	if t.active {
		s := t.session
		t.mu.Unlock()
		return s, fmt.Errorf(
			"a teaching session is already running (%s); stop it before starting another",
			s.Phase)
	}
	if memory == nil {
		t.mu.Unlock()
		return teach.Session{}, fmt.Errorf(
			"this Director has no durable memory, so it cannot recognise where you are " +
				"starting and cannot be taught")
	}
	c := teach.New(name, passes, memory, teach.DefaultBounds()).
		WithTail(tail).WithGrounding(ground).WithPlayName(actor, verb)
	ctx, cancel := context.WithCancel(context.Background())
	t.coord, t.cancel, t.session, t.selector = c, cancel, c.Session(), sel
	t.active = !t.session.Phase.Settled()
	first := t.session
	t.mu.Unlock()
	if !t.active {
		// A name Marco cannot use is refused before anything is observed, so there is
		// nothing to run.
		cancel()
		return first, nil
	}

	// The driver. One step at a time, each of which may block for the length of a pass.
	//
	// A WAITING phase is polled rather than driven: advancing it re-asks whether the user has
	// answered the question that already exists, and does nothing else. The coordinator holds
	// the bound on how long that may go on, so a session left open never runs forever.
	go func() {
		defer cancel()
		for {
			s := c.Advance(ctx)
			t.mu.Lock()
			t.session = s
			done := s.Phase.Settled()
			if done {
				t.active = false
			}
			t.mu.Unlock()
			if done {
				return
			}
			if s.Phase.Waiting() {
				select {
				case <-ctx.Done():
					return
				case <-time.After(teachAnswerPoll):
				}
			}
			// The patient case is PACED, not driven flat out. WaitingForStart is not a
			// Waiting() phase — nothing here is blocked on an answer, and advancing it is
			// how the retry happens — but each advance runs a full perception pass, and
			// with no pause between them a person who had not yet brought the window
			// forward was met with a perception loop running as fast as it could sample.
			// The bound on how long the waiting may go on stays where it always was: the
			// grant's own expiry.
			if s.Phase == teach.WaitingForStart {
				select {
				case <-ctx.Done():
					return
				case <-time.After(teachStartPoll):
				}
			}
		}
	}()
	return first, nil
}

// stop ends the session at the user's request.
func (t *teaching) stop() (teach.Session, error) {
	t.mu.Lock()
	c, cancel := t.coord, t.cancel
	t.mu.Unlock()
	if c == nil {
		return teach.Session{}, fmt.Errorf("no teaching session is running")
	}
	// The coordinator first, so the decision is durable for the phases after the in-flight
	// pass, then the context, which stops the pass itself.
	c.Cancel()
	if cancel != nil {
		cancel()
	}
	t.mu.Lock()
	t.session, t.active = c.Session(), false
	s := t.session
	t.mu.Unlock()
	return s, nil
}

// finish is the person saying the demonstration is over.
//
// The session KEEPS RUNNING. Only the capture ends, and everything after it — establishing where
// they finished, building the demonstration, assessing the candidate — runs on what was actually
// seen. Contrast stop, which ends the attempt and keeps nothing.
func (t *teaching) finish() (teach.Session, error) {
	t.mu.RLock()
	c := t.coord
	t.mu.RUnlock()
	if c == nil {
		return teach.Session{}, fmt.Errorf("no teaching session is running")
	}
	c.Finish()
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.session, nil
}

// read returns the session as it stands.
func (t *teaching) read() (teach.Session, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.coord == nil {
		return teach.Session{}, false
	}
	return t.session, true
}

// running reports whether a teach session is under way.
func (t *teaching) running() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.active
}

// teachApplication is which application the teach session turned out to be watching.
//
// Read from the coordinator rather than from the request, because a window may be selected by
// ephemeral id and which application that is is something the first pass discovers.
func (r *Runtime) teachApplication() string {
	if r.teach == nil {
		return ""
	}
	s, _ := r.teach.read()
	return s.Application
}

// teachAnswerPoll is how often a waiting teach session re-asks whether the user has answered.
//
// A read over state the service already holds: no sample, no capture, and no question raised. The
// bound on how long a wait may go on belongs to the coordinator, not here.
const teachAnswerPoll = 750 * time.Millisecond

// teachStartPoll is how long a patient rehearsal pauses between looks at where the user is.
//
// Each look is itself a bounded perception pass, so this is a pause between real work rather
// than a poll of cheap state — two seconds keeps the wait responsive to a person clicking
// back into the window while cutting the sampling to a fraction of a flat-out loop.
const teachStartPoll = 2 * time.Second

// teachView is the safe outward shape of a teaching session.
//
// Two readings from one value, and the rule between them is the whole privacy boundary: `Saying`
// is what a person is told and never names a subject id, a fingerprint or a verdict; `Watching`
// is the evidence underneath and is developer-facing.
type teachView struct {
	Name        string      `json:"name,omitempty"`
	Application string      `json:"application,omitempty"`
	Phase       teach.Phase `json:"phase"`
	// Saying is the Normal-mode line.
	Saying string `json:"saying"`
	// Watching is the Watch-mode panel, present only when it was asked for.
	Watching []string `json:"watching,omitempty"`
	// Refused names the closed reason when the session was refused.
	Refused teach.Refusal `json:"refused,omitempty"`
	// Active says the coordinator is still working; Settled says it is over.
	Active   bool `json:"active"`
	Settled  bool `json:"settled"`
	Waiting  bool `json:"waiting"`
	Examples int  `json:"examples,omitempty"`
	// SessionID and QuestionID address the open question, so a caller can answer it through
	// the command that already accepts answers. Teach adds no shortcut around them.
	SessionID  observe.SessionID  `json:"session_id,omitempty"`
	QuestionID observe.ProposalID `json:"question_id,omitempty"`
	// Learned says a durable play exists. The ONLY basis on which anything may claim it.
	Learned bool `json:"learned,omitempty"`
	// Play is the slug the play was written under, and Registered whether a later request can
	// find it. Saved and registered stay two facts.
	Play       string `json:"play,omitempty"`
	Registered bool   `json:"registered,omitempty"`
	// WillBeCalled is the sentence a saved play WOULD become, present from the first read.
	// Said out loud so a name derived from the person's phrase is one they can see and
	// correct, rather than an identifier welded silently out of their words.
	WillBeCalled string `json:"will_be_called,omitempty"`
	// Grounding is where Marco's two decisions currently are on the display, or why they
	// cannot be shown. Present from the moment a start is established.
	Grounding []groundedView `json:"grounding,omitempty"`
}

func viewTeach(s teach.Session, active, watch bool) teachView {
	v := teachView{
		Name: s.Name, Application: s.Application, Phase: s.Phase, Saying: s.Say(),
		Refused: s.Refusal, Active: active, Settled: s.Phase.Settled(),
		Waiting: s.Phase.Waiting(), Examples: s.Examples, SessionID: s.SessionID,
	}
	if q := s.Question; q != nil {
		v.QuestionID = q.ID
	}
	if s.Actor != "" && s.Verb != "" {
		v.WillBeCalled = s.Actor + "'s " + s.Verb
	}
	// Read from the artifact, never from the phase. A session that says complete and cannot
	// point at a saved play is a session lying to somebody.
	v.Learned = s.Learned()
	if s.Saved != nil {
		v.Play, v.Registered = s.Saved.Name, s.Saved.Registered
	}
	if watch {
		v.Watching = s.Watch()
	}
	return v
}

// Teaching starts, reads or cancels the teaching session.
//
// THE teach call site, reached from Runtime.Observation. Teaching grants no authority: it starts
// bounded observation passes through the registry that already owns that permission, and every
// question it reaches — another example, permission to rehearse, a name for a screen — is put and
// answered through the proposal machinery that existed before it.
//
// Deleting this must fail TestTeachIsReachableThroughTheProductionRequestPath.
func (r *Runtime) Teaching(ctx context.Context, q service.ObserveTeach) (teachView, error) {
	if r.teach == nil {
		return teachView{}, fmt.Errorf("this Director cannot be taught")
	}
	switch {
	case q.Cancel:
		s, err := r.teach.stop()
		if err != nil {
			return teachView{}, err
		}
		r.owner.release()
		return viewTeach(s, false, q.Watch), nil

	case q.Finish:
		// STOP, the product event. The session continues — this ends the capture and lets
		// everything downstream run on what was actually seen.
		s, err := r.teach.finish()
		if err != nil {
			return teachView{}, err
		}
		return r.viewTeaching(ctx, s, q.Watch), nil

	case q.Name != "":
		if r.observations == nil {
			return teachView{}, fmt.Errorf("this Director has no observation registry")
		}
		// MARCO CHOOSES THE WINDOW when nobody named one.
		//
		// A person saying "teach yourself to open downloads" is standing in front of the thing
		// they mean, and requiring them to run `director windows` and copy an ephemeral id
		// first is a CLI asking to be operated rather than used. Resolving the foreground once
		// and pinning it is the same two steps observation already takes — and the pinning is
		// the half that matters, because a selector that kept re-asking would follow focus onto
		// whatever the user clicked next, halfway through their own demonstration.
		//
		// An explicitly named window still wins, and is still validated exactly as before.
		//
		// Deleting this must fail TestTeachingWithNoWindowNamedTeachesTheForegroundWindow.
		// WHO IS ASKING decides whether the window can be chosen yet.
		//
		// A request from Marco's own control surface cannot resolve one now: pressing the
		// button put Marco in front, so the foreground is Marco and pinning it would
		// fingerprint the control panel as the place the task starts from. That selector is
		// left empty and resolved by AwaitSubject at the first moment there is an answer
		// that is not Marco — see teach.WaitingForDemonstration.
		//
		// A request from anywhere else keeps the behaviour it has always had, resolving the
		// foreground once and pinning it. The command line is a developer tool and must not
		// change shape underneath the people and tests that use it.
		//
		// An explicitly named window wins in both cases and is validated exactly as before.
		target := q.Target
		if q.Surface {
			// The surface owns whatever window it asked FROM, so its buttons stop being
			// demonstration evidence. Adopted before the session exists, because the
			// click that starts one is already in the past by the time it does.
			r.adoptRequestingSurface(ctx)
		} else if target.Zero() {
			sel, err := r.currentContext(ctx)
			if err != nil {
				return teachView{}, err
			}
			target = sel
		}
		if !target.Zero() {
			if err := target.Validate(); err != nil {
				return teachView{}, err
			}
		}
		// The play's two names are derived and VALIDATED here, before anybody is asked to
		// demonstrate anything — a phrase Marco could never write down should fail in the
		// first second, not after two demonstrations and a rehearsal. They are bound to
		// nothing until the save.
		actor, verb, err := playNameFor(q.Name, q.Actor, q.Verb)
		if err != nil {
			return teachView{}, err
		}
		r.teachGround = newTeachGrounding(r)
		s, err := r.teach.start(q.Name, r.teachingPasses(target),
			r.observations.memory,
			&teachTail{rt: r, app: r.teachApplication, phrase: r.teachPhrase,
				live: !q.Dry},
			r.teachGround, target, actor, verb)
		if err != nil {
			return teachView{}, err
		}
		return r.viewTeaching(ctx, s, q.Watch), nil

	default:
		s, ok := r.teach.read()
		if !ok {
			return teachView{}, fmt.Errorf("nothing is being taught")
		}
		return r.viewTeaching(ctx, s, q.Watch), nil
	}
}

// viewTeaching is the outward view with the grounded endpoints filled in.
//
// The conversion happens HERE, per read, rather than once when the decision was made. That is
// deliberate: `referent.Map` compares the window rectangle the regions were measured against with
// where the window is right now, so a start grounded before the user dragged their window becomes
// a sentence rather than a highlight in the wrong place. Converting once and caching the boxes
// would freeze the answer and lose exactly that check.
func (r *Runtime) viewTeaching(ctx context.Context, s teach.Session, watch bool) teachView {
	v := viewTeach(s, r.teach.running(), watch)
	sel := windowref.Selector{}
	if r.teach != nil {
		r.teach.mu.RLock()
		sel = r.teach.selector
		r.teach.mu.RUnlock()
	}
	v.Grounding = r.groundingViews(ctx, s, sel)
	return v
}

// finishing reports whether the person has said the demonstration is over and the bounded work
// after it is still running.
//
// Read from the coordinator rather than tracked here, so there is one answer to it.
func (t *teaching) finishing() bool {
	t.mu.RLock()
	c := t.coord
	t.mu.RUnlock()
	return c != nil && c.Finished() && !c.Session().Phase.Settled()
}

// teachingPasses builds the pass runner for one attempt.
//
// A seam, and a narrow one: production always returns the real teachPasses against the ordinary
// registry. It exists so a test can drive the WHOLE request path — the coordinator, the teaching
// owner, the view — with the desktop substituted, rather than reaching past the request into the
// coordinator and proving something about a different call.
func (r *Runtime) teachingPasses(target windowref.Selector) teach.Passes {
	if r.passesFor != nil {
		return r.passesFor(target)
	}
	return &teachPasses{rt: r, selector: target}
}

// teachPhrase is what the Audience called this behaviour, read late.
//
// The words they typed, which is what they will ask for afterwards. Not the actor/verb the play
// declares — those are its identity inside Marco, and taking the route's name from them registered
// "Open Mouse Settings" as `mousesettings` so the taught phrase resolved to a did-you-mean.
func (r *Runtime) teachPhrase() string {
	if r.teach == nil {
		return ""
	}
	s, _ := r.teach.read()
	return s.Name
}
