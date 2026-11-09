package main

// CHANGING A STEP, FROM THE PANEL THAT SHOWS THE ROUTINE.
//
// Two operations and no more: set a wait's duration, remove a step. The whole contract is in
// `internal/orchestrator`'s `PrepareEditOf` and `SaveEdit` — the basis the operations count
// against, the step list the saved source is the render of, the whole-gesture removal, the compile
// gate, the route's lock, the three digests re-read, and the account written last — and what is
// here is this window's interaction with it, in four steps of which the first three write nothing:
//
//	the panel lists the steps, each with what can be done to it
//	Review applies the operations to THAT list and shows both sources and the result
//	Cancel drops it — the absence of an operation, because nothing was written
//	Save writes through the version-bound path, and says which of three things happened
//
// # The positions refer to the list that was displayed, and that is held to
//
// A panel lists steps at one moment and sends row numbers at another. Between the two anything can
// write the routine — a hand edit, a re-recording, another window's edit — and applying row 5 to a
// freshly loaded list applies somebody's intention to a routine they never saw. The result is a
// perfectly good routine that does the wrong thing, and no check afterwards can catch it, because
// by then the operations have been applied to the wrong steps.
//
// So the backend mints a BASIS for the list it hands over, the page sends it back with the
// operations, and a routine written since is a refusal with the list reloaded. This window never
// states operations without one; `ReviewStepEdit` refuses an empty basis rather than falling
// through to the unchecked call, because the thing that knows a list was on a screen is this.
//
// # Why the prepared edit is held here and the page gets a token
//
// The same reason `generation.go` holds a review and `rescope.go` holds a move: the promise is
// that what gets written is what was on the screen, and that needs the three digests the review
// was bound to. A page that handed the rendered source back could hand back something else, and a
// check against whatever arrived is not a check.
//
// # Three outcomes at the write, and one of them is neither yes nor no
//
// The source lands before the account, because the account records a digest of the source. So a
// failure at the last of the three writes leaves the routine running the edit with its step list
// no longer describing it — further editing refused, execution untouched. Reported as what it is:
// `orchestrator.ErrStepsNotWritten`, said as a problem rather than as a success or a refusal.
//
// See [[ADR-273-a-routine-writes-down-what-it-does]].

import (
	"errors"
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// StepRow is one step as the panel offers it.
//
// The words are the backend's — `oneStep`, the same function the recording preview uses — because
// two phrasings of one step is how somebody comes to believe they are looking at two routines.
type StepRow struct {
	// At is the position, from zero, and it is what an operation names. Shown as At+1.
	At int `json:"at"`
	// Says is the step in words, one line.
	Says string `json:"says"`
	// Wait says this step is a wait and Ms how long it is now, which is what the duration
	// control is for. Every other step offers only removal.
	Wait bool `json:"wait,omitempty"`
	Ms   int  `json:"ms,omitempty"`
	// Pairs says removing this step takes another with it, PairsAt is that step's
	// position, and PairSays the sentence.
	//
	// SHOWN BESIDE THE ROW, BEFORE ANYTHING IS PRESSED. A held key is a keydown and a
	// later keyup, and removing one half leaves a routine that holds a key it never
	// releases — so the removal takes both. That is a removal of two steps, and somebody
	// pressing ✕ on one row is owed the other before they press it rather than in the
	// review afterwards.
	Pairs    bool   `json:"pairs,omitempty"`
	PairsAt  int    `json:"pairs_at,omitempty"`
	PairSays string `json:"pair_says,omitempty"`
	// Types says this step types TEXT, which is what can be made into something
	// supplied, and Text is the text it types now.
	//
	// THE PANEL OFFERS THE CONTROL ONLY WHERE IT APPLIES. A box beside a click or a key
	// press would be a control that refuses, and the refusal would arrive after somebody
	// had typed a name into it.
	Types bool   `json:"types,omitempty"`
	Text  string `json:"text,omitempty"`
	// Supplied says the text is ALREADY something supplied when the routine runs, so
	// there is nothing to ask for, and Named is what it is called.
	//
	// Shown rather than hidden: a step that already takes its text from the person is the
	// answer to "can this change", and a row that simply offered the control again would
	// invite an edit that is refused.
	Supplied bool   `json:"supplied,omitempty"`
	Named    string `json:"named,omitempty"`
	// Point says this step presses somewhere, Movable whether its position can be
	// changed, and PointWhy what stops it when it cannot — shown ON THE ROW, because a
	// row with no box and no reason is a capability that looks absent.
	//
	// InWindow says which pair is offered: the offset INSIDE the window, or the point on
	// the screen. The two survive different things and a person needs to know which one
	// they are typing into — an offset still means the same place after the window has
	// moved, and a screen point does not.
	//
	// WHICH IS NEVER THE PAGE'S CHOICE. It comes from the same reading the validator
	// applies against, so a surface cannot offer a window to a press that was never
	// measured in one. See orchestrator.PointIn.
	Point    bool   `json:"point,omitempty"`
	Movable  bool   `json:"movable,omitempty"`
	PointWhy string `json:"point_why,omitempty"`
	InWindow bool   `json:"in_window,omitempty"`
	// X and Y are the pair, in the basis InWindow names. NOT omitempty: a press at the
	// very corner of a window is at 0, and a field that vanished would reach the page as
	// nothing and be drawn as an empty box.
	//
	// AND NOTHING ELSE. There was a `Terms` beside them — the button — which the row
	// drew after the numbers, while the same button was already at the front of the
	// row in the action column, where `Says` puts it. See orchestrator.MovablePoint.
	X int `json:"x"`
	Y int `json:"y"`
	// Keyed says this step presses, holds or releases a key, Key which one now, and Holds
	// that it is half of a held key, which may be a combo (`shift+w`, ADR-340).
	//
	// Changing either half of a hold changes both: the backend does it and the review says
	// so, and the row already says the halves go together (`PairSays`).
	Keyed bool   `json:"keyed,omitempty"`
	Key   string `json:"key,omitempty"`
	Holds bool   `json:"holds,omitempty"`
	// Repeats says this step has a count (a key press, or a repeated block) and Count what it
	// is now, from 1. A half of a hold has none: it repeats only as part of a block.
	Repeats bool `json:"repeats,omitempty"`
	Count   int  `json:"count,omitempty"`
	// Drag says this step is a drag, DragMovable whether its two ends can be changed (together),
	// DragWhy what stops it, and X,Y to X2,Y2 the two ends in the basis InWindow names.
	// ToDrag says this click can be made a drag, in its own basis.
	Drag        bool   `json:"drag,omitempty"`
	DragMovable bool   `json:"drag_movable,omitempty"`
	DragWhy     string `json:"drag_why,omitempty"`
	X2          int    `json:"x2"`
	Y2          int    `json:"y2"`
	ToDrag      bool   `json:"to_drag,omitempty"`
}

// StepChange is one operation the page asks for, against a position in the list it was shown.
type StepChange struct {
	// Op is one of the orchestrator's operations: "wait", "remove", "point", "typed",
	// "argument", "text" or "key". A word it does not know is refused there, which is where
	// the vocabulary lives.
	Op string `json:"op"`
	At int    `json:"at"`
	// To is the new duration in milliseconds, for a wait.
	To int `json:"to"`
	// X and Y are where a press should be, for a "point" change, in the basis the row
	// said it was offering.
	X int `json:"x"`
	Y int `json:"y"`
	// Upto is the last position of the range, for a "typed" change, with `At` the first
	// and both ends in it. Its own term rather than `To`, which is a duration: see
	// orchestrator.StepEdit.Upto.
	Upto int `json:"upto"`
	// Name is what the supplied text is called, for an "argument" change.
	Name string `json:"name"`
	// Text is what the step should type instead, for a "text" change. Sent exactly as
	// typed: spaces are part of what a routine types.
	Text string `json:"text"`
	// Key is the key to use instead, for a "key" change, as typed.
	Key string `json:"key"`
	// Kind is which step an "insert" adds after `At`: "wait" (with `To`), "key" (with `Key`)
	// or "type" (with `Text`). `At` is -1 for before the first step.
	Kind string `json:"kind"`
	// Count is how many times, for a "repeat" or a "loop" change.
	Count int `json:"count"`
	// Button, Screen and BasisAt describe a click an "insert" adds: which button; whether X,Y
	// is a point on the screen; or else the step whose measured window it goes inside.
	Button  string `json:"button"`
	Screen  bool   `json:"screen"`
	BasisAt int    `json:"basis_at"`
	// X2 and Y2 are where a drag ends, for a "drag" or "todrag" change, in the basis the row
	// said it was offering.
	X2 int `json:"x2"`
	Y2 int `json:"y2"`
}

// ClickBasisChoice is one window an added click can go inside: the step it was measured at, the
// window in words, and its size so the page can say what the offset is inside.
type ClickBasisChoice struct {
	At    int    `json:"at"`
	Label string `json:"label"`
	W     int    `json:"w"`
	H     int    `json:"h"`
}

// StepEditing is a routine's steps as the panel lists them, before anything is asked.
//
// NOTHING ON DISK HAS CHANGED, and nothing was even rendered if `Why` is set.
type StepEditing struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Why is why there is no panel at all: no such routine, a demonstration that is gone,
	// a source nothing here can say is the render of anything, a generation this build
	// cannot explain. In the backend's own words. Everything else is empty when set.
	Why string `json:"why,omitempty"`
	// Basis identifies the list below, and `ReviewStepEdit` must be given it back. Not
	// the routine's identity, which says nothing about which list was shown.
	Basis string `json:"basis,omitempty"`
	// Rows is every step, in order. A panel that left one out would be offering
	// operations against positions that mean something else.
	Rows []StepRow `json:"rows"`
	// Editable is whether anything can be asked at all, which is `Why` being empty and
	// there being steps to ask about.
	Editable bool `json:"editable,omitempty"`
	// TypedRuns is the runs of these steps that could be declared TYPED TEXT, and
	// TypedSays is what declaring one costs.
	//
	// HERE AS WELL AS ON THE RECORDING REVIEW, and not as a convenience: the recording
	// that exposed this defect is already saved, and a repair reachable only from the
	// review would mean recording it again. Same proposals, same validator — the
	// orchestrator’s `TypedRunsIn`, asked of the list this panel is showing.
	TypedRuns []TypedChoice `json:"typed_runs,omitempty"`
	TypedSays string        `json:"typed_says,omitempty"`
	// Blocks says whether steps can be repeated as one block in this routine, and BlocksSays
	// why not when they cannot: a routine whose presses are checked cannot, and is told how
	// a key press repeats instead. Asked of orchestrator.BlocksRepeatable.
	Blocks     bool   `json:"blocks,omitempty"`
	BlocksSays string `json:"blocks_says,omitempty"`
	// ClickBases are the windows an added click can go inside — the routine's own measured
	// presses, orchestrator.ClickBases — and ScreenClicks whether one may go at a point on the
	// screen instead, which only a routine that checks nothing allows. ClickSays is the sentence
	// when a window click is not possible, so its absence is explained rather than silent.
	ClickBases   []ClickBasisChoice `json:"click_bases,omitempty"`
	ScreenClicks bool               `json:"screen_clicks,omitempty"`
	ClickSays    string             `json:"click_says,omitempty"`
}

// StepPreview is one reviewed edit, before anything is written.
type StepPreview struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Token identifies THIS review and is the term `SaveStepEdit` and `CancelStepEdit`
	// are given back. Not the routine's identity, which two reviews of one routine share.
	Token string `json:"token,omitempty"`
	// Why is why there is no review: the routine was written since its steps were listed,
	// an operation that refers to nothing, no routine of that identity. Everything else
	// is empty when set.
	Why string `json:"why,omitempty"`
	// Correct says Why is about something that was ASKED, against a list that is still the one
	// on screen — an unknown key, braces in new text — so the page keeps what was typed and
	// the person corrects it. Without it, Why means the list itself is no longer current and
	// the page lists the steps again. See orchestrator.ErrChangeRefused.
	Correct bool `json:"correct,omitempty"`
	// Refused is why these operations produce no routine, with the review still worth
	// showing: an edit that would empty it, a held key whose partner cannot be found, a
	// render the generator refuses, a render that is not legal Marco.
	Refused string `json:"refused,omitempty"`
	// Doing is one line per operation, in the order they were asked. The backend's
	// sentences: they name positions and durations, and say when a removal took a
	// partner.
	Doing []string `json:"doing,omitempty"`
	// Was is the source as it stands and Now the source a save would write. Both in full,
	// because a list of changed lines is an interpretation and the file is what runs.
	Was string `json:"was"`
	Now string `json:"now,omitempty"`
	// Changes is whether the two differ at all, and Savable whether there is something a
	// save could write.
	Changes bool `json:"changes"`
	Savable bool `json:"savable"`
	// Rows is the step list the edit produces, so the result is readable as steps and not
	// only as Marco.
	Rows []StepRow `json:"rows,omitempty"`
}

// StepEditDone is what a save did, and the three answers are not one.
type StepEditDone struct {
	// Why is why nothing was written. Everything else is empty when this is set.
	Why string `json:"why,omitempty"`
	// Saved is whether the routine on disk is now the edited one.
	Saved bool `json:"saved,omitempty"`
	// Partial is the third answer: the routine IS the edited one and runs, and its step
	// list was not written down beside it — so it cannot be edited again until it has
	// been rebuilt. Rendered as a problem rather than as a success, because somebody has
	// to know why the next edit refuses. See orchestrator.ErrStepsNotWritten.
	Partial bool `json:"partial,omitempty"`
	// Says is the outcome in a sentence.
	Says string `json:"says,omitempty"`
}

// editsKept is how many reviews one window holds at once, for the reason `movesKept` is that
// many: every press of Review makes one, nothing obliges anybody to act on it, and an evicted one
// is a REFUSAL with a reason rather than a wrong write.
const editsKept = 16

// StepEditing lists one routine's steps and what can be done to each.
//
// IT WRITES NOTHING. It is also the only way to get a `Basis`, which is the only way to state an
// operation — so a page cannot ask for a change to a list it was never shown.
func (a *Acts) StepEditing(id string) StepEditing {
	id = strings.TrimSpace(id)
	if id == "" {
		return StepEditing{Rows: []StepRow{}, Why: "choose a play to edit"}
	}
	a.mu.Lock()
	recording := a.recordingLocked()
	a.mu.Unlock()
	if recording {
		return StepEditing{ID: id, Rows: []StepRow{},
			Why: "Marco is recording a demonstration; one thing at a time"}
	}
	chosen, ok := a.routineWithID(id)
	if !ok {
		return StepEditing{ID: id, Rows: []StepRow{},
			Why: fmt.Sprintf("there is no play %q in this library (%s) — press "+
				"Refresh and choose it again", id, a.routesDir)}
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	// NO OPERATIONS, which is the question "may this be edited, and what are its steps?"
	// — answered through the path that would perform one, so this panel cannot be told
	// yes by one set of rules and refused by another.
	e, err := d.PrepareEdit(chosen.route(), nil)
	if err != nil {
		return StepEditing{ID: id, Name: chosen.Name, Rows: []StepRow{},
			Why: err.Error()}
	}
	rows := stepRows(e)
	// AND WHICH RUNS OF THEM A PERSON COULD DECLARE TYPED TEXT, off the same list the
	// rows came from so a range the chooser offers is a range of the rows on screen.
	runs := typedChoices(e.Steps())
	// AND WHETHER A BLOCK CAN BE REPEATED HERE, said rather than simply not offered: a
	// control that vanishes reads as a capability that does not exist.
	// AND WHERE A CLICK CAN BE ADDED: inside which of its own measured windows, and whether at
	// a screen point at all. Both answers are the contract's, so nothing is offered that the
	// contract then refuses.
	var bases []ClickBasisChoice
	for _, b := range orchestrator.ClickBases(e.Steps()) {
		bases = append(bases, ClickBasisChoice{At: b.At, Label: b.Label(), W: b.WinW, H: b.WinH})
	}
	screen := orchestrator.ScreenClicksAllowed(e.Steps())
	clickSays := ""
	switch {
	case len(bases) == 0 && screen:
		clickSays = "No step here was measured inside a window, so a click can be added " +
			"only at a point on the screen."
	case len(bases) == 0:
		clickSays = "A click cannot be added here: this play checks each press against " +
			"its window, and none of its presses was measured fully enough to add one " +
			"inside it."
	}
	blocks := orchestrator.BlocksRepeatable(e.Steps())
	blocksSays := ""
	if !blocks {
		blocksSays = "This play checks each press against its window, so steps cannot " +
			"repeat as a block. A key press can still repeat by its count."
	}
	return StepEditing{ID: id, Name: e.Name, Basis: e.Basis, Rows: rows,
		Editable: len(rows) > 0, TypedRuns: runs, TypedSays: typedCaption(runs),
		Blocks: blocks, BlocksSays: blocksSays,
		ClickBases: bases, ScreenClicks: screen, ClickSays: clickSays}
}

// stepRows is a review's step list as the panel shows it.
//
// The words come from `oneStep` and the pairing from the review, which computes it with the same
// function the removal uses. Nothing here decides anything about a step.
func stepRows(e orchestrator.EditReview) []StepRow {
	steps := e.Steps()
	says := inWords(steps)
	out := make([]StepRow, 0, len(steps))
	for i, s := range steps {
		r := StepRow{At: i, Says: says[i]}
		if s.Kind == macroir.StepWait {
			r.Wait, r.Ms = true, s.Ms
		}
		switch s.Kind {
		case macroir.StepKey:
			r.Keyed, r.Key = true, s.Key
			r.Repeats, r.Count = true, max(s.Count, 1)
		case macroir.StepLoop:
			r.Repeats, r.Count = true, max(s.Count, 1)
		case macroir.StepKeyDown, macroir.StepKeyUp:
			r.Keyed, r.Key, r.Holds = true, s.Key, true
		}
		if s.Kind == macroir.StepType {
			// WHAT IT TYPES NOW, so the panel can offer to make it changeable and
			// say what it would be replacing.
			r.Types, r.Text = true, s.Text
			if args := routes.Arguments(s.Text); len(args) > 0 {
				r.Types = false
				r.Supplied, r.Named = true, args[0].Name
			}
		}
		// ── AND WHERE IT PRESSES, WHEN IT PRESSES ANYWHERE ──
		//
		// Asked for EVERY step, so this panel never works out for itself which rows
		// have a position: a surface that decided would be free to offer a box the
		// validator then refuses, which is a control whose refusal arrives after
		// somebody has typed into it.
		// A DRAG IS ITS OWN READING, two ends changed together, and never a point.
		if d := orchestrator.DragIn(steps, i); d.Is {
			r.Drag, r.DragMovable, r.DragWhy = true, d.Movable, d.Why
			r.InWindow, r.X, r.Y, r.X2, r.Y2 = d.InWindow, d.X, d.Y, d.X2, d.Y2
		} else if p := orchestrator.PointIn(steps, i); p.Is {
			r.Point, r.Movable, r.PointWhy = true, p.Movable, p.Why
			r.InWindow, r.X, r.Y = p.InWindow, p.X, p.Y
		}
		// AND A CLICK THAT CAN BECOME A DRAG, asked of the rule the contract refuses with, so
		// nothing is offered that is then refused.
		r.ToDrag = s.Kind == macroir.StepClick && orchestrator.WhyNotADrag(steps, i) == ""
		if partner := e.PairedWith(i); partner >= 0 {
			r.Pairs, r.PairsAt = true, partner
			r.PairSays = fmt.Sprintf("removes step %d too: they are the two "+
				"halves of holding %q",
				partner+1, s.Key)
		}
		out = append(out, r)
	}
	return out
}

// ReviewStepEdit applies the operations to the list they were counted against, and shows what
// would be written.
//
// IT WRITES NOTHING, and it is the only way to reach `SaveStepEdit`.
//
// `basis` is what `StepEditing` handed over with the list. An empty one is REFUSED here rather
// than passed through: the backend cannot tell whether a caller had a list on a screen, and this
// can.
func (a *Acts) ReviewStepEdit(id, basis string, changes []StepChange) StepPreview {
	id = strings.TrimSpace(id)
	basis = strings.TrimSpace(basis)
	switch {
	case id == "":
		return StepPreview{Why: "choose a play to edit"}
	case basis == "":
		// NO LIST, NO OPERATIONS. A caller without a basis has not been shown the
		// steps its positions refer to, and passing that through would reach the
		// unchecked call. See the note at the top of this file.
		return StepPreview{ID: id, Why: "list the steps before asking to change " +
			"them — a change refers to the list it was read off"}
	case len(changes) == 0:
		return StepPreview{ID: id, Why: "nothing was asked"}
	}
	a.mu.Lock()
	recording := a.recordingLocked()
	a.mu.Unlock()
	if recording {
		return StepPreview{ID: id, Why: "Marco is recording a demonstration; one " +
			"thing at a time"}
	}
	chosen, ok := a.routineWithID(id)
	if !ok {
		return StepPreview{ID: id, Why: fmt.Sprintf("there is no play %q in this "+
			"library (%s) — press Refresh and choose it again", id, a.routesDir)}
	}
	edits := make([]orchestrator.StepEdit, 0, len(changes))
	for _, c := range changes {
		// THE NAME TRAVELS UNTRIMMED OF ITS MEANING, and the contract validates it.
		// A window that lowered it, stripped it or rejected it here would be a second
		// opinion about what a placeholder may be called — and the one that matters is
		// the one `routes` own pattern will read back.
		edits = append(edits, orchestrator.StepEdit{
			Op: orchestrator.EditOp(strings.TrimSpace(c.Op)), At: c.At, To: c.To,
			Upto: c.Upto, X: c.X, Y: c.Y, Name: c.Name, Text: c.Text, Key: c.Key, Kind: c.Kind, Count: c.Count, X2: c.X2, Y2: c.Y2,
			Button: c.Button, Screen: c.Screen, BasisAt: c.BasisAt})
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	e, err := d.PrepareEditOf(chosen.route(), basis, edits)
	if err != nil {
		return StepPreview{ID: id, Name: chosen.Name, Was: chosen.Source,
			Why: err.Error(), Correct: errors.Is(err, orchestrator.ErrChangeRefused)}
	}
	// THE PREPARED EDIT STAYS HERE, UNDER ITS OWN TOKEN.
	a.keepEdit(e)
	return StepPreview{ID: id, Name: e.Name, Token: e.Token, Refused: e.Refused,
		Doing: e.Says, Was: e.Was, Now: e.Now, Changes: e.Changes(),
		Savable: e.Savable(), Rows: stepRows(e)}
}

// keepEdit holds one prepared edit, oldest out first.
func (a *Acts) keepEdit(e orchestrator.EditReview) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.edits == nil {
		a.edits = map[string]orchestrator.EditReview{}
	}
	if _, already := a.edits[e.Token]; !already {
		a.editOrder = append(a.editOrder, e.Token)
	}
	a.edits[e.Token] = e
	for len(a.editOrder) > editsKept {
		delete(a.edits, a.editOrder[0])
		a.editOrder = a.editOrder[1:]
	}
}

// CancelStepEdit drops one prepared edit without writing anything.
//
// There is nothing to undo — preparation wrote nothing — so this only lets go of what the window
// holds. It exists so that cancelling is something the page DOES rather than something it omits: a
// review left in memory would be saved with later, by a second press of a button somebody had
// already dismissed.
func (a *Acts) CancelStepEdit(token string) {
	token = strings.TrimSpace(token)
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.edits, token)
	for i, t := range a.editOrder {
		if t == token {
			a.editOrder = append(a.editOrder[:i], a.editOrder[i+1:]...)
			break
		}
	}
}

// SaveStepEdit writes one reviewed edit.
//
// It takes the REVIEW's token and nothing else. The three digests the review was bound to are in
// here, the source that lands is the one that was rendered for the screen, and the page contributes
// no part of either.
func (a *Acts) SaveStepEdit(token string) StepEditDone {
	token = strings.TrimSpace(token)
	a.mu.Lock()
	recording := a.recordingLocked()
	e, prepared := a.edits[token]
	a.mu.Unlock()
	switch {
	case recording:
		return StepEditDone{Why: "Marco is recording a demonstration; one thing at " +
			"a time"}
	case !prepared:
		// NOTHING IS SAVED WITHOUT THAT REVIEW IN HAND. Not the routine's most
		// recent one and not one made here as a convenience: the source that lands
		// is the one that was on the screen.
		return StepEditDone{Why: "the edit you are looking at is no longer the one " +
			"this window holds — review it again and the comparison will be " +
			"against the play as it now stands"}
	}
	d := orchestrator.Deps{Reg: a.reg(), Out: &strings.Builder{}}
	err := d.SaveEdit(e)
	switch {
	case errors.Is(err, orchestrator.ErrStepsNotWritten):
		// THE THIRD ANSWER. The routine IS the edited one and it runs; its step list
		// is not beside it, so the next edit will refuse. Spent either way: the
		// source has been written, and a second press is not a second edit.
		a.CancelStepEdit(token)
		a.trial.recorded(e.Name, err.Error(),
			"It runs as you edited it. Rebuild it from its demonstration before "+
				"editing it again.", true, Pictured{})
		return StepEditDone{Saved: true, Partial: true, Says: err.Error()}
	case err != nil:
		// THE REVIEW IS KEPT. Every refusal here names something that moved, and
		// the way forward is to look again — which needs the review still on the
		// screen to press the button beside.
		return StepEditDone{Why: err.Error()}
	}
	a.CancelStepEdit(token)
	says := fmt.Sprintf("Saved %q with %s.", e.Name, countedEdits(len(e.Edits)))
	a.trial.recorded(e.Name, says, "Run it and see.", false, Pictured{})
	return StepEditDone{Saved: true, Says: says}
}

// countedEdits is how many operations, in words that read in a sentence.
func countedEdits(n int) string {
	if n == 1 {
		return "one change to its steps"
	}
	return fmt.Sprintf("%d changes to its steps", n)
}
