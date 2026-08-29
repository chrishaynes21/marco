package main

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// Marco paying attention.
//
// # What ambient watching is, and the four things it is not
//
// It is one supervisor, owned by the one Director that owns this home, keeping the ONE
// observation substrate busy when nothing else needs it — so that "where is the Audience
// standing" usually has a recent answer instead of costing a six-second look every time somebody
// asks.
//
// It is not a Learn episode. Sessions are started through `Start`, which hands the runner the
// ZERO licence: no establishing places, no acquiring route evidence, no naming activated targets.
// A licence has to be named to be granted and this names none, so ambient watching cannot make
// anything durable however long it runs. See [[ADR-076-a-place-may-say-what-it-appears-to-be-called]]
// for why those three are separate in the first place.
//
// It is not a recording. What it keeps is in [ambient.Buffer]: tallies keyed on durable ids, a
// short recent walk, and nothing a person could read.
//
// It is not authority. Watching grants no permission to act and takes no desktop lease — see
// ADR-092, where observation is deliberately the one thing that needs neither.
//
// It is not a second observer. The registry allows one session at a time and this supervisor
// YIELDS: when Learn, Here or a performance owns the substrate, ambient watching waits rather
// than competing. There is never more than one observation session, and ambient is not the one
// that wins an argument.
//
// # Why yielding rather than layering
//
// The alternative would be one continuous session whose licences and attention change as
// consumers come and go. That is the prettier architecture and it is a rewrite of the session
// runner; this achieves the property that matters — one observer, ambient resumes afterwards —
// against the machinery that exists. The cost is honest and worth writing down: a Learn session
// interrupts ambient watching rather than borrowing it, so the ambient tally has a gap for the
// duration of the Learn.
type ambientObserver struct {
	rt  *Runtime
	buf *ambient.Buffer

	mu      sync.Mutex
	on      bool
	cancel  context.CancelFunc
	done    chan struct{}
	started time.Time
	// attention is the current gap between ambient sessions. It grows while nothing changes
	// and resets the moment something does — see [nextAttention].
	attention time.Duration
	// last is the Place the previous sample resolved to, which is what makes a change a
	// TRANSITION rather than two unrelated sightings.
	last    string
	lastApp string
	// lastState is the session-local screen state `last` was read from, lastShape its
	// description when Marco does not recognise it, and bridged how many unrecognised
	// readings have been crossed since. Together they are the SOURCE half of the next step.
	lastState observe.ScreenStateID
	lastShape *ambient.Shape
	bridged   int
	settled   int
	// session, cursor and behind track the input log this observer is reading. Session-local
	// and reset together; see ambientObserver.drain.
	session observe.SessionID
	cursor  int
	behind  int
	// pending holds the actions filed against each session-local screen state. It IS the
	// generation correlation — see ambientObserver.attribute for why an action is filed
	// against the state on its own stamp rather than against whatever is in front when it is
	// read.
	pending map[observe.ScreenStateID][]ambient.Act
	// actions counts the semantic actions this observer has attributed, for status.
	actions int
	// promotion is the ambient learning rule this observer runs under, and noticedEdges and
	// promoted are what the ledger has done under it. Separate from watching by construction:
	// see ADR-095.
	promotion    ambient.Policy
	noticedEdges int
	promoted     int
	// lastLedgerSession is which watching session the previous crossing belonged to.
	lastLedgerSession observe.SessionID
	samples           int
	sessions          int
	// loops counts supervisor goroutines that are currently running. It exists to be
	// asserted: "one observer" is the product claim, and the only way to see a second one
	// is to count them.
	loops       int
	lastChange  time.Time
	lastDegrade time.Time
}

// The bounds ambient watching runs under, and every one of them is a refusal to be greedy.
const (
	// ambientSession is how long one ambient observation runs before the supervisor decides
	// what to do next. Short enough that stopping is prompt, long enough that starting one
	// is not the dominant cost.
	ambientSession = 20 * time.Second
	// ambientBusy and ambientIdle bound the gap BETWEEN samples. Busy is what a desktop
	// somebody is using gets; idle is where it settles when nothing has changed.
	//
	// Neither is fast. A screen reading costs a run of accessibility snapshots — measured at
	// roughly two hundred milliseconds on one live machine — and something that read
	// continuously would be a background process a person could feel. Ambient watching is
	// meant to be affordable enough to leave on.
	ambientBusy = 1 * time.Second
	ambientIdle = 8 * time.Second
	// ambientYield is how long the supervisor waits before looking again when somebody else
	// owns the substrate. It is a poll of a local boolean, not of the screen.
	ambientYield = 500 * time.Millisecond
)

// EnableAmbient starts watching, and says so if it was already.
//
// IDEMPOTENT, and that is a product requirement rather than tidiness: `marco observe` is the
// sort of thing somebody types twice. A second call must not make a second supervisor, must not
// restart the substrate, and must not throw away what the first one has already noticed.
//
// Deleting the already-on check must fail TestWatchingTwiceIsStillWatchingOnce.
func (r *Runtime) EnableAmbient() service.AmbientView {
	if r == nil || r.observations == nil {
		return service.AmbientView{}
	}
	a := r.ambient()
	a.mu.Lock()
	if a.on {
		a.mu.Unlock()
		return a.view()
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.on, a.cancel, a.done = true, cancel, make(chan struct{})
	a.started, a.attention = time.Now(), ambientBusy
	// COUNTED HERE, not inside the goroutine. A count the loop increments when it happens
	// to be scheduled is a count a caller can read as zero while a supervisor is starting,
	// and "how many observers are there" must be answerable the instant this returns.
	a.loops++
	done := a.done
	a.mu.Unlock()

	go a.run(ctx, done)
	return a.view()
}

// DisableAmbient stops watching, and forgets what watching held.
//
// The forgetting is deliberate. What this buffer carries is the present tense — what is on
// screen and what just happened — and keeping it after Marco stops paying attention would make
// "stopped" a claim about the future only. Somebody who turns watching off should not find its
// evidence still sitting there.
//
// Stopping something already stopped is honest and harmless.
func (r *Runtime) DisableAmbient() service.AmbientView {
	if r == nil || r.observations == nil {
		return service.AmbientView{}
	}
	a := r.ambient()
	a.mu.Lock()
	if !a.on {
		a.mu.Unlock()
		return a.view()
	}
	a.on = false
	cancel, done := a.cancel, a.done
	a.cancel, a.done = nil, nil
	a.mu.Unlock()

	cancel()
	// WAIT FOR THE LOOP TO ACTUALLY STOP before reporting that it has. A view that said
	// "not watching" while a goroutine was still sampling would be the one lie this surface
	// cannot afford: the whole point of the command is that a person can trust it.
	select {
	case <-done:
	case <-time.After(2 * ambientYield):
	}
	a.buf.Forget()

	a.mu.Lock()
	a.last, a.lastApp, a.samples, a.sessions = "", "", 0, 0
	// AND EVERYTHING DERIVED FROM WATCHING, not only the buffer. The session cursor, the
	// screen states, the actions filed against them and the count of them are the same
	// present tense the buffer holds; leaving any of them behind would let the next watching
	// session attribute its first action to the previous one's screen.
	a.session, a.cursor, a.behind, a.actions = "", 0, 0, 0
	a.lastState, a.lastShape, a.bridged, a.settled = "", nil, 0, 0
	a.pending = map[observe.ScreenStateID][]ambient.Act{}
	a.mu.Unlock()
	return a.view()
}

// EnableAmbientLearning lets what Marco watches become durable memory, and starts watching if it
// was not.
//
// # Why this starts watching and its opposite does not stop it
//
// Because asking Marco to learn from what it sees is meaningless while it is not looking, and
// refusing would be pedantry about a state the person plainly did not want. The reverse is not
// symmetrical: somebody switching learning off asked for less MEMORY, not less attention, and
// stopping the observer would take away the thing they were happy with.
//
// # It is off until asked for, and it does not survive a restart
//
// The same two rules watching itself follows, and for the same reason — a durable toggle that
// makes Marco build permanent memory from a desktop is a consent conversation, and inventing one
// here would be arriving at it by implication. See ADR-093's note on why watching is a command
// rather than a setting; this is the sharper case of the same argument, because what it switches
// on writes to disk.
//
// Deleting the start-watching arm must fail TestLearningFromWhatYouSeeMeansWatching.
func (r *Runtime) EnableAmbientLearning() service.AmbientView {
	if r == nil || r.observations == nil {
		return service.AmbientView{}
	}
	a := r.ambient()
	a.mu.Lock()
	a.promotion.Enabled = true
	on := a.on
	a.mu.Unlock()
	if !on {
		return r.EnableAmbient()
	}
	return a.view()
}

// DisableAmbientLearning stops what Marco watches becoming durable memory, and leaves it watching.
//
// What has ALREADY been learned stays learned. Durable knowledge is not evidence of the mode that
// produced it, and deleting it because somebody switched a switch would be forgetting something
// true because they stopped wanting more of it. Candidate evidence stays too, under its own
// bound: it is what the next enabling would judge, and throwing it away would make the switch
// destructive in a way nobody asked for.
//
// Deleting the leave-watching-alone arm must fail TestTurningLearningOffLeavesMarcoWatching.
func (r *Runtime) DisableAmbientLearning() service.AmbientView {
	if r == nil || r.observations == nil {
		return service.AmbientView{}
	}
	a := r.ambient()
	a.mu.Lock()
	a.promotion.Enabled = false
	a.mu.Unlock()
	return a.view()
}

// AmbientStatus is what watching has noticed, for status and Activity.
func (r *Runtime) AmbientStatus() service.AmbientView {
	if r == nil || r.observations == nil {
		return service.AmbientView{}
	}
	return r.ambient().view()
}

// AmbientBuffer is the transient evidence, for the surfaces that report it.
func (r *Runtime) AmbientBuffer() *ambient.Buffer { return r.ambient().buf }

// run is the supervisor loop.
//
// # It never competes for the substrate
//
// One observation session may run at a time — the registry refuses a second, because two would
// contend for the screen and neither could attribute what it saw. So this asks before starting,
// and waits when the answer is no. Learn, Here and a performance's verification all own the
// substrate ahead of ambient watching, and none of them has to know this exists.
//
// # What enforces this, and what merely helps
//
// The REGISTRY is what guarantees one observer: it refuses a second session outright, so a
// supervisor without this check would simply be refused every time it asked. Deleting the yield
// therefore breaks no safety property, and the mutation survives the suite -- measured.
//
// What the yield buys is quiet. Without it ambient watching asks for the substrate every few
// seconds throughout somebody else's Learn session and is refused every time, which is churn
// against a shared registry for no purpose. The property that matters is held by
// TestWatchingWaitsForWhoeverElseIsLooking: the other session is still theirs afterwards.
func (a *ambientObserver) run(ctx context.Context, done chan struct{}) {
	defer close(done)
	defer func() {
		a.mu.Lock()
		a.loops--
		a.mu.Unlock()
	}()
	for ctx.Err() == nil {
		if a.rt.observations.ActiveID() != "" {
			// SOMEBODY ELSE IS LOOKING. Not an error and not a wait for a lock:
			// ambient watching is the lowest-priority consumer of the one substrate
			// and simply lets the others have it.
			if !sleepCtx(ctx, ambientYield) {
				return
			}
			continue
		}
		if moved := a.watchOnce(ctx); moved {
			// STRAIGHT ON TO THE NEW WINDOW, with no pause at all.
			//
			// The gap between sessions is attention — how long to wait before looking
			// again at a desktop that has been sitting still. A person who has just
			// switched program is the opposite of a desktop sitting still, and making
			// them wait for it is the difference between a mode you can use normally
			// and one you have to accommodate.
			//
			// Deleting this must fail TestWatchingIsNotBlindWhileYouSwitchWindows.
			continue
		}
		a.mu.Lock()
		gap := a.attention
		a.mu.Unlock()
		if !sleepCtx(ctx, gap) {
			return
		}
	}
}

// watchOnce runs one bounded ambient observation and records what it saw.
func (a *ambientObserver) watchOnce(ctx context.Context) (moved bool) {
	application := strings.TrimSpace(winctxActive())
	if application == "" {
		return false
	}
	// THE SAME ATTENTION GOVERNS THE GAP BETWEEN SESSIONS AND THE CADENCE INSIDE ONE.
	//
	// It used to govern only the first. `attention` already grows from a second to eight
	// while nothing changes and snaps back the moment something does — and then each session
	// it opened sampled at a flat one-second interval for twenty seconds regardless, so a
	// desktop nobody was touching still paid full price for the whole session and only the
	// silence between sessions got cheaper.
	//
	// Measured: one accessibility walk of a File Explorer window is 1.67s (298 elements, six
	// identical readings, 37E). At a one-second gap that is a 2.67s cycle, roughly seven
	// walks a session, about twelve seconds of walking in every twenty — against a tree that
	// did not change once. Passing the attention through makes the settled case sample at an
	// eight-second gap instead: two walks a session rather than seven.
	//
	// This is NOT a cache and costs no freshness. Every sample is still a complete walk taken
	// at the moment it is reported; there are simply fewer of them when nothing is happening,
	// decided by the signal that already existed for exactly this question.
	//
	// It reaches ambient watching ONLY. A look taken to answer a question sets its own
	// interval — see freshLookInterval — and execution is not slowed by a quiet desktop.
	//
	// Deleting this must fail TestAQuietDesktopIsWatchedMoreCheaply.
	a.mu.Lock()
	cadence := a.attention
	a.mu.Unlock()
	if cadence <= 0 {
		cadence = ambientBusy
	}
	view, err := ambientObserveNow(a.rt, service.ObservePayload{
		Target:   currentWindowSelector(application),
		Duration: ambientSession,
		Interval: cadence,
	})
	if err != nil {
		return false
	}
	a.mu.Lock()
	a.sessions++
	a.mu.Unlock()
	id := observe.SessionID(view.ID)

	changed := false
	deadline := time.Now().Add(ambientSession)
	for time.Now().Before(deadline) && ctx.Err() == nil {
		if a.rt.observations.ActiveID() == "" {
			break
		}
		// THE WINDOW IN FRONT IS THE ONE TO WATCH, and it is asked every reading.
		//
		// # The bug this exists to prevent, measured live
		//
		// The session pins whatever was in front when it STARTED and reads that window for
		// its whole length. Somebody starts watching from a terminal, opens Settings a
		// second later, and clicks through it — and Marco spends the next nineteen seconds
		// reading the terminal. The first live run of ambient watching did exactly that:
		// one screen recognised, no transitions, and every count honestly reporting a
		// window nobody was using.
		//
		// It is not enough to say a new session pins the new window. A person switching
		// application is the single most informative thing that happens on a desktop, and
		// a mode whose whole promise is "use your computer normally" cannot be blind for a
		// third of a minute after every switch.
		//
		// So the session ENDS at the switch and the supervisor immediately starts one on
		// the new window. A `winctx.Active()` is a cheap Win32 call — far cheaper than the
		// reading it decides whether to take — so this costs nothing against a screen
		// reading and buys the whole product claim.
		//
		// Deleting this must fail TestWatchingFollowsYouToAnotherWindow.
		if front := strings.TrimSpace(winctxActive()); front != "" &&
			!sameApplication(front, application) {
			moved = true
			break
		}
		if a.sample(application) {
			changed = true
		}
		// THE WAIT BETWEEN READINGS IS BROKEN UP, so a window switch is noticed in a
		// tenth of a second rather than at the next reading.
		//
		// Asking the desktop what is in front is a cheap Win32 call; TAKING a reading is a
		// run of accessibility snapshots costing about two hundred milliseconds. Those are
		// three orders of magnitude apart, so the cheap question can be asked far more
		// often than the expensive one without changing what watching costs — and the
		// reading cadence is untouched.
		if left, ok := a.waitOrLeave(ctx, ambientBusy, application); left {
			moved = true
			break
		} else if !ok {
			break
		}
	}
	a.rt.endLook(ctx, id)

	a.mu.Lock()
	// A SWITCH IS A CHANGE, for the purposes of attention. Somebody who has just moved to
	// another program is about to do something in it, and meeting them with a reading eight
	// seconds later is exactly the asymmetry the backoff exists to avoid.
	a.attention = nextAttention(a.attention, changed || moved)
	if changed {
		a.lastChange = time.Now()
	}
	a.mu.Unlock()
	return moved
}

// sample reads where Marco is AND what the person has done since the last look, and records both.
// Reports whether anything changed.
//
// One call, one snapshot. See ambientLook for why those two facts must not be fetched separately.
func (a *ambientObserver) sample(application string) bool {
	look := ambientLookNow(a.rt, application)
	if !look.OK {
		return false
	}
	// THE ACTIONS FIRST, before the place is recorded. They were performed on the screen the
	// person was on when they did them, and every one of them carries the state that says
	// which — but recording the new place first would move `lastState` out from under the
	// step that is about to be built from it.
	//
	// Deleting the drain must fail TestWatchingSeesWhatYouPressed.
	a.attribute(a.drain(look))
	return a.record(application, look, time.Now())
}

// record decides what one reading is worth keeping.
//
// Separated from the reading above so it can be driven from a test: the supervisor loop needs a
// desktop and this needs only a look, and the refusals below are the whole of what ambient
// watching is allowed to conclude.
func (a *ambientObserver) record(application string, look ambientLook, now time.Time) bool {
	place := look.Place
	a.mu.Lock()
	a.samples++
	if !place.Readable() {
		// DEGRADED PERCEPTION IS NOT A PLACE. The window is there and its content is
		// not being read; inventing a Place from that would put the frame every page of
		// an application shares into the buffer as a screen. See ADR-090.
		a.lastDegrade = now
		a.settled++
		a.mu.Unlock()
		return false
	}
	if place.Subject == "" && look.Shape == nil {
		// A SCREEN THAT IS NOT SOMEWHERE YOU WENT. Marco does not recognise it and the
		// evidence would not let a licensed session establish it either: it is still
		// loading, or unsettled, or nothing distinctive enough to tell apart.
		//
		// Crossed rather than refused. A real walk passes through frames like this on
		// the way somewhere, and the walk survives them — see the bridging note below.
		// Ambient watching holds no licence to establish anything and asking somebody to
		// name a spinner would turn paying attention into an acquisition episode.
		a.bridged++
		a.settled++
		a.mu.Unlock()
		return false
	}
	// A SCREEN MARCO DOES NOT KNOW IS STILL SOMEWHERE YOU WENT, when the evidence would let
	// an explicit Learn establish it. 36A recorded nothing for one, which meant a
	// demonstration through unfamiliar software produced no evidence at all — the first time
	// anybody uses a program is exactly when they most want to show Marco something.
	//
	// Nothing durable happens here: the description is transient, held under no licence, and
	// forgotten when watching stops. What it buys is that a later Learn, which DOES hold a
	// licence, can establish the same screen from the same signature.
	//
	// It is keyed on the session-local state rather than on an id, because it has no id —
	// that is what "not recognised" means.
	key := place.Subject
	if key == "" {
		key = transientKey(look.Session, look.State)
	}
	previous, previousApp := a.last, a.lastApp
	previousState, previousShape := a.lastState, a.lastShape
	bridged, settled := a.bridged, a.settled
	// WHETHER THIS CROSSING IS IN THE SAME WATCHING SESSION as the one before it, which is
	// one of the two things that make two crossings two occasions rather than one. Read here
	// because this is where the previous reading is still in scope.
	sameSession := a.lastLedgerSession == look.Session
	a.lastLedgerSession = look.Session
	a.last, a.lastApp = key, application
	a.lastState, a.lastShape = look.State, look.Shape
	changed := previous != key
	if changed {
		a.bridged, a.settled = 0, 0
	} else {
		a.settled++
	}
	a.mu.Unlock()

	if place.Subject != "" {
		a.buf.Saw(application, place.Subject, now)
	}
	if previous != "" && changed && previousApp == application {
		// WHO DID THIS. Ambient watching is ordinarily what somebody doing their own
		// work looks like — but a play running while watching is on moves the screen
		// too, and offering that back as something the person demonstrated is how a
		// system comes to learn its own behaviour from itself.
		//
		// Deleting the provenance question must fail
		// TestWhatMarcoDidIsNotWhatYouDemonstrated.
		by := ambient.ByHuman
		if a.rt.marcoIsActing() {
			by = ambient.ByMarco
		}
		did := a.takePending(previousState)
		a.mu.Lock()
		a.actions += len(did)
		a.mu.Unlock()
		step := ambient.Step{
			From: previous, To: key, Application: application,
			FromShape: previousShape, ToShape: look.Shape,
			Did: did, By: by, Bridged: bridged, Settled: settled, At: now,
		}
		a.buf.Walked(step)
		// AND THE LEDGER, on the same event and only on this event.
		//
		// A new semantic transition is the only thing that can change what repeated
		// evidence says, so this is the whole of when the promotion rule runs. An
		// unchanged desktop reaches nothing here, which is why ambient learning costs
		// 36A's idle profile exactly nothing.
		//
		// Same session means the same watching session. It gates nothing — one clean
		// traversal is already knowledge — and travels with the step so the durable edge
		// can record how widely it has been evidenced. See observe.WatchedEdge.Sessions.
		//
		// Deleting this must fail TestOneCleanTraversalBecomesGraphKnowledge.
		a.noticed(step, sameSession)
	}
	return changed
}

// recordPlace records one reading that carries a Place and nothing else.
//
// The place half of [ambientObserver.record], which is what 36A's gates are about: degraded
// perception, an unrecognised screen, a transition, the tally that makes repetition free. Those
// claims are unchanged by 36B and are still asserted through this shape, because a test that had
// to build an input log to say "watching all day costs nothing" would be saying two things at
// once and failing for either.
func (a *ambientObserver) recordPlace(application string, place observe.Place,
	now time.Time) bool {

	return a.record(application, ambientLook{
		OK: true, Application: application, Place: place, State: stateOfPlace(place),
		Session: "recordPlace",
	}, now)
}

// stateOfPlace gives a Place-only reading a stable session-local state to be attributed through.
//
// Derived from the subject so two readings of one screen share a state and two screens do not,
// which is the only property the correlation needs from it. A reading that resolved to nothing
// gets nothing, which is correct: there is no generation for a screen that was never placed.
func stateOfPlace(p observe.Place) observe.ScreenStateID {
	if p.Subject == "" {
		return ""
	}
	return observe.ScreenStateID("state_of_" + p.Subject)
}

// transientKey names a screen Marco does not recognise, for the length of one watching session.
//
// The session-local state id, prefixed so it can never be mistaken for a durable subject. It is a
// counter and it means nothing outside the session that issued it — which is exactly right for a
// place that has no identity yet, and is why the promotion step establishes the SHAPE rather than
// looking anything up by this.
//
// # It carries the session, and that is not decoration
//
// `state_2` in one session and `state_2` in the next are unrelated screens. Without the session in
// the name, two different unrecognised screens either side of a session boundary compare EQUAL —
// so `changed` is false, no transition is recorded, and the evidence is lost silently. Ambient
// sessions end every twenty seconds, so this is not a rare boundary: it is one every twenty
// seconds, all day.
//
// Deleting the session must fail TestTwoScreensEitherSideOfASessionAreNotOneScreen.
func transientKey(session observe.SessionID, state observe.ScreenStateID) string {
	if state == "" {
		return ""
	}
	return ambient.TransientKey(string(session) + ":" + string(state))
}

// marcoIsActing reports whether Marco currently holds the keyboard.
//
// Read from the ONE slot every actuating entrance funnels through — see beginPerformance, which
// exists because a rehearsal reached the desktop from inside a Learn episode without passing any
// handler. A second flag set at the call sites would have the same defect it fixed.
func (r *Runtime) marcoIsActing() bool {
	if r == nil {
		return false
	}
	r.actingMu.Lock()
	defer r.actingMu.Unlock()
	return r.acting > 0
}

// nextAttention is the backoff, and it is asymmetric on purpose.
//
// Doubling while nothing happens and dropping straight back to busy the moment something does.
// A gradual return would mean the first thing somebody did after a quiet afternoon was the thing
// Marco was slowest to notice — which is exactly backwards, because a change after a long pause
// is the most informative event there is.
func nextAttention(current time.Duration, changed bool) time.Duration {
	if changed {
		return ambientBusy
	}
	next := current * 2
	if next > ambientIdle {
		return ambientIdle
	}
	return next
}

// sleepCtx waits, and reports false if the wait was cut short.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}

// running is how many supervisor goroutines are alive. One, or none.
func (a *ambientObserver) running() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.loops
}

func (a *ambientObserver) view() service.AmbientView {
	a.mu.Lock()
	out := service.AmbientView{
		Watching:    a.on,
		Application: a.lastApp,
		Place:       a.last,
		Samples:     a.samples,
		Sessions:    a.sessions,
		AttentionMS: a.attention.Milliseconds(),
	}
	if !a.started.IsZero() && a.on {
		out.WatchingForMS = time.Since(a.started).Milliseconds()
	}
	if !a.lastDegrade.IsZero() && a.lastDegrade.After(a.lastChange) {
		out.PerceptionDegraded = true
	}
	// LEARNING, ALWAYS, and separately from watching. A status that mentioned it only while
	// it was on would make its silence mean two things at once — and this is the field that
	// says whether anything on this desktop is becoming permanent.
	out.Learning = a.promotion.Enabled
	out.Noticed, out.Learned = a.noticedEdges, a.promoted
	application := a.lastApp
	a.mu.Unlock()

	if store, ok := a.rt.watchedStore(); ok && application != "" {
		out.Candidates = len(store.Watched(application))
	}

	places, edges, recent := a.buf.Size()
	out.Places, out.Transitions, out.Recent = places, edges, recent
	return out
}

// ambient builds the supervisor on first use, once.
//
// Lazily, because most Director invocations are a single command that never watches anything, and
// once, because two supervisors would be two observers by another name.
func (r *Runtime) ambient() *ambientObserver {
	r.watchingOnce.Do(func() {
		r.watching = &ambientObserver{rt: r, buf: ambient.New(), attention: ambientBusy,
			pending: map[observe.ScreenStateID][]ambient.Act{},
		}
	})
	return r.watching
}

// winctxActive and currentWindowSelector are the two things the supervisor needs from the desktop.
//
// Package VARIABLES for the reason `windowLeads` is: they are the lines in this file a test cannot
// supply for itself, and without a seam the only thing any test could assert about ambient
// watching is that it compiles.
var winctxActive = func() string { return winctx.Active() }

// ambientLookNow is the reading the supervisor takes each cycle.
//
// A package VARIABLE for the same one reason the two below it are: it is the line in `sample`
// that a test cannot supply for itself — a real look needs a live observation session over a real
// desktop — and without a seam the only thing any test could assert about the sample loop is that
// it compiles.
//
// That is not hypothetical here. The drain call in `sample` was production code nothing invoked
// through the production path: deleting it left every test in this package passing, because each
// of them called `attribute(drain(...))` itself. Measured, by mutation, and it is the third time
// this repository has found a complete piece of working code that nothing ever ran.
//
// Production never reassigns it.
var ambientLookNow = func(r *Runtime, application string) ambientLook {
	return r.ambientLook(application)
}

// ambientObserveNow is how the supervisor opens one bounded session.
//
// A package VARIABLE for the third time in this file and for the same one reason: a real session
// needs a live window tracker and a desktop with something in front of it, so a test that could
// not replace this could only assert that `watchOnce` compiles.
//
// It exists because of a specific thing that must be tested through the LOOP rather than around
// it: the session ends when the person moves to another window. That check lives inside the
// sampling loop, and a test on an extracted predicate would pass with the check deleted from
// where it matters.
//
// Production never reassigns it.
var ambientObserveNow = func(r *Runtime, p service.ObservePayload) (service.ObserveStarted, error) {
	return r.StartObservation(p)
}

var currentWindowSelector = func(application string) windowref.Selector {
	return windowref.Selector{Application: application}
}

// ambientGlance is how often the supervisor asks the desktop what is in front, while waiting
// between readings.
//
// A tenth of a second. `winctx.Active()` is one cheap Win32 call; a screen READING is a run of
// accessibility snapshots costing around two hundred milliseconds. Three orders of magnitude
// apart, so asking the cheap question ten times a second changes nothing about what watching
// costs — and it is what makes a window switch cost a tenth of a second of blindness instead of a
// reading interval.
const ambientGlance = 100 * time.Millisecond

// waitOrLeave waits between two readings, and cuts the wait short if the person has gone
// somewhere else.
//
// Reports `left` when the foreground is a different application, and `ok` false when the context
// ended. A wait that ran to completion is `false, true`.
//
// # Why the foreground is polled and the readings are not
//
// Because they cost completely different amounts, and the thing that has to be prompt is the
// cheap one. Nothing here takes a reading, touches the session or writes anything: it asks a
// question whose answer decides whether the expensive work is still pointed at the right window.
func (a *ambientObserver) waitOrLeave(ctx context.Context, d time.Duration,
	application string) (left, ok bool) {

	deadline := time.Now().Add(d)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return false, true
		}
		if remaining > ambientGlance {
			remaining = ambientGlance
		}
		if !sleepCtx(ctx, remaining) {
			return false, false
		}
		if front := strings.TrimSpace(winctxActive()); front != "" &&
			!sameApplication(front, application) {
			return true, true
		}
	}
}
