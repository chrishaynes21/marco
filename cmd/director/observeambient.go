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
	last     string
	lastApp  string
	samples  int
	sessions int
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
		a.watchOnce(ctx)
		a.mu.Lock()
		gap := a.attention
		a.mu.Unlock()
		if !sleepCtx(ctx, gap) {
			return
		}
	}
}

// watchOnce runs one bounded ambient observation and records what it saw.
func (a *ambientObserver) watchOnce(ctx context.Context) {
	application := strings.TrimSpace(winctxActive())
	if application == "" {
		return
	}
	view, err := a.rt.StartObservation(service.ObservePayload{
		Target:   currentWindowSelector(application),
		Duration: ambientSession,
		Interval: ambientBusy,
	})
	if err != nil {
		return
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
		if a.sample(application) {
			changed = true
		}
		if !sleepCtx(ctx, ambientBusy) {
			break
		}
	}
	a.rt.endLook(ctx, id)

	a.mu.Lock()
	a.attention = nextAttention(a.attention, changed)
	if changed {
		a.lastChange = time.Now()
	}
	a.mu.Unlock()
}

// sample reads where Marco is and records it. Reports whether anything changed.
func (a *ambientObserver) sample(application string) bool {
	return a.recordPlace(application, a.rt.placeHereIn(application), time.Now())
}

// recordPlace decides what one reading is worth keeping.
//
// Separated from the reading above so it can be driven from a test: the supervisor loop needs a
// desktop and this needs only a Place, and the two refusals below are the whole of what ambient
// watching is allowed to conclude.
func (a *ambientObserver) recordPlace(application string, place observe.Place, now time.Time) bool {
	a.mu.Lock()
	a.samples++
	if !place.Readable() {
		// DEGRADED PERCEPTION IS NOT A PLACE. The window is there and its content is
		// not being read; inventing a Place from that would put the frame every page of
		// an application shares into the buffer as a screen. See ADR-090.
		a.lastDegrade = now
		a.mu.Unlock()
		return false
	}
	if place.Subject == "" {
		// A screen Marco does not recognise stays unrecognised. Ambient watching holds
		// no licence to establish it, and asking somebody to name it would turn paying
		// attention into an interactive acquisition episode.
		a.mu.Unlock()
		return false
	}
	previous, previousApp := a.last, a.lastApp
	a.last, a.lastApp = place.Subject, application
	a.mu.Unlock()

	a.buf.Saw(application, place.Subject, now)
	if previous != "" && previous != place.Subject && previousApp == application {
		// HUMAN, because ambient watching is what somebody doing their own work looks
		// like. A performance's transitions are recorded by the walk that made them, and
		// conflating the two would eventually teach Marco its own behaviour back.
		a.buf.Moved(application, previous, place.Subject, ambient.ByHuman, now)
	}
	return previous != place.Subject
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
	a.mu.Unlock()

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
		r.watching = &ambientObserver{rt: r, buf: ambient.New(), attention: ambientBusy}
	})
	return r.watching
}

// winctxActive and currentWindowSelector are the two things the supervisor needs from the desktop.
//
// Package VARIABLES for the reason `windowLeads` is: they are the lines in this file a test cannot
// supply for itself, and without a seam the only thing any test could assert about ambient
// watching is that it compiles.
var winctxActive = func() string { return winctx.Active() }

var currentWindowSelector = func(application string) windowref.Selector {
	return windowref.Selector{Application: application}
}
