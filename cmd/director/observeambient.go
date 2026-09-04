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
	// held is the observation session ambient watching currently owns, empty when it owns
	// none. Recorded so an explicit Learn can ask for the slot back — see standAside.
	held observe.SessionID
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
	// carried is one unresolved action held over a session boundary, nil the rest of the
	// time. Transient by construction: it lives for at most one session, reaches no store,
	// and does not survive a restart. See carriedActs.
	carried *carriedActs
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
	// WHAT THE AFFORDANCE SWEEP SAW AND WHAT IT REFUSED, so a dogfood can answer whether an
	// interface that teaches Marco nothing is a QUANTITY problem or an ADMISSION problem.
	//
	// Counts and role names only. The refused text is exactly what the gate exists to
	// withhold, and a diagnostic that held it would be a copy of the thing being refused.
	//
	// Cumulative across the session, and reset with watching, like every counter here.
	affordancesVisible  int
	affordancesAdmitted int
	affordancesWithheld map[string]int
	affordancesStored   int
	// WHETHER A READING COULD SAY WHERE IT WAS, and why not when it could not. The
	// evidence-density question: settlement wants a screen's word twice and a brief page
	// gets it once. Reason strings from the naming rule's own vocabulary, never the word.
	namingProduced int
	namingAbsent   map[string]int
	// trace is what became of every reading, for the diagnostics. See recognition.go: it is
	// written after the decisions and read by nothing that makes one.
	trace recognitionTrace
	// lastInput is when the session's input log last grew, in wall-clock, so the trace can
	// say how long before a reading somebody last did something. Wall-clock rather than the
	// session's own milliseconds because a reading is compared against readings, and those
	// are stamped by the supervisor.
	lastInput time.Time
	// inputSeen is the input-log high-water mark the above was set at.
	inputSeen int
	// lastEstablished is the subject a dwell-establishment made durable ON THIS READING,
	// consumed by the trace. A one-slot handoff rather than a return value, because the write
	// happens three calls deep and the only thing that wants to know is a diagnostic —
	// threading it back through would put a reporting concern into a policy signature.
	//
	// NEW ones only. `establish` is idempotent and answers with the same id on every reading
	// of a screen already known, so setting this unconditionally made "established" mean
	// "recognised", which is a different fact and the one the trace already had.
	lastEstablished string
	// establishedHere is every subject this run of attention made durable, so the line above
	// can tell a first write from the idempotent ones after it. Bounded by how many screens
	// somebody visits while watching, and cleared with the trace.
	establishedHere map[string]bool
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
	// THE TRACE BELONGS TO THIS RUN OF ATTENTION, not to the process. A diagnostic carrying
	// readings from before somebody turned watching on is a diagnostic that answers a
	// question about a different afternoon — which is how four dogfood runs were read as
	// evidence about a configuration they were not exercising.
	a.trace.reset()
	a.namingProduced, a.namingAbsent = 0, nil
	a.lastInput, a.inputSeen = time.Time{}, 0
	a.establishedHere = nil
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
	// LEARNING ENDS WITH WATCHING, and this is the containment made real rather than
	// described.
	//
	// Learning is a permission to remember what Marco SEES. With nothing being seen there is
	// nothing for it to apply to, so leaving it set would leave a switch on that governs
	// nothing — and the status would then say "watching: no, learning: yes", which is a state
	// no person can act on and no product should be able to reach. The relationship is a
	// containment: LEARN is inside WATCH.
	//
	// It is NOT symmetrical, deliberately. `DisableAmbientLearning` leaves watching alone:
	// somebody switching learning off asked for less memory, not less attention. Stopping
	// altogether is the other thing, and it means both.
	//
	// Above the already-stopped return, so the invariant holds unconditionally rather than
	// only on the path that was watching.
	//
	// Deleting this must fail TestStoppingWatchingStopsLearning.
	a.promotion.Enabled = false
	// AND ANY ACTION WAITING TO CROSS A SESSION BOUNDARY. Somebody who stopped watching did
	// not leave a press behind for the next session to finish — and this is above the
	// already-stopped return for the same reason the line before it is: the invariant holds
	// unconditionally or it is not one.
	//
	// Deleting this must fail TestStoppingClearsAPendingCarry.
	a.carried = nil
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

// ambientLearning is whether a person has agreed that what Marco watches may be remembered.
//
// # Read by perception, which is why it is here and not only in the policy
//
// The ambient learning switch decides two things that used to be one thing in name only: whether
// a candidate may be PROMOTED — asked in `ambient.Judge` — and whether the name of the control
// somebody clicked may travel on the input event at all, which is asked in the sampler, before
// any of it. The second was never wired, and without it the first could not fire in any interface
// that navigates by list items. See liveSampler.mayNameTargets.
//
// A read, on the same lock the policy uses, cheap enough for a per-cycle caller.
func (r *Runtime) ambientLearning() bool {
	if r == nil || r.observations == nil {
		return false
	}
	return r.ambient().policy().Enabled
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
	id := observe.SessionID(view.ID)
	a.mu.Lock()
	a.sessions++
	// WHAT AMBIENT IS HOLDING, so an explicit Learn can ask for it back. Recorded rather
	// than inferred: the registry knows which session is active and not whose it is, and a
	// passive observe-game somebody set up deliberately is not Marco.s to cancel.
	a.held = id
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		if a.held == id {
			a.held = ""
		}
		a.mu.Unlock()
	}()

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
	// WHAT MARCO ALREADY KNOWS HAS TURNED OUT TO BE CALLED — first, and deliberately above
	// every refusal below.
	//
	// None of the refusals below is about naming. They are about whether THIS reading is a
	// place: degraded perception, a screen nothing could describe, a transition frame. A name
	// settles by RECURRENCE across readings that already happened, so a frame Marco cannot
	// read does not un-settle a word that recurred — and skipping the sweep on it would mean
	// a name arriving during a degraded moment waited for the next clean one for no reason.
	//
	// It is idempotent at the store, so the readings after the first write say nothing at all.
	//
	// Deleting this call must fail TestWatchingAndLearningNamesAPlaceItAlreadyKnows.
	a.callPlaces(application, look)
	// AND WHAT THOSE PLACES OFFER, beside the naming sweep and above the same refusals, for
	// the same reason: what a screen offers settles by recurrence across readings that already
	// happened, and a frame Marco cannot read does not un-settle a control that recurred.
	//
	// Deleting this call must fail TestWatchingAndLearningRemembersWhatAScreenOffers.
	// AND THE SCREEN ITSELF, before what it offers — a control is scoped to a Place, so there
	// has to be one to scope it to. See settlePlace for the measured gap this closes.
	a.settlePlace(application, look)
	a.rememberOffers(application, look)
	// AND WHAT BECAME OF THIS READING, for the diagnostics. After the decisions, never
	// before them, and read by nothing that makes one. See recognition.go.
	a.traceReading(application, look, now)
	place := look.Place
	a.mu.Lock()
	a.samples++
	if !place.Readable() {
		// DEGRADED PERCEPTION IS NOT A PLACE. The window is there and its content is
		// not being read; inventing a Place from that would put the frame every page of
		// an application shares into the buffer as a screen. See ADR-090.
		a.lastDegrade = now
		a.settled++
		// AND CONTINUITY IS BROKEN. A reading that got no further than the window frame
		// is a gap nobody read, and an action waiting for its destination cannot be said
		// to have crossed something Marco could not see. Dropped rather than bridged.
		//
		// Deleting this must fail TestADegradedReadingBreaksTheCarry.
		a.carried = nil
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
	// EXCEPT WHEN IT IS THE SAME SCREEN WEARING A NEW SESSION'S NAME.
	//
	// # The invented transition this removes
	//
	// A screen Marco does not recognise is keyed on `session:state`, and the session belongs
	// in the key: `state_2` in one session and `state_2` in the next are unrelated screens, and
	// without it two DIFFERENT screens either side of a boundary would compare equal and a
	// real crossing would be lost — see TestTwoScreensEitherSideOfASessionAreNotOneScreen,
	// which this must not break.
	//
	// The cost was the mirror image. The SAME screen either side compared UNEQUAL, so every
	// rollover recorded a step from a screen to itself. Ambient sessions last twenty seconds,
	// so that fired three times a minute forever. Measured live on an untouched desktop:
	// sixteen "transitions" in five minutes, one every ten polls of a two-second poller —
	// periodic by construction rather than by anything on screen.
	//
	// So a boundary is not taken at its word. The two readings are compared through the ONE
	// identity test, the same `CompareStructure` the candidate ledger already uses to unify a
	// wide and a narrow reading of one screen — never a second opinion about identity.
	//
	// Measured after: an untouched desktop crossed five session boundaries in 132 seconds and
	// recorded no transitions at all.
	//
	// Deleting this must fail TestASessionBoundaryIsNotACrossing.
	if changed && !sameSession && previousShape != nil && look.Shape != nil &&
		observe.CompareStructure(previousShape.Signature, look.Shape.Signature) ==
			observe.MatchSame {

		// THE NEW KEY IS KEPT — the one already stored above — and only the CONCLUSION
		// is withdrawn.
		//
		// Keeping the old key instead was tried and is wrong in a way that hides itself:
		// the first reading of the new session is suppressed, and then the SECOND reading
		// of that same session compares against the old session's key, finds it different,
		// and crosses anyway. The phantom is delayed by one reading rather than removed,
		// which is why it survived a live measurement and a test that only read once per
		// session.
		changed = false
	}
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
		did := a.takeForCrossing(previousState, look.State)
		if len(did) == 0 {
			// NOTHING WAS FILED AGAINST THE SCREEN THEY LEFT, which is what a session
			// boundary between the press and its destination looks like from here. The
			// carry answers whether this is still that same interaction; every way it
			// is not, it says no.
			//
			// Deleting this must fail
			// TestARolloverBetweenAPressAndItsDestinationKeepsTheEdge.
			a.mu.Lock()
			did = a.claimCarried(now)
			a.mu.Unlock()
		}
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
	// WHAT THE AFFORDANCE SWEEP SAW AND REFUSED. Copied rather than shared, because the map
	// keeps being written while whoever asked is rendering it.
	out.AffordancesVisible = a.affordancesVisible
	out.AffordancesAdmitted = a.affordancesAdmitted
	out.AffordancesStored = a.affordancesStored
	out.NamingProduced = a.namingProduced
	if len(a.namingAbsent) > 0 {
		out.NamingAbsent = make(map[string]int, len(a.namingAbsent))
		for why, n := range a.namingAbsent {
			out.NamingAbsent[why] = n
		}
	}
	steps := a.trace.all()
	if len(a.affordancesWithheld) > 0 {
		out.AffordancesWithheld = make(map[string]int, len(a.affordancesWithheld))
		for role, n := range a.affordancesWithheld {
			out.AffordancesWithheld[role] = n
		}
	}
	application := a.lastApp
	a.mu.Unlock()

	if store, ok := a.rt.watchedStore(); ok && application != "" {
		out.Candidates = len(store.Watched(application))
	}

	out.Recognition = recognitionReport(steps)

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

// currentWindowSelector names the window an ambient session watches.
//
// # THE WINDOW IN FRONT, not the application it belongs to
//
// The supervisor has just asked the desktop what is in front, in order to decide whether to
// start a session at all. Asking the resolver to find "that application" is a DIFFERENT question,
// and it has a different answer whenever one executable owns two windows.
//
// Windows hosts Settings, XBOX and Realtek Audio Console in one `applicationframehost`. With the
// audio console open, every ambient session over Settings resolved as `ambiguous` and skipped
// every reading — measured live, `State: target_unavailable`, `Samples: 0  skipped: 39` — and
// because starting the session succeeded, nothing anywhere reported a failure. A person walked
// Home to Bluetooth to Mouse three times and Marco noticed nothing, silently, for twenty minutes.
//
// The application is still passed, and still decides when the session has been left behind: see
// the foreground check inside watchOnce. What changed is only which window this session is about.
//
// Deleting the foreground selector must fail TestWatchingTheWindowInFrontIsNotAmbiguous.
var currentWindowSelector = func(application string) windowref.Selector {
	return windowref.Selector{Foreground: true}
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
		// AND A PRESS CUTS THE WAIT SHORT.
		//
		// The same cheap loop, asked one more question. A person clicking through an
		// application is the opposite of a desktop sitting still, and waiting out an
		// interval chosen for stillness is how a four-screen walk becomes three edges.
		//
		// Deleting this must fail TestAPressCutsTheWaitShort.
		if a.somethingHappened(application) {
			return false, true
		}
	}
}

// standAside gives up the observation slot if ambient watching is the one holding it.
//
// # Why watching yields to a demonstration
//
// Measured in the 38A dogfood, and it stopped the session dead: with `marco observe` on, an
// explicit `learn` came back
//
//	phase: refused   refused: no_observation
//	"I couldn't watch — I lost sight of that window."
//
// Every time. One observation runs at a time, ambient watching had the slot, and Learn was
// refused — so the product's headline loop, watch me and then teach me, could not be walked from
// the command line at all. The sentence made it worse by blaming the window, which sends somebody
// to check their screen for a fault that is not there.
//
// This is the rule Light Mode already follows, and for the same reason: background attention is
// an instrument, a demonstration is the work, and somebody who typed `learn` has said which of
// the two they want. See yieldWatching.
//
// It yields ONLY the session ambient itself started. Watching is not switched off — the
// supervisor's own loop sees the slot go, stops early, and asks again afterwards.
//
// Deleting this must fail TestLearningTakesTheSlotBackFromWatching.
func (a *ambientObserver) standAside() {
	if a == nil || a.rt == nil || a.rt.observations == nil {
		return
	}
	a.mu.Lock()
	held := a.held
	a.held = ""
	a.mu.Unlock()
	if held == "" {
		return
	}
	_ = a.rt.observations.Cancel(held)
	a.rt.awaitLookRetired(context.Background())
}

// ambientHoldsSession reports whether an observation session is the one ambient watching owns.
//
// # Asked of what ambient RECORDED, not inferred from the registry
//
// The registry knows which session is active; it does not know whose purpose it serves. A passive
// observe-game somebody set up deliberately, and a Learn demonstration, look identical to it — and
// neither is Marco's to cancel. `held` is written when the supervisor opens a session and cleared
// when it closes, so this is the one honest answer to "is that mine".
//
// It does not construct an observer. A Director that has never watched anything holds nothing, and
// asking the question must not be what starts a supervisor.
func (r *Runtime) ambientHoldsSession(id observe.SessionID) bool {
	if r == nil || r.watching == nil || id == "" {
		return false
	}
	a := r.watching
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.held == id
}

// standAsideForAction gives up Marco's OWN watching so the Director can act, and does nothing to
// anybody else's session.
//
// # One place, because it is one decision
//
// Three doors can start something that moves the desktop — an executed phrase, a performance and
// an experiment — and every one of them met the same wall: ambient watching runs continuously, so
// with Watch & Learn on there was always an active session and every door refused. The control
// centre offered "test what I learned" and the watching that produced the offer forbade it.
//
// Asked the same way in all three, so a fourth door cannot answer it differently.
//
// Deleting this must fail TestWatchingStandsAsideForMarcosOwnActions.
func (r *Runtime) standAsideForAction() {
	if r == nil || r.observations == nil {
		return
	}
	id := r.observations.ActiveID()
	if id == "" || !r.ambientHoldsSession(id) {
		return
	}
	r.ambient().standAside()
}

// somethingHappened reports whether human input is waiting to be read.
//
// # Sample because something happened, rather than on a timer and hope
//
// The gap between readings is ATTENTION: a second on a desktop somebody is using, growing to eight
// while nothing changes. That is right for a screen being stared at and wrong for one being
// clicked through — a person moving at ordinary speed can press, arrive, and press again inside one
// interval, and Marco discovers the whole journey as a single unexplained change.
//
// Measured: a four-screen walk at normal speed yielded three edges, one of them false, and the
// screen in the middle never settled at all because nothing looked at it while it was up.
//
// So a press cuts the wait short. The input log is the session's own — the same one `drain` reads
// — and this only asks whether it has grown past the observer's cursor. It CONSUMES NOTHING:
// attribution still happens exactly once, in `drain`, from the same cursor it always used.
//
// # Why this is not raising the sampling rate
//
// Because nothing here samples. An idle desktop produces no input, asks this question ten times a
// second at the cost of comparing two integers, and waits the full eight seconds exactly as
// before. What changes is only that a desktop somebody is USING stops being watched on a schedule
// that has nothing to do with them.
//
// Deleting this must fail TestAPressCutsTheWaitShort.
func (a *ambientObserver) somethingHappened(application string) bool {
	g := a.rt.observations
	if g == nil {
		return false
	}
	ev := g.evidenceForPointing()
	if !ev.ok || !sameApplication(ev.app, application) {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if ev.session != a.session {
		// A NEW SESSION HAS ITS OWN LOG AND ITS OWN CURSOR. Comparing across them would
		// read one session's count against another's position, which is the same
		// restarting-counter mistake ADR-119 was about — so the question becomes whether
		// this session has anything in it at all.
		//
		// Returning true unconditionally was the first version and it cut the wait short
		// on every quiet desktop, which is the idle cost this whole mechanism promises not
		// to touch. Caught by TestAPressCutsTheWaitShort.
		return len(ev.shadow.InputLog.Events) > 0
	}
	return ev.shadow.InputLog.Dropped+len(ev.shadow.InputLog.Events) > a.cursor
}

// recordAffordanceAdmission tallies one sweep's outcome for the diagnostics.
//
// On the Runtime rather than on the sampler because the sampler is per session and the question a
// person asks — "why has Marco learned nothing about this application" — spans all of them.
//
// It counts and it decides nothing. Nothing reads these to admit or refuse anything, which is what
// keeps a diagnostic from quietly becoming a policy.
func (r *Runtime) recordAffordanceAdmission(visible, admitted int, withheld map[string]int) {
	if r == nil {
		return
	}
	a := r.ambient()
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.affordancesVisible += visible
	a.affordancesAdmitted += admitted
	for role, n := range withheld {
		if a.affordancesWithheld == nil {
			a.affordancesWithheld = map[string]int{}
		}
		a.affordancesWithheld[role] += n
	}
}

// recordNaming tallies whether a reading produced a destination claim, and why not.
//
// # The question this answers
//
// Settlement needs a screen's word twice, and a brief page gets it once. That is either a screen
// which says nothing on most readings or a screen whose claim keeps being refused, and the two are
// different investigations — the first is an acquisition question, the second a rule question.
//
// Counts and the naming rule's own reason strings. Never the text: `NameClaim.Value` is where the
// word lives and it is deliberately not passed here.
//
// It decides nothing. Nothing reads these to admit, refuse or name anything.
func (r *Runtime) recordNaming(produced bool, why string) {
	if r == nil {
		return
	}
	a := r.ambient()
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if produced {
		a.namingProduced++
		return
	}
	if why == "" {
		why = "no reason given"
	}
	if a.namingAbsent == nil {
		a.namingAbsent = map[string]int{}
	}
	a.namingAbsent[why]++
}
