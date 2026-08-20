package observesession_test

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// The transition substrate, driven through the production session path at realistic scale.
//
// Nothing here calls the segmenter, the transition recorder or the relationship projector
// directly. Every assertion is about what a Result carries after a real Runner has folded real
// samples — because the failure this repository keeps finding is a mechanism that works and is
// never reached, and a test that reached past the runner would pass while production did nothing.
//
// The compositions are `screenfixture`'s, which are shaped from what the repaired perception
// path actually measured against Chrome, VS Code and Task Manager. A three-element fixture
// would prove the substrate works on three elements.

// script is one deterministic session: what was on screen, and what the player did before it.
type script struct {
	frames []frame
	// source is which structural provider the composition came from. Empty means the fused
	// authoritative world, which is what almost every script here means.
	//
	// It is on the SCRIPT rather than the frame because a session's structure has one
	// provenance: the point of carrying it is to prove the model answers the same way
	// whoever was looking, and a script that mixed them would be testing something else.
	source observe.StructuralSource
}

type frame struct {
	regions []observe.ShadowRegion
	// unobserved marks a frame on which nothing looked at all.
	//
	// NOT the same as a frame with no regions, and the difference is the whole reason
	// StructuralView carries provenance: "there is nothing here" is a screen and "I could
	// not see" is not.
	unobserved bool
	// inputs are the navigation intents observed since the previous frame.
	inputs []observe.InputEvent
	// terms are the CLOSED interface concepts a scoped reading classified on this screen.
	//
	// Optional, and its absence is load-bearing rather than an omission: a screen with no
	// read text and no envelope carries no discriminator, so it can never become a durable
	// subject. Both cases are real — the live sessions reported `back, notifications,
	// search` for VS Code and `settings` for Chrome, and also ran many samples with no
	// label pass at all — so both are modelled.
	terms []observe.InterfaceTerm
}

// scripted is a sampler that plays one script and then holds its last frame.
//
// It supplies `Structure` exactly as the live sampler does — the fused composition with its
// provenance — and rides navigation on the shadow record, which is where the live sampler puts
// it whether or not a structural detector is configured.
type scripted struct {
	s     script
	calls int
}

func (p *scripted) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	f := p.s.frames[len(p.s.frames)-1]
	if p.calls < len(p.s.frames) {
		f = p.s.frames[p.calls]
	} else {
		f.inputs = nil // the script ended; the player stopped
	}
	p.calls++

	source := p.s.source
	if source == "" {
		source = observe.StructureFused
	}
	if f.unobserved {
		source = observe.StructureUnobserved
	}
	out := observe.Sample{
		Structure: observe.StructuralView{Source: source, Regions: f.regions},
	}
	if len(f.inputs) > 0 || len(f.terms) > 0 {
		// Navigation and classified terms ride on the shadow record because that is where
		// liveSampler puts them, through ensureShadow, even on a Director with no
		// structural detector.
		sh := &observe.ShadowSample{Inputs: f.inputs}
		if len(f.terms) > 0 {
			sh.Semantic = observe.SemanticEvidence{Observed: true, Terms: f.terms}
		}
		out.Shadow = sh
	}
	return out, nil
}

// reading marks every frame as one on which a scoped text pass classified these concepts.
func reading(in []frame, terms ...observe.InterfaceTerm) []frame {
	out := make([]frame, len(in))
	copy(out, in)
	for i := range out {
		out[i].terms = terms
	}
	return out
}

// hold repeats one composition n times with nothing happening.
func stayOn(regions []observe.ShadowRegion, n int) []frame {
	out := make([]frame, 0, n)
	for range n {
		out = append(out, frame{regions: regions})
	}
	return out
}

// blind is n frames on which nothing looked at all.
func blind(n int) []frame {
	out := make([]frame, 0, n)
	for range n {
		out = append(out, frame{unobserved: true})
	}
	return out
}

// press repeats a composition n times, with navigation observed before the FIRST of them.
func pressThen(regions []observe.ShadowRegion, n int, intents ...observe.NavIntent) []frame {
	out := make([]frame, 0, n)
	for i := range n {
		f := frame{regions: regions}
		if i == 0 {
			for j, intent := range intents {
				f.inputs = append(f.inputs, observe.InputEvent{
					Intent: intent, AtMS: int64(j) * 40,
				})
			}
		}
		out = append(out, f)
	}
	return out
}

func run(t *testing.T, s script) observesession.Result {
	t.Helper()
	got, err := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&scripted{s: s}, &recordingEvents{}).Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return got
}

// edge finds the transition between two states, by their observed order.
func edgeBetween(sh observe.ShadowTotals, from, to observe.ScreenStateID) (observe.ScreenTransition, bool) {
	for _, tr := range sh.Transitions {
		if tr.From == from && tr.To == to {
			return tr, true
		}
	}
	return observe.ScreenTransition{}, false
}

// ── Part 3: stability ─────────────────────────────────────────────────────────

// An application sitting still produces ONE screen and NO transitions, however much its
// accessibility tree churns.
//
// The failure this guards is a transition storm: a segmenter that read churn as change would
// bury a real interaction in hundreds of fabricated ones, and every relationship it recorded
// would be noise.
func TestARealShapedApplicationSittingStillProducesNoTransition(t *testing.T) {
	for name, base := range map[string][]observe.ShadowRegion{
		"editor":  screenfixture.Editor(),
		"browser": screenfixture.Browser(),
		"sparse":  screenfixture.Sparse(),
	} {
		t.Run(name, func(t *testing.T) {
			var frames []frame
			// Every kind of harmless variation a live tree produces, in turn.
			//
			// The jitter magnitudes matter and are stated rather than assumed: 1% of the
			// window is about nineteen pixels on a 1920-wide screen, which is an ordinary
			// layout re-measure — a scrollbar appearing, a panel resizing by a hair. A
			// screen model that split on that would split on nothing.
			//
			// Sweeping the magnitude rather than fixing one is what makes this catch an
			// over-sensitive model: a finer spatial grid keeps a tiny jitter inside its
			// cell and only shows up at a realistic one.
			for i, jitter := range []float64{0, 0.004, 0.010, 0.016, 0, 0.008} {
				v := screenfixture.Jitter(base, jitter)
				switch i % 3 {
				case 1:
					v = screenfixture.Reorder(v)
				case 2:
					v = screenfixture.Churn(v, 2, 1)
				}
				frames = append(frames, stayOn(v, 3)...)
			}
			sh := run(t, script{frames: frames}).Stats.Shadow

			if len(sh.States) != 1 {
				t.Errorf("an application sitting still produced %d screens", len(sh.States))
			}
			if len(sh.Transitions) != 0 {
				t.Errorf("%d transitions from an application that did not change",
					len(sh.Transitions))
			}
		})
	}
}

// ── Part 4: a meaningful transition, through the real path ────────────────────

// A genuinely different screen becomes a second state and a transition between them.
//
// The change modelled is the one the identity model can currently see: the principal content
// region replaced. See screenfixture's threshold characterisation for the interactions it
// cannot.
func TestAMeaningfulChangeBecomesATransitionOnTheProductionPath(t *testing.T) {
	editor, settings := screenfixture.Editor(), screenfixture.Settings()

	var frames []frame
	frames = append(frames, stayOn(editor, 5)...)
	frames = append(frames, pressThen(settings, 5, observe.NavConfirm)...)
	frames = append(frames, stayOn(editor, 4)...)
	frames = append(frames, pressThen(settings, 4, observe.NavConfirm)...)

	sh := run(t, script{frames: frames}).Stats.Shadow

	if len(sh.States) != 2 {
		t.Fatalf("an editor and a settings page produced %d screen(s)", len(sh.States))
	}
	if len(sh.Transitions) == 0 {
		t.Fatal("two screens alternating produced no transition. Nothing downstream — " +
			"relationships, learning, rehearsal — has anything to work with")
	}

	// BOTH directions, because direction is part of a relationship's identity: a page that
	// opens on `confirm` and closes on `back` must not collapse into one edge that appears
	// to respond to both.
	a, b := sh.States[0].ID, sh.States[1].ID
	fwd, okFwd := edgeBetween(sh, a, b)
	back, okBack := edgeBetween(sh, b, a)
	if !okFwd || !okBack {
		t.Fatalf("only one direction between %s and %s was recorded: forward=%v back=%v",
			a, b, okFwd, okBack)
	}

	// The script made the round trip twice, so one direction was seen twice and the other
	// once. Which is which depends on the ordering of States, and asserting on that would
	// be asserting on a sort.
	most, least := fwd.Count, back.Count
	if least > most {
		most, least = least, most
	}
	if most != 2 || least != 1 {
		t.Errorf("a script with two openings and one closing recorded %d and %d",
			most, least)
	}
	// The script pressed `confirm` before each OPENING and nothing before the closing, so
	// the same pair of screens carries two attributed observations and one unattributed —
	// which is the honest shape, and the one that keeps a correlation readable as a
	// correlation rather than as a cause.
	if got := fwd.Attributed() + back.Attributed(); got != 2 {
		t.Errorf("%d changes carried navigation; the script supplied it before two", got)
	}
	if got := fwd.Unattributed + back.Unattributed; got != 1 {
		t.Errorf("%d changes were recorded as unattributed; the script caused one with "+
			"nothing observed before it", got)
	}
}

// ── Part 5: unknown → unknown ─────────────────────────────────────────────────

// Neither screen needs to be recognised, named, or read for the transition to be evidence.
//
// The essential property. If Director had to understand a UI before it could learn that the UI
// changed, learning could never start on unfamiliar software.
func TestATransitionBetweenTwoUnknownScreensIsStillEvidence(t *testing.T) {
	var frames []frame
	frames = append(frames, stayOn(screenfixture.Editor(), 5)...)
	frames = append(frames, pressThen(screenfixture.Settings(), 5, observe.NavConfirm)...)

	got := run(t, script{frames: frames})
	sh := got.Stats.Shadow

	if len(sh.Transitions) == 0 {
		t.Fatal("no transition between two unknown screens")
	}
	// Nothing was recognised: this runner has no memory at all.
	for _, p := range got.Proposals.Proposals {
		if p.Recognised {
			t.Error("a session with no memory recognised a screen")
		}
	}
	// Nothing was named, and nothing was read.
	for _, st := range sh.States {
		if st.TermObservations != 0 {
			t.Errorf("state %s carries term evidence from a session that read no text", st.ID)
		}
	}
	// The states are session-local counters and say so.
	for _, st := range sh.States {
		if st.ID == "" || st.ID == observe.ScreenStateUnknown {
			t.Errorf("a screen that was observed has no identity: %q", st.ID)
		}
	}
}

// ── Part 6: attribution is separate from existence ────────────────────────────

// A. A change nobody caused is still a transition, recorded as unattributed.
func TestAChangeWithNoNavigationIsAnUnattributedTransition(t *testing.T) {
	var frames []frame
	frames = append(frames, stayOn(screenfixture.Editor(), 5)...)
	frames = append(frames, stayOn(screenfixture.Settings(), 5)...) // no inputs at all

	sh := run(t, script{frames: frames}).Stats.Shadow
	if len(sh.Transitions) == 0 {
		t.Fatal("a change with no observed cause was not recorded at all. A timer, a network " +
			"event or a scene load would be invisible")
	}
	tr := sh.Transitions[0]
	if tr.Unattributed == 0 {
		t.Errorf("a change with no navigation before it was not counted as unattributed: %+v", tr)
	}
	if len(tr.Preceded) != 0 {
		t.Errorf("a change with no navigation before it acquired a cause: %v", tr.Preceded)
	}
}

// B. A change with navigation before it carries the correlation — and only as correlation.
func TestAChangeWithNavigationCarriesTheCorrelation(t *testing.T) {
	var frames []frame
	frames = append(frames, stayOn(screenfixture.Editor(), 5)...)
	frames = append(frames, pressThen(screenfixture.Settings(), 5,
		observe.NavDown, observe.NavConfirm)...)

	sh := run(t, script{frames: frames}).Stats.Shadow
	tr := sh.Transitions[0]

	if tr.Preceded[observe.NavConfirm] == 0 {
		t.Fatalf("the navigation observed before the change did not reach it: %+v", tr)
	}
	if tr.Unattributed != 0 {
		t.Errorf("an attributed change was also counted as unattributed")
	}
	// The ORDER survives, because `down then confirm` and `confirm then down` are different
	// interactions and an unordered bundle has thrown that away.
	if len(tr.Sequences) == 0 {
		t.Fatal("the order of the navigation was not retained")
	}
	if got := tr.Sequences[0].Intents; len(got) != 2 ||
		got[0] != observe.NavDown || got[1] != observe.NavConfirm {
		t.Errorf("the order was not preserved: %v", got)
	}
}

// C. Navigation during a stable stretch cannot be attached to a later change.
//
// The staleness guard. Somebody scrolling around a screen for a minute and then opening a menu
// must not have the whole minute attributed to the opening.
func TestNavigationDuringAStableStretchIsNotAttributedToALaterChange(t *testing.T) {
	editor, settings := screenfixture.Editor(), screenfixture.Settings()

	var frames []frame
	frames = append(frames, stayOn(editor, 2)...)
	// A long stretch of the player doing things that changed nothing.
	frames = append(frames, pressThen(editor, 1, observe.NavLeft)...)
	frames = append(frames, pressThen(editor, 1, observe.NavRight)...)
	frames = append(frames, pressThen(editor, 1, observe.NavBack)...)
	frames = append(frames, stayOn(editor, 3)...)
	// Then the one that did.
	frames = append(frames, pressThen(settings, 5, observe.NavConfirm)...)

	sh := run(t, script{frames: frames}).Stats.Shadow
	tr := sh.Transitions[0]

	for _, stale := range []observe.NavIntent{observe.NavLeft, observe.NavRight, observe.NavBack} {
		if tr.Preceded[stale] > 0 {
			t.Errorf("%q was pressed while the screen was NOT changing and was attributed "+
				"to a change several samples later: %v", stale, tr.Preceded)
		}
	}
	if tr.Preceded[observe.NavConfirm] == 0 {
		t.Error("the navigation immediately before the change was lost")
	}
}

// D. Context-admitted evidence stays distinguishable from unambiguous evidence.
//
// A key that is only navigation because the screen looked like a set of choices is real
// evidence and weaker evidence. Folding the two together would let a session of somebody
// walking around a game produce the same confident edge as one of somebody using a menu.
func TestContextAdmittedNavigationStaysWeaker(t *testing.T) {
	conditional := func(regions []observe.ShadowRegion, n int,
		intents ...observe.NavIntent) []frame {

		out := pressThen(regions, n, intents...)
		for i := range out[0].inputs {
			out[0].inputs[i].Conditional = true
		}
		return out
	}

	var frames []frame
	frames = append(frames, stayOn(screenfixture.Editor(), 4)...)
	frames = append(frames, conditional(screenfixture.Settings(), 4, observe.NavDown)...)

	sh := run(t, script{frames: frames}).Stats.Shadow
	tr := sh.Transitions[0]

	if tr.ConditionalOnly == 0 {
		t.Fatalf("an observation resting entirely on context-admitted keys was recorded as "+
			"unambiguous: %+v", tr)
	}
	if tr.Attributed() != tr.ConditionalOnly {
		t.Errorf("attributed=%d conditional_only=%d; the weak evidence is being counted as "+
			"partly strong", tr.Attributed(), tr.ConditionalOnly)
	}

	// And one unambiguous key among them is enough to carry the observation on its own.
	mixed := func(regions []observe.ShadowRegion, n int) []frame {
		out := pressThen(regions, n, observe.NavDown, observe.NavConfirm)
		out[0].inputs[0].Conditional = true
		return out
	}
	var frames2 []frame
	frames2 = append(frames2, stayOn(screenfixture.Editor(), 4)...)
	frames2 = append(frames2, mixed(screenfixture.Settings(), 4)...)
	if tr2 := run(t, script{frames: frames2}).Stats.Shadow.Transitions[0]; tr2.ConditionalOnly != 0 {
		t.Errorf("an observation containing an unambiguous key was written off as "+
			"context-admitted: %+v", tr2)
	}
}

// E. Several different intents before one change do not collapse into one confident cause.
func TestSeveralIntentsBeforeOneChangeDoNotBecomeOneCause(t *testing.T) {
	var frames []frame
	frames = append(frames, stayOn(screenfixture.Editor(), 4)...)
	frames = append(frames, pressThen(screenfixture.Settings(), 4,
		observe.NavDown, observe.NavLeft, observe.NavConfirm)...)

	sh := run(t, script{frames: frames}).Stats.Shadow
	tr := sh.Transitions[0]

	if len(tr.Preceded) < 3 {
		t.Fatalf("three intents before one change were reduced to %d: %v",
			len(tr.Preceded), tr.Preceded)
	}
	// The dominant intent is reported WITH its count, so one-of-one cannot read as certainty.
	_, n := tr.Dominant()
	if n > tr.Count {
		t.Errorf("an intent was counted %d times against %d observations", n, tr.Count)
	}
	for intent, count := range tr.Preceded {
		if count > tr.Count {
			t.Errorf("%q preceded %d of %d observations", intent, count, tr.Count)
		}
	}
}

// E, continued. A key delivered many times before ONE change counts once against it.
//
// The question a correlation answers is which intents were present before the change, not how
// many events the hook produced. A held direction key delivering forty events must not outvote
// a single deliberate confirm, and a table that let it would make the loudest key look like the
// cause of everything.
//
// Found by mutation: removing the per-transition deduplication survived every other test,
// because nothing pressed the same key twice in one interval.
func TestARepeatedIntentCountsOnceAgainstOneChange(t *testing.T) {
	held := func(regions []observe.ShadowRegion, n int) []frame {
		out := pressThen(regions, n)
		for i := range 40 {
			out[0].inputs = append(out[0].inputs, observe.InputEvent{
				Intent: observe.NavDown, AtMS: int64(i) * 10,
			})
		}
		out[0].inputs = append(out[0].inputs, observe.InputEvent{
			Intent: observe.NavConfirm, AtMS: 500,
		})
		return out
	}
	var frames []frame
	frames = append(frames, stayOn(screenfixture.Editor(), 4)...)
	frames = append(frames, held(screenfixture.Settings(), 5)...)

	sh := run(t, script{frames: frames}).Stats.Shadow
	tr := sh.Transitions[0]

	if tr.Preceded[observe.NavDown] > tr.Count {
		t.Errorf("a key delivered 40 times before ONE change was credited %d times against "+
			"%d observation(s); a held key would outvote every deliberate press",
			tr.Preceded[observe.NavDown], tr.Count)
	}
	if tr.Preceded[observe.NavDown] != tr.Preceded[observe.NavConfirm] {
		t.Errorf("forty presses of one key and one press of another were counted "+
			"differently: down=%d confirm=%d",
			tr.Preceded[observe.NavDown], tr.Preceded[observe.NavConfirm])
	}
	// The ORDER still records the repeats — a selection moved forty times really did move
	// forty times, and reconstructing a procedure needs that. Only the CORRELATION table
	// deduplicates.
	if len(tr.Sequences) == 0 || len(tr.Sequences[0].Intents) < 2 {
		t.Errorf("the order of a held key followed by a confirm was lost: %v", tr.Sequences)
	}
}

// F. The transition survives when attribution is refused entirely.
//
// Navigation and screen evidence fail independently. An unrecognised key, a producer that was
// not listening, a session with no input source at all — none of them may take the transition
// down with them.
func TestATransitionSurvivesWhenNavigationIsRefused(t *testing.T) {
	refused := func(regions []observe.ShadowRegion, n int) []frame {
		out := pressThen(regions, n, observe.NavConfirm)
		// An intent outside the closed vocabulary. admissibleInputs drops it.
		out[0].inputs = []observe.InputEvent{{Intent: observe.NavIntent("wiggle"), AtMS: 10}}
		return out
	}
	var frames []frame
	frames = append(frames, stayOn(screenfixture.Editor(), 4)...)
	frames = append(frames, refused(screenfixture.Settings(), 4)...)

	sh := run(t, script{frames: frames}).Stats.Shadow
	if len(sh.Transitions) == 0 {
		t.Fatal("a refused navigation observation took the whole transition with it")
	}
	tr := sh.Transitions[0]
	if len(tr.Preceded) != 0 {
		t.Errorf("an intent outside the closed vocabulary was attributed: %v", tr.Preceded)
	}
	if tr.Unattributed == 0 {
		t.Error("a change whose only navigation was refused was not counted as unattributed")
	}
}
