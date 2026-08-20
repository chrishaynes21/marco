package observe

// Watching what the player did, without watching what the player typed.
//
// # Why this exists
//
// Screen-state segmentation can already say that the composition changed and came back:
// `state_1 → state_2 ×4`. It cannot say what made it change. Correlating those transitions with
// the player's own navigation is what turns a list of screens into a graph with edges, and a
// graph with edges is the first thing that could be learned from rather than merely reported.
//
// # Why the vocabulary is closed and has no string in it
//
// A keyboard observer is the highest-privacy surface in the system, and a promise not to record
// text is worth less than an inability to. There is NO free-form field on InputEvent — not a
// key name, not a label, not a note. The intent vocabulary is a closed set of navigation
// meanings, so the type cannot express "the user typed p" whatever a future caller wants, and a
// platform layer that wanted to leak a character would have to add a field to do it.
//
// This mirrors the two-stage structural classifier the vision path uses: decide by ROLE what
// may cross the boundary, and let the shape of the type enforce the decision rather than the
// diligence of each caller.
//
// # What this is not
//
// Not execution, and not a step toward it. Observing that the player pressed "down" gives the
// Director no way to press it — see [[ADR-010-passive-observation-cannot-execute]], which this
// does not weaken: it adds an observation path, and observation paths are what that ADR is
// written to protect. Nothing here reaches World State, fusion, planning or policy.

// NavIntent is a navigation meaning, from a closed vocabulary.
//
// Meanings rather than keys, on purpose. "The player moved down" is what a state transition can
// be correlated with; which physical key produced it is a binding detail that differs per game,
// per controller and per player, and recording it would be recording the keyboard.
type NavIntent string

const (
	NavUp      NavIntent = "up"
	NavDown    NavIntent = "down"
	NavLeft    NavIntent = "left"
	NavRight   NavIntent = "right"
	NavConfirm NavIntent = "confirm"
	NavBack    NavIntent = "back"
	// NavPause is the intent that opens or closes an application's own menu.
	NavPause NavIntent = "pause"
	// NavPoint is a pointer press. Its position rides in PointerAt, normalised and
	// window-relative like every other coordinate here.
	NavPoint NavIntent = "point"
)

var navIntents = []NavIntent{
	NavUp, NavDown, NavLeft, NavRight, NavConfirm, NavBack, NavPause, NavPoint,
}

// Known reports whether an intent is in the closed vocabulary.
//
// Everything that enters is checked against this. An unrecognised intent is DROPPED rather
// than carried: a platform layer that invented a word would otherwise have found a way to put
// arbitrary text into a privacy-bounded record.
func (n NavIntent) Known() bool {
	for _, k := range navIntents {
		if k == n {
			return true
		}
	}
	return false
}

// NavIntents returns the vocabulary.
func NavIntents() []NavIntent { return append([]NavIntent{}, navIntents...) }

// PointerAt is a normalised, window-relative pointer position.
type PointerAt struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// SemanticTarget is what a placed input event was aimed AT, when the evidence at the time
// identified it.
//
// # The enrichment, and the boundary it deliberately crosses
//
// A click at (0.42, 0.55) is evidence that the person clicked somewhere; a click on the
// list item called "Mouse" is evidence of what they MEANT. Resolving the second from the
// first is what turns a mouse-driven demonstration from an unlearnable coordinate into a
// semantic action — and it is an ENRICHMENT: the raw event survives whether or not this was
// ever filled in, and a target is never a prerequisite for capture.
//
// The Label is the one place observed interface text rides on an input event, and it is
// admitted under exactly two licences, both enforced where the evidence is pushed (the
// composition root) rather than promised here:
//
//   - a role the canonical plaintext allowlist already vouches for
//     (directorapi.ElementRole.NameablePlaintext) — a button's name is a fact about the
//     interface;
//   - during an explicit Learn demonstration, the control the person actually activated —
//     the same shape as [[ADR-047]]: an explicit human semantic event licenses retaining
//     the one identity it is about, and nothing else on the screen.
//
// Either way the text passes the standing shape filter before it may cross. A target that
// failed both keeps its Role and carries no Label, which is still more than a coordinate.
type SemanticTarget struct {
	// Role is the control's kind, shape-checked like every structural role.
	Role string `json:"role,omitempty"`
	// Label is the control's own name, when admitted; empty when withheld.
	Label string `json:"label,omitempty"`
}

// Named reports whether the target can be said out loud.
func (t SemanticTarget) Named() bool { return t.Label != "" }

// InputEvent is one observed navigation action.
//
// Deliberately small, and with one deliberate exception to "no string": Target may carry a
// control's admitted name. See SemanticTarget for the two licences and the shape filter —
// widening them is a privacy change and must be reviewed as one.
type InputEvent struct {
	Intent NavIntent `json:"intent"`
	// AtMS is milliseconds since the session began. Relative, so a record cannot be
	// correlated with anything outside the session it came from.
	AtMS int64 `json:"at_ms"`
	// Where is the pointer position for NavPoint, and zero for every other intent.
	Where PointerAt `json:"where,omitzero"`
	// Conditional says this intent came from a key that is only navigation in context.
	//
	// W/A/S/D and Space move a selection in a menu and move the player during gameplay.
	// They are admitted only while the screen looked like a set of choices, which is a
	// JUDGEMENT rather than an observation — the screen was assessed up to one inference
	// ago, and the player may have left. So the doubt travels with the evidence instead of
	// being resolved silently at the boundary.
	//
	// It is a bool, and that is the whole extent of what it adds. It says nothing about
	// WHICH key was pressed — `up` from an arrow and `up` from W are indistinguishable
	// here, which is the point: the privacy boundary is unchanged and only the strength of
	// the claim is qualified. Admitting this field required deliberately widening the type
	// allowlist in TestRawKeyIdentityCannotCrossTheBoundary; see the note there.
	Conditional bool `json:"conditional,omitempty"`
	// Target is what the event was aimed at, when the evidence at the time identified it.
	// Nil is the ordinary case and never a failure of capture. See SemanticTarget for what
	// may cross here and under which licences; admitting it widened the type allowlist in
	// TestRawKeyIdentityCannotCrossTheBoundary, deliberately and reviewed as such.
	Target *SemanticTarget `json:"target,omitempty"`
}

// admissibleInputs filters a batch down to the closed vocabulary.
//
// Bounded as well as filtered. A hook stream can deliver thousands of events between two
// inferences, and a correlation only needs to know WHICH intents preceded a change — retaining
// every repeat of a held key would make a session's memory a function of how hard somebody was
// holding down.
func admissibleInputs(in []InputEvent) []InputEvent {
	if len(in) == 0 {
		return nil
	}
	out := make([]InputEvent, 0, len(in))
	for _, e := range in {
		if !e.Intent.Known() {
			continue
		}
		if e.Intent != NavPoint {
			// Positions belong to pointer presses only. Zeroed here rather than trusted,
			// so a caller cannot smuggle a coordinate in on a keyboard intent.
			e.Where = PointerAt{}
		}
		// Admission, not trust, for the target too: a role that is not shaped like a role
		// is text in the wrong field, and a label past the bound is a paragraph wearing a
		// name's clothes. Dropping the TARGET, never the event — enrichment failing is not
		// capture failing.
		if e.Target != nil {
			t := *e.Target
			if !roleShaped(t.Role) || len([]rune(t.Label)) > MaxTargetLabelLength {
				e.Target = nil
			} else {
				e.Target = &t
			}
		}
		out = append(out, e)
		if len(out) >= MaxBufferedInputs {
			break
		}
	}
	return out
}

// MaxTargetLabelLength bounds a semantic target's label — generous for a control's name and
// far too small for captured content, like MaxScreenNameLength.
const MaxTargetLabelLength = 60

// MaxBufferedInputs bounds the events retained between two valid inferences.
const MaxBufferedInputs = 64

// MaxIntentsPerTransition bounds the per-transition correlation table.
const MaxIntentsPerTransition = 8

// IgnoreReason says why a physical event produced no navigation intent.
//
// A closed vocabulary, and it lives HERE rather than in the platform adapter that produces it,
// because it is a thing that CROSSES the boundary into a report. The consumer defines what may
// arrive; a producer that wanted to say something else would have to add a constant to this
// file, which is a reviewable act rather than a free-form string appearing in a diagnostic.
//
// None of these names a key. "Why was that ignored" has to be answerable without a diagnostic
// that reintroduces exactly the identity the boundary exists to discard.
type IgnoreReason string

const (
	// IgnoreUnsupported is a key with no navigation meaning — the overwhelming majority,
	// including every character key.
	IgnoreUnsupported IgnoreReason = "unsupported_navigation"
	// IgnoreAmbiguous is a key that navigates in menus and moves the player in gameplay.
	// Counted so the refusal is visible rather than silent.
	IgnoreAmbiguous IgnoreReason = "ambiguous_gameplay_key"
	// IgnoreRepeat is auto-repeat while a key is held.
	IgnoreRepeat IgnoreReason = "repeat"
	// IgnoreNoSession is navigation observed while nothing was watching.
	IgnoreNoSession IgnoreReason = "no_session"
	// IgnoreUnplaceablePointer is a pointer press Marco could not place inside the window
	// it was watching.
	//
	// A pointer press means nothing without a position, and a position means nothing
	// without the frame it is relative to. Both cases land here: no recent window bounds to
	// normalise against, and a press that landed outside the window. Counted rather than
	// admitted with a zero, because a press at the origin and a press nobody could place
	// are different things and only one of them happened at the origin.
	IgnoreUnplaceablePointer IgnoreReason = "unplaceable_pointer"
	// IgnoreMarcoOwned is input the person aimed at MARCO — its own control surface —
	// while a session was watching something else.
	//
	// # Why ownership is a capture-time question
	//
	// Pressing "Start Learning" is a click. So is pressing Stop, and so is clicking Marco to
	// see what it thinks. Every one of them is a real physical press that the hook sees, and
	// none of them is part of the task the person is demonstrating: they are how the person
	// OPERATES Marco, not something Marco is being shown.
	//
	// Geometry nearly hides this and does not solve it. A press outside the watched window's
	// rectangle is already refused as unplaceable, which covers a control panel sitting
	// beside the application — but Marco's overlay is a full-screen surface lying OVER the
	// watched window, so a press on it is inside the rectangle by construction and would be
	// admitted as though the person had clicked the application underneath.
	//
	// The alternative — discarding what was captured when the lifecycle buttons are pressed —
	// is worse than useless: it throws away real evidence to compensate for having admitted
	// evidence that was never real. Ownership is decided when the press is classified, from
	// context pushed by the same validated cycle every other piece of evidence is scoped to.
	IgnoreMarcoOwned IgnoreReason = "marco_owned"
)

var ignoreReasons = []IgnoreReason{
	IgnoreUnsupported, IgnoreAmbiguous, IgnoreRepeat, IgnoreNoSession,
	IgnoreUnplaceablePointer, IgnoreMarcoOwned,
}

// Known reports whether a reason is in the closed vocabulary.
func (r IgnoreReason) Known() bool {
	for _, k := range ignoreReasons {
		if k == r {
			return true
		}
	}
	return false
}

// InputStats is what the navigation producer did, in semantic terms only.
//
// Counts and reasons, never keys. It exists because a correlation that reported no edges has
// two very different explanations — the player pressed nothing, or the producer was not running
// — and a report that cannot tell them apart invites the reading this project has already made
// twice: infrastructure silence read as behaviour.
//
// Cumulative and monotone, so a snapshot taken on any slot supersedes the one before it.
type InputStats struct {
	// Received is every physical event the producer was offered.
	Received int `json:"received"`
	// Classified is how many became an intent inside an active session.
	Classified int `json:"classified"`
	// Ignored counts the rest, by reason.
	Ignored map[IgnoreReason]int `json:"ignored,omitempty"`
	// Conditional counts the classified intents that were admitted only because the screen
	// looked like a set of choices at the time.
	//
	// A subset of Classified, reported beside it so a reader can see how much of a session's
	// navigation rests on a judgement rather than on an unambiguous key.
	Conditional int `json:"conditional,omitempty"`
	// Dropped counts events the bounded hook queue refused. Non-zero means the classifier
	// fell behind; it is a diagnostic about Marco and never about the user.
	//
	// Reported rather than hidden because the alternative to dropping is blocking a
	// low-level keyboard hook, which is how Windows silently unhooks it.
	Dropped int `json:"dropped_buffer_full"`
	// Unavailable explains why no navigation can be observed at all, empty when it can.
	Unavailable string `json:"unavailable,omitempty"`
	// ControlsOffered is how many actionable controls the producer was last given to
	// resolve a press against, and PointerUnresolved how many placed presses matched none
	// of them.
	//
	// # Why these are counted separately from the refusals above
	//
	// A press that was PLACED and then resolved to nothing is not ignored — it is admitted,
	// it becomes evidence, and it is exactly the case that decides whether a click can be
	// learned. Until these existed the silence had two causes and neither was nameable:
	// nothing was on offer, or the press landed somewhere nothing claimed. Live, a
	// demonstration arrived complete and was refused `unresolved_pointer_target` with no way
	// to tell which had happened — the same shape of lost diagnosis this subsystem has now
	// hit four times.
	ControlsOffered   int `json:"controls_offered,omitempty"`
	PointerUnresolved int `json:"pointer_unresolved,omitempty"`
	// PointerUnnamed is presses that DID land on a control whose name was withheld — the
	// label gate working, not a failure to find one. The ordinary case for passive
	// observation, where only the plaintext role allowlist admits a name; an explicit Learn
	// licence is what widens it to the control a person activated.
	PointerUnnamed int `json:"pointer_unnamed,omitempty"`
	// PointerResolved is placed presses that DID name a control. The positive half, so a
	// reader can tell "resolution never works here" from "it worked and this one missed".
	PointerResolved int `json:"pointer_resolved,omitempty"`
}

// Observed reports whether the producer was actually running.
func (s InputStats) Observed() bool { return s.Unavailable == "" && s.Received > 0 }

// IgnoredTotal is every physical event that produced no intent.
func (s InputStats) IgnoredTotal() int {
	n := 0
	for _, v := range s.Ignored {
		n += v
	}
	return n
}

// admissibleStats filters a producer snapshot down to the closed reason vocabulary.
//
// The same treatment admissibleInputs gives events, for the same reason: IgnoreReason's
// underlying type is a string, so a platform layer could construct one holding anything.
func admissibleStats(in InputStats) InputStats {
	out := in
	out.Ignored = nil
	for r, n := range in.Ignored {
		if !r.Known() {
			continue
		}
		if out.Ignored == nil {
			out.Ignored = make(map[IgnoreReason]int, len(in.Ignored))
		}
		out.Ignored[r] = n
	}
	return out
}
