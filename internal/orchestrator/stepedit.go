package orchestrator

// EDITING A STEP: THE STEP LIST IS WHAT CHANGES, AND THE SOURCE IS RE-RENDERED FROM IT.
//
// Three operations — set a wait's duration, remove a step, make the text a step types something
// you SUPPLY — and the same four-step contract every other writer in this package uses:
//
//	PREPARE   read the routine, work out the step list its saved source is the render OF, apply
//	          the operations to THAT, render again. NOTHING IS WRITTEN. What comes back is both
//	          sources in full, the edited step list, and digests of everything it was built from.
//	REVIEW    the caller shows both sources and what the operations were.
//	SAVE      under the route's lock, revalidating all three files; assets, then the source,
//	          then the account.
//	CANCEL    the absence of an operation, because nothing was written.
//
// # Why the step list and not the lines
//
// Because a step's boundary is in the IR and only approximately in the file. `codegen` writes one
// of two shapes: a chained routine makes every press its own capability, called inside the
// previous one's `when ok?`, with the single `this is ok!` and the trailing settle wait inside the
// LAST link — so removing a press is a re-render and not a deletion, and removing the last press
// is a re-render that hands those two to the link that is now last. In the unguarded shape an
// anchored click is a dozen lines three indents deep. `cmd/marco/edit.go` edits lines, and its own
// test records what that costs: deleting a `do OS's Find …` row takes the header and leaves
// `when ok?` with no arm, which the compile gate then refuses.
//
// # What it is allowed to be an edit OF
//
// Only a routine whose saved source is exactly what its own current account renders today — the
// stored step list when there is one, the demonstration when there is not. That is checked by
// rendering and comparing, not asserted. Without it, changing one wait on a routine written by an
// older generator would rewrite the whole file under today's rules, which is the rebuild review's
// decision — made by somebody who was shown both sources — and not an edit.
//
// After the first save the source differs from the demonstration's rendering ON PURPOSE, and that
// difference IS the edit. So the second edit compares against the stored steps, which is what
// keeps this from being a one-use editor.
//
// # What it never touches
//
// The demonstration. `.rec.json` is the only durable account of what the person did; an edit is a
// decision about what Marco will do. A surface that rewrote the second to express the first would
// be editing somebody's afternoon.
//
// See [[Step-editing-proposal-2026-10-30]].

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// ErrStepsNotWritten is the answer when a routine's SOURCE landed and its account did not.
//
// A DISTINCT OUTCOME AND THEREFORE A DISTINCT ANSWER. The routine on disk is the one the person
// asked for and it runs exactly as they asked, because execution reads only the `.marco`; what
// failed is the explanation beside it, so further editing is blocked until the routine is rebuilt.
// A surface that reported this as "nothing was written" would be telling somebody their save did
// not happen, and one that reported it as success would leave them to discover the refusal later.
//
// TWO WRITERS NOW, which is why the sentence says nothing about an edit. A step edit lands here,
// and so does a SAVE of a draft that carries its own basis — the simplified one, whose account is
// the whole of what makes it editable afterwards. Each caller supplies the clause saying what was
// saved; this says what did not happen to it.
//
// Wrapped, so the sentence naming the routine and the reason travels with it.
var ErrStepsNotWritten = errors.New("its steps were not written down beside it")

// EditOp is one of the three things an edit can do; see the note above.
type EditOp string

const (
	// EditWait sets a wait's duration, in milliseconds.
	EditWait EditOp = "wait"
	// EditRemove removes one step, whole.
	EditRemove EditOp = "remove"
	// EditPoint moves where a press happens.
	//
	// It changes ONE step and leaves the list the length it was, so unlike EditTyped it
	// collects with removals and waits in the ordinary way.
	//
	// WHICH PAIR IT SETS IS THE STEP'S OWN ANSWER, never the caller's: a press measured
	// inside a window is moved by its OFFSET, and the point on the screen follows out of
	// the same measurement; a press that was never measured against a window is moved by
	// the screen point. A caller that chose would be free to give a press a window it was
	// never recorded in. See pointedit.go, which is the whole of the rule.
	EditPoint EditOp = "point"
	// EditArgument makes the text a step types something SUPPLIED, under a name.
	//
	// # Why this is an edit rather than something you do while demonstrating
	//
	// It was already possible, and the two ways in were both invisible: name the recording
	// `write a note with message` so the phrase declares an argument, and tap a RESERVED KEY
	// (`simplify.DefaultArgKey`, f9) at the moment in the demonstration where the text goes.
	// Both have to be known in advance, and neither is discoverable from any surface — the
	// owner of this repository could not find them, which is how this came to be built.
	//
	// As an edit it needs no foreknowledge and no second demonstration: somebody types the
	// text they actually want, sees it in the step list, and says THIS is the part that
	// changes. It works on routines that were saved before any of this existed.
	//
	// What it does is replace the step's literal with `{{name}}`, which is the placeholder
	// the whole of [[ADR-276-a-routine-may-be-told-what-to-type]] is already built on:
	// `routes.Arguments` reads it, Studio's field beside Play asks for it, `routes.ApplyArgs`
	// fills it at run time, and a request can now propose it
	// ([[ADR-277-a-request-may-propose-the-text-a-routine-types]]). Nothing downstream needed
	// changing, which is the whole reason this is three lines of IR and not a feature.
	EditArgument EditOp = "argument"
	// EditTyped replaces a RANGE of steps with one step that types the text they typed,
	// because somebody has said that is what they are.
	//
	// THE ONLY OPERATION HERE THAT IS NOT ABOUT ONE STEP, and the reason it needs a
	// second position: typing comes back from a recording as held keys whenever the next
	// key went down before the last came up, so the thing being declared is a run. See
	// `typedRun`, which is also what the recording review asks
	// ([Deps.TypedInDraft]), and [[ADR-282-a-run-of-steps-can-be-said-to-be-typed-text]].
	//
	// IT IS REVIEWED ON ITS OWN. Every other operation keeps the list the same length, so
	// they can be collected against the positions the caller was shown and applied
	// together; this one makes it shorter, which moves every position after it. Rather
	// than translate the others — which cannot be done honestly, because nothing says
	// whether a position was read before or after the run — a review holding this holds
	// nothing else, and says so.
	EditTyped EditOp = "typed"
)

// argName is what a placeholder's name may be: exactly what `routes` own pattern will read back.
//
// THE SAME CHARACTERS AND NO OTHERS. A name this accepts and `phRe` does not would be written
// into the source as text that looks like a placeholder and is never filled — the routine would
// type two braces and the person's name at them.
var argName = regexp.MustCompile(`^[a-z0-9_-]+$`)

// StepEdit is one operation, against a position in the step list as the caller was shown it.
//
// # Why the index is into the list that was displayed
//
// Because every operation in one review refers to the same list, and applying them one after
// another would move the positions under each other: remove step 3 and the caller's "step 5" is
// now step 4. So the operations are collected against the ORIGINAL positions and the new list is
// built once. See applyEdits.
type StepEdit struct {
	Op EditOp `json:"op"`
	// At is the position in the step list, from zero.
	At int `json:"at"`
	// To is the new duration in milliseconds, for EditWait. Ignored otherwise.
	To int `json:"to"`
	// Upto is the LAST position of the range, for EditTyped, with `At` the first and both
	// ends in it. Ignored otherwise.
	//
	// Its own term rather than `To`, which is a duration: a field meaning milliseconds for
	// one operation and a step number for another is a field whose meaning a reader has to
	// hold in their head, in the one struct where a wrong number changes somebody's
	// routine quietly.
	Upto int `json:"upto"`
	// X and Y are where the press should be, for EditPoint. Ignored otherwise.
	//
	// IN WHICHEVER BASIS THE STEP ITSELF USES — the offset inside the window when it was
	// measured in one, the point on the screen when it was not. `MovablePoint.InWindow`
	// is what a surface shows beside the box, and it comes from the same reading this
	// applies against, so the two cannot disagree about which pair somebody typed.
	X, Y int `json:"x,omitempty"`
	// Name is what the supplied text is called, for EditArgument. Ignored otherwise.
	//
	// LOWERCASED when it is applied, because every reader downstream compares these
	// case-insensitively — `CheckArgs` lowers them, `zipPositional` lowers them — and a
	// stored `{{Message}}` beside a field asking for `message` is a confusion with nothing
	// to gain from it.
	Name string `json:"name"`
}

// EditReview is one routine with its step list edited, for review.
//
// NOTHING ON DISK HAS CHANGED. Every field is a reading or a rendering held in memory, and the
// routine is exactly as it was until [Deps.SaveEdit] is called with this and succeeds.
type EditReview struct {
	// Token identifies THIS review, and a save must carry it back. Derived from the
	// content and the target, exactly as a rebuild's is: two reviews of one routine are
	// two different things to write, and a slot keyed by the routine writes whichever
	// arrived last. See Prepared.Token for the defect that lesson came from.
	Token string
	// Route is the exact routine this is about.
	Route routes.Route
	// Name is what the routine calls itself, as its own header records it.
	Name string
	// Edits is what was asked, in the order it was asked.
	//
	// IN MEMORY AND NOWHERE ELSE. Nothing durable records editing history: what the
	// routine does now is necessary and a record of how it got there is not, and the
	// source comparison below is better evidence of what an edit costs than any summary.
	Edits []StepEdit
	// Basis identifies the step list this review was built against — the list a surface
	// displayed, and the positions every operation in `Edits` refers to.
	//
	// WHY A SURFACE NEEDS ONE. A panel lists the steps at one moment and sends row
	// numbers at another, and between the two anything can write this routine: a hand
	// edit, a re-recording, another edit of it from a second window. Applying row 5 to a
	// freshly loaded list is applying somebody's intention to a routine they never saw,
	// and the result is a perfectly good routine that does the wrong thing — which no
	// check after the fact can catch, because by then the operations have already been
	// applied to the wrong steps.
	//
	// It is DERIVED FROM THE THREE DIGESTS and nothing else, because the step list is a
	// pure function of them: the account's steps when it is intact, the demonstration's
	// rendering when there is none. So this moves when any of the three files moves, and
	// does not move otherwise.
	//
	// Separate from `Token`, which identifies the whole review. This identifies only what
	// the operations were counted against, which is the question a surface has to ask
	// BEFORE it is allowed to state any.
	Basis string
	// Says is one line per operation, for the review to show.
	//
	// It names positions and durations and does NOT describe what a step IS — the surface
	// already has words for a step (`oneStep`), and a second vocabulary here would be a
	// second thing to disagree with it.
	Says []string
	// Was is the saved source as it stands, and Now the source a save would write. BOTH
	// IN FULL, because a list of changed lines is an interpretation and the file is the
	// thing that runs.
	Was, Now string
	// Refused is why these operations produce no routine, in as few words as the reason
	// allows, and empty when they produce one.
	//
	// A REVIEWABLE ANSWER AND NOT AN ERROR, for the cases where the operations are
	// expressible and the RESULT is not a routine: an edit that would empty it, a held key
	// whose partner cannot be found, a render the generator refuses, a render that is not
	// legal Marco. An operation that cannot be expressed at all — a position that is not
	// there, a duration on something that is not a wait — is an error instead, because
	// there is nothing to show anybody.
	Refused string
	// library is the store this was reviewed against, which is part of the target and
	// therefore part of the token. See Prepared.library.
	library string
	// sourceAt, recordingAt and stepsAt are digests of the three files this was built
	// from. A save re-reads all three and refuses if any moved, naming which.
	//
	// THE THIRD ONE IS WHY IT IS THREE. An account that moved means another edit of the
	// same routine landed while this review was open, and writing anyway would undo it
	// invisibly: the result would look exactly like an ordinary successful save.
	sourceAt, recordingAt, stepsAt string
	// steps is the edited step list — what the account would hold.
	steps []macroir.Step
	// stepsToWrite is that list marshalled, done ONCE here rather than at the write.
	//
	// So the bytes that will land beside the routine are fixed at the moment the review
	// is, and the token covers them: a step list that could be marshalled at the write
	// is a step list that could be different by then, and the account would describe a
	// routine the review never showed anybody.
	stepsToWrite []byte
	// assets are the files the rendering produced, held here rather than written.
	assets map[string][]byte
}

// Savable is whether this review has something a save could write.
func (e EditReview) Savable() bool {
	return e.Refused == "" && e.Now != "" && e.Now != e.Was && len(e.Edits) > 0
}

// Changes is whether a save would change the file at all.
func (e EditReview) Changes() bool { return e.Refused == "" && e.Now != "" && e.Now != e.Was }

// Steps is the step list this review is about — the edited one, or the routine's own when nothing
// was asked.
//
// A copy, because a caller holding the slice this review was sealed over could change what the
// token says it is.
func (e EditReview) Steps() []macroir.Step { return clonedSteps(e.steps) }

// clonedSteps is a step list a caller may do anything at all to.
//
// DEEP, AND THE DEPTH IS THE POINT. A `macroir.Step` carries two references: `Template`, the PNG
// an anchored click looks for, and `Steps`, a loop's body. Copying the outer slice leaves both of
// those SHARED with the review the copy came from — so a caller rewriting a byte of an anchor
// image, or a step inside a repetition, would change what the review says it is while its token,
// which is derived from the marshalled bytes, went on saying otherwise. The shallow version of
// this looked exactly like a copy and defended nothing an anchored click or a folded loop cares
// about.
func clonedSteps(in []macroir.Step) []macroir.Step {
	if in == nil {
		return nil
	}
	out := make([]macroir.Step, len(in))
	for i, st := range in {
		if st.Template != nil {
			st.Template = append([]byte(nil), st.Template...)
		}
		st.Steps = clonedSteps(st.Steps)
		out[i] = st
	}
	return out
}

// PairedWith is the position of the other half of a held key at `at`, or -1 when that step is
// not half of one.
//
// FOR THE SURFACE TO SAY SO BEFORE ANYBODY PRESSES ANYTHING. A removal that quietly took two steps
// would be a removal nobody agreed to, and the panel has to be able to name the partner on the row
// beside the control. Computed by the same function the removal itself uses, so what the panel
// says and what the operation does cannot disagree about which pair is which.
//
// -1 for a position that is not there, which is the honest answer and not a pair.
func (e EditReview) PairedWith(at int) int {
	if at < 0 || at >= len(e.steps) {
		return -1
	}
	partner, _ := holdPartner(e.steps, at)
	return partner
}

// PrepareEdit works out what a routine's step list is, applies the operations to it, and renders
// the result. IT WRITES NOTHING.
//
// Called with no operations it is the question "may this be edited, and what are its steps?" —
// answered through the same path that would perform one, so a surface cannot be told yes by one
// set of rules and refused by another.
func (d Deps) PrepareEdit(rt routes.Route, edits []StepEdit) (EditReview, error) {
	return d.PrepareEditOf(rt, "", edits)
}

// PrepareEditOf is PrepareEdit against a step list somebody has already been shown.
//
// `shown` is the `Basis` of the review that displayed the list these operations count against, and
// it is REFUSED when the routine's step list is no longer that one. See EditReview.Basis for why
// an index is meaningless without it.
//
// An empty `shown` states nothing and checks nothing, which is what the first call of a flow
// passes: there is nothing to have been shown yet. A surface that offers operations must state one
// on the second call, and that is a rule of the surface rather than of this function — there is no
// way to tell, from here, whether a caller had a list on a screen.
func (d Deps) PrepareEditOf(rt routes.Route, shown string, edits []StepEdit) (EditReview, error) {
	if rt.Slug == "" {
		return EditReview{}, fmt.Errorf("an edit needs the routine it is about")
	}
	raw, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		return EditReview{}, fmt.Errorf("%s could not be read, so there is nothing to "+
			"edit: %w", rt.Slug, err)
	}
	src := string(raw)
	at, readable := d.Reg.StepsDigest(rt)
	if !readable {
		// THERE AND UNREADABLE IS NOT ABSENT. A review is of three files and this
		// is one of them; a review bound to "" for a file that exists would
		// compare "" with "" at the write and go ahead having never read it. See
		// routes.Registry.StepsDigest.
		return EditReview{}, fmt.Errorf("%s cannot be edited: the step list stored "+
			"beside it is there and could not be read, so nothing here can "+
			"say what its source is the render of", rt.Slug)
	}
	e := EditReview{Route: rt, Name: NameIn(src, rt.Slug), Was: src, Edits: edits,
		library: d.Reg.Dir, sourceAt: routes.DigestOf(src), stepsAt: at}

	// ── ELIGIBILITY, AGAINST THE SOURCE THIS ACTUALLY READ ──
	//
	// The same restriction the rebuild review applies, in the same place and for the same
	// reason: a caller's listing is one read of the library, and the file can have been
	// replaced since by something stating a generation this build cannot explain.
	if g := GenerationOf(src); !g.Offerable() {
		return EditReview{}, fmt.Errorf("%q cannot be edited: %s", e.Name,
			GenerationSentence(g, d.Reg.HasRecording(rt)))
	}
	rec, ok := d.Reg.LoadRecording(rt)
	if !ok {
		// EDITING NEEDS THE DEMONSTRATION EVEN WHEN IT IS NOT THE BASIS, because
		// the account has to say which demonstration it descends from — an account
		// that could not name one is an account nothing can check, and that is the
		// one state routes.SaveSteps refuses to write.
		return EditReview{}, fmt.Errorf("%q no longer has the demonstration it was "+
			"made from, so its steps cannot be edited — recording it again is "+
			"the way to get a routine that can be", e.Name)
	}
	e.recordingAt = routes.DigestOf(string(rec))
	e.Basis = routes.DigestOf(strings.Join([]string{e.sourceAt, e.recordingAt,
		e.stepsAt}, "\x00"))
	if shown != "" && shown != e.Basis {
		// THE LIST THE OPERATIONS WERE COUNTED AGAINST IS NOT THIS ROUTINE'S LIST.
		//
		// Refused rather than applied, and refused BEFORE anything is rendered: the
		// positions in `edits` mean something only against the list they were read
		// off, and this routine has been written since. Nothing here can translate
		// them, because a step inserted or removed by somebody else moves every
		// position after it and nothing says which.
		return EditReview{}, fmt.Errorf("%q has been written since its steps were "+
			"listed, so the changes you asked for refer to steps it no longer "+
			"has — look at it again and ask once more", e.Name)
	}

	// ── WHAT THE SAVED SOURCE IS THE RENDER OF ──
	basis, why, err := d.basisFor(rt, src, rec)
	if err != nil {
		return EditReview{}, err
	}
	if why != "" {
		return EditReview{}, fmt.Errorf("%q cannot be edited: %s", e.Name, why)
	}
	// AND THAT IS CHECKED BY RENDERING IT, NOT ASSERTED. An account's digests say it
	// describes this file; they do not say that rendering it today reproduces this file,
	// which is a different question and the one that matters — an account bound to a
	// source written by an older generator renders to something else now.
	scope := ScopeAlreadyOf(rt)
	asIs, _, rerr := d.render(&Draft{Name: basis.name, Args: basis.args,
		Steps: basis.steps, Mode: basis.mode}, scope.mode(), rt.App, rt)
	if rerr != nil || asIs != src {
		return EditReview{}, fmt.Errorf("%q cannot be edited: %s", e.Name, basis.mismatch)
	}

	// ── THE OPERATIONS ──
	edited, says, refused, declared, aerr := applyEdits(basis.steps, edits)
	if aerr != nil {
		return EditReview{}, aerr
	}
	// AND THE NAMES THE EDIT DECLARED JOIN THE ROUTINE'S OWN, for the render below.
	//
	// Without them `codegen` keeps a `{{name}}` in the typed literal only if the routine
	// already declared it, and turns any other one into `do OS's Secret …`. So the first
	// version of this rendered `do OS's Secret with "message"` where
	// `do OS's Type with "{{message}}"` belonged — a routine that reads somebody's
	// credential store where they asked for their own words. Found by reading the render,
	// not by a failing test: every assertion was about the step list, which was right.
	args := append(append([]string{}, basis.args...), declared...)
	e.steps, e.Says = edited, says
	if refused != "" {
		e.Refused = refused
		return e.sealed(), nil
	}
	// AND THE BYTES THE ACCOUNT WOULD HOLD, settled now. See EditReview.stepsToWrite.
	written, merr := json.Marshal(edited)
	if merr != nil {
		return EditReview{}, fmt.Errorf("%q: the edited steps could not be written "+
			"down, so the edit is not offered: %w", e.Name, merr)
	}
	e.stepsToWrite = written
	now, assets, rerr := d.render(&Draft{Name: basis.name, Args: args,
		Steps: edited, Mode: basis.mode}, scope.mode(), rt.App, rt)
	if rerr != nil {
		// THE GENERATOR'S OWN WORDS, as the rebuild review does with them: a thing
		// to read rather than a thing to translate.
		e.Refused = rerr.Error()
		return e.sealed(), nil
	}
	// ── AND IT IS LEGAL MARCO BEFORE IT CAN BE OFFERED ──
	//
	// `codegen` is trusted everywhere else in this package because its input came from a
	// recorder. After an edit it did not, so the one compile gate is asked — here, where
	// a refusal is a review somebody reads, rather than at the moment of writing.
	if cerr := d.checkSource(now); cerr != nil {
		e.Refused = fmt.Sprintf("the edited routine would not compile, so it cannot "+
			"be saved: %v", cerr)
		return e.sealed(), nil
	}
	e.Now, e.assets = now, assets
	return e.sealed(), nil
}

// basis is the step list a routine's saved source is the render of, and the three things `render`
// needs beside it.
type basis struct {
	steps []macroir.Step
	name  string
	args  []string
	mode  RecordedAs
	// mismatch is what to say when rendering this basis does not reproduce the saved
	// source. Two different sentences, because the two cases are not the same situation.
	mismatch string
}

// basisFor decides what a routine's step list is, or why it cannot be known.
//
// Two answers and four refusals, and the refusals are the point: only an account that still
// describes both files beside it may be rendered from, and a mismatch is described as what it is
// rather than as what it suggests.
func (d Deps) basisFor(rt routes.Route, src string, rec []byte) (basis, string, error) {
	name := NameIn(src, rt.Slug)
	stored, state := d.Reg.Steps(rt)
	switch state {
	case routes.StepsIntact:
		var steps []macroir.Step
		if err := json.Unmarshal(stored.Steps, &steps); err != nil {
			return basis{}, "", fmt.Errorf("%q: its stored steps could not be read "+
				"back, so nothing can be edited from them: %w", name, err)
		}
		if len(steps) == 0 {
			return basis{}, "its stored steps contain nothing that can be " +
				"performed", nil
		}
		base, args := declaredFor(name, steps)
		return basis{steps: steps, name: base, args: args, mode: ModeOf(src),
			mismatch: "its saved source is not what its stored steps render " +
				"to under today's rules, so the two no longer agree — " +
				"rebuilding it from its demonstration is the way to get a " +
				"routine that can be edited"}, "", nil
	case routes.StepsNone:
		// NO ACCOUNT MEANS NO EDIT, which is the one-way inference every routine
		// already on disk depends on: absence may never read as a claim.
		var events []recorder.RecordedEvent
		if err := json.Unmarshal(rec, &events); err != nil {
			return basis{}, "", fmt.Errorf("%q: its demonstration could not be "+
				"read back, so its steps cannot be worked out: %w", name, err)
		}
		dr := d.draftFor(RebuildCurrent, name, events, rt.App, "an edit", ModeOf(src))
		switch {
		case dr == nil || len(dr.Steps) == 0:
			return basis{}, "its demonstration contains nothing that can be " +
				"performed", nil
		case dr.Unrepresentable != "":
			return basis{}, fmt.Sprintf("a version rebuilt from its "+
				"demonstration cannot be saved as it stands, so there is "+
				"nothing to edit against: %s", dr.Unrepresentable), nil
		}
		return basis{steps: dr.Steps, name: dr.Name, args: dr.Args, mode: dr.Mode,
			mismatch: "its saved source is not what its own demonstration " +
				"renders to under today's rules, so what the file does and " +
				"what was demonstrated no longer agree — reviewing a " +
				"rebuild is where that is decided"}, "", nil
	case routes.StepsMismatched:
		// NEUTRAL, AND DELIBERATELY SO. A source digest that moved is consistent
		// with somebody editing the file by hand AND with an account write that
		// failed after the source landed, and nothing on disk tells them apart. A
		// sentence naming either would send whoever reads it to the wrong place.
		return basis{}, "its saved source and its stored steps no longer match, so " +
			"nothing here can say what the file is the render of — reviewing " +
			"a rebuild from its demonstration is where that is decided", nil
	case routes.StepsAdrift:
		return basis{}, "its stored steps are bound to a different demonstration " +
			"than the one beside it now, so they are not an account of this " +
			"routine — reviewing a rebuild is where that is decided", nil
	case routes.StepsOrphaned:
		return basis{}, "its stored steps describe a source that is not there", nil
	default:
		return basis{}, "its stored steps cannot be read, and a file that cannot " +
			"answer is not an absent one — it was written by something this " +
			"build does not understand, so nothing was assumed about it", nil
	}
}

// applyEdits builds the edited step list, the lines describing what was done, and the reason the
// result is not a routine.
//
// # Why the operations are collected before any of them is applied
//
// Because every index in one review is into the list the caller was shown. Splicing as it went
// would move the later positions under the earlier edits, so the two operations somebody asked
// for would not be the two that happened — and silently, because the result is a perfectly good
// routine that does the wrong thing.
// The fourth return is the argument names the edits DECLARED, which the re-render needs.
//
// # Why a render has to be told
//
// `codegen.splitSecrets` keeps a `{{name}}` in the typed literal only when the name is DECLARED
// (or numeric); any other one becomes `do OS's Secret …`. So an edit that writes `{{message}}`
// into a step and renders against the routine's OLD declarations produces a routine that reads
// the credential store — which is exactly what this did before it returned these: the diagnostic
// read `do OS's Secret with "message"` where `do OS's Type with "{{message}}"` belonged.
//
// Returned rather than derived by the caller from the steps, because the caller would then be
// working out what an edit meant a second time, and the two readings would be free to disagree.
func applyEdits(steps []macroir.Step, edits []StepEdit) ([]macroir.Step, []string, string, []string, error) {
	drop := map[int]bool{}
	waits := map[int]int{}
	args := map[int]string{}
	points := map[int]macroir.Step{}
	var says []string
	// ── A DECLARATION THAT A RUN IS TYPED TEXT IS REVIEWED ON ITS OWN ──
	//
	// Because it is the one operation that changes the LENGTH of the list, and every
	// position in this review was read off the list as it stands. Collecting it with the
	// others would mean deciding whether each of them referred to a position before or
	// after the run, which nothing says. So it arrives alone or it is refused, before
	// anything is applied. See EditTyped.
	for _, ed := range edits {
		if ed.Op != EditTyped {
			continue
		}
		if len(edits) != 1 {
			return nil, nil, "", nil, fmt.Errorf("saying which steps are typed " +
				"text changes how many steps there are, so it is the only " +
				"thing one set of changes can do — ask for it on its own, " +
				"and for the rest afterwards")
		}
		text, terr := typedRun(steps, ed.At, ed.Upto)
		if terr != nil {
			return nil, nil, "", nil, terr
		}
		out := make([]macroir.Step, 0, len(steps))
		out = append(out, steps[:ed.At]...)
		out = append(out, macroir.Step{Kind: macroir.StepType, Text: text})
		out = append(out, steps[ed.Upto+1:]...)
		// AND IT DECLARES NO ARGUMENT, which is not an omission: `typedRun` refuses a
		// range holding text the routine already asks for, so a placeholder cannot be
		// inside what this produces. The names the ROUTINE already declares are the
		// caller's to carry, as they are for every other operation here.
		return out, []string{fmt.Sprintf("steps %d to %d become one step that types "+
			"%q, and the waits between them are gone", ed.At+1, ed.Upto+1,
			text)}, "", nil, nil
	}
	for _, ed := range edits {
		if ed.At < 0 || ed.At >= len(steps) {
			return nil, nil, "", nil, fmt.Errorf("there is no step %d in a routine of %d "+
				"steps, so nothing was changed — look at it again",
				ed.At+1, len(steps))
		}
		switch ed.Op {
		case EditWait:
			if steps[ed.At].Kind != macroir.StepWait {
				return nil, nil, "", nil, fmt.Errorf("step %d is not a wait, so it "+
					"has no duration to change", ed.At+1)
			}
			if ed.To < 0 {
				return nil, nil, "", nil, fmt.Errorf("a wait cannot be %dms long",
					ed.To)
			}
			if drop[ed.At] {
				return nil, nil, "", nil, fmt.Errorf("step %d is asked to be both "+
					"changed and removed, so nothing was changed",
					ed.At+1)
			}
			waits[ed.At] = ed.To
			says = append(says, fmt.Sprintf("the wait at step %d becomes %dms "+
				"(it was %dms)", ed.At+1, ed.To, steps[ed.At].Ms))
		case EditRemove:
			if _, changed := waits[ed.At]; changed {
				return nil, nil, "", nil, fmt.Errorf("step %d is asked to be both "+
					"changed and removed, so nothing was changed",
					ed.At+1)
			}
			if _, placed := points[ed.At]; placed {
				return nil, nil, "", nil, fmt.Errorf("step %d is asked to be both "+
					"moved and removed, so nothing was changed",
					ed.At+1)
			}
			if _, supplied := args[ed.At]; supplied {
				return nil, nil, "", nil, fmt.Errorf("step %d is asked to be both "+
					"supplied and removed, so nothing was changed",
					ed.At+1)
			}
			// ── A GESTURE GOES WHOLE OR NOT AT ALL ──
			//
			// `macroir` already makes a drag, a typed run, an anchored click
			// and a folded repetition ONE step each. A held key is the one
			// gesture it does not: a keydown and a later keyup. Removing
			// either half alone leaves a routine that holds a key it never
			// releases, or releases one it never held.
			partner, pwhy := holdPartner(steps, ed.At)
			if pwhy != "" {
				return nil, nil, pwhy, nil, nil
			}
			drop[ed.At] = true
			line := fmt.Sprintf("step %d is removed", ed.At+1)
			if partner >= 0 {
				if waits[partner] != 0 {
					return nil, nil, "", nil, fmt.Errorf("step %d is asked to "+
						"be changed and is the other half of the "+
						"hold at step %d, so nothing was changed",
						partner+1, ed.At+1)
				}
				drop[partner] = true
				line += fmt.Sprintf(", and step %d with it, because they are "+
					"the two halves of holding %q", partner+1,
					steps[ed.At].Key)
			}
			says = append(says, line)
		case EditPoint:
			if drop[ed.At] {
				return nil, nil, "", nil, fmt.Errorf("step %d is asked to be both "+
					"moved and removed, so nothing was changed", ed.At+1)
			}
			if _, already := points[ed.At]; already {
				return nil, nil, "", nil, fmt.Errorf("step %d is asked to be "+
					"moved to two places, so nothing was changed",
					ed.At+1)
			}
			// AND NO CHECK AGAINST A DURATION OR A SUPPLIED NAME. A step is a
			// click or a wait and never both, and a click types nothing — so
			// "moved and given a duration" cannot be asked of one step, and a
			// guard for it would be one that cannot fire. `movedPoint` answers
			// with the true reason: the step does not press anywhere.
			moved, said, perr := movedPoint(steps, ed.At, ed.X, ed.Y)
			if perr != nil {
				return nil, nil, "", nil, perr
			}
			points[ed.At] = moved
			says = append(says, said)
		case EditArgument:
			if drop[ed.At] {
				return nil, nil, "", nil, fmt.Errorf("step %d is asked to be both "+
					"supplied and removed, so nothing was changed",
					ed.At+1)
			}
			// AND NO CHECK AGAINST A DURATION. A step is a Type or a Wait and
			// never both, so "supplied and given a duration" cannot be asked of
			// one step: the kind check below is what answers, with the true
			// reason. A guard for it would be one that cannot fire.
			name, err := suppliedName(steps, ed.At, ed.Name)
			if err != nil {
				return nil, nil, "", nil, err
			}
			// ── AND NOTHING HERE CHECKS WHETHER THIS ROUTINE ALREADY HAS ONE ──
			//
			// `routes.Arguments` reports placeholders first and secrets after and
			// drops a name it has already seen, so a `{{token}}` beside a
			// `do OS's Secret …:token` would make the SECRET vanish from the list
			// and offer somebody a text box for their password. That is a real
			// defect and it cannot happen here: `macroir` has no step kind for a
			// secret, so a routine this editor will touch — one whose source is
			// exactly what its own step list renders — contains none. The name
			// check above is about the NAME and needs no routine to look at.
			//
			// A guard for it would be a guard that cannot fire, which reads to the
			// next person exactly like an enforced rule. Two of those were written
			// and removed in the slice this feature serves. If the IR ever gains a
			// secret step, this is the paragraph that says what to add.
			args[ed.At] = name
			says = append(says, fmt.Sprintf("the text at step %d becomes "+
				"something you supply, named %q (it was %q)", ed.At+1, name,
				steps[ed.At].Text))
		default:
			return nil, nil, "", nil, fmt.Errorf("%q is not something an edit can do",
				ed.Op)
		}
	}
	out := make([]macroir.Step, 0, len(steps))
	for i, s := range steps {
		if drop[i] {
			continue
		}
		if ms, ok := waits[i]; ok {
			s.Ms = ms
		}
		// THE WHOLE STEP, because a moved press is the step it was with four
		// numbers changed — `movedPoint` copied it, so the button, the modifiers,
		// the count and every guard term are already on it. Assigning the
		// coordinates here instead would be a second place that decides which of
		// them a move may touch.
		if moved, ok := points[i]; ok {
			s = moved
		}
		if name, ok := args[i]; ok {
			s.Text = "{{" + name + "}}"
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		// THE EMPTY ROUTINE IS ITS OWN ANSWER. A routine that does nothing is not a
		// smaller routine, and deleting one is an operation that already exists and
		// already sweeps what sits beside it. An edit that quietly amounted to a
		// delete would be offering that without any of it.
		return nil, says, "removing that would leave the routine with nothing to do. " +
			"Deleting the routine is how to get rid of it", nil, nil
	}
	// THE NAMES THIS EDIT DECLARED, so a render keeps them in the typed literal instead of
	// reaching for the credential store.
	//
	// WALKED OVER THE ORIGINAL LIST, because that is what `args` is keyed by. Walking the
	// edited one would read the right names only while nothing was removed: a removal
	// earlier in the same review shifts every later position, so the lookup would land on
	// a different step — or on none, losing a declaration and turning the placeholder into
	// a credential-store read. It is the same reason every operation here is collected
	// against original positions in the first place.
	declared := make([]string, 0, len(args))
	for i := range steps {
		if name, ok := args[i]; ok {
			declared = append(declared, name)
		}
	}
	return out, says, "", declared, nil
}

// declaredFor is a routine's name without its arguments, and every argument a render of these
// STORED STEPS must declare.
//
// # One owner, because three renders need it and two of them got it wrong
//
// Rendering a stored step list means handing `codegen` a [Draft], and `codegen` keeps a
// `{{name}}` inside a typed literal only when the name is DECLARED — anything else becomes
// `do OS's Secret …`, a read of somebody's credential store. The declarations used to come from
// the routine's NAME alone (`say hello with message`), which was enough while the learn phrase was
// the only way a placeholder could arrive.
//
// [EditArgument] is a second way, and it broke BOTH other renders:
//
//   - the step panel's basis check, so the second edit of an edited routine refused — the
//     re-render did not reproduce the saved source, because it rendered a credential read; and
//   - the REBUILD review's "today's rules" arm, which rendered `do OS's Secret with "message"` as
//     the version it offered to replace the routine with. Found by the regression test the
//     Architect asked for, after the first two had been fixed one at a time.
//
// So the question is answered once, here, and a fourth render can ask it instead of rediscovering
// it. The name's own arguments come first and keep their order, because positional `{{N}}`
// placeholders are named from that order.
func declaredFor(name string, steps []macroir.Step) (string, []string) {
	base, args := routes.SplitArgs(name)
	return base, append(args, suppliedIn(steps, args)...)
}

// TypedRun is one range of a step list that could be declared typed text, and what it would say.
//
// The range is INCLUSIVE at both ends and counted from zero, which is how every other operation
// in this package counts and how both surfaces list steps. `Text` is what the range would become,
// so a surface offering it can name the result rather than the positions — "steps 3 to 11 type
// \"stone\"" is a proposal somebody can check, and "steps 3 to 11" is a number they have to trust.
type TypedRun struct {
	From int
	To   int
	Text string
}

// TypedRunsIn is every maximal range of these steps that could be declared typed text.
//
// # Why the orchestrator proposes them and not the window
//
// Because `typedRun` is the rules, and a surface that worked out which runs were eligible would be
// a second reading of them — free to offer a range the validator then refuses, which is a control
// whose refusal arrives after somebody presses it. Every proposal here is validated by the same
// function that applies it, so an offer is a promise.
//
// # Maximal, and what that costs
//
// A run is extended as far as the vocabulary allows and then its leading and trailing waits are
// trimmed, because a `type` step cannot hold them and a run that began or ended with one would
// silently swallow a pause that belongs to the steps around it. Two words separated by a long
// pause are therefore proposed as ONE run, which is greedier than somebody may want. That is a
// known limit rather than a judgement: splitting them would need a threshold on what counts as a
// gap between words, and this whole feature exists because thresholds cannot tell typing from a
// game. The saved routine's editor takes an explicit range, which is where a narrower selection
// is made.
//
// A run whose text is a single character is not proposed. It would be a `key` step becoming a
// `type` step and nothing else, and the chooser is for turning a rollover-shredded word back into
// a word. `typedRun` still allows it, because what may be ASKED FOR explicitly is a different
// question from what is worth offering.
func TypedRunsIn(steps []macroir.Step) []TypedRun {
	var out []TypedRun
	for i := 0; i < len(steps); {
		if !couldBeTyping(steps[i]) {
			i++
			continue
		}
		j := i
		for j+1 < len(steps) && couldBeTyping(steps[j+1]) {
			j++
		}
		next := j + 1
		// TRIMMED OF ITS OUTER WAITS, which belong to whatever is on either side.
		from, to := i, j
		for from <= to && steps[from].Kind == macroir.StepWait {
			from++
		}
		for to >= from && steps[to].Kind == macroir.StepWait {
			to--
		}
		if from <= to {
			if text, err := typedRun(steps, from, to); err == nil &&
				len([]rune(text)) > 1 {
				out = append(out, TypedRun{From: from, To: to, Text: text})
			}
		}
		i = next
	}
	return out
}

// couldBeTyping is whether a step is in the vocabulary a typed run can be made of.
//
// THE SHAPE ONLY, and not whether the run is valid: balance, what is held across it and what each
// key produces are `typedRun`'s, asked of the whole range. This is what decides where a candidate
// range ENDS, so it has to answer for one step with nothing around it.
func couldBeTyping(s macroir.Step) bool {
	switch s.Kind {
	case macroir.StepWait, macroir.StepKeyDown, macroir.StepKeyUp:
		return true
	case macroir.StepKey:
		return s.Text != ""
	case macroir.StepType:
		return routes.Arguments(s.Text) == nil
	}
	return false
}

// typedRun is the text a range of steps would become when somebody declares it typed, or why it
// cannot be declared.
//
// ONE VALIDATOR, BECAUSE THERE ARE TWO WAYS IN. [EditTyped] asks it of a SAVED routine's list and
// [Deps.TypedInDraft] asks it of a recording nobody has kept yet, which is the reason
// `suppliedName` below is one function too. The saved routine matters here and is not a
// convenience: the recording that exposed this defect is already on disk, and a repair reachable
// only from the review would mean re-recording it.
//
// # What this is for, and what it deliberately does not know
//
// A key is recorded as a HOLD when another key goes down before it comes up, and that is what
// typing rollover IS: `s` down, `t` down, `s` up. Measured on the owner's own recording — reduced
// below in the fixtures — the presses are 93–149ms and overlap by 12–56ms, so the word `stone`
// was saved as four KeyDown/KeyUp pairs and one tap, and neither typing-fold pass can touch it
// because a hold carries no text.
//
// THE CLASSIFIER IS NOT WRONG AND IS NOT CHANGED. `s↓ t↓ s↑ t↑` and a brief two-key gameplay
// combination `q↓ e↓ q↑ e↑` are the same shape, and nothing in the events says which one somebody
// meant — not the key names, because Q and E are printable, and not the durations, because a
// person can type slowly and press a game key briefly. So this does not guess. A PERSON SAYS, by
// selecting the steps, and what follows is an EDIT to the list rather than a different reading of
// the demonstration. `markHolds` is untouched, an ordinary save of that same recording still
// produces the holds, and so does a rebuild of a routine nobody has edited.
//
// # The order is the order the keys went DOWN
//
// Which is the one thing rollover makes non-obvious: the steps read
// `s↓ t↓ s↑ o↓ t↑ n↓ o↑ e n↑`, and the word is `stone` only if each character is taken at its
// PRESS. Taking them in step order would read `stsotnoen`.
//
// # What it refuses, and why each one is a refusal rather than a best effort
//
//   - A key that is not one printable character — `esc`, `tab`, an f-key, a modifier on its own.
//     Somebody has selected a chord or a game key, and typing its name is not what they asked
//     for.
//   - An unbalanced hold inside the range: a `keydown` whose `keyup` is outside it, or the other
//     way round. Replacing it would leave a routine holding a key it never releases.
//   - A key held ACROSS the whole range, from before it to after it. The range looks clean from
//     inside — both halves of that hold are outside and untouched — and converting it would still
//     change that gesture, because the conversion removes the WAITS inside the range and the hold
//     is as long as what it spans. A 600ms charge-hold would quietly become as long as typing two
//     letters. Mouse buttons cannot be in this state: `macroir` has no step for a button held
//     across other steps — a click is one step and a drag is one step — so there is nothing here
//     to check for them, which is why this says so rather than pretending to.
//   - Anything else in the range: a click, an activate, a loop, a drag. A loop could in principle
//     be expanded and is refused on purpose, because a representation this cannot reproduce
//     exactly must not be flattened into an approximation of itself.
//   - Text that is already supplied. A `{{name}}` swallowed into a literal would make the routine
//     type the braces.
//
// # And a repeat is kept, exactly
//
// `macroir` folds a key pressed several times into one step with a `Count`, and a declaration that
// dropped it would turn `aaa` into `a` — the quietest possible way to change what a routine types.
// A repeated character is legitimate here and the "two distinct characters" rule is NOT applied:
// that rule belongs to automatic coalescing, where it exists to keep game key-spam out of typed
// text, and it has no business overruling somebody who has said what the steps are. A `Count` on
// anything this cannot expand that way is refused rather than ignored.
func typedRun(steps []macroir.Step, from, to int) (string, error) {
	switch {
	case from < 0 || from >= len(steps) || to < 0 || to >= len(steps):
		return "", fmt.Errorf("there is no range of steps %d to %d in a routine of %d "+
			"steps", from+1, to+1, len(steps))
	case to < from:
		return "", fmt.Errorf("steps %d to %d is not a range", from+1, to+1)
	}
	// ── NOTHING MAY BE HELD WHEN THE RANGE BEGINS ──
	//
	// Read off the steps BEFORE it, which is the only place it can be seen: a key held
	// across the whole selection appears nowhere inside it. See the note above for why
	// this is a refusal.
	held := map[string]int{}
	for i := 0; i < from; i++ {
		switch steps[i].Kind {
		case macroir.StepKeyDown:
			held[steps[i].Key] = i
		case macroir.StepKeyUp:
			delete(held, steps[i].Key)
		}
	}
	for k, at := range held {
		return "", fmt.Errorf("%q is still held from step %d, across the steps you "+
			"picked — converting them would shorten that hold by the waits inside "+
			"it, so the gesture it is part of has to be dealt with first", k, at+1)
	}
	var b strings.Builder
	open := map[string]bool{}
	for i := from; i <= to; i++ {
		s := steps[i]
		switch s.Kind {
		case macroir.StepWait:
			// DROPPED, AND SAID SO ON THE CONTROL. The rhythm of somebody's
			// typing is not something a `type` step can hold, and the caption
			// names it before anybody presses. Waits OUTSIDE the range are
			// untouched.
		case macroir.StepType:
			if routes.Arguments(s.Text) != nil {
				return "", fmt.Errorf("step %d is already something the routine "+
					"asks for, so it cannot be folded into typed text", i+1)
			}
			b.WriteString(s.Text)
		case macroir.StepKey:
			c, err := oneCharacter(s, i)
			if err != nil {
				return "", err
			}
			n := s.Count
			if n < 1 {
				n = 1
			}
			b.WriteString(strings.Repeat(c, n))
		case macroir.StepKeyDown:
			c, err := oneCharacter(s, i)
			if err != nil {
				return "", err
			}
			if s.Count > 1 {
				return "", fmt.Errorf("step %d holds %q %d times, which is not "+
					"something typed text can say", i+1, s.Key, s.Count)
			}
			// THE CHARACTER IS TAKEN AT THE PRESS. See the note above.
			open[s.Key] = true
			b.WriteString(c)
		case macroir.StepKeyUp:
			if !open[s.Key] {
				return "", fmt.Errorf("step %d releases %q, which was not held "+
					"inside the steps you picked — a held key goes in whole "+
					"or stays out", i+1, s.Key)
			}
			delete(open, s.Key)
		default:
			return "", fmt.Errorf("step %d is not typing — typed text can only be "+
				"made out of key presses, held keys and the waits between "+
				"them", i+1)
		}
	}
	for k := range open {
		return "", fmt.Errorf("%q is held inside the steps you picked and released "+
			"after them — a held key goes in whole or stays out", k)
	}
	if b.Len() == 0 {
		return "", fmt.Errorf("steps %d to %d type nothing", from+1, to+1)
	}
	return b.String(), nil
}

// oneCharacter is the character a key step types, or why that step is not typing.
//
// # Two sources, and the order matters
//
// `Text` first, because it is what the recorder read off the keystroke and the only thing that
// knows what the key PRODUCED: `shift`+`t` is `T` and the key is still `t`. A tap carries it.
//
// `Key` second, for a HELD key, because a held key carries no text at all — which is the same fact
// both typing-fold passes trip on, and the reason this operation has to exist. A key name that is
// itself one character is that character, and the standard being met is NOT "what the person's
// fingers did" but "what this step types when the routine runs": `do OS's KeyDown with "s"`
// followed by its `KeyUp` types a lowercase `s`, and so does `do OS's Type with "s"`. The
// conversion is faithful to the routine.
//
// WHICH MEANS A CAPITAL TYPED WITH ROLLOVER COMES OUT LOWER CASE, and that is already true before
// anything here runs. Measured: `shift` held across `s` and `t` typed with overlap renders as
// `keydown "s"` — no text, no shift — beside `key "t"` carrying `"T"`, because `markHolds` leaves a
// modifier out of a hold and only the tap reaches the chord fold. So the `S` was lost when the
// recording was written down, not when it was declared. That is an adjacent defect in how a held
// key is REPRESENTED and it is not this slice's; naming it here is what stops the next reader
// concluding this function introduced it.
//
// A key that is neither — `esc`, `tab`, `f5`, a modifier on its own, a multi-character text — is
// not typing, and a representation this build does not produce is refused rather than guessed at.
func oneCharacter(s macroir.Step, at int) (string, error) {
	if s.Text != "" {
		if len([]rune(s.Text)) != 1 {
			return "", fmt.Errorf("step %d types %q, which is not one character",
				at+1, s.Text)
		}
		return s.Text, nil
	}
	if len([]rune(s.Key)) == 1 {
		return s.Key, nil
	}
	return "", fmt.Errorf("step %d presses %q, which types nothing — typed text cannot "+
		"be made out of it", at+1, s.Key)
}

// suppliedName is the name the text at `at` would be supplied under, or why it cannot be.
//
// ONE SET OF RULES, BECAUSE THERE ARE TWO WAYS IN NOW. [EditArgument] asks it of a SAVED
// routine's step list, and [Deps.SuppliedInDraft] asks it of a recording nobody has kept yet —
// and the two are the same question about the same kind of step. A second copy of these four
// checks is a second place for the credential refusal to be missing from, which is the one of
// them that quietly changes what a routine does.
//
// The sentences name the step by its position from one, because that is how both surfaces list
// them and an index nobody can see is not an explanation.
func suppliedName(steps []macroir.Step, at int, raw string) (string, error) {
	switch {
	case at < 0 || at >= len(steps):
		return "", fmt.Errorf("there is no step %d to supply", at+1)
	case steps[at].Kind != macroir.StepType:
		return "", fmt.Errorf("step %d does not type any text, so there is nothing "+
			"about it to supply", at+1)
	// ALREADY SUPPLIED IS NOT A CHANGE. A second pass over the same step would bury one
	// placeholder inside another and the routine would type the braces.
	case routes.Arguments(steps[at].Text) != nil:
		return "", fmt.Errorf("the text at step %d is already supplied when the "+
			"routine runs", at+1)
	}
	name := strings.ToLower(strings.TrimSpace(raw))
	if !argName.MatchString(name) {
		return "", fmt.Errorf("%q is not a name a supplied value can have — letters, "+
			"digits, dashes and underscores, and at least one of them", raw)
	}
	// ── AND NOT A NAME THE GENERATOR READS FROM THE CREDENTIAL STORE ──
	//
	// `codegen` resolves a placeholder called `password`, `token`, `pin`, `key` and a handful
	// of others from the CREDENTIAL STORE rather than typing it — `codegen.MeansASecret` is
	// the list. So naming the changing text one of those would not produce supplied text at
	// all: the routine would read whatever is in the store and type that.
	//
	// Somebody asking for a value they supply must not get a credential lookup because of the
	// word they chose, and they cannot be expected to know the list. So it is refused, named,
	// and said plainly.
	if codegen.MeansASecret(name) {
		return "", fmt.Errorf("a supplied value cannot be called %q: the generator reads "+
			"that name from the credential store instead of typing it, so the routine "+
			"would type whatever is saved there — pick another name", name)
	}
	return name, nil
}

// suppliedIn is the placeholder names a step list already types, that the name has not declared.
//
// # Why a placeholder in the steps IS a declaration
//
// `codegen` keeps a `{{name}}` inside a typed literal only when the name is DECLARED, and turns
// any other one into a credential-store read. The declarations have always come from the learn
// phrase — `say hello with message` — so a routine whose placeholder arrived any OTHER way reads
// back as a routine that types a password.
//
// That was fine while the only way in was the phrase. [EditArgument] is a second way in, and
// without this the second edit of an edited routine REFUSES: the basis is rendered to check that
// it reproduces the saved source, and rendering `{{message}}` with nothing declaring it produces
// `do OS's Secret with "message"`, which is not what is on disk. Found by a test that made the
// same edit twice.
//
// So the step list declares what it holds. Nothing new is stored and no format changed: the
// placeholder was always the fact, and this is the reading of it.
//
// # Except a name the generator means as a secret
//
// `codegen.MeansASecret` — `password`, `token`, `pin` and the rest. For those, declaring changes
// what is RENDERED (a global secret becomes a route-qualified one), so a routine on disk with one
// of those typed in it would stop matching its own source and become uneditable. [EditArgument]
// refuses those names outright, so this cannot be the only thing standing between somebody and
// that surprise — and leaving them undeclared keeps every routine already saved rendering exactly
// as it does today.
func suppliedIn(steps []macroir.Step, declared []string) []string {
	has := map[string]bool{}
	for _, a := range declared {
		has[strings.ToLower(a)] = true
	}
	var out []string
	for _, s := range steps {
		if s.Kind != macroir.StepType {
			continue
		}
		for _, a := range routes.Arguments(s.Text) {
			name := strings.ToLower(a.Name)
			if has[name] || codegen.MeansASecret(name) {
				continue
			}
			has[name] = true
			out = append(out, name)
		}
	}
	return out
}

// holdPartner is the other half of a held key, -1 when the step is not half of one, and the
// reason a half cannot be paired.
//
// A pair it cannot resolve is REFUSED, never guessed: the alternative is removing one half of a
// hold, and a routine that holds a key it never releases is a routine that has changed what every
// press after it means.
func holdPartner(steps []macroir.Step, at int) (int, string) {
	switch steps[at].Kind {
	case macroir.StepKeyDown:
		for i := at + 1; i < len(steps); i++ {
			if steps[i].Kind == macroir.StepKeyUp && steps[i].Key == steps[at].Key {
				return i, ""
			}
		}
		return -1, fmt.Sprintf("step %d holds %q and nothing in this routine releases "+
			"it, so removing it is not something this can work out — rebuilding "+
			"the routine from its demonstration is where that is decided",
			at+1, steps[at].Key)
	case macroir.StepKeyUp:
		for i := at - 1; i >= 0; i-- {
			if steps[i].Kind == macroir.StepKeyDown && steps[i].Key == steps[at].Key {
				return i, ""
			}
		}
		return -1, fmt.Sprintf("step %d releases %q and nothing in this routine holds "+
			"it, so removing it is not something this can work out — rebuilding "+
			"the routine from its demonstration is where that is decided",
			at+1, steps[at].Key)
	}
	return -1, ""
}

// SaveEdit writes the reviewed source over the routine it was prepared from, and the edited step
// list beside it.
//
// # It refuses unless the world is still the one that was reviewed
//
// All three files this was built from are re-read and re-digested under the route's lock, taken
// BEFORE the first read because what has to be atomic is the comparison and the writes together.
//
// # The order, and what each failure leaves
//
//	ASSETS   first, and only the missing ones — see imagesToWrite for why a path holding a
//	         different picture is a refusal with nothing written.
//	SOURCE   atomically. The executable truth, and the only file a run reads.
//	ACCOUNT  last, because the digest it records is of the source it describes and there is
//	         nothing to record until that source is on disk.
//
// A failure writing the account therefore leaves the edit in force with an account that no longer
// matches it. The routine runs exactly as the new source says; what is blocked is FURTHER
// EDITING, and it is reported rather than discovered.
func (d Deps) SaveEdit(e EditReview) error {
	switch {
	case e.Route.Slug == "":
		return fmt.Errorf("an edit needs the routine it was prepared from")
	case e.library != "" && e.library != d.Reg.Dir:
		return fmt.Errorf("%q was edited in the library at %s and this is %s, so "+
			"nothing was written — a routine of that name here is a different "+
			"routine", e.Name, e.library, d.Reg.Dir)
	case e.Token == "" || e.Token != e.sealed().Token:
		return fmt.Errorf("the edit offered for %q is not the one that was reviewed, "+
			"so nothing was written — review it again", e.Name)
	case e.Refused != "":
		return fmt.Errorf("%q cannot be saved with that edit: %s", e.Name, e.Refused)
	case !e.Savable():
		return fmt.Errorf("%q has no edit to save", e.Name)
	}
	hold, lerr := d.Reg.Lock(e.Route, SaveWithin)
	if lerr != nil {
		return fmt.Errorf("%q is left as it is: %w", e.Name, lerr)
	}
	defer hold.Release()

	raw, err := os.ReadFile(d.Reg.Path(e.Route))
	if err != nil {
		return fmt.Errorf("%q could not be read back, so it is left as it is: %w",
			e.Name, err)
	}
	if routes.DigestOf(string(raw)) != e.sourceAt {
		return fmt.Errorf("%q has been saved again since the edit you reviewed was "+
			"prepared, so it is left as it is — review it again and the "+
			"comparison will be against the routine as it now stands", e.Name)
	}
	rec, ok := d.Reg.LoadRecording(e.Route)
	if !ok {
		return fmt.Errorf("%q no longer has the demonstration its steps are an "+
			"account of, so it is left as it is", e.Name)
	}
	if routes.DigestOf(string(rec)) != e.recordingAt {
		return fmt.Errorf("%q has been recorded again since the edit you reviewed was "+
			"prepared, so it is left as it is — review it again and the "+
			"comparison will be against the new recording", e.Name)
	}
	if at, readable := d.Reg.StepsDigest(e.Route); !readable || at != e.stepsAt {
		// SOMETHING ELSE WROTE THIS ROUTINE'S STEP LIST. Writing anyway would undo
		// it invisibly: every other check passes and the result looks exactly like
		// an ordinary successful save. An unreadable file counts as moved, because
		// a save may not be made against a file nothing has read.
		return fmt.Errorf("the step list stored beside %q has changed since the edit "+
			"you reviewed was prepared, so it is left as it is — review it "+
			"again and the comparison will be against the steps as they now "+
			"stand", e.Name)
	}
	// THE ONE INSTANT A TEST CAN STAND IN: after the revalidation and before anything is
	// written, inside the lock. Nil in every production path.
	if d.AfterValidating != nil {
		d.AfterValidating()
	}
	images, why := imagesToWrite(e.assets)
	if why != "" {
		return fmt.Errorf("%q is left as it is: %s", e.Name, why)
	}
	if err := d.Reg.CreateAssets(images); err != nil {
		return fmt.Errorf("%q is left as it is: the images the edited version needs "+
			"could not be written: %w", e.Name, err)
	}
	if err := d.Reg.Replace(e.Route, e.Now); err != nil {
		return fmt.Errorf("%q is left as it is: %w", e.Name, err)
	}
	if err := d.Reg.SaveSteps(e.Route, routes.StoredSteps{
		Source: routes.DigestOf(e.Now), Demonstration: e.recordingAt,
		Steps: e.stepsToWrite}); err != nil {
		// THE SOURCE IS IN FORCE AND THE ACCOUNT IS NOT. Said rather than
		// swallowed, in the shape `SaveWithOrigin` already uses for exactly this:
		// the edit happened, the routine runs, and the next edit will refuse
		// because the file and the stored steps no longer match.
		return fmt.Errorf("%q was saved with the edit and %w, so it cannot be "+
			"edited again until it has been rebuilt from its demonstration "+
			"(%v)", e.Name, ErrStepsNotWritten, err)
	}
	mlog.Info("edit: saved from its own step list", "route", e.Route.Slug,
		"edits", len(e.Edits), "steps", len(e.steps),
		"scope", string(ScopeAlreadyOf(e.Route)))
	return nil
}

// sealed is this review with its token, which is the last thing done to it.
//
// Every exit goes through here, refusals included: a refused review is still shown on a screen
// and still has to be told apart from another refused review of the same routine.
//
// The three digests, what would be written, why nothing would be, the operations asked for, and
// the whole identity of the target including the library — the same four questions a rebuild's
// token answers, plus the account. See Prepared.sealed for why the target is in it.
func (e EditReview) sealed() EditReview {
	parts := []string{e.sourceAt, e.recordingAt, e.stepsAt, routes.DigestOf(e.Now),
		// AND THE SOURCE IT SHOWED, which is a SEPARATE thing from the digest
		// above it.
		//
		// `sourceAt` is a reading of the file taken at preparation and compared
		// against the file at the write: it answers "is the routine on disk still
		// the one this was built from". It does NOT answer "is this review still
		// showing what it showed", because it is a stored value and the field it
		// describes is exported and mutable. A caller that rewrote `Was` left the
		// token intact, the write went ahead, and what was displayed as the file
		// as it stands was something nobody read off disk.
		//
		// Digesting the field itself closes that, because `sealed` recomputes this
		// from whatever `Was` now holds. The two are equal by construction and
		// diverge exactly when somebody has changed what the review says it
		// showed. WHAT WAS DISPLAYED and WHAT CURRENTLY EXISTS are two questions
		// and both have to match the preparation.
		routes.DigestOf(e.Was),
		// AND THE BYTES THAT WOULD LAND BESIDE THE ROUTINE. The source is
		// covered by `Now` and the account is a second file this would write,
		// so it is a second thing the review has to be about.
		routes.DigestOf(string(e.stepsToWrite)),
		e.Refused, e.Route.Slug, e.Route.App, fmt.Sprint(e.Route.Focus), e.library}
	for _, ed := range e.Edits {
		parts = append(parts, string(ed.Op), fmt.Sprint(ed.At), fmt.Sprint(ed.To))
	}
	e.Token = routes.DigestOf(strings.Join(parts, "\x00"))
	return e
}

// checkSource asks the one compile gate, through the seam if a caller supplied one.
//
// The default is the real gate, so nothing has to be wired for an edit to be checked. See
// Deps.CheckSource, and [[ADR-005-legal-marco-only]] for why there is exactly one of these.
func (d Deps) checkSource(src string) error {
	if d.CheckSource != nil {
		return d.CheckSource(src)
	}
	return driver.CheckSource(src)
}
