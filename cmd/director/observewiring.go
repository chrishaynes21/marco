package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Wiring a passive observation session to the live Director.
//
// Two adapters, and nothing else. Everything they need already exists: the window tracker
// validates and reacquires, the collector runs the providers, fusion produces the world,
// and the analysis core does the thinking. This is the seam where those meet, and it is
// deliberately thin — an adapter that started making decisions would be a fourth place
// where perception happens.

// ── the target ────────────────────────────────────────────────────────────────

// liveTarget resolves the session's selector against the real desktop.
//
// It adds no rules of its own. Selector semantics — which kinds may follow a restart, which
// name one generation and stop — belong to windowref and are enforced there, so an
// observation session cannot accidentally be more permissive than an ordinary command.
type liveTarget struct {
	tracker   *windowref.Tracker
	platform  windowref.Platform
	directory *windowref.Directory
}

var _ observesession.Target = (*liveTarget)(nil)

// Acquire validates the selected window, or explains why it cannot.
//
// Never falls back to the foreground window and never reuses old bounds: the two failures
// the stale-capture incident was made of. An error here means the runner takes no sample,
// which is the honest outcome.
func (t *liveTarget) Acquire(ctx context.Context, s windowref.Selector) (windowref.Ref, error) {
	if t.tracker == nil || t.platform == nil {
		return windowref.Ref{}, fmt.Errorf(
			"this Director has no window tracker, so no window can be validated")
	}
	v := t.tracker.AcquireBy(ctx, t.platform, t.directory, s)
	if !v.State.OK() {
		// The state is carried in the message rather than flattened away, so a reader
		// can tell "ambiguous" from "closed" from "the id went stale".
		return windowref.Ref{}, fmt.Errorf("%s (%s)", v.Reason, v.State)
	}
	return v.Ref, nil
}

// ── the sampler ───────────────────────────────────────────────────────────────

// liveSampler turns one validated window into one safe semantic snapshot.
//
// The path is the ordinary one: providers, then fusion, then a conversion that keeps only
// what the analysis core is allowed to see. No session-specific perception, no second
// fusion, and no interpretation — a sample is evidence, and what it MEANS is decided
// nowhere in this file.
type liveSampler struct {
	rt *Runtime
	// labels says whether this Director can read text inside detected controls at all.
	labels bool
	// nav is this session's navigation subscription, nil when no source is available.
	nav *navsource.Subscription
	// nameActivatedTargets is the licence under which an activatable control's name may
	// travel on a semantic target — the ONE control the person's own input landed on.
	//
	// Named for the permission rather than for the caller. It used to be `demonstration`,
	// set from `Episode.EstablishPlaces`, which meant this sampler's privacy behaviour was
	// decided by a field about establishing PLACES. The two were welded because Learn was
	// the only caller that set either; Roadmap 35A separated them, and the licence now
	// arrives as `Episode.NameActivatedTargets`.
	//
	// Set by the registry from the episode the CALLER declared; the zero value is the
	// passive default and admits nothing beyond the canonical role allowlist. The shape
	// filter is unconditional either way.
	nameActivatedTargets bool

	// frameMu guards frame, which is written from the sampling goroutine and read by
	// whatever wants to point at something.
	frameMu sync.Mutex
	frame   sampledFrame
}

// sampledFrame is the window rectangle the most recent sample's geometry is relative TO.
//
// # Why it is held here and nowhere else
//
// Because every region downstream is a proportion of it, and proportions of an unknown rectangle
// cannot be turned back into a place on a screen. `pkg/referent` needs exactly this and refuses
// without it.
//
// It is deliberately NOT on `observe.Sample`, `observesession.Result` or the service protocol. A
// window's desktop rectangle is one of the things the sample conversion drops on purpose — it
// describes somebody's monitor layout, it changes the moment the window moves, and a report that
// carried it would be a report that leaked it. Live process state, replaced every sample, gone
// when the process ends: the same lifetime as the window it describes.
type sampledFrame struct {
	Application string
	Window      directorapi.WindowID
	Generation  uint64
	Sequence    int
	Bounds      directorapi.Rect
	At          time.Time
}

// LastFrame is the frame the most recent sample was normalised against.
//
// ok is false before any sample has been taken, and for a frame with no usable extent — which is
// a refusal rather than a zero rectangle, because a caller handed 0x0 would divide by it.
func (s *liveSampler) LastFrame() (sampledFrame, bool) {
	s.frameMu.Lock()
	defer s.frameMu.Unlock()
	return s.frame, s.frame.Sequence > 0 &&
		s.frame.Bounds.Width > 0 && s.frame.Bounds.Height > 0
}

// recordFrame remembers what this sample's geometry is relative to.
func (s *liveSampler) recordFrame(req observesession.SampleRequest) {
	s.frameMu.Lock()
	defer s.frameMu.Unlock()
	s.frame = sampledFrame{
		Application: req.Window.Application,
		Window:      req.Window.ID,
		Generation:  req.Window.Generation,
		Sequence:    req.Sequence,
		Bounds:      req.Window.Bounds,
		At:          time.Now(),
	}
}

var _ observesession.Sampler = (*liveSampler)(nil)

// Sample performs one observation cycle against the validated window.
func (s *liveSampler) Sample(ctx context.Context, req observesession.SampleRequest) (observe.Sample, error) {
	if s.rt == nil {
		return observe.Sample{}, fmt.Errorf("no Director runtime is wired")
	}
	start := time.Now()

	// The window the runner validated, pinned for the duration of this cycle. Without
	// this the collector would look at whatever is in front — which, while somebody is
	// playing a game and the session runs in a service, is usually the game, and
	// occasionally is not. "Usually" is not a basis for attributing evidence.
	pinned := windowRefToWindow(req.Window)
	// The rectangle every region in this sample will be a proportion of. Recorded BEFORE the
	// cycle runs, from the reference the runner validated, so it describes the same window the
	// providers are about to be pinned to.
	//
	// Deleting this must fail TestPointingUsesTheFrameTheSampleWasNormalisedAgainst.
	s.recordFrame(req)

	s.rt.mu.Lock()
	s.rt.pinnedWindow = &req.Window
	cycleStart := time.Now()
	cycle := s.rt.collector.Collect(ctx, s.request(req))
	collectFor := time.Since(cycleStart)

	fuseStart := time.Now()
	world, _, err := s.rt.engine.Fuse(cycle)
	fuseFor := time.Since(fuseStart)
	s.rt.pinnedWindow = nil
	s.rt.mu.Unlock()

	if err != nil {
		return observe.Sample{}, fmt.Errorf("fusing this cycle: %w", err)
	}

	// The world this session just fused, for the perception surfaces. Recorded here
	// because a session never goes through the foreground pipeline, so nothing else
	// publishes it — see Runtime.lastWatched.
	//
	// Deleting this must fail TestLightModeDescribesTheWatchedWindow.
	s.rt.diagMu.Lock()
	copied := world
	s.rt.lastWatched = &copied
	s.rt.diagMu.Unlock()

	convertStart := time.Now()
	sample := buildSample(world, cycle, pinned, req)
	convertFor := time.Since(convertStart)

	sample.Phases = observe.Phases{
		Detect:   collectFor,
		Fuse:     fuseFor,
		Snapshot: convertFor,
		Total:    time.Since(start),
	}
	if req.ReadLabels {
		sample.Phases.LabelsRun = 1
	}
	// The experiment travels with the sample. Attached here because this is where the
	// cycle and the sample are both in scope, and because a Sample is the one value that
	// already reaches the session accumulator, the terminal Result, the protocol and the
	// CLI — the previous attempt gave the report its own path and production never used it.
	sample.Shadow = shadowSampleFor(s.rt.shadowDetectorName(), s.rt.shadowUnavailable,
		cycle, pinned.Bounds)
	// Where the scoped-reading budget went, from the detector's own diagnostic.
	//
	// Attached here because this is the only place the provider and the sample are both in
	// scope, and it rides on the sample for the reason everything else does: a counter
	// delivered on its own channel is a counter nothing reads. Without it, "no terms" has
	// four indistinguishable explanations and the report offers no way to choose between them.
	if sh := sample.Shadow; sh != nil && s.rt.shadowVision != nil {
		sh.Labels = shadowLabelBudget(s.rt.shadowVision)
	}
	// What the interface said about itself this cycle, as closed-vocabulary concepts.
	//
	// THE text boundary. The entities in scope here carry labels that already passed the
	// privacy classifier; SemanticEvidenceFrom reads them, matches whole words against the
	// generic interface vocabulary, and returns terms. The label text does not travel with
	// the result, so a username in a search box matches nothing and has nowhere to go.
	//
	// Attached HERE rather than in the runner because the trace is written from this
	// function: semantic evidence that appeared only after the recorder ran would replay
	// differently from production, which is the parity defect the navigation milestone had
	// to fix retrospectively.
	//
	// TWO sources, merged rather than ranked. Accessibility names controls where an
	// application exposes a tree; the shadow detector's own boxes, read by scoped OCR, name
	// them where it does not — and that second half is attached in shadowSampleFor, above.
	// Neither is complete and neither is authoritative about meaning, so the union is the
	// honest reading: a concept was on the screen, and it was no less there for having been
	// found by the other source.
	//
	// Merging rather than overwriting is load-bearing. Assigning here would erase whatever the
	// detector read on every application that also has an accessibility tree, which is most of
	// them, and the loss would be invisible — the terms would simply be the ones accessibility
	// already supplied, which is exactly what the previous milestone measured and concluded
	// from.
	// WHAT THE PLACE APPEARS TO BE CALLED, read from the fused world under the licence.
	//
	// Here because this is the one point where the world, the licence and the sample are all
	// in scope — and because THIS is the carrier that reaches a durable place on an ordinary
	// Director. `ShadowSample` reads as the vision experiment's record and is not: the
	// accessibility path attaches its semantic evidence to the same structure through
	// `ensureShadow`, which is how `terms` reach the store with no detector configured. The
	// obvious-looking seam a layer up runs only when the experiment is on.
	//
	// Deleting this must fail TestADemonstrationOffersThePlaceItsName.
	named := s.placeName(world)
	if sem := observe.SemanticEvidenceFrom(sample.Entities); !sem.Empty() || named != "" ||
		(sample.Shadow != nil && !sample.Shadow.Semantic.Empty()) {

		sem.PlaceName = named
		sample.Shadow = s.ensureShadow(sample.Shadow)
		sample.Shadow.Semantic = sem.Merge(sample.Shadow.Semantic)
	}
	// The player's navigation since the previous sample, as closed-vocabulary intents.
	//
	// Attached even when the shadow detector skipped this slot, which is why it is set on
	// the sample rather than inside shadowSampleFor: screen evidence and input evidence
	// fail independently, and at the real skip rate the keypress that opened a menu very
	// often lands in a slot the detector sat out.
	//
	// The producer's own counters ride along on EVERY sample, including the ones that
	// carried no navigation at all. An empty correlation has two explanations — the player
	// pressed nothing, or nothing was listening — and a report that cannot separate them is
	// the shape of failure this repository has already made twice.
	if s.rt.navSource != nil {
		st := s.rt.navSource.Stats()
		if s.nav != nil {
			if in := s.nav.Drain(); len(in) > 0 {
				sample.Shadow = s.ensureShadow(sample.Shadow)
				sample.Shadow.Inputs = in
			}
		}
		sample.Shadow = s.ensureShadow(sample.Shadow)
		sample.Shadow.InputStats = &st
		// THE admission context. What this inference's RAW detections say the screen looks
		// like, handed to the producer so an ambiguous key can be read as navigation while
		// a set of choices is on screen and refused during play.
		//
		// Pushed AFTER the drain, deliberately: the events just drained were classified
		// against what Marco had seen before them, which is the honest basis. This call
		// sets the basis for the next ones.
		//
		// Only on a VALID inference. A skipped slot or an unproven target is UNKNOWN, and
		// unknown is not false ([[ADR-006-unknown-is-not-false]]): flipping admission off
		// because the detector sat a slot out would say the menu had closed, which nothing
		// observed. The previous assessment stands, bounded by ScreenContextTTL.
		//
		// The timestamp is wall-clock, NOT the session clock, because it is compared
		// against the hook's own event times — which are wall-clock by construction. This
		// is a different question from the session-relative stamps carried in the evidence.
		s.pushNavContext(sample.Shadow, pinned.Bounds, time.Now())
		// And what that window OFFERED, in the same breath and under the same freshness
		// rules, so the next click can be resolved to the control it lands on.
		s.pushActionables(world, time.Now())
	} else if s.rt.navUnavailable != "" {
		sample.Shadow = s.ensureShadow(sample.Shadow)
		sample.Shadow.InputStats = &observe.InputStats{Unavailable: s.rt.navUnavailable}
	}
	shadowTracer().record(sample.Shadow, shadowGeneration(cycle))
	return sample, nil
}

// ensureShadow returns the sample's shadow record, creating an empty one if the detector
// produced none. Input evidence and screen evidence fail independently, so navigation must be
// able to ride a cycle the detector sat out.
func (s *liveSampler) ensureShadow(in *observe.ShadowSample) *observe.ShadowSample {
	if in != nil {
		return in
	}
	return &observe.ShadowSample{Detector: s.rt.shadowDetectorName()}
}

// request asks for the providers this sample should run, against the validated window.
//
// Vision is opt-in and expensive, so it runs on every sample; scoped label reading is far
// more expensive and runs only when the runner's budget allows, which is what ReadLabels
// carries. Accessibility and the window system are always in the cycle and cost nothing
// worth managing.
//
// # Window, and why this line matters more than it looks
//
// There are TWO pinning mechanisms, and for a long time this used only one. Vision and OCR
// read `Runtime.activeWindow`, which returns the `pinnedWindow` field Sample sets. But
// accessibility reads `Request.Window` — and this function used to leave it nil, which the
// provider and the bridge both document as meaning "the foreground window".
//
// So while a session watched VS Code from a service, the accessibility walk observed
// whatever was actually in front, usually the terminal running the diagnostics, and
// buildSample's window filter then correctly discarded all of it. The result read as
// "a targeted session is vision-only by construction" — it was one unset field. It cost
// three milestones and invalidated a baseline measurement (see
// docs/director-accessibility-targeting.md).
//
// The nil that remains is a REGION, which is a different narrowing and correctly absent:
// a tree walk has no notion of a rectangle.
func (s *liveSampler) request(req observesession.SampleRequest) observation.Request {
	out := observation.WithVision(nil)
	if s.labels && req.ReadLabels {
		out = observation.WithPixels(nil)
	}
	// The window the RUNNER validated this cycle. Every provider that can be scoped now
	// describes the same target, which is what makes evidence from one cycle
	// attributable to one window.
	window := req.Window.ID
	out.Window = &window

	// And which GENERATION of it. Window says where to look and is satisfied by any
	// window bearing that id; Target says which live window the answer is allowed to
	// describe, and is not satisfied by a replacement. Setting only the first is what
	// leaves the in-flight replacement race undetectable — see the guard in fusion.
	out.Target = expectedTarget(req.Window)
	return out
}

// windowRefToWindow converts a validated reference into the shape the rest of the Director
// speaks. The handle does not travel: an observation sample must carry no platform
// reference, and the generation is the durable thing anyway.
func windowRefToWindow(r windowref.Ref) directorapi.Window {
	return directorapi.Window{
		ID: r.ID, Application: r.Application, Title: r.Title, Bounds: r.Bounds,
	}
}

// newObservationTarget and newObservationSampler build the adapters.
func (r *Runtime) newObservationTarget() observesession.Target {
	return &liveTarget{tracker: r.winTracker, platform: r.winPlatform, directory: r.winDirectory}
}

func (r *Runtime) newObservationSampler(clock observesession.Clock) observesession.Sampler {
	s := &liveSampler{rt: r, labels: r.ocrUnavailable == ""}
	// The navigation subscription is stamped from THE SESSION'S OWN CLOCK, not from
	// time.Now(), so input times and observation times share one basis.
	//
	// The clock is threaded through for this one line, and it is worth the parameter: with
	// two independent bases a keypress is attributed to whichever transition happened to be
	// nearest in a drifting frame of reference, and the drift is invisible in production
	// (where both are the wall clock) and only appears under an injected clock — which is
	// to say, it would be a correctness bug that only the tests could have, and they would
	// have been written to accommodate it.
	if r.navSource != nil {
		s.nav = r.navSource.Open(clock.Now())
	}
	return s
}

// detachNavigation ends this sampler's navigation subscription.
func (s *liveSampler) detachNavigation() {
	if s.rt != nil && s.rt.navSource != nil && s.nav != nil {
		s.rt.navSource.CloseSession(s.nav)
	}
	s.nav = nil
}

// realClock is time, for production.
type realClock struct{}

func (realClock) Now() time.Time                         { return time.Now() }
func (realClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

// sessionClock is THE clock an observation session runs on — the runner's scheduling and the
// navigation producer's timestamps both read it.
//
// One value rather than two constructions, so "the same basis" is a fact about the code rather
// than a coincidence of both happening to call time.Now().
var sessionClock observesession.Clock = realClock{}

// pushNavContext tells the navigation producer what this cycle saw, so the NEXT events can be
// judged against it.
//
// Two things, and they are one call because they are one decision: an ambiguous key is navigation
// only while a set of choices is on screen, and a pointer press is this application's business
// only if it landed inside the window perception actually looked at. Both are properties of what
// was just observed, and both go stale the same way.
//
// # Why it is a method rather than four lines inside Sample
//
// So it can be driven by a test. This repository has three recorded cases of complete, correct
// production code that nothing ever called, and the wiring for both of these pushes survived a
// deliberate deletion until it was extracted here. A seam a test can reach is the difference
// between a mechanism that works and one that merely exists.
//
// Only on a VALID inference. A skipped slot or an unproven target is UNKNOWN, and unknown is not
// false ([[ADR-006-unknown-is-not-false]]): flipping admission off because the detector sat a
// slot out would say the menu had closed, which nothing observed. The previous assessment stands,
// bounded by ScreenContextTTL.
//
// The timestamp is wall-clock, NOT the session clock, because it is compared against the hook's
// own event times — which are wall-clock by construction.
func (s *liveSampler) pushNavContext(sh *observe.ShadowSample, bounds directorapi.Rect,
	now time.Time) {

	if s.rt == nil || s.rt.navSource == nil {
		return
	}
	// ── THE WINDOW FRAME, on every cycle that resolved a window ──
	//
	// NOT gated on the structural detector, and that gate was a real defect: it is the
	// same shape as the one recorded on 2026-08-10, where screen segmentation had exactly
	// one evidence source and it was opt-in.
	//
	// Vision is off by default, so `shadowSampleFor` returns nil, `ensureShadow` supplies a
	// record with `Ran: false`, and the gate below returned on EVERY cycle. The producer
	// therefore never received a frame, `windowBounds.fresh` was false forever, and every
	// pointer press on a default Director was refused as `unplaceable_pointer` — counted,
	// and gone. A live Learn against Windows Settings reported exactly that:
	// `received=2 classified=0 unplaceable_pointer=2`, for the two clicks the person made.
	//
	// A window's rectangle is a fact about the WINDOW. It comes from the reference the
	// runner validated before this sample, not from any provider and certainly not from an
	// experiment — so it is pushed whenever there is one, and a zero-area rectangle (a
	// minimised or unvalidated target) is skipped rather than pushed as a frame, leaving
	// the previous one to expire on its own TTL.
	//
	// Deleting this must fail TestADefaultDirectorGivesThePointerAFrame.
	if bounds.Width > 0 && bounds.Height > 0 {
		s.rt.navSource.SetWindowBounds(bounds.X, bounds.Y, bounds.Width, bounds.Height, now)
	}
	// ── WHOSE SURFACE IS IN FRONT, on every cycle ──
	//
	// Ungated for the same reason the frame above is: this is a fact about the DESKTOP, not
	// about any experiment, and gating it on a detector that is off by default is precisely
	// the defect recorded in the comment above. Pushed rather than asked at capture time
	// because answering it means a syscall, and a low-level hook callback that makes one is
	// a hook Windows drops.
	//
	// Deleting this must fail TestMarcoOwnedInputNeverBecomesDemonstrationEvidence.
	s.rt.navSource.SetSurfaceOwner(s.rt.surfaceOwnsForeground(), now)
	// ── THE ADMISSION CONTEXT, which genuinely is the detector's ──
	//
	// `MenuLike` reads the detector's own RAW detections, so this half keeps the gate: with
	// no detector there are no regions, and pushing "not menu-like" from an absent source
	// would be an assertion nothing observed. Unset simply leaves context-conditional keys
	// refused, which is the conservative reading and what ADR-013 wants.
	if sh == nil || !sh.Ran || !sh.TargetProven || sh.Unavailable != "" {
		return
	}
	s.rt.navSource.SetScreenContext(observe.MenuLike(sh.Regions), now)
}

// MaxActionables bounds what one inference offers the navigation producer to resolve
// against. A realistic accessibility screen holds tens of activatable controls; the bound
// exists so a pathological tree cannot grow the worker's index without limit.
const MaxActionables = 256

// pushActionables tells the navigation producer what the watched window OFFERED this cycle,
// so a press or a confirm that arrives before the next one can be resolved to the control it
// landed on — at event time, from evidence that was already admitted.
//
// # The two label gates, applied here and nowhere later
//
// The role allowlist and the demonstration licence both live in
// observe.AdmittedTargetLabel, beside the classifier whose shape filter it shares. What the
// producer receives is already the admitted form; what it emits is re-checked at the
// consumer (admissibleInputs). Three layers, one policy site.
//
// Same freshness contract as the two pushes above: only on a valid inference, wall-clock
// stamped, stale under the producer's own TTL.
//
// Deleting this call must fail TestAValidInferenceOffersActionablesToTheProducer.
func (s *liveSampler) pushActionables(world directorapi.WorldState, now time.Time) {
	if s.rt == nil || s.rt.navSource == nil {
		return
	}
	// Not gated on the detector either, and for the same reason as the frame above: the
	// controls come from the FUSED world, which accessibility fills on every ordinary
	// cycle. Gating this on the vision experiment would mean click-target resolution never
	// ran on a default Director — the enrichment would exist and never once fire.
	// DETERMINISTIC ORDER, BEFORE THE BOUND.
	//
	// # The defect this closes
	//
	// A fused world is a MAP, and this loop truncated at MaxActionables. So on a screen
	// offering more clickable controls than the bound, WHICH of them reached the navigation
	// producer depended on Go's map iteration — and Go randomises that per range. Two
	// readings of one unchanged screen offered different sets, and the set is what a human
	// click is attributed against (36B): the same press could resolve to a Target on one
	// reading and to nothing on the next, with no way to tell that from a perception
	// failure.
	//
	// Sorted by geometry — top to bottom, then left to right, then role and label — which is
	// reading order and therefore the order a person would name them in. Nothing here is
	// durable identity; it is a deterministic way to decide what to keep when there is too
	// much to keep.
	//
	// Deleting the sort must fail TestWhichControlsAreOfferedDoesNotDependOnMapOrder.
	found := make([]*directorapi.Element, 0, 64)
	for _, el := range world.Elements {
		if el == nil || !el.Visible || el.Offscreen || el.Bounds.Empty() {
			continue
		}
		// A control someone can aim at, or the one holding the keyboard's attention — the
		// two ways an input event acquires a target.
		if !el.Role.Clickable() && !el.Focused {
			continue
		}
		found = append(found, el)
	}
	sort.Slice(found, func(i, j int) bool { return earlierOnScreen(found[i], found[j]) })

	items := make([]navsource.Actionable, 0, len(found))
	for _, el := range found {
		items = append(items, navsource.Actionable{
			X: el.Bounds.X, Y: el.Bounds.Y, W: el.Bounds.Width, H: el.Bounds.Height,
			Role: string(el.Role),
			Label: observe.AdmittedTargetLabel(el.Role, s.nameActivatedTargets, el.Label,
				el.Confidence),
			Focused: el.Focused,
		})
		if len(items) >= MaxActionables {
			break
		}
	}
	s.rt.navSource.SetActionables(items, now)
}

// placeNameEvidence reads the fused world for what the Place appears to be called.
//
// # Why here
//
// This is the one point where the FUSED world and the demonstration licence are both in scope.
// The world is where selection and parentage survive — a raw provider observation carries neither
// in a form this can walk — and the licence is what decides whether a word read off somebody's
// screen may be written down at all.
//
// It reads; it decides nothing. observe.AdmittedPlaceName owns the rule, beside the target-label
// gate whose shape filter it shares, so there is exactly one policy site for "may this text be
// kept". See [[ADR-076-a-place-may-say-what-it-appears-to-be-called]].
//
// Deleting the value-chooser walk must fail TestASelectedValueIsNotOfferedAsAPlaceName.
func placeNameEvidence(world directorapi.WorldState) []observe.PlaceNameEvidence {
	// The world is already keyed by id, and it is a MAP — so this walk has no order. The
	// rule it feeds is order-independent by construction: agreement gives one name and any
	// disagreement gives none, so no answer here can depend on how the map happened to walk.
	byID := world.Elements
	// A selected item inside a control for PICKING A VALUE is a value, not a destination.
	// Settings Home reports `Home` in the navigation pane and `Dark` inside the Color-mode
	// combo box; one says where you are and the other says what a setting is set to.
	inValueChooser := func(el *directorapi.Element) bool {
		for cur, depth := el, 0; depth < 8; depth++ {
			if cur.ParentID == nil {
				return false
			}
			parent, ok := byID[*cur.ParentID]
			if !ok {
				return false
			}
			// Only the combo box today: it is the one value chooser the measured trees
			// actually produced a selected child inside. Adding roles here without a
			// tree that shows one would be guessing at the shape of the problem.
			if parent.Role == directorapi.RoleComboBox {
				return true
			}
			cur = parent
		}
		return false
	}
	out := make([]observe.PlaceNameEvidence, 0, 4)
	for _, el := range world.Elements {
		if el == nil || !el.Selected || el.Offscreen || !el.Visible {
			continue
		}
		out = append(out, observe.PlaceNameEvidence{
			Role: el.Role, Label: el.Label, Confidence: el.Confidence,
			Selected: true, InsideValueChooser: inValueChooser(el),
			Trail: trailContaining(byID, el.Label),
		})
	}
	return out
}

// placeName is what this sampler believes the Place on screen is called, or nothing.
//
// A method rather than an expression so a test can enter through the same door production does.
// It used to take the sampler's `demonstration` field as a licence, and that was the whole reason
// it was not a free function; Roadmap 35A removed the licence from the INFERENCE and left it on
// the PERSISTENCE (see observe.AdmittedPlaceName), so the method now survives for the wiring
// reason alone — a free function proves the rule and not that anything calls it.
//
// See placeNameEvidence and observe.AdmittedPlaceName.
//
// Deleting the AdmittedPlaceName call must fail TestTheSamplerNamesThePlaceWhoeverIsWatching.
func (s *liveSampler) placeName(world directorapi.WorldState) string {
	return observe.AdmittedPlaceName(placeNameEvidence(world))
}

// trailContaining is the navigation trail a word appears in, or nothing.
//
// # Why the trail identifies itself
//
// A selected navigation item names the SECTION. On a sub-page that is not where you are, and the
// tree says so: measured on the Settings Mouse page, two sibling buttons under one parent read
// `Bluetooth & devices` and `Mouse`, while the rail reports `Bluetooth & devices` selected. On Home
// the same parent holds one button, `Home`.
//
// So the trail is the set of sibling button labels that CONTAINS the selected word. That is a
// relationship between two independent pieces of evidence — nothing here names an application, and
// nothing reads a rectangle. The fused world is a map with no order, so order is never used
// either; the rule that consumes this compares membership only.
//
// Buttons, because that is what a trail entry is: something you can press to go back up it. The
// rail's own items are list items elsewhere in the tree and cannot be confused with these.
//
// Deleting this must fail TestTheTrailIsTheSiblingsContainingTheSelectedWord.
func trailContaining(byID map[directorapi.ElementID]*directorapi.Element, word string) []string {
	word = strings.TrimSpace(word)
	if word == "" {
		return nil
	}
	// Sibling buttons, grouped by the parent they share.
	siblings := map[directorapi.ElementID][]string{}
	for _, el := range byID {
		if el == nil || el.Role != directorapi.RoleButton || el.ParentID == nil {
			continue
		}
		if el.Offscreen || !el.Visible {
			continue
		}
		if label := strings.TrimSpace(el.Label); label != "" {
			siblings[*el.ParentID] = append(siblings[*el.ParentID], label)
		}
	}
	for _, labels := range siblings {
		for _, l := range labels {
			if l == word {
				return labels
			}
		}
	}
	return nil
}

// earlierOnScreen is reading order over two elements, total and stable.
//
// Top to bottom, then left to right, then role and label. Geometry first because that is how a
// person would enumerate what is on a screen, and the remaining keys because two controls really
// can share a rectangle — a total order has to break every tie or the sort is not one.
//
// TRANSIENT, and it is worth saying: this is a way to decide what to KEEP when a bound is spent,
// and never a claim about identity. Nothing durable is derived from it, and moving a control
// changes where it sorts and nothing else. See [[ADR-100-marco-sees-through-evidence]].
func earlierOnScreen(a, b *directorapi.Element) bool {
	if a.Bounds.Y != b.Bounds.Y {
		return a.Bounds.Y < b.Bounds.Y
	}
	if a.Bounds.X != b.Bounds.X {
		return a.Bounds.X < b.Bounds.X
	}
	if a.Role != b.Role {
		return a.Role < b.Role
	}
	if a.Label != b.Label {
		return a.Label < b.Label
	}
	return a.ID < b.ID
}
