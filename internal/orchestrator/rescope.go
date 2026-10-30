package orchestrator

// A RESCOPE IS A REVIEWED MOVE BETWEEN TWO IDENTITIES, AND NOT A SAVE UNDER ANOTHER WORD.
//
// Where a routine WORKS is its folder, so changing it creates a route at one path and ends one at
// another: `routes/<app>/context/<slug>.marco` and `routes/<app>/focus/<slug>.marco` are two
// files, and `routes/global/<slug>.marco` is a third. Two files means two things that can already
// be there, two things that can change while somebody reads a review, and a window between the
// write and the removal in which both exist.
//
//	PREPARE   render the routine for the scope asked for, from its own demonstration.
//	          NOTHING IS WRITTEN. What comes back is the source as it stands, the source a
//	          move would write, what is already at the destination, and digests of all of it.
//	REVIEW    the caller shows both sources and whatever is in the way. A move is approved
//	          against THE SOURCE ON THE SCREEN, which is what makes it safe for a routine
//	          somebody edited by hand: their own text is the left-hand side.
//	MOVE      under BOTH locks, revalidating both ends; create the destination exclusively
//	          unless a version-bound consent says to replace it; read the destination back;
//	          revalidate the original; and only then remove it.
//
// # Why `Resave` is not this
//
// `Resave` repairs a routine that cannot press anything at all, and it is immediate on purpose:
// the file it replaces refuses every press, so there is nothing to weigh up and nothing at the
// destination worth protecting. It takes one lock, consents to nothing, and deletes the original
// after a save it never reads back. Useful machinery; not a move contract.
//
// # Why rebuilding from the demonstration, rather than editing the source
//
// Because the difference between the three scopes is IN the generated source — a focus route
// carries the `Activate` that brings its application forward, and the other two have it stripped
// — and rewriting generated Marco by hand is how two things that have to agree stop agreeing.
// The cost is that a routine somebody edited by hand comes back as a rendering of its recording,
// which is why the rendering is on the screen before anything is written and why the token binds
// it: nothing is moved that was not displayed.
//
// # What a hand edit costs, said plainly
//
// `Was` is the file as it stands, hand edits and all. `Now` is what the move would write. If
// somebody edited their routine, those differ by their edit and the review is where they see
// that. Approving it is approving the loss of the edit; there is no third thing this could do
// that would not be inventing source nobody wrote.
//
// See [[ADR-266-one-set-of-rules-for-keeping-a-recording]] for the lock, the consent-to-a-version
// rule and the exclusive create, all of which this uses unchanged.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/mlog"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Rescoping is one routine rendered for another scope, for review.
//
// NOTHING ON DISK HAS CHANGED. Every field is a reading or a rendering held in memory, and both
// routines are exactly as they were until `Rescope` is called with this and succeeds.
type Rescoping struct {
	// Token identifies THIS preparation, and a move must carry it back. Derived from
	// everything the review showed and everything it would touch — see `sealed`.
	Token string
	// From is the routine as it is saved now, To where it would go. Two identities, and
	// the whole of what this flow is about.
	From, To routes.Route
	// Want is the scope asked for, as a word a person chose.
	Want Scope
	// Name is what the routine calls itself, out of its own header.
	Name string
	// Was is the source as it stands — hand edits and all — and Now the source a move
	// would write at the destination. Both in full: a list of differences is an
	// interpretation, and the file is the thing that will run.
	Was, Now string
	// Refused is why this move cannot be made at all, in the words of whatever refused
	// it: a scope this routine's presses cannot be performed under, a demonstration
	// today's rules will not write down, a destination that needs an application nobody
	// named. A reviewable answer rather than an error, because the review is still worth
	// showing — a `Rescoping` carrying one cannot be moved with.
	Refused string
	// Taken is whether something is already saved at the destination, and TakenSays is
	// that in a sentence. A move onto an occupied identity needs its own consent: the
	// person agreed to move THIS routine, which is not agreement to lose THAT one.
	Taken     bool
	TakenSays string
	// TakenID is the destination's whole identity, which is the term a consent names.
	TakenID string
	// library is the store this was prepared against. A `Deps` pointed at another one may
	// not move with it: the same routine exists in a dogfood store, in a test store and
	// in somebody's real one.
	library string
	// The four digests this review is bound to: the original's source and demonstration,
	// and whatever was at the destination — "" when nothing was. Every one of them is
	// re-read under the locks before anything is written.
	fromSourceAt, fromRecordingAt string
	toSourceAt, toRecordingAt     string
	// fromStepsAt is a digest of the original's step account, and "" when it has none —
	// re-read with the other two, because an edit landing while somebody reads this
	// review is a routine that has changed.
	fromStepsAt string
	// fromAccount is whether the version to be written was rendered from the routine's
	// own stored steps, and accountSteps is that step list.
	//
	// A MOVE CARRIES THE STORED BASIS. The difference between the scopes is in the
	// generated source, so a move renders again — and rendering the DEMONSTRATION would
	// hand somebody back a routine with whatever the basis holds undone, through a surface
	// whose whole job is to show them what will be written. The account travels with it,
	// rebound to the source the move wrote; the same demonstration is written beside it,
	// so its other digest does not change.
	//
	// A step edit is the case this was built for; a re-simplification stores a basis too,
	// and a move of one of those is the same operation for the same reason.
	fromAccount  bool
	accountSteps []byte
	// assets are the anchor images the rendering produced for the DESTINATION directory,
	// held here rather than written, because preparation writes nothing.
	assets map[string][]byte
	// recording is the demonstration to be written beside the moved routine. The same
	// bytes, carried rather than re-read: what lands at the destination is what this
	// review was built from.
	recording []byte
}

// Movable is whether this review has something a move could write.
func (m Rescoping) Movable() bool { return m.Refused == "" && m.Now != "" }

// ConsentToReplace is the version-bound agreement to lose what was at the destination.
//
// THE DIGESTS ARE THIS PACKAGE'S AND A CALLER CANNOT AUTHOR THEM, which is the point: a consent
// says what somebody SAW, and a surface that assembled one from its own fields would be handing
// back a value whose only job is to be compared with the thing it came from. A caller shows
// `TakenSays`, gets a yes, and asks for this.
//
// Zero when nothing was there, so passing it unconditionally is not a way to consent to a
// collision that appeared afterwards: `Rescope` revalidates the destination under the locks and
// an empty consent leaves the create EXCLUSIVE.
func (m Rescoping) ConsentToReplace() Consent {
	if !m.Taken {
		return Consent{}
	}
	return Consent{ID: m.TakenID, Source: m.toSourceAt, Recording: m.toRecordingAt}
}

// Rescoped is what a move did, and it is NOT always "moved".
//
// A destination that was written and an original that could not be removed is TWO routines, and
// calling that a move would be this flow reporting a state it is not in — the person would go
// looking for one routine and find two, having been told there was one.
type Rescoped struct {
	// To is where the routine now is, which is the same either way: the destination
	// exists in both outcomes this reports.
	To routes.Route
	// Removed is whether the original is gone. False means both exist.
	Removed bool
	// Residue is what sat beside the original and could not be swept once it had gone —
	// its demonstration, an anchor image, its provenance sidecar. Empty in the ordinary
	// case, and NEVER a reason the move failed: the routine is gone, so what is left is
	// litter rather than a routine. Reported because a recording left under a slug is a
	// file the next routine saved there would inherit.
	Residue []string
	// Says is the OUTCOME in a sentence, and nothing else: "Moved …", or
	// "Copied …; original retained: …" with the reason the original is still there.
	Says string
	// Left is the residue above as a sentence, and it is SEPARATE from `Says` because
	// it is a separate thing to be told: the move happened, and something is still
	// lying at the old place. A caller that renders both renders two facts; one that
	// had them composed into `Says` and a residue line beside it would be saying one
	// of them twice, which is the defect this window keeps finding in itself. Empty
	// when `Residue` is.
	Left string
}

// PrepareRescope renders a saved routine for another scope, for review.
//
// IT WRITES NOTHING. Not the source, not the images, not the demonstration — a failure anywhere
// in here leaves both ends of the move exactly as they were.
//
// `app` is used only when the destination needs an application and the routine has none: a global
// routine being given one. It is ignored otherwise, because a move does not get to change which
// application a routine is about on the way past.
func (d Deps) PrepareRescope(rt routes.Route, want Scope, app string) (Rescoping, error) {
	want, why := WantedScope(string(want))
	if why != "" {
		return Rescoping{}, fmt.Errorf("%s", why)
	}
	if rt.Slug == "" {
		return Rescoping{}, fmt.Errorf("a move needs the routine it is about")
	}
	// ── THE SCOPE IT ALREADY HAS IS NOT A MOVE ──
	//
	// Said rather than performed. The destination identity would be the source identity,
	// which means one lock taken twice, an exclusive create onto the file being moved, and
	// a removal of what had just been written. Every one of those is a way of losing a
	// routine to answer a question nobody asked.
	if want == ScopeAlreadyOf(rt) {
		return Rescoping{}, fmt.Errorf("%q already works there, so there is nothing "+
			"to move", NameOf(d.Reg, rt))
	}
	path := d.Reg.Path(rt)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Rescoping{}, fmt.Errorf("%s could not be read, so there is nothing to "+
			"move: %w", rt.Slug, err)
	}
	src := string(raw)
	m := Rescoping{From: rt, Want: want, Name: NameIn(src, rt.Slug), Was: src,
		library: d.Reg.Dir, fromSourceAt: routes.DigestOf(src)}

	// ── ITS OWN DEMONSTRATION, WHICH IS THE ONLY THING A SCOPE CAN BE CHANGED FROM ──
	rec, ok := d.Reg.LoadRecording(rt)
	if !ok {
		return Rescoping{}, fmt.Errorf("%q no longer has the demonstration it was "+
			"made from, and the difference between the scopes is in the "+
			"generated source — so there is nothing to render it from. Record "+
			"it again where you want it", m.Name)
	}
	m.recording, m.fromRecordingAt = rec, routes.DigestOf(string(rec))
	var events []recorder.RecordedEvent
	if err := json.Unmarshal(rec, &events); err != nil {
		return Rescoping{}, fmt.Errorf("%q: its demonstration could not be read back, "+
			"so no version can be rendered from it: %w", m.Name, err)
	}
	// THE MODE THE FILE STATES, read off the source being moved. It decides the
	// simplification as well as the line it writes, so taking it from this `Deps` would
	// rebuild somebody's routine with every recorded pause capped — the defect `Resave`
	// had. See ModeOf.
	dr := d.draftFor(RebuildCurrent, m.Name, events, rt.App, "a move", ModeOf(src))

	// ── OR FROM WHAT THE ROUTINE SAYS IT DOES, WHEN IT HAS WRITTEN THAT DOWN ──
	//
	// An edited routine's source is the render of its ACCOUNT. Moving it is still a
	// re-render — the scopes differ in the generated source — but of the step list the
	// routine states, not of the demonstration, which has never contained the edit.
	//
	// The account is only believed while it still describes both files beside it. One that
	// does not is not an account of this routine, so the move renders the demonstration
	// exactly as it always did, and the review's two sources are where that is read.
	stored, state := d.Reg.Steps(rt)
	at, readable := d.Reg.StepsDigest(rt)
	if !readable {
		// THERE AND UNREADABLE IS NOT ABSENT: a move is bound to three files and
		// this is one of them. See routes.Registry.StepsDigest.
		return Rescoping{}, fmt.Errorf("%q cannot be moved: the step list stored "+
			"beside it is there and could not be read, so a move cannot say "+
			"what it would do to it", m.Name)
	}
	m.fromStepsAt = at
	if state.Usable() {
		var steps []macroir.Step
		if err := json.Unmarshal(stored.Steps, &steps); err != nil {
			return Rescoping{}, fmt.Errorf("%q: its stored steps could not be read "+
				"back, so no version can be rendered from them: %w",
				m.Name, err)
		}
		if len(steps) > 0 {
			base, args := routes.SplitArgs(m.Name)
			dr = &Draft{Name: base, Args: args, Steps: steps, Mode: ModeOf(src)}
			m.fromAccount, m.accountSteps = true, stored.Steps
		}
	}

	to, toApp, why := rescopeTarget(rt, want, app, dr)
	if why != "" {
		m.Refused = why
		return m.sealed(), nil
	}
	m.To, m.TakenID = to, RouteID(to)

	switch {
	case dr == nil || len(dr.Steps) == 0:
		m.Refused = "its demonstration contains nothing that can be performed"
		return m.sealed(), nil
	case m.fromAccount:
		// ALREADY REPRESENTABLE, BY CONSTRUCTION. These steps were rendered and
		// compiled once to become the file being moved; `Unrepresentable` is a
		// property of simplifying a recording, and this list was not simplified
		// here. The draft built above carries the refusal of the recording it was
		// NOT rendered from, which would refuse a move for a reason that is not
		// about what is being moved.
	case dr.Unrepresentable != "":
		m.Refused = fmt.Sprintf("a version rendered from its recording cannot be "+
			"saved as it stands: %s", dr.Unrepresentable)
		return m.sealed(), nil
	}
	// ── AND WHETHER ITS PRESSES CAN BE PERFORMED THERE AT ALL ──
	//
	// The same rule the save path applies, asked here so it is a sentence in a review
	// rather than a refusal after somebody has decided. A recording whose presses check
	// the window they were made in needs that window brought forward, and two of the three
	// scopes bring none — their routes would refuse every press.
	if why := d.WhyNotThisScope(dr, want); why != "" {
		m.Refused = why
		return m.sealed(), nil
	}
	now, assets, err := d.render(dr, want.mode(), toApp, to)
	if err != nil {
		// THE GENERATOR'S OWN WORDS. It refuses a recording whose press cannot be
		// stopped at, which is a thing to read rather than a thing to translate.
		m.Refused = err.Error()
		return m.sealed(), nil
	}
	m.Now, m.assets = now, assets
	// ── AND WHAT IS ALREADY THERE, WHICH IS A SEPARATE QUESTION FROM THIS ROUTINE ──
	m.toSourceAt, m.toRecordingAt = d.versionAt(to)
	if m.toSourceAt != "" {
		m.Taken = true
		m.TakenSays = fmt.Sprintf("there is already a routine at %s. Moving %q there "+
			"would replace it, which is a second thing to agree to", m.TakenID,
			m.Name)
	}
	return m.sealed(), nil
}

// rescopeTarget is where a move would go, and which application it would be about.
//
// The SLUG NEVER CHANGES: a move is about where a routine works, and renaming it on the way would
// be two decisions wearing one word. The application changes only in the one direction that has
// no other answer — a global routine has none, and the two application scopes need one.
func rescopeTarget(rt routes.Route, want Scope, app string, dr *Draft) (routes.Route, string, string) {
	if want == ScopeEverywhere {
		// APP-LESS, DELIBERATELY. A routine that works anywhere is about no
		// application, which is the whole of what the scope means.
		return routes.Route{Slug: rt.Slug}, "", ""
	}
	which := rt.App
	if which == "" {
		which = app
	}
	if which == "" && dr != nil {
		// THE ONE IT BRINGS FORWARD, when the file says. `Resave` reads it the same
		// way for the same reason: a global routine's identity has lost the
		// application, and its demonstration has not.
		which = firstActivateApp(dr.Steps)
	}
	if which == "" {
		return routes.Route{}, "", fmt.Sprintf("%q works anywhere and is about no "+
			"application, and %q needs one — name the application it should "+
			"work in", rt.Slug, want)
	}
	return routes.Route{App: which, Focus: want == ScopeAppAnywhere, Slug: rt.Slug},
		which, ""
}

// Rescope performs the reviewed move, or leaves both ends as they were.
//
// # The order, and what each step is for
//
//	BOTH LOCKS, IN A FIXED ORDER      two writers taking two locks in two orders is a deadlock,
//	                                  so the order is the identities' own and never the
//	                                  caller's.
//	REVALIDATE BOTH ENDS              the review was of a pair of files, and either can move
//	                                  while somebody reads it.
//	CREATE OR REPLACE THE DESTINATION exclusive unless a version-bound consent says otherwise.
//	READ THE DESTINATION BACK         source AND demonstration. A write that returned no error
//	                                  is not a routine that is there.
//	REVALIDATE THE ORIGINAL           never delete a newer version than the one reviewed.
//	REMOVE THE ORIGINAL               and only now. A failure here is reported as a COPY.
func (d Deps) Rescope(m Rescoping, consent Consent) (Rescoped, error) {
	switch {
	case m.From.Slug == "" || m.To.Slug == "":
		return Rescoped{}, fmt.Errorf("a move needs both the routine and where it " +
			"is going")
	case m.library != "" && m.library != d.Reg.Dir:
		return Rescoped{}, fmt.Errorf("%q was reviewed in the library at %s and this "+
			"is %s, so nothing was written — a routine of that name here is a "+
			"different routine", m.Name, m.library, d.Reg.Dir)
	case m.Token == "" || m.Token != m.sealed().Token:
		return Rescoped{}, fmt.Errorf("the move offered for %q is not the one that "+
			"was prepared, so nothing was written — review it again", m.Name)
	case !m.Movable():
		if m.Refused != "" {
			return Rescoped{}, fmt.Errorf("%q cannot be moved: %s", m.Name, m.Refused)
		}
		return Rescoped{}, fmt.Errorf("%q has no rendered version to move", m.Name)
	case RouteID(m.From) == RouteID(m.To):
		// UNREACHABLE THROUGH `PrepareRescope`, which refuses the unchanged scope
		// before anything is read. Here because this function takes a value a
		// caller holds, and the failure it prevents is the destructive one: one
		// lock taken twice, then a removal of what was just written.
		return Rescoped{}, fmt.Errorf("%q is already at %s", m.Name, RouteID(m.To))
	}

	// ── BOTH ROUTES, IN AN ORDER NEITHER CALLER CHOOSES ──
	//
	// Two of these can run at once — two windows, a window and a terminal — and if one
	// takes the source first while the other takes the destination first, both wait for
	// the other forever. Ordering the acquisitions by the identity itself makes that
	// impossible: whatever the two moves are, they ask for the same lock first.
	//
	// `SaveWithin` bounds the wait, so a lock held by something wedged is a sentence
	// rather than a window that never answers.
	first, second := lockOrder(m.From, m.To)
	holdFirst, err := d.Reg.Lock(first, SaveWithin)
	if err != nil {
		return Rescoped{}, fmt.Errorf("%q is left as it is: %w", m.Name, err)
	}
	defer holdFirst.Release()
	holdSecond, err := d.Reg.Lock(second, SaveWithin)
	if err != nil {
		return Rescoped{}, fmt.Errorf("%q is left as it is: %w", m.Name, err)
	}
	defer holdSecond.Release()

	// ── AND THE WORLD IS STILL THE ONE THAT WAS REVIEWED, AT BOTH ENDS ──
	fromSrc, fromRec := d.versionAt(m.From)
	fromStepsAt, stepsReadable := d.Reg.StepsDigest(m.From)
	switch {
	case fromSrc == "":
		return Rescoped{}, fmt.Errorf("%q is no longer at %s, so nothing was "+
			"written — it has been deleted or moved since you looked at it",
			m.Name, RouteID(m.From))
	case fromSrc != m.fromSourceAt:
		return Rescoped{}, fmt.Errorf("%q has been saved again since the move you "+
			"reviewed was prepared, so it is left as it is — review it again "+
			"and the comparison will be against the routine as it now stands",
			m.Name)
	case fromRec != m.fromRecordingAt:
		return Rescoped{}, fmt.Errorf("%q has been recorded again since the move you "+
			"reviewed was prepared, so it is left as it is — review it again "+
			"and what would be written will be rendered from the new recording",
			m.Name)
	case !stepsReadable || fromStepsAt != m.fromStepsAt:
		// ITS STORED STEP LIST MOVED WHILE SOMEBODY READ THIS. The source digest
		// would catch an edit too, and would say the wrong thing about it; this
		// says what happened. An unreadable file counts as moved, because a move
		// may not be made against a file nothing has read.
		return Rescoped{}, fmt.Errorf("the step list stored beside %q has changed "+
			"since the move you reviewed was prepared, so it is left as it "+
			"is — review it again and what would be written will be rendered "+
			"from its steps as they now stand", m.Name)
	}
	toSrc, toRec := d.versionAt(m.To)
	switch {
	case toSrc == "" && m.Taken:
		return Rescoped{}, fmt.Errorf("the routine at %s that you agreed to replace "+
			"has gone since you were shown it, so nothing was written — move "+
			"%q again and it will be saved there with nothing to replace",
			m.TakenID, m.Name)
	case toSrc != "" && !consent.Given():
		// A COLLISION THAT WAS THERE, OR ONE THAT APPEARED WHILE SOMEBODY READ. The
		// same sentence either way, because the person's position is the same:
		// something is at the destination and they have agreed to nothing about it.
		return Rescoped{}, fmt.Errorf("%q was not moved: there is already a routine "+
			"at %s, and moving this one there would replace it", m.Name,
			m.TakenID)
	case toSrc != "" && consent.ID != m.TakenID:
		return Rescoped{}, fmt.Errorf("this would not replace the routine you chose. "+
			"You chose %q, and moving %q there would replace %q — choose once "+
			"more", consent.ID, m.Name, m.TakenID)
	case toSrc != "" && (consent.Source != toSrc || consent.Recording != toRec):
		// AND THE VERSION THEY AGREED TO LOSE IS THE ONE THAT IS STILL THERE. The
		// identity does not change when the contents do, so "same route" is not the
		// same question: another save landing there leaves a newer routine at the
		// identity somebody agreed to overwrite.
		return Rescoped{}, fmt.Errorf("the routine at %s has been saved again since "+
			"you were shown it, so nothing was written — look at what is there "+
			"now and choose once more", m.TakenID)
	}
	// THE ONE INSTANT A TEST CAN STAND IN: after both revalidations and before anything
	// is written, under both locks. Nil in every production path.
	if d.AfterValidating != nil {
		d.AfterValidating()
	}

	// ── THE IMAGES FIRST, BECAUSE THEY ARE WHAT THE SOURCE REFERS TO ──
	//
	// Decided whole and created exclusively, exactly as a replacement's are: a path that
	// already holds a DIFFERENT picture is a refusal with nothing written, because
	// overwriting it would change what some other routine beside it looks for. See
	// imagesToWrite.
	images, why := imagesToWrite(m.assets)
	if why != "" {
		return Rescoped{}, fmt.Errorf("%q is left as it is: %s", m.Name, why)
	}
	if err := d.Reg.CreateAssets(images); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return Rescoped{}, fmt.Errorf("%q is left as it is: an image the moved "+
				"version needs appeared while this was being prepared to "+
				"write, so something else is writing beside it — review it "+
				"again (%v)", m.Name, err)
		}
		return Rescoped{}, fmt.Errorf("%q is left as it is: the images the moved "+
			"version needs could not be written: %w", m.Name, err)
	}

	// ── THE DESTINATION: EXCLUSIVE UNLESS SOMEBODY CONSENTED TO WHAT IS THERE ──
	//
	// The create is itself the check. A routine appearing between the revalidation above
	// and this write belongs to whoever created it, and a caller that was told the path
	// was free has consented to nothing about it — so the path existing IS the failure,
	// with nothing written to it.
	if consent.Given() {
		err = d.Reg.Replace(m.To, m.Now)
	} else {
		err = d.Reg.Create(m.To, m.Now)
	}
	if err != nil {
		if os.IsExist(err) {
			return Rescoped{}, fmt.Errorf("%q is left as it is: a routine was saved "+
				"at %s while this move was being written, so nothing has been "+
				"written over it", m.Name, m.TakenID)
		}
		return Rescoped{}, fmt.Errorf("%q is left as it is: %w", m.Name, err)
	}
	// AND ITS DEMONSTRATION BESIDE IT, because a routine that arrives without one cannot
	// be reviewed, rebuilt or moved again. Not best-effort here, unlike the save path: a
	// move that lands a routine without its recording has quietly made it a thing nobody
	// can do any of that to, and the original it was about to delete is the only other
	// copy.
	if err := d.Reg.SaveRecording(m.To, m.recording); err != nil {
		return Rescoped{To: m.To, Says: copiedSays(m.Name, m.TakenID,
			fmt.Sprintf("its demonstration could not be written beside it (%v)",
				err))}, nil
	}
	// AND ITS ACCOUNT, WHEN THE VERSION BEING WRITTEN IS THE RENDER OF ONE.
	//
	// Bound to the source this move just wrote and to the demonstration written beside
	// it, which is the same one — so the moved routine arrives editable, and the edit it
	// carries is still the explanation of its file. Last of the three, for the reason
	// routes.SaveSteps gives: there is nothing to record until the source is on disk.
	if m.fromAccount {
		if err := d.Reg.SaveSteps(m.To, routes.StoredSteps{
			Source: routes.DigestOf(m.Now), Demonstration: m.fromRecordingAt,
			Steps: m.accountSteps}); err != nil {
			return Rescoped{To: m.To, Says: copiedSays(m.Name, m.TakenID,
				fmt.Sprintf("its steps could not be written down beside "+
					"it (%v), so the original has not been touched",
					err))}, nil
		}
	}
	if d.afterDestination != nil {
		d.afterDestination()
	}

	// ── READ THE DESTINATION BACK BEFORE ANYTHING IS REMOVED ──
	//
	// A write that returned no error is not a routine that is there. The file can be
	// truncated by something else, replaced by a save that landed in the same instant, or
	// written to a path that resolves somewhere nobody expected — and the original is
	// about to be deleted on the strength of this.
	landedSrc, landedRec := d.versionAt(m.To)
	switch {
	case landedSrc != routes.DigestOf(m.Now):
		return Rescoped{To: m.To, Says: copiedSays(m.Name, m.TakenID,
			"what is at "+m.TakenID+" is not what this move wrote, so the "+
				"original has not been touched — look at both before "+
				"deleting either")}, nil
	case landedRec != routes.DigestOf(string(m.recording)):
		return Rescoped{To: m.To, Says: copiedSays(m.Name, m.TakenID,
			"the demonstration beside "+m.TakenID+" is not the one this move "+
				"wrote, so the original has not been touched")}, nil
	}
	// AND THE ACCOUNT ARRIVED AND DESCRIBES WHAT IS THERE.
	//
	// Part of reading the destination back, not a step afterwards: the original is about
	// to be deleted, and it is the only other copy of the step list. Asked through
	// `Steps`, which re-reads the destination's source and demonstration and answers
	// whether the account still describes them — so this is the same question a later
	// edit will ask, rather than a weaker one taken here.
	if m.fromAccount {
		if _, st := d.Reg.Steps(m.To); !st.Usable() {
			return Rescoped{To: m.To, Says: copiedSays(m.Name, m.TakenID,
				"the steps beside "+m.TakenID+" are not an account of what "+
					"this move wrote, so the original has not been "+
					"touched — look at both before deleting either")}, nil
		}
	}

	// ── AND THE ORIGINAL IS STILL THE VERSION THAT WAS REVIEWED ──
	//
	// NEVER DELETE A NEWER VERSION. Both locks have been held since the first
	// revalidation, so nothing taking them could have written here — and this asks anyway,
	// because the thing it protects against is the writer that does NOT take the lock: a
	// hand edit in `marco edit`, a text editor, a sync. Deleting that is deleting work
	// nobody reviewed.
	againSrc, againRec := d.versionAt(m.From)
	againSteps, againReadable := d.Reg.StepsDigest(m.From)
	if againSrc != m.fromSourceAt || againRec != m.fromRecordingAt ||
		!againReadable || againSteps != m.fromStepsAt {
		return Rescoped{To: m.To, Says: copiedSays(m.Name, m.TakenID,
			"the original changed while this was being written, so it has been "+
				"left where it is")}, nil
	}
	// ── THE SOURCE FIRST, AND NOTHING BESIDE IT UNTIL THAT HAS SUCCEEDED ──
	//
	// NOT `Registry.Delete`, which removes the demonstration and the anchor images FIRST
	// and the source last. That order is the ordinary one for a deletion — everything is
	// going — though not a safe one even there, and it is exactly wrong here, because this
	// removal is allowed to fail and the promise when it does is
	// that the original is RETAINED. Through `Delete`, a source removal that failed
	// (Windows holding the file open, a permission, a sync) would have left a routine
	// whose recording and images were already gone: a routine that can no longer be
	// reviewed, rebuilt or moved, and with anchors one that can no longer run. "Original
	// retained" would have been a sentence about something that was not there any more.
	//
	// A routine IS its `.marco`: while that is there the routine is there, so removing it
	// is what makes this a move, and everything else is litter afterwards.
	if err := removeSource(d.Reg, m.From); err != nil {
		return Rescoped{To: m.To, Says: copiedSays(m.Name, m.TakenID,
			fmt.Sprintf("the original could not be removed (%v), and nothing "+
				"beside it was touched", err))}, nil
	}
	// ── AND ONLY NOW WHAT SAT BESIDE IT ──
	//
	// The routine is gone: nothing resolves it, nothing can play it. What is left cannot
	// fail the move, and is still worth saying — a recording left under a slug is a file
	// the next routine saved there would inherit.
	residue := d.Reg.DeleteLeftovers(m.From)
	// AND THE PROVENANCE SIDECAR, which neither of those takes.
	//
	// It is not copied to the destination: it describes bytes that no longer exist there,
	// and the destination is a fresh rendering. Left behind it would sit under that slug
	// waiting to be inherited by the next unrelated routine saved there — the same orphan
	// `Registry.Rename` carries its sidecar to avoid.
	if err := d.Reg.DeleteOrigin(m.From); err != nil {
		residue = append(residue, fmt.Sprintf("its provenance (%v)", err))
	}
	mlog.Info("rescope: moved", "route", m.Name, "from", RouteID(m.From),
		"to", RouteID(m.To), "scope", string(m.Want),
		"replaced", consent.Given(), "residue", len(residue))
	// AND WHAT IS LEFT IS SAID IN ITS OWN SENTENCE, not folded into the outcome. See
	// Rescoped.Left.
	var left string
	if len(residue) > 0 {
		left = fmt.Sprintf("What was beside it at %s could not be removed: %s.",
			RouteID(m.From), strings.Join(residue, "; "))
	}
	return Rescoped{To: m.To, Removed: true, Residue: residue, Left: left,
		Says: fmt.Sprintf("Moved %q to %s.", m.Name, RouteID(m.To))}, nil
}

// removeSource is the one removal a move's promise depends on.
//
// A variable so a test can make it fail. Nothing else replaces it, and the claim it exists for —
// that a failed source removal leaves the demonstration and the images exactly where they are —
// cannot be made to happen on a real filesystem on purpose without arranging a locked file, a
// permission or a sync, each of which is a different answer on each platform. The same seam the
// guarded-press boundary uses for `windowFacts`, for the same reason. See
// TestAMoveThatCannotRemoveTheOriginalLeavesItWhole.
var removeSource = func(reg routes.Registry, rt routes.Route) error {
	return reg.DeleteSource(rt)
}

// lockOrder is which of two routes is locked first, and it never depends on which end of the move
// they are.
//
// TWO MOVES IN OPPOSITE DIRECTIONS IS THE CASE. One window moving `settings` from in-place to
// anywhere while another moves it back takes the same two locks in opposite orders, and each
// waits for the one the other holds — a deadlock bounded only by `SaveWithin`, after which both
// fail and neither has done anything. Ordering the acquisitions by the identities themselves
// makes it impossible rather than unlikely: whatever the two moves are, they ask for the same
// lock first.
//
// A pure function so the rule can be read back from both input orders, which is the whole of the
// claim and is not visible in a test that only runs one move. See
// TestBothEndsOfAMoveAreLockedInTheSameOrderWhicheverWayItGoes.
func lockOrder(from, to routes.Route) (first, second routes.Route) {
	if RouteID(to) < RouteID(from) {
		return to, from
	}
	return from, to
}

// copiedSays is the sentence for the one outcome that is not a move.
//
// `Copied; original retained` and never `Moved`: there are two routines now, and a person told
// otherwise goes looking for one of them. The reason comes with it, because every one of these
// is something they can act on — and the destination is named, because the thing they have to
// look at is the one they cannot see from where they are.
func copiedSays(name, toID, why string) string {
	return fmt.Sprintf("Copied %q to %s; original retained: %s.", name, toID, why)
}

// sealed is this preparation with its token, which is the last thing done to it.
//
// EVERY EXIT GOES THROUGH HERE, including the refusals — a refused move is still shown, and a
// `Rescoping` without a token is one `Rescope` will not act on at all.
//
// WHAT THE TOKEN BINDS is everything the review showed and everything the move would touch: the
// original's source and demonstration, the exact source that would be written, the destination's
// whole identity and what was at it, the library, and the scope asked for. A `Rescoping` that
// differs from the one prepared in ANY of those no longer matches its own token — so a caller
// cannot hand back a value assembled from parts of two reviews, and a move cannot write bytes
// nobody was shown.
func (m Rescoping) sealed() Rescoping {
	m.Token = routes.DigestOf(join(
		m.fromSourceAt, m.fromRecordingAt, m.fromStepsAt,
		fmt.Sprint(m.fromAccount),
		routes.DigestOf(m.Now), m.Refused,
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
		routes.DigestOf(m.Was),
		m.toSourceAt, m.toRecordingAt,
		m.From.Slug, m.From.App, fmt.Sprint(m.From.Focus),
		m.To.Slug, m.To.App, fmt.Sprint(m.To.Focus),
		m.library, string(m.Want),
	))
	return m
}

// join is the token's fields with a separator no path, digest or scope word contains, so no two
// fields can run together into a third.
func join(parts ...string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "\x00"
		}
		out += p
	}
	return out
}

// NameOf is what a saved routine calls itself, read off the file.
//
// Falls back to the slug when the file cannot be read, because this is only ever used to put a
// name in a sentence and a refusal that cannot name what it refused is worse than an approximate
// one.
func NameOf(reg routes.Registry, rt routes.Route) string {
	raw, err := os.ReadFile(reg.Path(rt))
	if err != nil {
		return rt.Slug
	}
	return NameIn(string(raw), rt.Slug)
}
