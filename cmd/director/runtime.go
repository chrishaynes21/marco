package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/bridgehost"
	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/game"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/diagnostics"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/visualstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/shadow"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/timeline"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/plan"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/internal/director/variables"
	waitengine "github.com/chaynes-simpleclouds/marco/internal/director/wait/engine"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/platform/marcorunner"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/platform/uiaclient"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// runtimeEpochCounter breaks ties between runtimes constructed inside one clock tick.
//
// `time.Now().UnixNano()` is nanosecond-TYPED but not nanosecond-RESOLVED: the Windows system
// clock advances in roughly 15ms steps, so two runtimes built back to back read the same
// timestamp and produced the same epoch — exactly the case an epoch exists to distinguish.
// It surfaced as a flaky test, and the flake was the defect, not the test.
//
// Process-wide, because across processes the pid already separates them. Not a source of
// ordering: an epoch is opaque and only equality is meaningful.
var runtimeEpochCounter atomic.Uint64

// Runtime is the long-lived Director: providers, pipeline and graph, constructed
// once and reused for every request.
//
// This is where the milestone's central win lives. Under the old model the
// accessibility bridge was a subprocess created and killed per command, so Chrome
// and VS Code — which only expose their interiors after sustained client presence —
// were measured from cold every single time. Holding the bridge open across requests
// is not an optimisation; it is the difference between seeing 65 elements of Chrome
// and seeing 2248.
//
// Director construction lives HERE and nowhere else. `marco director` is a pure
// client that talks to this process; it never builds a Director of its own.
type Runtime struct {
	pipeline *execute.Pipeline
	graph    *actiongraph.FileGraph
	tracker  *service.ProviderTracker
	// bridge is the Accessibility act's implementation, held open for the life of the
	// process — see the note above about Chrome's tree being walked from cold otherwise.
	//
	// Typed as the INTERFACE rather than as *bridgehost.Host. It is what the Theater's
	// accessibility Actor is built over, and a wiring test that cannot substitute it cannot
	// prove the Theater is reached — which in this repository is not a theoretical worry.
	// Close is asked for rather than assumed; see Close below.
	bridge runtime.Host

	// watchArming, watchCancel and watchMu are LIGHT MODE waiting for somewhere to watch.
	//
	// Pressing Watch cannot resolve a window: the button is in Marco, so the foreground at
	// that instant is the browser showing the control centre. So it arms, waits for a window
	// that is not Marco to hold still, and observes that — the same shape as Start, and for
	// the same reason. See Runtime.watchHere.
	//
	// The cancel is kept so "Stop watching" can end the WAIT as well as the session;
	// otherwise a goroutine outlives the decision and starts observing minutes later.
	watchMu     sync.Mutex
	watchArming bool
	// watchSession is the session Light Mode started, so teaching can take the slot back
	// from it and from nothing else. See Runtime.yieldWatching.
	watchSession observe.SessionID
	watchCancel  context.CancelFunc

	// collector and history are the perception side: what ran, and what the last few
	// cycles produced. The history is diagnostics only — nothing plans from it.
	collector *providers.Collector
	history   *observation.History
	// lastReport is the most recent fusion report, paired with the newest cycle.
	lastReport fusion.Report
	// lastWorld is the newest world, held only so the provenance invariant can be
	// CHECKED rather than claimed.
	lastWorld *directorapi.WorldState
	// timeline folds each cycle into an ordered event log, so a front-end can show
	// CHANGE rather than only a snapshot. Diagnostics: nothing plans from it, and
	// removing it would alter no behaviour.
	timeline *timeline.Recorder
	// epoch identifies this service instance, so a client can tell a restart (sequence
	// numbers begin again) from a log rollover. Opaque; only equality is meaningful.
	// See runtimeEpochCounter for why a timestamp alone is not enough to make it unique.
	epoch string
	// explainer is the engine's ability to account for a retained cycle.
	explainer fusion.Explainer
	// engine is the fusion engine, held so a diagnostics pass can run a full cycle.
	engine fusion.Engine
	// ocr is the text provider. Opt-in: it observes only when a request names it.
	ocr *ocr.Provider
	// ocrUnavailable explains why OCR cannot run, when it cannot. Held so a diagnostic
	// can say "tesseract is not installed" rather than "this window has no text".
	ocrUnavailable string
	// capture photographs windows for OCR.
	ocrBridge *bridgehost.Host
	// winTracker validates that the window about to be photographed still exists, still
	// belongs to the expected application, and is where the platform says it is NOW.
	// Without it a destroyed handle kept its remembered rectangle and the capture read
	// another monitor — see activeWindow and internal/director/perception/windowref.
	winTracker *windowref.Tracker
	// winDirectory issues the ephemeral ids `director windows` hands out, and winPlatform
	// is the live desktop both it and the tracker ask.
	winDirectory *windowref.Directory
	// observations owns the one active passive session and the recent finished ones.
	observations *observationRegistry
	// teach owns the one teaching session, when somebody is teaching.
	//
	// Beside the observation registry rather than inside it, because teaching is a
	// conversation ABOUT observation rather than a kind of it — and because a Director that
	// could not be taught should still observe.
	teach *teaching
	// teachGround is what turns a screen the teach session decided about into where it is on
	// the display, and the record of the frames those measurements were taken against.
	//
	// Beside the coordinator rather than inside it: `internal/director/teach` must not be able
	// to reach the platform at all, which is what its boundary test holds.
	teachGround *teachGrounding
	// liveMarco is the Marco runner over the REAL hosts.
	//
	// Held so an explicitly-live rehearsal can be handed one. Never installed by default:
	// `director rehearse` uses the recording host unless `--live` is given, and nothing
	// below cmd/director can obtain this field. See [[ADR-024-a-dry-step-is-not-evidence]].
	liveMarco directorapi.MarcoRunner
	// pinnedWindow, when set, is the EXPLICITLY selected window for the pass in flight.
	// It overrides "whatever is in front" for exactly the duration of that pass — see
	// ReadVision and activeWindow. Guarded by mu, which the pass already holds.
	pinnedWindow *windowref.Ref
	winPlatform  windowref.Platform
	// owner decides whether MARCO'S OWN control surface is the thing in front, so input
	// aimed at Marco never becomes evidence of what the person was demonstrating. See
	// surfaceowner.go.
	owner surfaceOwner
	// passesFor substitutes the pass runner a teach attempt uses. Nil in production; see
	// teachingPasses.
	passesFor func(windowref.Selector) teach.Passes
	// visual observes appearance and change in bounded regions. Opt-in like OCR, and
	// used by the pipeline's retry guard.
	visual *visualstate.Provider
	// vision detects UI elements in a captured frame. Opt-in like OCR: it observes only
	// when a request names it, because a capture plus a detection pass is expensive and
	// usually changes nothing.
	vision *vision.Provider
	// visionUnavailable is why vision cannot run, empty when it can. Held so a
	// diagnostic can say "no model is installed" rather than "this window is empty".
	visionUnavailable string
	visionBridge      *bridgehost.Host
	// shadowVision is an experimental detector running beside authoritative perception.
	//
	// Nil unless explicitly requested — see shadowwiring.go. Its evidence is collected into
	// Cycle.Shadow and never reaches fusion; it is held here only so a diagnostic can report
	// what it saw and what it cost.
	shadowVision      *shadow.Provider
	shadowBridge      *bridgehost.Host
	shadowUnavailable string
	// navSource observes the player's NAVIGATION — closed-vocabulary intents, never keys.
	// Held here so an observation session can subscribe for its lifetime and detach when
	// it ends; retention is session-bounded, never Director-lifetime.
	navSource      *navsource.Source
	navUnavailable string
	// lastWatched is the newest world an observation SESSION fused, held for the
	// perception surfaces.
	//
	// Separate from lastWorld, which is the foreground pipeline's. A session fuses its own
	// world for its pinned window and never touches lastWorld — so a surface reading only
	// lastWorld describes whatever happens to be in front while Marco is watching
	// something else entirely. Observed live: Light Mode reported "1 control I could aim
	// at" against a Settings window offering forty.
	//
	// Held under diagMu with the rest of the diagnostic snapshots, and read by whichever
	// of the two is FRESHER — so neither surface has to know whether a session is running.
	lastWatched *directorapi.WorldState
	// uia is the accessibility client, held so a rehearsal can resolve a demonstrated
	// target's name against the live tree at emission time. The same client the perception
	// pipeline observes through; a second one would be a second bridge process.
	uia *uiaclient.Provider
	// semanticMemory is what earlier sessions established, surviving restarts.
	semanticMemory            *semanticmemory.Store
	semanticMemoryUnavailable string
	// waits answers semantic conditions by observing, replacing the fixed settle delay.
	waits *waitengine.Waiter
	// edits keeps the recent editing outcomes, for `director edit`. Diagnostics only.
	edits *editHistory
	// actions keeps the recent semantic action outcomes, for `director explain action`.
	// Diagnostics only: which verb was asked for, which implementation the capability
	// ladder chose, and which stronger ones were unavailable.
	actions *actionHistory
	// effects is the ONE path to a desktop effect: every operation is lowered to
	// Marco, compiled and run. Nothing else in the Director may reach a host.
	effects *marcoexec.Executor
	// lowerings keeps the recent lowered operations and their generated source.
	lowerings *loweringHistory
	// traces holds the recent command phase traces. Bounded, timings only.
	traces *trace.History
	// confirmations is how the Director asks a person to agree to something before it
	// happens. Built at construction and installed as the pipeline's Confirmer, so the
	// production request path is the one the regression suite already covers — there is
	// no second execution mode and no build-time special case.
	confirmations *service.ConfirmationBroker
	// vars is the Director.s semantic memory, owned by the SERVICE so separate
	// client processes reach the same variables.
	vars *variables.Store
	// demos records what the user DEMONSTRATES, and demoStore is where recorded
	// demonstrations and approved procedures live.
	//
	// Owned by the service for the same reason the variables are, and more so: a
	// demonstration spans several requests, and the CLI is a fresh process each time. The
	// recorder subscribes to Handle's own outcome and observes nothing of its own.
	demos     *demo.Recorder
	demoStore *demo.Store
	// goals is the live procedure registry, held so an approved procedure can be
	// registered into the running service rather than only after a restart.
	goals *goal.Registry
	// games is the capability-pack registry, and gameState what it currently detects.
	//
	// The registry is built at the composition root and never modified afterwards; the
	// state is refreshed on every observation and guarded by its own lock, so a policy
	// decision and a `director game` query both read it without touching the command
	// lock. See gamewiring.go.
	games     *game.Registry
	gameState gameState
	// activeValues is the running program.s environment, held only while it runs. See
	// valuediag.go: it is a reference for READING, and the program keeps ownership.
	activeValues *values.Environment
	// activeCollections is the running program.s collection environment, held only
	// while it runs. Guarded by valuesMu alongside activeValues.
	activeCollections *collections.Environment
	// valuesMu guards it. Separate from mu so a status request is answerable while a
	// command is mid-execution — which is the whole point of a control plane.
	valuesMu sync.RWMutex

	// paused is a program waiting for a clarification answer, so answering RESUMES
	// it rather than restarting it. See resume.go.
	paused *pausedProgram
	// pausedMu guards it, and is NOT mu for the same reason valuesMu is not: the
	// diagnostics answer from the paused program when nothing is running, so reading it
	// under the command lock made `director status`, `director collections` and
	// `explain value` all block behind whatever desktop work happened to be in flight —
	// the control plane going silent exactly while a slow command made someone want to
	// ask what it was doing.
	//
	// Ordering: the command path takes mu and then pausedMu; the control plane takes
	// pausedMu alone. Never the reverse, so the pair cannot deadlock.
	pausedMu sync.RWMutex

	// diagMu guards the diagnostics fields, which are written on the command
	// goroutine and read on connection goroutines.
	diagMu sync.RWMutex

	// mu serialises Handle. The registry already permits only one mutating command,
	// but the pipeline holds a stateful world Builder whose element identity depends
	// on snapshots arriving in order, so concurrent use would corrupt it.
	mu sync.Mutex
}

var _ service.Runtime = (*Runtime)(nil)

// NewRuntime builds the Director once.
func NewRuntime(bridgePath string, maxNodes int, dryRun bool, g *actiongraph.FileGraph) (*Runtime, error) {
	bridge := bridgehost.New(bridgePath)
	accessibility := uiaclient.New(bridge)
	accessibility.MaxNodes = maxNodes
	windows := winprovider.New()
	tracker := service.NewProviderTracker()

	var osHost runtime.Host = oshost.New()
	if dryRun {
		osHost = runtime.DryRunHost{}
	}

	// The editing primitives go through Marco the LANGUAGE, not through a host call.
	//
	// The runner compiles a generated program — lexer, parser, graph, compiler — and
	// only then executes it, which means a capability Marco does not export fails at
	// compile time, before any desktop mutation. The acts are keyed by the names
	// os.marco and accessibility.marco declare, because that is what the runtime looks up when
	// it reaches a foreign action node.
	marco := marcorunner.New(map[string]runtime.Host{
		"OS":            osHost,
		"Accessibility": bridge,
	})
	// ONE executor for every desktop effect. It is the Actuator, the Focuser, the
	// value provider and the clipboard — so there is no other implementation left for
	// the Director to reach, which is what makes "no direct host calls" structural
	// rather than a rule someone has to remember.
	effects := marcoexec.New(marco)

	// Perception is a pipeline now, not a function that knows about accessibility.
	//
	// The collector runs whatever providers are registered and produces a cycle of
	// EVIDENCE; the engine turns that cycle into BELIEF. Nothing between this line and
	// the planner knows that accessibility exists. Adding OCR is adding one provider
	// to this list.
	// OCR is registered as a provider but is OPT-IN: it emits nothing unless a request
	// names SourceOCR. Registering it unconditionally would put a screen capture and a
	// tesseract subprocess on the path of every click, to produce evidence that usually
	// changes nothing. The provider enforces that itself rather than trusting callers.
	textEngine, textHost, textReason := newOCREngine(defaultOCRBridge())
	rt := &Runtime{graph: g, tracker: tracker, bridge: bridge, ocrBridge: textHost,
		ocrUnavailable: textReason, liveMarco: marco, uia: accessibility}
	rt.winTracker = windowref.NewTracker(windows)
	rt.winDirectory = windowref.NewDirectory()
	// Durable semantic memory, opened once for the life of the service.
	//
	// Failure here is NOT fatal and never has been allowed to be: a Director whose memory
	// file is corrupt must still perceive, still execute, and still ask. The reason is
	// carried so every report can say why nothing was recognised — "Marco did not recognise
	// this" and "Marco could not read its memory" are different sentences, and only one of
	// them is about the screen.
	rt.semanticMemory, rt.semanticMemoryUnavailable = semanticmemory.Open(semanticMemoryPath())
	rt.observations = newObservationRegistry().withMemory(rt.semanticMemory)
	// Somewhere for the one teach session to live. Empty and inert until somebody asks to
	// teach something; it holds no authority, and a restart begins with nothing.
	rt.teach = &teaching{}
	rt.winPlatform = windows

	// Provenance, wired once, here. Both the tree walk and the pixels have to be able to
	// prove which window generation they describe, and they establish it by different
	// routes — the bridge names the window it walked, the capture names the window it
	// photographed — which is exactly why one shared resolver serves both. It resolves;
	// it never selects.
	resolver := newTargetResolver(rt.winTracker)
	capture := newCapture(windows)
	// The WINDOW capture is wrapped rather than modified: attribution happens strictly
	// AFTER the pixels are in hand, and a decorator is the one arrangement where that
	// ordering cannot be got wrong later.
	//
	// Region capture is deliberately not wrapped. A rectangle of the desktop is not a
	// picture of a window and has no generation to carry; visual state, its only consumer,
	// reports that something CHANGED rather than what it was, and is not a collector
	// provider. Wrapping it would have to invent a provenance, which is worse than none.
	provenCapture := providers.NewProvenCapture(capture, resolver)
	// Every input operation confirms the intended window is still in front, per
	// operation and immediately before lowering. A previous focus proves nothing at
	// the instant of execution — see foregroundGuard for the incident behind this.
	rt.effects = effects.
		WithGuard(foregroundGuard{observe: rt.observeForGuard, activate: activateApp(effects)}).
		WithRecorder(rt.recordLowering)

	rt.ocr = ocr.New(textEngine, provenCapture, rt.activeWindow)
	rt.visual = visualstate.New(capture)

	// The vision provider, on the same terms as OCR: opt-in, over the bridge, and absent
	// on most machines. A Director with no detector behaves exactly as it did before it
	// existed — nothing asks for vision unless it means to.
	detector, visionHost, visionReason := newVisionDetector(defaultVisionBridge())
	rt.visionBridge, rt.visionUnavailable = visionHost, visionReason
	rt.vision = newVisionProvider(detector, provenCapture, rt.activeWindow)

	// The detector borrows the OCR engine to read the words inside its own boxes, so a
	// detected control arrives NAMED rather than as an anonymous rectangle. This is the
	// only place that knows both exist; neither provider imports the other.
	//
	// Both must be installed for it to do anything. With no OCR the reader is nil and a
	// detection is the unnamed shape it has always been — which is a smaller loss than it
	// sounds, because fusion still names anything the accessibility tree covers.
	rt.vision.Reader = newLabelReader(textEngine)

	// The capability packs are built here because the observe closure below enriches
	// every world with what they make of it. They contribute no perception source: only
	// internal/director/perception may see evidence, so a pack is handed the FUSED
	// elements and says which of them are inventory slots. See game.Registry.Enrich.
	packRegistry, packErr := newGameRegistry()
	if packErr != nil {
		return nil, packErr
	}
	sources := []observation.Provider{
		providers.NewAccessibility(accessibility).WithTargetResolver(resolver),
		providers.NewWindowSystem(windows),
		rt.ocr,
		// Vision runs in the same collector as everything else, which is the whole
		// point: the Director cannot tell where an observation came from, and adding a
		// source is adding a line here. It is opt-in, so an ordinary cycle skips it.
		rt.vision,
	}
	// An experimental detector, only when one was asked for. It is registered like any
	// other provider — the collector routes its evidence away from belief because of what
	// it IS (an observation.ShadowProvider), not because of where it sits in this list.
	// Nothing here decides admissibility, and nothing here could.
	// The experiment gets the SAME label reader, for the same reason the authoritative
	// detector has one: a box with no name is a shape nobody can ask for, and the surfaces
	// this detector exists for are exactly the ones where nothing else supplies a name.
	// Without this argument ScreenParser reaches the session as roles and rectangles and its
	// screens can never acquire a semantic discriminator — the gap
	// [[Experiment-009-ocr-as-a-semantic-discriminator]] measured and could not close.
	rt.shadowVision, rt.shadowBridge, rt.shadowUnavailable =
		newShadowVision(provenCapture, rt.activeWindow, newLabelReader(textEngine))
	if rt.shadowVision != nil {
		sources = append(sources, rt.shadowVision)
	}
	// The navigation observer. Deliberately NOT a perception provider: it observes the
	// player rather than the screen, its evidence has no authority, and routing it through
	// the collector would put it one wiring mistake away from fusion. It is held on the
	// runtime and read only by an observation session that subscribed.
	rt.navSource, rt.navUnavailable = navsource.New()
	collector := providers.NewCollector(sources...)
	engine := fusion.NewEngine()
	history := observation.NewHistory(observation.DefaultHistory)
	rt.timeline = timeline.New(timelineEvents)
	// The epoch is what lets a client tell a RESTART from a rollover: after a restart
	// sequence numbers begin again at 1, which to a client holding a high cursor looks
	// exactly like a log that rolled past it.
	//
	// The counter is not decoration. `UnixNano` is nanosecond-TYPED but not
	// nanosecond-RESOLVED — the Windows system clock advances in ~15ms steps, so two
	// runtimes built back to back within one tick read the same timestamp and produced the
	// same epoch, which is precisely the case the epoch exists to distinguish. It showed up
	// as a flaky test; the flake was the real defect, in-process, reported honestly.
	rt.epoch = fmt.Sprintf("%d-%d-%d",
		os.Getpid(), time.Now().UnixNano(), runtimeEpochCounter.Add(1))

	// The engine can account for what it did. Asserted here rather than assumed: the
	// explain path is optional on the interface, and a silent fallback to "no
	// explanation available" would be indistinguishable from a bug.
	explainer, _ := engine.(fusion.Explainer)

	rt.collector, rt.history, rt.explainer, rt.engine = collector, history, explainer, engine

	observe := func(ctx context.Context) (directorapi.WorldState, error) {
		cycle := collector.Collect(ctx, observation.Request{})
		w, report, err := engine.Fuse(cycle)
		if err != nil {
			return directorapi.WorldState{}, err
		}
		// What the capability packs make of what was seen. It adds ENTITIES to elements
		// the fusion engine already produced and nothing else: no element is created,
		// removed or re-identified, and an element no pack recognises is untouched.
		packRegistry.Enrich(&w)
		// Bounded history, for diagnostics only. The observations themselves are
		// ephemeral and go with the cycle when it falls off the end; what survives is
		// the world, and the provenance references on its elements.
		rt.record(cycle, report, &w)
		// Which application a capability pack serves, re-decided against THIS world.
		// Detection that lagged behind the screen would apply one game's safety
		// declaration to another game's window — and it costs nothing: a pack's Detect
		// is a comparison over labels and a process record.
		rt.detect(&w)
		// Every observation feeds the lifecycle tracker, which is what makes
		// provider persistence visible in `status` rather than merely true.
		tracker.Observe(w)
		return w, nil
	}

	// The wait engine observes through the SAME pipeline a command does — collector,
	// fusion, identity. A wait that observed by some other route would be answering
	// conditions against a world nothing else plans from.
	rt.waits = waitengine.New(observe)

	// Editing is semantic: the Director plans TEXT STATE and this editor chooses how
	// to reach it — the control.s own value API first, then selection, then a borrowed
	// clipboard, then typing. Marco supplies the primitives; the Director never became
	// a keyboard macro engine.
	rt.edits = &editHistory{}
	rt.actions = &actionHistory{}
	// The procedure registry is validated HERE, at construction, so a duplicate or
	// permanently shadowed registration stops the service before it serves anything.
	// Deferring it would mean the first user to ask for that goal discovers it, mid
	// request, on a live desktop.
	goals, gerr := goal.NewValidatedRegistry()
	if gerr != nil {
		return nil, gerr
	}
	// What the user has TAUGHT this Director, loaded before it serves anything and
	// registered into the same registry as the built-ins. A malformed learned procedure
	// stops the service for the reason a malformed built-in does: a procedure the user
	// believes they taught, silently absent, is discovered when the Director does
	// something else instead.
	demoStore, derr := demo.Open(configDir())
	if derr != nil {
		return nil, derr
	}
	// The capability packs' procedures, before the registry is validated: a pack's
	// procedures are ordinary procedures, so a pack that shadowed a built-in is caught by
	// exactly the check that catches a built-in shadowing another. The registry itself
	// was built earlier, because the collector needed its observers.
	rt.games = packRegistry
	packRegistry.RegisterProcedures(goals)
	demoStore.Register(goals)
	if shadowed := goals.Validate(); len(shadowed) > 0 {
		return nil, fmt.Errorf(
			"a learned procedure makes the registry unusable: %s is shadowed by %s — %s.\n"+
				"Remove it with: director procedures --forget %q",
			shadowed[0].Procedure, shadowed[0].By, shadowed[0].Reason, shadowed[0].Procedure)
	}
	rt.goals, rt.demoStore = goals, demoStore
	rt.demos = demo.NewRecorder()
	// A finished session is persisted the moment it closes, by the recorder itself.
	// Anything else would mean a demonstration that existed only in memory until someone
	// asked for it.
	rt.demos.OnComplete = func(d *demo.Demonstration) {
		if err := demoStore.SaveDemonstration(d); err != nil {
			fmt.Fprintf(os.Stderr, "director: could not store demonstration %s: %v\n", d.ID, err)
		}
	}
	// One store, owned by the long-lived service, so separate CLI invocations reach the
	// same variables. A load failure is surfaced rather than swallowed: silently
	// starting empty would look like the user never taught the Director anything.
	vars, verr := variables.Open(configDir())
	if verr != nil {
		return nil, verr
	}
	rt.vars = vars
	rt.lowerings = &loweringHistory{}
	rt.traces = trace.NewHistory()
	// The confirmer, installed HERE rather than left to a front-end. A daemon that
	// could not ask blocked every destructive action, which was safe and useless; a
	// daemon that assumed yes would be neither. The broker asks whichever client is
	// watching the command and blocks for the answer.
	rt.confirmations = service.NewConfirmationBroker()
	// Values, Focus, Input and Clipboard all come from the Marco executor now: the
	// four rungs of the ladder are four Marco capabilities, not four host calls.
	// rt.effects, not the bare executor: the guarded, recording copy is the one that
	// must be used everywhere. Using the unguarded original anywhere silently removes
	// the foreground protection from that path.
	editor := newEditor(rt.effects, rt.effects, guardedInput{exec: rt.effects}, rt.effects, observe, rt.waits)

	if !rt.effects.Guarded() {
		// A programming error, not a runtime condition: the guarded copy is built a
		// few lines above and there is no supported configuration without it.
		panic("director: the effects executor reached the pipeline without a foreground guard")
	}
	rt.pipeline = &execute.Pipeline{
		Observe:  observe,
		Intent:   intent.New().Parse,
		Resolver: target.NewResolver(),
		Planner:  plan.New(),
		// Policy, plus what the capability packs declare about the applications they
		// serve. A contributed rule may only NARROW a decision — see
		// internal/director/policy/contributed.go — so adding a pack can make the
		// Director more cautious and can never make it less.
		Policy: gamePolicy(packRegistry, rt.DetectedGame),
		// Goals turn "rename this file to Budget" into a program. The application is
		// read from the world at expansion time so an override — Explorer's rename,
		// VS Code's rename-symbol — is chosen against what is actually in front of the
		// user rather than against whatever was in front when the service started.
		Goals: &execute.Goals{
			Registry:    goals,
			Application: rt.activeApplication,
		},
		// The production confirmation gate. The same field the unit tests set, reached
		// by the same requests, so the daemon's path IS the tested path.
		Confirmer: rt.confirmations,
		// Verification, plus the evidence the packs can produce and the Director cannot —
		// "the craft queue gained an entry" is a fact about a craft queue. Additive,
		// weighed and capped: see internal/director/verify/contributed.go.
		Verifier: gameVerifier(packRegistry),
		// Reading the result back off disk, which is the only way to tell "the file you
		// pointed at became this name" from "something with this name exists". READ-ONLY
		// by type — see resources.go.
		Resources: osResources{},
		Graph:     g,
		Monitors:  windows.Monitors,
		// Watching a bounded region around the target is what lets a slow navigation be
		// distinguished from nothing happening — the case that made a retry click Back
		// twice. One small rectangle per action, never the desktop.
		// Provisional bounds on every phase that can block on something outside the
		// Director. See trace.DefaultDeadlines for how they were chosen.
		Deadlines: trace.DefaultDeadlines(),
		Watcher:   &regionWatcher{provider: rt.visual, window: rt.activeWindow},
		// Waits are answered from OBSERVATIONS, so the engine observes through exactly
		// the path a command does — the same collector, fusion and identity.
		Variables:     rt.vars,
		OnEnvironment: rt.setActiveValues,
		OnCollections: rt.setActiveCollections,
		Waiter:        rt.waits,
		// Reading a value is not a lesser version of writing one, and it goes through
		// the same guarded executor: a control value is read through the accessibility
		// value API, and the clipboard through legal Marco. Nothing here reaches a host
		// directly.
		//
		// ControlValues is typed as the READ half only (execute.ControlReader), so a
		// capture cannot set a value even though the executor behind it can.
		ControlValues: rt.effects,
		Clipboard:     rt.effects,
		Executor: &execute.Executor{
			Actuator: rt.effects,
			Focus:    rt.effects,
			Resolve: func(ref directorapi.ElementReference) (directorapi.ResolvedTarget, error) {
				return resolveRef(observe, ref)
			},
			Lowerings:    rt.lowerings,
			Editor:       editor,
			EditRecorder: rt.edits.record,
			// The runner a semantic action.s chosen mechanism lowers into. Without it a
			// semantic action fails rather than degrading to a click.
			Operations:       rt.effects,
			SemanticRecorder: rt.actions.record,
		},
	}
	return rt, nil
}

// SetConfirmer replaces the confirmer the runtime built for itself.
//
//	nil confirmer → unavailable, and no action may execute after it.
//
// The daemon installs a working one at construction (see NewRuntime), so this exists for
// a front-end that can present a better prompt than the wire protocol does — not as the
// only way to get one. Passing nil disables confirmation entirely, which BLOCKS every
// action that needs it rather than allowing it; that is the safe direction and the only
// one available.
func (r *Runtime) SetConfirmer(c execute.Confirmer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pipeline.Confirmer = c
}

// Confirmations is the broker the service publishes questions through.
//
// Never nil for a runtime built by NewRuntime. The service treats nil as "this Director
// cannot ask", which blocks; there is no configuration in which a missing broker becomes
// a yes.
func (r *Runtime) Confirmations() *service.ConfirmationBroker { return r.confirmations }

// windowFreshness is how old the window list may be before Windows looks again.
//
// A second, because that is roughly how long a person waits between opening a window and
// asking about it. Shorter would observe on every status poll; longer would report a
// window that has since closed as though it were there.
const windowFreshness = time.Second

// Windows is what the Director can currently see.
//
// A control-plane method with one unusual property: it may OBSERVE. That is deliberate
// and it is the difference between "what the Director can see" and "what it happened to
// see once at startup" — the second is useless to a front-end asking which window it is
// about to act in, and was exactly what a stale answer here produced.
//
// Observing is not a desktop effect. It reads the accessibility tree; it focuses nothing,
// clicks nothing and types nothing, which is the same reason expansion is allowed to
// observe before the confirmation gate.
//
// The lock is TRIED, never taken. The world builder is stateful and expects snapshots in
// order, so observing while a command runs would corrupt it — and blocking here would
// make `director status` hang behind whatever desktop work was in flight, which is the
// control-plane failure this codebase already has a rule about. So a busy Director
// answers from its last observation, which is the right answer while it is mid-command
// anyway.
func (r *Runtime) Windows() []directorapi.Window {
	if r.stale() && r.mu.TryLock() {
		_, _ = r.pipeline.Observe(context.Background())
		r.mu.Unlock()
	}
	r.diagMu.RLock()
	defer r.diagMu.RUnlock()
	if r.lastWorld == nil {
		return nil
	}
	return append([]directorapi.Window{}, r.lastWorld.Windows...)
}

// stale reports whether the last observation is old enough to be worth repeating.
func (r *Runtime) stale() bool {
	r.diagMu.RLock()
	defer r.diagMu.RUnlock()
	return r.lastWorld == nil || time.Since(r.lastWorld.Timestamp) > windowFreshness
}

// Warm brings the accessibility client up before the first command.
//
// Attaching early is the point: Chromium enables its renderer accessibility only
// after sustained client presence, so a service that waits until the first request
// to attach hands that request the same cold, shallow tree the old model always saw.
func (r *Runtime) Warm(ctx context.Context) {
	_, _ = r.pipeline.Observe(ctx)
}

// Handle runs one phrase.
func (r *Runtime) Handle(ctx context.Context, phrase string, progress func(service.ProgressPayload)) execute.Outcome {
	if refusal, refused := r.refuseWhileObserved(); refused {
		return refusal
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if progress != nil {
		r.pipeline.OnProgress = func(ev execute.Progress) {
			progress(service.ProgressPayload{
				Stage: ev.Stage, Iteration: ev.Iteration, Total: ev.Total,
				Detail: ev.Detail, Verified: ev.Verified,
			})
		}
		// Step boundaries reuse Iteration/Total, which is what makes a status query
		// during a program answer "step 3 of 5" rather than a bare "busy".
		r.pipeline.OnProgram = func(ev execute.ProgramProgress) {
			progress(service.ProgressPayload{
				Stage:     "step",
				Iteration: ev.Index, Total: ev.Total,
				Detail:   fmt.Sprintf("%s — %s", ev.Phrase, ev.Status),
				Verified: ev.Status == "verified",
			})
		}
		defer func() { r.pipeline.OnProgress = nil; r.pipeline.OnProgram = nil }()
	}
	// Cancellation is now service-native: the context carries it. The legacy stop
	// file is not consulted here at all.
	r.pipeline.StopCheck = nil
	t := r.beginTrace(phrase)
	out := r.pipeline.HandleRequest(ctx, phrase)
	r.finishTrace(t, string(out.Status))
	r.pauseProgram(phrase, out)
	// The demonstration recorder's ONE subscription: a request that has finished being
	// observed, executed, verified and recorded. It observes nothing itself, which is why
	// recording cannot bypass verification — there is nothing here to record until the
	// pipeline has verified it. A no-op when no session is open.
	if r.demos != nil {
		r.demos.Observe(out)
	}
	return out
}

// record files one cycle and its fusion report for diagnostics.
//
// Held under the same lock as the pipeline because observe runs inside Handle, and a
// diagnostics request arrives on a different goroutine while it does.
func (r *Runtime) record(cycle observation.Cycle, report fusion.Report, w *directorapi.WorldState) {
	r.history.Record(cycle)
	r.diagMu.Lock()
	r.lastReport = report
	r.lastWorld = w
	// The event log is folded here, under the lock that already guards the diagnostic
	// state, because this is the one place every completed cycle passes through with
	// its report and its world. A recorder fed anywhere else would see a subset.
	if r.timeline != nil {
		r.timeline.Observe(cycle, report, w)
	}
	r.diagMu.Unlock()
}

// World reports the Director's current BELIEF, as entities.
//
// # It observes nothing
//
// This copies the world already held. It starts no cycle, attaches no provider, captures
// no screen and mutates nothing — the same contract Windows() follows. Asking what the
// Director believes must never change what it believes, or a HUD polling twice a second
// would be steering the thing it is describing.
func (r *Runtime) World(p service.WorldPayload) service.WorldResponse {
	r.diagMu.RLock()
	defer r.diagMu.RUnlock()

	out := service.WorldResponse{}
	if r.lastWorld == nil {
		// No world yet is DIFFERENT from a world with nothing in it, and a client
		// showing "0 entities" for the first is telling a lie about the second.
		return out
	}
	w := r.lastWorld
	out.Believed = true
	out.Observed = w.Timestamp
	out.FreshnessMS = time.Since(w.Timestamp).Milliseconds()
	if w.ActiveApp != nil {
		out.App = w.ActiveApp.ID
	}
	out.Total = len(w.Elements)

	limit := p.Limit
	if limit <= 0 || limit > worldEntityCap {
		limit = worldEntityCap
	}

	// Sorted by id so the same world always serialises the same way. Map iteration
	// would reorder the list between polls, and a HUD would show it churning.
	ids := make([]directorapi.ElementID, 0, len(w.Elements))
	for id := range w.Elements {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	now := time.Now()
	for _, id := range ids {
		el := w.Elements[id]
		if el == nil {
			continue
		}
		if p.Actionable && !el.Actionable() {
			continue
		}
		if len(out.Entities) >= limit {
			out.Truncated = true
			break
		}
		out.Entities = append(out.Entities, r.worldEntity(id, el, now))
	}
	return out
}

// worldEntity converts one believed element into its wire form.
//
// Called with diagMu held.
func (r *Runtime) worldEntity(id directorapi.ElementID, el *directorapi.Element,
	now time.Time) service.WorldEntity {

	sources := make([]string, 0, len(el.Sources))
	for _, s := range el.Sources {
		sources = append(sources, string(s))
	}

	// The SAME classifier a passive observation session uses. Not a copy of the
	// allowlist: two copies of that rule would eventually disagree, and the one that
	// disagreed quietly would be the one on the wire.
	label := observe.Classify(el.Label, el.LabelConfidence,
		observe.LabelContext{Role: el.Role, Sources: sources},
		observe.DefaultLabelPolicy())

	acts := el.Actions()
	e := service.WorldEntity{
		Identity:        entityIdentity(id),
		Role:            string(el.Role),
		Label:           label,
		Confidence:      el.Confidence,
		LabelConfidence: el.LabelConfidence,
		Sources:         sources,
		Actionable:      el.Actionable(),
		Targetable:      acts.Targetable(),
		Focusable:       acts.Focusable,
		Invokable:       acts.Invokable,
		Enabled:         el.Enabled,
		Visible:         el.Visible,
	}
	if r.timeline != nil {
		st := r.timeline.Stability(id)
		e.StableCycles = st.Cycles
		e.Stable = st.Promoted
		if !st.FirstSeen.IsZero() {
			e.AgeMS = now.Sub(st.FirstSeen).Milliseconds()
		}
	}
	return e
}

// Events returns the perception event log from a cursor.
func (r *Runtime) Events(p service.EventsPayload) service.EventsResponse {
	r.diagMu.RLock()
	defer r.diagMu.RUnlock()

	out := service.EventsResponse{Epoch: r.epoch}
	if r.timeline == nil {
		return out
	}
	events, newest := r.timeline.Since(p.Cursor, p.Limit)
	out.Events = events
	out.Newest = newest
	out.Oldest = r.timeline.Oldest()
	return out
}

// entityIdentity is a stable, opaque handle for one believed element.
//
// A digest rather than the ElementID itself. The id is stable, which is what a client
// needs, but it may encode a platform RuntimeId or a window handle — and a client that
// came to depend on one would be depending on the Director's storage schema. Hashing keeps
// the stability and drops everything else. Same rule as the ephemeral window ids.
func entityIdentity(id directorapi.ElementID) string {
	sum := sha256.Sum256([]byte(id))
	return hex.EncodeToString(sum[:6])
}

// worldEntityCap bounds a world listing.
//
// A warm editor reports thousands of elements, and a payload carrying all of them would be
// unusable on a HUD refreshing twice a second. Total is reported separately so a truncated
// list is never mistaken for the whole world.
const worldEntityCap = 200

// timelineEvents is how many events the log retains.
//
// Bounded because it runs for the life of the service and nothing prunes it otherwise. A
// client polling at any sane rate stays well inside it; one that falls further behind
// learns it lost events from the Oldest sequence rather than being told a comfortable lie.
const timelineEvents = 512

// Perception reports what the providers are and what the recent cycles looked like.
func (r *Runtime) Perception() diagnostics.Perception {
	r.diagMu.RLock()
	report := r.lastReport
	r.diagMu.RUnlock()
	snap := diagnostics.Build(r.collector, r.history, report)
	snap.ProvenanceOK = r.provenanceOK()
	return snap
}

// Explanation is Perception plus the account of every element in the latest cycle.
func (r *Runtime) Explanation() diagnostics.Perception {
	r.diagMu.RLock()
	report := r.lastReport
	r.diagMu.RUnlock()
	snap := diagnostics.BuildWithExplanation(r.collector, r.history, report, r.explainer)
	snap.ProvenanceOK = r.provenanceOK()
	return snap
}

// provenanceOK checks the definition-of-done claim against the actual world rather
// than asserting it.
//
// It was hardcoded true when provenance was added, which is exactly the kind of claim
// that keeps being made after it stops being true. Checking costs one pass over the
// newest world.
func (r *Runtime) provenanceOK() bool {
	r.diagMu.RLock()
	defer r.diagMu.RUnlock()
	return r.lastWorld != nil && diagnostics.AllElementsHaveProvenance(r.lastWorld)
}

// Graph is the durable action history.
func (r *Runtime) Graph() actiongraph.Graph { return r.graph }

// Providers reports accessibility lifecycle.
func (r *Runtime) Providers() []service.ProviderStatus { return r.tracker.Status() }

// AttachedAt is when the accessibility client came up.
func (r *Runtime) AttachedAt() time.Time { return r.tracker.AttachedAt() }

// Close releases the accessibility bridge.
func (r *Runtime) Close() {
	// ASKED FOR, not assumed. The field is an interface so a test can substitute a host, and
	// a substitute has nothing to close.
	if c, ok := r.bridge.(interface{ Close() error }); ok {
		_ = c.Close()
	}
	if r.navSource != nil {
		// Unhooks and stops the worker. A low-level hook that outlived its process would
		// be the kind of leak nobody notices until the keyboard misbehaves.
		_ = r.navSource.Close()
	}
}

// HandleClarified re-runs a phrase with a clarification answer applied.
func (r *Runtime) HandleClarified(ctx context.Context, phrase string,
	refinement intent.Refinement, progress func(service.ProgressPayload)) execute.Outcome {

	r.mu.Lock()
	defer r.mu.Unlock()

	if progress != nil {
		r.pipeline.OnProgress = func(ev execute.Progress) {
			progress(service.ProgressPayload{
				Stage: ev.Stage, Iteration: ev.Iteration, Total: ev.Total,
				Detail: ev.Detail, Verified: ev.Verified,
			})
		}
		defer func() { r.pipeline.OnProgress = nil }()
	}
	r.pipeline.StopCheck = nil
	// A paused program resumes from the step that asked, keeping every completed step
	// completed. Only if there is none does the answer re-run the whole phrase.
	// A clarification answer continues the command that asked, so its trace is linked
	// rather than orphaned.
	t := r.beginTraceLinked(phrase, r.lastTraceID())
	if out, ok := r.resumeProgram(ctx, phrase, refinement); ok {
		r.finishTrace(t, string(out.Status))
		r.pauseProgram(phrase, out)
		return out
	}
	out := r.pipeline.HandleRefined(ctx, phrase, refinement.Apply)
	r.finishTrace(t, string(out.Status))
	r.pauseProgram(phrase, out)
	return out
}
