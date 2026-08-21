package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/referent"
)

// Turning what Marco means into where it is on this screen.
//
// # The whole chain, in one place, so it can be read
//
//	the live application
//	  → the accessibility provider              (perception/providers)
//	  → an admitted Director observation        (fusion, provenance-guarded)
//	  → a safe sample, geometry relative to the window frame   (buildSample)
//	  → recurring structures and screens        (ShadowTracker)
//	  → a subject Marco actually said something about          (WhatToPointAt)
//	  → the members' own current regions        (ReferentForSubject)
//	  → desktop rectangles                      (pkg/referent)
//	  → a presentation surface
//
// Every step above already existed and was already tested. This file is the join, and it is
// deliberately thin: it decides nothing about what is meant, and it does no arithmetic of its own.
//
// # The two frames
//
// `pkg/referent` needs the window rectangle the regions were normalised AGAINST and the rectangle
// the window is at NOW, and refuses when they differ. Both are read here, from two different
// places on purpose: the first from the sampler that took the measurement, the second from a fresh
// platform resolution. Reading one and using it twice would satisfy the freshness check by
// construction and prove nothing.

// pointView is what Marco means, where it is, and how it knows.
//
// The provenance fields are not decoration. A highlight is a claim, and the difference between
// "I read this from the application's own accessibility tree" and "I guessed from pixels" is
// exactly the difference a person needs in order to decide how much to trust it.
type pointView struct {
	// What Marco means.
	Application    string `json:"application,omitempty"`
	Say            string `json:"say"`
	Basis          string `json:"basis,omitempty"`
	Question       string `json:"question,omitempty"`
	Interpretation string `json:"interpretation,omitempty"`
	Role           string `json:"role,omitempty"`
	About          string `json:"about,omitempty"`
	Subject        string `json:"subject,omitempty"`
	// Place is what Marco takes the current screen to be, in words. Empty when it has not
	// settled on one — which is a real and common answer, not a missing field.
	Place string `json:"place,omitempty"`
	// Targets is what Marco knows it can ACT ON here: the durable semantic targets grounded
	// in the current place, in the words on them.
	//
	// The Theater's side of "what are you seeing". Perception answers what is on screen;
	// this answers what Marco has learned is here and could be asked for by name, which is a
	// different and much smaller list. Empty is honest and ordinary — most places have no
	// remembered targets, and inventing a plausible one would be the worst possible answer to
	// a question about what Marco knows. See
	// [[ADR-068-the-theater-is-the-durable-semantic-world]].
	Targets []string `json:"targets,omitempty"`
	// LastAction is the last thing Marco DID, in the user's own terms.
	//
	// Read from the action graph, which is where actions are already recorded, rather than
	// from a second history kept for this surface. A person asking what Marco is seeing has
	// almost always just watched it do something, and "what did you just do" is the other
	// half of that question.
	LastAction string `json:"last_action,omitempty"`
	// LastActionWhen is how long ago that was, in words. Empty when nothing has been done.
	LastActionWhen string `json:"last_action_when,omitempty"`

	// Why there is nothing to show, when there is nothing.
	Refusal     string `json:"refusal,omitempty"`
	Unavailable string `json:"unavailable,omitempty"`
	Unmappable  string `json:"unmappable,omitempty"`

	// Where it is. Regions are window-relative as everything upstream is; Boxes are the same
	// rectangles on this desktop, and there is one of each per member.
	Regions []referent.Norm `json:"regions,omitempty"`
	Boxes   []referent.Box  `json:"boxes,omitempty"`
	At      referent.Frame  `json:"frame_measured_against,omitzero"`
	Now     referent.Frame  `json:"frame_now,omitzero"`

	// How it knows.
	Sources     []sourceStatus `json:"perception_sources,omitempty"`
	Source      string         `json:"perception_source,omitempty"`
	SourceWhy   string         `json:"perception_source_why,omitempty"`
	Pixels      string         `json:"pixel_vision,omitempty"`
	AtInference int            `json:"at_inference,omitempty"`
	Sequence    int            `json:"measured_at_sample,omitempty"`
	Session     string         `json:"session,omitempty"`
}

// CanPoint reports whether there is something to draw.
func (v pointView) CanPoint() bool { return len(v.Boxes) > 0 }

// pointingEvidence is one session's account, as pointing needs it.
type pointingEvidence struct {
	session    observe.SessionID
	app        string
	proposals  []observe.Proposal
	hypotheses []observe.Hypothesis
	shadow     observe.ShadowTotals
	selector   windowref.Selector
	ok         bool
}

// evidenceForPointing reads the current session's account — active if one is running, otherwise
// the most recent finished one.
//
// The same ordering rule as `analysis`, and for the same reason: "what is Marco talking about"
// means the session that is happening, and a finished session's conclusions are frozen rather
// than erased.
func (g *observationRegistry) evidenceForPointing() pointingEvidence {
	g.mu.RLock()
	runner := g.active
	finished := make([]observesession.Result, len(g.finished))
	copy(finished, g.finished)
	g.mu.RUnlock()

	if runner != nil {
		session, stats := runner.Snapshot()
		ledger := runner.Proposals()
		return pointingEvidence{
			session: session.ID, app: session.Application,
			proposals: ledger.Proposals,
			hypotheses: ledger.Annotate(
				observe.Hypotheses(stats.Shadow, observe.DefaultHypothesisThresholds())),
			shadow: stats.Shadow, selector: session.Selector, ok: true,
		}
	}
	for i := len(finished) - 1; i >= 0; i-- {
		r := finished[i]
		return pointingEvidence{
			session: r.Session.ID, app: r.Session.Application,
			proposals: r.Proposals.Proposals, hypotheses: r.Hypotheses,
			shadow: r.Stats.Shadow, selector: r.Session.Selector, ok: true,
		}
	}
	return pointingEvidence{}
}

// sampledFrameNow is the frame the last sample measured against, from the sampler that took it.
func (g *observationRegistry) lastSampledFrame() (sampledFrame, bool) {
	g.mu.RLock()
	s := g.lastSampler
	g.mu.RUnlock()
	f, ok := s.(interface{ LastFrame() (sampledFrame, bool) })
	if !ok {
		return sampledFrame{}, false
	}
	return f.LastFrame()
}

// PointAt resolves what Marco is currently referring to, and where that is on this desktop.
//
// Reads only. Nothing is written, nothing is remembered, and no proposal, hypothesis, judgement or
// piece of durable memory is touched — which is what makes it safe to call while a person is being
// asked a question about the very subject it locates.
func (r *Runtime) PointAt(ctx context.Context, q service.ObservePoint) (pointView, error) {
	if r.observations == nil {
		return pointView{}, fmt.Errorf("this Director has no observation registry")
	}
	ev := r.observations.evidenceForPointing()
	if !ev.ok {
		// NOTHING OBSERVED is not nothing to say. What Marco last DID is history, and
		// history does not depend on whether a session has run — a person who just watched
		// it do something and asks what it is seeing is owed that answer even when the
		// answer to everything else is "I haven't watched anything yet".
		//
		// Found live: the whole line was missing on a freshly started Director with 183
		// actions behind it, because this branch returned before reaching it.
		//
		// Deleting this must fail TestSightSaysWhatItLastDidEvenWithNothingObserved.
		what, when := lastActionDone()
		return pointView{
			Refusal:    string(observe.NothingObserved),
			Say:        observe.NothingObserved.Say(),
			LastAction: what, LastActionWhen: when,
		}, nil
	}

	frame, _ := r.observations.lastSampledFrame()
	live := r.liveGeometry(ev.app, ev.shadow)

	// Three questions, three answers, and they are NOT interchangeable. A named durable subject
	// resolves through recognition; a named question resolves that question's own subject and
	// refuses rather than substituting; anything else is whatever Marco is currently referring
	// to. Collapsing the middle one into the last is how a button under a question comes to
	// highlight a different subject with nothing saying so.
	//
	// Deleting the question branch must fail
	// TestAskingAboutOneQuestionDoesNotGetWhateverMarcoIsTalkingAbout.
	p := observe.WhatToPointAt(ev.proposals, ev.hypotheses, live)
	switch {
	case q.Subject != "":
		p = observe.WhatRemembersThis(q.Subject, ev.proposals, live)
	case q.Question != "":
		p = observe.WhatThisQuestionMeans(observe.ProposalID(q.Question), ev.proposals, live)
	}
	view := pointView{
		Application: ev.app, Session: string(ev.session),
		Say: p.Say(), Basis: string(p.Basis),
		Question: p.Question, Interpretation: p.Interpretation,
		Role: string(p.Referent.Role), About: p.Referent.About,
		Refusal: string(p.Refusal), Unavailable: string(p.Referent.Unavailable),
		AtInference: p.Referent.AtInference, Sequence: frame.Sequence,
	}
	view.Subject = q.Subject
	view.Source, view.SourceWhy, view.Pixels = provenanceOf(ev.shadow)
	view.Sources = r.perceptionSources()
	view.Place = r.observations.placeNow(ev.shadow, ev.app)
	// WHAT MARCO CAN ACT ON HERE, and WHAT IT LAST DID. Both read-only, both from records
	// that already exist — the Theater's durable targets and the action graph.
	//
	// Deleting either must fail TestSightSaysWhatItCanActOnAndWhatItLastDid.
	view.Targets = r.observations.targetsHere(ev.shadow, ev.app)
	view.LastAction, view.LastActionWhen = lastActionDone()
	if q.Role != "" && string(p.Referent.Role) != q.Role {
		// Asked for one kind of reference and Marco is making another. Saying so beats
		// showing the wrong one.
		view.Refusal = string(observe.NothingMeant)
		view.Say = observe.NothingMeant.Say()
		return view, nil
	}
	if !p.Referent.CanPoint() {
		return view, nil
	}

	for _, reg := range p.Referent.Regions {
		view.Regions = append(view.Regions,
			referent.Norm{X: reg.X, Y: reg.Y, Width: reg.Width, Height: reg.Height})
	}
	view.At = referent.Frame{X: frame.Bounds.X, Y: frame.Bounds.Y,
		Width: frame.Bounds.Width, Height: frame.Bounds.Height}

	// WHERE IT IS NOW, asked of the platform rather than assumed from the measurement. The
	// window may have moved between the sample and this call, and the whole design of
	// referent.Map is that the answer to that is a refusal.
	now, why := r.frameNow(ctx, ev.selector)
	if why != "" {
		view.Unmappable = string(referent.NoWindow)
		view.Say = referent.NoWindow.Say()
		return view, nil
	}
	view.Now = now

	m := referent.Map(view.Regions, view.At, view.Now)
	if !m.Drawable() {
		view.Unmappable = string(m.Reason)
		view.Say = m.Reason.Say()
		return view, nil
	}
	view.Boxes = m.Boxes
	return view, nil
}

// liveGeometry is what is on screen right now, as the referent resolver needs it.
//
// ONE construction, used by every caller that resolves a referent. Tracks and states are the
// session's own recurring-structure account; `Reliable` is false when there is no sampled frame to
// convert against, which hides every referent rather than drawing it approximately.
//
// A second copy of this would be a second place deciding whether geometry can be trusted, and the
// two would disagree the first time a sampler stopped reporting a frame.
func (r *Runtime) liveGeometry(application string, t observe.ShadowTotals) observe.LiveGeometry {
	frame, haveFrame := r.observations.lastSampledFrame()
	return observe.LiveGeometry{
		Application: application,
		Window:      string(frame.Window),
		AtInference: t.Inferences,
		Tracks:      t.Tracks,
		States:      t.States,
		Reliable:    haveFrame,
		// The frame's own sequence, carried whether or not it was judged usable — LastFrame
		// returns the value either way. It decides nothing; it exists so a diagnosis can
		// separate "nothing has sampled" from "a sample ran and its rectangle was unusable",
		// which are different defects and were indistinguishable from one boolean.
		FrameSequence: frame.Sequence,
	}
}

// currentContext is the window the person is working in, as a pinned selector.
//
// Two steps, and both matter. Resolving the foreground answers "what am I looking at"; adopting it
// as an ephemeral id turns that answer into a reference that names ONE window of ONE process and
// refuses to follow a replacement. Skipping the second step — handing the session a live
// "foreground" selector — would make every validation re-ask the question and let the session drift
// onto whatever came forward, which is precisely the failure windowref exists to prevent.
//
// Deleting the Adopt call must fail TestTheChosenContextIsPinnedAndDoesNotFollowFocus.
// foregroundCandidate is the window in front, BEFORE it is given an ephemeral id.
//
// # Why the candidate and not the selector
//
// Because Directory.Adopt mints a NEW id on every call — window_1, window_2, window_3 — so two
// selectors for the same unmoved window never compare equal. Anything that has to ask "is this
// still the same window?" across time must ask it of the candidate, which carries the handle.
//
// Live, the settle rule in AwaitSubject compared selectors and therefore never settled: it polled
// every 400ms forever, minting an ephemeral id each time, and the panel sat on
// "waiting_for_demonstration" with Target locked: NO while somebody stood in Settings waiting for
// it to notice.
//
// Deleting this and comparing selectors again must fail TestTheSettleRuleComparesTheWindow.
func (r *Runtime) foregroundCandidate(ctx context.Context) (windowref.Candidate, error) {
	// MARCO'S OWN SURFACE IS NEVER THE SUBJECT.
	//
	// The same check subjectContext makes, and it must be here too: the platform chokepoint
	// excludes Marco's registered presentation surfaces, but the control centre is a BROWSER
	// window that Marco merely asked for — indistinguishable from any other browser except
	// through the ownership the surface registry records. Pressing Start necessarily leaves
	// it in front, so without this the panel becomes the window the learn session is about.
	//
	// Deleting this must fail TestTheSettleRuleNeverLatchesMarcosOwnSurface.
	if r.surfaceOwnsForeground() {
		return windowref.Candidate{}, fmt.Errorf(
			"the window in front is Marco itself, so there is nothing to watch yet")
	}
	if r.winPlatform == nil {
		return windowref.Candidate{}, fmt.Errorf(
			"this Director cannot see the desktop, so it cannot tell what you're working in")
	}
	c, res, why := windowref.Foreground(ctx, r.winPlatform)
	if !res.OK() {
		return windowref.Candidate{}, fmt.Errorf("%s", why)
	}
	return c, nil
}

// adopt gives a settled candidate the ephemeral id a session refers to it by.
//
// Called ONCE per session, after the window has held still. The old path adopted on every poll,
// which grew the directory by one entry every 400ms for as long as somebody took to walk to their
// application.
func (r *Runtime) adopt(c windowref.Candidate) (windowref.Selector, error) {
	if r.winDirectory == nil {
		return windowref.Selector{}, fmt.Errorf("this Director cannot refer to windows")
	}
	id := r.winDirectory.Adopt(c)
	if id == "" {
		return windowref.Selector{}, fmt.Errorf("the window in front could not be referred to")
	}
	return windowref.Selector{EphemeralID: id}, nil
}

func (r *Runtime) currentContext(ctx context.Context) (windowref.Selector, error) {
	if r.winPlatform == nil || r.winDirectory == nil {
		return windowref.Selector{}, fmt.Errorf(
			"this Director cannot see the desktop, so it cannot tell what you're working in")
	}
	c, res, why := windowref.Foreground(ctx, r.winPlatform)
	if !res.OK() {
		return windowref.Selector{}, fmt.Errorf("%s", why)
	}
	id := r.winDirectory.Adopt(c)
	if id == "" {
		return windowref.Selector{}, fmt.Errorf(
			"the window in front could not be referred to")
	}
	return windowref.Selector{EphemeralID: id}, nil
}

// frameNow reads the target window's CURRENT rectangle.
//
// Through the ordinary selector resolution, so it is the same identity check every other command
// makes: an ephemeral id that now names a different process resolves stale rather than plausible,
// and a window that has closed resolves not-found. A direct handle read would skip all of that and
// happily report the rectangle of whatever inherited the handle.
func (r *Runtime) frameNow(ctx context.Context, s windowref.Selector) (referent.Frame, string) {
	if r.winPlatform == nil {
		return referent.Frame{}, "this Director cannot see the desktop"
	}
	c, res, why := windowref.Resolve(ctx, r.winPlatform, r.winDirectory, s)
	if !res.OK() {
		if why == "" {
			why = string(res)
		}
		return referent.Frame{}, why
	}
	return referent.Frame{X: c.Bounds.X, Y: c.Bounds.Y,
		Width: c.Bounds.Width, Height: c.Bounds.Height}, ""
}

// sourceStatus is one perception source, and whether it is actually running.
//
// A closed shape rather than a sentence, so a surface cannot render "Vision: ON" over a Director
// that has no detector wired. Reason is filled in only when the answer is off.
type sourceStatus struct {
	Name   string `json:"name"`
	On     bool   `json:"on"`
	Reason string `json:"reason,omitempty"`
}

// perceptionSources is the truth about what this Director can currently perceive with.
//
// # Why it is asked of the runtime and not configured
//
// Because the whole point is that it cannot be wrong. Each answer is read from the field the
// composition root set when it tried to wire that source — the same field that decides whether the
// source runs. There is no way for this to say ON while the provider is absent, which is the one
// failure a status display must not have: a person told Marco is reading pixels, when it is reading
// an accessibility tree, will trust a highlight for reasons that are not true.
//
// Accessibility has no "unavailable" field because it is not optional: it is wired by the
// composition root unconditionally, and a Director without a collector cannot answer at all.
//
// Every source needs a POSITIVE signal that it exists, not merely the absence of a complaint. An
// empty `ocrUnavailable` means "nothing went wrong", and on a Director where OCR was never wired at
// all nothing did go wrong — so reading emptiness as availability reported OCR as ON in a runtime
// that had no text engine. Caught by the test below rather than by a person being told Marco could
// read their screen.
//
// Deleting any reason must fail TestADisabledPerceptionSourceIsNeverShownAsOn.
func (r *Runtime) perceptionSources() []sourceStatus {
	out := []sourceStatus{{Name: "accessibility", On: r.collector != nil}}
	if r.collector == nil {
		out[0].Reason = "no perception collector is wired"
	}
	out = append(out,
		sourceStatus{Name: "vision", On: r.visionUnavailable == "" && r.shadowVision != nil,
			Reason: firstReason(r.visionUnavailable, r.shadowUnavailable,
				"no detector is running")},
		sourceStatus{Name: "ocr", On: r.ocr != nil && r.ocrUnavailable == "",
			Reason: firstReason(r.ocrUnavailable, "no text engine is wired")})
	// A source that IS on carries no excuse for being off.
	for i := range out {
		if out[i].On {
			out[i].Reason = ""
		}
	}
	return out
}

// firstReason is the first explanation that was actually given.
func firstReason(reasons ...string) string {
	for _, r := range reasons {
		if r != "" {
			return r
		}
	}
	return ""
}

// provenanceOf says where the geometry came from, in the plainest honest terms.
//
// Three separate facts, kept separate: which evidence the composition was segmented from, why when
// that is nothing, and what the pixel side of perception was doing. Collapsing them is how a
// report comes to imply Marco looked at the screen when it read a tree.
func provenanceOf(t observe.ShadowTotals) (source, why, pixels string) {
	switch t.Structure {
	case observe.StructureFused:
		source = "accessibility"
	case observe.StructureDetector:
		source = "pixel_detector"
	default:
		source, why = "none", t.StructureWhy
	}
	switch {
	case t.Unavailable != "":
		pixels = "off: " + t.Unavailable
	case t.Inferences > 0:
		pixels = "on"
	default:
		pixels = "off"
	}
	return source, why, pixels
}

// placeNow is what Marco takes the current screen to be, in words a person can read.
//
// # Why it is words and not a subject id
//
// Because "what does Marco think this screen is" is a question about understanding, and a subject
// reference answers a different question — which record it matched — that only somebody with the
// memory file open can act on. If Marco has been given a name for this screen, that name is the
// answer. If it recognised the screen without a name, saying so is the answer. If it did not
// recognise it, that is also an answer, and a common one.
//
// Empty means Marco has not settled on anything, and the surface says so rather than showing a
// blank field.
func (g *observationRegistry) placeNow(t observe.ShadowTotals, application string) string {
	g.mu.RLock()
	m := g.memory
	g.mu.RUnlock()
	if m == nil {
		return ""
	}
	p := observe.PlaceNow(t, application, m, observe.DefaultHypothesisThresholds())
	if !p.Placed {
		return ""
	}
	switch p.Verdict {
	case observe.MatchSame:
		if name := screenNameOf(m, p.Subject); name != "" {
			return name + " — a screen you've named"
		}
		return "a screen I've seen before"
	case observe.MatchCandidate:
		return "possibly a screen I've seen before"
	case observe.MatchDifferent:
		return "somewhere I haven't seen before"
	}
	return ""
}

// screenNameOf is the user's own word for a remembered screen, empty when they have not given one.
func screenNameOf(m observe.Memory, subject string) string {
	store, ok := m.(observe.KnowledgeStore)
	if !ok || subject == "" {
		return ""
	}
	s, ok := store.Subject(subject)
	if !ok {
		return ""
	}
	return string(s.Called)
}

// MaxTargetsShown bounds the "can act on" list.
//
// A reading, not an inventory. Twelve remembered targets on one screen is already more than
// anybody scans, and a surface that printed sixty would be a dump wearing a sentence's clothes.
const MaxTargetsShown = 12

// targetsHere is what Marco knows it can act on in the place it is standing in.
//
// The Theater's answer, not perception's: these are DURABLE targets grounded in this place, which
// is a claim about what Marco has learned rather than about what happens to be on screen. Empty
// when Marco has not settled on a place at all, because a target grounded nowhere is not a thing
// it can act on here — it is a thing it can act on somewhere.
func (g *observationRegistry) targetsHere(t observe.ShadowTotals, application string) []string {
	g.mu.RLock()
	m := g.memory
	g.mu.RUnlock()
	if m == nil || application == "" {
		return nil
	}
	p := observe.PlaceNow(t, application, m, observe.DefaultHypothesisThresholds())
	// Only a SETTLED place. A candidate match is not enough to say "I can act on these here":
	// it would attribute one screen's targets to another that merely resembles it.
	//
	// One clause, because the safety is STRUCTURAL rather than defended here. Place.Subject is
	// filled only when the verdict is MatchSame (Verdict.Established), and every durable
	// target is grounded in a real place id, so an unsettled place asks for targets grounded
	// in "" and there are none. Adding `p.Verdict != MatchSame` beside this would read as a
	// second lock and be an unfalsifiable restatement of the first — see
	// TestTargetsAreNotClaimedForAPlaceMarcoHasNotSettledOn, which asserts the structural
	// property instead of pretending the clause is killable.
	if p.Subject == "" {
		return nil
	}
	store, ok := m.(interface {
		TargetsIn(application, place string) []observe.RememberedSubject
	})
	if !ok {
		return nil
	}
	var out []string
	for _, s := range store.TargetsIn(application, p.Subject) {
		out = append(out, describeTarget(s))
		if len(out) == MaxTargetsShown {
			break
		}
	}
	return out
}

// describeTarget is one durable target as a person reads it.
//
// The label with its kind, because "Mouse" alone does not say whether Marco thinks that is a
// button or a heading, and that difference decides whether asking for it does anything.
func describeTarget(s observe.RememberedSubject) string {
	label := s.Structure.Label
	if label == "" {
		label = "something unnamed"
	}
	if k := s.Structure.Kind; k != "" {
		return label + " (" + k + ")"
	}
	return label
}

// lastActionDone is the last thing Marco did, in the user's own terms.
//
// Read from the action graph on disk. A miss of any kind — no graph, unreadable, empty, a newest
// node with nothing sayable in it — is silence rather than a placeholder: "Marco has not done
// anything" and "Marco cannot tell you what it did" both read better as an absent line than as a
// confident-looking blank.
func lastActionDone() (what, when string) {
	g, err := openGraph()
	if err != nil {
		return "", ""
	}
	recent, err := g.Recent(1)
	if err != nil || len(recent) == 0 {
		return "", ""
	}
	n := recent[0]
	if strings.TrimSpace(n.Goal) == "" {
		return "", ""
	}
	return n.Goal, agoInWords(time.Since(n.Timestamp))
}

// agoInWords is a duration a person reads rather than measures.
func agoInWords(d time.Duration) string {
	switch {
	case d < 0:
		return ""
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}

// placeNowSubject is the durable place Marco is standing in RIGHT NOW, empty when unsettled.
//
// The subject id behind placeNow's sentence. Separate because a surface that has to compare where
// somebody IS against where a route BEGINS needs the identity, not the prose — and comparing the
// prose would make two screens described alike look like one place.
//
// Live rather than pinned. A learn session carries `Start`, which is the place the demonstration
// began on and is deliberately frozen; using it to answer "where are you now" made a panel report
// that the person was standing on the start at the very moment they were somewhere else.
//
// Deleting this must fail TestHereMeansWhereYouAreNowNotWhereTheDemonstrationBegan.
func (g *observationRegistry) placeNowSubject() string {
	p := g.placeHere()
	if !p.Placed {
		return ""
	}
	return p.Subject
}

// placeHere is the WHOLE answer, not just which screen it was.
//
// The subject alone cannot say why there is no subject, and the difference between a page Marco
// does not remember and a window it could not read is carried on the Place -- see observe.Reach.
// Callers that only want the id keep asking placeNowSubject; callers that have to explain a
// refusal ask this.
func (g *observationRegistry) placeHere() observe.Place {
	ev := g.evidenceForPointing()
	if !ev.ok || ev.app == "" {
		return observe.Place{}
	}
	g.mu.RLock()
	m := g.memory
	g.mu.RUnlock()
	if m == nil {
		return observe.Place{}
	}
	return observe.PlaceNow(ev.shadow, ev.app, m, observe.DefaultHypothesisThresholds())
}
