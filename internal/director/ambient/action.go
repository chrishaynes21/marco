package ambient

import (
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// What the person DID, and the one word of theirs it is allowed to keep.
//
// # The boundary this file moves, deliberately
//
// 36A's buffer held ids, counts, times and a two-word provenance vocabulary — no labels, no
// titles, no screen text, nothing anybody could read. That was the right boundary for a buffer
// whose only job was "where are you", and it is not sufficient for "learn what I just did": a
// route reconstructed from it would know where somebody went and not what they pressed to get
// there, which is the difference between a fact and a play.
//
// So exactly one kind of word crosses now, and it is the narrowest one available:
//
//	the name of the ONE control a person's own action landed on.
//
// Not the screen's other controls, not the window title, not a line of text, not what anybody
// typed. The same shape as [[ADR-047-a-place-is-remembered-a-meaning-is-answered]] — an explicit human
// semantic event licenses retaining the one identity it is about, and nothing else that was on
// screen at the time.
//
// # And the licence that bounds which words those can be
//
// A control's label reaches an input event only through observe.AdmittedTargetLabel, which has
// two stages: a canonical role allowlist that stands whatever anybody declared
// (directorapi.ElementRole.NameablePlaintext — button, menu item, menu, tab, checkbox, radio),
// and a WIDER stage that any clickable role passes and that only an explicit Learn licence opens.
//
// Ambient watching holds the zero licence, so only the first stage is ever in play for it. A list
// item, a link, an icon and a text field carry no name into this buffer, which is where a
// document title, a contact and a chat line would otherwise arrive. That is not a promise made
// here — it is a permission that does not exist in the object, decided one layer up in the
// sampler and unchanged by this file.

// ActionKind is what somebody did, from a closed vocabulary.
//
// Meanings rather than mechanics, and the vocabulary is short on purpose. "They activated the
// control called Mouse" is a thing a play can be built from; "they moved the pointer to 742,318
// and pressed the left button" is a recording of an afternoon. Where those two describe the same
// event, this keeps the first.
type ActionKind string

const (
	// Activate is somebody pressing a control — with the pointer, or with the keyboard on
	// whatever held its attention. ONE kind for both, because the semantic result is the
	// same and the modality is not part of what was learned; see the note on Act.
	Activate ActionKind = "activate"
	// Back is leaving, however it was done.
	Back ActionKind = "back"
	// Menu is opening or closing an application's own menu.
	Menu ActionKind = "menu"
)

var actionKinds = []ActionKind{Activate, Back, Menu}

// Known reports whether a kind is in the closed vocabulary. Everything entering is checked
// against it, so a caller that invented a word finds it dropped rather than carried.
func (k ActionKind) Known() bool {
	for _, known := range actionKinds {
		if known == k {
			return true
		}
	}
	return false
}

// ActionKinds returns the vocabulary, for diagnostics and documentation.
func ActionKinds() []ActionKind { return append([]ActionKind{}, actionKinds...) }

// Target is the semantic identity of the thing an action was aimed at.
//
// A ROLE and a NAME, never a position. The whole point of resolving a press against the
// accessibility tree at the moment it happens is that "the button called Mouse" survives a window
// moving, a resolution change and a reflow, and (742, 318) survives none of them. A coordinate
// may exist transiently inside the perception layer as the evidence that produced this; it does
// not exist here, and there is no field it could arrive in.
type Target struct {
	// Role is the control's kind, from the shape-checked structural vocabulary.
	Role string
	// Label is the control's own name, when it was admitted; empty when it was withheld.
	//
	// Withheld is the ORDINARY case and never a failure — see the licence note at the top
	// of this file. An act with a role and no name is still evidence that somebody pressed
	// something of that sort, and it is not evidence a route can be built from.
	Label string
}

// Named reports whether the target can be said out loud.
func (t Target) Named() bool { return t.Label != "" }

// Empty reports whether nothing at all was resolved.
func (t Target) Empty() bool { return t.Role == "" && t.Label == "" }

// Act is one semantic action.
//
// # Why there is no modality here
//
// Because a demonstration teaches INTENT, and the physical means is the demonstrator's habit
// rather than part of what was shown. Somebody who reaches the Mouse page by clicking it and
// somebody who reaches it by pressing Down twice and Enter have demonstrated the same thing, and
// a route that recorded which would replay one person's hands rather than either person's
// intention. When Marco later walks this, the Actor chooses whatever legal operation the target
// affords.
//
// Where modality genuinely changes the meaning — a drag, a scroll that is not merely how the
// target was brought into view — the honest answer today is that this vocabulary cannot say it,
// and selection refuses rather than flattening it into an activation. See ADR-094.
type Act struct {
	Kind   ActionKind
	Target Target
}

// Representable reports whether this act says enough to become a step of a play.
//
// An activation needs to know WHAT was activated: "they pressed something" cannot be walked.
// Leaving and opening a menu need no target — they are about the place, not about a control in
// it — so they are representable on their own.
func (a Act) Representable() bool {
	switch a.Kind {
	case Activate:
		return a.Target.Named()
	case Back, Menu:
		return true
	}
	return false
}

// Step is one leg of the recent trail: where somebody was, what they did there, and where it left
// them.
//
// # This is the shape "learn what I just did" reads
//
// 36A's tail held From and To, which answers "where have you been" and cannot answer "how". A
// step holds the middle of that sentence, so a walk through the trail reconstructs
//
//	place -> action on a target -> place -> action on a target -> place
//
// and a route can be built from it without a recording, a coordinate or a replay.
type Step struct {
	// Order is a monotonic sequence over the whole watching session, and it is what a
	// promotion watermark is expressed in. It survives eviction from the trail — the tail
	// forgets steps, and the numbers it forgot are never reissued — so "everything up to
	// here has already been learned" stays true after the evidence behind it is gone.
	Order int
	// From and To are the durable subject ids either side. Empty means the screen was not
	// recognised; see Shape for what is held about it instead.
	From, To string
	// FromShape and ToShape describe a screen Marco does not yet recognise, so an explicit
	// Learn may later establish it. Nil for a place already known, which needs no
	// description because it already has an identity.
	FromShape, ToShape *Shape
	Application        string
	// Did is what the person did to get from one to the other, in order.
	Did []Act
	// By is whose action this was. Load-bearing: a transition Marco performed is not
	// something the person demonstrated, and a promotion that confused them would teach
	// Marco its own behaviour back from itself.
	By Source
	// Bridged counts screens crossed on the way that were never recognised — a page part-way
	// through arriving, a spinner, a frame. Reported rather than hidden, because a leg that
	// crossed four of them is a leg whose middle was genuinely not seen.
	Bridged int
	// Settled counts readings between the action and the destination resolving. It is how a
	// long load is told from an instant change, and it is deliberately NOT a timeout — see
	// the retention note in ADR-094.
	Settled int
	At      time.Time
}

// Shape is the transient structural description of a screen Marco does not recognise.
//
// # Why the buffer holds this at all
//
// Because a demonstration through screens Marco has never seen is the ordinary case the first
// time anybody uses a program, and a buffer that held nothing about them could only ever learn
// routes between places that were already durable. So the trail keeps enough for an EXPLICIT
// Learn to establish them later — transiently, under no licence, exactly like everything else
// here.
//
// It is a description, not a persistence. Nothing writes it anywhere; watching stopping forgets
// it with the rest.
//
// # Why it carries the signature whole
//
// Because the signature IS the identity: a Place is established under it, and recalled by matching
// against it. Narrowing it here and rebuilding it at promotion time would establish a place from a
// value that is not quite the one that was seen — a near-duplicate of a screen Marco was about to
// recognise anyway, minted by a lossy conversion. That is exactly the growth Part 28 of the
// roadmap is about, and the way to not have it is to not re-derive the value.
//
// # And what a screen's signature actually contains
//
// Structural role counts, how many members, a normalised window-relative envelope, and the closed
// generic interface vocabulary. No raw text, no absolute coordinate, no title, no screenshot. The
// type also has a Label, a Kind and a Place — those belong to a TARGET's identity and are empty
// for a screen, which is a fact about the type rather than a promise made here.
//
// Deleting the screen check in TestATransientShapeCarriesNoWordsAnybodyCouldRead is what would
// let that stop being true quietly.
type Shape struct {
	// Signature is the durable identity-bearing description, exactly as perception produced
	// it. It is what an explicit Learn hands to the place store.
	Signature observe.StructureSignature
	// Called is what the screen APPEARS to be called, when a name settled by recurrence.
	//
	// # Why a word off somebody's screen is allowed here
	//
	// Because it is one somebody's own interface put in front of them as the name of where
	// they are, and because passive observation already reads it: `AdmittedPlaceName` is
	// unconditional and applies the same shape filter every other admitted text passes, so
	// this is not a widening of what Marco perceives. It is the same value a licensed
	// session would write, held transiently until an explicit Learn writes it.
	//
	// # And why it has to be here at all
	//
	// A Place with no name cannot be lowered into a play — `JudgeLowering` refuses
	// `screen_unnamed` and the lifecycle asks the person what to call it. On a retrospective
	// Learn that question is exactly the interruption Part 25 of the roadmap forbids: the
	// person is not demonstrating anything, they said one sentence, and being asked to name
	// three screens they walked past ten minutes ago is worse than the repeat demonstration
	// this was meant to replace.
	//
	// Empty is ordinary and is not a failure: a screen whose name never settled is
	// established without one, exactly as a live Learn would.
	Called string
	// Local is the session-local state this shape was read from. A counter, useful only for
	// matching a shape back to the reading that produced it, and never durable identity.
	Local string
}

// Clone copies a shape, so a snapshot cannot be changed under a reader.
//
// The signature's maps and slices are copied too: a View is a snapshot, and aliasing the live
// value would let a reader's answer change while it reads.
func (s *Shape) Clone() *Shape {
	if s == nil {
		return nil
	}
	out := &Shape{Signature: s.Signature, Called: s.Called, Local: s.Local}
	if s.Signature.Roles != nil {
		out.Signature.Roles = make(map[string]int, len(s.Signature.Roles))
		for k, v := range s.Signature.Roles {
			out.Signature.Roles[k] = v
		}
	}
	out.Signature.Terms = append([]observe.InterfaceTerm{}, s.Signature.Terms...)
	return out
}

// TransientPrefix marks a key that names a screen Marco does not recognise.
//
// # Why the prefix exists rather than an extra field
//
// Because these keys travel through everything a durable subject id travels through — the trail,
// selection, a step's endpoints — and the one thing that must never happen is one being mistaken
// for the other. A subject id is a durable identity that outlives the session; this is a
// session-local counter that means nothing outside the watching session that issued it. A
// separate boolean would be a second thing to keep in agreement with the string, and it would be
// the boolean that got dropped.
const TransientPrefix = "seen_"

// TransientKey names a screen Marco does not recognise, for the length of one watching session.
func TransientKey(local string) string {
	if local == "" {
		return ""
	}
	return TransientPrefix + local
}

// Recognised reports whether a key is a durable subject id rather than a transient name.
func Recognised(key string) bool {
	return key != "" && !hasPrefix(key, TransientPrefix)
}

func hasPrefix(s, p string) bool { return len(s) >= len(p) && s[:len(p)] == p }

// Placed reports whether this step knows both endpoints well enough to be promoted — either as an
// already-durable subject, or as a describable screen an explicit Learn could establish.
//
// A transient key with no shape is NEITHER, and that is the case this exists for: the observer
// saw somebody move to a screen it could not recognise and could not describe well enough for
// anything to establish. There is a name for it in the trail so the walk stays a walk, and there
// is nothing behind the name.
func (s Step) Placed() bool {
	return placedEnd(s.From, s.FromShape) && placedEnd(s.To, s.ToShape)
}

func placedEnd(key string, shape *Shape) bool {
	if Recognised(key) {
		return true
	}
	return key != "" && shape != nil
}

// Representable reports whether the step says what was done, not merely that something was.
func (s Step) Representable() bool {
	if len(s.Did) == 0 {
		return false
	}
	for _, a := range s.Did {
		if !a.Representable() {
			return false
		}
	}
	return true
}
