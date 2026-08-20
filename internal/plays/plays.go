// Package plays is the Audience's account of everything Marco can do.
//
// # Why a package, and why this word
//
// A PLAY is a durable behaviour Marco can perform: legal Core Marco on disk, with a past. That is
// the noun a person deals in. [[internal/routes]] is where one is STORED — a directory tree,
// slugs, scopes and sidecars — and it keeps its name, because a package name is not a product word
// and renaming it would be a very large diff for a reader who never sees it. This package is the
// seam where the two vocabularies are allowed to differ, and a package boundary is the honest
// place to put that. See [[34F-legacy-marco-product-audit]] §8 and §18.
//
// # It computes no second account of anything
//
// Every field below is a rendering of something `internal/routes` already knows, and each says
// which. In particular KIND and PROVENANCE come from `routes.Registry.KindOf`, the same call
// `orchestrator.Classify` makes on the way to the authority door — so a surface cannot call a play
// learned that the door calls authored.
//
// # Read-only
//
// Listing plays touches no file. Nothing here creates a directory, writes a file or moves one;
// see the note on List, which names the functions that may never appear below it.
package plays

import (
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Scope is where a play applies, in the three words the product already uses.
//
// The words are good — [[34F-legacy-marco-product-audit]] §18 says so — and they are kept.
type Scope string

const (
	// ScopeGlobal is app-less: it answers anywhere and switches nothing.
	ScopeGlobal Scope = "global"
	// ScopeContext answers only while its application is already in front.
	ScopeContext Scope = "context"
	// ScopeFocus answers from anywhere AND brings its application forward first.
	//
	// The most-liked behaviour in the product and the one with no other name. It must stay
	// distinguishable from context wherever a play is shown.
	ScopeFocus Scope = "focus"
)

// ScopeOf names a play's scope.
//
// App-less FIRST, then Focus — the same order `routes.Registry.dir` uses to decide which folder
// the file goes in, so the label can never disagree with the location. The copy this replaces in
// cmd/marco/edit.go tested Focus first, and would have called an app-less play "focus" the moment
// a Route was built from user input.
//
// Deleting the App arm must fail TestAnAppLessPlayIsGlobalWhateverItsFocusBit.
func ScopeOf(rt routes.Route) Scope {
	switch {
	case rt.App == "":
		return ScopeGlobal
	case rt.Focus:
		return ScopeFocus
	default:
		return ScopeContext
	}
}

// Word is the scope as a badge says it.
func (s Scope) Word() string {
	switch s {
	case ScopeFocus:
		return "From anywhere"
	case ScopeContext:
		return "In the app"
	default:
		return "Anywhere"
	}
}

// Says is the scope as a sentence, naming the application when there is one.
//
// FOCUS SAYS WHAT IT DOES. "From anywhere" alone is the same words a global play would use, and
// the difference — Marco brings the application forward first — is the capability worth having.
//
// Deleting the focus arm, or collapsing it into the context arm, must fail
// TestFocusIsPresentedDistinctlyFromContext.
func (s Scope) Says(app string) string {
	switch s {
	case ScopeFocus:
		if app == "" {
			return "You can ask for it from anywhere; Marco brings its application forward first."
		}
		return "You can ask for it from anywhere — Marco brings " + app + " forward first."
	case ScopeContext:
		if app == "" {
			return "Only while its application is already in front."
		}
		return "Only while " + app + " is already in front."
	default:
		return "Anywhere. It does not switch applications."
	}
}

// Life is where a play stands: whether anything can ask for it, and whether Marco can still vouch
// for where it came from.
//
// # Why this is not a boolean
//
// Because SAVED and REGISTERED are different facts — a play can be written down, readable and
// editable, and still be something nothing can ask for — and because a registered play's past can
// stop describing it. Five states, and each of them is reachable; see LifeOf.
type Life string

const (
	// LifeReady is registered, with provenance that is either absent or still intact.
	LifeReady Life = "ready"
	// LifeEdited is registered, and the file has changed since Marco wrote it down.
	//
	// NOT damage. The person was invited to edit their play and did. It still runs; it is
	// simply no longer the artifact Director verified.
	LifeEdited Life = "edited"
	// LifeUnverified is registered, with a past this version of Marco cannot read.
	LifeUnverified Life = "unverified"
	// LifeSaved is written down and not yet askable — the staging state, and the one with a
	// button beside it.
	LifeSaved Life = "saved"
	// LifeStuck is written down, not askable, and not registerable as learned either, because
	// its provenance no longer describes it.
	LifeStuck Life = "stuck"
)

// LifeOf reads a play's standing from the two facts that decide it.
//
// # On OriginOrphaned
//
// It cannot arrive here. Both enumerations start from `.marco` files, so `Origin` always finds the
// source it was asked about, and orphaned means the source is gone. It is folded into the
// unreadable arms rather than given a product sentence of its own: a state a person can never be
// shown does not get words, and inventing them would be a category dead on arrival.
//
// Deleting the `registered` parameter — deciding standing from provenance alone — must fail
// TestAStagedPlayIsNeverReady.
func LifeOf(registered bool, state routes.OriginState) Life {
	if registered {
		switch state {
		case routes.OriginEdited:
			return LifeEdited
		case routes.OriginUnreadable, routes.OriginOrphaned:
			return LifeUnverified
		default: // OriginNone (an ordinary authored play) or OriginIntact
			return LifeReady
		}
	}
	// STAGED. `routes.Registry.Register` refuses anything whose staged provenance is not
	// verified, so a staged play whose file has been edited is not merely unregistered — it is
	// unregisterable as learned, and offering a Register button beside it would be an offer
	// Marco cannot keep.
	if state == routes.OriginIntact {
		return LifeSaved
	}
	return LifeStuck
}

// Askable reports whether anything can ask for this play by name.
//
// The one question the whole staging design exists to answer, and it is answered by which
// enumeration the row came from — never by a flag on disk that could disagree with the filesystem.
func (l Life) Askable() bool { return l == LifeReady || l == LifeEdited || l == LifeUnverified }

// Registerable reports whether offering a Register button here is an offer Marco can keep.
func (l Life) Registerable() bool { return l == LifeSaved }

// Word is the standing as a badge says it.
//
// Deleting the LifeSaved arm — or returning "Ready" from it — must fail
// TestNoSurfaceCallsAStagedPlayReady.
func (l Life) Word() string {
	switch l {
	case LifeReady:
		return "Ready"
	case LifeEdited:
		return "Ready · edited"
	case LifeUnverified:
		return "Needs attention"
	case LifeSaved:
		return "Saved — not askable yet"
	case LifeStuck:
		return "Saved — cannot be registered"
	}
	return string(l)
}

// Says is the standing as a sentence.
func (l Life) Says() string {
	switch l {
	case LifeReady:
		return "You can ask for this play."
	case LifeEdited:
		return "You can ask for this play. You have edited it since Marco wrote it down, so it " +
			"is now your play rather than the one Marco verified."
	case LifeUnverified:
		return "You can ask for this play, but Marco cannot read where it came from."
	case LifeSaved:
		return "Saved. It is a file you can read and edit — nothing can ask for it until you " +
			"register it."
	case LifeStuck:
		return "Saved, and it cannot be registered as learned: the file has changed since Marco " +
			"wrote down where it came from."
	}
	return ""
}

// AfterLearn is where a play stands the moment Learn finishes with it.
//
// # Why Learn does not get its own sentence
//
// Because the two surfaces were telling different stories about the same file. Learn used to say
// "Saved. It is in the Routes tab." on the strength of a play existing on disk, while the tab it
// named scans four directories that structurally cannot contain a staged play — a claim about
// DISCOVERY made from a fact about STORAGE. Phase 0 split the claim in two; this makes the two
// surfaces read the same words out of the same function, so a future edit cannot re-open the gap
// by changing one of them.
//
// `registered` is the same fact the Plays listing calls Registered: not a flag anybody set, but
// whether the resolver can reach the file.
//
// Deleting this — and giving the Learn panel its own words — must fail
// TestLearnAndThePlaysListingSayTheSameThing.
func AfterLearn(saved, registered bool) (Life, bool) {
	if !saved {
		return "", false
	}
	if registered {
		// Written down, moved where the resolver looks, and its provenance was written
		// beside it in the same breath — the ordinary happy path, and the only one that may
		// say Ready.
		return LifeReady, true
	}
	// SAVED AND NOT ASKABLE is a real state with a real cause — a name already taken is the
	// ordinary one — and it gets the staged sentence, not a softened version of the happy one.
	return LifeSaved, true
}

// KindWord is where a play came from, in the product's words.
//
// # Recorded, not taught
//
// `routes.KindTaught` is a play generated from a demonstration, and the obvious label for it is
// taken: [[Glossary]] and [[ADR-048-learn-teach-and-do-are-three-different-sentences]] reserve
// TEACH for Marco guiding the person, which is the opposite direction. "Recorded" says what
// actually happened and collides with nothing.
//
// Mapping learned to authored — or dropping the distinction — must fail
// TestEveryKindIsPresentedAsItself.
func KindWord(k routes.Kind) string {
	switch k {
	case routes.KindLearned:
		return "Learned"
	case routes.KindTaught:
		return "Recorded"
	default:
		return "Authored"
	}
}

// KindSays is where a play came from, as a sentence.
func KindSays(k routes.Kind) string {
	switch k {
	case routes.KindLearned:
		return "Marco watched you, worked out how to do this, and wrote it down."
	case routes.KindTaught:
		return "You demonstrated this once and Marco kept the recording."
	default:
		return "Written by hand."
	}
}

// Play is one durable behaviour, as a person is shown it.
type Play struct {
	// Name is the play as the Audience says it: the slug with its dashes back as spaces.
	Name string
	// Slug is the play as it sits on disk, and the ONLY safe handle for an action.
	//
	// Carried separately from Name because a staged play is registered BY SLUG, and the slug
	// it was saved under came from the phrase the Audience used rather than from anything a
	// listing can re-derive. A button that posts the display name targets a different play.
	Slug string
	// Application is the program the play is about, empty for an app-less play.
	Application string
	// Kind is where it came from: authored, taught or learned. From routes.Registry.KindOf,
	// which is also what the authority door reads.
	Kind routes.Kind
	// Provenance is the state of its origin record, unabridged. Surfaces render Life; this is
	// here so an Advanced view has it without a second read.
	Provenance routes.OriginState
	// Scope is where it applies.
	Scope Scope
	// Registered is whether the resolver can see it.
	//
	// Not a flag on disk and not read as one: it is which enumeration this row came out of.
	// `routes.Registry.List` sees only what the resolver sees, `ListStaged` only what it
	// cannot, so a Registered that disagreed with the filesystem is unrepresentable.
	Registered bool
	// Life is the standing a surface shows, from Registered and Provenance together.
	Life Life
	// Activates is the application this play brings forward before it runs, or empty.
	//
	// The same fact as Scope == ScopeFocus, named separately on purpose: it is the capability
	// worth having, and a view that shortened the scope column to a badge would drop it.
	Activates string
	// Route is the store's handle for the actions a surface offers.
	//
	// For a STAGED play the Focus bit is `routes.LearnedFocus` — the intention registration
	// will act on, not something read from disk — so this handle is good for Register,
	// StagedSource and StagedOrigin, and for nothing that asks about the registered location.
	Route routes.Route
}

// List is every play Marco has: the ones anything can ask for, and the ones still waiting.
//
// # Two enumerations, deliberately
//
// `reg.List` is what the resolver can see. `reg.ListStaged` is what it structurally cannot. They
// are joined HERE, in a projection, and nowhere lower down — widening `reg.List` to include the
// staging directory would make every staged play answerable from every application, because
// `Resolve` walks `List`.
//
// Deleting the ListStaged half must fail TestStagedPlaysAreListedAndStayUnresolvable.
//
// # Read-only, and this is the property to protect
//
// Every call below is os.ReadDir, os.ReadFile or os.Stat. The temptations are adjacent and real:
// `routes.Registry.ScopeDir` reads like a writer and is not, while Save, SaveRecording,
// SaveStaged, WriteAssets and writeAtomic all begin with os.MkdirAll and none of them may ever
// appear on this path.
//
// Adding any writer must fail TestBrowsingPlaysChangesNothingOnDisk.
func List(reg routes.Registry) []Play {
	return append(Registered(reg), Staged(reg)...)
}

// Registered is only the plays anything can ask for — `reg.List`, projected.
//
// Kept separate so a caller that must not offer an unaskable name (name completion, the
// compatibility CLI) can say so in one word rather than filtering a mixed list correctly.
func Registered(reg routes.Registry) []Play {
	list := reg.List()
	out := make([]Play, 0, len(list))
	for _, rt := range list {
		kind, state := reg.KindOf(rt)
		out = append(out, of(rt, taughtIfRecorded(reg, rt, kind, state), state, true))
	}
	return out
}

// Staged is only the plays that are saved and not yet askable.
//
// Its own call because the product shows them as their own group, with their own action — a list
// that mixed them into the things that work would be telling somebody a play is available when
// asking for it will not find it.
func Staged(reg routes.Registry) []Play {
	staged := reg.ListStaged()
	out := make([]Play, 0, len(staged))
	for _, rt := range staged {
		// KindOfStaged, never KindOf: the sidecar is in `learned/`, and asking about the
		// registered location would report a learned play as somebody's own writing.
		//
		// Deleting the Staged suffix must fail TestAStagedPlayIsNotReportedAsAuthored.
		kind, state := reg.KindOfStaged(rt)
		out = append(out, of(rt, kind, state, false))
	}
	return out
}

// Find returns one play by the handle a surface holds, so an action acts on the row the same
// listing produced.
//
// The SLUG identifies it, never the display name — see Play.Slug for why that distinction is
// load-bearing for the Register button.
func Find(reg routes.Registry, slug, app string, staged bool) (Play, bool) {
	list := Registered(reg)
	if staged {
		list = Staged(reg)
	}
	for _, p := range list {
		if p.Slug == slug && p.Application == app {
			return p, true
		}
	}
	return Play{}, false
}

// of assembles one row. No I/O of its own — every fact is already decided by its caller.
func of(rt routes.Route, kind routes.Kind, state routes.OriginState, registered bool) Play {
	sc := ScopeOf(rt)
	p := Play{
		Name: Pretty(rt.Slug), Slug: rt.Slug, Application: rt.App,
		Kind: kind, Provenance: state, Scope: sc,
		Registered: registered, Life: LifeOf(registered, state), Route: rt,
	}
	if sc == ScopeFocus {
		p.Activates = rt.App
	}
	return p
}

// taughtIfRecorded refines authored to recorded when the demonstration is still beside the play.
//
// # Why the sidecar cannot answer this
//
// Because nothing writes `routes.KindTaught`. The only Kind any production code assigns is
// `KindLearned`; a demonstrated play gets no sidecar at all, so its provenance reads `authored` and
// it would list as somebody's own writing. The `.rec.json` beside it is the only durable evidence
// that a person demonstrated it rather than typed it.
//
// # And why the inference only runs one way
//
// A recording present means recorded. A recording ABSENT means nothing — a play demonstrated
// before recordings were kept has none, and so does a hand-written one. So this promotes and never
// demotes, and it never touches a play whose provenance already spoke.
//
// It is DISPLAY, and it is here rather than in `KindOf` on purpose: `KindOf` feeds the authority
// door, which treats authored and taught identically, and a listing has no business changing what
// that door sees.
//
// Deleting this must fail TestARecordedPlayIsNotShownAsAuthored.
func taughtIfRecorded(reg routes.Registry, rt routes.Route, kind routes.Kind,
	state routes.OriginState) routes.Kind {

	if kind == routes.KindAuthored && state == routes.OriginNone && reg.HasRecording(rt) {
		return routes.KindTaught
	}
	return kind
}

// Pretty is a slug as a person says it.
func Pretty(slug string) string { return strings.ReplaceAll(slug, "-", " ") }
