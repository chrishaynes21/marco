package orchestrator

// EDITING A STEP: THE STEP LIST IS WHAT CHANGES, AND THE SOURCE IS RE-RENDERED FROM IT.
//
// Two operations — set a wait's duration, remove a step — and the same four-step contract every
// other writer in this package uses:
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
	"fmt"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// EditOp is one of the two things an edit can do. There are two on purpose; see the note above.
type EditOp string

const (
	// EditWait sets a wait's duration, in milliseconds.
	EditWait EditOp = "wait"
	// EditRemove removes one step, whole.
	EditRemove EditOp = "remove"
)

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

// PrepareEdit works out what a routine's step list is, applies the operations to it, and renders
// the result. IT WRITES NOTHING.
//
// Called with no operations it is the question "may this be edited, and what are its steps?" —
// answered through the same path that would perform one, so a surface cannot be told yes by one
// set of rules and refused by another.
func (d Deps) PrepareEdit(rt routes.Route, edits []StepEdit) (EditReview, error) {
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
	edited, says, refused, aerr := applyEdits(basis.steps, edits)
	if aerr != nil {
		return EditReview{}, aerr
	}
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
	now, assets, rerr := d.render(&Draft{Name: basis.name, Args: basis.args,
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
	base, args := routes.SplitArgs(name)
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
func applyEdits(steps []macroir.Step, edits []StepEdit) ([]macroir.Step, []string, string, error) {
	drop := map[int]bool{}
	waits := map[int]int{}
	var says []string
	for _, ed := range edits {
		if ed.At < 0 || ed.At >= len(steps) {
			return nil, nil, "", fmt.Errorf("there is no step %d in a routine of %d "+
				"steps, so nothing was changed — look at it again",
				ed.At+1, len(steps))
		}
		switch ed.Op {
		case EditWait:
			if steps[ed.At].Kind != macroir.StepWait {
				return nil, nil, "", fmt.Errorf("step %d is not a wait, so it "+
					"has no duration to change", ed.At+1)
			}
			if ed.To < 0 {
				return nil, nil, "", fmt.Errorf("a wait cannot be %dms long",
					ed.To)
			}
			if drop[ed.At] {
				return nil, nil, "", fmt.Errorf("step %d is asked to be both "+
					"changed and removed, so nothing was changed",
					ed.At+1)
			}
			waits[ed.At] = ed.To
			says = append(says, fmt.Sprintf("the wait at step %d becomes %dms "+
				"(it was %dms)", ed.At+1, ed.To, steps[ed.At].Ms))
		case EditRemove:
			if _, changed := waits[ed.At]; changed {
				return nil, nil, "", fmt.Errorf("step %d is asked to be both "+
					"changed and removed, so nothing was changed",
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
				return nil, nil, pwhy, nil
			}
			drop[ed.At] = true
			line := fmt.Sprintf("step %d is removed", ed.At+1)
			if partner >= 0 {
				if waits[partner] != 0 {
					return nil, nil, "", fmt.Errorf("step %d is asked to "+
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
		default:
			return nil, nil, "", fmt.Errorf("%q is not something an edit can do",
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
		out = append(out, s)
	}
	if len(out) == 0 {
		// THE EMPTY ROUTINE IS ITS OWN ANSWER. A routine that does nothing is not a
		// smaller routine, and deleting one is an operation that already exists and
		// already sweeps what sits beside it. An edit that quietly amounted to a
		// delete would be offering that without any of it.
		return nil, says, "removing that would leave the routine with nothing to do. " +
			"Deleting the routine is how to get rid of it", nil
	}
	return out, says, "", nil
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
		return fmt.Errorf("%q was saved with your edit but its steps were not "+
			"written down beside it, so it cannot be edited again until it has "+
			"been rebuilt from its demonstration: %w", e.Name, err)
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
